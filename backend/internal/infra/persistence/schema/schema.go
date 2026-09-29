package schema

import (
	"errors"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/channelconfig"
	"gorm.io/gorm"
)

// Models 返回应用使用的全部持久化 Gorm 模型。
func Models() []any {
	return []any{
		&models.User{},
		&models.UserContactVerification{},
		&models.UserCredential{},
		&models.UserSession{},
		&models.UserAuthEvent{},
		&models.AuthIdentityProvider{},
		&models.UserIdentity{},
		&models.UserTwoFactor{},
		&models.TrustedDevice{},
		&models.LLMUpstream{},
		&models.LLMUpstreamModel{},
		&models.LLMModelVendor{},
		&models.LLMModelDisplayGroup{},
		&models.LLMModelIconAsset{},
		&models.LLMPlatformModel{},
		&models.LLMPlatformModelRoute{},
		&models.MCPServer{},
		&models.MCPTool{},
		&models.Conversation{},
		&models.ConversationProject{},
		&models.ConversationShare{},
		&models.Message{},
		&models.ConversationMessageFeedback{},
		&models.Attachment{},
		&models.FileObject{},
		&models.UserStorageQuota{},
		&models.ConversationRun{},
		&models.ContentModerationEvent{},
		&models.ContentModerationDailyStat{},
		&models.ChatRunEvent{},
		&models.ChatContextRecord{},
		&models.UserMemory{},
		&models.BillingPlan{},
		&models.BillingPrice{},
		&models.Subscription{},
		&models.PaymentOrder{},
		&models.BillingAccount{},
		&models.BalanceTransaction{},
		&models.UsageReservation{},
		&models.RedemptionCode{},
		&models.Redemption{},
		&models.ModelPricing{},
		&models.UsageLedger{},
		&models.AuditLog{},
		&models.Announcement{},
		&models.AnnouncementUserState{},
		&models.PromptPreset{},
		&models.Skill{},
		&models.UIComponent{},
		&models.KnowledgeBase{},
		&models.KnowledgeBaseFile{},
		&models.ConversationProjectMCPTool{},
		&models.ConversationProjectSkill{},
		&models.ConversationProjectKnowledgeBase{},
		&models.SystemSetting{},
		&models.UserSetting{},
		&models.FileChunk{},
		&models.MessageChunk{},
		&models.PermissionGroup{},
		&models.PermissionGroupModelAccess{},
		&models.PermissionGroupModelRule{},
		&models.PermissionGroupUserAccess{},
	}
}

// SeedModelVendors 初始化内置厂商，并为存量模型中的技术厂商补齐目录项。
// 同 key 的现有目录项会晋升为内置，但管理员修改的展示名称、图标和排序不会被覆盖。
func SeedModelVendors(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range domainchannel.BuiltInModelVendors() {
			entity := models.LLMModelVendor{
				Key:       item.Key,
				Name:      item.Name,
				Icon:      item.Icon,
				BuiltIn:   true,
				SortOrder: item.SortOrder,
			}
			if err := tx.Where("key = ?", entity.Key).Attrs(entity).FirstOrCreate(&entity).Error; err != nil {
				return err
			}
			if !entity.BuiltIn {
				if err := tx.Model(&entity).Update("built_in", true).Error; err != nil {
					return err
				}
			}
		}

		var vendorKeys []string
		if err := tx.Model(&models.LLMPlatformModel{}).
			Distinct("vendor").
			Where("vendor <> ?", "").
			Pluck("vendor", &vendorKeys).Error; err != nil {
			return err
		}
		for _, key := range vendorKeys {
			entity := models.LLMModelVendor{Key: key, Name: key}
			if err := tx.Where("key = ?", key).Attrs(entity).FirstOrCreate(&entity).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SeedUIComponents 按 Name 播种内置交互式组件。内置行的目录字段（描述、入参、版本、排序、渲染方式）
// 永远以代码为准，每次启动同步，只保留管理员设置的 Enabled；目录中已移除的内置组件会被删除，
// 避免提示词继续宣告前端不再渲染的组件。
func SeedUIComponents(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		builtin := domainuicomponent.Builtin()
		names := make([]string, 0, len(builtin))
		for _, item := range builtin {
			names = append(names, item.Name)
		}
		if err := tx.Where("scope = ? AND name NOT IN ?", domainuicomponent.ScopeBuiltin, names).Delete(&models.UIComponent{}).Error; err != nil {
			return err
		}
		for _, item := range builtin {
			entity := models.UIComponent{
				Scope:          domainuicomponent.ScopeBuiltin,
				OwnerUserID:    0,
				Name:           item.Name,
				Version:        item.Version,
				Description:    item.Description,
				PropsSummary:   item.PropsSummary,
				RendererKind:   domainuicomponent.RendererBuiltin,
				RendererSource: "",
				Enabled:        item.Enabled,
				SortOrder:      item.SortOrder,
			}
			if err := tx.Where("scope = ? AND owner_user_id = 0 AND name = ?", domainuicomponent.ScopeBuiltin, item.Name).
				Attrs(entity).
				FirstOrCreate(&entity).Error; err != nil {
				return err
			}
			if entity.RendererKind != domainuicomponent.RendererBuiltin || entity.Description != item.Description || entity.PropsSummary != item.PropsSummary || entity.Version != item.Version || entity.SortOrder != item.SortOrder {
				if err := tx.Model(&entity).Updates(map[string]any{
					"renderer_kind": domainuicomponent.RendererBuiltin,
					"description":   item.Description,
					"props_summary": item.PropsSummary,
					"version":       item.Version,
					"sort_order":    item.SortOrder,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Migrate 使用 Gorm 的可移植 migrator 创建或更新基线 schema。
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
	if err := invalidateUnsignedFileEmbeddings(db); err != nil {
		return err
	}
	if err := backfillContextArtifactMessageIDs(db); err != nil {
		return err
	}
	return backfillUsageLedgerBillingAt(db)
}

// invalidateUnsignedFileEmbeddings 使旧版向量进入现有的重建索引流程。
// 无签名的消息与记忆向量在自然重新生成前保持隐藏。
func invalidateUnsignedFileEmbeddings(db *gorm.DB) error {
	return db.Exec(`
		UPDATE file_objects
		SET embed_status = 'stale'
		WHERE embed_status = 'ready'
		  AND EXISTS (
			SELECT 1
			FROM file_chunks
			WHERE file_chunks.file_obj_id = file_objects.id
			  AND file_chunks.embedding_signature = ''
		  )`).Error
}

// backfillContextArtifactMessageIDs 将旧证据统一迁移到产生该证据的助手运行节点。
// 同一次生成的 user/assistant 消息共享 run_id；仅在唯一匹配助手消息时回填，异常重复 run 数据保持不变。
func backfillContextArtifactMessageIDs(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.ChatContextRecord{}) || !db.Migrator().HasTable(&models.Message{}) {
		return nil
	}
	return db.Exec(`
		WITH artifacts_to_backfill AS (
			SELECT records.id, records.run_id, records.conversation_id, records.user_id
			FROM chat_context_records AS records
			LEFT JOIN chat_messages AS current_owner
				ON current_owner.id = records.message_id
				AND current_owner.run_id = records.run_id
				AND current_owner.conversation_id = records.conversation_id
				AND current_owner.user_id = records.user_id
				AND current_owner.role = 'assistant'
				AND current_owner.deleted_at IS NULL
			WHERE records.record_type = 'artifact'
				AND records.run_id <> ''
				AND records.deleted_at IS NULL
				AND current_owner.id IS NULL
		),
		unique_assistant_run_owners AS (
			SELECT artifacts.id AS record_id, MIN(messages.id) AS message_id
			FROM artifacts_to_backfill AS artifacts
			JOIN chat_messages AS messages
				ON messages.run_id = artifacts.run_id
				AND messages.conversation_id = artifacts.conversation_id
				AND messages.user_id = artifacts.user_id
				AND messages.role = 'assistant'
				AND messages.deleted_at IS NULL
			GROUP BY artifacts.id
			HAVING COUNT(*) = 1
		)
		UPDATE chat_context_records
		SET message_id = (
			SELECT owners.message_id
			FROM unique_assistant_run_owners AS owners
			WHERE owners.record_id = chat_context_records.id
		)
		WHERE EXISTS (
				SELECT 1
				FROM unique_assistant_run_owners AS owners
				WHERE owners.record_id = chat_context_records.id
			)
	`).Error
}

func backfillUsageLedgerBillingAt(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.UsageLedger{}) || !db.Migrator().HasColumn(&models.UsageLedger{}, "billing_at") {
		return nil
	}
	return db.Model(&models.UsageLedger{}).
		Where("billing_at IS NULL").
		Update("billing_at", gorm.Expr("created_at")).Error
}

// CleanupRemovedColumns 删除已从 Gorm 模型中移除的列。
func CleanupRemovedColumns(db *gorm.DB) error {
	if err := dropColumns(db, &models.PromptPreset{}, []string{"use_count", "last_used_at", "category", "tags_json"}); err != nil {
		return err
	}
	if err := dropColumns(db, &models.Skill{}, []string{"content", "sections_json"}); err != nil {
		return err
	}
	// discount_percent 从未进入任何计价路径，随字段移除一并清理。
	if err := dropColumns(db, &models.BillingPlan{}, []string{"discount_percent"}); err != nil {
		return err
	}
	return nil
}

func dropColumns(db *gorm.DB, table any, columns []string) error {
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

// SeedLLMSettings 在默认 LLM 运行时设置不存在时插入它们。
func SeedLLMSettings(db *gorm.DB) error {
	breakerDefaultsJSON, err := channelconfig.MarshalBreakerDefaults(domainchannel.DefaultBreakerDefaults())
	if err != nil {
		return err
	}
	settings := []models.SystemSetting{
		{
			Namespace:   "llm",
			Key:         "circuit_breaker.error_classification",
			Value:       `{"circuit_errors":["5xx","timeout","connection_error"],"rate_limit_errors":["429"],"ignore_errors":["4xx"]}`,
			ValueType:   "json",
			Description: "熔断错误分类配置",
		},
		{
			Namespace:   "llm",
			Key:         channelconfig.BreakerDefaultsKey,
			Value:       breakerDefaultsJSON,
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

// SeedPermissionGroups 在内置默认权限组不存在时插入。
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

func ensureSingleDefaultPermissionGroup(db *gorm.DB) (*models.PermissionGroup, error) {
	defaultGroups := make([]models.PermissionGroup, 0)
	if err := db.Where("is_default = ?", true).Order("id ASC").Find(&defaultGroups).Error; err != nil {
		return nil, err
	}
	if len(defaultGroups) == 0 {
		defaultGroup := models.PermissionGroup{
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
		if err := db.Model(&models.PermissionGroup{}).
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
	if err := db.Model(&models.PermissionGroupModelAccess{}).Count(&manualCount).Error; err != nil {
		return err
	}
	if manualCount > 0 {
		return nil
	}
	var ruleCount int64
	if err := db.Model(&models.PermissionGroupModelRule{}).Count(&ruleCount).Error; err != nil {
		return err
	}
	if ruleCount > 0 {
		return nil
	}
	rule := models.PermissionGroupModelRule{
		GroupID:  defaultGroupID,
		RuleType: domainchannel.PermissionGroupModelRuleAll,
		Value:    "",
	}
	return db.Where(rule).FirstOrCreate(&rule).Error
}

// SeedBillingCatalog 在计费目录为空时插入默认套餐与价格。
func SeedBillingCatalog(db *gorm.DB) error {
	defaultGroupID, err := defaultPermissionGroupID(db)
	if err != nil {
		return err
	}
	var planCount int64
	if err := db.Model(&models.BillingPlan{}).Count(&planCount).Error; err != nil {
		return err
	}
	var priceCount int64
	if err := db.Model(&models.BillingPrice{}).Count(&priceCount).Error; err != nil {
		return err
	}
	if planCount > 0 || priceCount > 0 {
		return bindBillingPlansToDefaultGroup(db, defaultGroupID)
	}

	plans := []models.BillingPlan{
		{
			Code:                "free",
			Name:                "Free",
			Description:         "默认免费套餐",
			FeatureJSON:         `{"priority":"shared"}`,
			PeriodCreditNanousd: 1000000000,
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

		prices := []models.BillingPrice{
			{PlanID: planIDByCode["free"], Code: "free-default", BillingInterval: models.BillingIntervalLifetime, Currency: "USD", AmountCents: 0, IsActive: true, IsDefault: true},
			{PlanID: planIDByCode["pro"], Code: "pro-monthly", BillingInterval: models.BillingIntervalMonth, Currency: "USD", AmountCents: 2000, IsActive: true, IsDefault: true},
			{PlanID: planIDByCode["max"], Code: "max-monthly", BillingInterval: models.BillingIntervalMonth, Currency: "USD", AmountCents: 5000, IsActive: true, IsDefault: true},
			{PlanID: planIDByCode["ultra"], Code: "ultra-monthly", BillingInterval: models.BillingIntervalMonth, Currency: "USD", AmountCents: 20000, IsActive: true, IsDefault: true},
		}
		return tx.Create(&prices).Error
	})
}

func defaultPermissionGroupID(db *gorm.DB) (*uint, error) {
	var group models.PermissionGroup
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
	return db.Model(&models.BillingPlan{}).
		Where("permission_group_id IS NULL").
		Update("permission_group_id", *defaultGroupID).Error
}

func clearDefaultPermissionGroupUsers(db *gorm.DB) error {
	defaultGroupIDs := db.Model(&models.PermissionGroup{}).
		Select("id").
		Where("is_default = ?", true)
	return db.Where("group_id IN (?)", defaultGroupIDs).
		Delete(&models.PermissionGroupUserAccess{}).Error
}

func copyUintPointer(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
