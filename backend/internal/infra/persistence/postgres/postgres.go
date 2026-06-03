package db

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// New 初始化 PostgreSQL 连接并执行迁移与种子数据。
func New(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.PostgresDSN), newGORMConfig(cfg))
	if err != nil {
		return nil, err
	}
	if err = configureTracing(db, cfg); err != nil {
		return nil, err
	}
	if err = configureConnectionPool(db, cfg); err != nil {
		return nil, err
	}

	if err = migrate(db, cfg); err != nil {
		return nil, err
	}

	if err = seedBillingCatalog(db); err != nil {
		return nil, err
	}

	return db, nil
}

func newGORMConfig(cfg config.Config) *gorm.Config {
	gormConfig := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	if isProductionEnv(cfg.Env) {
		gormConfig.Logger = productionGORMLogger()
	}
	return gormConfig
}

func productionGORMLogger() gormlogger.Interface {
	return gormlogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gormlogger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func configureConnectionPool(db *gorm.DB, cfg config.Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	maxOpen := cfg.PostgresMaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 30
	}
	maxIdle := cfg.PostgresMaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)

	if cfg.PostgresConnMaxLifetimeMin > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.PostgresConnMaxLifetimeMin) * time.Minute)
	}
	if cfg.PostgresConnMaxIdleTimeMin > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.PostgresConnMaxIdleTimeMin) * time.Minute)
	}
	return nil
}

func migrate(db *gorm.DB, cfg config.Config) error {
	if err := applySchemaBaseline(db); err != nil {
		return err
	}

	tableComments := map[string]string{
		"identity_users":                 "用户账户主表",
		"identity_contact_verifications": "用户邮箱与手机号验证记录表",
		"identity_credentials":           "用户认证凭据表",
		"identity_sessions":              "用户登录会话表",
		"identity_auth_events":           "用户认证事件表",
		"identity_providers":             "企业身份源配置表",
		"identity_user_links":            "用户第三方身份绑定表",
		"identity_mfa_settings":          "用户双因素认证配置表",
		"identity_trusted_devices":       "双因素认证可信设备表",
		"llm_upstreams":                  "上游配置表",
		"llm_upstream_models":            "上游真实模型清单表",
		"llm_platform_models":            "平台模型表",
		"llm_model_routes":               "平台模型路由绑定表",
		"mcp_servers":                    "MCP服务配置表",
		"mcp_tools":                      "MCP工具发现表",
		"chat_conversations":             "聊天会话表",
		"chat_conversation_projects":     "会话项目分组表",
		"chat_conversation_shares":       "会话公开分享快照表",
		"chat_messages":                  "会话消息表",
		"chat_feedback":                  "会话消息反馈表",
		"chat_attachments":               "多模态附件元信息表",
		"file_objects":                   "文件对象与处理结果表",
		"file_storage_quotas":            "用户文件配额表",
		"chat_runs":                      "会话运行日志表",
		"chat_run_events":                "会话运行轨迹与工具事件表",
		"chat_context_records":           "会话上下文快照与证据表",
		"user_memories":                  "用户长期个性化记忆表",
		"billing_plans":                  "订阅套餐定义表",
		"billing_prices":                 "订阅价格版本表",
		"billing_subscriptions":          "用户订阅表",
		"billing_payment_orders":         "支付订单表",
		"billing_accounts":               "按量计费余额账户表",
		"billing_balance_transactions":   "按量计费余额流水表",
		"billing_redemption_codes":       "计费兑换码定义表",
		"billing_redemptions":            "计费兑换记录表",
		"billing_model_prices":           "平台模型按量单价配置表",
		"billing_usage_ledgers":          "按量用量账本表",
		"audit_logs":                     "可追溯审计日志表",
		"system_events":                  "后台系统事件表",
		"system_announcements":           "站点公告表",
		"announcement_user_states":       "用户公告展示状态表",
		"system_settings":                "系统动态配置表",
		"user_settings":                  "用户个人偏好配置表",
		"file_chunks":                    "RAG文件分片表",
		"chat_message_chunks":            "会话消息向量分片表(历史对话语义检索)",
	}

	for table, comment := range tableComments {
		statement := fmt.Sprintf(`COMMENT ON TABLE "%s" IS '%s'`, table, escapeSQLLiteral(comment))
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}

	if err := applyIdentityBaselineConstraints(db); err != nil {
		return err
	}
	if err := applyIdentityProviderBaseline(db); err != nil {
		return err
	}
	if err := applyConversationBaselineIndexes(db); err != nil {
		return err
	}
	if err := applyLLMBaselineIndexes(db); err != nil {
		return err
	}
	if err := applyBillingBaselineIndexes(db); err != nil {
		return err
	}
	if err := applyAnnouncementBaseline(db); err != nil {
		return err
	}
	if err := applyVectorBaseline(db, vectorBaselineRequired(cfg)); err != nil {
		return err
	}
	if err := seedLLMSettings(db); err != nil {
		return err
	}

	return nil
}

func applySchemaBaseline(db *gorm.DB) error {
	models := []interface{}{
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
	for _, item := range models {
		if db.Migrator().HasTable(item) {
			continue
		}
		if err := db.Migrator().CreateTable(item); err != nil {
			return err
		}
	}
	return nil
}

func escapeSQLLiteral(input string) string {
	return strings.ReplaceAll(input, "'", "''")
}

func applyLLMBaselineIndexes(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE "llm_upstreams"
		ADD COLUMN IF NOT EXISTS "protocol_defaults_json" text NOT NULL DEFAULT '{}'`,
		`COMMENT ON COLUMN "llm_upstreams"."protocol_defaults_json" IS '按模型类型配置的默认协议JSON'`,
		`ALTER TABLE "llm_platform_models"
		ADD COLUMN IF NOT EXISTS "system_prompt" text NOT NULL DEFAULT ''`,
		`COMMENT ON COLUMN "llm_platform_models"."system_prompt" IS '模型级系统提示词'`,
		`ALTER TABLE "llm_platform_models"
		ADD COLUMN IF NOT EXISTS "access_scope" varchar(32) NOT NULL DEFAULT 'public'`,
		`COMMENT ON COLUMN "llm_platform_models"."access_scope" IS '模型使用范围: public用户可用 internal仅内部任务'`,
		`CREATE INDEX IF NOT EXISTS idx_llm_platform_models_access_scope
			ON "llm_platform_models" ("access_scope")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_upstream_models_upstream_name
			ON "llm_upstream_models" ("upstream_id", "upstream_model_name")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_upstream_models_binding_code
			ON "llm_upstream_models" ("binding_code")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_platform_models_name
			ON "llm_platform_models" ("name")`,
		`DROP INDEX IF EXISTS idx_llm_model_routes_unique`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_model_routes_unique
			ON "llm_model_routes" ("platform_model_id", "upstream_model_id", "protocol")`,
		`CREATE INDEX IF NOT EXISTS idx_llm_model_routes_routing
			ON "llm_model_routes" ("platform_model_id", "status", "priority", "weight")
			WHERE status = 'active'`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyBillingBaselineIndexes(db *gorm.DB) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_billing_usage_ledgers_user_date_model
		ON "billing_usage_ledgers" ("user_id", "usage_date", "platform_model_name")`,
		`CREATE INDEX IF NOT EXISTS idx_billing_usage_ledgers_user_created_billable
		ON "billing_usage_ledgers" ("user_id", "created_at")
		WHERE is_free_model = FALSE`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_balance_transactions_usage_ref
		ON "billing_balance_transactions" ("user_id", "type", "ref_no")
		WHERE ref_no <> '' AND type IN ('usage_reserve', 'usage_refund')`,
		`ALTER TABLE "billing_redemption_codes"
		ADD COLUMN IF NOT EXISTS "code_encrypted" text NOT NULL DEFAULT ''`,
		`COMMENT ON COLUMN "billing_redemption_codes"."code_encrypted" IS 'AES-GCM加密后的兑换码明文'`,
		`UPDATE "billing_redemption_codes"
		SET "code_hint" = replace("code_hint", '...', '***')
		WHERE "code_hint" LIKE '%...%'`,
		`CREATE INDEX IF NOT EXISTS idx_billing_redemption_codes_status_mode
		ON "billing_redemption_codes" ("status", "mode", "id")`,
		`CREATE INDEX IF NOT EXISTS idx_billing_redemptions_code_user_created
		ON "billing_redemptions" ("code_id", "user_id", "created_at")`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyAnnouncementBaseline(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE "system_announcements"
		ADD COLUMN IF NOT EXISTS "type" varchar(32) NOT NULL DEFAULT 'general'`,
		`COMMENT ON COLUMN "system_announcements"."type" IS '公告类型(critical/warning/info/normal/general)'`,
		`ALTER TABLE "system_announcements"
		ADD COLUMN IF NOT EXISTS "pinned" boolean NOT NULL DEFAULT false`,
		`COMMENT ON COLUMN "system_announcements"."pinned" IS '是否置顶'`,
		`CREATE INDEX IF NOT EXISTS idx_system_announcements_sort
		ON "system_announcements" ("pinned", "priority", "updated_at", "id")`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_announcement_user_states_version
		ON "announcement_user_states" ("announcement_id", "user_id", "announcement_updated_at")`,
		`ALTER TABLE "announcement_user_states"
		ADD COLUMN IF NOT EXISTS "closed_at" timestamptz`,
		`COMMENT ON COLUMN "announcement_user_states"."closed_at" IS '关闭时间'`,
		`CREATE INDEX IF NOT EXISTS idx_announcement_user_states_user_dismissed
		ON "announcement_user_states" ("user_id", "dismissed_until")
		WHERE "dismissed_until" IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_announcement_user_states_user_closed
		ON "announcement_user_states" ("user_id", "closed_at")
		WHERE "closed_at" IS NOT NULL`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyIdentityBaselineConstraints(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE "identity_users"
		ADD COLUMN IF NOT EXISTS "appearance_preferences" text NOT NULL DEFAULT ''`,
		`COMMENT ON COLUMN "identity_users"."appearance_preferences" IS '外观偏好JSON'`,
		`DROP INDEX IF EXISTS uk_identity_users_single_superadmin`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyIdentityProviderBaseline(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE "identity_providers"
		ADD COLUMN IF NOT EXISTS "email_verified_field" varchar(64) NOT NULL DEFAULT 'email_verified'`,
		`COMMENT ON COLUMN "identity_providers"."email_verified_field" IS '邮箱验证状态字段'`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyConversationBaselineIndexes(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE "chat_conversations"
		ADD COLUMN IF NOT EXISTS "project_id" bigint`,
		`COMMENT ON COLUMN "chat_conversations"."project_id" IS '项目分组ID'`,
		`ALTER TABLE "chat_conversation_projects"
		ADD COLUMN IF NOT EXISTS "system_prompt" text NOT NULL DEFAULT ''`,
		`COMMENT ON COLUMN "chat_conversation_projects"."system_prompt" IS '项目级系统提示词'`,
		`CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_status_starred_updated_at
		ON "chat_conversations" ("user_id", "status", "is_starred", "updated_at" DESC, "id" DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_status_starred_starred_at
		ON "chat_conversations" ("user_id", "status", "is_starred", "starred_at" DESC, "id" DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_conversation_projects_public_id
		ON "chat_conversation_projects" ("public_id")
		WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_chat_conversation_projects_user_status_sort
		ON "chat_conversation_projects" ("user_id", "status", "sort_order" ASC, "id" DESC)
		WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_project_status_updated
		ON "chat_conversations" ("user_id", "project_id", "status", "updated_at" DESC, "id" DESC)
		WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_chat_conversation_shares_active_conversation
		ON "chat_conversation_shares" ("conversation_id", "updated_at" DESC, "id" DESC)
		WHERE status = 'active'`,
		`CREATE INDEX IF NOT EXISTS idx_chat_conversation_shares_user_status_updated_at
		ON "chat_conversation_shares" ("user_id", "status", "updated_at" DESC, "id" DESC)`,
		`ALTER TABLE "chat_runs"
		ADD COLUMN IF NOT EXISTS "task_type" varchar(32) NOT NULL DEFAULT 'chat'`,
		`COMMENT ON COLUMN "chat_runs"."task_type" IS '任务类型'`,
		`CREATE INDEX IF NOT EXISTS idx_chat_runs_task_type
		ON "chat_runs" ("task_type")`,
		`ALTER TABLE "chat_run_events"
		ALTER COLUMN "event_id" TYPE varchar(255),
		ALTER COLUMN "parent_event_id" TYPE varchar(255),
		ALTER COLUMN "title" TYPE varchar(255),
		ALTER COLUMN "tool_call_id" TYPE varchar(255)`,
		`COMMENT ON COLUMN "chat_run_events"."event_id" IS '事件ID'`,
		`COMMENT ON COLUMN "chat_run_events"."parent_event_id" IS '父事件ID'`,
		`COMMENT ON COLUMN "chat_run_events"."title" IS '轨迹标题'`,
		`COMMENT ON COLUMN "chat_run_events"."tool_call_id" IS '工具调用ID'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_file_objects_active_user_content
		ON "file_objects" ("user_id", "sha256", "size_bytes")
		WHERE status = 'active' AND deleted_at IS NULL AND sha256 <> ''`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}

	return nil
}

func vectorBaselineRequired(cfg config.Config) bool {
	return cfg.EmbeddingEnabled || cfg.RAGEnabled || cfg.MessageEmbeddingEnabled || cfg.SemanticContextEnabled
}

// applyVectorBaseline 确保 pgvector 扩展、向量列和检索索引存在。
func applyVectorBaseline(db *gorm.DB, required bool) error {
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`).Error; err != nil {
		return handleOptionalVectorBaselineError(required, "create pgvector extension", err)
	}

	statements := []struct {
		name string
		sql  string
	}{
		{
			name: "add file_chunks embedding column",
			sql:  `ALTER TABLE "file_chunks" ADD COLUMN IF NOT EXISTS embedding vector(1536)`,
		},
		{
			name: "index file_chunks embedding",
			sql: `CREATE INDEX IF NOT EXISTS idx_file_chunks_embedding
				ON "file_chunks" USING ivfflat (embedding vector_cosine_ops)
				WITH (lists = 100)`,
		},
		{
			name: "add chat_message_chunks embedding column",
			sql:  `ALTER TABLE "chat_message_chunks" ADD COLUMN IF NOT EXISTS embedding vector(1536)`,
		},
		{
			name: "index chat_message_chunks embedding",
			sql: `CREATE INDEX IF NOT EXISTS idx_chat_message_chunks_embedding
				ON "chat_message_chunks" USING ivfflat (embedding vector_cosine_ops)
				WITH (lists = 100)`,
		},
		{
			name: "add user_memories embedding column",
			sql:  `ALTER TABLE "user_memories" ADD COLUMN IF NOT EXISTS embedding vector(1536)`,
		},
		{
			name: "index user_memories embedding",
			sql: `CREATE INDEX IF NOT EXISTS idx_user_memories_embedding
				ON "user_memories" USING ivfflat (embedding vector_cosine_ops)
				WITH (lists = 50)`,
		},
	}

	for _, statement := range statements {
		if err := db.Exec(statement.sql).Error; err != nil {
			if baselineErr := handleOptionalVectorBaselineError(required, statement.name, err); baselineErr != nil {
				return baselineErr
			}
		}
	}
	return nil
}

func handleOptionalVectorBaselineError(required bool, operation string, err error) error {
	if err == nil {
		return nil
	}
	if required {
		return fmt.Errorf("%s failed: %w", operation, err)
	}
	log.Printf("postgres vector baseline skipped: %s failed: %v", operation, err)
	return nil
}

func seedLLMSettings(db *gorm.DB) error {
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

func seedBillingCatalog(db *gorm.DB) error {
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
			{
				PlanID:          planIDByCode["free"],
				Code:            "free-default",
				BillingInterval: model.BillingIntervalLifetime,
				Currency:        "USD",
				AmountCents:     0,
				IsActive:        true,
				IsDefault:       true,
			},
			{
				PlanID:          planIDByCode["pro"],
				Code:            "pro-monthly",
				BillingInterval: model.BillingIntervalMonth,
				Currency:        "USD",
				AmountCents:     2000,
				IsActive:        true,
				IsDefault:       true,
			},
			{
				PlanID:          planIDByCode["max"],
				Code:            "max-monthly",
				BillingInterval: model.BillingIntervalMonth,
				Currency:        "USD",
				AmountCents:     5000,
				IsActive:        true,
				IsDefault:       true,
			},
			{
				PlanID:          planIDByCode["ultra"],
				Code:            "ultra-monthly",
				BillingInterval: model.BillingIntervalMonth,
				Currency:        "USD",
				AmountCents:     20000,
				IsActive:        true,
				IsDefault:       true,
			},
		}

		return tx.Create(&prices).Error
	})
}
