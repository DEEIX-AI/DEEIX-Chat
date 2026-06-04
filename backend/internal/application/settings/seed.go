package settings

import (
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

const (
	defaultAllowedMIMETypes = "image/jpeg,image/png,image/webp,image/gif,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel"
	defaultRAGModel         = "sentence-transformers/all-MiniLM-L6-v2"
	defaultLoginPageTitle   = "Sign in to DEEIX Chat"
)

// defaultSettings 返回所有动态配置的默认种子数据。
func defaultSettings() []domainsettings.SystemSetting {
	return []domainsettings.SystemSetting{
		// 认证配置
		{Namespace: "auth", Key: "token_ttl_hours", Value: "24", ValueType: "int", Description: "Access Token 有效期(小时)"},
		{Namespace: "auth", Key: "refresh_token_ttl_hours", Value: "720", ValueType: "int", Description: "Refresh Token 有效期(小时)"},
		{Namespace: "auth", Key: "login_max_failures", Value: "5", ValueType: "int", Description: "登录失败锁定阈值"},
		{Namespace: "auth", Key: "login_lock_minutes", Value: "15", ValueType: "int", Description: "锁定时长(分钟)"},
		{Namespace: "auth", Key: "rate_limit_enabled", Value: "false", ValueType: "bool", Description: "是否启用平台 HTTP 429 限流"},
		{Namespace: "auth", Key: "rate_limit_rpm", Value: "60", ValueType: "int", Description: "全局限流 RPM"},
		{Namespace: "auth", Key: "public_auth_rate_limit_rpm", Value: "30", ValueType: "int", Description: "公开鉴权接口限流 RPM"},
		{Namespace: "auth", Key: "login_page_title", Value: defaultLoginPageTitle, ValueType: "string", Description: "登录页面标题"},
		{Namespace: "auth", Key: "login_default_next_path", Value: "/chat", ValueType: "string", Description: "无 next 参数时登录成功后的默认跳转路径"},
		{Namespace: "auth", Key: "username_login_enabled", Value: "true", ValueType: "bool", Description: "是否允许用户名密码登录"},
		{Namespace: "auth", Key: "email_login_enabled", Value: "true", ValueType: "bool", Description: "是否允许邮箱登录"},
		{Namespace: "auth", Key: "third_party_login_enabled", Value: "true", ValueType: "bool", Description: "是否启用第三方登录入口"},
		{Namespace: "auth", Key: "email_registration_enabled", Value: "true", ValueType: "bool", Description: "是否允许邮箱注册"},
		{Namespace: "auth", Key: "email_verification_enabled", Value: "false", ValueType: "bool", Description: "邮箱注册时是否要求邮箱验证码"},
		{Namespace: "auth", Key: "smtp_host", Value: "", ValueType: "string", Description: "邮箱验证码 SMTP 主机"},
		{Namespace: "auth", Key: "smtp_port", Value: "587", ValueType: "int", Description: "邮箱验证码 SMTP 端口"},
		{Namespace: "auth", Key: "smtp_username", Value: "", ValueType: "string", Description: "邮箱验证码 SMTP 用户名"},
		{Namespace: "auth", Key: "smtp_password", Value: "", ValueType: "string", Description: "邮箱验证码 SMTP 密码"},
		{Namespace: "auth", Key: "smtp_from", Value: "", ValueType: "string", Description: "邮箱验证码发件人"},
		{Namespace: "auth", Key: "email_registration_allowed_domains", Value: "", ValueType: "string", Description: "邮箱注册域名白名单，留空表示不限制"},
		{Namespace: "auth", Key: "email_registration_block_plus_alias", Value: "false", ValueType: "bool", Description: "邮箱注册是否禁止 + 别名地址"},
		{Namespace: "auth", Key: "auto_link_verified_email", Value: "true", ValueType: "bool", Description: "是否允许相同已验证邮箱自动绑定第三方身份"},
		{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "false", ValueType: "bool", Description: "邮箱注册是否启用 Cloudflare Turnstile 人机验证"},
		{Namespace: "auth", Key: "turnstile_site_key", Value: "", ValueType: "string", Description: "Cloudflare Turnstile Site Key"},
		{Namespace: "auth", Key: "turnstile_secret_key", Value: "", ValueType: "string", Description: "Cloudflare Turnstile Secret Key"},

		// 计费配置
		{Namespace: "billing", Key: "mode", Value: "self", ValueType: "string", Description: "计费方式：self=自用模式，period=周期计费，usage=按量计费"},
		{Namespace: "billing", Key: "prepaid_amount_usd", Value: "0", ValueType: "string", Description: "按量调用前要求账户保留的最低预付余额(美元)"},
		{Namespace: "billing", Key: "native_tool_billing_enabled", Value: "true", ValueType: "bool", Description: "是否按官方默认价格计费模型原生工具调用"},
		{Namespace: "billing", Key: "native_tool_pricing_json", Value: nativetool.DefaultPricingJSON(), ValueType: "json", Description: "官方原生工具计费覆盖 JSON，按 toolKey 配置 priceNanousd、unit、priceLabel、billable"},
		{Namespace: "billing", Key: "usd_to_cny_rate", Value: "7.2", ValueType: "string", Description: "美元换人民币支付汇率"},
		{Namespace: "billing", Key: "payment_providers", Value: "disabled", ValueType: "string", Description: "启用支付渠道，多个用英文逗号分隔：stripe,epay"},
		{Namespace: "billing", Key: "stripe_publishable_key", Value: "", ValueType: "string", Description: "Stripe Publishable Key"},
		{Namespace: "billing", Key: "stripe_secret_key", Value: "", ValueType: "string", Description: "Stripe Secret Key"},
		{Namespace: "billing", Key: "stripe_webhook_secret", Value: "", ValueType: "string", Description: "Stripe Webhook Secret"},
		{Namespace: "billing", Key: "epay_gateway_url", Value: "", ValueType: "string", Description: "易支付网关地址"},
		{Namespace: "billing", Key: "epay_types", Value: `[{"name":"支付宝","type":"alipay"},{"name":"微信支付","type":"wxpay"}]`, ValueType: "string", Description: "易支付启用的支付类型 JSON"},
		{Namespace: "billing", Key: "epay_pid", Value: "", ValueType: "string", Description: "易支付商户 ID"},
		{Namespace: "billing", Key: "epay_key", Value: "", ValueType: "string", Description: "易支付商户密钥"},

		// 对话配置
		{Namespace: "chat", Key: "max_context_messages", Value: "20", ValueType: "int", Description: "上下文消息数"},
		{Namespace: "chat", Key: "context_max_turns", Value: "48", ValueType: "int", Description: "最大对话轮次"},
		{Namespace: "chat", Key: "context_max_input_tokens", Value: "32000", ValueType: "int", Description: "最大输入 token"},
		{Namespace: "chat", Key: "context_compact_trigger_tokens", Value: "32768", ValueType: "int", Description: "压缩触发阈值"},
		{Namespace: "chat", Key: "context_compact_preserve_recent_turns", Value: "8", ValueType: "int", Description: "压缩保留轮次"},
		{Namespace: "chat", Key: "conversation_task_model", Value: "follow", ValueType: "string", Description: "会话标题/标签生成任务使用的聊天模型，follow 表示跟随当前会话模型；图片模型不会用于标题/标签生成"},
		{Namespace: "chat", Key: "conversation_title_prompt", Value: "", ValueType: "string", Description: "会话标题生成提示词，支持 {{MESSAGES}} 占位符；空串使用内置默认值"},
		{Namespace: "chat", Key: "conversation_labels_prompt", Value: "", ValueType: "string", Description: "会话标签生成提示词，支持 {{MESSAGES}} 占位符；空串使用内置默认值"},
		{Namespace: "chat", Key: "default_system_prompt", Value: "", ValueType: "string", Description: "全局默认系统提示词，仅对聊天任务生效；空串表示不注入"},
		{Namespace: "chat", Key: "model_option_policy_mode", Value: "allowlist", ValueType: "string", Description: "模型 options 透传策略：allowlist=仅白名单，denylist=黑名单拦截，disabled=禁止透传"},
		{Namespace: "chat", Key: "model_option_allowed_paths", Value: config.DefaultModelOptionAllowedPathsJSON(), ValueType: "json", Description: "模型 options 白名单路径 JSON，default 对所有协议生效"},
		{Namespace: "chat", Key: "model_option_denied_paths", Value: config.DefaultModelOptionDeniedPathsJSON(), ValueType: "json", Description: "模型 options 黑名单路径 JSON，default 对所有协议生效"},

		// 存储配置
		{Namespace: "storage", Key: "user_storage_quota_bytes", Value: "104857600", ValueType: "int", Description: "用户总存储配额(字节)，0表示不限制"},
		{Namespace: "storage", Key: "max_upload_file_bytes", Value: "20971520", ValueType: "int", Description: "默认附件大小上限(字节)"},
		{Namespace: "storage", Key: "max_message_files", Value: "10", ValueType: "int", Description: "单消息附件数"},

		// 文件处理配置
		{Namespace: "file", Key: "image_max_dimension", Value: "1024", ValueType: "int", Description: "图片发送前缩放最大边长(px)，0=不缩放"},
		{Namespace: "file", Key: "full_context_limit_enabled", Value: "true", ValueType: "bool", Description: "是否启用全文注入大小、Token、PDF页数限制"},
		{Namespace: "file", Key: "file_full_context_max_bytes", Value: "51200", ValueType: "int", Description: "文本文件全文注入最大字节数(50KB)，留空或0表示不限制"},
		{Namespace: "file", Key: "full_context_max_tokens", Value: "12000", ValueType: "int", Description: "全文注入最大token预算，留空或0表示不限制"},
		{Namespace: "file", Key: "image_max_bytes", Value: "", ValueType: "int", Description: "图片单文件大小上限(字节)，留空则回退默认附件大小上限"},
		{Namespace: "file", Key: "doc_max_bytes", Value: "", ValueType: "int", Description: "文档单文件大小上限(字节)，留空则回退默认附件大小上限"},
		{Namespace: "file", Key: "full_context_pdf_max_pages", Value: "20", ValueType: "int", Description: "PDF Full Context最大页数，留空或0表示不限制"},
		{Namespace: "file", Key: "allowed_mime_types", Value: defaultAllowedMIMETypes, ValueType: "string", Description: "白名单MIME类型(逗号分隔)"},
		{Namespace: "extract", Key: "engine", Value: "builtin", ValueType: "string", Description: "提取主引擎枚举(builtin/tika/docling/mineru)"},
		{Namespace: "extract", Key: "ocr_engine", Value: "rapidocr", ValueType: "string", Description: "OCR 引擎枚举(rapidocr/tesseract/paddle/tencent/aliyun/llm)"},
		{Namespace: "extract", Key: "image_ocr_enabled", Value: "false", ValueType: "bool", Description: "是否对图片附件执行 OCR"},
		{Namespace: "extract", Key: "pdf_ocr_fallback_enabled", Value: "false", ValueType: "bool", Description: "PDF 原生文本提取失败或质量较差时是否启用 OCR 回退"},
		{Namespace: "extract", Key: "tika_source", Value: "external", ValueType: "string", Description: "Tika 服务来源枚举(external/managed)"},
		{Namespace: "extract", Key: "tika_base_url", Value: "http://127.0.0.1:9998", ValueType: "string", Description: "Apache Tika 服务地址，默认 http://127.0.0.1:9998"},
		{Namespace: "extract", Key: "tika_timeout_seconds", Value: "60", ValueType: "int", Description: "Apache Tika 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "tika_auth_token", Value: "", ValueType: "string", Description: "Apache Tika 鉴权 Token"},
		{Namespace: "extract", Key: "docling_base_url", Value: "http://127.0.0.1:8005/ocr", ValueType: "string", Description: "Docling 服务地址，默认 http://127.0.0.1:8005/ocr"},
		{Namespace: "extract", Key: "docling_auth_token", Value: "", ValueType: "string", Description: "Docling 鉴权 Token"},
		{Namespace: "extract", Key: "docling_timeout_seconds", Value: "60", ValueType: "int", Description: "Docling 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "tesseract_ocr_base_url", Value: "http://127.0.0.1:8004/ocr", ValueType: "string", Description: "Tesseract OCR 服务地址，默认 http://127.0.0.1:8004/ocr"},
		{Namespace: "extract", Key: "tesseract_ocr_auth_token", Value: "", ValueType: "string", Description: "Tesseract OCR 鉴权 Token"},
		{Namespace: "extract", Key: "tesseract_ocr_timeout_seconds", Value: "60", ValueType: "int", Description: "Tesseract OCR 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "rapidocr_source", Value: "external", ValueType: "string", Description: "RapidOCR 服务来源枚举(external/managed)"},
		{Namespace: "extract", Key: "rapidocr_base_url", Value: "http://127.0.0.1:8002/ocr", ValueType: "string", Description: "RapidOCR 服务地址，默认 http://127.0.0.1:8002/ocr"},
		{Namespace: "extract", Key: "rapidocr_auth_token", Value: "", ValueType: "string", Description: "RapidOCR 鉴权 Token"},
		{Namespace: "extract", Key: "rapidocr_timeout_seconds", Value: "60", ValueType: "int", Description: "RapidOCR 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "paddle_ocr_base_url", Value: "", ValueType: "string", Description: "Paddle OCR 服务地址"},
		{Namespace: "extract", Key: "paddle_ocr_auth_token", Value: "", ValueType: "string", Description: "Paddle OCR 鉴权 Token"},
		{Namespace: "extract", Key: "paddle_ocr_timeout_seconds", Value: "60", ValueType: "int", Description: "Paddle OCR 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "tencent_ocr_secret_id", Value: "", ValueType: "string", Description: "腾讯云 OCR SecretId"},
		{Namespace: "extract", Key: "tencent_ocr_secret_key", Value: "", ValueType: "string", Description: "腾讯云 OCR SecretKey"},
		{Namespace: "extract", Key: "tencent_ocr_region", Value: "ap-guangzhou", ValueType: "string", Description: "腾讯云 OCR 地域"},
		{Namespace: "extract", Key: "tencent_ocr_endpoint", Value: "ocr.tencentcloudapi.com", ValueType: "string", Description: "腾讯云 OCR 接入点"},
		{Namespace: "extract", Key: "tencent_ocr_timeout_seconds", Value: "60", ValueType: "int", Description: "腾讯云 OCR 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "aliyun_ocr_access_key_id", Value: "", ValueType: "string", Description: "阿里云 OCR AccessKey ID"},
		{Namespace: "extract", Key: "aliyun_ocr_access_key_secret", Value: "", ValueType: "string", Description: "阿里云 OCR AccessKey Secret"},
		{Namespace: "extract", Key: "aliyun_ocr_region", Value: "cn-hangzhou", ValueType: "string", Description: "阿里云 OCR 地域"},
		{Namespace: "extract", Key: "aliyun_ocr_endpoint", Value: "ocr-api.cn-hangzhou.aliyuncs.com", ValueType: "string", Description: "阿里云 OCR 接入点"},
		{Namespace: "extract", Key: "aliyun_ocr_timeout_seconds", Value: "60", ValueType: "int", Description: "阿里云 OCR 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "mineru_source", Value: "cloud", ValueType: "string", Description: "MinerU 服务类型(cloud/self_hosted)"},
		{Namespace: "extract", Key: "mineru_base_url", Value: "https://mineru.net/api/v4", ValueType: "string", Description: "MinerU 服务地址，默认 https://mineru.net/api/v4"},
		{Namespace: "extract", Key: "mineru_auth_token", Value: "", ValueType: "string", Description: "MinerU 鉴权 Token"},
		{Namespace: "extract", Key: "mineru_timeout_seconds", Value: "180", ValueType: "int", Description: "MinerU 请求超时(秒)，默认 180s"},
		{Namespace: "extract", Key: "llm_ocr_base_url", Value: "", ValueType: "string", Description: "LLM OCR 服务地址（OpenAI 兼容 chat/completions 视觉模型）"},
		{Namespace: "extract", Key: "llm_ocr_model", Value: "", ValueType: "string", Description: "LLM OCR 请求模型"},
		{Namespace: "extract", Key: "llm_ocr_auth_token", Value: "", ValueType: "string", Description: "LLM OCR 鉴权 Token / API Key"},
		{Namespace: "extract", Key: "llm_ocr_timeout_seconds", Value: "60", ValueType: "int", Description: "LLM OCR 请求超时(秒)，默认 60s"},
		{Namespace: "extract", Key: "llm_ocr_prompt", Value: "", ValueType: "string", Description: "LLM OCR 系统提示词"},
		{Namespace: "file", Key: "embedding_enabled", Value: "false", ValueType: "bool", Description: "是否启用 Embedding 服务"},
		{Namespace: "file", Key: "embedding_host", Value: "", ValueType: "string", Description: "Embedding HTTP 服务地址，本地或远程均可"},
		{Namespace: "file", Key: "embedding_key", Value: "", ValueType: "string", Description: "Embedding HTTP 服务鉴权 Key，可留空"},
		{Namespace: "file", Key: "embedding_timeout_seconds", Value: "60", ValueType: "int", Description: "Embedding 请求超时时间(秒)"},
		{Namespace: "file", Key: "embedding_output_dimensions", Value: "1536", ValueType: "int", Description: "写库和检索统一使用的向量维度"},
		{Namespace: "file", Key: "embedding_normalize", Value: "true", ValueType: "bool", Description: "是否归一化Embedding向量"},
		{Namespace: "file", Key: "embedding_model_signature", Value: "", ValueType: "string", Description: "当前生效的 Embedding 模型签名（系统自动维护，格式: model@dims，勿手动修改）"},
		{Namespace: "file", Key: "embed_trigger_on_upload", Value: "true", ValueType: "bool", Description: "上传后异步触发embedding"},
		{Namespace: "file", Key: "embed_chunk_size_tokens", Value: "1024", ValueType: "int", Description: "RAG分片大小(token估算)"},
		{Namespace: "file", Key: "embed_chunk_overlap_tokens", Value: "64", ValueType: "int", Description: "分片重叠token数"},
		{Namespace: "file", Key: "embed_batch_size", Value: "20", ValueType: "int", Description: "Embedding单批处理文本数"},
		{Namespace: "file", Key: "rag_top_k", Value: "5", ValueType: "int", Description: "RAG检索返回片段数"},
		{Namespace: "file", Key: "rag_model", Value: defaultRAGModel, ValueType: "string", Description: "Embedding使用的模型名"},
		// chat（补充）
		{Namespace: "chat", Key: "rag_enabled", Value: "false", ValueType: "bool", Description: "全局开关：是否允许RAG功能（需 Embedding 服务就绪）"},
		{Namespace: "chat", Key: "rag_min_similarity", Value: "0.45", ValueType: "string", Description: "RAG最低相似度阈值"},
		{Namespace: "chat", Key: "rag_token_budget", Value: "2000", ValueType: "int", Description: "RAG注入token预算"},
		{Namespace: "chat", Key: "rag_fetch_multiplier", Value: "3", ValueType: "int", Description: "RAG检索抓取倍数"},
		{Namespace: "chat", Key: "rag_wait_ready_ms", Value: "3000", ValueType: "int", Description: "发送时等待embedding就绪时长(ms)"},
		{Namespace: "chat", Key: "rag_query_history_turns", Value: "0", ValueType: "int", Description: "RAG查询带入最近用户轮次"},
		{Namespace: "chat", Key: "rag_retrieval_cache_ttl_seconds", Value: "120", ValueType: "int", Description: "RAG检索缓存TTL(秒)"},
		{Namespace: "chat", Key: "rag_hybrid_enabled", Value: "false", ValueType: "bool", Description: "启用混合检索（BM25+向量 RRF 合并），可提升召回率"},
		{Namespace: "chat", Key: "context_compact_highlights_per_role", Value: "6", ValueType: "int", Description: "上下文压缩每个角色保留亮点数"},
		{Namespace: "chat", Key: "context_compact_snippet_chars", Value: "140", ValueType: "int", Description: "上下文压缩片段最大字符数"},
		// 上下文压缩增强
		{Namespace: "chat", Key: "compact_llm_enabled", Value: "true", ValueType: "bool", Description: "启用 LLM 语义压缩（4级回退中的 Level 2/3），关闭后仅使用模板摘要"},
		{Namespace: "chat", Key: "compact_task_model", Value: "follow", ValueType: "string", Description: "上下文压缩任务使用的聊天模型，follow 表示跟随当前会话模型；非聊天模型会回退到默认聊天模型"},
		{Namespace: "chat", Key: "compact_async_enabled", Value: "true", ValueType: "bool", Description: "将压缩移出响应关键路径，异步执行，减少用户等待"},
		{Namespace: "chat", Key: "compact_max_failures", Value: "3", ValueType: "int", Description: "LLM 压缩熔断阈值：连续失败次数超出后自动降级为模板压缩"},
		{Namespace: "chat", Key: "compact_system_prompt", Value: "", ValueType: "string", Description: "全量摘要提示词，支持 {{FROM_TURN}}、{{TO_TURN}} 占位符；空串使用内置默认值"},
		{Namespace: "chat", Key: "compact_light_prompt", Value: "", ValueType: "string", Description: "轻量摘要提示词，支持 {{FROM_TURN}}、{{TO_TURN}} 占位符；空串使用内置默认值"},
		{Namespace: "chat", Key: "context_token_budget_enabled", Value: "true", ValueType: "bool", Description: "按模型 Token 预算截断上下文（替代消息数截断），更精准地利用上下文窗口"},
		// 语义增强
		{Namespace: "chat", Key: "message_embedding_enabled", Value: "false", ValueType: "bool", Description: "每轮对话结束后异步生成消息向量（需 Embedding 服务就绪）"},
		{Namespace: "chat", Key: "semantic_context_enabled", Value: "false", ValueType: "bool", Description: "发送消息时语义召回历史相关片段注入上下文（需 message_embedding_enabled 开启）"},
		{Namespace: "chat", Key: "process_trace_enabled", Value: "true", ValueType: "bool", Description: "启用聊天页处理轨迹"},
		{Namespace: "chat", Key: "process_trace_visible_to_user", Value: "true", ValueType: "bool", Description: "向聊天页展示处理轨迹"},
		{Namespace: "chat", Key: "process_trace_store_upstream_think", Value: "true", ValueType: "bool", Description: "持久化模型思考原文"},
		{Namespace: "chat", Key: "process_trace_persist_inflight", Value: "true", ValueType: "bool", Description: "流式阶段增量持久化处理轨迹"},
		{Namespace: "chat", Key: "context_artifact_retention_days", Value: "90", ValueType: "int", Description: "上下文证据保留天数，<=0 表示不自动过期"},
		// MCP 配置
		{Namespace: "mcp", Key: "mcp_enable", Value: "false", ValueType: "bool", Description: "启用 MCP 工具"},
		{Namespace: "mcp", Key: "mcp_tool_timeout_seconds", Value: "10", ValueType: "int", Description: "MCP Tool Call 超时(秒)"},
		{Namespace: "mcp", Key: "mcp_tool_retry_count", Value: "0", ValueType: "int", Description: "MCP Tool Call 重试次数"},
		{Namespace: "mcp", Key: "mcp_max_concurrent_calls", Value: "8", ValueType: "int", Description: "MCP Tool Call 并发上限"},
		{Namespace: "mcp", Key: "mcp_max_selected_tools_per_message", Value: "32", ValueType: "int", Description: "单次消息最多可选择的 MCP 工具数量"},
		{Namespace: "mcp", Key: "mcp_max_llm_calls_per_run", Value: "5", ValueType: "int", Description: "单次 MCP 工具运行最大 LLM 请求次数（最小 2，首次请求 + 工具后续请求 + 最终总结）"},
		{Namespace: "mcp", Key: "mcp_max_tool_calls_per_run", Value: "8", ValueType: "int", Description: "单次 MCP 工具运行最大 MCP Tool Call 次数"},

		// 熔断配置
		{Namespace: "circuit", Key: "channel_failure_threshold", Value: "3", ValueType: "int", Description: "熔断触发次数"},
		{Namespace: "circuit", Key: "channel_failure_window_seconds", Value: "120", ValueType: "int", Description: "计数窗口(秒)"},
		{Namespace: "circuit", Key: "channel_circuit_open_seconds", Value: "60", ValueType: "int", Description: "熔断持续时间(秒)"},
	}
}

func defaultSettingsWithConfig(cfg config.Config) []domainsettings.SystemSetting {
	return defaultSettings()
}

func obsoleteSettings() []domainsettings.SystemSetting {
	return []domainsettings.SystemSetting{
		{Namespace: "mcp", Key: "mcp_connect_timeout_ms"},
		{Namespace: "mcp", Key: "mcp_tool_timeout_ms"},
	}
}
