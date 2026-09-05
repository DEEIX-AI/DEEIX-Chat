package billing

import (
	"context"
	"errors"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestEstimateUsageNanousdFollowsPricingMode(t *testing.T) {
	tests := []struct {
		name    string
		pricing *domainbilling.ModelPricing
		input   UsageEstimateInput
		want    int64
	}{
		{
			name:    "missing pricing estimates zero",
			pricing: nil,
			input:   UsageEstimateInput{PlatformModelName: "unpriced", InputTokens: 1_000},
			want:    0,
		},
		{
			name: "free model estimates zero",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:      "free-model",
				IsFree:                 true,
				InputNanousdPerMTokens: 1_000_000_000,
			},
			input: UsageEstimateInput{PlatformModelName: "free-model", InputTokens: 1_000},
			want:  0,
		},
		{
			name: "token pricing charges non-cached input plus declared max output",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:       "gpt-test",
				PricingMode:             domainbilling.PricingModeToken,
				InputNanousdPerMTokens:  1_000_000_000,
				OutputNanousdPerMTokens: 4_000_000_000,
			},
			input: UsageEstimateInput{PlatformModelName: "gpt-test", InputTokens: 2_000, OutputTokens: 500},
			want:  2_000_000 + 2_000_000,
		},
		{
			name: "token pricing without max output only charges input",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:       "gpt-test",
				PricingMode:             domainbilling.PricingModeToken,
				InputNanousdPerMTokens:  1_000_000_000,
				OutputNanousdPerMTokens: 4_000_000_000,
			},
			input: UsageEstimateInput{PlatformModelName: "gpt-test", InputTokens: 2_000},
			want:  2_000_000,
		},
		{
			name: "anthropic fast mode applies the same six times multiplier as the ledger",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:      "claude-opus-4.6",
				PricingMode:            domainbilling.PricingModeToken,
				InputNanousdPerMTokens: 1_000_000_000,
			},
			input: UsageEstimateInput{
				PlatformModelName: "claude-opus-4.6",
				ProviderProtocol:  "anthropic_messages",
				RequestSpeed:      "fast",
				InputTokens:       1_000,
			},
			want: 6_000_000,
		},
		{
			name: "openai priority tier doubles the estimate",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:      "gpt-5",
				PricingMode:            domainbilling.PricingModeToken,
				InputNanousdPerMTokens: 1_000_000_000,
			},
			input: UsageEstimateInput{
				PlatformModelName:  "gpt-5",
				ProviderProtocol:   "openai_responses",
				RequestServiceTier: "priority",
				InputTokens:        1_000,
			},
			want: 2_000_000,
		},
		{
			name: "token pricing charges observed cache usage at the cache rates like the ledger",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:           "claude-sonnet",
				PricingMode:                 domainbilling.PricingModeToken,
				InputNanousdPerMTokens:      3_000_000_000,
				CacheReadNanousdPerMTokens:  300_000_000,
				CacheWriteNanousdPerMTokens: 3_000_000_000,
				OutputNanousdPerMTokens:     15_000_000_000,
			},
			input: UsageEstimateInput{
				PlatformModelName: "claude-sonnet",
				ProviderProtocol:  "anthropic_messages",
				CacheTimeout:      "1h",
				InputTokens:       1_000,
				CacheReadTokens:   10_000,
				CacheWriteTokens:  2_000,
				OutputTokens:      400,
			},
			// 输入 3000 + 缓存读 3000 + 缓存写 2000×(3.0×2 for 1h)=12000 + 输出 6000
			want: 3_000_000 + 3_000_000 + 12_000_000 + 6_000_000,
		},
		{
			name: "tiered pricing resolves the tier by estimated input",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName: "gemini-tiered",
				PricingMode:       domainbilling.PricingModeTiered,
				TieredPricingJSON: `{"tiers":[{"upToTokens":1000,"inputUSDPerMTokens":1,"outputUSDPerMTokens":2},{"upToTokens":0,"inputUSDPerMTokens":3,"outputUSDPerMTokens":6}]}`,
			},
			input: UsageEstimateInput{PlatformModelName: "gemini-tiered", InputTokens: 2_000, OutputTokens: 1_000},
			want:  6_000_000 + 6_000_000,
		},
		{
			name: "tiered pricing counts cached tokens toward the tier like the ledger",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName: "gemini-tiered",
				PricingMode:       domainbilling.PricingModeTiered,
				TieredPricingJSON: `{"tiers":[{"upToTokens":1000,"inputUSDPerMTokens":1,"cacheReadUSDPerMTokens":0.1,"outputUSDPerMTokens":2},{"upToTokens":0,"inputUSDPerMTokens":3,"cacheReadUSDPerMTokens":0.3,"outputUSDPerMTokens":6}]}`,
			},
			input: UsageEstimateInput{PlatformModelName: "gemini-tiered", InputTokens: 500, CacheReadTokens: 1_000, OutputTokens: 1_000},
			// 500 + 1000 = 1500 tokens 落入第二档：输入 500×3，缓存读 1000×0.3，输出 1000×6。
			want: 1_500_000 + 300_000 + 6_000_000,
		},
		{
			name: "call pricing charges at least one call",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:  "image-model",
				PricingMode:        domainbilling.PricingModeCall,
				CallNanousdPerCall: 40_000_000,
			},
			input: UsageEstimateInput{PlatformModelName: "image-model"},
			want:  40_000_000,
		},
		{
			name: "duration pricing charges requested seconds",
			pricing: &domainbilling.ModelPricing{
				PlatformModelName:        "video-model",
				PricingMode:              domainbilling.PricingModeDuration,
				DurationNanousdPerSecond: 500_000_000,
			},
			input: UsageEstimateInput{PlatformModelName: "video-model", DurationSeconds: 8},
			want:  4_000_000_000,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(&billingRepositoryStub{mode: "usage", pricing: tc.pricing})

			got, err := service.EstimateUsageNanousd(context.Background(), 1, tc.input)
			if err != nil {
				t.Fatalf("EstimateUsageNanousd() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("EstimateUsageNanousd() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEnsureUsageAuthorizationBudgetSkipsWhenReservationAlreadyCovers(t *testing.T) {
	repo := &billingRepositoryStub{mode: "usage"}
	service := NewService(repo)
	authorization := &domainbilling.UsageAuthorization{
		Mode:        "usage",
		Reservation: &domainbilling.UsageBalanceReservation{UserID: 1, RefNo: "run_1", Mode: "usage", BalanceNanousd: 300, PeriodCreditNanousd: 200},
	}

	if err := service.EnsureUsageAuthorizationBudget(context.Background(), authorization, 500); err != nil {
		t.Fatalf("EnsureUsageAuthorizationBudget() error = %v", err)
	}
	if err := service.EnsureUsageAuthorizationBudget(context.Background(), &domainbilling.UsageAuthorization{Mode: "self"}, 900); err != nil {
		t.Fatalf("EnsureUsageAuthorizationBudget() self mode error = %v", err)
	}
	if err := service.EnsureUsageAuthorizationBudget(context.Background(), nil, 900); err != nil {
		t.Fatalf("EnsureUsageAuthorizationBudget() nil authorization error = %v", err)
	}
	if repo.raisedReservationNanousd != 0 {
		t.Fatalf("reservation raised to %d, want no raise when budget already covers the estimate", repo.raisedReservationNanousd)
	}
}

func TestEnsureUsageAuthorizationBudgetRaisesReservationToEstimate(t *testing.T) {
	repo := &billingRepositoryStub{mode: "usage"}
	service := NewService(repo)
	authorization := &domainbilling.UsageAuthorization{
		Mode:        "usage",
		Reservation: &domainbilling.UsageBalanceReservation{UserID: 1, RefNo: "run_1", Mode: "usage", BalanceNanousd: 100},
	}

	if err := service.EnsureUsageAuthorizationBudget(context.Background(), authorization, 900); err != nil {
		t.Fatalf("EnsureUsageAuthorizationBudget() error = %v", err)
	}
	if repo.raisedReservationRefNo != "run_1" || repo.raisedReservationNanousd != 900 {
		t.Fatalf("raised reservation = (%q, %d), want run_1 raised to 900", repo.raisedReservationRefNo, repo.raisedReservationNanousd)
	}
}

func TestEnsureUsageAuthorizationBudgetMapsRepositoryErrors(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		want    error
	}{
		{name: "insufficient balance", repoErr: repository.ErrInsufficientBalance, want: ErrUsageBalanceInsufficient},
		{name: "inactive reservation", repoErr: repository.ErrConflict, want: ErrUsageReservationConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &billingRepositoryStub{mode: "usage", raiseReservationErr: tc.repoErr}
			service := NewService(repo)
			authorization := &domainbilling.UsageAuthorization{
				Mode:        "usage",
				Reservation: &domainbilling.UsageBalanceReservation{UserID: 1, RefNo: "run_1", Mode: "usage", BalanceNanousd: 100},
			}

			err := service.EnsureUsageAuthorizationBudget(context.Background(), authorization, 900)
			if !errors.Is(err, tc.want) {
				t.Fatalf("EnsureUsageAuthorizationBudget() error = %v, want %v", err, tc.want)
			}
			if authorization.Reservation.BalanceNanousd != 100 {
				t.Fatalf("reservation mutated on failure: %+v", authorization.Reservation)
			}
		})
	}
}
