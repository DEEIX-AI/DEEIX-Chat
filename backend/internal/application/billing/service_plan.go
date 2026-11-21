package billing

import (
	"context"
	"errors"
	"regexp"
	"strings"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

var planCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// PlanPriceInput 定义套餐单个计费周期的价格。
type PlanPriceInput struct {
	BillingInterval string
	AmountCents     int64
	IsDefault       bool
}

// PlanSaveInput 定义周期套餐新建与保存入参；Code 仅在新建时生效，创建后不可修改。
type PlanSaveInput struct {
	Code                string
	Name                string
	Description         string
	PeriodCreditNanousd int64
	SortOrder           int
	IsActive            bool
	PermissionGroupID   *uint
	Prices              []PlanPriceInput
}

// ListAdminPlans 查询全部套餐（含已下架）及其启用价格。
func (s *Service) ListAdminPlans(ctx context.Context) ([]BillingPlanView, error) {
	plans, err := s.repo.ListAllPlans(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildPlanViews(ctx, plans)
}

// CreatePlan 新建周期套餐及其价格。
func (s *Service) CreatePlan(ctx context.Context, input PlanSaveInput) (*BillingPlanView, error) {
	code := strings.ToLower(strings.TrimSpace(input.Code))
	if !planCodePattern.MatchString(code) {
		return nil, ErrInvalidBillingPlan
	}
	plan := &domainbilling.Plan{Code: code, FeatureJSON: "{}"}
	return s.savePlan(ctx, plan, input)
}

// UpdatePlan 保存周期套餐信息与价格。
func (s *Service) UpdatePlan(ctx context.Context, planID uint, input PlanSaveInput) (*BillingPlanView, error) {
	if planID == 0 {
		return nil, ErrInvalidBillingPlan
	}
	current, err := s.repo.GetPlanByID(ctx, planID)
	if err != nil {
		return nil, mapPlanMutationError(err)
	}
	return s.savePlan(ctx, current, input)
}

// DeletePlan 删除未被任何订阅、订单或兑换码引用的套餐。
func (s *Service) DeletePlan(ctx context.Context, planID uint) error {
	if planID == 0 {
		return ErrInvalidBillingPlan
	}
	current, err := s.repo.GetPlanByID(ctx, planID)
	if err != nil {
		return mapPlanMutationError(err)
	}
	if isFreePlanCode(current.Code) {
		return ErrBillingPlanProtected
	}
	if err := s.repo.DeletePlan(ctx, planID); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return ErrBillingPlanInUse
		}
		return mapPlanMutationError(err)
	}
	return nil
}

func (s *Service) savePlan(ctx context.Context, plan *domainbilling.Plan, input PlanSaveInput) (*BillingPlanView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrInvalidBillingPlan
	}
	if isFreePlanCode(plan.Code) && !input.IsActive {
		return nil, ErrBillingPlanProtected
	}
	prices, err := normalizePlanPrices(input.Prices)
	if err != nil {
		return nil, err
	}
	permissionGroupID, err := s.resolvePlanPermissionGroupID(ctx, input.PermissionGroupID)
	if err != nil {
		return nil, err
	}
	if err := s.validatePermissionGroupID(ctx, permissionGroupID); err != nil {
		return nil, err
	}

	plan.Name = name
	plan.Description = strings.TrimSpace(input.Description)
	plan.PeriodCreditNanousd = clampNonNegative(input.PeriodCreditNanousd)
	plan.SortOrder = max(input.SortOrder, 0)
	plan.IsActive = input.IsActive
	plan.PermissionGroupID = permissionGroupID

	if err := s.repo.SavePlanWithPrices(ctx, plan, prices); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrBillingPlanCodeConflict
		}
		return nil, mapPlanMutationError(err)
	}
	views, err := s.buildPlanViews(ctx, []domainbilling.Plan{*plan})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// normalizePlanPrices 要求每个计费周期至多一个价格且恰好一个默认价格；未指定默认时取第一个。
func normalizePlanPrices(inputs []PlanPriceInput) ([]domainbilling.Price, error) {
	if len(inputs) == 0 {
		return nil, ErrInvalidBillingPlan
	}
	seen := make(map[string]struct{}, len(inputs))
	prices := make([]domainbilling.Price, 0, len(inputs))
	defaultCount := 0
	for _, input := range inputs {
		interval := strings.TrimSpace(input.BillingInterval)
		if interval != domainbilling.IntervalMonth && interval != domainbilling.IntervalYear && interval != domainbilling.IntervalLifetime {
			return nil, ErrInvalidBillingPlan
		}
		if _, ok := seen[interval]; ok {
			return nil, ErrInvalidBillingPlan
		}
		if input.AmountCents < 0 {
			return nil, ErrInvalidBillingPlan
		}
		seen[interval] = struct{}{}
		if input.IsDefault {
			defaultCount++
		}
		prices = append(prices, domainbilling.Price{
			BillingInterval: interval,
			Currency:        "USD",
			AmountCents:     input.AmountCents,
			IsActive:        true,
			IsDefault:       input.IsDefault,
		})
	}
	switch defaultCount {
	case 0:
		prices[0].IsDefault = true
	case 1:
	default:
		return nil, ErrInvalidBillingPlan
	}
	return prices, nil
}
