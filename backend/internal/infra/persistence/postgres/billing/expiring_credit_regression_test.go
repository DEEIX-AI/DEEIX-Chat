package billing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

func grantRegressionCredit(t *testing.T, repo *Repo, userID uint) *domainbilling.ExpiringCredit {
	t.Helper()
	credit, _, err := repo.GrantExpiringCredit(t.Context(), repository.ExpiringCreditGrantInput{
		UserID: userID, AmountNanousd: 100, DurationSeconds: 8640, CreatedByUserID: 9, RefNo: "campaign",
	})
	if err != nil {
		t.Fatal(err)
	}
	return credit
}

func makeCreditDue(t *testing.T, db *gorm.DB, creditID uint) {
	t.Helper()
	if err := db.Model(&models.ExpiringCredit{}).Where("id = ?", creditID).
		Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestExpiringGrantConcurrentRetryIsIdempotent(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	input := repository.ExpiringCreditGrantInput{UserID: 1, AmountNanousd: 100, DurationSeconds: 8640, CreatedByUserID: 9, RefNo: "retry-key"}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := repo.GrantExpiringCredit(context.Background(), input)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertBillingBalance(t, db, 1, 100)
	var count int64
	if err := db.Model(&models.BalanceTransaction{}).Where("type = ?", domainbilling.BalanceTransactionTypeExpiringCreditGrant).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("grant ledger entries = %d, want 1", count)
	}
	input.DurationSeconds *= 2
	if _, _, err := repo.GrantExpiringCredit(t.Context(), input); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("changed duration: %v", err)
	}
	assertBillingBalance(t, db, 1, 100)
}

func TestRaiseReservationRechecksExpiredCreditEvenBelowExistingBudget(t *testing.T) {
	for _, required := range []int64{30, 90} {
		t.Run(time.Duration(required).String(), func(t *testing.T) {
			db := openBillingSQLiteTestDB(t)
			repo := NewRepo(db)
			credit := grantRegressionCredit(t, repo, 1)
			_, err := repo.ReserveUsageBalance(t.Context(), domainbilling.UsageBalanceReservationRequest{UserID: 1, RefNo: "run", Mode: "usage", RequestedNanousd: 50})
			if err != nil {
				t.Fatal(err)
			}
			makeCreditDue(t, db, credit.ID)
			if err = repo.RaiseUsageBalanceReservation(t.Context(), 1, "run", required); !errors.Is(err, repository.ErrInsufficientBalance) {
				t.Fatalf("raise = %v", err)
			}
			account, err := repo.GetOrCreateBillingAccount(t.Context(), 1)
			if err != nil || account.BalanceNanousd != 0 {
				t.Fatalf("account = %+v, error = %v", account, err)
			}
		})
	}
}

func TestAccountListExpiresOnlyRequestedUsers(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	one, two := grantRegressionCredit(t, repo, 1), grantRegressionCredit(t, repo, 2)
	makeCreditDue(t, db, one.ID)
	makeCreditDue(t, db, two.ID)
	accounts, err := repo.ListBillingAccountsByUserIDs(t.Context(), []uint{1})
	if err != nil || len(accounts) != 1 || accounts[0].BalanceNanousd != 0 {
		t.Fatalf("accounts = %+v, error = %v", accounts, err)
	}
	assertExpiringCredit(t, db, two.ID, domainbilling.ExpiringCreditStatusActive, 100)
	assertBillingBalance(t, db, 2, 100)
}

func TestExpiryAndLedgerRollbackTogether(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	credit := grantRegressionCredit(t, repo, 1)
	makeCreditDue(t, db, credit.ID)
	if err := db.Exec(`CREATE TRIGGER fail_expiry_ledger BEFORE INSERT ON billing_balance_transactions
		WHEN NEW.type = 'expiring_credit_expire' BEGIN SELECT RAISE(ABORT, 'injected ledger failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	count, err := repo.ExpireDueExpiringCredits(t.Context(), time.Now(), 10)
	if err == nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	assertBillingBalance(t, db, 1, 100)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusActive, 100)
}

func TestConcurrentSweepDeductsOnce(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	credit := grantRegressionCredit(t, repo, 1)
	makeCreditDue(t, db, credit.ID)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.ExpireDueExpiringCredits(context.Background(), time.Now(), 10); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	assertBillingBalance(t, db, 1, 0)
	var count int64
	if err := db.Model(&models.BalanceTransaction{}).Where("type = ?", domainbilling.BalanceTransactionTypeExpiringCreditExpire).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expiry entries=%d", count)
	}
}

func TestSettlementAfterExpiryUsesPermanentBalanceAndIsIdempotent(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	createBillingAccount(t, db, 1, 200)
	credit := grantRegressionCredit(t, repo, 1)
	reservation, err := repo.ReserveUsageBalance(t.Context(), domainbilling.UsageBalanceReservationRequest{UserID: 1, RefNo: "late-run", Mode: "usage", RequestedNanousd: 50})
	if err != nil {
		t.Fatal(err)
	}
	makeCreditDue(t, db, credit.ID)
	usage := &domainbilling.UsageLedger{UserID: 1, PlatformModelName: "mcp:test", BillingAt: time.Now(), UsageDate: time.Now(), BilledCurrency: "USD", BilledNanousd: 30}
	for range 2 {
		if err := repo.AddUsageAndSettleBalance(t.Context(), usage, reservation); err != nil {
			t.Fatal(err)
		}
	}
	assertBillingBalance(t, db, 1, 170)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)
}

func TestPeriodOverageConsumesExpiringCredit(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	credit := grantRegressionCredit(t, repo, 1)
	now := time.Now()
	usage := &domainbilling.UsageLedger{UserID: 1, PlatformModelName: "period-test", BillingAt: now, UsageDate: now, BilledCurrency: "USD", BilledNanousd: 70}
	if err := repo.AddPeriodUsageAndSettleOverage(t.Context(), usage, now.Add(-time.Hour), now.Add(time.Hour), 40, nil); err != nil {
		t.Fatal(err)
	}
	assertBillingBalance(t, db, 1, 70)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusActive, 70)
}

func TestExpiringGrantPaysDebtWithoutExpiringFutureTopups(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	repo := NewRepo(db)
	createBillingAccount(t, db, 1, -50)
	credit := grantRegressionCredit(t, repo, 1)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusActive, 50)
	assertBillingBalance(t, db, 1, 50)
	// Adding permanent credit must not revive the 50 already used to pay debt.
	if _, err := repo.SetBillingAccountBalance(t.Context(), 1, 150, "permanent", ""); err != nil {
		t.Fatal(err)
	}
	makeCreditDue(t, db, credit.ID)
	if _, err := repo.ExpireDueExpiringCredits(t.Context(), time.Now(), 10); err != nil {
		t.Fatal(err)
	}
	assertBillingBalance(t, db, 1, 100)
}

func TestPaymentAfterExpiryUsesCurrentTimeNotPaymentTimestamp(t *testing.T) {
	db := openBillingSQLiteTestDB(t)
	if err := db.AutoMigrate(&models.PaymentOrder{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	credit := grantRegressionCredit(t, repo, 1)
	makeCreditDue(t, db, credit.ID)
	order := models.PaymentOrder{OrderNo: "delayed-payment", UserID: 1,
		OrderType: domainbilling.PaymentOrderTypeTopUp, Status: domainbilling.PaymentStatusPending,
		CreditNanousd: 60}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	if _, credited, err := repo.MarkPaymentOrderPaidAndCreditBalance(t.Context(), order.OrderNo, "provider-id", time.Now().Add(-time.Hour)); err != nil || !credited {
		t.Fatalf("credited=%v err=%v", credited, err)
	}
	assertBillingBalance(t, db, 1, 60)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)
}
