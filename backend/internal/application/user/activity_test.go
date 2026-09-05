package user

import (
	"context"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

type activityStatsRepositoryStub struct {
	items []domainuser.DailyActivity
}

func (s activityStatsRepositoryStub) GetDailyActivityByUser(context.Context, uint, time.Time, time.Time) ([]domainuser.DailyActivity, error) {
	return s.items, nil
}

func TestGetDailyActivityFillsMissingBillingDates(t *testing.T) {
	service := &Service{activityStatsRepo: activityStatsRepositoryStub{items: []domainuser.DailyActivity{
		{Date: "2026-08-24", RequestCount: 1, TokenUsage: 12},
		{Date: "2026-08-26", RequestCount: 2, TokenUsage: 34},
	}}}

	items, err := service.GetDailyActivity(context.Background(), 7, 3, time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetDailyActivity() error = %v", err)
	}
	want := []domainuser.DailyActivity{
		{Date: "2026-08-24", RequestCount: 1, TokenUsage: 12},
		{Date: "2026-08-25"},
		{Date: "2026-08-26", RequestCount: 2, TokenUsage: 34},
	}
	if len(items) != len(want) {
		t.Fatalf("GetDailyActivity() returned %d rows, want %d", len(items), len(want))
	}
	for index := range want {
		if items[index] != want[index] {
			t.Fatalf("row %d = %+v, want %+v", index, items[index], want[index])
		}
	}
}
