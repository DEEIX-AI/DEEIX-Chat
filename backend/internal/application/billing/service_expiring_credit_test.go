package billing

import (
	"context"
	"errors"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestCreateUsageRedemptionCodePreservesOptionalCreditDuration(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)
	service.SetRedemptionCodeKeyring(testRedemptionKeyring(t))

	_, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:                "LIMITED-01D",
		Mode:                domainbilling.RedemptionCodeModeUsage,
		CreditUSD:           20,
		BalanceDurationDays: 0.1,
		PerUserLimit:        1,
	})
	if err != nil {
		t.Fatalf("CreateRedemptionCodes() error = %v", err)
	}
	if len(repo.created) != 1 || repo.created[0].DurationDays != 0 || repo.created[0].BalanceDurationSeconds != 8640 {
		t.Fatalf("created usage duration = %+v, want 8640 seconds", repo.created)
	}

	_, err = service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:                "TOO-SHORT",
		Mode:                domainbilling.RedemptionCodeModeUsage,
		CreditUSD:           20,
		BalanceDurationDays: 0.05,
		PerUserLimit:        1,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() short duration error = %v, want ErrInvalidRedemptionCode", err)
	}

	_, err = service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:                "TOO-LONG",
		Mode:                domainbilling.RedemptionCodeModeUsage,
		CreditUSD:           20,
		BalanceDurationDays: 3651,
		PerUserLimit:        1,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() duration error = %v, want ErrInvalidRedemptionCode", err)
	}
}

func TestFractionalDaysToSeconds(t *testing.T) {
	testCases := []struct {
		name      string
		days      float64
		allowZero bool
		seconds   int64
		valid     bool
	}{
		{name: "tenth day", days: 0.1, seconds: 8640, valid: true},
		{name: "permanent", days: 0, allowZero: true, seconds: 0, valid: true},
		{name: "zero disallowed", days: 0, valid: false},
		{name: "below minimum", days: 0.05, valid: false},
		{name: "above maximum", days: 3650.1, valid: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			seconds, valid := fractionalDaysToSeconds(testCase.days, testCase.allowZero)
			if seconds != testCase.seconds || valid != testCase.valid {
				t.Fatalf("fractionalDaysToSeconds(%v, %v) = (%d, %v), want (%d, %v)", testCase.days, testCase.allowZero, seconds, valid, testCase.seconds, testCase.valid)
			}
		})
	}
}

func TestRedemptionDurationFieldsCannotBeMixed(t *testing.T) {
	service := NewService(newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage))
	for _, input := range []RedemptionCodeInput{
		{Mode: "usage", CreditUSD: 20, DurationDays: 1},
		{Mode: "period", PlanID: 1, DurationDays: 30, BalanceDurationDays: 0.1},
	} {
		if _, err := service.normalizeRedemptionCodeInput(t.Context(), input); !errors.Is(err, ErrInvalidRedemptionCode) {
			t.Fatalf("mixed duration fields accepted: %+v, err=%v", input, err)
		}
	}
}

func TestRedemptionSnapshotKeepsFractionalBalanceSeparate(t *testing.T) {
	for _, raw := range []string{
		`{"reward_type":"balance","duration_days":0,"balance_duration_seconds":8640}`,
		`{"reward_type":"balance","duration_days":0.1}`,
	} {
		if got := redemptionSnapshotBalanceDuration(raw); got != 8640 {
			t.Fatalf("balance duration=%d", got)
		}
		if got := redemptionSnapshotDurationDays(raw); got != 0 {
			t.Fatalf("subscription duration=%d", got)
		}
	}
}
