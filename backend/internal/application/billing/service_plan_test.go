package billing

import (
	"context"
	"errors"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func monthlyPriceInput(amountCents int64) []PlanPriceInput {
	return []PlanPriceInput{{BillingInterval: domainbilling.IntervalMonth, AmountCents: amountCents, IsDefault: true}}
}

func newPlanTestService(repo *billingRepositoryStub) *Service {
	groupID := uint(1)
	service := NewService(repo)
	service.SetPermissionGroupLookup(permissionGroupLookupStub{
		validIDs:   map[uint]struct{}{groupID: {}},
		defaultIDs: []uint{groupID},
	})
	return service
}

func TestNormalizePlanPrices(t *testing.T) {
	tests := []struct {
		name        string
		inputs      []PlanPriceInput
		wantErr     bool
		wantDefault string
	}{
		{name: "empty", inputs: nil, wantErr: true},
		{name: "unknown interval", inputs: []PlanPriceInput{{BillingInterval: "week"}}, wantErr: true},
		{name: "negative amount", inputs: []PlanPriceInput{{BillingInterval: "month", AmountCents: -1}}, wantErr: true},
		{
			name:    "duplicate interval",
			inputs:  []PlanPriceInput{{BillingInterval: "month"}, {BillingInterval: "month"}},
			wantErr: true,
		},
		{
			name:    "multiple defaults",
			inputs:  []PlanPriceInput{{BillingInterval: "month", IsDefault: true}, {BillingInterval: "year", IsDefault: true}},
			wantErr: true,
		},
		{
			name:        "first price becomes default",
			inputs:      []PlanPriceInput{{BillingInterval: "month", AmountCents: 1000}, {BillingInterval: "year", AmountCents: 10000}},
			wantDefault: "month",
		},
		{
			name:        "explicit default kept",
			inputs:      []PlanPriceInput{{BillingInterval: "month"}, {BillingInterval: "year", IsDefault: true}},
			wantDefault: "year",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prices, err := normalizePlanPrices(tt.inputs)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidBillingPlan) {
					t.Fatalf("err = %v, want ErrInvalidBillingPlan", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizePlanPrices() error = %v", err)
			}
			defaults := 0
			for _, item := range prices {
				if item.IsDefault {
					defaults++
					if item.BillingInterval != tt.wantDefault {
						t.Fatalf("default interval = %s, want %s", item.BillingInterval, tt.wantDefault)
					}
				}
			}
			if defaults != 1 {
				t.Fatalf("default count = %d, want 1", defaults)
			}
		})
	}
}

func TestCreatePlanNormalizesCodeAndReturnsPrices(t *testing.T) {
	repo := &billingRepositoryStub{}
	service := newPlanTestService(repo)

	view, err := service.CreatePlan(context.Background(), PlanSaveInput{
		Code:     " Team-Plus ",
		Name:     "Team Plus",
		IsActive: true,
		Prices: []PlanPriceInput{
			{BillingInterval: "month", AmountCents: 3000},
			{BillingInterval: "year", AmountCents: 30000, IsDefault: true},
		},
	})
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}
	if view.Code != "team-plus" {
		t.Fatalf("code = %q, want team-plus", view.Code)
	}
	if len(view.Prices) != 2 {
		t.Fatalf("prices = %d, want 2", len(view.Prices))
	}
}

func TestCreatePlanRejectsInvalidCode(t *testing.T) {
	service := newPlanTestService(&billingRepositoryStub{})
	for _, code := range []string{"", "-pro", "pro plan", "超级", "a234567890123456789012345678901234"} {
		_, err := service.CreatePlan(context.Background(), PlanSaveInput{
			Code:     code,
			Name:     "Plan",
			IsActive: true,
			Prices:   monthlyPriceInput(100),
		})
		if !errors.Is(err, ErrInvalidBillingPlan) {
			t.Fatalf("code %q: err = %v, want ErrInvalidBillingPlan", code, err)
		}
	}
}

func TestCreatePlanMapsDuplicateCode(t *testing.T) {
	service := newPlanTestService(&billingRepositoryStub{saveErr: repository.ErrDuplicate})
	_, err := service.CreatePlan(context.Background(), PlanSaveInput{
		Code:     "pro",
		Name:     "Pro",
		IsActive: true,
		Prices:   monthlyPriceInput(100),
	})
	if !errors.Is(err, ErrBillingPlanCodeConflict) {
		t.Fatalf("err = %v, want ErrBillingPlanCodeConflict", err)
	}
}

func TestUpdatePlanKeepsCodeAndAllowsDeactivation(t *testing.T) {
	repo := &billingRepositoryStub{plans: []domainbilling.Plan{{ID: 2, Code: "pro", Name: "Pro", IsActive: true}}}
	service := newPlanTestService(repo)

	view, err := service.UpdatePlan(context.Background(), 2, PlanSaveInput{
		Code:     "renamed",
		Name:     "Pro",
		IsActive: false,
		Prices:   monthlyPriceInput(2000),
	})
	if err != nil {
		t.Fatalf("UpdatePlan() error = %v", err)
	}
	if view.Code != "pro" || view.IsActive {
		t.Fatalf("view code=%q active=%v, want pro inactive", view.Code, view.IsActive)
	}
}

func TestFreePlanCannotBeDeactivatedOrDeleted(t *testing.T) {
	repo := &billingRepositoryStub{plans: []domainbilling.Plan{{ID: 1, Code: "free", Name: "Free", IsActive: true}}}
	service := newPlanTestService(repo)

	_, err := service.UpdatePlan(context.Background(), 1, PlanSaveInput{
		Name:     "Free",
		IsActive: false,
		Prices:   []PlanPriceInput{{BillingInterval: "lifetime"}},
	})
	if !errors.Is(err, ErrBillingPlanProtected) {
		t.Fatalf("deactivate err = %v, want ErrBillingPlanProtected", err)
	}
	if err := service.DeletePlan(context.Background(), 1); !errors.Is(err, ErrBillingPlanProtected) {
		t.Fatalf("delete err = %v, want ErrBillingPlanProtected", err)
	}
	if repo.deletedPlanID != 0 {
		t.Fatalf("free plan reached repository delete")
	}
}

func TestDeletePlanReportsReferencedPlan(t *testing.T) {
	repo := &billingRepositoryStub{
		plans:     []domainbilling.Plan{{ID: 3, Code: "max", Name: "Max"}},
		deleteErr: repository.ErrConflict,
	}
	service := newPlanTestService(repo)
	if err := service.DeletePlan(context.Background(), 3); !errors.Is(err, ErrBillingPlanInUse) {
		t.Fatalf("err = %v, want ErrBillingPlanInUse", err)
	}
}

func TestDeletePlanMissing(t *testing.T) {
	service := newPlanTestService(&billingRepositoryStub{})
	if err := service.DeletePlan(context.Background(), 9); !errors.Is(err, ErrBillingPlanNotFound) {
		t.Fatalf("err = %v, want ErrBillingPlanNotFound", err)
	}
}
