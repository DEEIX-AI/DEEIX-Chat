package billing

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
func translateError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	return err
}

// Repo 封装计费数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListActivePlans 查询启用套餐。
func (r *Repo) ListActivePlans(ctx context.Context) ([]domainbilling.Plan, error) {
	items := make([]model.BillingPlan, 0)
	if err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainbilling.Plan, 0, len(items))
	for _, item := range items {
		results = append(results, domainbilling.Plan{
			ID:                  item.ID,
			Code:                item.Code,
			Name:                item.Name,
			Description:         item.Description,
			FeatureJSON:         item.FeatureJSON,
			PeriodCreditNanousd: item.PeriodCreditNanousd,
			DiscountPercent:     item.DiscountPercent,
			SortOrder:           item.SortOrder,
			IsActive:            item.IsActive,
			CreatedAt:           item.CreatedAt,
			UpdatedAt:           item.UpdatedAt,
		})
	}
	return results, nil
}

// ListActivePricesByPlanIDs 查询一批套餐的启用价格。
func (r *Repo) ListActivePricesByPlanIDs(ctx context.Context, planIDs []uint) ([]domainbilling.Price, error) {
	items := make([]model.BillingPrice, 0)
	if len(planIDs) == 0 {
		return []domainbilling.Price{}, nil
	}
	if err := r.db.WithContext(ctx).
		Where("plan_id IN ? AND is_active = ?", planIDs, true).
		Order("plan_id ASC, amount_cents ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainbilling.Price, 0, len(items))
	for _, item := range items {
		results = append(results, domainbilling.Price{
			ID:               item.ID,
			PlanID:           item.PlanID,
			Code:             item.Code,
			BillingInterval:  item.BillingInterval,
			Currency:         item.Currency,
			AmountCents:      item.AmountCents,
			IsActive:         item.IsActive,
			IsDefault:        item.IsDefault,
			ExternalPriceRef: item.ExternalPriceRef,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
		})
	}
	return results, nil
}

// GetPriceByID 查询价格。
func (r *Repo) GetPriceByID(ctx context.Context, priceID uint) (*domainbilling.Price, error) {
	var item model.BillingPrice
	if err := r.db.WithContext(ctx).Where("id = ?", priceID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return &domainbilling.Price{
		ID:               item.ID,
		PlanID:           item.PlanID,
		Code:             item.Code,
		BillingInterval:  item.BillingInterval,
		Currency:         item.Currency,
		AmountCents:      item.AmountCents,
		IsActive:         item.IsActive,
		IsDefault:        item.IsDefault,
		ExternalPriceRef: item.ExternalPriceRef,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}, nil
}

// GetPlanByID 查询套餐。
func (r *Repo) GetPlanByID(ctx context.Context, planID uint) (*domainbilling.Plan, error) {
	var item model.BillingPlan
	if err := r.db.WithContext(ctx).Where("id = ?", planID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return &domainbilling.Plan{
		ID:                  item.ID,
		Code:                item.Code,
		Name:                item.Name,
		Description:         item.Description,
		FeatureJSON:         item.FeatureJSON,
		PeriodCreditNanousd: item.PeriodCreditNanousd,
		DiscountPercent:     item.DiscountPercent,
		SortOrder:           item.SortOrder,
		IsActive:            item.IsActive,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}, nil
}

// ListPlansByIDs 查询一批套餐。
func (r *Repo) ListPlansByIDs(ctx context.Context, planIDs []uint) ([]domainbilling.Plan, error) {
	items := make([]model.BillingPlan, 0)
	if len(planIDs) == 0 {
		return []domainbilling.Plan{}, nil
	}
	if err := r.db.WithContext(ctx).
		Where("id IN ?", planIDs).
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainbilling.Plan, 0, len(items))
	for _, item := range items {
		results = append(results, domainbilling.Plan{
			ID:                  item.ID,
			Code:                item.Code,
			Name:                item.Name,
			Description:         item.Description,
			FeatureJSON:         item.FeatureJSON,
			PeriodCreditNanousd: item.PeriodCreditNanousd,
			DiscountPercent:     item.DiscountPercent,
			SortOrder:           item.SortOrder,
			IsActive:            item.IsActive,
			CreatedAt:           item.CreatedAt,
			UpdatedAt:           item.UpdatedAt,
		})
	}
	return results, nil
}

// GetActivePlanByCode 按编码查询启用套餐。
func (r *Repo) GetActivePlanByCode(ctx context.Context, code string) (*domainbilling.Plan, error) {
	var item model.BillingPlan
	if err := r.db.WithContext(ctx).
		Where("code = ? AND is_active = ?", strings.TrimSpace(code), true).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return &domainbilling.Plan{
		ID:                  item.ID,
		Code:                item.Code,
		Name:                item.Name,
		Description:         item.Description,
		FeatureJSON:         item.FeatureJSON,
		PeriodCreditNanousd: item.PeriodCreditNanousd,
		DiscountPercent:     item.DiscountPercent,
		SortOrder:           item.SortOrder,
		IsActive:            item.IsActive,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}, nil
}

// UpdatePlanWithDefaultPrice 更新套餐与默认价格。
func (r *Repo) UpdatePlanWithDefaultPrice(ctx context.Context, plan *domainbilling.Plan, price *domainbilling.Price) error {
	if plan == nil || price == nil || plan.ID == 0 {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		planUpdates := map[string]interface{}{
			"name":                  strings.TrimSpace(plan.Name),
			"description":           strings.TrimSpace(plan.Description),
			"period_credit_nanousd": clampNonNegative(plan.PeriodCreditNanousd),
			"discount_percent":      clampPercent(plan.DiscountPercent),
			"is_active":             true,
		}
		if err := tx.Model(&model.BillingPlan{}).
			Where("id = ?", plan.ID).
			Updates(planUpdates).Error; err != nil {
			return translateError(err)
		}

		if err := tx.Model(&model.BillingPrice{}).
			Where("plan_id = ? AND is_default = ?", plan.ID, true).
			Update("is_default", false).Error; err != nil {
			return translateError(err)
		}

		var record model.BillingPrice
		err := tx.Where("plan_id = ? AND code = ?", plan.ID, strings.TrimSpace(price.Code)).
			First(&record).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return translateError(err)
		}

		updates := map[string]interface{}{
			"plan_id":            plan.ID,
			"code":               strings.TrimSpace(price.Code),
			"billing_interval":   normalizeInterval(price.BillingInterval),
			"currency":           normalizeCurrency(price.Currency),
			"amount_cents":       clampNonNegative(price.AmountCents),
			"is_active":          true,
			"is_default":         true,
			"external_price_ref": strings.TrimSpace(price.ExternalPriceRef),
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			record = model.BillingPrice{
				PlanID: plan.ID,
				Code:   strings.TrimSpace(price.Code),
			}
			if err := tx.Create(&record).Error; err != nil {
				return translateError(err)
			}
		}
		return translateError(tx.Model(&record).Updates(updates).Error)
	})
}

// ListCurrentSubscriptionsByUserIDs 查询一批用户当前有效的活跃订阅。
func (r *Repo) ListCurrentSubscriptionsByUserIDs(
	ctx context.Context,
	userIDs []uint,
	now time.Time,
) ([]domainbilling.Subscription, error) {
	items := make([]model.Subscription, 0)
	if len(userIDs) == 0 {
		return []domainbilling.Subscription{}, nil
	}

	if err := r.db.WithContext(ctx).
		Where(
			"user_id IN ? AND status = ? AND (current_period_end_at IS NULL OR current_period_end_at >= ?)",
			userIDs,
			"active",
			now,
		).
		Order("user_id ASC, current_period_end_at DESC NULLS LAST, id DESC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainbilling.Subscription, 0, len(items))
	for _, item := range items {
		results = append(results, domainbilling.Subscription{
			ID:                   item.ID,
			UserID:               item.UserID,
			PlanID:               item.PlanID,
			PriceID:              item.PriceID,
			Status:               item.Status,
			StartAt:              item.StartAt,
			CurrentPeriodStartAt: item.CurrentPeriodStartAt,
			CurrentPeriodEndAt:   item.CurrentPeriodEndAt,
			CancelAtPeriodEnd:    item.CancelAtPeriodEnd,
			CanceledAt:           item.CanceledAt,
			AutoRenew:            item.AutoRenew,
			CreatedAt:            item.CreatedAt,
			UpdatedAt:            item.UpdatedAt,
		})
	}
	return results, nil
}

// CreateSubscription 创建订阅。
func (r *Repo) CreateSubscription(ctx context.Context, item *model.Subscription) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// ReplaceSubscription 原子替换用户当前活跃订阅。
func (r *Repo) ReplaceSubscription(ctx context.Context, item *domainbilling.Subscription) error {
	if item == nil {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Subscription{}).
			Where("user_id = ? AND status = ?", item.UserID, "active").
			Updates(map[string]interface{}{
				"status":                "expired",
				"auto_renew":            false,
				"cancel_at_period_end":  false,
				"current_period_end_at": time.Now(),
			}).Error; err != nil {
			return err
		}
		record := model.Subscription{
			UserID:               item.UserID,
			PlanID:               item.PlanID,
			PriceID:              item.PriceID,
			Status:               item.Status,
			StartAt:              item.StartAt,
			CurrentPeriodStartAt: item.CurrentPeriodStartAt,
			CurrentPeriodEndAt:   item.CurrentPeriodEndAt,
			CancelAtPeriodEnd:    item.CancelAtPeriodEnd,
			CanceledAt:           item.CanceledAt,
			AutoRenew:            item.AutoRenew,
		}
		return tx.Create(&record).Error
	})
}

// CreatePaymentOrder 创建支付单。
func (r *Repo) CreatePaymentOrder(ctx context.Context, item *domainbilling.PaymentOrder) (*domainbilling.PaymentOrder, error) {
	if item == nil || strings.TrimSpace(item.OrderNo) == "" {
		return nil, repository.ErrInvalidInput
	}
	record := model.PaymentOrder{
		OrderNo:         strings.TrimSpace(item.OrderNo),
		OrderType:       normalizeOrderType(item.OrderType),
		UserID:          item.UserID,
		PlanID:          item.PlanID,
		PriceID:         item.PriceID,
		Provider:        strings.TrimSpace(item.Provider),
		Status:          firstNonEmpty(strings.TrimSpace(item.Status), domainbilling.PaymentStatusPending),
		BaseCurrency:    normalizeCurrency(item.BaseCurrency),
		BaseAmountCents: clampNonNegative(item.BaseAmountCents),
		PayCurrency:     normalizeCurrency(item.PayCurrency),
		PayAmountCents:  clampNonNegative(item.PayAmountCents),
		FXRate:          strings.TrimSpace(item.FXRate),
		CreditNanousd:   clampNonNegative(item.CreditNanousd),
		BillingInterval: normalizeInterval(item.BillingInterval),
		Cycles:          item.Cycles,
		ExpiredAt:       item.ExpiredAt,
		SnapshotJSON:    strings.TrimSpace(item.SnapshotJSON),
	}
	if record.Cycles <= 0 {
		record.Cycles = 1
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomainPaymentOrder(record)
	return &result, nil
}

// UpdatePaymentOrderCheckout 保存外部收银台信息。
func (r *Repo) UpdatePaymentOrderCheckout(ctx context.Context, orderNo string, externalCheckoutID string, checkoutURL string) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return repository.ErrInvalidInput
	}
	return translateError(r.db.WithContext(ctx).
		Model(&model.PaymentOrder{}).
		Where("order_no = ?", orderNo).
		Updates(map[string]interface{}{
			"external_checkout_id": strings.TrimSpace(externalCheckoutID),
			"checkout_url":         strings.TrimSpace(checkoutURL),
		}).Error)
}

// GetPaymentOrderByOrderNo 查询支付单。
func (r *Repo) GetPaymentOrderByOrderNo(ctx context.Context, orderNo string) (*domainbilling.PaymentOrder, error) {
	var record model.PaymentOrder
	if err := r.db.WithContext(ctx).Where("order_no = ?", strings.TrimSpace(orderNo)).First(&record).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomainPaymentOrder(record)
	return &result, nil
}

// MarkPaymentOrderPaidAndReplaceSubscription 标记支付成功并开通订阅，重复回调保持幂等。
func (r *Repo) MarkPaymentOrderPaidAndReplaceSubscription(
	ctx context.Context,
	orderNo string,
	externalPaymentID string,
	paidAt time.Time,
	subscription *domainbilling.Subscription,
) (*domainbilling.PaymentOrder, bool, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" || subscription == nil {
		return nil, false, repository.ErrInvalidInput
	}
	if paidAt.IsZero() {
		paidAt = time.Now()
	}

	var result domainbilling.PaymentOrder
	activated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.PaymentOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return translateError(err)
		}
		if order.Status == domainbilling.PaymentStatusPaid {
			result = toDomainPaymentOrder(order)
			return nil
		}
		if order.Status != domainbilling.PaymentStatusPending {
			return repository.ErrInvalidInput
		}
		if order.OrderType != "" && order.OrderType != domainbilling.PaymentOrderTypeSubscription {
			return repository.ErrInvalidInput
		}
		if order.ExpiredAt != nil && order.ExpiredAt.Before(paidAt) {
			if err := tx.Model(&order).Updates(map[string]interface{}{
				"status": domainbilling.PaymentStatusExpired,
			}).Error; err != nil {
				return translateError(err)
			}
			return repository.ErrInvalidInput
		}

		if err := tx.Model(&model.Subscription{}).
			Where("user_id = ? AND status = ?", subscription.UserID, "active").
			Updates(map[string]interface{}{
				"status":                "expired",
				"auto_renew":            false,
				"cancel_at_period_end":  false,
				"current_period_end_at": paidAt,
			}).Error; err != nil {
			return translateError(err)
		}

		record := model.Subscription{
			UserID:               subscription.UserID,
			PlanID:               subscription.PlanID,
			PriceID:              subscription.PriceID,
			Status:               subscription.Status,
			StartAt:              subscription.StartAt,
			CurrentPeriodStartAt: subscription.CurrentPeriodStartAt,
			CurrentPeriodEndAt:   subscription.CurrentPeriodEndAt,
			CancelAtPeriodEnd:    subscription.CancelAtPeriodEnd,
			CanceledAt:           subscription.CanceledAt,
			AutoRenew:            subscription.AutoRenew,
		}
		if err := tx.Create(&record).Error; err != nil {
			return translateError(err)
		}

		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":              domainbilling.PaymentStatusPaid,
			"external_payment_id": strings.TrimSpace(externalPaymentID),
			"paid_at":             paidAt,
		}).Error; err != nil {
			return translateError(err)
		}
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return translateError(err)
		}
		result = toDomainPaymentOrder(order)
		activated = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &result, activated, nil
}

// AddUsage 写入账本。
func (r *Repo) AddUsage(ctx context.Context, usage *domainbilling.UsageLedger) error {
	if usage == nil {
		return nil
	}
	record := toModelUsageLedger(usage)
	return r.db.WithContext(ctx).Create(&record).Error
}

// AddUsageAndDebitBalance 写入用量并按实际金额扣减余额。
func (r *Repo) AddUsageAndDebitBalance(ctx context.Context, usage *domainbilling.UsageLedger) error {
	if usage == nil {
		return nil
	}
	return r.AddUsageAndSettleBalance(ctx, usage, nil)
}

// AddUsageAndSettleBalance 写入用量，并结算预扣差额。
func (r *Repo) AddUsageAndSettleBalance(ctx context.Context, usage *domainbilling.UsageLedger, reservation *domainbilling.UsageBalanceReservation) error {
	if usage == nil {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := toModelUsageLedger(usage)
		chargeNanousd := usage.BilledNanousd
		if usage.IsFreeModel || chargeNanousd <= 0 {
			chargeNanousd = 0
		}
		reservedNanousd := int64(0)
		if reservation != nil {
			reservedNanousd = reservation.AmountNanousd
			if reservedNanousd < 0 {
				return repository.ErrInvalidInput
			}
		}
		deltaNanousd := chargeNanousd - reservedNanousd
		needsBalanceChange := deltaNanousd != 0

		var account *model.BillingAccount
		if needsBalanceChange {
			var err error
			account, err = getOrCreateBillingAccountForUpdate(tx, usage.UserID)
			if err != nil {
				return err
			}
			if deltaNanousd > 0 && account.BalanceNanousd < deltaNanousd {
				return repository.ErrInsufficientBalance
			}
		}

		if err := tx.Create(&record).Error; err != nil {
			return translateError(err)
		}
		if !needsBalanceChange {
			return nil
		}

		nextBalance := account.BalanceNanousd - deltaNanousd
		if err := tx.Model(account).Updates(map[string]interface{}{
			"balance_nanousd": nextBalance,
			"currency":        "USD",
			"status":          "active",
		}).Error; err != nil {
			return translateError(err)
		}
		transactionType := domainbilling.BalanceTransactionTypeUsage
		description := "按量模型用量扣费"
		if deltaNanousd < 0 {
			transactionType = domainbilling.BalanceTransactionTypeUsageRefund
			description = "按量模型预扣差额退回"
		}
		transaction := model.BalanceTransaction{
			AccountID:           account.ID,
			UserID:              usage.UserID,
			Type:                transactionType,
			AmountNanousd:       -deltaNanousd,
			BalanceAfterNanousd: nextBalance,
			RefType:             "usage_ledger",
			RefID:               record.ID,
			RefNo:               reservationRefNo(reservation),
			Description:         description,
		}
		return translateError(tx.Create(&transaction).Error)
	})
}

// ReserveUsageBalance 在真实调用前预扣固定金额，避免并发请求透支余额。
func (r *Repo) ReserveUsageBalance(ctx context.Context, userID uint, amountNanousd int64, refNo string) (*domainbilling.UsageBalanceReservation, error) {
	refNo = strings.TrimSpace(refNo)
	if userID == 0 || amountNanousd < 0 || refNo == "" {
		return nil, repository.ErrInvalidInput
	}
	if amountNanousd == 0 {
		return nil, nil
	}
	var result *domainbilling.UsageBalanceReservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateBillingAccountForUpdate(tx, userID)
		if err != nil {
			return err
		}
		var existing model.BalanceTransaction
		err = tx.Where("user_id = ? AND type = ? AND ref_no = ?", userID, domainbilling.BalanceTransactionTypeUsageReserve, refNo).
			First(&existing).Error
		if err == nil {
			result = &domainbilling.UsageBalanceReservation{
				UserID:        userID,
				AmountNanousd: -existing.AmountNanousd,
				RefNo:         refNo,
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return translateError(err)
		}
		if account.BalanceNanousd < amountNanousd {
			return repository.ErrInsufficientBalance
		}
		nextBalance := account.BalanceNanousd - amountNanousd
		if err = tx.Model(account).Updates(map[string]interface{}{
			"balance_nanousd": nextBalance,
			"currency":        "USD",
			"status":          "active",
		}).Error; err != nil {
			return translateError(err)
		}
		transaction := model.BalanceTransaction{
			AccountID:           account.ID,
			UserID:              userID,
			Type:                domainbilling.BalanceTransactionTypeUsageReserve,
			AmountNanousd:       -amountNanousd,
			BalanceAfterNanousd: nextBalance,
			RefType:             "usage_reservation",
			RefNo:               refNo,
			Description:         "按量模型调用预扣",
		}
		if err = tx.Create(&transaction).Error; err != nil {
			return translateError(err)
		}
		result = &domainbilling.UsageBalanceReservation{
			UserID:        userID,
			AmountNanousd: amountNanousd,
			RefNo:         refNo,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ReleaseUsageBalanceReservation 在调用失败时退回预扣金额，重复调用保持幂等。
func (r *Repo) ReleaseUsageBalanceReservation(ctx context.Context, userID uint, refNo string, description string) error {
	refNo = strings.TrimSpace(refNo)
	if userID == 0 || refNo == "" {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateBillingAccountForUpdate(tx, userID)
		if err != nil {
			return err
		}
		var reserve model.BalanceTransaction
		if err = tx.Where("user_id = ? AND type = ? AND ref_no = ?", userID, domainbilling.BalanceTransactionTypeUsageReserve, refNo).
			First(&reserve).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return translateError(err)
		}
		var existingRefund model.BalanceTransaction
		err = tx.Where("user_id = ? AND type = ? AND ref_no = ?", userID, domainbilling.BalanceTransactionTypeUsageRefund, refNo).
			First(&existingRefund).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return translateError(err)
		}
		refundNanousd := -reserve.AmountNanousd
		if refundNanousd <= 0 {
			return nil
		}
		nextBalance := account.BalanceNanousd + refundNanousd
		if err = tx.Model(account).Updates(map[string]interface{}{
			"balance_nanousd": nextBalance,
			"currency":        "USD",
			"status":          "active",
		}).Error; err != nil {
			return translateError(err)
		}
		transaction := model.BalanceTransaction{
			AccountID:           account.ID,
			UserID:              userID,
			Type:                domainbilling.BalanceTransactionTypeUsageRefund,
			AmountNanousd:       refundNanousd,
			BalanceAfterNanousd: nextBalance,
			RefType:             "usage_reservation",
			RefID:               reserve.ID,
			RefNo:               refNo,
			Description:         firstNonEmpty(strings.TrimSpace(description), "按量模型调用失败退回预扣"),
		}
		return translateError(tx.Create(&transaction).Error)
	})
}

func reservationRefNo(reservation *domainbilling.UsageBalanceReservation) string {
	if reservation == nil {
		return ""
	}
	return strings.TrimSpace(reservation.RefNo)
}

// GetOrCreateBillingAccount 查询或创建用户按量余额账户。
func (r *Repo) GetOrCreateBillingAccount(ctx context.Context, userID uint) (*domainbilling.BillingAccount, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	var result *domainbilling.BillingAccount
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateBillingAccountForUpdate(tx, userID)
		if err != nil {
			return err
		}
		domain := toDomainBillingAccount(*account)
		result = &domain
		return nil
	})
	return result, err
}

// ListBillingAccountsByUserIDs 批量查询用户按量余额账户。
func (r *Repo) ListBillingAccountsByUserIDs(ctx context.Context, userIDs []uint) ([]domainbilling.BillingAccount, error) {
	if len(userIDs) == 0 {
		return []domainbilling.BillingAccount{}, nil
	}
	items := make([]model.BillingAccount, 0, len(userIDs))
	if err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainbilling.BillingAccount, 0, len(items))
	for _, item := range items {
		results = append(results, toDomainBillingAccount(item))
	}
	return results, nil
}

// SetBillingAccountBalance 设置用户按量余额并记录余额流水。
func (r *Repo) SetBillingAccountBalance(ctx context.Context, userID uint, balanceNanousd int64, refNo string, description string) (*domainbilling.BillingAccount, error) {
	if userID == 0 || balanceNanousd < 0 {
		return nil, repository.ErrInvalidInput
	}
	var result domainbilling.BillingAccount
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := getOrCreateBillingAccountForUpdate(tx, userID)
		if err != nil {
			return err
		}
		amount := balanceNanousd - account.BalanceNanousd
		if err := tx.Model(account).Updates(map[string]interface{}{
			"balance_nanousd": balanceNanousd,
			"currency":        "USD",
			"status":          "active",
		}).Error; err != nil {
			return translateError(err)
		}
		if amount != 0 {
			transaction := model.BalanceTransaction{
				AccountID:           account.ID,
				UserID:              userID,
				Type:                domainbilling.BalanceTransactionTypeAdminSet,
				AmountNanousd:       amount,
				BalanceAfterNanousd: balanceNanousd,
				RefType:             "admin",
				RefNo:               strings.TrimSpace(refNo),
				Description:         firstNonEmpty(strings.TrimSpace(description), "管理员设置余额"),
			}
			if err := tx.Create(&transaction).Error; err != nil {
				return translateError(err)
			}
		}
		if err := tx.Where("id = ?", account.ID).First(account).Error; err != nil {
			return translateError(err)
		}
		result = toDomainBillingAccount(*account)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// MarkPaymentOrderPaidAndCreditBalance 标记充值支付成功并入账余额，重复回调保持幂等。
func (r *Repo) MarkPaymentOrderPaidAndCreditBalance(
	ctx context.Context,
	orderNo string,
	externalPaymentID string,
	paidAt time.Time,
) (*domainbilling.PaymentOrder, bool, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, false, repository.ErrInvalidInput
	}
	if paidAt.IsZero() {
		paidAt = time.Now()
	}

	var result domainbilling.PaymentOrder
	credited := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.PaymentOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return translateError(err)
		}
		if order.Status == domainbilling.PaymentStatusPaid {
			result = toDomainPaymentOrder(order)
			return nil
		}
		if order.Status != domainbilling.PaymentStatusPending || order.OrderType != domainbilling.PaymentOrderTypeTopUp || order.CreditNanousd <= 0 {
			return repository.ErrInvalidInput
		}
		if order.ExpiredAt != nil && order.ExpiredAt.Before(paidAt) {
			if err := tx.Model(&order).Updates(map[string]interface{}{
				"status": domainbilling.PaymentStatusExpired,
			}).Error; err != nil {
				return translateError(err)
			}
			return repository.ErrInvalidInput
		}

		account, err := getOrCreateBillingAccountForUpdate(tx, order.UserID)
		if err != nil {
			return err
		}
		nextBalance := account.BalanceNanousd + order.CreditNanousd
		if err := tx.Model(account).Updates(map[string]interface{}{
			"balance_nanousd": nextBalance,
			"currency":        "USD",
			"status":          "active",
		}).Error; err != nil {
			return translateError(err)
		}
		transaction := model.BalanceTransaction{
			AccountID:           account.ID,
			UserID:              order.UserID,
			Type:                domainbilling.BalanceTransactionTypeTopUp,
			AmountNanousd:       order.CreditNanousd,
			BalanceAfterNanousd: nextBalance,
			RefType:             "payment_order",
			RefID:               order.ID,
			RefNo:               order.OrderNo,
			Description:         "按量余额充值",
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return translateError(err)
		}
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":              domainbilling.PaymentStatusPaid,
			"external_payment_id": strings.TrimSpace(externalPaymentID),
			"paid_at":             paidAt,
		}).Error; err != nil {
			return translateError(err)
		}
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return translateError(err)
		}
		result = toDomainPaymentOrder(order)
		credited = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &result, credited, nil
}

// GetBillingMode 查询当前计费方式。
func (r *Repo) GetBillingMode(ctx context.Context) (string, error) {
	var item model.SystemSetting
	if err := r.db.WithContext(ctx).
		Where("namespace = ? AND key = ?", "billing", "mode").
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "self", nil
		}
		return "", translateError(err)
	}
	mode := strings.TrimSpace(item.Value)
	switch mode {
	case "self", "usage", "period":
		return mode, nil
	default:
		return "self", nil
	}
}

// GetBillingPrepaidAmountNanousd 查询按量调用前要求保留的最低预付余额。
func (r *Repo) GetBillingPrepaidAmountNanousd(ctx context.Context) (int64, error) {
	var item model.SystemSetting
	if err := r.db.WithContext(ctx).
		Where("namespace = ? AND key = ?", "billing", "prepaid_amount_usd").
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, translateError(err)
	}
	value := strings.TrimSpace(item.Value)
	if value == "" {
		return 0, nil
	}
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil || amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, repository.ErrInvalidInput
	}
	return int64(math.Round(amount * 1000000000)), nil
}

// GetModelPricing 查询模型计费配置。
func (r *Repo) GetModelPricing(ctx context.Context, platformModelName string) (*domainbilling.ModelPricing, error) {
	var item model.ModelPricing
	if err := r.db.WithContext(ctx).
		Where("platform_model_name = ?", strings.TrimSpace(platformModelName)).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomainModelPricing(item)
	return &result, nil
}

// ListModelPricing 分页查询模型单价。
func (r *Repo) ListModelPricing(ctx context.Context, query string, offset int, limit int) ([]domainbilling.ModelPricing, int64, error) {
	items := make([]model.ModelPricing, 0)
	var total int64

	dbq := r.db.WithContext(ctx).Model(&model.ModelPricing{})
	if keyword := strings.TrimSpace(query); keyword != "" {
		like := "%" + keyword + "%"
		dbq = dbq.Where("platform_model_name ILIKE ?", like)
	}

	if err := dbq.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := dbq.Order("platform_model_name ASC, id ASC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}

	results := make([]domainbilling.ModelPricing, 0, len(items))
	for _, item := range items {
		results = append(results, toDomainModelPricing(item))
	}
	return results, total, nil
}

// UpsertModelPricing 按平台模型名保存模型单价。
func (r *Repo) UpsertModelPricing(ctx context.Context, item *domainbilling.ModelPricing) (*domainbilling.ModelPricing, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	platformModelName := strings.TrimSpace(item.PlatformModelName)
	if platformModelName == "" {
		return nil, repository.ErrInvalidInput
	}

	var record model.ModelPricing
	err := r.db.WithContext(ctx).Where("platform_model_name = ?", platformModelName).First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, translateError(err)
	}

	updates := map[string]interface{}{
		"platform_model_name":              platformModelName,
		"currency":                         normalizeCurrency(item.Currency),
		"is_free":                          item.IsFree,
		"pricing_mode":                     normalizePricingMode(item.PricingMode),
		"input_nanousd_per_m_tokens":       clampNonNegative(item.InputNanousdPerMTokens),
		"cache_read_nanousd_per_m_tokens":  clampNonNegative(item.CacheReadNanousdPerMTokens),
		"cache_write_nanousd_per_m_tokens": clampNonNegative(item.CacheWriteNanousdPerMTokens),
		"output_nanousd_per_m_tokens":      clampNonNegative(item.OutputNanousdPerMTokens),
		"call_nanousd_per_call":            clampNonNegative(item.CallNanousdPerCall),
		"duration_nanousd_per_second":      clampNonNegative(item.DurationNanousdPerSecond),
		"tiered_pricing_json":              strings.TrimSpace(item.TieredPricingJSON),
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = model.ModelPricing{
			PlatformModelName: platformModelName,
		}
		if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, translateError(err)
		}
	}
	if err := r.db.WithContext(ctx).Model(&record).Updates(updates).Error; err != nil {
		return nil, translateError(err)
	}
	if err := r.db.WithContext(ctx).Where("platform_model_name = ?", platformModelName).First(&record).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomainModelPricing(record)
	return &result, nil
}

// ListUsageByUser 分页查询账本。
func (r *Repo) ListUsageByUser(ctx context.Context, userID uint, filter repository.UsageListFilter, offset int, limit int) ([]domainbilling.UsageLedger, int64, error) {
	items := make([]model.UsageLedger, 0)
	var total int64
	query := r.db.WithContext(ctx).Model(&model.UsageLedger{}).Where("user_id = ?", userID)
	if search := strings.TrimSpace(filter.Query); search != "" {
		like := "%" + search + "%"
		query = query.Where("platform_model_name ILIKE ?", like)
	}
	switch strings.TrimSpace(filter.Status) {
	case "free":
		query = query.Where("is_free_model = ?", true)
	case "billable":
		query = query.Where("is_free_model = ?", false)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	order := "usage_date DESC, id DESC"
	switch strings.TrimSpace(filter.Sort) {
	case "oldest":
		order = "usage_date ASC, id ASC"
	case "tokens_desc":
		order = "(input_tokens + cache_read_tokens + cache_write_tokens + output_tokens + reasoning_tokens) DESC, id DESC"
	case "cost_desc":
		order = "billed_nanousd DESC, id DESC"
	case "latency_desc":
		order = "latency_ms DESC, id DESC"
	}
	if err := query.
		Order(order).
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	results := make([]domainbilling.UsageLedger, 0, len(items))
	for _, item := range items {
		results = append(results, toDomainUsageLedger(item))
	}
	return results, total, nil
}

// ListUsageLogs 分页查询管理员调用日志。
func (r *Repo) ListUsageLogs(ctx context.Context, filter repository.UsageLogListFilter, offset int, limit int) ([]domainbilling.UsageLedger, int64, error) {
	items := make([]model.UsageLedger, 0)
	var total int64
	query := r.db.WithContext(ctx).Model(&model.UsageLedger{})
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"platform_model_name ILIKE ? OR upstream_model_name ILIKE ? OR upstream_name ILIKE ? OR routed_binding_code ILIKE ? OR provider_protocol ILIKE ?",
			like,
			like,
			like,
			like,
			like,
		)
	}
	if platformModelName := strings.TrimSpace(filter.PlatformModelName); platformModelName != "" {
		query = query.Where("platform_model_name = ?", platformModelName)
	}
	switch strings.TrimSpace(filter.BillingMode) {
	case "free":
		query = query.Where("is_free_model = ?", true)
	case "token", "call", "duration", "tiered":
		query = query.Where("is_free_model = ?", false)
		query = query.Where("COALESCE(NULLIF(pricing_snapshot_json, '')::jsonb ->> 'pricing_mode', 'token') = ?", strings.TrimSpace(filter.BillingMode))
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	order := "created_at DESC, id DESC"
	switch strings.TrimSpace(filter.Sort) {
	case "created_asc":
		order = "created_at ASC, id ASC"
	case "tokens_desc":
		order = "(input_tokens + cache_read_tokens + cache_write_tokens + output_tokens + reasoning_tokens) DESC, id DESC"
	case "cost_desc":
		order = "billed_nanousd DESC, id DESC"
	case "latency_desc":
		order = "latency_ms DESC, id DESC"
	}
	if err := query.
		Order(order).
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	results := make([]domainbilling.UsageLedger, 0, len(items))
	for _, item := range items {
		results = append(results, toDomainUsageLedger(item))
	}
	return results, total, nil
}

// ListMonthlyUsageByUser 按月份聚合用户用量。
func (r *Repo) ListMonthlyUsageByUser(ctx context.Context, userID uint, limit int) ([]domainbilling.UsageMonthlySummary, error) {
	if limit <= 0 {
		limit = 6
	}
	if limit > 24 {
		limit = 24
	}
	now := time.Now()
	endMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
	startMonth := endMonth.AddDate(0, -limit, 0)

	type monthlyUsageRow struct {
		MonthStartAt     time.Time `gorm:"column:month_start_at"`
		RecordCount      int64     `gorm:"column:record_count"`
		InputTokens      int64     `gorm:"column:input_tokens"`
		CacheReadTokens  int64     `gorm:"column:cache_read_tokens"`
		CacheWriteTokens int64     `gorm:"column:cache_write_tokens"`
		OutputTokens     int64     `gorm:"column:output_tokens"`
		ReasoningTokens  int64     `gorm:"column:reasoning_tokens"`
		CallCount        int64     `gorm:"column:call_count"`
		DurationSeconds  int64     `gorm:"column:duration_seconds"`
		AvgLatencyMS     int64     `gorm:"column:avg_latency_ms"`
		BilledNanousd    int64     `gorm:"column:billed_nanousd"`
	}

	rows := make([]monthlyUsageRow, 0, limit)
	if err := r.db.WithContext(ctx).
		Model(&model.UsageLedger{}).
		Select(`
			date_trunc('month', usage_date)::date AS month_start_at,
			COUNT(*) AS record_count,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(reasoning_tokens), 0) AS reasoning_tokens,
			COALESCE(SUM(call_count), 0) AS call_count,
			COALESCE(SUM(duration_seconds), 0) AS duration_seconds,
			COALESCE(ROUND(AVG(NULLIF(latency_ms, 0))), 0) AS avg_latency_ms,
			COALESCE(SUM(billed_nanousd), 0) AS billed_nanousd
		`).
		Where("user_id = ? AND usage_date >= ? AND usage_date < ?", userID, startMonth, endMonth).
		Group("month_start_at").
		Order("month_start_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}

	results := make([]domainbilling.UsageMonthlySummary, 0, len(rows))
	for _, row := range rows {
		results = append(results, domainbilling.UsageMonthlySummary{
			MonthStartAt:     row.MonthStartAt,
			RecordCount:      row.RecordCount,
			InputTokens:      row.InputTokens,
			CacheReadTokens:  row.CacheReadTokens,
			CacheWriteTokens: row.CacheWriteTokens,
			OutputTokens:     row.OutputTokens,
			ReasoningTokens:  row.ReasoningTokens,
			CallCount:        row.CallCount,
			DurationSeconds:  row.DurationSeconds,
			AvgLatencyMS:     row.AvgLatencyMS,
			BilledNanousd:    row.BilledNanousd,
		})
	}
	return results, nil
}

// GetUserCreatedAt 查询用户注册时间。
func (r *Repo) GetUserCreatedAt(ctx context.Context, userID uint) (time.Time, error) {
	var item model.User
	if err := r.db.WithContext(ctx).
		Select("created_at").
		Where("id = ?", userID).
		First(&item).Error; err != nil {
		return time.Time{}, translateError(err)
	}
	return item.CreatedAt, nil
}

// ListDailyUsageByUser 按日期聚合用户用量。
func (r *Repo) ListDailyUsageByUser(ctx context.Context, userID uint, startDate time.Time, endDate time.Time) ([]domainbilling.UsageDailySummary, error) {
	type dailyModelUsageRow struct {
		UsageDate           time.Time `gorm:"column:usage_date"`
		PlatformModelName   string    `gorm:"column:platform_model_name"`
		PricingSnapshotJSON string    `gorm:"column:pricing_snapshot_json"`
		RecordCount         int64     `gorm:"column:record_count"`
		InputTokens         int64     `gorm:"column:input_tokens"`
		CacheReadTokens     int64     `gorm:"column:cache_read_tokens"`
		CacheWriteTokens    int64     `gorm:"column:cache_write_tokens"`
		OutputTokens        int64     `gorm:"column:output_tokens"`
		ReasoningTokens     int64     `gorm:"column:reasoning_tokens"`
		CallCount           int64     `gorm:"column:call_count"`
		DurationSeconds     int64     `gorm:"column:duration_seconds"`
		AvgLatencyMS        int64     `gorm:"column:avg_latency_ms"`
		LatencyCount        int64     `gorm:"column:latency_count"`
		BilledNanousd       int64     `gorm:"column:billed_nanousd"`
	}

	modelRows := make([]dailyModelUsageRow, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.UsageLedger{}).
		Select(`
			date_trunc('day', usage_date)::date AS usage_date,
			platform_model_name,
			(ARRAY_AGG(NULLIF(pricing_snapshot_json, '') ORDER BY created_at DESC, id DESC)
				FILTER (WHERE NULLIF(pricing_snapshot_json, '') IS NOT NULL))[1] AS pricing_snapshot_json,
			COUNT(*) AS record_count,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(reasoning_tokens), 0) AS reasoning_tokens,
			COALESCE(SUM(call_count), 0) AS call_count,
			COALESCE(SUM(duration_seconds), 0) AS duration_seconds,
			COALESCE(ROUND(AVG(NULLIF(latency_ms, 0))), 0) AS avg_latency_ms,
			COUNT(NULLIF(latency_ms, 0)) AS latency_count,
			COALESCE(SUM(billed_nanousd), 0) AS billed_nanousd
		`).
		Where("user_id = ? AND usage_date >= ? AND usage_date < ?", userID, startDate, endDate).
		Group("date_trunc('day', usage_date)::date, platform_model_name").
		Order("usage_date ASC, billed_nanousd DESC, platform_model_name ASC").
		Scan(&modelRows).Error; err != nil {
		return nil, translateError(err)
	}

	resultsByDate := make(map[string]domainbilling.UsageDailySummary)
	dateKeys := make([]string, 0)
	latencyCountsByDate := make(map[string]int64)
	modelsByDate := make(map[string][]domainbilling.UsageDailyModelSummary)
	for _, row := range modelRows {
		key := row.UsageDate.Format("2006-01-02")
		summary, exists := resultsByDate[key]
		if !exists {
			summary = domainbilling.UsageDailySummary{UsageDate: row.UsageDate}
			dateKeys = append(dateKeys, key)
		}
		summary.RecordCount += row.RecordCount
		summary.InputTokens += row.InputTokens
		summary.CacheReadTokens += row.CacheReadTokens
		summary.CacheWriteTokens += row.CacheWriteTokens
		summary.OutputTokens += row.OutputTokens
		summary.ReasoningTokens += row.ReasoningTokens
		summary.CallCount += row.CallCount
		summary.DurationSeconds += row.DurationSeconds
		summary.BilledNanousd += row.BilledNanousd
		if row.LatencyCount > 0 {
			currentLatencyCount := latencyCountsByDate[key]
			summary.AvgLatencyMS = weightedAverageLatency(summary.AvgLatencyMS, currentLatencyCount, row.AvgLatencyMS, row.LatencyCount)
			latencyCountsByDate[key] = currentLatencyCount + row.LatencyCount
		}
		resultsByDate[key] = summary
		modelsByDate[key] = append(modelsByDate[key], domainbilling.UsageDailyModelSummary{
			PlatformModelName: row.PlatformModelName,
			RecordCount:       row.RecordCount,
			InputTokens:       row.InputTokens,
			CacheReadTokens:   row.CacheReadTokens,
			CacheWriteTokens:  row.CacheWriteTokens,
			OutputTokens:      row.OutputTokens,
			ReasoningTokens:   row.ReasoningTokens,
			CallCount:         row.CallCount,
			DurationSeconds:   row.DurationSeconds,
			AvgLatencyMS:      row.AvgLatencyMS,
			BilledNanousd:     row.BilledNanousd,
		})
	}

	results := make([]domainbilling.UsageDailySummary, 0, len(dateKeys))
	for _, key := range dateKeys {
		summary := resultsByDate[key]
		summary.Models = modelsByDate[key]
		results = append(results, summary)
	}
	return results, nil
}

func weightedAverageLatency(currentAvg int64, currentCount int64, nextAvg int64, nextCount int64) int64 {
	totalCount := currentCount + nextCount
	if totalCount <= 0 {
		return 0
	}
	return ((currentAvg * currentCount) + (nextAvg * nextCount)) / totalCount
}

// SumBillableNanousd 统计周期内付费模型的用量金额。
func (r *Repo) SumBillableNanousd(ctx context.Context, userID uint, startAt time.Time, endAt time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.UsageLedger{}).
		Select("COALESCE(SUM(billed_nanousd), 0)").
		Where("user_id = ? AND is_free_model = ? AND created_at >= ? AND created_at < ?", userID, false, startAt, endAt).
		Scan(&total).Error
	if err != nil {
		return 0, translateError(err)
	}
	return total, nil
}

func toDomainModelPricing(item model.ModelPricing) domainbilling.ModelPricing {
	return domainbilling.ModelPricing{
		ID:                          item.ID,
		PlatformModelName:           item.PlatformModelName,
		Currency:                    item.Currency,
		IsFree:                      item.IsFree,
		PricingMode:                 normalizePricingMode(item.PricingMode),
		InputNanousdPerMTokens:      item.InputNanousdPerMTokens,
		CacheReadNanousdPerMTokens:  item.CacheReadNanousdPerMTokens,
		CacheWriteNanousdPerMTokens: item.CacheWriteNanousdPerMTokens,
		OutputNanousdPerMTokens:     item.OutputNanousdPerMTokens,
		CallNanousdPerCall:          item.CallNanousdPerCall,
		DurationNanousdPerSecond:    item.DurationNanousdPerSecond,
		TieredPricingJSON:           item.TieredPricingJSON,
		CreatedAt:                   item.CreatedAt,
		UpdatedAt:                   item.UpdatedAt,
	}
}

func toDomainPaymentOrder(item model.PaymentOrder) domainbilling.PaymentOrder {
	return domainbilling.PaymentOrder{
		ID:                 item.ID,
		OrderNo:            item.OrderNo,
		OrderType:          item.OrderType,
		UserID:             item.UserID,
		PlanID:             item.PlanID,
		PriceID:            item.PriceID,
		Provider:           item.Provider,
		Status:             item.Status,
		BaseCurrency:       item.BaseCurrency,
		BaseAmountCents:    item.BaseAmountCents,
		PayCurrency:        item.PayCurrency,
		PayAmountCents:     item.PayAmountCents,
		FXRate:             item.FXRate,
		CreditNanousd:      item.CreditNanousd,
		BillingInterval:    item.BillingInterval,
		Cycles:             item.Cycles,
		ExternalPaymentID:  item.ExternalPaymentID,
		ExternalCheckoutID: item.ExternalCheckoutID,
		CheckoutURL:        item.CheckoutURL,
		PaidAt:             item.PaidAt,
		ExpiredAt:          item.ExpiredAt,
		SnapshotJSON:       item.SnapshotJSON,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func toModelUsageLedger(usage *domainbilling.UsageLedger) model.UsageLedger {
	return model.UsageLedger{
		UserID:              usage.UserID,
		ConversationID:      usage.ConversationID,
		ProviderProtocol:    usage.ProviderProtocol,
		UpstreamName:        usage.UpstreamName,
		PlatformModelName:   usage.PlatformModelName,
		RoutedBindingCode:   usage.RoutedBindingCode,
		UpstreamModelName:   usage.UpstreamModelName,
		IsFreeModel:         usage.IsFreeModel,
		UsageDate:           usage.UsageDate,
		InputTokens:         usage.InputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheWriteTokens:    usage.CacheWriteTokens,
		CacheWrite5mTokens:  usage.CacheWrite5mTokens,
		CacheWrite1hTokens:  usage.CacheWrite1hTokens,
		OutputTokens:        usage.OutputTokens,
		ReasoningTokens:     usage.ReasoningTokens,
		CallCount:           usage.CallCount,
		DurationSeconds:     usage.DurationSeconds,
		LatencyMS:           usage.LatencyMS,
		UsageSpeed:          usage.UsageSpeed,
		ServiceTier:         usage.ServiceTier,
		BilledCurrency:      usage.BilledCurrency,
		BilledNanousd:       usage.BilledNanousd,
		PricingSnapshotJSON: usage.PricingSnapshotJSON,
	}
}

func toDomainUsageLedger(item model.UsageLedger) domainbilling.UsageLedger {
	return domainbilling.UsageLedger{
		ID:                  item.ID,
		UserID:              item.UserID,
		ConversationID:      item.ConversationID,
		ProviderProtocol:    item.ProviderProtocol,
		UpstreamName:        item.UpstreamName,
		PlatformModelName:   item.PlatformModelName,
		RoutedBindingCode:   item.RoutedBindingCode,
		UpstreamModelName:   item.UpstreamModelName,
		IsFreeModel:         item.IsFreeModel,
		UsageDate:           item.UsageDate,
		InputTokens:         item.InputTokens,
		CacheReadTokens:     item.CacheReadTokens,
		CacheWriteTokens:    item.CacheWriteTokens,
		CacheWrite5mTokens:  item.CacheWrite5mTokens,
		CacheWrite1hTokens:  item.CacheWrite1hTokens,
		OutputTokens:        item.OutputTokens,
		ReasoningTokens:     item.ReasoningTokens,
		CallCount:           item.CallCount,
		DurationSeconds:     item.DurationSeconds,
		LatencyMS:           item.LatencyMS,
		UsageSpeed:          item.UsageSpeed,
		ServiceTier:         item.ServiceTier,
		BilledCurrency:      item.BilledCurrency,
		BilledNanousd:       item.BilledNanousd,
		PricingSnapshotJSON: item.PricingSnapshotJSON,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func getOrCreateBillingAccountForUpdate(tx *gorm.DB, userID uint) (*model.BillingAccount, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	var account model.BillingAccount
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&account).Error
	if err == nil {
		return &account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, translateError(err)
	}
	account = model.BillingAccount{
		UserID:         userID,
		Currency:       "USD",
		BalanceNanousd: 0,
		Status:         "active",
	}
	if err := tx.Create(&account).Error; err != nil {
		return nil, translateError(err)
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", account.ID).First(&account).Error; err != nil {
		return nil, translateError(err)
	}
	return &account, nil
}

func toDomainBillingAccount(item model.BillingAccount) domainbilling.BillingAccount {
	return domainbilling.BillingAccount{
		ID:             item.ID,
		UserID:         item.UserID,
		Currency:       item.Currency,
		BalanceNanousd: item.BalanceNanousd,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func normalizeOrderType(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.PaymentOrderTypeTopUp:
		return domainbilling.PaymentOrderTypeTopUp
	default:
		return domainbilling.PaymentOrderTypeSubscription
	}
}

func normalizeCurrency(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "USD"
	}
	return normalized
}

func normalizePricingMode(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.PricingModeCall:
		return domainbilling.PricingModeCall
	case domainbilling.PricingModeDuration:
		return domainbilling.PricingModeDuration
	case domainbilling.PricingModeTiered:
		return domainbilling.PricingModeTiered
	default:
		return domainbilling.PricingModeToken
	}
}

func clampNonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeInterval(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.IntervalYear:
		return domainbilling.IntervalYear
	case domainbilling.IntervalLifetime:
		return domainbilling.IntervalLifetime
	default:
		return domainbilling.IntervalMonth
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
