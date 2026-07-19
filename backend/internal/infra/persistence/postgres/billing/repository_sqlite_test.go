package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	statistics, err := repo.GetUsageStatistics(ctx, repository.UsageStatisticsFilter{
		StartDate:        usageDate,
		EndDateExclusive: usageDate.AddDate(0, 0, 1),
		Granularity:      "day",
		ModelRankBy:      "tokens",
		UserRankBy:       "cost",
		RankLimit:        10,
	})
	if err != nil {
		t.Fatalf("GetUsageStatistics() error = %v", err)
	}
	if len(statistics.TopModels) != 3 || statistics.TopModels[0].PlatformModelName != "gpt-test" {
		t.Fatalf("expected models ranked by tokens, got %+v", statistics.TopModels)
	}
	if len(statistics.TopUsers) != 2 || statistics.TopUsers[0].UserID != 2 {
		t.Fatalf("expected users ranked by cost, got %+v", statistics.TopUsers)
	}

	modelStatistics, err := repo.GetUsageStatistics(ctx, repository.UsageStatisticsFilter{
		StartDate:        usageDate,
		EndDateExclusive: usageDate.AddDate(0, 0, 1),
		Granularity:      "day",
		Section:          "models",
		ModelRankBy:      "tokens",
		RankLimit:        10,
	})
	if err != nil {
		t.Fatalf("GetUsageStatistics(models) error = %v", err)
	}
	if len(modelStatistics.TopModels) != 3 || len(modelStatistics.TopUsers) != 0 || len(modelStatistics.Trend) != 0 {
		t.Fatalf("expected model-only statistics, got %+v", modelStatistics)
	}

	weeklyStatistics, err := repo.GetUsageStatistics(ctx, repository.UsageStatisticsFilter{
		StartDate:        usageDate,
		EndDateExclusive: usageDate.AddDate(0, 0, 1),
		Granularity:      "week",
		Section:          "all",
		ModelRankBy:      "cost",
		UserRankBy:       "cost",
		RankLimit:        10,
	})
	if err != nil {
		t.Fatalf("GetUsageStatistics(week) error = %v", err)
	}
	if len(weeklyStatistics.Trend) != 1 || weeklyStatistics.Trend[0].PeriodStart.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("unexpected weekly statistics: %+v", weeklyStatistics.Trend)
	}
}

func TestUsageStatisticsFiltersByCurrentPermissionGroupMembership(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	if err := db.AutoMigrate(
		&model.PermissionGroup{},
		&model.PermissionGroupUserAccess{},
		&model.BillingPlan{},
		&model.Subscription{},
	); err != nil {
		t.Fatalf("migrate permission group tables: %v", err)
	}

	groups := []model.PermissionGroup{
		{Name: "Default", IsDefault: true},
		{Name: "Pro"},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("create permission groups: %v", err)
	}
	if err := db.Create(&model.PermissionGroupUserAccess{GroupID: groups[1].ID, UserID: 1}).Error; err != nil {
		t.Fatalf("create manual group member: %v", err)
	}

	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	activePlan := model.BillingPlan{Code: "pro", Name: "Pro", IsActive: true, PermissionGroupID: &groups[1].ID}
	inactivePlan := model.BillingPlan{Code: "legacy", Name: "Legacy", IsActive: false, PermissionGroupID: &groups[1].ID}
	if err := db.Create(&[]model.BillingPlan{activePlan, inactivePlan}).Error; err != nil {
		t.Fatalf("create billing plans: %v", err)
	}
	var plans []model.BillingPlan
	if err := db.Order("id ASC").Find(&plans).Error; err != nil {
		t.Fatalf("reload billing plans: %v", err)
	}
	periodEnd := now.Add(24 * time.Hour)
	expiredEnd := now.Add(-time.Hour)
	futureStart := now.Add(time.Hour)
	subscriptions := []model.Subscription{
		{UserID: 1, PlanID: plans[0].ID, PriceID: 1, Status: "active", StartAt: now.Add(-time.Hour), CurrentPeriodStartAt: now.Add(-time.Hour), CurrentPeriodEndAt: &periodEnd},
		{UserID: 2, PlanID: plans[0].ID, PriceID: 1, Status: "active", StartAt: now.Add(-time.Hour), CurrentPeriodStartAt: now.Add(-time.Hour), CurrentPeriodEndAt: &periodEnd},
		{UserID: 3, PlanID: plans[0].ID, PriceID: 1, Status: "active", StartAt: now.Add(-48 * time.Hour), CurrentPeriodStartAt: now.Add(-48 * time.Hour), CurrentPeriodEndAt: &expiredEnd},
		{UserID: 4, PlanID: plans[0].ID, PriceID: 1, Status: "active", StartAt: futureStart, CurrentPeriodStartAt: futureStart, CurrentPeriodEndAt: &periodEnd},
		{UserID: 5, PlanID: plans[1].ID, PriceID: 1, Status: "active", StartAt: now.Add(-time.Hour), CurrentPeriodStartAt: now.Add(-time.Hour), CurrentPeriodEndAt: &periodEnd},
		{UserID: 6, PlanID: plans[0].ID, PriceID: 1, Status: "active", StartAt: now.Add(-time.Hour), CurrentPeriodStartAt: now.Add(-time.Hour), CurrentPeriodEndAt: &periodEnd},
	}
	if err := db.Create(&subscriptions).Error; err != nil {
		t.Fatalf("create subscriptions: %v", err)
	}
	if err := db.Delete(&subscriptions[5]).Error; err != nil {
		t.Fatalf("soft delete active subscription: %v", err)
	}

	usageDate := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	ledgers := make([]model.UsageLedger, 0, 6)
	for userID := uint(1); userID <= 6; userID++ {
		ledgers = append(ledgers, model.UsageLedger{
			UserID:            userID,
			PlatformModelName: "gpt-test",
			BillingAt:         usageDate,
			UsageDate:         usageDate,
			CallCount:         1,
			BilledNanousd:     int64(userID) * 100,
		})
	}
	if err := db.Create(&ledgers).Error; err != nil {
		t.Fatalf("create usage ledgers: %v", err)
	}

	filter := repository.UsageStatisticsFilter{
		StartDate:         usageDate,
		EndDateExclusive:  usageDate.AddDate(0, 0, 1),
		PermissionGroupID: groups[1].ID,
		MembershipAt:      now,
		Granularity:       "day",
		ModelRankBy:       "cost",
		UserRankBy:        "cost",
		RankLimit:         10,
	}
	statistics, err := NewRepo(db).GetUsageStatistics(context.Background(), filter)
	if err != nil {
		t.Fatalf("GetUsageStatistics() error = %v", err)
	}
	if statistics.Totals.RecordCount != 2 || statistics.Totals.CallCount != 2 || statistics.Totals.BilledNanousd != 300 {
		t.Fatalf("expected users 1 and 2 to be counted once, got %+v", statistics.Totals)
	}
	if len(statistics.TopUsers) != 2 || statistics.TopUsers[0].UserID != 2 || statistics.TopUsers[1].UserID != 1 {
		t.Fatalf("unexpected permission group user ranking: %+v", statistics.TopUsers)
	}

	filter.PermissionGroupID = groups[0].ID
	statistics, err = NewRepo(db).GetUsageStatistics(context.Background(), filter)
	if err != nil {
		t.Fatalf("GetUsageStatistics(default group) error = %v", err)
	}
	if statistics.Totals.RecordCount != 6 || statistics.Totals.BilledNanousd != 2100 {
		t.Fatalf("expected default group to include all users, got %+v", statistics.Totals)
	}
}

func TestAddUsageAndSettleBalanceRecordsDebtWithoutReservation(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	account := model.BillingAccount{
		UserID:         1,
		Currency:       "USD",
		BalanceNanousd: 100,
		Status:         "active",
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "gpt-test",
		BillingAt:         now,
		UsageDate:         now,
		BilledCurrency:    "USD",
		BilledNanousd:     300,
	}

	if err := repo.AddUsageAndSettleBalance(ctx, usage, nil); err != nil {
		t.Fatalf("AddUsageAndSettleBalance() error = %v", err)
	}

	assertUsageSettlement(t, db, 1, "gpt-test", -200, -300, -200, "")
}

func TestAddUsageAndSettleBalanceRecordsDebtBeyondReservation(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	account := model.BillingAccount{
		UserID:         1,
		Currency:       "USD",
		BalanceNanousd: 200,
		Status:         "active",
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 100, "run-debt"))
	if err != nil {
		t.Fatalf("ReserveUsageBalance() error = %v", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "gpt-reserved",
		BillingAt:         now,
		UsageDate:         now,
		BilledCurrency:    "USD",
		BilledNanousd:     350,
	}

	if err = repo.AddUsageAndSettleBalance(ctx, usage, reservation); err != nil {
		t.Fatalf("AddUsageAndSettleBalance() error = %v", err)
	}

	assertUsageSettlement(t, db, 1, "gpt-reserved", -150, -350, -150, "run-debt")
}

func TestAddUsageAndSettleBalanceChargesActualBelowReservation(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	account := model.BillingAccount{
		UserID:         1,
		Currency:       "USD",
		BalanceNanousd: 300,
		Status:         "active",
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 200, "run-refund"))
	if err != nil {
		t.Fatalf("ReserveUsageBalance() error = %v", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "gpt-refund",
		BillingAt:         now,
		UsageDate:         now,
		BilledCurrency:    "USD",
		BilledNanousd:     50,
	}

	if err = repo.AddUsageAndSettleBalance(ctx, usage, reservation); err != nil {
		t.Fatalf("AddUsageAndSettleBalance() error = %v", err)
	}

	assertUsageSettlement(t, db, 1, "gpt-refund", 250, -50, 250, "run-refund")
}

func TestAddUsageAndSettleBalanceLeavesFreeModelBalanceUnchanged(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	account := model.BillingAccount{
		UserID:         1,
		Currency:       "USD",
		BalanceNanousd: 100,
		Status:         "active",
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "free-model",
		IsFreeModel:       true,
		BillingAt:         now,
		UsageDate:         now,
		BilledCurrency:    "USD",
		BilledNanousd:     0,
	}

	if err := repo.AddUsageAndSettleBalance(ctx, usage, nil); err != nil {
		t.Fatalf("AddUsageAndSettleBalance() error = %v", err)
	}

	var refreshed model.BillingAccount
	if err := db.Where("user_id = ?", 1).First(&refreshed).Error; err != nil {
		t.Fatalf("load billing account: %v", err)
	}
	if refreshed.BalanceNanousd != 100 {
		t.Fatalf("balance = %d, want unchanged 100", refreshed.BalanceNanousd)
	}
	var ledger model.UsageLedger
	if err := db.Where("user_id = ? AND platform_model_name = ?", 1, "free-model").First(&ledger).Error; err != nil {
		t.Fatalf("load usage ledger: %v", err)
	}
	var transactionCount int64
	if err := db.Model(&model.BalanceTransaction{}).Where("user_id = ?", 1).Count(&transactionCount).Error; err != nil {
		t.Fatalf("count balance transactions: %v", err)
	}
	if transactionCount != 0 {
		t.Fatalf("balance transaction count = %d, want 0", transactionCount)
	}
}

func assertUsageSettlement(
	t *testing.T,
	db *gorm.DB,
	userID uint,
	platformModelName string,
	wantBalance int64,
	wantTransactionAmount int64,
	wantTransactionBalance int64,
	wantRefNo string,
) {
	t.Helper()

	var account model.BillingAccount
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load billing account: %v", err)
	}
	if account.BalanceNanousd != wantBalance {
		t.Fatalf("balance = %d, want %d", account.BalanceNanousd, wantBalance)
	}

	var ledger model.UsageLedger
	if err := db.Where("user_id = ? AND platform_model_name = ?", userID, platformModelName).First(&ledger).Error; err != nil {
		t.Fatalf("load usage ledger: %v", err)
	}

	var transaction model.BalanceTransaction
	if err := db.Where("user_id = ? AND type = ? AND ref_id = ?", userID, domainbilling.BalanceTransactionTypeUsage, ledger.ID).First(&transaction).Error; err != nil {
		t.Fatalf("load settlement transaction: %v", err)
	}
	if transaction.AmountNanousd != wantTransactionAmount || transaction.BalanceAfterNanousd != wantTransactionBalance {
		t.Fatalf(
			"transaction amount/balance = %d/%d, want %d/%d",
			transaction.AmountNanousd,
			transaction.BalanceAfterNanousd,
			wantTransactionAmount,
			wantTransactionBalance,
		)
	}
	if transaction.RefNo != wantRefNo {
		t.Fatalf("transaction ref no = %q, want %q", transaction.RefNo, wantRefNo)
	}
	if wantRefNo != "" {
		var reservation model.UsageReservation
		if err := db.Where("user_id = ? AND ref_no = ?", userID, wantRefNo).First(&reservation).Error; err != nil {
			t.Fatalf("load usage reservation: %v", err)
		}
		if reservation.Status != domainbilling.UsageReservationStatusSettled || reservation.UsageLedgerID != ledger.ID {
			t.Fatalf("reservation status/ledger = %s/%d, want settled/%d", reservation.Status, reservation.UsageLedgerID, ledger.ID)
		}
	}
}

func usageReservationRequest(userID uint, amountNanousd int64, refNo string) domainbilling.UsageBalanceReservationRequest {
	return domainbilling.UsageBalanceReservationRequest{
		UserID:           userID,
		RefNo:            refNo,
		Mode:             "usage",
		RequestedNanousd: amountNanousd,
	}
}

func TestReserveUsageBalanceDefaultBudgetAllowsFiveConcurrentCalls(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	if err := db.Create(&model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 100, Status: "active"}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}

	reservations := make([]*domainbilling.UsageBalanceReservation, 0, domainbilling.UsageReservationMaxActivePerUser)
	for index := 0; index < domainbilling.UsageReservationMaxActivePerUser; index++ {
		reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 0, fmt.Sprintf("run_%d", index)))
		if err != nil {
			t.Fatalf("reserve request %d: %v", index, err)
		}
		if reservation.BalanceNanousd != 20 || reservation.PeriodCreditNanousd != 0 {
			t.Fatalf("reservation %d = %+v, want balance budget 20", index, reservation)
		}
		reservations = append(reservations, reservation)
	}
	if _, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 0, "run_over_limit")); !errors.Is(err, repository.ErrUsageReservationLimitExceeded) {
		t.Fatalf("reserve over limit error = %v, want ErrUsageReservationLimitExceeded", err)
	}
	if err := repo.ReleaseUsageBalanceReservation(ctx, 1, reservations[0].RefNo); err != nil {
		t.Fatalf("release first reservation: %v", err)
	}
	if _, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 0, "run_after_release")); err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
}

func TestSettledReservationReopensSlotOnlyWhenBudgetRemains(t *testing.T) {
	tests := []struct {
		name                  string
		settledNanousd        int64
		wantReservationBudget int64
		wantErr               error
	}{
		{name: "budget remains", settledNanousd: 10, wantReservationBudget: 10},
		{name: "actual charge exhausts budget", settledNanousd: 50, wantErr: repository.ErrInsufficientBalance},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openBillingSQLiteTestDB(t)
			repo := NewRepo(db)
			ctx := context.Background()
			if err := db.Create(&model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 100, Status: "active"}).Error; err != nil {
				t.Fatalf("create billing account: %v", err)
			}

			reservations := make([]*domainbilling.UsageBalanceReservation, 0, domainbilling.UsageReservationMaxActivePerUser)
			for index := 0; index < domainbilling.UsageReservationMaxActivePerUser; index++ {
				reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 0, fmt.Sprintf("settle_run_%d", index)))
				if err != nil {
					t.Fatalf("reserve request %d: %v", index, err)
				}
				reservations = append(reservations, reservation)
			}

			now := time.Now()
			err := repo.AddUsageAndSettleBalance(ctx, &domainbilling.UsageLedger{
				UserID:            1,
				PlatformModelName: "gpt-settled",
				BillingAt:         now,
				UsageDate:         now,
				BilledCurrency:    "USD",
				BilledNanousd:     tt.settledNanousd,
			}, reservations[0])
			if err != nil {
				t.Fatalf("settle first reservation: %v", err)
			}

			refill, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 0, "refill_run"))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("refill reservation error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (refill == nil || refill.BalanceNanousd != tt.wantReservationBudget) {
				t.Fatalf("refill reservation = %+v, want budget %d", refill, tt.wantReservationBudget)
			}
		})
	}
}

func TestReserveUsageBalanceRejectsReusedReference(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Now()
	if err := db.Create(&model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 100, Status: "active"}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}

	reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 50, "run_reused"))
	if err != nil {
		t.Fatalf("reserve request: %v", err)
	}
	if _, err = repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 50, "run_reused")); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("reserve active reference error = %v, want ErrConflict", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "gpt-replay",
		BillingAt:         now,
		UsageDate:         now,
		BilledCurrency:    "USD",
		BilledNanousd:     25,
	}
	periodStart := now.Add(-time.Hour)
	periodEnd := now.Add(time.Hour)
	if err = repo.AddPeriodUsageAndSettleOverage(ctx, usage, periodStart, periodEnd, 100, reservation); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("settle usage reservation as period error = %v, want ErrConflict", err)
	}
	if err = repo.AddUsageAndSettleBalance(ctx, usage, reservation); err != nil {
		t.Fatalf("settle reservation: %v", err)
	}
	replayedUsage := *usage
	replayedPlatformModelName := "gpt-replay-duplicate"
	replayedUsage.PlatformModelName = replayedPlatformModelName
	if err = repo.AddUsageAndSettleBalance(ctx, &replayedUsage, reservation); err != nil {
		t.Fatalf("retry settled reservation: %v", err)
	}
	if replayedUsage.PlatformModelName != usage.PlatformModelName || replayedUsage.BilledNanousd != usage.BilledNanousd {
		t.Fatalf("retried usage was not restored from settled ledger: %+v", replayedUsage)
	}
	var replayedLedgerCount int64
	if err = db.Model(&model.UsageLedger{}).Where("platform_model_name = ?", replayedPlatformModelName).Count(&replayedLedgerCount).Error; err != nil {
		t.Fatalf("count replayed usage ledgers: %v", err)
	}
	if replayedLedgerCount != 0 {
		t.Fatalf("replayed usage ledger count = %d, want 0", replayedLedgerCount)
	}
	if _, err = repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 50, "run_reused")); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("reserve settled reference error = %v, want ErrConflict", err)
	}
}

func TestReserveUsageBalanceRejectsLegacyReference(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	account := model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 100, Status: "active"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	if err := db.Create(&model.BalanceTransaction{
		AccountID:           account.ID,
		UserID:              1,
		Type:                domainbilling.BalanceTransactionTypeUsageReserve,
		AmountNanousd:       -50,
		BalanceAfterNanousd: 50,
		RefType:             "usage_reservation",
		RefNo:               "legacy_run",
	}).Error; err != nil {
		t.Fatalf("create legacy reservation: %v", err)
	}

	if _, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 50, "legacy_run")); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("reserve legacy reference error = %v, want ErrConflict", err)
	}
}

func TestRenewUsageBalanceReservationExtendsActiveLease(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	if err := db.Create(&model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 100, Status: "active"}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 50, "run_renew"))
	if err != nil {
		t.Fatalf("reserve request: %v", err)
	}
	originalExpiry := reservation.ExpiresAt
	if err = db.Model(&model.UsageReservation{}).Where("id = ?", reservation.ID).Update("expires_at", time.Now().Add(time.Minute)).Error; err != nil {
		t.Fatalf("shorten reservation lease: %v", err)
	}
	if err = repo.RenewUsageBalanceReservation(ctx, 1, reservation.RefNo); err != nil {
		t.Fatalf("renew reservation: %v", err)
	}
	var renewed model.UsageReservation
	if err = db.First(&renewed, reservation.ID).Error; err != nil {
		t.Fatalf("load renewed reservation: %v", err)
	}
	if !renewed.ExpiresAt.After(originalExpiry.Add(-time.Minute)) {
		t.Fatalf("renewed expiry = %v, original expiry = %v", renewed.ExpiresAt, originalExpiry)
	}
}

func TestReconciliationReservationContinuesBlockingBudgetAfterExpiry(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	if err := db.Create(&model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 100, Status: "active"}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	reservation, err := repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 100, "run_reconcile"))
	if err != nil {
		t.Fatalf("reserve request: %v", err)
	}
	if err = repo.MarkUsageReservationReconciliationRequired(ctx, 1, reservation.RefNo, "settle_failed"); err != nil {
		t.Fatalf("mark reconciliation: %v", err)
	}
	if err = db.Model(&model.UsageReservation{}).Where("id = ?", reservation.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire reconciliation reservation: %v", err)
	}
	if _, err = repo.ReserveUsageBalance(ctx, usageReservationRequest(1, 1, "run_after_reconcile")); !errors.Is(err, repository.ErrInsufficientBalance) {
		t.Fatalf("reserve after reconciliation error = %v, want ErrInsufficientBalance", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "gpt-reconciled",
		BillingAt:         time.Now(),
		UsageDate:         time.Now(),
		BilledCurrency:    "USD",
		BilledNanousd:     25,
	}
	if err = repo.AddUsageAndSettleBalance(ctx, usage, reservation); err != nil {
		t.Fatalf("settle reconciliation reservation: %v", err)
	}
}

func TestMarkUsageReservationReconciliationRejectsMissingReservation(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)

	err := repo.MarkUsageReservationReconciliationRequired(context.Background(), 1, "missing_run", "settle_failed")
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("mark missing reservation error = %v, want ErrConflict", err)
	}
}

func TestReservePeriodUsageDefaultBudgetAllowsFiveConcurrentCalls(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Now()
	periodStart := now.Add(-time.Hour)
	periodEnd := now.Add(time.Hour)
	if err := db.Create(&model.BillingAccount{UserID: 1, Currency: "USD", BalanceNanousd: 0, Status: "active"}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	request := domainbilling.UsageBalanceReservationRequest{
		UserID:              1,
		Mode:                "period",
		PeriodStartAt:       &periodStart,
		PeriodEndAt:         &periodEnd,
		PeriodCreditNanousd: 1000,
	}

	for index := 0; index < domainbilling.UsageReservationMaxActivePerUser; index++ {
		request.RefNo = fmt.Sprintf("run_period_%d", index)
		reservation, err := repo.ReserveUsageBalance(ctx, request)
		if err != nil {
			t.Fatalf("reserve period request %d: %v", index, err)
		}
		if reservation.PeriodCreditNanousd != 200 || reservation.BalanceNanousd != 0 {
			t.Fatalf("period reservation %d = %+v, want credit budget 200", index, reservation)
		}
	}
	request.RefNo = "run_period_over_limit"
	if _, err := repo.ReserveUsageBalance(ctx, request); !errors.Is(err, repository.ErrUsageReservationLimitExceeded) {
		t.Fatalf("reserve period over limit error = %v, want ErrUsageReservationLimitExceeded", err)
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
		BillingAt:           now.Add(-time.Hour),
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
		BillingAt:           now,
		UsageDate:           now,
		BilledCurrency:      "USD",
		BilledNanousd:       500,
		PricingSnapshotJSON: `{"pricing_mode":"token"}`,
	}
	reservation, err := repo.ReserveUsageBalance(ctx, domainbilling.UsageBalanceReservationRequest{
		UserID:              1,
		RefNo:               "run-period-split",
		Mode:                "period",
		RequestedNanousd:    300,
		PeriodStartAt:       &periodStart,
		PeriodEndAt:         &periodEnd,
		PeriodCreditNanousd: 1000,
	})
	if err != nil {
		t.Fatalf("ReserveUsageBalance() error = %v", err)
	}
	if reservation.PeriodCreditNanousd != 200 || reservation.BalanceNanousd != 100 {
		t.Fatalf("reservation split = %+v, want credit 200 and balance 100", reservation)
	}
	if reservation.PeriodLimitNanousd != 1000 {
		t.Fatalf("reservation period limit = %d, want 1000", reservation.PeriodLimitNanousd)
	}
	err = repo.AddPeriodUsageAndSettleOverage(ctx, usage, periodStart, periodEnd, 1000, reservation)
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
	charged := int64(snapshot["period_balance_charged_nanousd"].(float64))
	delta := int64(snapshot["period_balance_settlement_delta_nanousd"].(float64))
	reservedCredit := int64(snapshot["period_credit_reserved_nanousd"].(float64))
	if charged != 300 || delta != 200 || reservedCredit != 200 {
		t.Fatalf("snapshot balance = charged %d delta %d credit reserved %d, want 300/200/200", charged, delta, reservedCredit)
	}
	var settledReservation model.UsageReservation
	if err := db.Where("id = ?", reservation.ID).First(&settledReservation).Error; err != nil {
		t.Fatalf("load settled reservation: %v", err)
	}
	if settledReservation.Status != domainbilling.UsageReservationStatusSettled {
		t.Fatalf("reservation status = %q, want settled", settledReservation.Status)
	}

	retryUsage := *usage
	retryUsage.PricingSnapshotJSON = ""
	if err := repo.AddPeriodUsageAndSettleOverage(ctx, &retryUsage, periodStart, periodEnd, 1000, reservation); err != nil {
		t.Fatalf("retry settled period reservation: %v", err)
	}
	if retryUsage.PricingSnapshotJSON != ledger.PricingSnapshotJSON {
		t.Fatalf("retry snapshot was not restored from settled ledger")
	}
	var ledgerCount int64
	if err := db.Model(&model.UsageLedger{}).Where("user_id = ?", 1).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count period ledgers: %v", err)
	}
	if ledgerCount != 2 {
		t.Fatalf("period ledger count = %d, want existing + settled ledgers only", ledgerCount)
	}
}

func TestAddPeriodUsageAndSettleOverageRecordsDebt(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	periodStart := now.Add(-2 * time.Hour)
	periodEnd := now.Add(2 * time.Hour)

	account := model.BillingAccount{
		UserID:         1,
		Currency:       "USD",
		BalanceNanousd: 100,
		Status:         "active",
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	if err := db.Create(&model.UsageLedger{
		BaseModel:           model.BaseModel{CreatedAt: now.Add(-time.Hour)},
		UserID:              1,
		PlatformModelName:   "gpt-before-debt",
		BillingAt:           now.Add(-time.Hour),
		UsageDate:           now.Add(-time.Hour),
		BilledCurrency:      "USD",
		BilledNanousd:       900,
		PricingSnapshotJSON: `{}`,
	}).Error; err != nil {
		t.Fatalf("create previous usage: %v", err)
	}
	usage := &domainbilling.UsageLedger{
		UserID:              1,
		PlatformModelName:   "gpt-period-debt",
		BillingAt:           now,
		UsageDate:           now,
		BilledCurrency:      "USD",
		BilledNanousd:       500,
		PricingSnapshotJSON: `{"pricing_mode":"token"}`,
	}

	if err := repo.AddPeriodUsageAndSettleOverage(ctx, usage, periodStart, periodEnd, 1000, nil); err != nil {
		t.Fatalf("AddPeriodUsageAndSettleOverage() error = %v", err)
	}

	assertUsageSettlement(t, db, 1, "gpt-period-debt", -300, -400, -300, "")
	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(usage.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("decode pricing snapshot: %v", err)
	}
	if got := int64(snapshot["period_credit_covered_nanousd"].(float64)); got != 100 {
		t.Fatalf("period credit covered = %d, want 100", got)
	}
	if got := int64(snapshot["period_overage_billed_nanousd"].(float64)); got != 400 {
		t.Fatalf("period overage = %d, want 400", got)
	}
}

func TestAddPeriodUsageAndSettleOverageUsesBillingAtForPeriodBoundary(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	createdInside := periodStart.Add(5 * time.Second)
	billedOutside := periodStart.Add(-2 * time.Second)

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
		BaseModel:           model.BaseModel{CreatedAt: createdInside},
		UserID:              1,
		PlatformModelName:   "gpt-previous-period",
		BillingAt:           billedOutside,
		UsageDate:           billedOutside,
		BilledCurrency:      "USD",
		BilledNanousd:       800,
		PricingSnapshotJSON: `{}`,
	}).Error; err != nil {
		t.Fatalf("create previous period usage: %v", err)
	}

	total, err := repo.SumBillableNanousd(ctx, 1, periodStart, periodEnd)
	if err != nil {
		t.Fatalf("SumBillableNanousd() error = %v", err)
	}
	if total != 0 {
		t.Fatalf("period total = %d, want 0", total)
	}

	usage := &domainbilling.UsageLedger{
		UserID:              1,
		PlatformModelName:   "gpt-current-period",
		BillingAt:           periodStart.Add(time.Hour),
		UsageDate:           periodStart,
		BilledCurrency:      "USD",
		BilledNanousd:       500,
		PricingSnapshotJSON: `{}`,
	}
	err = repo.AddPeriodUsageAndSettleOverage(ctx, usage, periodStart, periodEnd, 1000, nil)
	if err != nil {
		t.Fatalf("AddPeriodUsageAndSettleOverage() error = %v", err)
	}

	var refreshed model.BillingAccount
	if err := db.Where("user_id = ?", 1).First(&refreshed).Error; err != nil {
		t.Fatalf("load billing account: %v", err)
	}
	if refreshed.BalanceNanousd != 500 {
		t.Fatalf("balance = %d, want unchanged 500", refreshed.BalanceNanousd)
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

	if err := db.AutoMigrate(&model.UsageLedger{}, &model.BillingAccount{}, &model.BalanceTransaction{}, &model.UsageReservation{}, &model.Redemption{}); err != nil {
		t.Fatalf("migrate billing tables: %v", err)
	}
	return db
}
