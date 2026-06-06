package schema

import (
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
		&model.RedemptionCode{},
		&model.Redemption{},
		&model.ModelPricing{},
		&model.UsageLedger{},
		&model.AuditLog{},
		&model.SystemEvent{},
		&model.Announcement{},
		&model.AnnouncementUserState{},
		&model.SystemSetting{},
		&model.UserSetting{},
		&model.FileChunk{},
		&model.MessageChunk{},
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
	return db.AutoMigrate(Models()...)
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

// SeedBillingCatalog inserts the default plans and prices if the billing catalog is empty.
func SeedBillingCatalog(db *gorm.DB) error {
	var planCount int64
	if err := db.Model(&model.BillingPlan{}).Count(&planCount).Error; err != nil {
		return err
	}
	var priceCount int64
	if err := db.Model(&model.BillingPrice{}).Count(&priceCount).Error; err != nil {
		return err
	}
	if planCount > 0 || priceCount > 0 {
		return nil
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
