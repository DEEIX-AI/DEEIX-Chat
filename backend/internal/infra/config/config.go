package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

const (
	defaultJWTSecret                    = "deeix-chat-dev-secret"
	defaultDataEncryptionKey            = "deeix-chat-dev-data-encryption-key"
	defaultAdminUsername                = "admin"
	defaultAdminDisplayName             = "System Admin"
	defaultGeoIPMaxBytes                = 100 * 1024 * 1024
	defaultHTTPReadHeaderTimeoutSeconds = 10
	defaultHTTPReadTimeoutSeconds       = 120
	defaultHTTPIdleTimeoutSeconds       = 120
	defaultHTTPMaxHeaderBytes           = 1 << 20
)

const (
	// DefaultTurnstileSiteverifyURL 是 Cloudflare Turnstile 默认校验端点。
	DefaultTurnstileSiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// DefaultMCPMaxSelectedToolsPerMessage 是单次消息可选择 MCP 工具数量的默认值。
	DefaultMCPMaxSelectedToolsPerMessage = 32
	// MaxMCPSelectedToolsPerMessage 是运行时配置允许的安全上限，防止一次请求暴露过多工具 schema。
	MaxMCPSelectedToolsPerMessage = 128
)

// DefaultModelOptionAllowedPathsJSON 返回用户可透传模型参数的默认白名单。
func DefaultModelOptionAllowedPathsJSON() string {
	return `{
  "default": [
    "temperature",
    "top_p",
    "max_tokens",
    "max_output_tokens",
    "max_completion_tokens",
    "stop",
    "response_format.type"
  ],
  "openai_chat_completions": [
    "service_tier",
    "presence_penalty",
    "frequency_penalty",
    "reasoning_effort",
    "verbosity",
    "thinking.type",
    "stream_options.include_usage"
  ],
  "openai_responses": [
    "service_tier",
    "reasoning.effort",
    "reasoning.summary",
    "text.verbosity"
  ],
  "openai_image_generations": [
    "background",
    "moderation",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "style",
    "user"
  ],
  "openai_image_edits": [
    "background",
    "input_fidelity",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "user"
  ],
  "google_image_generation": [
    "generationConfig.responseModalities",
    "generationConfig.imageConfig.aspectRatio",
    "generationConfig.imageConfig.imageSize"
  ],
  "anthropic_messages": [
    "speed",
    "top_k",
    "thinking.type",
    "thinking.budget_tokens"
  ],
  "xai_responses": [
    "reasoning.effort"
  ],
  "xai_image": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "xai_image_edits": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "gemini_generate_content": [
    "generationConfig.temperature",
    "generationConfig.topP",
    "generationConfig.maxOutputTokens",
    "generationConfig.responseMimeType"
  ]
}`
}

// DefaultModelOptionDeniedPathsJSON 返回所有策略模式都会叠加拦截的默认黑名单。
func DefaultModelOptionDeniedPathsJSON() string {
	return `{
  "default": [
    "model",
    "messages",
    "input",
    "instructions",
    "prompt",
    "system",
    "systemInstruction",
    "headers",
    "api_key",
    "apiKey",
    "base_url",
    "baseURL",
    "stream",
    "previous_response_id"
  ]
}`
}

// yamlConfig 对应 config.yaml 的字段映射（仅保留基础设施与安全凭据）。
type yamlConfig struct {
	sourceDir string

	App struct {
		Name string `yaml:"name"`
		Env  string `yaml:"env"`
	} `yaml:"app"`
	Server struct {
		HTTPPort                 string `yaml:"http_port"`
		CORSAllowOrigin          string `yaml:"cors_allow_origin"`
		TrustedProxies           string `yaml:"trusted_proxies"`
		PublicAPIBaseURL         string `yaml:"public_api_base_url"`
		PublicWebBaseURL         string `yaml:"public_web_base_url"`
		FrontendDistDir          string `yaml:"frontend_dist_dir"`
		ReadHeaderTimeoutSeconds int    `yaml:"read_header_timeout_seconds"`
		ReadTimeoutSeconds       int    `yaml:"read_timeout_seconds"`
		IdleTimeoutSeconds       int    `yaml:"idle_timeout_seconds"`
		MaxHeaderBytes           int    `yaml:"max_header_bytes"`
	} `yaml:"server"`
	Security struct {
		JWTSecret              string `yaml:"jwt_secret"`
		DataEncryptionKey      string `yaml:"data_encryption_key"`
		SSRFProtectionEnabled  *bool  `yaml:"ssrf_protection_enabled"`
		TurnstileSiteverifyURL string `yaml:"turnstile_siteverify_url"`
	} `yaml:"security"`
	Database struct {
		Driver   string `yaml:"driver"`
		Postgres struct {
			DSN                string `yaml:"dsn"`
			MaxOpenConns       int    `yaml:"max_open_conns"`
			MaxIdleConns       int    `yaml:"max_idle_conns"`
			ConnMaxLifetimeMin int    `yaml:"conn_max_lifetime_minutes"`
			ConnMaxIdleTimeMin int    `yaml:"conn_max_idle_time_minutes"`
		} `yaml:"postgres"`
		SQLite struct {
			Path          string `yaml:"path"`
			DSN           string `yaml:"dsn"`
			MaxOpenConns  int    `yaml:"max_open_conns"`
			BusyTimeoutMS int    `yaml:"busy_timeout_ms"`
			CacheSizeKB   int    `yaml:"cache_size_kb"`
			MmapSizeBytes int64  `yaml:"mmap_size_bytes"`
			Synchronous   string `yaml:"synchronous"`
			TempStore     string `yaml:"temp_store"`
		} `yaml:"sqlite"`
		Redis struct {
			Addr     string `yaml:"addr"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
			DB       int    `yaml:"db"`
		} `yaml:"redis"`
	} `yaml:"database"`
	Cache struct {
		Driver string `yaml:"driver"`
	} `yaml:"cache"`
	Storage struct {
		Backend string `yaml:"backend"`
		Local   struct {
			RootDir string `yaml:"root_dir"`
		} `yaml:"local"`
		S3 struct {
			Endpoint        string `yaml:"endpoint"`
			Region          string `yaml:"region"`
			Bucket          string `yaml:"bucket"`
			Prefix          string `yaml:"prefix"`
			AccessKeyID     string `yaml:"access_key_id"`
			SecretAccessKey string `yaml:"secret_access_key"`
			ForcePathStyle  *bool  `yaml:"force_path_style"`
		} `yaml:"s3"`
	} `yaml:"storage"`
	GeoIP struct {
		Provider             string `yaml:"provider"`
		BaseURL              string `yaml:"base_url"`
		Token                string `yaml:"token"`
		TimeoutMS            int    `yaml:"timeout_ms"`
		DatabaseURL          string `yaml:"database_url"`
		DatabasePath         string `yaml:"database_path"`
		DatabaseMaxBytes     int64  `yaml:"database_max_bytes"`
		RefreshIntervalHours int    `yaml:"refresh_interval_hours"`
	} `yaml:"geoip"`
	Observability struct {
		Tracing struct {
			Enabled      *bool   `yaml:"enabled"`
			Endpoint     string  `yaml:"endpoint"`
			Headers      string  `yaml:"headers"`
			Insecure     *bool   `yaml:"insecure"`
			SamplingRate float64 `yaml:"sampling_rate"`
		} `yaml:"tracing"`
	} `yaml:"observability"`
}

// Config 管理后端服务的运行参数。
// 静态字段由 YAML/ENV 加载；动态字段由 settings.RuntimeSettings.ApplyTo 从数据库覆盖。
type Config struct {
	// ── 静态配置（YAML/ENV） ──
	AppName                      string
	Env                          string
	HTTPPort                     string
	CORSAllowOrigin              string
	TrustedProxies               string
	PublicAPIBaseURL             string
	PublicWebBaseURL             string
	FrontendDistDir              string
	HTTPReadHeaderTimeoutSeconds int
	HTTPReadTimeoutSeconds       int
	HTTPIdleTimeoutSeconds       int
	HTTPMaxHeaderBytes           int
	JWTSecret                    string
	DataEncryptionKey            string
	SSRFProtectionEnabled        bool
	DatabaseDriver               string
	PostgresDSN                  string
	PostgresMaxOpenConns         int
	PostgresMaxIdleConns         int
	PostgresConnMaxLifetimeMin   int
	PostgresConnMaxIdleTimeMin   int
	SQLitePath                   string
	SQLiteDSN                    string
	SQLiteMaxOpenConns           int
	SQLiteBusyTimeoutMS          int
	SQLiteCacheSizeKB            int
	SQLiteMmapSizeBytes          int64
	SQLiteSynchronous            string
	SQLiteTempStore              string
	CacheDriver                  string
	RedisAddr                    string
	RedisUsername                string
	RedisPassword                string
	RedisDB                      int
	StorageBackend               string
	StorageRootDir               string
	StorageS3Endpoint            string
	StorageS3Region              string
	StorageS3Bucket              string
	StorageS3Prefix              string
	StorageS3AccessKeyID         string
	StorageS3SecretAccessKey     string
	StorageS3ForcePathStyle      bool
	AdminUsername                string
	AdminDisplayName             string
	GeoIPProvider                string
	GeoIPBaseURL                 string
	GeoIPToken                   string
	GeoIPTimeoutMS               int
	GeoIPDatabaseURL             string
	GeoIPDatabasePath            string
	GeoIPDatabaseMaxBytes        int64
	GeoIPRefreshIntervalHours    int
	SMTPHost                     string
	SMTPPort                     int
	SMTPUsername                 string
	SMTPPassword                 string
	SMTPFrom                     string
	TurnstileSiteverifyURL       string
	OTelEnabled                  *bool
	OTelExporterOTLPEndpoint     string
	OTelExporterOTLPHeaders      string
	OTelExporterOTLPInsecure     bool
	OTelSamplingRate             float64

	// ── 动态配置（由 DB 种子初始化默认值，settings.RuntimeSettings.ApplyTo 覆盖） ──
	// 认证配置
	TokenTTLHours                int
	RefreshTokenTTLHours         int
	LoginMaxFailures             int
	LoginLockMinutes             int
	RateLimitEnabled             bool
	RateLimitRPM                 int
	PublicAuthRateLimitRPM       int
	UsernameLoginEnabled         bool
	EmailLoginEnabled            bool
	ThirdPartyLoginEnabled       bool
	EmailRegistrationEnabled     bool
	EmailVerificationEnabled     bool
	EmailRegistrationDomains     string
	EmailRegistrationNoAlias     bool
	AutoLinkVerifiedEmail        bool
	TurnstileRegistrationEnabled bool
	TurnstileSiteKey             string
	TurnstileSecretKey           string
	// 对话配置
	MaxContextMessages       int
	ContextMaxTurns          int
	ContextMaxInputTokens    int
	ContextCompactTrigger    int
	ContextCompactPreserve   int
	ConversationTaskModel    string
	ConversationTitlePrompt  string
	ConversationLabelsPrompt string
	DefaultSystemPrompt      string
	ModelOptionPolicyMode    string
	ModelOptionAllowedPaths  string
	ModelOptionDeniedPaths   string
	// 存储配置
	UserStorageQuotaBytes int64
	MaxUploadFileBytes    int64
	MaxMessageFiles       int
	// 文件处理配置
	ImageMaxDimension                 int    // 图片缩放最大边长（像素），0 = 不缩放
	FileFullContextLimitEnabled       bool   // 是否启用全文注入阈值限制
	FileFullContextMaxBytes           int64  // 文本文件全文注入阈值（字节），超出不注入
	FileFullContextMaxTokens          int    // 文本文件全文注入阈值（token）
	FileImageMaxBytes                 int64  // 图片单文件上限（字节）
	FileDocMaxBytes                   int64  // 文档单文件上限（字节）
	FileFullContextPDFMaxPages        int    // PDF Full Context 页数上限，超出走 RAG
	FileAllowedMIMETypes              string // 白名单 MIME 类型（逗号分隔）
	ExtractEngine                     string // 提取主引擎枚举
	ExtractOCREngine                  string // OCR 引擎枚举
	ExtractImageOCREnabled            bool   // 是否对图片附件执行 OCR
	ExtractPDFOCRFallbackEnabled      bool   // PDF 原生文本提取失败时是否启用 OCR 回退
	ExtractTikaSource                 string // Tika 服务来源(external/managed)
	ExtractTikaBaseURL                string // Apache Tika 服务地址
	ExtractTikaTimeoutSeconds         int    // Apache Tika 请求超时(秒)
	ExtractTikaAuthToken              string // Apache Tika 鉴权 Token
	ExtractDoclingBaseURL             string // Docling 服务地址
	ExtractDoclingTimeoutSeconds      int    // Docling 请求超时(秒)
	ExtractDoclingAuthToken           string // Docling 鉴权 Token
	ExtractTesseractOCRBaseURL        string // Tesseract OCR 服务地址
	ExtractTesseractOCRTimeoutSeconds int    // Tesseract OCR 请求超时(秒)
	ExtractTesseractOCRAuthToken      string // Tesseract OCR 鉴权 Token
	ExtractRapidOCRSource             string // RapidOCR 服务来源(external/managed)
	ExtractRapidOCRBaseURL            string // RapidOCR 服务地址
	ExtractRapidOCRTimeoutSeconds     int    // RapidOCR 请求超时(秒)
	ExtractRapidOCRAuthToken          string // RapidOCR 鉴权 Token
	ExtractPaddleOCRBaseURL           string // Paddle OCR 服务地址
	ExtractPaddleOCRTimeoutSeconds    int    // Paddle OCR 请求超时(秒)
	ExtractPaddleOCRAuthToken         string // Paddle OCR 鉴权 Token
	ExtractTencentOCRSecretID         string // 腾讯云 OCR SecretId
	ExtractTencentOCRSecretKey        string // 腾讯云 OCR SecretKey
	ExtractTencentOCRRegion           string // 腾讯云 OCR 地域
	ExtractTencentOCREndpoint         string // 腾讯云 OCR 接入点
	ExtractTencentOCRTimeoutSeconds   int    // 腾讯云 OCR 请求超时(秒)
	ExtractAliyunOCRAccessKeyID       string // 阿里云 OCR AccessKey ID
	ExtractAliyunOCRAccessKeySecret   string // 阿里云 OCR AccessKey Secret
	ExtractAliyunOCRRegion            string // 阿里云 OCR 地域
	ExtractAliyunOCREndpoint          string // 阿里云 OCR 接入点
	ExtractAliyunOCRTimeoutSeconds    int    // 阿里云 OCR 请求超时(秒)
	ExtractMinerUSource               string // MinerU 服务类型(cloud/self_hosted)
	ExtractMinerUBaseURL              string // MinerU 服务地址
	ExtractMinerUTimeoutSeconds       int    // MinerU 请求超时(秒)
	ExtractMinerUAuthToken            string // MinerU 鉴权 Token
	ExtractLLMOCRBaseURL              string // LLM OCR 服务地址
	ExtractLLMOCRModel                string // LLM OCR 请求模型
	ExtractLLMOCRTimeoutSeconds       int    // LLM OCR 请求超时(秒)
	ExtractLLMOCRAuthToken            string // LLM OCR 鉴权 Token
	ExtractLLMOCRPrompt               string // LLM OCR 提示词
	EmbeddingEnabled                  bool   // 是否启用 Embedding 服务
	EmbeddingHost                     string // Embedding HTTP 服务地址
	EmbeddingKey                      string // Embedding HTTP 服务鉴权 Key，可选
	EmbeddingTimeoutSeconds           int    // Embedding 请求超时（秒）
	EmbeddingOutputDimensions         int    // 写库/检索统一输出维度
	EmbeddingNormalize                bool   // 是否做归一化
	EmbeddingModelSignature           string // 当前生效的模型签名（派生值，由 settings 变更时自动更新）
	EmbedTriggerOnUpload              bool   // 上传后是否异步触发 embedding
	EmbedChunkSizeTokens              int    // RAG 分片大小（token 估算）
	EmbedChunkOverlapTokens           int    // 分片重叠 token 数
	EmbedBatchSize                    int    // embedding API 单批次数量
	RAGTopK                           int    // RAG 检索返回片段数
	RAGModel                          string // embedding 使用的模型名
	// chat（补充）
	RAGEnabled                      bool    // 全局开关：是否允许 RAG 功能
	RAGMinSimilarity                float64 // RAG 最低相似度阈值
	RAGTokenBudget                  int     // RAG 注入 token 预算
	RAGFetchMultiplier              int     // 检索阶段额外抓取倍数，用于过滤后留足候选
	RAGWaitReadyMS                  int     // 发送时等待 embedding 就绪的最长时间
	RAGQueryHistoryTurns            int     // 构建 RAG 查询时带入的最近用户轮次
	RAGRetrievalCacheTTL            int     // RAG 检索缓存 TTL(秒)
	RAGHybridEnabled                bool    // 是否启用混合检索（BM25+向量）
	ContextCompactHighlightsPerRole int     // 压缩摘要每个角色保留的亮点条数
	ContextCompactSnippetChars      int     // 压缩摘要单条片段最大字符数
	// 上下文压缩增强选项
	CompactLLMEnabled   bool // 是否启用 LLM 语义压缩（4级回退的 Level 2/3）
	CompactTaskModel    string
	CompactAsyncEnabled bool   // 是否将压缩移出响应关键路径（异步执行）
	CompactMaxFailures  int    // LLM 压缩熔断阈值：连续失败次数上限
	CompactSystemPrompt string // 全量摘要提示词（空串用内置默认值）；支持占位符 {{FROM_TURN}}、{{TO_TURN}}
	CompactLightPrompt  string // 轻量摘要提示词（空串用内置默认值）；支持占位符 {{FROM_TURN}}、{{TO_TURN}}
	// Token 预算感知上下文截断
	ContextTokenBudgetEnabled bool // 是否按 Token 预算截断上下文（替代消息数截断）
	// 消息历史 Embedding（语义上下文召回）
	MessageEmbeddingEnabled        bool // 是否对每轮消息异步生成向量嵌入
	SemanticContextEnabled         bool // 是否在上下文组装时加入语义召回片段
	ProcessTraceEnabled            bool // 是否启用消息处理轨迹
	ProcessTraceVisibleToUser      bool // 是否向聊天页暴露处理轨迹
	ProcessTraceStoreUpstreamThink bool // 是否持久化上游 think
	ProcessTracePersistInflight    bool // 是否在流式阶段持久化轨迹
	ContextArtifactRetentionDays   int  // 上下文证据保留天数，<=0 表示不自动过期
	// MCP 配置
	MCPEnable                     bool
	MCPToolTimeoutSeconds         int
	MCPToolRetryCount             int
	MCPMaxConcurrentCalls         int
	MCPMaxSelectedToolsPerMessage int
	MCPMaxLLMCallsPerRun          int
	MCPMaxToolCallsPerRun         int
}

// defaultYAMLPaths 固定读取仓库根目录的 config.yaml。
// 从仓库根目录或 Docker /app 启动时读取 ./config.yaml；从 backend/ 启动时读取 ../config.yaml。
var defaultYAMLPaths = []string{
	"config.yaml",
	"../config.yaml",
}

// Load 加载配置：先读仓库根目录 config.yaml，再用环境变量覆盖。
// 动态业务配置使用硬编码默认值，启动后由 settings.RuntimeSettings 从 DB 覆盖。
func Load() Config {
	yc := loadYAML()
	return Config{
		// 静态基础设施
		AppName:                      envOr("APP_NAME", yc.App.Name, "DEEIX Chat"),
		Env:                          normalizeEnv(envOrNonEmpty("APP_ENV", yc.App.Env, "prod")),
		HTTPPort:                     envOr("HTTP_PORT", yc.Server.HTTPPort, "8080"),
		CORSAllowOrigin:              envOr("CORS_ALLOW_ORIGIN", yc.Server.CORSAllowOrigin, "http://127.0.0.1:8080,http://localhost:8080"),
		TrustedProxies:               envOr("TRUSTED_PROXIES", yc.Server.TrustedProxies, ""),
		PublicAPIBaseURL:             envOr("PUBLIC_API_BASE_URL", yc.Server.PublicAPIBaseURL, ""),
		PublicWebBaseURL:             envOr("PUBLIC_WEB_BASE_URL", yc.Server.PublicWebBaseURL, ""),
		FrontendDistDir:              envOrPath("FRONTEND_DIST_DIR", yc.Server.FrontendDistDir, "../frontend/out", yc.sourceDir),
		HTTPReadHeaderTimeoutSeconds: envOrInt("HTTP_READ_HEADER_TIMEOUT_SECONDS", yc.Server.ReadHeaderTimeoutSeconds, defaultHTTPReadHeaderTimeoutSeconds),
		HTTPReadTimeoutSeconds:       envOrInt("HTTP_READ_TIMEOUT_SECONDS", yc.Server.ReadTimeoutSeconds, defaultHTTPReadTimeoutSeconds),
		HTTPIdleTimeoutSeconds:       envOrInt("HTTP_IDLE_TIMEOUT_SECONDS", yc.Server.IdleTimeoutSeconds, defaultHTTPIdleTimeoutSeconds),
		HTTPMaxHeaderBytes:           envOrInt("HTTP_MAX_HEADER_BYTES", yc.Server.MaxHeaderBytes, defaultHTTPMaxHeaderBytes),
		JWTSecret:                    envOr("JWT_SECRET", yc.Security.JWTSecret, defaultJWTSecret),
		DataEncryptionKey:            envOr("DATA_ENCRYPTION_KEY", yc.Security.DataEncryptionKey, defaultDataEncryptionKey),
		SSRFProtectionEnabled:        envOrBoolPtr("SSRF_PROTECTION_ENABLED", yc.Security.SSRFProtectionEnabled, false),
		DatabaseDriver:               normalizeDatabaseDriver(envOr("DATABASE_DRIVER", yc.Database.Driver, "postgres")),
		PostgresDSN:                  envOr("POSTGRES_DSN", yc.Database.Postgres.DSN, "host=127.0.0.1 user=deeix_chat password=deeix_chat_dev_2026 dbname=deeix_chat port=5432 sslmode=disable TimeZone=Asia/Shanghai"),
		PostgresMaxOpenConns:         envOrInt("POSTGRES_MAX_OPEN_CONNS", yc.Database.Postgres.MaxOpenConns, 30),
		PostgresMaxIdleConns:         envOrInt("POSTGRES_MAX_IDLE_CONNS", yc.Database.Postgres.MaxIdleConns, 10),
		PostgresConnMaxLifetimeMin:   envOrInt("POSTGRES_CONN_MAX_LIFETIME_MINUTES", yc.Database.Postgres.ConnMaxLifetimeMin, 60),
		PostgresConnMaxIdleTimeMin:   envOrInt("POSTGRES_CONN_MAX_IDLE_TIME_MINUTES", yc.Database.Postgres.ConnMaxIdleTimeMin, 10),
		SQLitePath:                   envOrPath("SQLITE_PATH", yc.Database.SQLite.Path, "./data/deeix.db", yc.sourceDir),
		SQLiteDSN:                    envOr("SQLITE_DSN", yc.Database.SQLite.DSN, ""),
		SQLiteMaxOpenConns:           envOrInt("SQLITE_MAX_OPEN_CONNS", yc.Database.SQLite.MaxOpenConns, 1),
		SQLiteBusyTimeoutMS:          envOrInt("SQLITE_BUSY_TIMEOUT_MS", yc.Database.SQLite.BusyTimeoutMS, 5000),
		SQLiteCacheSizeKB:            envOrInt("SQLITE_CACHE_SIZE_KB", yc.Database.SQLite.CacheSizeKB, 20480),
		SQLiteMmapSizeBytes:          envOrInt64("SQLITE_MMAP_SIZE_BYTES", yc.Database.SQLite.MmapSizeBytes, 268435456),
		SQLiteSynchronous:            normalizeSQLiteSynchronous(envOr("SQLITE_SYNCHRONOUS", yc.Database.SQLite.Synchronous, "NORMAL")),
		SQLiteTempStore:              normalizeSQLiteTempStore(envOr("SQLITE_TEMP_STORE", yc.Database.SQLite.TempStore, "MEMORY")),
		CacheDriver:                  normalizeCacheDriver(envOr("CACHE_DRIVER", yc.Cache.Driver, "redis")),
		RedisAddr:                    envOr("REDIS_ADDR", yc.Database.Redis.Addr, "127.0.0.1:6379"),
		RedisUsername:                envOr("REDIS_USERNAME", yc.Database.Redis.Username, ""),
		RedisPassword:                envOr("REDIS_PASSWORD", yc.Database.Redis.Password, ""),
		RedisDB:                      envOrInt("REDIS_DB", yc.Database.Redis.DB, 0),
		StorageBackend:               envOr("STORAGE_BACKEND", yc.Storage.Backend, "local"),
		StorageRootDir:               envOrPath("STORAGE_ROOT_DIR", yc.Storage.Local.RootDir, "./storage", yc.sourceDir),
		StorageS3Endpoint:            envOr("STORAGE_S3_ENDPOINT", yc.Storage.S3.Endpoint, ""),
		StorageS3Region:              envOr("STORAGE_S3_REGION", yc.Storage.S3.Region, "auto"),
		StorageS3Bucket:              envOr("STORAGE_S3_BUCKET", yc.Storage.S3.Bucket, ""),
		StorageS3Prefix:              envOr("STORAGE_S3_PREFIX", yc.Storage.S3.Prefix, ""),
		StorageS3AccessKeyID:         envOr("STORAGE_S3_ACCESS_KEY_ID", yc.Storage.S3.AccessKeyID, ""),
		StorageS3SecretAccessKey:     envOr("STORAGE_S3_SECRET_ACCESS_KEY", yc.Storage.S3.SecretAccessKey, ""),
		StorageS3ForcePathStyle:      envOrBoolPtr("STORAGE_S3_FORCE_PATH_STYLE", yc.Storage.S3.ForcePathStyle, true),
		AdminUsername:                defaultAdminUsername,
		AdminDisplayName:             defaultAdminDisplayName,
		GeoIPProvider:                envOr("GEOIP_PROVIDER", yc.GeoIP.Provider, "ipwhois"),
		GeoIPBaseURL:                 envOr("GEOIP_BASE_URL", yc.GeoIP.BaseURL, "https://ipwho.is"),
		GeoIPToken:                   envOr("GEOIP_TOKEN", yc.GeoIP.Token, ""),
		GeoIPTimeoutMS:               envOrInt("GEOIP_TIMEOUT_MS", yc.GeoIP.TimeoutMS, 2500),
		GeoIPDatabaseURL:             envOr("GEOIP_DATABASE_URL", yc.GeoIP.DatabaseURL, ""),
		GeoIPDatabasePath:            envOrPath("GEOIP_DATABASE_PATH", yc.GeoIP.DatabasePath, "./data/geoip/geoip.mmdb", yc.sourceDir),
		GeoIPDatabaseMaxBytes:        envOrInt64("GEOIP_DATABASE_MAX_BYTES", yc.GeoIP.DatabaseMaxBytes, defaultGeoIPMaxBytes),
		GeoIPRefreshIntervalHours:    envOrInt("GEOIP_REFRESH_INTERVAL_HOURS", yc.GeoIP.RefreshIntervalHours, 168),
		SMTPHost:                     "",
		SMTPPort:                     587,
		SMTPUsername:                 "",
		SMTPPassword:                 "",
		SMTPFrom:                     "",
		TurnstileSiteverifyURL:       envOr("TURNSTILE_SITEVERIFY_URL", yc.Security.TurnstileSiteverifyURL, DefaultTurnstileSiteverifyURL),
		OTelEnabled:                  envOrBoolOptional("OTEL_ENABLED", yc.Observability.Tracing.Enabled),
		OTelExporterOTLPEndpoint:     envOr("OTEL_EXPORTER_OTLP_ENDPOINT", yc.Observability.Tracing.Endpoint, ""),
		OTelExporterOTLPHeaders:      envOr("OTEL_EXPORTER_OTLP_HEADERS", yc.Observability.Tracing.Headers, ""),
		OTelExporterOTLPInsecure:     envOrBoolPtr("OTEL_EXPORTER_OTLP_INSECURE", yc.Observability.Tracing.Insecure, false),
		OTelSamplingRate:             envOrFloat("OTEL_TRACES_SAMPLER_ARG", envOrFloat("OTEL_SAMPLING_RATE", yc.Observability.Tracing.SamplingRate, 1), 1),

		// 动态配置默认值（会被 DB 覆盖）
		TokenTTLHours:                     24,
		RefreshTokenTTLHours:              720,
		LoginMaxFailures:                  5,
		LoginLockMinutes:                  15,
		RateLimitEnabled:                  false,
		RateLimitRPM:                      60,
		PublicAuthRateLimitRPM:            30,
		UsernameLoginEnabled:              true,
		EmailLoginEnabled:                 true,
		ThirdPartyLoginEnabled:            true,
		EmailRegistrationEnabled:          true,
		EmailVerificationEnabled:          false,
		EmailRegistrationDomains:          "",
		EmailRegistrationNoAlias:          false,
		AutoLinkVerifiedEmail:             true,
		TurnstileRegistrationEnabled:      false,
		TurnstileSiteKey:                  "",
		TurnstileSecretKey:                "",
		MaxContextMessages:                20,
		ContextMaxTurns:                   48,
		ContextMaxInputTokens:             32000,
		ContextCompactTrigger:             32768,
		ContextCompactPreserve:            8,
		ConversationTaskModel:             "follow",
		ConversationTitlePrompt:           "",
		ConversationLabelsPrompt:          "",
		DefaultSystemPrompt:               "",
		ModelOptionPolicyMode:             "allowlist",
		ModelOptionAllowedPaths:           DefaultModelOptionAllowedPathsJSON(),
		ModelOptionDeniedPaths:            DefaultModelOptionDeniedPathsJSON(),
		UserStorageQuotaBytes:             104857600,
		MaxUploadFileBytes:                20971520,
		MaxMessageFiles:                   10,
		ImageMaxDimension:                 1024,
		FileFullContextLimitEnabled:       true,
		FileFullContextMaxBytes:           51200, // 50KB
		FileFullContextMaxTokens:          12000,
		FileImageMaxBytes:                 0,
		FileDocMaxBytes:                   0,
		FileFullContextPDFMaxPages:        20,
		FileAllowedMIMETypes:              "image/jpeg,image/png,image/webp,image/gif,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel",
		ExtractEngine:                     "builtin",
		ExtractOCREngine:                  "rapidocr",
		ExtractImageOCREnabled:            false,
		ExtractPDFOCRFallbackEnabled:      false,
		ExtractTikaSource:                 "external",
		ExtractTikaBaseURL:                "http://127.0.0.1:9998",
		ExtractTikaTimeoutSeconds:         60,
		ExtractTikaAuthToken:              "",
		ExtractDoclingBaseURL:             "http://127.0.0.1:8005/ocr",
		ExtractDoclingTimeoutSeconds:      60,
		ExtractDoclingAuthToken:           "",
		ExtractTesseractOCRBaseURL:        "http://127.0.0.1:8004/ocr",
		ExtractTesseractOCRTimeoutSeconds: 60,
		ExtractTesseractOCRAuthToken:      "",
		ExtractRapidOCRSource:             "external",
		ExtractRapidOCRBaseURL:            "http://127.0.0.1:8002/ocr",
		ExtractRapidOCRTimeoutSeconds:     60,
		ExtractRapidOCRAuthToken:          "",
		ExtractPaddleOCRBaseURL:           "",
		ExtractPaddleOCRTimeoutSeconds:    60,
		ExtractPaddleOCRAuthToken:         "",
		ExtractTencentOCRSecretID:         "",
		ExtractTencentOCRSecretKey:        "",
		ExtractTencentOCRRegion:           "ap-guangzhou",
		ExtractTencentOCREndpoint:         "ocr.tencentcloudapi.com",
		ExtractTencentOCRTimeoutSeconds:   60,
		ExtractAliyunOCRAccessKeyID:       "",
		ExtractAliyunOCRAccessKeySecret:   "",
		ExtractAliyunOCRRegion:            "cn-hangzhou",
		ExtractAliyunOCREndpoint:          "ocr-api.cn-hangzhou.aliyuncs.com",
		ExtractAliyunOCRTimeoutSeconds:    60,
		ExtractMinerUSource:               "cloud",
		ExtractMinerUBaseURL:              "https://mineru.net/api/v4",
		ExtractMinerUTimeoutSeconds:       180,
		ExtractMinerUAuthToken:            "",
		ExtractLLMOCRBaseURL:              "",
		ExtractLLMOCRModel:                "",
		ExtractLLMOCRTimeoutSeconds:       60,
		ExtractLLMOCRAuthToken:            "",
		ExtractLLMOCRPrompt:               "",
		EmbeddingEnabled:                  false,
		EmbeddingHost:                     "",
		EmbeddingKey:                      "",
		EmbeddingTimeoutSeconds:           60,
		EmbeddingOutputDimensions:         1536,
		EmbeddingNormalize:                true,
		EmbedTriggerOnUpload:              true,
		EmbedChunkSizeTokens:              1024,
		EmbedChunkOverlapTokens:           64,
		EmbedBatchSize:                    20,
		RAGTopK:                           5,
		RAGModel:                          "sentence-transformers/all-MiniLM-L6-v2",
		RAGEnabled:                        false,
		RAGMinSimilarity:                  0.45,
		RAGTokenBudget:                    2000,
		RAGFetchMultiplier:                3,
		RAGWaitReadyMS:                    3000,
		RAGQueryHistoryTurns:              0,
		RAGRetrievalCacheTTL:              120,
		ContextCompactHighlightsPerRole:   6,
		ContextCompactSnippetChars:        140,
		CompactLLMEnabled:                 true,
		CompactTaskModel:                  "follow",
		CompactAsyncEnabled:               true,
		CompactMaxFailures:                3,
		ContextTokenBudgetEnabled:         true,
		MessageEmbeddingEnabled:           false, // 默认关闭，需要 embedding 服务就绪后开启
		SemanticContextEnabled:            false,
		ProcessTraceEnabled:               true,
		ProcessTraceVisibleToUser:         true,
		ProcessTraceStoreUpstreamThink:    true,
		ProcessTracePersistInflight:       true,
		ContextArtifactRetentionDays:      90,
		MCPEnable:                         false,
		MCPToolTimeoutSeconds:             10,
		MCPToolRetryCount:                 0,
		MCPMaxConcurrentCalls:             8,
		MCPMaxSelectedToolsPerMessage:     DefaultMCPMaxSelectedToolsPerMessage,
		MCPMaxLLMCallsPerRun:              5,
		MCPMaxToolCallsPerRun:             8,
	}
}

// Validate 检查关键配置是否合法。
func (c Config) Validate() error {
	if err := c.validateDatabase(); err != nil {
		return err
	}
	if err := c.validateCache(); err != nil {
		return err
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	env := normalizeEnv(c.Env)
	if env != "dev" && env != "prod" {
		if env == "" {
			return errors.New("invalid config: APP_ENV/app.env must be dev, development, prod, or production (got empty)")
		}
		return fmt.Errorf("invalid config: APP_ENV/app.env must be dev, development, prod, or production (got %q)", c.Env)
	}
	if env != "prod" {
		return nil
	}

	if strings.TrimSpace(c.JWTSecret) == "" || strings.TrimSpace(c.JWTSecret) == defaultJWTSecret {
		return errors.New("invalid production config: JWT_SECRET must be explicitly set")
	}
	if len(strings.TrimSpace(c.JWTSecret)) < 16 {
		return errors.New("invalid production config: JWT_SECRET is too short")
	}
	if strings.TrimSpace(c.DataEncryptionKey) == "" || strings.TrimSpace(c.DataEncryptionKey) == defaultDataEncryptionKey {
		return errors.New("invalid production config: DATA_ENCRYPTION_KEY must be explicitly set")
	}
	if len(strings.TrimSpace(c.DataEncryptionKey)) < 32 {
		return errors.New("invalid production config: DATA_ENCRYPTION_KEY is too short")
	}

	if strings.TrimSpace(c.CORSAllowOrigin) == "" || strings.TrimSpace(c.CORSAllowOrigin) == "*" {
		return errors.New("invalid production config: CORS_ALLOW_ORIGIN must be explicitly set (wildcard * is not allowed)")
	}
	if err := validatePublicURL(c.PublicAPIBaseURL, "PUBLIC_API_BASE_URL"); err != nil {
		return err
	}
	if err := validatePublicURL(c.PublicWebBaseURL, "PUBLIC_WEB_BASE_URL"); err != nil {
		return err
	}

	return nil
}

func (c Config) validateDatabase() error {
	switch normalizeDatabaseDriver(c.DatabaseDriver) {
	case "postgres":
		return nil
	case "sqlite":
		if strings.TrimSpace(c.SQLiteDSN) == "" && strings.TrimSpace(c.SQLitePath) == "" {
			return errors.New("invalid database config: SQLITE_PATH or SQLITE_DSN must be set when DATABASE_DRIVER=sqlite")
		}
		if normalizeSQLiteSynchronous(c.SQLiteSynchronous) == "" {
			return fmt.Errorf("invalid database config: unsupported SQLITE_SYNCHRONOUS %q", c.SQLiteSynchronous)
		}
		if normalizeSQLiteTempStore(c.SQLiteTempStore) == "" {
			return fmt.Errorf("invalid database config: unsupported SQLITE_TEMP_STORE %q", c.SQLiteTempStore)
		}
		return nil
	default:
		return fmt.Errorf("invalid database config: unsupported DATABASE_DRIVER %q", c.DatabaseDriver)
	}
}

func (c Config) validateCache() error {
	switch normalizeCacheDriver(c.CacheDriver) {
	case "redis":
		return nil
	case "memory":
		return nil
	default:
		return fmt.Errorf("invalid cache config: unsupported CACHE_DRIVER %q", c.CacheDriver)
	}
}

func (c Config) validateStorage() error {
	switch strings.ToLower(strings.TrimSpace(c.StorageBackend)) {
	case "", "local":
		return nil
	case "s3":
		if strings.TrimSpace(c.StorageS3Bucket) == "" {
			return errors.New("invalid storage config: STORAGE_S3_BUCKET must be set when STORAGE_BACKEND=s3")
		}
		if strings.TrimSpace(c.StorageS3Region) == "" {
			return errors.New("invalid storage config: STORAGE_S3_REGION must be set when STORAGE_BACKEND=s3")
		}
		return nil
	default:
		return fmt.Errorf("invalid storage config: unsupported STORAGE_BACKEND %q", c.StorageBackend)
	}
}

func validatePublicURL(raw string, label string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Host == "" || parsed.Scheme != "https" {
		return fmt.Errorf("invalid production config: %s must be an https URL", label)
	}
	return nil
}

// loadYAML 尝试读取仓库根目录 config.yaml，找不到则返回空结构。
func loadYAML() yamlConfig {
	// 支持通过环境变量指定配置文件路径
	if custom := os.Getenv("CONFIG_FILE"); custom != "" {
		if yc, err := readYAML(custom); err == nil {
			return yc
		}
		fmt.Fprintf(os.Stderr, "warning: CONFIG_FILE=%s not readable, falling back to defaults\n", custom)
	}

	for _, path := range defaultYAMLPaths {
		if yc, err := readYAML(path); err == nil {
			return yc
		}
	}
	return yamlConfig{}
}

func readYAML(path string) (yamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return yamlConfig{}, err
	}
	var yc yamlConfig
	if err = yaml.Unmarshal(data, &yc); err != nil {
		return yamlConfig{}, err
	}
	if absolutePath, absErr := filepath.Abs(path); absErr == nil {
		yc.sourceDir = filepath.Dir(absolutePath)
	}
	return yc, nil
}

// envOr 优先级：环境变量 > YAML 值 > 硬编码默认值。
func envOr(envKey string, yamlVal string, defaultVal string) string {
	if v, ok := os.LookupEnv(envKey); ok {
		return v
	}
	if yamlVal != "" {
		return yamlVal
	}
	return defaultVal
}

func envOrNonEmpty(envKey string, yamlVal string, defaultVal string) string {
	if v, ok := os.LookupEnv(envKey); ok && strings.TrimSpace(v) != "" {
		return v
	}
	if strings.TrimSpace(yamlVal) != "" {
		return yamlVal
	}
	return defaultVal
}

func normalizeEnv(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development":
		return "dev"
	case "production":
		return "prod"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeDatabaseDriver(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "postgres", "postgresql", "pg":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeCacheDriver(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "redis":
		return "redis"
	case "memory", "mem", "inmemory", "in-memory":
		return "memory"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeSQLiteSynchronous(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "NORMAL":
		return "NORMAL"
	case "OFF", "FULL", "EXTRA":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeSQLiteTempStore(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "MEMORY":
		return "MEMORY"
	case "DEFAULT", "FILE":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func envOrPath(envKey string, yamlVal string, defaultVal string, sourceDir string) string {
	if v, ok := os.LookupEnv(envKey); ok {
		return v
	}
	if yamlVal != "" {
		return resolveConfigPath(yamlVal, sourceDir)
	}
	return defaultVal
}

func resolveConfigPath(value string, sourceDir string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || filepath.IsAbs(trimmed) || strings.TrimSpace(sourceDir) == "" {
		return trimmed
	}
	return filepath.Clean(filepath.Join(sourceDir, trimmed))
}

func envOrInt(envKey string, yamlVal int, defaultVal int) int {
	if v, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	if yamlVal != 0 {
		return yamlVal
	}
	return defaultVal
}

func envOrInt64(envKey string, yamlVal int64, defaultVal int64) int64 {
	if v, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
	}
	if yamlVal != 0 {
		return yamlVal
	}
	return defaultVal
}

func envOrFloat(envKey string, yamlVal float64, defaultVal float64) float64 {
	if v, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	if yamlVal != 0 {
		return yamlVal
	}
	return defaultVal
}

func envOrBoolOptional(envKey string, yamlVal *bool) *bool {
	if v, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return &parsed
		}
	}
	return yamlVal
}

func envOrBoolPtr(envKey string, yamlVal *bool, defaultVal bool) bool {
	if v, ok := os.LookupEnv(envKey); ok {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	if yamlVal != nil {
		return *yamlVal
	}
	return defaultVal
}

// TrustedProxyList 返回受信代理列表，支持逗号分隔。
func (c Config) TrustedProxyList() []string {
	raw := strings.TrimSpace(c.TrustedProxies)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

// Runtime 提供线程安全的运行时配置快照读取。
type Runtime struct {
	value atomic.Value
}

// NewRuntime 创建运行时配置容器。
func NewRuntime(cfg Config) *Runtime {
	runtime := &Runtime{}
	runtime.value.Store(cfg)
	return runtime
}

// Snapshot 返回当前配置快照。
func (r *Runtime) Snapshot() Config {
	if r == nil {
		return Config{}
	}
	value := r.value.Load()
	if value == nil {
		return Config{}
	}
	item, ok := value.(Config)
	if !ok {
		return Config{}
	}
	return item
}

// Store 覆盖当前配置快照。
func (r *Runtime) Store(cfg Config) {
	if r == nil {
		return
	}
	r.value.Store(cfg)
}
