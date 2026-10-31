package billing

import (
	"context"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

const testNanousdPerUSD int64 = 1_000_000_000

func TestExpiringCreditGrantConsumeAndExpire(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	createBillingAccount(t, db, 1, 10*testNanousdPerUSD)

	expiresAt := time.Now().Add(48 * time.Hour)
	credit, account, err := grantTestCredit(ctx, repo, 1, 20*testNanousdPerUSD, expiresAt, 9, "grant-1", "two day credit")
	if err != nil {
		t.Fatalf("GrantExpiringCredit() error = %v", err)
	}
	if account.BalanceNanousd != 30*testNanousdPerUSD || credit.RemainingNanousd != 20*testNanousdPerUSD {
		t.Fatalf("unexpected grant result: account=%+v credit=%+v", account, credit)
	}

	settleUsage(t, repo, 1, 5*testNanousdPerUSD, "usage-1")
	assertBillingBalance(t, db, 1, 25*testNanousdPerUSD)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusActive, 15*testNanousdPerUSD)

	count, err := repo.ExpireDueExpiringCredits(ctx, expiresAt.Add(time.Second), 100)
	if err != nil {
		t.Fatalf("ExpireDueExpiringCredits() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("expired count = %d, want 1", count)
	}
	assertBillingBalance(t, db, 1, 10*testNanousdPerUSD)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)

	var transaction models.BalanceTransaction
	if err := db.Where("user_id = ? AND type = ?", 1, domainbilling.BalanceTransactionTypeExpiringCreditExpire).First(&transaction).Error; err != nil {
		t.Fatalf("load expiration transaction: %v", err)
	}
	if transaction.AmountNanousd != -15*testNanousdPerUSD || transaction.BalanceAfterNanousd != 10*testNanousdPerUSD {
		t.Fatalf("unexpected expiration transaction: %+v", transaction)
	}
}

func TestExpiringCreditFullyConsumedIsNotDeductedAtExpiration(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	createBillingAccount(t, db, 1, 10*testNanousdPerUSD)

	expiresAt := time.Now().Add(48 * time.Hour)
	credit, _, err := grantTestCredit(ctx, repo, 1, 20*testNanousdPerUSD, expiresAt, 9, "grant-full", "")
	if err != nil {
		t.Fatalf("GrantExpiringCredit() error = %v", err)
	}
	settleUsage(t, repo, 1, 25*testNanousdPerUSD, "usage-full")
	assertBillingBalance(t, db, 1, 5*testNanousdPerUSD)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusConsumed, 0)

	count, err := repo.ExpireDueExpiringCredits(ctx, expiresAt.Add(time.Second), 100)
	if err != nil {
		t.Fatalf("ExpireDueExpiringCredits() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expired count = %d, want 0", count)
	}
	assertBillingBalance(t, db, 1, 5*testNanousdPerUSD)
}

func TestExpiringCreditsConsumeEarliestExpirationFirst(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	createBillingAccount(t, db, 1, 0)

	now := time.Now()
	early, _, err := grantTestCredit(ctx, repo, 1, 10*testNanousdPerUSD, now.Add(24*time.Hour), 9, "grant-early", "")
	if err != nil {
		t.Fatalf("grant early credit: %v", err)
	}
	late, _, err := grantTestCredit(ctx, repo, 1, 20*testNanousdPerUSD, now.Add(48*time.Hour), 9, "grant-late", "")
	if err != nil {
		t.Fatalf("grant late credit: %v", err)
	}

	settleUsage(t, repo, 1, 15*testNanousdPerUSD, "usage-fifo")
	assertExpiringCredit(t, db, early.ID, domainbilling.ExpiringCreditStatusConsumed, 0)
	assertExpiringCredit(t, db, late.ID, domainbilling.ExpiringCreditStatusActive, 15*testNanousdPerUSD)
	assertBillingBalance(t, db, 1, 15*testNanousdPerUSD)
}

func TestAdminBalanceReductionDoesNotCauseDoubleExpiration(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	createBillingAccount(t, db, 1, 10*testNanousdPerUSD)

	expiresAt := time.Now().Add(48 * time.Hour)
	credit, _, err := grantTestCredit(ctx, repo, 1, 20*testNanousdPerUSD, expiresAt, 9, "grant-admin", "")
	if err != nil {
		t.Fatalf("GrantExpiringCredit() error = %v", err)
	}
	if _, err = repo.SetBillingAccountBalance(ctx, 1, 25*testNanousdPerUSD, "admin-set", ""); err != nil {
		t.Fatalf("SetBillingAccountBalance() error = %v", err)
	}
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusActive, 15*testNanousdPerUSD)

	if _, err = repo.ExpireDueExpiringCredits(ctx, expiresAt.Add(time.Second), 100); err != nil {
		t.Fatalf("ExpireDueExpiringCredits() error = %v", err)
	}
	assertBillingBalance(t, db, 1, 10*testNanousdPerUSD)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)
}

func TestUsageRedemptionCodeCreatesFractionalDayExpiringCredit(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	if err := db.AutoMigrate(&models.RedemptionCode{}); err != nil {
		t.Fatalf("migrate redemption codes: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	maxRedemptions := 1
	code := models.RedemptionCode{
		CodeHash:               "limited-code-hash",
		Mode:                   domainbilling.RedemptionCodeModeUsage,
		RewardType:             domainbilling.RedemptionRewardTypeBalance,
		CreditNanousd:          20 * testNanousdPerUSD,
		BalanceDurationSeconds: 8640,
		MaxRedemptions:         &maxRedemptions,
		PerUserLimit:           1,
		Status:                 domainbilling.RedemptionCodeStatusActive,
		CreatedByUserID:        9,
		Description:            "fractional-day campaign",
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("create redemption code: %v", err)
	}

	result, err := repo.RedeemCode(ctx, repository.RedemptionApplyInput{
		CodeHash:       code.CodeHash,
		UserID:         1,
		CurrentMode:    domainbilling.RedemptionCodeModeUsage,
		RefNo:          "redeem-limited",
		SubscriptionAt: now,
	})
	if err != nil {
		t.Fatalf("RedeemCode() error = %v", err)
	}
	if result.Account == nil || result.Account.BalanceNanousd != 20*testNanousdPerUSD {
		t.Fatalf("unexpected redemption account: %+v", result.Account)
	}

	var credit models.ExpiringCredit
	if err := db.Where("user_id = ? AND ref_no = ?", 1, "redeem-limited").First(&credit).Error; err != nil {
		t.Fatalf("load redemption expiring credit: %v", err)
	}
	if credit.OriginalNanousd != 20*testNanousdPerUSD || credit.RemainingNanousd != 20*testNanousdPerUSD {
		t.Fatalf("unexpected redemption expiring credit: %+v", credit)
	}
	wantExpiresAt := now.Add(2*time.Hour + 24*time.Minute)
	if !credit.ExpiresAt.Equal(wantExpiresAt) {
		t.Fatalf("credit expires at = %v, want %v", credit.ExpiresAt, wantExpiresAt)
	}

	if _, err = repo.ExpireDueExpiringCredits(ctx, wantExpiresAt.Add(time.Second), 100); err != nil {
		t.Fatalf("ExpireDueExpiringCredits() error = %v", err)
	}
	assertBillingBalance(t, db, 1, 0)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)
}

func TestUsageRedemptionCodeSupportsLegacyIntegerDayDuration(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	if err := db.AutoMigrate(&models.RedemptionCode{}); err != nil {
		t.Fatalf("migrate redemption codes: %v", err)
	}
	repo := NewRepo(db)
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	maxRedemptions := 1
	code := models.RedemptionCode{
		CodeHash:       "legacy-duration-code-hash",
		Mode:           domainbilling.RedemptionCodeModeUsage,
		RewardType:     domainbilling.RedemptionRewardTypeBalance,
		CreditNanousd:  5 * testNanousdPerUSD,
		DurationDays:   2,
		MaxRedemptions: &maxRedemptions,
		PerUserLimit:   1,
		Status:         domainbilling.RedemptionCodeStatusActive,
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("create redemption code: %v", err)
	}

	if _, err := repo.RedeemCode(context.Background(), repository.RedemptionApplyInput{
		CodeHash:       code.CodeHash,
		UserID:         1,
		CurrentMode:    domainbilling.RedemptionCodeModeUsage,
		RefNo:          "redeem-legacy-duration",
		SubscriptionAt: now,
	}); err != nil {
		t.Fatalf("RedeemCode() error = %v", err)
	}

	var credit models.ExpiringCredit
	if err := db.Where("user_id = ?", 1).First(&credit).Error; err != nil {
		t.Fatalf("load redemption expiring credit: %v", err)
	}
	if want := now.Add(48 * time.Hour); !credit.ExpiresAt.Equal(want) {
		t.Fatalf("credit expires at = %v, want %v", credit.ExpiresAt, want)
	}
}

func TestUsageRedemptionCodeWithoutDurationRemainsPermanent(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	if err := db.AutoMigrate(&models.RedemptionCode{}); err != nil {
		t.Fatalf("migrate redemption codes: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	maxRedemptions := 1
	code := models.RedemptionCode{
		CodeHash:       "permanent-code-hash",
		Mode:           domainbilling.RedemptionCodeModeUsage,
		RewardType:     domainbilling.RedemptionRewardTypeBalance,
		CreditNanousd:  20 * testNanousdPerUSD,
		DurationDays:   0,
		MaxRedemptions: &maxRedemptions,
		PerUserLimit:   1,
		Status:         domainbilling.RedemptionCodeStatusActive,
	}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("create redemption code: %v", err)
	}

	if _, err := repo.RedeemCode(ctx, repository.RedemptionApplyInput{
		CodeHash:       code.CodeHash,
		UserID:         1,
		CurrentMode:    domainbilling.RedemptionCodeModeUsage,
		RefNo:          "redeem-permanent",
		SubscriptionAt: now,
	}); err != nil {
		t.Fatalf("RedeemCode() error = %v", err)
	}
	assertBillingBalance(t, db, 1, 20*testNanousdPerUSD)

	var creditCount int64
	if err := db.Model(&models.ExpiringCredit{}).Where("user_id = ?", 1).Count(&creditCount).Error; err != nil {
		t.Fatalf("count expiring credits: %v", err)
	}
	if creditCount != 0 {
		t.Fatalf("expiring credit count = %d, want 0", creditCount)
	}
}

func createBillingAccount(t *testing.T, db *gorm.DB, userID uint, balance int64) {
	t.Helper()
	if err := db.Create(&models.BillingAccount{
		UserID:         userID,
		Currency:       "USD",
		BalanceNanousd: balance,
		Status:         "active",
	}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
}

func settleUsage(t *testing.T, repo *Repo, userID uint, amount int64, modelName string) {
	t.Helper()
	now := time.Now()
	if err := repo.AddUsageAndSettleBalance(context.Background(), &domainbilling.UsageLedger{
		UserID:            userID,
		PlatformModelName: modelName,
		BillingAt:         now,
		UsageDate:         now,
		BilledCurrency:    "USD",
		BilledNanousd:     amount,
	}, nil); err != nil {
		t.Fatalf("AddUsageAndSettleBalance() error = %v", err)
	}
}

func assertBillingBalance(t *testing.T, db *gorm.DB, userID uint, want int64) {
	t.Helper()
	var account models.BillingAccount
	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		t.Fatalf("load billing account: %v", err)
	}
	if account.BalanceNanousd != want {
		t.Fatalf("balance = %d, want %d", account.BalanceNanousd, want)
	}
}

func assertExpiringCredit(t *testing.T, db *gorm.DB, creditID uint, status string, remaining int64) {
	t.Helper()
	var credit models.ExpiringCredit
	if err := db.First(&credit, creditID).Error; err != nil {
		t.Fatalf("load expiring credit: %v", err)
	}
	if credit.Status != status || credit.RemainingNanousd != remaining {
		t.Fatalf("credit = %+v, want status=%s remaining=%d", credit, status, remaining)
	}
}

func grantTestCredit(ctx context.Context, repo *Repo, userID uint, amount int64, expires time.Time, actor uint, ref, description string) (*domainbilling.ExpiringCredit, *domainbilling.BillingAccount, error) {
	return repo.GrantExpiringCredit(ctx, repository.ExpiringCreditGrantInput{
		UserID: userID, AmountNanousd: amount, DurationSeconds: int64(time.Until(expires).Round(time.Second) / time.Second),
		CreatedByUserID: actor, RefNo: ref, Description: description,
	})
}
