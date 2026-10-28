package billing

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GrantExpiringCredit 发放一笔独立到期的限时额度，并同步增加账户总余额。
func (r *Repo) GrantExpiringCredit(ctx context.Context, input repository.ExpiringCreditGrantInput) (*domainbilling.ExpiringCredit, *domainbilling.BillingAccount, error) {
	userID, amountNanousd := input.UserID, input.AmountNanousd
	createdByUserID, refNo, description := input.CreatedByUserID, input.RefNo, input.Description
	if input.DurationSeconds < 8640 || input.DurationSeconds > 3650*86400 {
		return nil, nil, repository.ErrInvalidInput
	}
	if userID == 0 || amountNanousd <= 0 {
		return nil, nil, repository.ErrInvalidInput
	}
	var creditResult domainbilling.ExpiringCredit
	var accountResult domainbilling.BillingAccount
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, now, err := getOrCreateBillingAccountForUpdate(tx, userID)
		if err != nil {
			return err
		}
		refNo = strings.TrimSpace(refNo)
		if refNo != "" {
			var existing models.ExpiringCredit
			err = tx.Where("user_id = ? AND ref_no = ?", userID, refNo).First(&existing).Error
			if err == nil {
				if existing.OriginalNanousd != amountNanousd || existing.CreatedByUserID != createdByUserID ||
					existing.Description != textutil.FirstNonEmpty(strings.TrimSpace(description), "管理员发放限时额度") ||
					existing.GrantDurationSeconds != input.DurationSeconds {
					return repository.ErrConflict
				}
				creditResult = toDomainExpiringCredit(existing)
				accountResult = toDomainBillingAccount(*account)
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return dberror.Translate(err)
			}
		}
		if account.BalanceNanousd > math.MaxInt64-amountNanousd {
			return repository.ErrInvalidInput
		}
		expiresAt := now.Add(time.Duration(input.DurationSeconds) * time.Second)
		credit, err := createExpiringCreditBatch(tx, account, amountNanousd, expiresAt, createdByUserID, refNo,
			textutil.FirstNonEmpty(strings.TrimSpace(description), "管理员发放限时额度"), input.DurationSeconds)
		if err != nil {
			return err
		}
		account.BalanceNanousd += amountNanousd
		account.Currency = "USD"
		account.Status = "active"
		if err = tx.Model(account).Updates(map[string]any{
			"balance_nanousd": account.BalanceNanousd,
			"currency":        account.Currency,
			"status":          account.Status,
		}).Error; err != nil {
			return dberror.Translate(err)
		}
		transaction := models.BalanceTransaction{
			AccountID:           account.ID,
			UserID:              userID,
			Type:                domainbilling.BalanceTransactionTypeExpiringCreditGrant,
			AmountNanousd:       amountNanousd,
			BalanceAfterNanousd: account.BalanceNanousd,
			RefType:             "expiring_credit",
			RefID:               credit.ID,
			RefNo:               credit.RefNo,
			Description:         credit.Description,
		}
		if err = tx.Create(&transaction).Error; err != nil {
			return dberror.Translate(err)
		}
		creditResult = toDomainExpiringCredit(*credit)
		accountResult = toDomainBillingAccount(*account)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &creditResult, &accountResult, nil
}

// ExpireDueExpiringCredits 回收已到期批次的剩余额度。
func (r *Repo) ExpireDueExpiringCredits(ctx context.Context, now time.Time, limit int) (int64, error) {
	if now.IsZero() {
		now = time.Now()
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	dueRows := make([]struct {
		UserID uint
	}, 0, limit)
	if err := r.db.WithContext(ctx).Model(&models.ExpiringCredit{}).
		Select("user_id").Group("user_id").
		Where("status = ? AND remaining_nanousd > 0 AND expires_at <= ?", domainbilling.ExpiringCreditStatusActive, now).
		Order("MIN(expires_at) ASC, user_id ASC").
		Limit(limit).
		Find(&dueRows).Error; err != nil {
		return 0, dberror.Translate(err)
	}
	var expiredCount int64
	for _, row := range dueRows {
		userID := row.UserID
		if userID == 0 {
			continue
		}
		var count int64
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			account, err := lockBillingAccount(tx, userID)
			if err != nil {
				return err
			}
			count, err = expireDueExpiringCreditsForAccount(tx, account, now)
			return err
		})
		if err != nil {
			return expiredCount, err
		}
		expiredCount += count
	}
	return expiredCount, nil
}

func expireDueExpiringCreditsForAccount(tx *gorm.DB, account *models.BillingAccount, now time.Time) (int64, error) {
	if account == nil || account.UserID == 0 {
		return 0, repository.ErrInvalidInput
	}
	credits := make([]models.ExpiringCredit, 0)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status = ? AND remaining_nanousd > 0 AND expires_at <= ?", account.UserID, domainbilling.ExpiringCreditStatusActive, now).
		Order("expires_at ASC, id ASC").
		Find(&credits).Error; err != nil {
		return 0, dberror.Translate(err)
	}
	for index := range credits {
		credit := &credits[index]
		deduction := credit.RemainingNanousd
		if deduction > account.BalanceNanousd {
			deduction = account.BalanceNanousd
		}
		if deduction < 0 {
			deduction = 0
		}
		account.BalanceNanousd -= deduction
		if err := tx.Model(credit).Updates(map[string]any{
			"remaining_nanousd": 0,
			"status":            domainbilling.ExpiringCreditStatusExpired,
			"expired_at":        now,
		}).Error; err != nil {
			return 0, dberror.Translate(err)
		}
		if deduction > 0 {
			transaction := models.BalanceTransaction{
				AccountID:           account.ID,
				UserID:              account.UserID,
				Type:                domainbilling.BalanceTransactionTypeExpiringCreditExpire,
				AmountNanousd:       -deduction,
				BalanceAfterNanousd: account.BalanceNanousd,
				RefType:             "expiring_credit",
				RefID:               credit.ID,
				RefNo:               credit.RefNo,
				Description:         "限时额度到期回收",
			}
			if err := tx.Create(&transaction).Error; err != nil {
				return 0, dberror.Translate(err)
			}
		}
	}
	if len(credits) > 0 {
		if err := tx.Model(account).Update("balance_nanousd", account.BalanceNanousd).Error; err != nil {
			return 0, dberror.Translate(err)
		}
	}
	return int64(len(credits)), nil
}

// reduceActiveExpiringCredits 按最早到期优先减少批次剩余值；账户余额由调用方统一变更。
func reduceActiveExpiringCredits(tx *gorm.DB, userID uint, amountNanousd int64, now time.Time, exhaustedStatus string) error {
	if userID == 0 || amountNanousd <= 0 {
		return nil
	}
	credits := make([]models.ExpiringCredit, 0)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status = ? AND remaining_nanousd > 0 AND expires_at > ?", userID, domainbilling.ExpiringCreditStatusActive, now).
		Order("expires_at ASC, id ASC").
		Find(&credits).Error; err != nil {
		return dberror.Translate(err)
	}
	remaining := amountNanousd
	for index := range credits {
		if remaining <= 0 {
			break
		}
		credit := &credits[index]
		used := credit.RemainingNanousd
		if used > remaining {
			used = remaining
		}
		next := credit.RemainingNanousd - used
		updates := map[string]any{"remaining_nanousd": next}
		if next == 0 {
			updates["status"] = exhaustedStatus
			if exhaustedStatus == domainbilling.ExpiringCreditStatusConsumed {
				updates["consumed_at"] = now
			}
		}
		if err := tx.Model(credit).Updates(updates).Error; err != nil {
			return dberror.Translate(err)
		}
		remaining -= used
	}
	return nil
}

func toDomainExpiringCredit(item models.ExpiringCredit) domainbilling.ExpiringCredit {
	return domainbilling.ExpiringCredit{
		ID:               item.ID,
		AccountID:        item.AccountID,
		UserID:           item.UserID,
		OriginalNanousd:  item.OriginalNanousd,
		RemainingNanousd: item.RemainingNanousd,
		Status:           item.Status,
		ExpiresAt:        item.ExpiresAt,
		ConsumedAt:       item.ConsumedAt,
		ExpiredAt:        item.ExpiredAt,
		CreatedByUserID:  item.CreatedByUserID,
		RefNo:            item.RefNo,
		Description:      item.Description,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

// getOrCreateBillingAccountForUpdate is the single balance gate. Call only inside
// a transaction: lock the account before credit batches, then expire due funds.
// Reservations are budget checks, not an extension of a credit's lifetime;
// eligibility is evaluated when settling the actual usage.
func getOrCreateBillingAccountForUpdate(tx *gorm.DB, userID uint) (*models.BillingAccount, time.Time, error) {
	account, err := lockBillingAccount(tx, userID)
	if err != nil {
		return nil, time.Time{}, err
	}
	// Reuse one timestamp after the lock wait for expiration and consumption.
	now := time.Now()
	_, err = expireDueExpiringCreditsForAccount(tx, account, now)
	return account, now, err
}

// createExpiringCreditBatch records the expirable portion of a deposit. A deposit
// first pays existing debt; that portion is already spent and must never expire
// again against a subsequent permanent top-up. Account/ledger writes remain in
// the caller's same transaction.
func createExpiringCreditBatch(tx *gorm.DB, account *models.BillingAccount, amount int64, expiresAt time.Time,
	actor uint, refNo, description string, durationSeconds int64) (*models.ExpiringCredit, error) {
	if amount <= 0 || account.BalanceNanousd > math.MaxInt64-amount {
		return nil, repository.ErrInvalidInput
	}
	remaining := amount
	if account.BalanceNanousd < 0 {
		remaining = max(int64(0), account.BalanceNanousd+amount)
	}
	credit := models.ExpiringCredit{
		AccountID: account.ID, UserID: account.UserID, OriginalNanousd: amount,
		RemainingNanousd: remaining, GrantDurationSeconds: durationSeconds,
		Status: domainbilling.ExpiringCreditStatusActive, ExpiresAt: expiresAt,
		CreatedByUserID: actor, RefNo: strings.TrimSpace(refNo), Description: description,
	}
	if remaining == 0 {
		now := time.Now()
		credit.Status = domainbilling.ExpiringCreditStatusConsumed
		credit.ConsumedAt = &now
	}
	if err := tx.Create(&credit).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return &credit, nil
}
