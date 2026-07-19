package billing

import (
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestUsageStatisticsGranularity(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{days: 1, want: "day"},
		{days: 30, want: "day"},
		{days: 31, want: "week"},
		{days: 90, want: "week"},
		{days: 179, want: "week"},
		{days: 180, want: "month"},
		{days: 366, want: "month"},
	}

	for _, tt := range tests {
		if got := usageStatisticsGranularity(tt.days); got != tt.want {
			t.Fatalf("usageStatisticsGranularity(%d) = %q, want %q", tt.days, got, tt.want)
		}
	}
}

func TestFillUsageStatisticsTrendUsesMondayWeekBoundaries(t *testing.T) {
	location := time.UTC
	startDate := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	endDate := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	items := []domainbilling.UsageStatisticsTrendPoint{
		{
			PeriodStart: time.Date(2026, 7, 6, 0, 0, 0, 0, location),
			Metrics:     domainbilling.UsageStatisticsMetrics{CallCount: 3},
		},
	}

	result := fillUsageStatisticsTrend(items, startDate, endDate, "week")
	wantDates := []string{"2026-06-29", "2026-07-06", "2026-07-13", "2026-07-20"}
	if len(result) != len(wantDates) {
		t.Fatalf("weekly trend length = %d, want %d", len(result), len(wantDates))
	}
	for index, wantDate := range wantDates {
		if got := result[index].PeriodStart.Format("2006-01-02"); got != wantDate {
			t.Fatalf("weekly trend[%d] = %q, want %q", index, got, wantDate)
		}
	}
	if result[1].Metrics.CallCount != 3 {
		t.Fatalf("weekly trend did not preserve existing metrics: %+v", result[1])
	}
}
