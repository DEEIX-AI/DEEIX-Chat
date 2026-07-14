package billing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestReserveUsageBalanceSerializesConcurrentPostgresRequests(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL billing concurrency integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve postgres db: %v", err)
	}
	sqlDB.SetMaxOpenConns(8)
	defer sqlDB.Close()
	if err = db.AutoMigrate(&model.UsageLedger{}, &model.BillingAccount{}, &model.BalanceTransaction{}, &model.UsageReservation{}); err != nil {
		t.Fatalf("migrate billing tables: %v", err)
	}

	userID := uint(time.Now().UnixNano()%1_000_000_000) + 1
	if err = db.Create(&model.BillingAccount{UserID: userID, Currency: "USD", BalanceNanousd: 100, Status: "active"}).Error; err != nil {
		t.Fatalf("create billing account: %v", err)
	}
	defer func() {
		_ = db.Where("user_id = ?", userID).Delete(&model.UsageReservation{}).Error
		_ = db.Where("user_id = ?", userID).Delete(&model.BalanceTransaction{}).Error
		_ = db.Where("user_id = ?", userID).Delete(&model.BillingAccount{}).Error
	}()

	repo := NewRepo(db)
	start := make(chan struct{})
	requestCount := domainbilling.UsageReservationMaxActivePerUser + 1
	results := make(chan error, requestCount)
	var wg sync.WaitGroup
	for index := 0; index < requestCount; index++ {
		refNo := fmt.Sprintf("postgres_run_%d", index)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, reserveErr := repo.ReserveUsageBalance(context.Background(), domainbilling.UsageBalanceReservationRequest{
				UserID:           userID,
				RefNo:            refNo,
				Mode:             "usage",
				RequestedNanousd: 0,
			})
			results <- reserveErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var successCount int
	var limitCount int
	for reserveErr := range results {
		switch {
		case reserveErr == nil:
			successCount++
		case errors.Is(reserveErr, repository.ErrUsageReservationLimitExceeded):
			limitCount++
		default:
			t.Fatalf("concurrent reserve error = %v", reserveErr)
		}
	}
	if successCount != domainbilling.UsageReservationMaxActivePerUser || limitCount != 1 {
		t.Fatalf("concurrent reserve results = success %d, limited %d; want %d/1", successCount, limitCount, domainbilling.UsageReservationMaxActivePerUser)
	}
}
