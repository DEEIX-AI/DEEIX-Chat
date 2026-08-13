package billing

import (
	"testing"
	"time"
)

func TestFillMonthlyUsageSummariesCapsRequestedMonths(t *testing.T) {
	now := time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)

	items := fillMonthlyUsageSummaries(nil, int(^uint(0)>>1), now)

	if len(items) != maxMonthlyUsageMonths {
		t.Fatalf("len(items) = %d, want %d", len(items), maxMonthlyUsageMonths)
	}
}
