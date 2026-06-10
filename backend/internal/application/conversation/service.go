package conversation

import (
	"context"
	"sync"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

const (
	// semanticRecallDeadline：语义召回截止时限，超时后优雅跳过，不阻塞 LLM 关键路径。
	semanticRecallDeadline = 200 * time.Millisecond
)

type routeResolver interface {
	ResolveRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error)
	MarkRouteFailure(ctx context.Context, route *channel.ResolvedRoute, cause error)
	MarkRouteSuccess(ctx context.Context, route *channel.ResolvedRoute)
}

// defaultRouteResolver 表示按任务类型解析默认路由的可选能力。
// conversation 只依赖这个窄接口，不直接感知 channel.Service 的具体实现。
type defaultRouteResolver interface {
	ResolveDefaultRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error)
}

type memoryRecorder interface {
	UpsertUserMemory(ctx context.Context, userID uint, memoryKey string, value string, scope string, updatedBy string) error
	ListUserMemories(ctx context.Context, userID uint) ([]domainmemory.UserMemory, error)
	SearchUserMemoriesByEmbedding(ctx context.Context, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainmemory.UserMemory, error)
	UpsertUserMemoryEmbedding(ctx context.Context, userID uint, memoryKey string, expectedValue string, embedding []float32) error
}

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

type basicServiceBillingContextKey struct{}

type basicServiceBillingContext struct {
	UserID         uint
	ConversationID uint
}

// Service 封装会话业务能力。
type Service struct {
	cfg               *config.Runtime
	repo              repository.ConversationRepository
	cache             repository.ConversationCacheRepository
	routeResolver     routeResolver
	memoryRecorder    memoryRecorder
	mcpRepo           repository.MCPRepository
	llmClient         *llm.Client
	mcpClient         *mcp.Client
	uploadSvc         *appupload.Service
	compactSvc        *appcompact.Service
	embeddingSvc      *appembedding.Service
	processingSvc     *appprocessing.Service
	extractSvc        *extraction.Service
	ragSvc            *apprag.Service
	billingSvc        *appbilling.Service
	auditWriter       auditWriter
	storeProvider     appstorage.Provider
	logger            *zap.Logger
	toolLimiters      sync.Map
	generationStreams *generationStreamRegistry
	snapshotCache     sync.Map // conversationID (uint) → *cachedSnapshot
	userMemCache      sync.Map // userID (uint) → *cachedUserMemories
	userSettingCache  sync.Map // "userID:key" (string) → *cachedUserSetting
}

func (s *Service) llmAttribution() (string, string) {
	if s == nil || s.cfg == nil {
		return "", ""
	}
	cfg := s.cfg.Snapshot()
	return cfg.PublicWebBaseURL, cfg.AppName
}

// AttachmentInput 是消息附件入参（应用层内部传递，无序列化标签）。
type AttachmentInput struct {
	FileObjID              uint
	FileID                 string
	Kind                   string
	FileName               string
	MimeType               string
	DetectedMIME           string
	FileCategory           string
	FileSize               int64
	SHA256                 string
	StoragePath            string
	MetaJSON               string
	PageCount              int
	ProcessingStatus       string
	ProcessingReady        bool
	ProcessingErrorCode    string
	ProcessingErrorMessage string
	ExtractStatus          string
	EmbedStatus            string
	ExtractedText          string
	RagOptOut              bool // 用户是否关闭该文件的 RAG；RAG 段直接复用，无需重查 DB
	ChunkCount             int  // 向量分块数；RAG 缓存 key 需要
	Current                bool // 是否为本轮用户显式上传的附件
	ContextMode            string
}

// SendMessageInput 定义消息发送请求。
type SendMessageInput struct {
	UserID                  uint
	ConversationID          uint
	RequestID               string
	ContentType             string
	Content                 string
	PlatformModelName       string
	Options                 map[string]interface{}
	ClientRunID             string
	FileIDs                 []string
	SelectedToolIDs         []uint
	HTMLVisualPromptEnabled bool
	HTMLVisualColorMode     string
	ParentMessagePublicID   string
	SourceMessagePublicID   string
	BranchReason            string
	Cancelable              bool
	// OnEvent 用于向调用方推送中间事件（如 rag_search），流式场景使用。
	OnEvent func(eventType string, payload map[string]interface{}) error
}

// SendMessageResult 返回用户消息与 AI 消息。
type SendMessageResult struct {
	UserMessage         model.Message
	AssistantMessage    model.Message
	Billable            bool
	UpstreamID          uint
	UpstreamName        string
	PlatformModelName   string
	RoutedBindingCode   string
	UpstreamModelName   string
	UpstreamProtocol    string
	EffectiveOptions    map[string]interface{}
	UsageSpeed          string
	UsageServiceTier    string
	RawUsageJSON        string
	CacheWrite5mTokens  int64
	CacheWrite1hTokens  int64
	ServerSideToolUsage map[string]int64
	LatencyMS           int64
}

// MessageFeedbackResult 返回反馈后的当前状态（内部传输，不携带序列化标记）。
type MessageFeedbackResult struct {
	MessageID       uint
	MessagePublicID string
	MyFeedback      string
	ThumbsUpCount   int64
	ThumbsDownCount int64
}

// NewService 创建服务。
func NewService(
	cfg config.Config,
	repo repository.ConversationRepository,
	cache repository.ConversationCacheRepository,
	routeResolver routeResolver,
	memoryRecorder memoryRecorder,
	llmClient *llm.Client,
	mcpClient *mcp.Client,
	embedClient *embedding.Client,
	uploadSvc *appupload.Service,
	compactSvc *appcompact.Service,
	embeddingSvc *appembedding.Service,
	processingSvc *appprocessing.Service,
	extractSvc *extraction.Service,
	ragSvc *apprag.Service,
	logger *zap.Logger,
) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, cache, routeResolver, memoryRecorder, llmClient, mcpClient, embedClient, uploadSvc, compactSvc, embeddingSvc, processingSvc, extractSvc, ragSvc, logger)
}

// NewServiceWithRuntime 创建使用运行时配置容器的服务。
func NewServiceWithRuntime(
	cfg *config.Runtime,
	repo repository.ConversationRepository,
	cache repository.ConversationCacheRepository,
	routeResolver routeResolver,
	memoryRecorder memoryRecorder,
	llmClient *llm.Client,
	mcpClient *mcp.Client,
	embedClient *embedding.Client,
	uploadSvc *appupload.Service,
	compactSvc *appcompact.Service,
	embeddingSvc *appembedding.Service,
	processingSvc *appprocessing.Service,
	extractSvc *extraction.Service,
	ragSvc *apprag.Service,
	logger *zap.Logger,
) *Service {
	svc := &Service{
		cfg:               cfg,
		repo:              repo,
		cache:             cache,
		routeResolver:     routeResolver,
		memoryRecorder:    memoryRecorder,
		llmClient:         llmClient,
		mcpClient:         mcpClient,
		compactSvc:        compactSvc,
		embeddingSvc:      embeddingSvc,
		processingSvc:     processingSvc,
		extractSvc:        extractSvc,
		ragSvc:            ragSvc,
		storeProvider:     appstorage.NewRuntimeProvider(cfg, nil),
		logger:            logger,
		generationStreams: newGenerationStreamRegistry(cache, defaultGenerationStreamOptions()),
	}
	if extractSvc == nil {
		extractSvc = extraction.NewServiceWithRuntime(cfg)
	}
	extractSvc.SetObjectStoreProvider(svc.storeProvider)
	if embeddingSvc == nil {
		embeddingSvc = appembedding.NewServiceWithRuntime(cfg, repo, extractSvc, embedClient, logger)
	}
	if processingSvc == nil {
		processingSvc = appprocessing.NewServiceWithRuntime(cfg, repo, cache, extractSvc, embeddingSvc, logger, appprocessing.DefaultExtractorVersion)
	}
	if uploadSvc == nil {
		uploadSvc = appupload.NewServiceWithRuntime(cfg, repo, logger, appupload.Hooks{
			ResolveCapability: func(ctx context.Context) appupload.FileCapability {
				capability := svc.resolveChatFileCapability(ctx)
				return appupload.FileCapability{
					RAGAvailable:         capability.RAGAvailable,
					EffectiveDocMaxBytes: capability.EffectiveDocMaxBytes,
				}
			},
			InitializeUploadedFile: processingSvc.InitializeUploadedFile,
		}, appupload.ErrorSet{
			InvalidFileReference: ErrInvalidFileReference,
			InvalidFileName:      ErrInvalidFileName,
			FileNotFound:         ErrFileNotFound,
			StorageQuotaExceeded: ErrStorageQuotaExceeded,
			FileTooLarge:         ErrFileTooLarge,
			MIMEBlocked:          ErrMIMEBlocked,
			EmbeddingUnavailable: ErrEmbeddingUnavailable,
			DangerousMIMEType:    ErrDangerousMIMEType,
		}, appprocessing.DefaultExtractorVersion)
	}
	uploadSvc.SetObjectStoreProvider(svc.storeProvider)
	if compactSvc == nil {
		compactSvc = appcompact.NewServiceWithRuntime(cfg, repo, logger)
	}
	if ragSvc == nil {
		ragSvc = apprag.NewServiceWithRuntime(cfg, repo, cache, embedClient)
	}
	svc.uploadSvc = uploadSvc
	svc.compactSvc = compactSvc
	svc.embeddingSvc = embeddingSvc
	svc.processingSvc = processingSvc
	svc.extractSvc = extractSvc
	svc.ragSvc = ragSvc
	// 注入 LLM 语义压缩回调（在 svc 完全初始化后绑定）
	svc.compactSvc.SetLLMSummarizer(svc.callCompactLLM)
	return svc
}

// InvalidateMemoryCache 清除指定用户的记忆缓存，使下一次请求重新从 DB 加载。
// 由外部（memory handler 写入后）通过回调触发，避免循环依赖。
func (s *Service) InvalidateMemoryCache(userID uint) {
	s.userMemCache.Delete(userID)
}

// SetBillingService 注入计费服务，用于记录标题、标签、上下文压缩等基础 LLM 服务用量。
func (s *Service) SetBillingService(billingSvc *appbilling.Service) {
	s.billingSvc = billingSvc
}

// SetAuditWriter 注入会话域审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	if provider != nil {
		s.storeProvider = provider
		if s.uploadSvc != nil {
			s.uploadSvc.SetObjectStoreProvider(provider)
		}
		if s.extractSvc != nil {
			s.extractSvc.SetObjectStoreProvider(provider)
		}
	}
}

func (s *Service) SetMCPRepository(repo repository.MCPRepository) {
	s.mcpRepo = repo
}
