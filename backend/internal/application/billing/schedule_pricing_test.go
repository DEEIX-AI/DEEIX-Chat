package billing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestParseSchedulePeriodsNormalizesAndRejectsBadInput(t *testing.T) {
	periods, err := parseSchedulePeriods(`{"periods":[{"name":" 高峰 ","weekdays":[5,1,3],"start":"9:00","end":"18:00","ratePercent":150}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(periods) != 1 || periods[0].Name != "高峰" || periods[0].Start != "09:00" || periods[0].End != "18:00" {
		t.Fatalf("unexpected normalization: %+v", periods)
	}
	if got := periods[0].Weekdays; len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("weekdays must be sorted, got %v", got)
	}
	for _, empty := range []string{"", "{}", `{"periods":[]}`} {
		if got, err := parseSchedulePeriods(empty); err != nil || got != nil {
			t.Fatalf("%q must parse to no periods, got %v err=%v", empty, got, err)
		}
	}
	for name, raw := range map[string]string{
		"missing name":   `{"periods":[{"weekdays":[1],"start":"09:00","end":"18:00","ratePercent":150}]}`,
		"zero rate":      `{"periods":[{"name":"a","weekdays":[1],"start":"09:00","end":"18:00","ratePercent":0}]}`,
		"bad weekday":    `{"periods":[{"name":"a","weekdays":[7],"start":"09:00","end":"18:00","ratePercent":150}]}`,
		"dup weekday":    `{"periods":[{"name":"a","weekdays":[1,1],"start":"09:00","end":"18:00","ratePercent":150}]}`,
		"no weekdays":    `{"periods":[{"name":"a","weekdays":[],"start":"09:00","end":"18:00","ratePercent":150}]}`,
		"bad time":       `{"periods":[{"name":"a","weekdays":[1],"start":"25:00","end":"18:00","ratePercent":150}]}`,
		"empty range":    `{"periods":[{"name":"a","weekdays":[1],"start":"09:00","end":"09:00","ratePercent":150}]}`,
		"overlap same":   `{"periods":[{"name":"a","weekdays":[1],"start":"09:00","end":"18:00","ratePercent":150},{"name":"b","weekdays":[1],"start":"17:00","end":"20:00","ratePercent":50}]}`,
		"overlap wrap":   `{"periods":[{"name":"night","weekdays":[0],"start":"22:00","end":"06:00","ratePercent":50},{"name":"early","weekdays":[1],"start":"05:00","end":"08:00","ratePercent":80}]}`,
		"malformed json": `{"periods":`,
		"not an object":  `[]`,
		"too many":       tooManyPeriodsJSON(),
		"name too long":  `{"periods":[{"name":"` + string(make([]rune, 33)) + `","weekdays":[1],"start":"09:00","end":"18:00","ratePercent":150}]}`,
		"rate above cap": `{"periods":[{"name":"a","weekdays":[1],"start":"09:00","end":"18:00","ratePercent":10001}]}`,
	} {
		if _, err := parseSchedulePeriods(raw); !errors.Is(err, repository.ErrInvalidInput) {
			t.Fatalf("%s: expected invalid input, got %v", name, err)
		}
	}
	// 相邻不重叠：18:00 结束与 18:00 开始可以共存。
	if _, err := parseSchedulePeriods(`{"periods":[{"name":"a","weekdays":[1],"start":"09:00","end":"18:00","ratePercent":150},{"name":"b","weekdays":[1],"start":"18:00","end":"22:00","ratePercent":80}]}`); err != nil {
		t.Fatalf("adjacent periods must be allowed: %v", err)
	}
}

func tooManyPeriodsJSON() string {
	raw := `{"periods":[`
	for index := 0; index <= maxSchedulePeriods; index++ {
		if index > 0 {
			raw += ","
		}
		raw += `{"name":"p","weekdays":[1],"start":"09:00","end":"10:00","ratePercent":100}`
	}
	return raw + "]}"
}

func localTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", value, time.Local)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}

func TestResolveSchedulePeriodUsesLocalClockAndWrapsMidnight(t *testing.T) {
	// 2026-09-21 是周一。
	periods, err := parseSchedulePeriods(`{"periods":[
		{"name":"peak","weekdays":[1,2,3,4,5],"start":"09:00","end":"18:00","ratePercent":150},
		{"name":"night","weekdays":[0,1,2,3,4,5,6],"start":"23:00","end":"06:00","ratePercent":50}
	]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := map[string]string{
		"2026-09-21 09:00": "peak",
		"2026-09-21 17:59": "peak",
		"2026-09-21 18:00": "",
		"2026-09-21 23:30": "night",
		"2026-09-22 05:59": "night",
		"2026-09-22 06:00": "",
		"2026-09-26 12:00": "",      // 周六白天无规则
		"2026-09-27 00:30": "night", // 周六 23:00 跨到周日
	}
	for at, want := range cases {
		got := resolveSchedulePeriod(periods, localTime(t, at))
		name := ""
		if got != nil {
			name = got.Name
		}
		if name != want {
			t.Fatalf("%s: want %q, got %q", at, want, name)
		}
	}
}

func TestComposeRatePercentStacksOnBaseMultiplier(t *testing.T) {
	base := billingRateMultiplier{Numerator: 2, Denominator: 1}
	got := composeRatePercent(base, 50)
	if applyRateMultiplier(1_000_000, got) != 1_000_000 {
		t.Fatalf("2x then 50%% must equal 1x, got %v", got)
	}
	if applyRateMultiplier(1_000_000, composeRatePercent(base, 100)) != 2_000_000 {
		t.Fatal("100% must leave the base multiplier untouched")
	}
}

func TestNormalizeSchedulePricingJSONRoundTrips(t *testing.T) {
	normalized, err := normalizeSchedulePricingJSON(`{"periods":[{"name":"off","weekdays":[6,0],"start":"0:00","end":"23:59","ratePercent":70}]}`)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized != `{"periods":[{"name":"off","weekdays":[0,6],"start":"00:00","end":"23:59","ratePercent":70}]}` {
		t.Fatalf("unexpected normalized json: %s", normalized)
	}
	if empty, err := normalizeSchedulePricingJSON(""); err != nil || empty != "{}" {
		t.Fatalf("empty must normalize to {}, got %q err=%v", empty, err)
	}
}

func TestBuildUsageLedgerAppliesSchedulePeriodAtBillingTime(t *testing.T) {
	// 两段拼满全天：00:00–12:00 ×2，12:00–00:00 ×0.5；用 BillingAt 固定命中哪一段。
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:      "gpt-test",
			Currency:               "USD",
			PricingMode:            domainbilling.PricingModeToken,
			InputNanousdPerMTokens: 1_000_000_000,
			SchedulePricingJSON: `{"periods":[
				{"name":"morning","weekdays":[0,1,2,3,4,5,6],"start":"00:00","end":"12:00","ratePercent":200},
				{"name":"afternoon","weekdays":[0,1,2,3,4,5,6],"start":"12:00","end":"00:00","ratePercent":50}
			]}`,
		},
	}
	service := NewService(repo)
	service.SetGroupRateMultiplierResolver(&groupRateResolverStub{percent: 100})

	for _, tc := range []struct {
		at     string
		billed int64
		name   string
		rate   float64
	}{
		{at: "2026-09-21 08:00", billed: 2_000_000_000, name: "morning", rate: 2},
		{at: "2026-09-21 15:00", billed: 500_000_000, name: "afternoon", rate: 0.5},
	} {
		ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
			UserID:            1,
			PlatformModelName: "gpt-test",
			InputTokens:       1_000_000,
			BillingAt:         localTime(t, tc.at),
		})
		if err != nil {
			t.Fatalf("%s: BuildUsageLedger: %v", tc.at, err)
		}
		if ledger.BilledNanousd != tc.billed {
			t.Fatalf("%s: billed=%d want %d", tc.at, ledger.BilledNanousd, tc.billed)
		}
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
			t.Fatalf("unmarshal snapshot: %v", err)
		}
		if snapshot["schedule_period_name"] != tc.name || snapshot["rate_multiplier"] != tc.rate {
			t.Fatalf("%s: snapshot period=%v rate=%v, want %s / %v", tc.at, snapshot["schedule_period_name"], snapshot["rate_multiplier"], tc.name, tc.rate)
		}
	}
}

func TestUpsertModelPricingValidatesAndNormalizesSchedule(t *testing.T) {
	repo := &billingRepositoryStub{}
	service := NewService(repo)
	service.SetModelPricingCatalogProvider(modelPricingCatalogStub{names: map[string]struct{}{"chat-model": {}}})

	_, err := service.UpsertModelPricing(t.Context(), ModelPricingInput{
		PlatformModelName:      "chat-model",
		PricingMode:            domainbilling.PricingModeToken,
		InputNanousdPerMTokens: 1,
		SchedulePricingJSON:    `{"periods":[{"name":"a","weekdays":[1],"start":"09:00","end":"18:00","ratePercent":150},{"name":"b","weekdays":[1],"start":"17:00","end":"20:00","ratePercent":50}]}`,
	})
	if !errors.Is(err, ErrInvalidModelPricing) {
		t.Fatalf("overlapping periods must be rejected, got %v", err)
	}

	view, err := service.UpsertModelPricing(t.Context(), ModelPricingInput{
		PlatformModelName:      "chat-model",
		PricingMode:            domainbilling.PricingModeToken,
		InputNanousdPerMTokens: 1,
		SchedulePricingJSON:    `{"periods":[{"name":" 高峰 ","weekdays":[5,1],"start":"9:00","end":"18:00","ratePercent":150}]}`,
	})
	if err != nil {
		t.Fatalf("valid schedule must be accepted: %v", err)
	}
	want := `{"periods":[{"name":"高峰","weekdays":[1,5],"start":"09:00","end":"18:00","ratePercent":150}]}`
	if view.SchedulePricingJSON != want {
		t.Fatalf("stored schedule must be normalized, got %s", view.SchedulePricingJSON)
	}

	public := toPublicModelPricing(view.ModelPricing)
	if len(public.SchedulePeriods) != 1 || public.SchedulePeriods[0].Name != "高峰" || public.SchedulePeriods[0].RatePercent != 150 {
		t.Fatalf("public pricing must expose the schedule, got %+v", public.SchedulePeriods)
	}

	cleared, err := service.UpsertModelPricing(t.Context(), ModelPricingInput{PlatformModelName: "chat-model", PricingMode: domainbilling.PricingModeToken, InputNanousdPerMTokens: 1})
	if err != nil || cleared.SchedulePricingJSON != "{}" {
		t.Fatalf("omitting the schedule must clear it to {}, got %q err=%v", cleared.SchedulePricingJSON, err)
	}
}
