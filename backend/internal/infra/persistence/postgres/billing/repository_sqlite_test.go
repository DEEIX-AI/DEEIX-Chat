package billing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
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

func TestAddPeriodUsageAndSettleOverageSplitsCreditAndBalance(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	periodStart := now.Add(-2 * time.Hour)
	periodEnd := now.Add(2 * time.Hour)

	account := model.BillingAccount{
		UserID:         1,
		Currency:       "USD",
		BalanceNanousd: 500,
		Status:         "active",
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	if err := db.Create(&model.UsageLedger{
		BaseModel:           model.BaseModel{CreatedAt: now.Add(-time.Hour)},
		UserID:              1,
		PlatformModelName:   "gpt-before",
		UsageDate:           now.Add(-time.Hour),
		BilledCurrency:      "USD",
		BilledNanousd:       800,
		PricingSnapshotJSON: `{}`,
	}).Error; err != nil {
		t.Fatalf("create previous usage: %v", err)
	}

	usage := &domainbilling.UsageLedger{
		UserID:              1,
		PlatformModelName:   "gpt-current",
		UsageDate:           now,
		BilledCurrency:      "USD",
		BilledNanousd:       500,
		PricingSnapshotJSON: `{"pricing_mode":"token"}`,
	}
	err := repo.AddPeriodUsageAndSettleOverage(ctx, usage, periodStart, periodEnd, 1000, nil)
	if err != nil {
		t.Fatalf("AddPeriodUsageAndSettleOverage() error = %v", err)
	}

	var refreshed model.BillingAccount
	if err := db.Where("user_id = ?", 1).First(&refreshed).Error; err != nil {
		t.Fatalf("load billing account: %v", err)
	}
	if refreshed.BalanceNanousd != 200 {
		t.Fatalf("balance = %d, want 200", refreshed.BalanceNanousd)
	}
	var tx model.BalanceTransaction
	if err := db.Where("user_id = ? AND type = ?", 1, domainbilling.BalanceTransactionTypeUsage).First(&tx).Error; err != nil {
		t.Fatalf("load balance transaction: %v", err)
	}
	if tx.AmountNanousd != -300 || tx.BalanceAfterNanousd != 200 {
		t.Fatalf("transaction amount/balance = %d/%d, want -300/200", tx.AmountNanousd, tx.BalanceAfterNanousd)
	}

	var ledger model.UsageLedger
	if err := db.Where("platform_model_name = ?", "gpt-current").First(&ledger).Error; err != nil {
		t.Fatalf("load current usage: %v", err)
	}
	if ledger.BilledNanousd != 500 {
		t.Fatalf("billed nanousd = %d, want 500", ledger.BilledNanousd)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("decode pricing snapshot: %v", err)
	}
	if usage.PricingSnapshotJSON != ledger.PricingSnapshotJSON {
		t.Fatalf("usage snapshot was not updated after settlement")
	}
	covered := int64(snapshot["period_credit_covered_nanousd"].(float64))
	overage := int64(snapshot["period_overage_billed_nanousd"].(float64))
	if covered != 200 || overage != 300 {
		t.Fatalf("snapshot split = covered %d overage %d, want 200/300", covered, overage)
	}
	debited := int64(snapshot["period_balance_debited_nanousd"].(float64))
	delta := int64(snapshot["period_balance_settlement_delta_nanousd"].(float64))
	if debited != 300 || delta != 300 {
		t.Fatalf("snapshot balance = debit %d delta %d, want 300/300", debited, delta)
	}
}

func TestValidateRedeemableCodeAllowsUsageCodeInPeriodModeOnly(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	usageCode := model.RedemptionCode{
		Status:         domainbilling.RedemptionCodeStatusActive,
		Mode:           domainbilling.RedemptionCodeModeUsage,
		RewardType:     domainbilling.RedemptionRewardTypeBalance,
		CreditNanousd:  100,
		PerUserLimit:   1,
		RedeemedCount:  0,
		MaxRedemptions: nil,
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return validateRedeemableCode(tx, usageCode, 1, domainbilling.RedemptionCodeModePeriod, now)
	})
	if err != nil {
		t.Fatalf("validateRedeemableCode(usage code in period mode) error = %v", err)
	}

	periodCode := model.RedemptionCode{
		Status:        domainbilling.RedemptionCodeStatusActive,
		Mode:          domainbilling.RedemptionCodeModePeriod,
		RewardType:    domainbilling.RedemptionRewardTypeSubscription,
		PlanID:        2,
		PerUserLimit:  1,
		RedeemedCount: 0,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		return validateRedeemableCode(tx, periodCode, 1, domainbilling.RedemptionCodeModeUsage, now)
	})
	if err != repository.ErrRedemptionUnavailable {
		t.Fatalf("validateRedeemableCode(period code in usage mode) error = %v, want ErrRedemptionUnavailable", err)
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

	if err := db.AutoMigrate(&model.UsageLedger{}, &model.BillingAccount{}, &model.BalanceTransaction{}, &model.Redemption{}); err != nil {
		t.Fatalf("migrate billing tables: %v", err)
	}
	return db
}
