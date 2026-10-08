package billing

import (
	"context"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

const personalModelRef = "personal:ab12cd34ef56/gpt-4o"

func TestAuthorizeUsageReservesBudgetForPersonalModelsWithoutPricing(t *testing.T) {
	repo := &billingRepositoryStub{mode: "usage"}
	service := NewService(repo)

	authorization, err := service.AuthorizeUsage(context.Background(), 1, personalModelRef, "run_personal")
	if err != nil {
		t.Fatalf("AuthorizeUsage() error = %v; a personal model must not require platform pricing", err)
	}
	if authorization == nil || authorization.Mode != "usage" {
		t.Fatalf("authorization = %#v", authorization)
	}
	// 预留仍然发生：同一请求中由平台付费的服务需要受余额约束。
	if repo.reservationRequest == nil {
		t.Fatal("expected a budget reservation for platform-paid services")
	}
}

func TestBuildUsageLedgerNeverChargesPersonalModelTokens(t *testing.T) {
	service := NewService(&billingRepositoryStub{mode: "usage"})

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		Authorization:     &domainbilling.UsageAuthorization{Mode: "usage"},
		UserID:            1,
		PlatformModelName: personalModelRef,
		UpstreamName:      "My OpenAI",
		InputTokens:       1_000_000,
		OutputTokens:      1_000_000,
		BillingAt:         time.Now(),
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger() error = %v", err)
	}
	if ledger.BilledNanousd != 0 {
		t.Fatalf("personal model tokens were billed: %d", ledger.BilledNanousd)
	}
	if ledger.InputTokens != 1_000_000 || ledger.OutputTokens != 1_000_000 {
		t.Fatalf("usage must still be recorded: %#v", ledger)
	}
	if ledger.PlatformModelName != personalModelRef {
		t.Fatalf("ledger model = %q", ledger.PlatformModelName)
	}
}

func TestPlatformModelsStillRequirePricing(t *testing.T) {
	// 防回归：只有 personal: 前缀的模型跳过价格校验。
	service := NewService(&billingRepositoryStub{mode: "usage"})
	if _, err := service.AuthorizeUsage(context.Background(), 1, "personal-looking-model", "run_platform"); err == nil {
		t.Fatal("a platform model without pricing must still be rejected")
	}
}

func TestServiceItemsRunOnPersonalModelsAreFree(t *testing.T) {
	service := NewService(&billingRepositoryStub{mode: "usage"})
	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		Authorization:     &domainbilling.UsageAuthorization{Mode: "usage"},
		UserID:            1,
		PlatformModelName: personalModelRef,
		ServiceOnly:       true,
		ServiceItems: []ServiceUsageInput{{
			ServiceCode:       "title",
			PlatformModelName: personalModelRef,
			InputTokens:       500,
			OutputTokens:      20,
		}},
		BillingAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger() error = %v; title generation on a personal model must not need platform pricing", err)
	}
	if ledger.BilledNanousd != 0 {
		t.Fatalf("personal service item billed: %d", ledger.BilledNanousd)
	}
}
