package billing

import (
	"context"
	"errors"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPlanSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&models.BillingPlan{},
		&models.BillingPrice{},
		&models.Subscription{},
		&models.PaymentOrder{},
		&models.RedemptionCode{},
		&models.Redemption{},
	); err != nil {
		t.Fatalf("migrate plan tables: %v", err)
	}
	return db
}

func activePricesByInterval(t *testing.T, db *gorm.DB, planID uint) map[string]models.BillingPrice {
	t.Helper()
	var prices []models.BillingPrice
	if err := db.Where("plan_id = ? AND is_active = ?", planID, true).Find(&prices).Error; err != nil {
		t.Fatalf("list prices: %v", err)
	}
	result := make(map[string]models.BillingPrice, len(prices))
	for _, item := range prices {
		result[item.BillingInterval] = item
	}
	return result
}

func TestSavePlanWithPricesCreatesInactivePlanAndPrices(t *testing.T) {
	db := openPlanSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	plan := &domainbilling.Plan{Code: "team", Name: "Team", SortOrder: 50, IsActive: false}
	err := repo.SavePlanWithPrices(ctx, plan, []domainbilling.Price{
		{BillingInterval: "month", Currency: "USD", AmountCents: 3000, IsDefault: true},
		{BillingInterval: "year", Currency: "USD", AmountCents: 30000},
	})
	if err != nil {
		t.Fatalf("SavePlanWithPrices() error = %v", err)
	}
	if plan.ID == 0 || plan.IsActive {
		t.Fatalf("plan = %+v, want persisted inactive plan", plan)
	}
	prices := activePricesByInterval(t, db, plan.ID)
	if len(prices) != 2 || !prices["month"].IsDefault || prices["year"].IsDefault || prices["year"].Code != "team-year" {
		t.Fatalf("prices = %+v", prices)
	}

	duplicate := &domainbilling.Plan{Code: "team", Name: "Team 2", IsActive: true}
	err = repo.SavePlanWithPrices(ctx, duplicate, []domainbilling.Price{{BillingInterval: "month", IsDefault: true}})
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate code err = %v, want ErrDuplicate", err)
	}
}

func TestSavePlanWithPricesReusesLegacyRowsAndDeactivatesRemoved(t *testing.T) {
	db := openPlanSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	plan := models.BillingPlan{Code: "pro", Name: "Pro", IsActive: true}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	legacy := []models.BillingPrice{
		{PlanID: plan.ID, Code: "pro-monthly", BillingInterval: "month", Currency: "USD", AmountCents: 2000, IsActive: true, IsDefault: true},
		{PlanID: plan.ID, Code: "pro-default", BillingInterval: "month", Currency: "USD", AmountCents: 2500, IsActive: true},
		{PlanID: plan.ID, Code: "pro-lifetime", BillingInterval: "lifetime", Currency: "USD", AmountCents: 50000, IsActive: true},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create prices: %v", err)
	}

	domainPlan := toPlanDomain(plan)
	err := repo.SavePlanWithPrices(ctx, &domainPlan, []domainbilling.Price{
		{BillingInterval: "month", Currency: "USD", AmountCents: 2200},
		{BillingInterval: "year", Currency: "USD", AmountCents: 22000, IsDefault: true},
	})
	if err != nil {
		t.Fatalf("SavePlanWithPrices() error = %v", err)
	}

	prices := activePricesByInterval(t, db, plan.ID)
	if len(prices) != 2 {
		t.Fatalf("active prices = %+v, want month and year", prices)
	}
	if prices["month"].ID != legacy[0].ID || prices["month"].AmountCents != 2200 || prices["month"].IsDefault {
		t.Fatalf("month price = %+v, want reused default row updated to non-default", prices["month"])
	}
	if !prices["year"].IsDefault {
		t.Fatalf("year price should be default: %+v", prices["year"])
	}
	var total int64
	db.Model(&models.BillingPrice{}).Where("plan_id = ?", plan.ID).Count(&total)
	if total != 4 {
		t.Fatalf("price rows = %d, want 4 (removed prices kept inactive)", total)
	}
}

func TestDeletePlanRejectsReferencedPlanAndFreesCode(t *testing.T) {
	db := openPlanSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	used := models.BillingPlan{Code: "used", Name: "Used", IsActive: true}
	unused := models.BillingPlan{Code: "unused", Name: "Unused", IsActive: true}
	if err := db.Create(&[]*models.BillingPlan{&used, &unused}).Error; err != nil {
		t.Fatalf("create plans: %v", err)
	}
	if err := db.Create(&models.BillingPrice{PlanID: unused.ID, Code: "unused-month", BillingInterval: "month", IsActive: true, IsDefault: true}).Error; err != nil {
		t.Fatalf("create price: %v", err)
	}
	if err := db.Create(&models.PaymentOrder{OrderNo: "order-1", UserID: 1, PlanID: used.ID, PriceID: 1}).Error; err != nil {
		t.Fatalf("create payment order: %v", err)
	}

	if err := repo.DeletePlan(ctx, used.ID); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("delete referenced plan err = %v, want ErrConflict", err)
	}
	if err := repo.DeletePlan(ctx, unused.ID); err != nil {
		t.Fatalf("delete unused plan: %v", err)
	}
	var priceCount int64
	db.Unscoped().Model(&models.BillingPrice{}).Where("plan_id = ?", unused.ID).Count(&priceCount)
	if priceCount != 0 {
		t.Fatalf("prices left after delete = %d", priceCount)
	}

	recreated := &domainbilling.Plan{Code: "unused", Name: "Again", IsActive: true}
	if err := repo.SavePlanWithPrices(ctx, recreated, []domainbilling.Price{{BillingInterval: "month", IsDefault: true}}); err != nil {
		t.Fatalf("recreate deleted code: %v", err)
	}
	if err := repo.DeletePlan(ctx, 9999); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("delete missing err = %v, want ErrNotFound", err)
	}
}
