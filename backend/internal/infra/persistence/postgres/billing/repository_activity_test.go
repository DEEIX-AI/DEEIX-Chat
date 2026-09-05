package billing

import (
	"context"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
)

func TestGetDailyActivityByUserUsesMainUsageLedgers(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	usageDate := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)

	entries := []model.UsageLedger{
		{
			UserID: 7, UsageDate: usageDate, BillingAt: usageDate.Add(time.Hour),
			InputTokens: 100, CacheReadTokens: 10, CacheWriteTokens: 5, OutputTokens: 20, ReasoningTokens: 3,
			PricingSnapshotJSON: `{"pricing_mode":"token"}`,
		},
		{
			UserID: 7, UsageDate: usageDate, BillingAt: usageDate.Add(2 * time.Hour),
			InputTokens: 7, CacheReadTokens: 2, CacheWriteTokens: 1, OutputTokens: 4, ReasoningTokens: 6,
			PricingSnapshotJSON: `{"service_only":false}`,
		},
		{
			UserID: 7, UsageDate: usageDate, BillingAt: usageDate.Add(3 * time.Hour), InputTokens: 999,
			PricingSnapshotJSON: `{"service_items":[{"service_code":"title"}]}`,
		},
		{
			UserID: 7, UsageDate: usageDate, BillingAt: usageDate.Add(4 * time.Hour), InputTokens: 888,
			PricingSnapshotJSON: `{"service_only":true,"service_items":[{"service_code":"future_internal_task"}]}`,
		},
		{
			UserID: 8, UsageDate: usageDate, BillingAt: usageDate.Add(5 * time.Hour), InputTokens: 777,
			PricingSnapshotJSON: `{"service_only":false}`,
		},
		{
			UserID: 7, UsageDate: usageDate.AddDate(0, 0, 1), BillingAt: usageDate.AddDate(0, 0, 1), InputTokens: 666,
			PricingSnapshotJSON: `{"service_only":false}`,
		},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatalf("create usage ledgers: %v", err)
	}

	items, err := repo.GetDailyActivityByUser(context.Background(), 7, usageDate, usageDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("GetDailyActivityByUser() error = %v", err)
	}
	want := []domainuser.DailyActivity{{Date: "2026-08-26", RequestCount: 2, TokenUsage: 158}}
	if len(items) != len(want) {
		t.Fatalf("GetDailyActivityByUser() returned %d rows (%v), want %d", len(items), items, len(want))
	}
	if items[0] != want[0] {
		t.Fatalf("GetDailyActivityByUser() = %+v, want %+v", items[0], want[0])
	}
}
