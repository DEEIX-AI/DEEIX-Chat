package billing

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// ListAllPlans 查询全部套餐（含已下架），供后台管理使用。
func (r *Repo) ListAllPlans(ctx context.Context) ([]domainbilling.Plan, error) {
	items := make([]models.BillingPlan, 0)
	if err := r.db.WithContext(ctx).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	results := make([]domainbilling.Plan, 0, len(items))
	for _, item := range items {
		results = append(results, toPlanDomain(item))
	}
	return results, nil
}

// SavePlanWithPrices 新建或更新套餐，并按计费周期同步价格。
func (r *Repo) SavePlanWithPrices(ctx context.Context, plan *domainbilling.Plan, prices []domainbilling.Price) error {
	if plan == nil {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, err := upsertPlanRecord(tx, plan)
		if err != nil {
			return err
		}
		if err := syncPlanPrices(tx, record, prices); err != nil {
			return err
		}
		*plan = toPlanDomain(*record)
		return nil
	})
}

// DeletePlan 硬删除未被引用的套餐及其价格；硬删除可释放唯一编码供后续复用。
func (r *Repo) DeletePlan(ctx context.Context, planID uint) error {
	if planID == 0 {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var plan models.BillingPlan
		if err := tx.Where("id = ?", planID).First(&plan).Error; err != nil {
			return dberror.Translate(err)
		}
		referenced, err := planHasReferences(tx, planID)
		if err != nil {
			return err
		}
		if referenced {
			return repository.ErrConflict
		}
		if err := tx.Unscoped().Where("plan_id = ?", planID).Delete(&models.BillingPrice{}).Error; err != nil {
			return dberror.Translate(err)
		}
		return dberror.Translate(tx.Unscoped().Delete(&plan).Error)
	})
}

func upsertPlanRecord(tx *gorm.DB, plan *domainbilling.Plan) (*models.BillingPlan, error) {
	fields := map[string]any{
		"name":                  strings.TrimSpace(plan.Name),
		"description":           strings.TrimSpace(plan.Description),
		"period_credit_nanousd": clampNonNegative(plan.PeriodCreditNanousd),
		"sort_order":            plan.SortOrder,
		"is_active":             plan.IsActive,
		"permission_group_id":   plan.PermissionGroupID,
	}
	if plan.ID == 0 {
		record := models.BillingPlan{
			Code:                strings.TrimSpace(plan.Code),
			Name:                strings.TrimSpace(plan.Name),
			Description:         strings.TrimSpace(plan.Description),
			FeatureJSON:         plan.FeatureJSON,
			PeriodCreditNanousd: clampNonNegative(plan.PeriodCreditNanousd),
			SortOrder:           plan.SortOrder,
			PermissionGroupID:   plan.PermissionGroupID,
		}
		if err := tx.Create(&record).Error; err != nil {
			return nil, dberror.Translate(err)
		}
		// is_active 带数据库默认值，零值 false 会被 Create 忽略，需显式写回。
		if err := tx.Model(&record).Update("is_active", plan.IsActive).Error; err != nil {
			return nil, dberror.Translate(err)
		}
		record.IsActive = plan.IsActive
		return &record, nil
	}

	var record models.BillingPlan
	if err := tx.Where("id = ?", plan.ID).First(&record).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	if err := tx.Model(&record).Updates(fields).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	if err := tx.Where("id = ?", plan.ID).First(&record).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return &record, nil
}

func syncPlanPrices(tx *gorm.DB, plan *models.BillingPlan, prices []domainbilling.Price) error {
	var existing []models.BillingPrice
	if err := tx.Where("plan_id = ?", plan.ID).Order("is_default DESC, id ASC").Find(&existing).Error; err != nil {
		return dberror.Translate(err)
	}

	keepIDs := make(map[uint]struct{}, len(prices))
	for _, input := range prices {
		interval := domainbilling.NormalizeInterval(input.BillingInterval)
		record := pickPriceForInterval(existing, interval, keepIDs)
		if record == nil {
			record = &models.BillingPrice{
				PlanID:          plan.ID,
				Code:            fmt.Sprintf("%s-%s", plan.Code, interval),
				BillingInterval: interval,
			}
			if err := tx.Create(record).Error; err != nil {
				return dberror.Translate(err)
			}
		}
		keepIDs[record.ID] = struct{}{}
		updates := map[string]any{
			"billing_interval": interval,
			"currency":         normalizeCurrency(input.Currency),
			"amount_cents":     clampNonNegative(input.AmountCents),
			"is_active":        true,
			"is_default":       input.IsDefault,
		}
		if err := tx.Model(record).Updates(updates).Error; err != nil {
			return dberror.Translate(err)
		}
	}

	for _, item := range existing {
		if _, ok := keepIDs[item.ID]; ok {
			continue
		}
		if err := tx.Model(&models.BillingPrice{}).
			Where("id = ?", item.ID).
			Updates(map[string]any{"is_active": false, "is_default": false}).Error; err != nil {
			return dberror.Translate(err)
		}
	}
	return nil
}

// pickPriceForInterval 复用同周期的已有价格行，优先当前启用的行，避免为相同周期反复新增价格编码。
func pickPriceForInterval(existing []models.BillingPrice, interval string, taken map[uint]struct{}) *models.BillingPrice {
	var fallback *models.BillingPrice
	for index := range existing {
		item := &existing[index]
		if _, ok := taken[item.ID]; ok || item.BillingInterval != interval {
			continue
		}
		if item.IsActive {
			return item
		}
		if fallback == nil {
			fallback = item
		}
	}
	return fallback
}

func planHasReferences(tx *gorm.DB, planID uint) (bool, error) {
	for _, model := range []any{
		&models.Subscription{},
		&models.PaymentOrder{},
		&models.RedemptionCode{},
		&models.Redemption{},
	} {
		var count int64
		if err := tx.Unscoped().Model(model).Where("plan_id = ?", planID).Count(&count).Error; err != nil {
			return false, dberror.Translate(err)
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func toPriceDomain(item models.BillingPrice) domainbilling.Price {
	return domainbilling.Price{
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
	}
}
