package billing

import (
	"encoding/json"
	"sync/atomic"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
)

func TestAccountListBatchesExpirationChecks(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	userIDs := make([]uint, 32)
	for i := range userIDs {
		userIDs[i] = uint(i + 1)
		createBillingAccount(t, db, userIDs[i], 100)
	}
	credit := grantRegressionCredit(t, repo, userIDs[0])
	makeCreditDue(t, db, credit.ID)

	var queries atomic.Int32
	const callback = "test:count-expiration-queries"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "billing_expiring_credits" {
			queries.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	items, err := repo.ListBillingAccountsByUserIDs(t.Context(), userIDs)
	if err != nil || len(items) != len(userIDs) {
		t.Fatalf("account list: count=%d err=%v", len(items), err)
	}
	if n := queries.Load(); n > 3 {
		t.Fatalf("expiration lookups scale with account count: %d queries for %d accounts", n, len(userIDs))
	}
	for _, item := range items {
		if item.BalanceNanousd != 100 {
			t.Fatalf("user %d: balance=%d, want 100", item.UserID, item.BalanceNanousd)
		}
	}
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)
}

func TestSubscriptionSnapshotDoesNotAcquireBalanceLifetime(t *testing.T) {
	code := models.RedemptionCode{
		Mode: domainbilling.RedemptionCodeModePeriod, RewardType: domainbilling.RedemptionRewardTypeSubscription,
		DurationDays: 30,
	}
	var snapshot struct {
		DurationDays           int   `json:"duration_days"`
		BalanceDurationSeconds int64 `json:"balance_duration_seconds"`
	}
	if err := json.Unmarshal([]byte(redemptionSnapshotJSON(code)), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.DurationDays != 30 || snapshot.BalanceDurationSeconds != 0 {
		t.Fatalf("subscription acquired a balance lifetime: %+v", snapshot)
	}
}
