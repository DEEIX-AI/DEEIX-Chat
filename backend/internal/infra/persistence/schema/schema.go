package schema

import (
	"errors"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
)

// Models returns all persistent Gorm models used by the application.
func Models() []interface{} {
	return []interface{}{
		&model.User{},
		&model.UserContactVerification{},
		&model.UserCredential{},
		&model.UserSession{},
		&model.UserAuthEvent{},
		&model.AuthIdentityProvider{},
		&model.UserIdentity{},
		&model.UserTwoFactor{},
		&model.TrustedDevice{},
		&model.LLMUpstream{},
		&model.LLMUpstreamModel{},
		&model.LLMPlatformModel{},
		&model.LLMPlatformModelRoute{},
		&model.MCPServer{},
		&model.MCPTool{},
		&model.Conversation{},
		&model.ConversationProject{},
		&model.ConversationShare{},
		&model.Message{},
		&model.ConversationMessageFeedback{},
		&model.Attachment{},
		&model.FileObject{},
		&model.UserStorageQuota{},
		&model.ConversationRun{},
		&model.ChatRunEvent{},
		&model.ChatContextRecord{},
		&model.UserMemory{},
		&model.BillingPlan{},
		&model.BillingPrice{},
		&model.Subscription{},
		&model.PaymentOrder{},
		&model.BillingAccount{},
		&model.BalanceTransaction{},
		&model.UsageReservation{},
		&model.RedemptionCode{},
		&model.Redemption{},
		&model.ModelPricing{},
		&model.UsageLedger{},
		&model.AuditLog{},
		&model.SystemEvent{},
		&model.Announcement{},
		&model.AnnouncementUserState{},
		&model.PromptPreset{},
		&model.Skill{},
		&model.SystemSetting{},
		&model.UserSetting{},
		&model.FileChunk{},
		&model.MessageChunk{},
		&model.PermissionGroup{},
		&model.PermissionGroupModelAccess{},
		&model.PermissionGroupModelRule{},
		&model.PermissionGroupUserAccess{},
	}
}

// Migrate creates or updates the baseline schema with Gorm's portable migrator.
func Migrate(db *gorm.DB) error {
	for _, item := range Models() {
		if db.Migrator().HasTable(item) {
			continue
		}
		if err := db.Migrator().CreateTable(item); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(Models()...); err != nil {
		return err
	}
	return backfillUsageLedgerBillingAt(db)
}

func backfillUsageLedgerBillingAt(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.UsageLedger{}) || !db.Migrator().HasColumn(&model.UsageLedger{}, "billing_at") {
		return nil
	}
	return db.Model(&model.UsageLedger{}).
		Where("billing_at IS NULL").
		Update("billing_at", gorm.Expr("created_at")).Error
}

// CleanupRemovedColumns drops columns that were removed from the Gorm models.
func CleanupRemovedColumns(db *gorm.DB) error {
	if err := dropColumns(db, &model.PromptPreset{}, []string{"use_count", "last_used_at", "category", "tags_json"}); err != nil {
		return err
	}
	if err := dropColumns(db, &model.Skill{}, []string{"content", "sections_json"}); err != nil {
		return err
	}
	return nil
}

func dropColumns(db *gorm.DB, table interface{}, columns []string) error {
	if !db.Migrator().HasTable(table) {
		return nil
	}
	for _, column := range columns {
		if !db.Migrator().HasColumn(table, column) {
			continue
		}
		if err := db.Migrator().DropColumn(table, column); err != nil {
			return err
		}
	}
	return nil
}

// SeedLLMSettings inserts default LLM runtime settings if they do not exist.
func SeedLLMSettings(db *gorm.DB) error {
	settings := []model.SystemSetting{
		{
			Namespace:   "llm",
			Key:         "circuit_breaker.error_classification",
			Value:       `{"circuit_errors":["5xx","timeout","connection_error"],"rate_limit_errors":["429"],"ignore_errors":["4xx"]}`,
			ValueType:   "json",
			Description: "熔断错误分类配置",
		},
		{
			Namespace:   "llm",
			Key:         "circuit_breaker.defaults",
			Value:       `{"model_failure_threshold":5,"model_duration_min":15,"model_window_min":3,"upstream_failure_threshold":20,"upstream_model_threshold":3,"upstream_threshold_logic":"or","upstream_duration_min":30,"upstream_window_min":5}`,
			ValueType:   "json",
			Description: "熔断默认参数",
		},
		{
			Namespace:   "llm",
			Key:         "rate_limit.defaults",
			Value:       `{"backoff_base_sec":5,"backoff_max_sec":60,"backoff_multiplier":2}`,
			ValueType:   "json",
			Description: "限流退避默认参数",
		},
		{
			Namespace:   "llm",
			Key:         "load_balance.defaults",
			Value:       `{"algorithm":"weighted_random"}`,
			ValueType:   "json",
			Description: "负载均衡默认参数",
		},
	}

	for i := range settings {
		if err := db.Where("namespace = ? AND key = ?", settings[i].Namespace, settings[i].Key).
			FirstOrCreate(&settings[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedPermissionGroups inserts the built-in default permission group if it does not exist.
func SeedPermissionGroups(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		defaultGroup, err := ensureSingleDefaultPermissionGroup(tx)
		if err != nil {
			return err
		}
		if err := clearDefaultPermissionGroupUsers(tx); err != nil {
			return err
		}
		return seedInitialDefaultModelAccessRule(tx, defaultGroup.ID)
	})
}

func ensureSingleDefaultPermissionGroup(db *gorm.DB) (*model.PermissionGroup, error) {
	defaultGroups := make([]model.PermissionGroup, 0)
	if err := db.Where("is_default = ?", true).Order("id ASC").Find(&defaultGroups).Error; err != nil {
		return nil, err
	}
	if len(defaultGroups) == 0 {
		defaultGroup := model.PermissionGroup{
			Name:        "Default",
			Description: "All users implicitly belong to this group",
			IsDefault:   true,
		}
		if err := db.Create(&defaultGroup).Error; err != nil {
			return nil, err
		}
		return &defaultGroup, nil
	}
	defaultGroup := defaultGroups[0]
	if len(defaultGroups) > 1 {
		if err := db.Model(&model.PermissionGroup{}).
			Where("is_default = ? AND id <> ?", true, defaultGroup.ID).
			Update("is_default", false).Error; err != nil {
			return nil, err
		}
	}
	return &defaultGroup, nil
}

func seedInitialDefaultModelAccessRule(db *gorm.DB, defaultGroupID uint) error {
	if defaultGroupID == 0 {
		return nil
	}
	var manualCount int64
	if err := db.Model(&model.PermissionGroupModelAccess{}).Count(&manualCount).Error; err != nil {
		return err
	}
	if manualCount > 0 {
		return nil
	}
	var ruleCount int64
	if err := db.Model(&model.PermissionGroupModelRule{}).Count(&ruleCount).Error; err != nil {
		return err
	}
	if ruleCount > 0 {
		return nil
	}
	rule := model.PermissionGroupModelRule{
		GroupID:  defaultGroupID,
		RuleType: domainchannel.PermissionGroupModelRuleAll,
		Value:    "",
	}
	return db.Where(rule).FirstOrCreate(&rule).Error
}

// SeedBillingCatalog inserts the default plans and prices if the billing catalog is empty.
func SeedBillingCatalog(db *gorm.DB) error {
	defaultGroupID, err := defaultPermissionGroupID(db)
	if err != nil {
		return err
	}
	var planCount int64
	if err := db.Model(&model.BillingPlan{}).Count(&planCount).Error; err != nil {
		return err
	}
	var priceCount int64
	if err := db.Model(&model.BillingPrice{}).Count(&priceCount).Error; err != nil {
		return err
	}
	if planCount > 0 || priceCount > 0 {
		return bindBillingPlansToDefaultGroup(db, defaultGroupID)
	}

	plans := []model.BillingPlan{
		{
			Code:                "free",
			Name:                "Free",
			Description:         "默认免费套餐",
			FeatureJSON:         `{"priority":"shared"}`,
			PeriodCreditNanousd: 1000000000,
			DiscountPercent:     0,
			SortOrder:           10,
			IsActive:            true,
			PermissionGroupID:   copyUintPointer(defaultGroupID),
		},
		{
			Code:                "pro",
			Name:                "Pro",
			Description:         "轻度使用套餐",
			FeatureJSON:         `{"priority":"standard"}`,
			PeriodCreditNanousd: 30000000000,
			DiscountPercent:     0,
			SortOrder:           20,
			IsActive:            true,
			PermissionGroupID:   copyUintPointer(defaultGroupID),
		},
		{
			Code:                "max",
			Name:                "Max",
			Description:         "中度使用套餐",
			FeatureJSON:         `{"priority":"advanced"}`,
			PeriodCreditNanousd: 75000000000,
			DiscountPercent:     0,
			SortOrder:           30,
			IsActive:            true,
			PermissionGroupID:   copyUintPointer(defaultGroupID),
		},
		{
			Code:                "ultra",
			Name:                "Ultra",
			Description:         "重度使用套餐",
			FeatureJSON:         `{"priority":"premium"}`,
			PeriodCreditNanousd: 300000000000,
			DiscountPercent:     0,
			SortOrder:           40,
			IsActive:            true,
			PermissionGroupID:   copyUintPointer(defaultGroupID),
		},
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&plans).Error; err != nil {
			return err
		}

		planIDByCode := make(map[string]uint, len(plans))
		for _, item := range plans {
			planIDByCode[item.Code] = item.ID
		}

		prices := []model.BillingPrice{
			{PlanID: planIDByCode["free"], Code: "free-default", BillingInterval: model.BillingIntervalLifetime, Currency: "USD", AmountCents: 0, IsActive: true, IsDefault: true},
			{PlanID: planIDByCode["pro"], Code: "pro-monthly", BillingInterval: model.BillingIntervalMonth, Currency: "USD", AmountCents: 2000, IsActive: true, IsDefault: true},
			{PlanID: planIDByCode["max"], Code: "max-monthly", BillingInterval: model.BillingIntervalMonth, Currency: "USD", AmountCents: 5000, IsActive: true, IsDefault: true},
			{PlanID: planIDByCode["ultra"], Code: "ultra-monthly", BillingInterval: model.BillingIntervalMonth, Currency: "USD", AmountCents: 20000, IsActive: true, IsDefault: true},
		}
		return tx.Create(&prices).Error
	})
}

func defaultPermissionGroupID(db *gorm.DB) (*uint, error) {
	var group model.PermissionGroup
	if err := db.Where("is_default = ?", true).Order("id ASC").First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &group.ID, nil
}

func bindBillingPlansToDefaultGroup(db *gorm.DB, defaultGroupID *uint) error {
	if defaultGroupID == nil {
		return nil
	}
	return db.Model(&model.BillingPlan{}).
		Where("permission_group_id IS NULL").
		Update("permission_group_id", *defaultGroupID).Error
}

func clearDefaultPermissionGroupUsers(db *gorm.DB) error {
	defaultGroupIDs := db.Model(&model.PermissionGroup{}).
		Select("id").
		Where("is_default = ?", true)
	return db.Where("group_id IN (?)", defaultGroupIDs).
		Delete(&model.PermissionGroupUserAccess{}).Error
}

func copyUintPointer(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
