package settings

import (
	"errors"
	"strconv"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

// settingSpec 是一个动态配置项的唯一定义：种子默认值、敏感性、取值校验与运行时写入。
// 新增配置只需在 settingSpecs 追加一项，合法 key、种子、脱敏、PATCH 校验与 ApplyTo 全部由此派生。
type settingSpec struct {
	Namespace   string
	Key         string
	ValueType   string
	Default     string
	Description string
	// Sensitive 为 true 的值加密落库、对外脱敏，并允许通过 Clear 清空。
	Sensitive bool
	// Validate 校验去除首尾空白后的新值；nil 表示不做取值校验。
	Validate settingValidator
	// Apply 把生效值写入运行时配置；nil 表示该项不进入 config.Config，由各模块在运行时按需读取。
	Apply func(cfg *config.Config, value string)
}

func (s settingSpec) fullKey() string {
	return s.Namespace + ":" + s.Key
}

func (s settingSpec) seedSetting() domainsettings.SystemSetting {
	return domainsettings.SystemSetting{
		Namespace:   s.Namespace,
		Key:         s.Key,
		Value:       s.Default,
		ValueType:   s.ValueType,
		Description: s.Description,
	}
}

var settingSpecs = []settingSpec{
	// 认证配置
	{Namespace: "auth", Key: "token_ttl_hours", ValueType: "int", Default: "24", Description: "Access Token 有效期(小时)",
		Validate: intRange(1, 168), Apply: applyField(func(c *config.Config) *int { return &c.TokenTTLHours }, toInt)},
	{Namespace: "auth", Key: "refresh_token_ttl_hours", ValueType: "int", Default: "720", Description: "Refresh Token 有效期(小时)",
		Validate: intRange(1, 8760), Apply: applyField(func(c *config.Config) *int { return &c.RefreshTokenTTLHours }, toInt)},
	{Namespace: "auth", Key: "login_max_failures", ValueType: "int", Default: "5", Description: "登录失败锁定阈值",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.LoginMaxFailures }, toInt)},
	{Namespace: "auth", Key: "login_lock_minutes", ValueType: "int", Default: "15", Description: "锁定时长(分钟)",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.LoginLockMinutes }, toInt)},
	{Namespace: "auth", Key: "rate_limit_enabled", ValueType: "bool", Default: "false", Description: "是否启用平台 HTTP 429 限流",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.RateLimitEnabled }, toBool)},
	{Namespace: "auth", Key: "rate_limit_rpm", ValueType: "int", Default: "60", Description: "全局限流 RPM",
		Validate: intRange(1, 100000), Apply: applyField(func(c *config.Config) *int { return &c.RateLimitRPM }, toInt)},
	{Namespace: "auth", Key: "public_auth_rate_limit_rpm", ValueType: "int", Default: "30", Description: "公开鉴权接口限流 RPM",
		Validate: intRange(1, 100000), Apply: applyField(func(c *config.Config) *int { return &c.PublicAuthRateLimitRPM }, toInt)},
	{Namespace: "auth", Key: "login_default_next_path", ValueType: "string", Default: "/chat", Description: "无 next 参数时登录成功后的默认跳转路径",
		Validate: validateLoginDefaultNextPath},
	{Namespace: "auth", Key: "username_login_enabled", ValueType: "bool", Default: "true", Description: "是否允许用户名密码登录",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.UsernameLoginEnabled }, toBool)},
	{Namespace: "auth", Key: "email_login_enabled", ValueType: "bool", Default: "true", Description: "是否允许邮箱登录",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmailLoginEnabled }, toBool)},
	{Namespace: "auth", Key: "third_party_login_enabled", ValueType: "bool", Default: "true", Description: "是否启用第三方登录入口",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ThirdPartyLoginEnabled }, toBool)},
	{Namespace: "auth", Key: "email_registration_enabled", ValueType: "bool", Default: "true", Description: "是否允许邮箱注册",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmailRegistrationEnabled }, toBool)},
	{Namespace: "auth", Key: "email_verification_enabled", ValueType: "bool", Default: "false", Description: "邮箱注册时是否要求邮箱验证码",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmailVerificationEnabled }, toBool)},
	{Namespace: "auth", Key: "password_reset_enabled", ValueType: "bool", Default: "false", Description: "是否允许用户在登录页重置密码",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.PasswordResetEnabled }, toBool)},
	{Namespace: "auth", Key: "smtp_host", ValueType: "string", Default: "", Description: "邮箱验证码 SMTP 主机",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.SMTPHost }, trimmedText)},
	{Namespace: "auth", Key: "smtp_port", ValueType: "int", Default: "587", Description: "邮箱验证码 SMTP 端口",
		Validate: intRange(1, 65535), Apply: applyField(func(c *config.Config) *int { return &c.SMTPPort }, toInt)},
	{Namespace: "auth", Key: "smtp_username", ValueType: "string", Default: "", Description: "邮箱验证码 SMTP 用户名",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.SMTPUsername }, trimmedText)},
	{Namespace: "auth", Key: "smtp_password", ValueType: "string", Default: "", Description: "邮箱验证码 SMTP 密码", Sensitive: true,
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.SMTPPassword }, trimmedText)},
	{Namespace: "auth", Key: "smtp_from", ValueType: "string", Default: "", Description: "邮箱验证码发件人",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.SMTPFrom }, trimmedText)},
	{Namespace: "auth", Key: "email_registration_allowed_domains", ValueType: "string", Default: "", Description: "邮箱注册域名白名单，留空表示不限制",
		Validate: validateEmailDomainList, Apply: applyField(func(c *config.Config) *string { return &c.EmailRegistrationDomains }, trimmedText)},
	{Namespace: "auth", Key: "email_registration_block_plus_alias", ValueType: "bool", Default: "false", Description: "邮箱注册是否禁止 + 别名地址",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmailRegistrationNoAlias }, toBool)},
	{Namespace: "auth", Key: "auto_link_verified_email", ValueType: "bool", Default: "true", Description: "是否允许相同已验证邮箱自动绑定第三方身份",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.AutoLinkVerifiedEmail }, toBool)},
	{Namespace: "auth", Key: "turnstile_registration_enabled", ValueType: "bool", Default: "false", Description: "邮箱注册是否启用 Cloudflare Turnstile 人机验证",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.TurnstileRegistrationEnabled }, toBool)},
	{Namespace: "auth", Key: "turnstile_site_key", ValueType: "string", Default: "", Description: "Cloudflare Turnstile Site Key",
		Validate: maxLength(512), Apply: applyField(func(c *config.Config) *string { return &c.TurnstileSiteKey }, trimmedText)},
	{Namespace: "auth", Key: "turnstile_secret_key", ValueType: "string", Default: "", Description: "Cloudflare Turnstile Secret Key", Sensitive: true,
		Validate: maxLength(512), Apply: applyField(func(c *config.Config) *string { return &c.TurnstileSecretKey }, trimmedText)},

	// 计费配置：由 billing 模块在运行时按 namespace 读取，不进入 config.Config。
	{Namespace: "billing", Key: "mode", ValueType: "string", Default: "self", Description: "计费方式：self=自用模式，period=周期计费，usage=按量计费",
		Validate: oneOf("self", "period", "usage")},
	{Namespace: "billing", Key: "prepaid_amount_usd", ValueType: "string", Default: "0", Description: "每个付费调用预留的风险预算(美元)，0表示按剩余槽位动态分配可用预算，最多5个并发调用",
		Validate: floatRange(0, 1000000)},
	{Namespace: "billing", Key: "native_tool_billing_enabled", ValueType: "bool", Default: "true", Description: "是否按官方默认价格计费模型原生工具调用",
		Validate: boolValue()},
	{Namespace: "billing", Key: "native_tool_pricing_json", ValueType: "json", Default: nativetool.DefaultPricingJSON(), Description: "官方原生工具计费覆盖 JSON，按 toolKey 配置 priceNanousd、unit、priceLabel、billable",
		Validate: validateNativeToolPricingJSON},
	{Namespace: "billing", Key: "usd_to_cny_rate", ValueType: "string", Default: "7.2", Description: "易支付美元兑人民币汇率",
		Validate: floatRange(0.000001, 1000)},
	{Namespace: "billing", Key: "display_currency", ValueType: "string", Default: "USD", Description: "用户端费用展示币种：USD 或 CNY",
		Validate: oneOf("USD", "CNY")},
	{Namespace: "billing", Key: "payment_providers", ValueType: "string", Default: "disabled", Description: "启用支付渠道，多个用英文逗号分隔：stripe,epay",
		Validate: validatePaymentProviders},
	{Namespace: "billing", Key: "stripe_publishable_key", ValueType: "string", Default: "", Description: "Stripe Publishable Key",
		Validate: maxLength(512)},
	{Namespace: "billing", Key: "stripe_secret_key", ValueType: "string", Default: "", Description: "Stripe Secret Key", Sensitive: true,
		Validate: maxLength(512)},
	{Namespace: "billing", Key: "stripe_webhook_secret", ValueType: "string", Default: "", Description: "Stripe Webhook Secret", Sensitive: true,
		Validate: maxLength(512)},
	{Namespace: "billing", Key: "epay_gateway_url", ValueType: "string", Default: "", Description: "易支付 submit.php 页面跳转网关地址",
		Validate: validateEPayGatewayURL},
	{Namespace: "billing", Key: "epay_types", ValueType: "string", Default: `[{"name":"支付宝","type":"alipay"},{"name":"微信支付","type":"wxpay"}]`, Description: "易支付启用的支付类型 JSON",
		Validate: allOf(maxLength(4000), validateEPayTypesJSON)},
	{Namespace: "billing", Key: "epay_pid", ValueType: "string", Default: "", Description: "易支付商户 ID",
		Validate: maxLength(512)},
	{Namespace: "billing", Key: "epay_key", ValueType: "string", Default: "", Description: "易支付商户密钥", Sensitive: true,
		Validate: maxLength(512)},

	// 对话配置
	{Namespace: "chat", Key: "max_context_messages", ValueType: "int", Default: "20", Description: "上下文消息数",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.MaxContextMessages }, toInt)},
	{Namespace: "chat", Key: "context_max_turns", ValueType: "int", Default: "48", Description: "最大对话轮次",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.ContextMaxTurns }, toInt)},
	{Namespace: "chat", Key: "context_compact_enabled", ValueType: "bool", Default: "false", Description: "是否允许上下文压缩功能",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ContextCompactEnabled }, toBool)},
	{Namespace: "chat", Key: "context_window_fallback_tokens", ValueType: "int", Default: strconv.Itoa(config.DefaultContextWindowFallbackTokens), Description: "无法识别模型上下文窗口时使用的默认 Token 数",
		Validate: intRange(config.MinContextWindowFallbackTokens, config.MaxContextWindowFallbackTokens), Apply: applyField(func(c *config.Config) *int { return &c.ContextWindowFallbackTokens }, toInt)},
	{Namespace: "chat", Key: "context_compact_trigger_percent", ValueType: "int", Default: strconv.Itoa(config.DefaultContextCompactTriggerPercent), Description: "达到当前模型有效上下文预算的指定百分比时触发压缩；0 表示关闭按 Token 触发",
		Validate: optionalIntZeroOrRange(config.MinContextCompactTriggerPercent, config.MaxContextCompactTriggerPercent), Apply: applyField(func(c *config.Config) *int { return &c.ContextCompactTriggerPercent }, toInt)},
	{Namespace: "chat", Key: "context_compact_preserve_recent_turns", ValueType: "int", Default: "8", Description: "压缩保留轮次",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.ContextCompactPreserve }, toInt)},
	{Namespace: "chat", Key: "conversation_default_model", ValueType: "string", Default: "", Description: "新会话系统推荐模型；留空时回退到第一个可用模型",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.ConversationDefaultModel }, trimmedText)},
	{Namespace: "chat", Key: "conversation_task_model", ValueType: "string", Default: "follow", Description: "会话标题/标签生成任务使用的聊天模型，follow 表示跟随当前会话模型；图片模型不会用于标题/标签生成",
		Apply: applyField(func(c *config.Config) *string { return &c.ConversationTaskModel }, rawText)},
	{Namespace: "chat", Key: "conversation_title_prompt", ValueType: "string", Default: "", Description: "会话标题生成提示词，支持 {{MESSAGES}} 占位符；空串使用内置默认值",
		Apply: applyField(func(c *config.Config) *string { return &c.ConversationTitlePrompt }, rawText)},
	{Namespace: "chat", Key: "conversation_labels_prompt", ValueType: "string", Default: "", Description: "会话标签生成提示词，支持 {{MESSAGES}} 占位符；空串使用内置默认值",
		Apply: applyField(func(c *config.Config) *string { return &c.ConversationLabelsPrompt }, rawText)},
	{Namespace: "chat", Key: "default_system_prompt", ValueType: "string", Default: "", Description: "全局默认系统提示词，仅对聊天任务生效；空串表示不注入",
		Validate: maxLength(20000), Apply: applyField(func(c *config.Config) *string { return &c.DefaultSystemPrompt }, rawText)},
	{Namespace: "chat", Key: "skills_prompt", ValueType: "string", Default: "", Description: "Skills 调用提示词；空串使用内置默认值",
		Validate: maxLength(20000), Apply: applyField(func(c *config.Config) *string { return &c.SkillsPrompt }, rawText)},
	{Namespace: "chat", Key: "model_option_policy_mode", ValueType: "string", Default: "allowlist", Description: "模型 options 透传策略：allowlist=仅白名单，denylist=黑名单拦截，disabled=禁止透传",
		Validate: oneOf("allowlist", "denylist", "disabled"), Apply: applyField(func(c *config.Config) *string { return &c.ModelOptionPolicyMode }, trimmedText)},
	{Namespace: "chat", Key: "model_option_allowed_paths", ValueType: "json", Default: config.DefaultModelOptionAllowedPathsJSON(), Description: "模型 options 白名单路径 JSON，default 对所有协议生效",
		Validate: validateModelOptionPathsJSON, Apply: applyField(func(c *config.Config) *string { return &c.ModelOptionAllowedPaths }, rawText)},
	{Namespace: "chat", Key: "model_option_denied_paths", ValueType: "json", Default: config.DefaultModelOptionDeniedPathsJSON(), Description: "模型 options 黑名单路径 JSON，default 对所有协议生效",
		Validate: validateModelOptionPathsJSON, Apply: applyField(func(c *config.Config) *string { return &c.ModelOptionDeniedPaths }, rawText)},

	// 知识库配置
	{Namespace: "knowledgebase", Key: "enabled", ValueType: "bool", Default: "true", Description: "是否启用知识库功能；关闭后隐藏用户侧入口并拒绝知识库请求",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.KnowledgeBaseEnabled }, toBool)},

	// 存储配置
	{Namespace: "storage", Key: "user_storage_quota_bytes", ValueType: "int", Default: "104857600", Description: "用户总存储配额（管理页面按 MB 输入，内部以字节保存），0表示不限制",
		Validate: int64Min(0), Apply: applyField(func(c *config.Config) *int64 { return &c.UserStorageQuotaBytes }, toInt64)},
	{Namespace: "storage", Key: "max_upload_file_bytes", ValueType: "int", Default: "20971520", Description: "默认附件大小上限（管理页面按 MB 输入，内部以字节保存）",
		Validate: int64Min(1), Apply: applyField(func(c *config.Config) *int64 { return &c.MaxUploadFileBytes }, toInt64)},
	{Namespace: "storage", Key: "max_message_files", ValueType: "int", Default: "10", Description: "单消息附件数",
		Validate: intRange(1, 50), Apply: applyField(func(c *config.Config) *int { return &c.MaxMessageFiles }, toInt)},

	// 文件处理配置
	{Namespace: "file", Key: "image_max_dimension", ValueType: "int", Default: "1024", Description: "图片发送前缩放最大边长(px)，0=不缩放",
		Validate: intRange(0, 8192), Apply: applyField(func(c *config.Config) *int { return &c.ImageMaxDimension }, toInt)},
	{Namespace: "file", Key: "full_context_limit_enabled", ValueType: "bool", Default: "true", Description: "是否启用全文注入大小、Token、PDF页数限制",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.FileFullContextLimitEnabled }, toBool)},
	{Namespace: "file", Key: "file_full_context_max_bytes", ValueType: "int", Default: strconv.FormatInt(config.DefaultFileFullContextMaxBytes, 10), Description: "文本文件全文注入最大大小（管理页面按 MB 输入，内部以字节保存，默认2MB），留空或0表示不限制",
		Validate: optionalInt64Min(0), Apply: applyField(func(c *config.Config) *int64 { return &c.FileFullContextMaxBytes }, toOptionalInt64)},
	{Namespace: "file", Key: "full_context_max_tokens", ValueType: "int", Default: "65536", Description: "全文注入最大token预算，留空或0表示不限制",
		Validate: optionalIntZeroOrRange(128, 1000000), Apply: applyField(func(c *config.Config) *int { return &c.FileFullContextMaxTokens }, toOptionalInt)},
	{Namespace: "file", Key: "image_max_bytes", ValueType: "int", Default: "", Description: "图片单文件大小上限（管理页面按 MB 输入，内部以字节保存），留空则回退默认附件大小上限",
		Validate: optionalInt64Min(1), Apply: applyField(func(c *config.Config) *int64 { return &c.FileImageMaxBytes }, toOptionalInt64)},
	{Namespace: "file", Key: "doc_max_bytes", ValueType: "int", Default: "", Description: "文档单文件大小上限（管理页面按 MB 输入，内部以字节保存），留空则回退默认附件大小上限",
		Validate: optionalInt64Min(1), Apply: applyField(func(c *config.Config) *int64 { return &c.FileDocMaxBytes }, toOptionalInt64)},
	{Namespace: "file", Key: "full_context_pdf_max_pages", ValueType: "int", Default: "20", Description: "PDF Full Context最大页数，留空或0表示不限制",
		Validate: optionalIntZeroOrRange(1, 500), Apply: applyField(func(c *config.Config) *int { return &c.FileFullContextPDFMaxPages }, toOptionalInt)},
	{Namespace: "file", Key: "allowed_mime_types", ValueType: "string", Default: defaultAllowedMIMETypes, Description: "白名单MIME类型(逗号分隔)",
		Validate: validateAllowedMIMETypes, Apply: applyField(func(c *config.Config) *string { return &c.FileAllowedMIMETypes }, rawText)},

	// 文本提取与 OCR 配置
	{Namespace: "extract", Key: "engine", ValueType: "string", Default: "builtin", Description: "提取主引擎枚举(builtin/tika/docling/mineru)",
		Validate: oneOf(extraction.EngineBuiltin, extraction.EngineTika, extraction.EngineDocling, extraction.EngineMinerU), Apply: applyField(func(c *config.Config) *string { return &c.ExtractEngine }, rawText)},
	{Namespace: "extract", Key: "ocr_engine", ValueType: "string", Default: "rapidocr", Description: "OCR 引擎枚举(rapidocr/tesseract/paddle/tencent/aliyun/mistral/llm)",
		Validate: oneOf(extraction.OCREngineRapidOCR, extraction.OCREngineTesseract, extraction.OCREnginePaddle, extraction.OCREngineTencent, extraction.OCREngineAliyun, extraction.OCREngineMistral, extraction.OCREngineLLM), Apply: applyField(func(c *config.Config) *string { return &c.ExtractOCREngine }, rawText)},
	{Namespace: "extract", Key: "image_ocr_enabled", ValueType: "bool", Default: "false", Description: "是否对图片附件执行 OCR",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ExtractImageOCREnabled }, toBool)},
	{Namespace: "extract", Key: "pdf_ocr_fallback_enabled", ValueType: "bool", Default: "false", Description: "PDF 原生文本提取失败或质量较差时是否启用 OCR 回退",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ExtractPDFOCRFallbackEnabled }, toBool)},
	{Namespace: "extract", Key: "tika_source", ValueType: "string", Default: "external", Description: "Tika 服务来源枚举(external/managed)",
		Validate: oneOf(extraction.TikaSourceExternal, extraction.TikaSourceManaged), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTikaSource }, rawText)},
	{Namespace: "extract", Key: "tika_base_url", ValueType: "string", Default: "http://127.0.0.1:9998", Description: "Apache Tika 服务地址，默认 http://127.0.0.1:9998",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTikaBaseURL }, rawText)},
	{Namespace: "extract", Key: "tika_timeout_seconds", ValueType: "int", Default: "60", Description: "Apache Tika 请求超时(秒)，默认 60s",
		Validate: intRange(1, 120), Apply: applyField(func(c *config.Config) *int { return &c.ExtractTikaTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "tika_auth_token", ValueType: "string", Default: "", Description: "Apache Tika 鉴权 Token", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractTikaAuthToken }, rawText)},
	{Namespace: "extract", Key: "docling_base_url", ValueType: "string", Default: "http://127.0.0.1:8005/ocr", Description: "Docling 服务地址，默认 http://127.0.0.1:8005/ocr",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractDoclingBaseURL }, rawText)},
	{Namespace: "extract", Key: "docling_auth_token", ValueType: "string", Default: "", Description: "Docling 鉴权 Token", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractDoclingAuthToken }, rawText)},
	{Namespace: "extract", Key: "docling_timeout_seconds", ValueType: "int", Default: "60", Description: "Docling 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractDoclingTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "tesseract_ocr_base_url", ValueType: "string", Default: "http://127.0.0.1:8004/ocr", Description: "Tesseract OCR 服务地址，默认 http://127.0.0.1:8004/ocr",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTesseractOCRBaseURL }, rawText)},
	{Namespace: "extract", Key: "tesseract_ocr_auth_token", ValueType: "string", Default: "", Description: "Tesseract OCR 鉴权 Token", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractTesseractOCRAuthToken }, rawText)},
	{Namespace: "extract", Key: "tesseract_ocr_timeout_seconds", ValueType: "int", Default: "60", Description: "Tesseract OCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractTesseractOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "rapidocr_source", ValueType: "string", Default: "external", Description: "RapidOCR 服务来源枚举(external/managed)",
		Validate: oneOf(extraction.TikaSourceExternal, extraction.TikaSourceManaged), Apply: applyField(func(c *config.Config) *string { return &c.ExtractRapidOCRSource }, rawText)},
	{Namespace: "extract", Key: "rapidocr_base_url", ValueType: "string", Default: "http://127.0.0.1:8002/ocr", Description: "RapidOCR 服务地址，默认 http://127.0.0.1:8002/ocr",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractRapidOCRBaseURL }, rawText)},
	{Namespace: "extract", Key: "rapidocr_auth_token", ValueType: "string", Default: "", Description: "RapidOCR 鉴权 Token", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractRapidOCRAuthToken }, rawText)},
	{Namespace: "extract", Key: "rapidocr_timeout_seconds", ValueType: "int", Default: "60", Description: "RapidOCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractRapidOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "paddle_ocr_base_url", ValueType: "string", Default: "", Description: "Paddle OCR 服务地址",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractPaddleOCRBaseURL }, rawText)},
	{Namespace: "extract", Key: "paddle_ocr_auth_token", ValueType: "string", Default: "", Description: "Paddle OCR 鉴权 Token", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractPaddleOCRAuthToken }, rawText)},
	{Namespace: "extract", Key: "paddle_ocr_timeout_seconds", ValueType: "int", Default: "60", Description: "Paddle OCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractPaddleOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "tencent_ocr_secret_id", ValueType: "string", Default: "", Description: "腾讯云 OCR SecretId",
		Validate: maxLength(512), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTencentOCRSecretID }, rawText)},
	{Namespace: "extract", Key: "tencent_ocr_secret_key", ValueType: "string", Default: "", Description: "腾讯云 OCR SecretKey", Sensitive: true,
		Validate: maxLength(512), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTencentOCRSecretKey }, rawText)},
	{Namespace: "extract", Key: "tencent_ocr_region", ValueType: "string", Default: "ap-guangzhou", Description: "腾讯云 OCR 地域",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTencentOCRRegion }, rawText)},
	{Namespace: "extract", Key: "tencent_ocr_endpoint", ValueType: "string", Default: "ocr.tencentcloudapi.com", Description: "腾讯云 OCR 接入点",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.ExtractTencentOCREndpoint }, rawText)},
	{Namespace: "extract", Key: "tencent_ocr_timeout_seconds", ValueType: "int", Default: "60", Description: "腾讯云 OCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractTencentOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "aliyun_ocr_access_key_id", ValueType: "string", Default: "", Description: "阿里云 OCR AccessKey ID",
		Validate: maxLength(512), Apply: applyField(func(c *config.Config) *string { return &c.ExtractAliyunOCRAccessKeyID }, rawText)},
	{Namespace: "extract", Key: "aliyun_ocr_access_key_secret", ValueType: "string", Default: "", Description: "阿里云 OCR AccessKey Secret", Sensitive: true,
		Validate: maxLength(512), Apply: applyField(func(c *config.Config) *string { return &c.ExtractAliyunOCRAccessKeySecret }, rawText)},
	{Namespace: "extract", Key: "aliyun_ocr_region", ValueType: "string", Default: "cn-hangzhou", Description: "阿里云 OCR 地域",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.ExtractAliyunOCRRegion }, rawText)},
	{Namespace: "extract", Key: "aliyun_ocr_endpoint", ValueType: "string", Default: "ocr-api.cn-hangzhou.aliyuncs.com", Description: "阿里云 OCR 接入点",
		Validate: maxLength(255), Apply: applyField(func(c *config.Config) *string { return &c.ExtractAliyunOCREndpoint }, rawText)},
	{Namespace: "extract", Key: "aliyun_ocr_timeout_seconds", ValueType: "int", Default: "60", Description: "阿里云 OCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractAliyunOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "mineru_source", ValueType: "string", Default: "cloud", Description: "MinerU 服务类型(cloud/self_hosted)",
		Validate: oneOf(extractport.MinerUSourceCloud, extractport.MinerUSourceSelfHosted), Apply: applyField(func(c *config.Config) *string { return &c.ExtractMinerUSource }, rawText)},
	{Namespace: "extract", Key: "mineru_base_url", ValueType: "string", Default: "https://mineru.net/api/v4", Description: "MinerU 服务地址，默认 https://mineru.net/api/v4",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractMinerUBaseURL }, rawText)},
	{Namespace: "extract", Key: "mineru_file_types", ValueType: "string", Default: "pdf,word,presentation", Description: "MinerU 处理的文件类型，逗号分隔：pdf,word,presentation,excel",
		Validate: validateMinerUFileTypes, Apply: applyField(func(c *config.Config) *string { return &c.ExtractMinerUFileTypes }, rawText)},
	{Namespace: "extract", Key: "mineru_auth_token", ValueType: "string", Default: "", Description: "MinerU 鉴权 Token", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractMinerUAuthToken }, rawText)},
	{Namespace: "extract", Key: "mineru_timeout_seconds", ValueType: "int", Default: "180", Description: "MinerU 请求超时(秒)，默认 180s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractMinerUTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "mistral_ocr_base_url", ValueType: "string", Default: "https://api.mistral.ai/v1/ocr", Description: "Mistral OCR 服务地址，默认 https://api.mistral.ai/v1/ocr",
		Validate: optionalTrustedHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractMistralOCRBaseURL }, rawText)},
	{Namespace: "extract", Key: "mistral_ocr_auth_token", ValueType: "string", Default: "", Description: "Mistral OCR API Key", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractMistralOCRAuthToken }, rawText)},
	{Namespace: "extract", Key: "mistral_ocr_model", ValueType: "string", Default: "mistral-ocr-latest", Description: "Mistral OCR 请求模型，默认 mistral-ocr-latest",
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractMistralOCRModel }, rawText)},
	{Namespace: "extract", Key: "mistral_ocr_timeout_seconds", ValueType: "int", Default: "60", Description: "Mistral OCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractMistralOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "llm_ocr_base_url", ValueType: "string", Default: "", Description: "LLM OCR 服务地址（OpenAI 兼容 chat/completions 视觉模型）",
		Validate: optionalHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.ExtractLLMOCRBaseURL }, rawText)},
	{Namespace: "extract", Key: "llm_ocr_model", ValueType: "string", Default: "", Description: "LLM OCR 请求模型",
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractLLMOCRModel }, rawText)},
	{Namespace: "extract", Key: "llm_ocr_auth_token", ValueType: "string", Default: "", Description: "LLM OCR 鉴权 Token / API Key", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractLLMOCRAuthToken }, rawText)},
	{Namespace: "extract", Key: "llm_ocr_timeout_seconds", ValueType: "int", Default: "60", Description: "LLM OCR 请求超时(秒)，默认 60s",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.ExtractLLMOCRTimeoutSeconds }, toInt)},
	{Namespace: "extract", Key: "llm_ocr_prompt", ValueType: "string", Default: "", Description: "LLM OCR 系统提示词",
		Apply: applyField(func(c *config.Config) *string { return &c.ExtractLLMOCRPrompt }, rawText)},

	// Embedding 与 RAG 配置
	{Namespace: "file", Key: "embedding_enabled", ValueType: "bool", Default: "false", Description: "是否启用 Embedding 服务",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmbeddingEnabled }, toBool)},
	{Namespace: "file", Key: "embedding_host", ValueType: "string", Default: "", Description: "Embedding HTTP 服务地址，本地或远程均可",
		Validate: optionalTrustedHTTPURL(), Apply: applyField(func(c *config.Config) *string { return &c.EmbeddingHost }, rawText)},
	{Namespace: "file", Key: "embedding_key", ValueType: "string", Default: "", Description: "Embedding HTTP 服务鉴权 Key，可留空", Sensitive: true,
		Apply: applyField(func(c *config.Config) *string { return &c.EmbeddingKey }, rawText)},
	{Namespace: "file", Key: "embedding_timeout_seconds", ValueType: "int", Default: "60", Description: "Embedding 请求超时时间(秒)",
		Validate: intRange(1, 600), Apply: applyField(func(c *config.Config) *int { return &c.EmbeddingTimeoutSeconds }, toInt)},
	{Namespace: "file", Key: "embedding_output_dimensions", ValueType: "int", Default: "1536", Description: "写库和检索统一使用的向量维度",
		Validate: intRange(64, 4096), Apply: applyField(func(c *config.Config) *int { return &c.EmbeddingOutputDimensions }, toInt)},
	{Namespace: "file", Key: "embedding_normalize", ValueType: "bool", Default: "true", Description: "是否归一化Embedding向量",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmbeddingNormalize }, toBool)},
	{Namespace: "file", Key: "embedding_model_signature", ValueType: "string", Default: "", Description: "当前生效的 Embedding 向量空间标识（系统自动维护，勿手动修改）",
		Apply: applyField(func(c *config.Config) *string { return &c.EmbeddingModelSignature }, rawText)},
	{Namespace: "file", Key: "embed_trigger_on_upload", ValueType: "bool", Default: "true", Description: "上传后异步触发embedding",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.EmbedTriggerOnUpload }, toBool)},
	{Namespace: "file", Key: "embed_chunk_size_tokens", ValueType: "int", Default: "1024", Description: "RAG分片大小(token估算)",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.EmbedChunkSizeTokens }, toInt)},
	{Namespace: "file", Key: "embed_chunk_overlap_tokens", ValueType: "int", Default: "64", Description: "分片重叠token数",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.EmbedChunkOverlapTokens }, toInt)},
	{Namespace: "file", Key: "embed_batch_size", ValueType: "int", Default: "20", Description: "Embedding单批处理文本数",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.EmbedBatchSize }, toInt)},
	{Namespace: "file", Key: "rag_top_k", ValueType: "int", Default: "5", Description: "RAG检索返回片段数",
		Validate: intRange(1, 50), Apply: applyField(func(c *config.Config) *int { return &c.RAGTopK }, toInt)},
	{Namespace: "file", Key: "rag_model", ValueType: "string", Default: defaultRAGModel, Description: "Embedding使用的模型名",
		Apply: applyField(func(c *config.Config) *string { return &c.RAGModel }, rawText)},
	{Namespace: "chat", Key: "rag_enabled", ValueType: "bool", Default: "false", Description: "全局开关：是否允许RAG功能（需 Embedding 服务就绪）",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.RAGEnabled }, toBool)},
	{Namespace: "chat", Key: "rag_min_similarity", ValueType: "string", Default: "0.45", Description: "RAG最低相似度阈值",
		Validate: floatRange(0.000001, 1), Apply: applyField(func(c *config.Config) *float64 { return &c.RAGMinSimilarity }, toFloat)},
	{Namespace: "chat", Key: "rag_token_budget", ValueType: "int", Default: "2000", Description: "RAG注入token预算",
		Validate: intRange(128, 100000), Apply: applyField(func(c *config.Config) *int { return &c.RAGTokenBudget }, toInt)},
	{Namespace: "chat", Key: "rag_fetch_multiplier", ValueType: "int", Default: "3", Description: "RAG检索抓取倍数",
		Validate: intRange(1, 20), Apply: applyField(func(c *config.Config) *int { return &c.RAGFetchMultiplier }, toInt)},
	{Namespace: "chat", Key: "rag_wait_ready_ms", ValueType: "int", Default: "3000", Description: "发送时等待embedding就绪时长(ms)",
		Validate: intRange(1000, 120000), Apply: applyField(func(c *config.Config) *int { return &c.RAGWaitReadyMS }, toInt)},
	{Namespace: "chat", Key: "rag_query_history_turns", ValueType: "int", Default: "0", Description: "RAG查询带入最近用户轮次",
		Validate: intRange(0, 20), Apply: applyField(func(c *config.Config) *int { return &c.RAGQueryHistoryTurns }, toInt)},
	{Namespace: "chat", Key: "rag_retrieval_cache_ttl_seconds", ValueType: "int", Default: "120", Description: "RAG检索缓存TTL(秒)",
		Validate: intRange(0, 86400), Apply: applyField(func(c *config.Config) *int { return &c.RAGRetrievalCacheTTL }, toInt)},
	{Namespace: "chat", Key: "rag_hybrid_enabled", ValueType: "bool", Default: "false", Description: "启用混合检索（BM25+向量 RRF 合并），可提升召回率",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.RAGHybridEnabled }, toBool)},

	// 上下文压缩与语义增强
	{Namespace: "chat", Key: "context_compact_highlights_per_role", ValueType: "int", Default: "6", Description: "上下文压缩每个角色保留亮点数",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.ContextCompactHighlightsPerRole }, toInt)},
	{Namespace: "chat", Key: "context_compact_snippet_chars", ValueType: "int", Default: "140", Description: "上下文压缩片段最大字符数",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.ContextCompactSnippetChars }, toInt)},
	{Namespace: "chat", Key: "compact_llm_enabled", ValueType: "bool", Default: "true", Description: "启用 LLM 语义压缩（4级回退中的 Level 2/3），关闭后仅使用模板摘要",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.CompactLLMEnabled }, toBool)},
	{Namespace: "chat", Key: "compact_task_model", ValueType: "string", Default: "follow", Description: "上下文压缩任务使用的聊天模型，follow 表示跟随当前会话模型；非聊天模型会回退到默认聊天模型",
		Apply: applyField(func(c *config.Config) *string { return &c.CompactTaskModel }, rawText)},
	{Namespace: "chat", Key: "compact_async_enabled", ValueType: "bool", Default: "true", Description: "将压缩移出响应关键路径，异步执行，减少用户等待",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.CompactAsyncEnabled }, toBool)},
	{Namespace: "chat", Key: "compact_max_failures", ValueType: "int", Default: "3", Description: "LLM 压缩熔断阈值：连续失败次数超出后自动降级为模板压缩",
		Validate: integerValue(), Apply: applyField(func(c *config.Config) *int { return &c.CompactMaxFailures }, toInt)},
	{Namespace: "chat", Key: "compact_system_prompt", ValueType: "string", Default: "", Description: "全量摘要提示词，支持 {{FROM_TURN}}、{{TO_TURN}} 占位符；空串使用内置默认值",
		Apply: applyField(func(c *config.Config) *string { return &c.CompactSystemPrompt }, rawText)},
	{Namespace: "chat", Key: "compact_light_prompt", ValueType: "string", Default: "", Description: "轻量摘要提示词，支持 {{FROM_TURN}}、{{TO_TURN}} 占位符；空串使用内置默认值",
		Apply: applyField(func(c *config.Config) *string { return &c.CompactLightPrompt }, rawText)},
	{Namespace: "chat", Key: "context_token_budget_enabled", ValueType: "bool", Default: "true", Description: "按模型 Token 预算截断上下文（替代消息数截断），更精准地利用上下文窗口",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ContextTokenBudgetEnabled }, toBool)},
	{Namespace: "chat", Key: "message_embedding_enabled", ValueType: "bool", Default: "false", Description: "每轮对话结束后异步生成消息向量（需 Embedding 服务就绪）",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.MessageEmbeddingEnabled }, toBool)},
	{Namespace: "chat", Key: "semantic_context_enabled", ValueType: "bool", Default: "false", Description: "发送消息时语义召回历史相关片段注入上下文（需 message_embedding_enabled 开启）",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.SemanticContextEnabled }, toBool)},
	{Namespace: "chat", Key: "process_trace_enabled", ValueType: "bool", Default: "true", Description: "启用聊天页处理轨迹",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ProcessTraceEnabled }, toBool)},
	{Namespace: "chat", Key: "process_trace_visible_to_user", ValueType: "bool", Default: "true", Description: "向聊天页展示处理轨迹",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ProcessTraceVisibleToUser }, toBool)},
	{Namespace: "chat", Key: "process_trace_store_upstream_think", ValueType: "bool", Default: "true", Description: "持久化模型思考原文",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ProcessTraceStoreUpstreamThink }, toBool)},
	{Namespace: "chat", Key: "process_trace_persist_inflight", ValueType: "bool", Default: "true", Description: "流式阶段增量持久化处理轨迹",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.ProcessTracePersistInflight }, toBool)},
	{Namespace: "chat", Key: "context_artifact_retention_days", ValueType: "int", Default: "90", Description: "上下文证据保留天数，<=0 表示不自动过期",
		Validate: intRange(0, 3650), Apply: applyField(func(c *config.Config) *int { return &c.ContextArtifactRetentionDays }, toInt)},

	// MCP 配置
	{Namespace: "mcp", Key: "mcp_enable", ValueType: "bool", Default: "false", Description: "启用 MCP 工具",
		Validate: boolValue(), Apply: applyField(func(c *config.Config) *bool { return &c.MCPEnable }, toBool)},
	{Namespace: "mcp", Key: "mcp_tool_timeout_seconds", ValueType: "int", Default: "10", Description: "MCP Tool Call 超时(秒)",
		Validate: intRange(0, maxMCPToolTimeoutSeconds), Apply: applyField(func(c *config.Config) *int { return &c.MCPToolTimeoutSeconds }, toInt)},
	{Namespace: "mcp", Key: "mcp_tool_retry_count", ValueType: "int", Default: "0", Description: "MCP Tool Call 重试次数",
		Validate: intRange(0, 5), Apply: applyField(func(c *config.Config) *int { return &c.MCPToolRetryCount }, toInt)},
	{Namespace: "mcp", Key: "mcp_max_concurrent_calls", ValueType: "int", Default: "8", Description: "MCP Tool Call 并发上限",
		Validate: intRange(1, 64), Apply: applyField(func(c *config.Config) *int { return &c.MCPMaxConcurrentCalls }, toInt)},
	{Namespace: "mcp", Key: "mcp_max_selected_tools_per_message", ValueType: "int", Default: "32", Description: "单次消息最多可选择的 MCP 工具数量",
		Validate: intRange(1, config.MaxMCPSelectedToolsPerMessage), Apply: applyField(func(c *config.Config) *int { return &c.MCPMaxSelectedToolsPerMessage }, toInt)},
	{Namespace: "mcp", Key: "mcp_max_llm_calls_per_run", ValueType: "int", Default: "5", Description: "单次 MCP 工具运行最大 LLM 请求次数（最小 2，首次请求 + 工具后续请求 + 最终总结）",
		Validate: intRange(2, 32), Apply: applyField(func(c *config.Config) *int { return &c.MCPMaxLLMCallsPerRun }, toInt)},
	{Namespace: "mcp", Key: "mcp_max_tool_calls_per_run", ValueType: "int", Default: "8", Description: "单次 MCP 工具运行最大 MCP Tool Call 次数",
		Validate: intRange(1, 64), Apply: applyField(func(c *config.Config) *int { return &c.MCPMaxToolCallsPerRun }, toInt)},
	{Namespace: "mcp", Key: "mcp_tool_prompt", ValueType: "string", Default: "", Description: "MCP Tool 调用提示词；空串使用内置默认值",
		Validate: maxLength(20000), Apply: applyField(func(c *config.Config) *string { return &c.MCPToolPrompt }, rawText)},

	// 熔断配置：由渠道模块在运行时按 namespace 读取，不进入 config.Config。
	{Namespace: "circuit", Key: "channel_failure_threshold", ValueType: "int", Default: "3", Description: "熔断触发次数",
		Validate: integerValue()},
	{Namespace: "circuit", Key: "channel_failure_window_seconds", ValueType: "int", Default: "120", Description: "计数窗口(秒)",
		Validate: integerValue()},
	{Namespace: "circuit", Key: "channel_circuit_open_seconds", ValueType: "int", Default: "60", Description: "熔断持续时间(秒)",
		Validate: integerValue()},
}

var settingSpecIndex = indexSettingSpecs(settingSpecs)

func indexSettingSpecs(specs []settingSpec) map[string]settingSpec {
	index := make(map[string]settingSpec, len(specs))
	for _, spec := range specs {
		index[spec.fullKey()] = spec
	}
	return index
}

func lookupSettingSpec(namespace string, key string) (settingSpec, bool) {
	spec, ok := settingSpecIndex[namespace+":"+key]
	return spec, ok
}

// IsValidNamespace 判断 namespace 是否允许被动态配置。
func IsValidNamespace(namespace string) bool {
	for _, spec := range settingSpecs {
		if spec.Namespace == namespace {
			return true
		}
	}
	return false
}

// IsSensitiveSetting 判断配置项是否为加密存储、对外脱敏的敏感项。
func IsSensitiveSetting(namespace string, key string) bool {
	spec, ok := lookupSettingSpec(strings.TrimSpace(namespace), strings.TrimSpace(key))
	return ok && spec.Sensitive
}

func isSensitiveSetting(namespace string, key string) bool {
	spec, ok := lookupSettingSpec(namespace, key)
	return ok && spec.Sensitive
}

// validateSettingValue 用注册表中的规则校验一个配置值；未注册的 key 直接判为非法。
func validateSettingValue(namespace string, key string, value string) error {
	spec, ok := lookupSettingSpec(namespace, key)
	if !ok {
		return newSettingValidationError(settingCodeInvalidKey, SettingValidationDetails{Rule: "invalid_key"})
	}
	if spec.Validate == nil {
		return nil
	}
	if err := spec.Validate(strings.TrimSpace(value), spec.fullKey()); err != nil {
		var validationErr *SettingValidationError
		if errors.As(err, &validationErr) {
			return validationErr
		}
		return settingValidationForKey(namespace, key, err)
	}
	return nil
}

// defaultSettings 返回所有动态配置的默认种子数据。
func defaultSettings() []domainsettings.SystemSetting {
	items := make([]domainsettings.SystemSetting, 0, len(settingSpecs))
	for _, spec := range settingSpecs {
		items = append(items, spec.seedSetting())
	}
	return items
}
