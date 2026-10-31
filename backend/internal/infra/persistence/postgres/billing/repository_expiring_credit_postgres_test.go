package billing

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestExpiringCreditPostgresConcurrentGrantAndSweep(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL expiring-credit concurrency tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&models.BillingAccount{}, &models.BalanceTransaction{}, &models.ExpiringCredit{}); err != nil {
		t.Fatal(err)
	}
	userID := uint(time.Now().UnixNano()%1_000_000_000) + 1
	t.Cleanup(func() {
		for _, model := range []any{&models.ExpiringCredit{}, &models.BalanceTransaction{}, &models.BillingAccount{}} {
			if err := db.Where("user_id = ?", userID).Delete(model).Error; err != nil {
				t.Error(err)
			}
		}
	})
	input := repository.ExpiringCreditGrantInput{UserID: userID, CreatedByUserID: 9, AmountNanousd: 100, DurationSeconds: 8640, RefNo: "same-grant"}
	start := make(chan struct{})
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			<-start
			// Independent Repo values use separate connections from the real pool.
			_, _, err := NewRepo(db).GrantExpiringCredit(context.Background(), input)
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertBillingBalance(t, db, userID, 100)
	var credit models.ExpiringCredit
	if err := db.Where("user_id = ?", userID).First(&credit).Error; err != nil {
		t.Fatal(err)
	}
	input.AmountNanousd++
	if _, _, err := NewRepo(db).GrantExpiringCredit(t.Context(), input); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("changed retry accepted: %v", err)
	}
	if err := db.Model(&credit).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	for range 4 {
		wg.Go(func() {
			if _, err := NewRepo(db).ExpireDueExpiringCredits(context.Background(), time.Now(), 1000); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	assertBillingBalance(t, db, userID, 0)
	assertExpiringCredit(t, db, credit.ID, domainbilling.ExpiringCreditStatusExpired, 0)
	for _, kind := range []string{domainbilling.BalanceTransactionTypeExpiringCreditGrant, domainbilling.BalanceTransactionTypeExpiringCreditExpire} {
		var count int64
		if err := db.Model(&models.BalanceTransaction{}).Where("user_id = ? AND type = ?", userID, kind).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s ledger entries = %d, want 1", kind, count)
		}
	}

	t.Run("expiration after account lock wait", func(t *testing.T) {
		grant := input
		grant.AmountNanousd, grant.RefNo = 100, "lock-wait"
		credit, _, err := NewRepo(db).GrantExpiringCredit(t.Context(), grant)
		if err != nil {
			t.Fatal(err)
		}
		expiresAt := time.Now().Add(2 * time.Second)
		if err := db.Model(&models.ExpiringCredit{}).Where("id = ?", credit.ID).Update("expires_at", expiresAt).Error; err != nil {
			t.Fatal(err)
		}
		lock := db.Begin()
		if lock.Error != nil {
			t.Fatal(lock.Error)
		}
		defer func() { _ = lock.Rollback().Error }()
		if _, err := lockBillingAccount(lock, userID); err != nil {
			t.Fatal(err)
		}
		type readResult struct {
			account *domainbilling.BillingAccount
			err     error
		}
		results := make(chan readResult, 1)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		go func() {
			account, err := NewRepo(db).GetOrCreateBillingAccount(ctx, userID)
			results <- readResult{account, err}
		}()
		// Observe the actual PostgreSQL row-lock wait before crossing expiry.
		deadline := time.Now().Add(time.Second)
		for {
			var blocked int64
			if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%billing_accounts%'").Scan(&blocked).Error; err != nil {
				t.Fatal(err)
			}
			if blocked > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("account reader did not reach the row lock")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !time.Now().Before(expiresAt) {
			t.Fatal("reader reached row lock after expiry")
		}
		time.Sleep(time.Until(expiresAt) + 25*time.Millisecond)
		if err := lock.Commit().Error; err != nil {
			t.Fatal(err)
		}
		result := <-results
		if result.err != nil || result.account == nil || result.account.BalanceNanousd != 0 {
			t.Fatalf("expired credit survived lock wait: account=%+v err=%v", result.account, result.err)
		}
	})
}
