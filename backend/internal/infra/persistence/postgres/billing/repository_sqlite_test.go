package billing

import (
	"context"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUsageQueriesUseSQLitePortableExpressions(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	usageDate := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	entries := []model.UsageLedger{
		{
			UserID:              1,
			PlatformModelName:   "gpt-test",
			UpstreamModelName:   "gpt-test-upstream",
			UsageDate:           usageDate,
			InputTokens:         100,
			OutputTokens:        50,
			CallCount:           1,
			LatencyMS:           100,
			BilledNanousd:       300,
			PricingSnapshotJSON: `{"pricing_mode":"token"}`,
		},
		{
			UserID:              1,
			PlatformModelName:   "call-test",
			UpstreamModelName:   "call-test-upstream",
			UsageDate:           usageDate,
			CallCount:           2,
			LatencyMS:           300,
			BilledNanousd:       500,
			PricingSnapshotJSON: `{"pricing_mode":"call"}`,
		},
		{
			UserID:            2,
			PlatformModelName: "other-user",
			UsageDate:         usageDate,
			BilledNanousd:     900,
		},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatalf("create usage ledgers: %v", err)
	}

	logs, total, err := repo.ListUsageLogs(ctx, repository.UsageLogListFilter{
		UserID:      1,
		BillingMode: "call",
	}, 0, 10)
	if err != nil {
		t.Fatalf("ListUsageLogs() error = %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].PlatformModelName != "call-test" {
		t.Fatalf("expected one call-mode usage log, total=%d logs=%v", total, logs)
	}

	monthly, err := repo.ListMonthlyUsageByUser(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ListMonthlyUsageByUser() error = %v", err)
	}
	if len(monthly) != 1 {
		t.Fatalf("expected one monthly summary, got %d", len(monthly))
	}
	if monthly[0].MonthStartAt.Format("2006-01-02") != "2026-06-01" || monthly[0].BilledNanousd != 800 {
		t.Fatalf("unexpected monthly summary: %+v", monthly[0])
	}

	daily, err := repo.ListDailyUsageByUser(ctx, 1, usageDate, usageDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("ListDailyUsageByUser() error = %v", err)
	}
	if len(daily) != 1 {
		t.Fatalf("expected one daily summary, got %d", len(daily))
	}
	if daily[0].UsageDate.Format("2006-01-02") != "2026-06-06" || daily[0].BilledNanousd != 800 {
		t.Fatalf("unexpected daily summary: %+v", daily[0])
	}
	if len(daily[0].Models) != 2 {
		t.Fatalf("expected two daily model summaries, got %d", len(daily[0].Models))
	}
}

func openBillingSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:billing_usage_queries?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(&model.UsageLedger{}); err != nil {
		t.Fatalf("migrate usage ledgers: %v", err)
	}
	return db
}
