package billing

import (
	"context"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openRedemptionSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:billing_redemption_records?mode=memory&cache=shared"), &gorm.Config{})
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

	if err := db.AutoMigrate(&model.Redemption{}, &model.RedemptionCode{}, &model.BillingPlan{}, &model.BalanceTransaction{}); err != nil {
		t.Fatalf("migrate redemption tables: %v", err)
	}
	return db
}

func TestListRedemptionsJoinsCodeAndBalanceContext(t *testing.T) {
	db := openRedemptionSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	usageCode := model.RedemptionCode{
		CodeHash:    "hash-usage",
		CodeHint:    "AAAA***0001",
		Mode:        domainbilling.RedemptionCodeModeUsage,
		RewardType:  domainbilling.RedemptionRewardTypeBalance,
		Status:      domainbilling.RedemptionCodeStatusActive,
		Description: "邀请返利",
	}
	deletedCode := model.RedemptionCode{
		CodeHash:   "hash-period",
		CodeHint:   "BBBB***0002",
		Mode:       domainbilling.RedemptionCodeModePeriod,
		RewardType: domainbilling.RedemptionRewardTypeSubscription,
		Status:     domainbilling.RedemptionCodeStatusDeleted,
	}
	if err := db.Create(&usageCode).Error; err != nil {
		t.Fatalf("create usage code: %v", err)
	}
	if err := db.Create(&deletedCode).Error; err != nil {
		t.Fatalf("create deleted code: %v", err)
	}
	plan := model.BillingPlan{Code: "pro", Name: "Pro"}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	balanceTx := model.BalanceTransaction{
		UserID:              7,
		Type:                domainbilling.BalanceTransactionTypeRedemption,
		AmountNanousd:       5_000_000_000,
		BalanceAfterNanousd: 12_000_000_000,
	}
	if err := db.Create(&balanceTx).Error; err != nil {
		t.Fatalf("create balance transaction: %v", err)
	}

	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	redemptions := []model.Redemption{
		{
			CodeID:               usageCode.ID,
			UserID:               7,
			Mode:                 domainbilling.RedemptionCodeModeUsage,
			RewardType:           domainbilling.RedemptionRewardTypeBalance,
			CreditNanousd:        5_000_000_000,
			BalanceTransactionID: balanceTx.ID,
			RefNo:                "redemption_7_1",
		},
		{
			CodeID:         deletedCode.ID,
			UserID:         8,
			Mode:           domainbilling.RedemptionCodeModePeriod,
			RewardType:     domainbilling.RedemptionRewardTypeSubscription,
			PlanID:         plan.ID,
			SubscriptionID: 33,
			RefNo:          "redemption_8_1",
		},
		{
			CodeID:     usageCode.ID,
			UserID:     8,
			Mode:       domainbilling.RedemptionCodeModeUsage,
			RewardType: domainbilling.RedemptionRewardTypeBalance,
			RefNo:      "redemption_8_2",
		},
	}
	for index := range redemptions {
		redemptions[index].CreatedAt = base.Add(time.Duration(index) * time.Minute)
		if err := db.Create(&redemptions[index]).Error; err != nil {
			t.Fatalf("create redemption %d: %v", index, err)
		}
	}

	items, total, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions() error = %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("ListRedemptions() = %d/%d, want 3/3", len(items), total)
	}
	if items[0].Redemption.RefNo != "redemption_8_2" {
		t.Fatalf("default order first = %q, want redemption_8_2 (created desc)", items[0].Redemption.RefNo)
	}

	balanceItems, _, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{UserID: 7}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions(user) error = %v", err)
	}
	if len(balanceItems) != 1 {
		t.Fatalf("ListRedemptions(user) = %d, want 1", len(balanceItems))
	}
	record := balanceItems[0]
	if record.CodeHint != "AAAA***0001" || record.CodeDescription != "邀请返利" || record.CodeStatus != domainbilling.RedemptionCodeStatusActive {
		t.Fatalf("joined code context = %+v", record)
	}
	if record.BalanceAmountNanousd == nil || *record.BalanceAmountNanousd != 5_000_000_000 {
		t.Fatalf("BalanceAmountNanousd = %v, want 5e9", record.BalanceAmountNanousd)
	}
	if record.BalanceAfterNanousd == nil || *record.BalanceAfterNanousd != 12_000_000_000 {
		t.Fatalf("BalanceAfterNanousd = %v, want 12e9", record.BalanceAfterNanousd)
	}

	deletedItems, _, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{CodeID: deletedCode.ID}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions(deleted code) error = %v", err)
	}
	if len(deletedItems) != 1 {
		t.Fatalf("ListRedemptions(deleted code) = %d, want 1", len(deletedItems))
	}
	if deletedItems[0].CodeStatus != domainbilling.RedemptionCodeStatusDeleted {
		t.Fatalf("deleted code status = %q, want deleted", deletedItems[0].CodeStatus)
	}
	if deletedItems[0].PlanName != "Pro" {
		t.Fatalf("PlanName = %q, want Pro", deletedItems[0].PlanName)
	}
	if deletedItems[0].BalanceAmountNanousd != nil || deletedItems[0].BalanceAfterNanousd != nil {
		t.Fatalf("subscription redemption balance context = %+v, want nil", deletedItems[0])
	}

	keywordItems, _, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{Query: "bbbb"}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions(query) error = %v", err)
	}
	if len(keywordItems) != 1 || keywordItems[0].Redemption.RefNo != "redemption_8_1" {
		t.Fatalf("ListRedemptions(query bbbb) = %+v, want redemption_8_1", keywordItems)
	}

	// keyword(OR 条件)与其他 AND 筛选组合时必须整体括号，不能吞掉 user_id 条件。
	comboItems, comboTotal, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{Query: "aaaa", UserID: 7}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions(query+user) error = %v", err)
	}
	if comboTotal != 1 || len(comboItems) != 1 || comboItems[0].Redemption.RefNo != "redemption_7_1" {
		t.Fatalf("ListRedemptions(query aaaa + user 7) = %d/%d, want only redemption_7_1", len(comboItems), comboTotal)
	}

	rewardItems, _, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{RewardType: domainbilling.RedemptionRewardTypeSubscription}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions(rewardType) error = %v", err)
	}
	if len(rewardItems) != 1 || rewardItems[0].Redemption.RefNo != "redemption_8_1" {
		t.Fatalf("ListRedemptions(subscription) = %d, want redemption_8_1", len(rewardItems))
	}

	from := base.Add(90 * time.Second)
	rangedItems, _, err := repo.ListRedemptions(ctx, repository.RedemptionListFilter{CreatedFrom: &from, Sort: "created_asc"}, 0, 20)
	if err != nil {
		t.Fatalf("ListRedemptions(range) error = %v", err)
	}
	if len(rangedItems) != 1 || rangedItems[0].Redemption.RefNo != "redemption_8_2" {
		t.Fatalf("ListRedemptions(range) = %d, want redemption_8_2", len(rangedItems))
	}
}
