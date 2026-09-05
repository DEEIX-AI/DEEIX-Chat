package billing

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const usageReservationTTL = 3 * time.Hour

// ReserveUsageBalance 在真实调用前原子预留余额与周期额度预算。
func (r *Repo) ReserveUsageBalance(ctx context.Context, input domainbilling.UsageBalanceReservationRequest) (*domainbilling.UsageBalanceReservation, error) {
	input.RefNo = strings.TrimSpace(input.RefNo)
	input.Mode = strings.TrimSpace(input.Mode)
	if input.UserID == 0 || input.RefNo == "" || input.RequestedNanousd < 0 {
		return nil, repository.ErrInvalidInput
	}
	if input.Mode != "usage" && input.Mode != "period" {
		return nil, repository.ErrInvalidInput
	}
	if input.Mode == "period" && (input.PeriodStartAt == nil || input.PeriodEndAt == nil || !input.PeriodEndAt.After(*input.PeriodStartAt) || input.PeriodCreditNanousd < 0) {
		return nil, repository.ErrInvalidInput
	}

	var result *domainbilling.UsageBalanceReservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateBillingAccountForUpdate(tx, input.UserID)
		if err != nil {
			return err
		}
		var existing model.UsageReservation
		err = tx.Where("user_id = ? AND ref_no = ?", input.UserID, input.RefNo).First(&existing).Error
		if err == nil {
			return repository.ErrConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return dberror.Translate(err)
		}
		var legacyReservationCount int64
		if err = tx.Model(&model.BalanceTransaction{}).
			Where(
				"user_id = ? AND ref_no = ? AND type IN ?",
				input.UserID,
				input.RefNo,
				[]string{
					domainbilling.BalanceTransactionTypeUsageReserve,
					domainbilling.BalanceTransactionTypeUsageRefund,
				},
			).
			Count(&legacyReservationCount).Error; err != nil {
			return dberror.Translate(err)
		}
		if legacyReservationCount > 0 {
			return repository.ErrConflict
		}

		now := time.Now()
		activeReservationCount, err := countActiveUsageReservations(tx, input.UserID, now)
		if err != nil {
			return err
		}
		if activeReservationCount >= domainbilling.UsageReservationMaxActivePerUser {
			return repository.ErrUsageReservationLimitExceeded
		}
		availableBalanceNanousd := account.BalanceNanousd
		if availableBalanceNanousd < 0 {
			return repository.ErrInsufficientBalance
		}
		activeBalanceNanousd, err := sumActiveBalanceReservations(tx, input.UserID, 0, now)
		if err != nil {
			return err
		}
		availableBalanceNanousd = remainingNonNegativeBudget(availableBalanceNanousd, activeBalanceNanousd, 0)

		availableCreditNanousd := int64(0)
		if input.Mode == "period" {
			usedNanousd, usedErr := sumPeriodBilledNanousd(tx, input.UserID, *input.PeriodStartAt, *input.PeriodEndAt)
			if usedErr != nil {
				return usedErr
			}
			reservedCreditNanousd, reserveErr := sumActivePeriodCreditReservations(
				tx,
				input.UserID,
				*input.PeriodStartAt,
				*input.PeriodEndAt,
				0,
				now,
			)
			if reserveErr != nil {
				return reserveErr
			}
			availableCreditNanousd = remainingNonNegativeBudget(input.PeriodCreditNanousd, usedNanousd, reservedCreditNanousd)
		}

		availableNanousd := addNonNegativeInt64(availableCreditNanousd, availableBalanceNanousd)
		requestedNanousd := input.RequestedNanousd
		if requestedNanousd <= 0 {
			// 默认预算按剩余槽位分配，在限制并发风险的同时保留分支并行生成能力。
			remainingSlots := int64(domainbilling.UsageReservationMaxActivePerUser) - activeReservationCount
			requestedNanousd = divideBudgetAcrossSlots(availableNanousd, remainingSlots)
		}
		if requestedNanousd <= 0 || requestedNanousd > availableNanousd {
			return repository.ErrInsufficientBalance
		}
		periodCreditNanousd := minInt64(requestedNanousd, availableCreditNanousd)
		reservationRow := model.UsageReservation{
			UserID:              input.UserID,
			RefNo:               input.RefNo,
			Mode:                input.Mode,
			BalanceNanousd:      requestedNanousd - periodCreditNanousd,
			PeriodCreditNanousd: periodCreditNanousd,
			PeriodLimitNanousd:  input.PeriodCreditNanousd,
			PeriodStartAt:       input.PeriodStartAt,
			PeriodEndAt:         input.PeriodEndAt,
			Status:              domainbilling.UsageReservationStatusActive,
			ExpiresAt:           now.Add(usageReservationTTL),
		}
		if err = tx.Create(&reservationRow).Error; err != nil {
			if translated := dberror.Translate(err); errors.Is(translated, repository.ErrDuplicate) {
				return repository.ErrConflict
			}
			return dberror.Translate(err)
		}
		domain := toDomainUsageReservation(reservationRow)
		result = &domain
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RaiseUsageBalanceReservation 在请求形状确定后把预留抬高到覆盖预估成本。预留只增不减：
// 预估不超过现有预留时保持管理端配置的风险预算；可用预算（排除本预留后的余额与周期额度）
// 不足以覆盖预估时返回 ErrInsufficientBalance，由调用方在产生任何上游费用之前终止运行。
func (r *Repo) RaiseUsageBalanceReservation(ctx context.Context, userID uint, refNo string, requiredNanousd int64) error {
	refNo = strings.TrimSpace(refNo)
	if userID == 0 || refNo == "" || requiredNanousd < 0 {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateBillingAccountForUpdate(tx, userID)
		if err != nil {
			return err
		}
		now := time.Now()
		var reservation model.UsageReservation
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND ref_no = ?", userID, refNo).
			First(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrConflict
			}
			return dberror.Translate(err)
		}
		if reservation.Status != domainbilling.UsageReservationStatusActive || !reservation.ExpiresAt.After(now) {
			return repository.ErrConflict
		}
		if requiredNanousd <= addNonNegativeInt64(reservation.BalanceNanousd, reservation.PeriodCreditNanousd) {
			return nil
		}

		otherBalanceReservedNanousd, err := sumActiveBalanceReservations(tx, userID, reservation.ID, now)
		if err != nil {
			return err
		}
		availableBalanceNanousd := remainingNonNegativeBudget(account.BalanceNanousd, otherBalanceReservedNanousd, 0)
		availableCreditNanousd := int64(0)
		if reservation.Mode == "period" && reservation.PeriodStartAt != nil && reservation.PeriodEndAt != nil {
			usedNanousd, usedErr := sumPeriodBilledNanousd(tx, userID, *reservation.PeriodStartAt, *reservation.PeriodEndAt)
			if usedErr != nil {
				return usedErr
			}
			otherCreditReservedNanousd, reserveErr := sumActivePeriodCreditReservations(
				tx,
				userID,
				*reservation.PeriodStartAt,
				*reservation.PeriodEndAt,
				reservation.ID,
				now,
			)
			if reserveErr != nil {
				return reserveErr
			}
			availableCreditNanousd = remainingNonNegativeBudget(reservation.PeriodLimitNanousd, usedNanousd, otherCreditReservedNanousd)
		}
		if requiredNanousd > addNonNegativeInt64(availableCreditNanousd, availableBalanceNanousd) {
			return repository.ErrInsufficientBalance
		}
		periodCreditNanousd := minInt64(requiredNanousd, availableCreditNanousd)
		return dberror.Translate(tx.Model(&reservation).Updates(map[string]any{
			"balance_nanousd":       requiredNanousd - periodCreditNanousd,
			"period_credit_nanousd": periodCreditNanousd,
		}).Error)
	})
}

// ReleaseUsageBalanceReservation 在调用失败时释放预留预算，重复调用保持幂等。
func (r *Repo) ReleaseUsageBalanceReservation(ctx context.Context, userID uint, refNo string) error {
	refNo = strings.TrimSpace(refNo)
	if userID == 0 || refNo == "" {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation model.UsageReservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND ref_no = ?", userID, refNo).
			First(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return dberror.Translate(err)
		}
		if reservation.Status != domainbilling.UsageReservationStatusActive {
			return nil
		}
		releasedAt := time.Now()
		return dberror.Translate(tx.Model(&reservation).Updates(map[string]any{
			"status":      domainbilling.UsageReservationStatusReleased,
			"released_at": releasedAt,
		}).Error)
	})
}

// RenewUsageBalanceReservation 延长仍在运行的预算预留租约。
func (r *Repo) RenewUsageBalanceReservation(ctx context.Context, userID uint, refNo string) error {
	refNo = strings.TrimSpace(refNo)
	if userID == 0 || refNo == "" {
		return repository.ErrInvalidInput
	}
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.UsageReservation{}).
		Where(
			"user_id = ? AND ref_no = ? AND status = ? AND expires_at > ?",
			userID,
			refNo,
			domainbilling.UsageReservationStatusActive,
			now,
		).
		Updates(map[string]any{"expires_at": now.Add(usageReservationTTL)})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrConflict
	}
	return nil
}

// MarkUsageReservationReconciliationRequired 保留已产生上游费用的预算，等待账单核对。
func (r *Repo) MarkUsageReservationReconciliationRequired(ctx context.Context, userID uint, refNo string, failureCode string) error {
	refNo = strings.TrimSpace(refNo)
	failureCode = strings.TrimSpace(failureCode)
	if userID == 0 || refNo == "" || failureCode == "" {
		return repository.ErrInvalidInput
	}
	if len(failureCode) > 64 {
		failureCode = failureCode[:64]
	}
	reconciliationAt := time.Now()
	result := r.db.WithContext(ctx).Model(&model.UsageReservation{}).
		Where(
			"user_id = ? AND ref_no = ? AND status IN ?",
			userID,
			refNo,
			[]string{
				domainbilling.UsageReservationStatusActive,
				domainbilling.UsageReservationStatusReconciliation,
			},
		).
		Updates(map[string]any{
			"status":            domainbilling.UsageReservationStatusReconciliation,
			"failure_code":      failureCode,
			"reconciliation_at": reconciliationAt,
		})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrConflict
	}
	return nil
}

func getUsageReservationForSettlement(
	tx *gorm.DB,
	userID uint,
	reservation *domainbilling.UsageBalanceReservation,
) (*model.UsageReservation, bool, error) {
	if reservation == nil {
		return nil, false, nil
	}
	if userID == 0 || (reservation.UserID != 0 && reservation.UserID != userID) || strings.TrimSpace(reservation.RefNo) == "" {
		return nil, false, repository.ErrConflict
	}
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND ref_no = ?", userID, strings.TrimSpace(reservation.RefNo))
	if reservation.ID > 0 {
		query = query.Where("id = ?", reservation.ID)
	}
	var item model.UsageReservation
	if err := query.First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, repository.ErrConflict
		}
		return nil, false, dberror.Translate(err)
	}
	switch item.Status {
	case domainbilling.UsageReservationStatusActive, domainbilling.UsageReservationStatusReconciliation:
		return &item, false, nil
	case domainbilling.UsageReservationStatusSettled:
		if item.UsageLedgerID == 0 {
			return nil, false, repository.ErrConflict
		}
		return &item, true, nil
	default:
		return nil, false, repository.ErrConflict
	}
}

func settleUsageReservation(tx *gorm.DB, reservation *model.UsageReservation, usageLedgerID uint) error {
	if reservation == nil {
		return nil
	}
	settledAt := time.Now()
	return dberror.Translate(tx.Model(reservation).Updates(map[string]any{
		"status":          domainbilling.UsageReservationStatusSettled,
		"usage_ledger_id": usageLedgerID,
		"settled_at":      settledAt,
	}).Error)
}

func matchesPeriodReservation(reservation *model.UsageReservation, periodStart time.Time, periodEnd time.Time, periodCreditNanousd int64) bool {
	return reservation != nil &&
		reservation.Mode == "period" &&
		reservation.PeriodStartAt != nil &&
		reservation.PeriodEndAt != nil &&
		reservation.PeriodStartAt.Equal(periodStart) &&
		reservation.PeriodEndAt.Equal(periodEnd) &&
		reservation.PeriodLimitNanousd == periodCreditNanousd
}

func sumActiveBalanceReservations(tx *gorm.DB, userID uint, excludeID uint, now time.Time) (int64, error) {
	query := tx.Model(&model.UsageReservation{}).
		Select("COALESCE(SUM(balance_nanousd), 0)").
		Where(
			"user_id = ? AND ((status = ? AND expires_at > ?) OR status = ?)",
			userID,
			domainbilling.UsageReservationStatusActive,
			now,
			domainbilling.UsageReservationStatusReconciliation,
		)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var result int64
	err := query.Scan(&result).Error
	return result, dberror.Translate(err)
}

// countActiveUsageReservations 统计仍占用并发槽位的有效租约与待核对记录。
func countActiveUsageReservations(tx *gorm.DB, userID uint, now time.Time) (int64, error) {
	var count int64
	err := tx.Model(&model.UsageReservation{}).
		Where(
			"user_id = ? AND ((status = ? AND expires_at > ?) OR status = ?)",
			userID,
			domainbilling.UsageReservationStatusActive,
			now,
			domainbilling.UsageReservationStatusReconciliation,
		).
		Count(&count).Error
	return count, dberror.Translate(err)
}

// sumPeriodBilledNanousd 统计周期内已结算的付费用量，免费模型不占用周期额度。
func sumPeriodBilledNanousd(tx *gorm.DB, userID uint, periodStart time.Time, periodEnd time.Time) (int64, error) {
	var result int64
	err := tx.Model(&model.UsageLedger{}).
		Select("COALESCE(SUM(billed_nanousd), 0)").
		Where("user_id = ? AND is_free_model = ? AND billing_at >= ? AND billing_at < ?", userID, false, periodStart, periodEnd).
		Scan(&result).Error
	return result, dberror.Translate(err)
}

func sumActivePeriodCreditReservations(
	tx *gorm.DB,
	userID uint,
	periodStart time.Time,
	periodEnd time.Time,
	excludeID uint,
	now time.Time,
) (int64, error) {
	query := tx.Model(&model.UsageReservation{}).
		Select("COALESCE(SUM(period_credit_nanousd), 0)").
		Where(
			"user_id = ? AND ((status = ? AND expires_at > ?) OR status = ?) AND period_start_at = ? AND period_end_at = ?",
			userID,
			domainbilling.UsageReservationStatusActive,
			now,
			domainbilling.UsageReservationStatusReconciliation,
			periodStart,
			periodEnd,
		)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var result int64
	err := query.Scan(&result).Error
	return result, dberror.Translate(err)
}

func addNonNegativeInt64(a int64, b int64) int64 {
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// divideBudgetAcrossSlots 向上取整分配预算，确保剩余槽位获得稳定的风险额度。
func divideBudgetAcrossSlots(availableNanousd int64, slots int64) int64 {
	if availableNanousd <= 0 || slots <= 0 {
		return 0
	}
	result := availableNanousd / slots
	if availableNanousd%slots != 0 {
		result++
	}
	return result
}

func remainingNonNegativeBudget(limit int64, consumed int64, reserved int64) int64 {
	if limit <= 0 {
		return 0
	}
	consumed = clampNonNegative(consumed)
	if consumed >= limit {
		return 0
	}
	remaining := limit - consumed
	reserved = clampNonNegative(reserved)
	if reserved >= remaining {
		return 0
	}
	return remaining - reserved
}

func toDomainUsageReservation(item model.UsageReservation) domainbilling.UsageBalanceReservation {
	return domainbilling.UsageBalanceReservation{
		ID:                  item.ID,
		UserID:              item.UserID,
		RefNo:               item.RefNo,
		Mode:                item.Mode,
		BalanceNanousd:      item.BalanceNanousd,
		PeriodCreditNanousd: item.PeriodCreditNanousd,
		PeriodLimitNanousd:  item.PeriodLimitNanousd,
		PeriodStartAt:       item.PeriodStartAt,
		PeriodEndAt:         item.PeriodEndAt,
		Status:              item.Status,
		UsageLedgerID:       item.UsageLedgerID,
		ExpiresAt:           item.ExpiresAt,
		SettledAt:           item.SettledAt,
		ReleasedAt:          item.ReleasedAt,
		ReconciliationAt:    item.ReconciliationAt,
		FailureCode:         item.FailureCode,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}
