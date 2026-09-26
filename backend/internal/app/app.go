package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/announcement"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	appcontentmoderation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	appknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/knowledgebase"
	applogcleanup "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/logcleanup"
	appmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/memory"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	apppromptpreset "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/promptpreset"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	appruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/runtime"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	appskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/skill"
	appsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	appuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/uicomponent"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	moderationclient "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	extractengines "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/engines"
	extractprobe "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/probe"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/geoip"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/identityprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mediaartifact"
	openrouterpricing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelpricing/openrouter"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	platformlogger "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/logger"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/openwebui"
	epaypayment "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/payment/epay"
	stripepayment "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/payment/stripe"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence"
	filecache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/filecache"
	announcementrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/announcement"
	auditrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/audit"
	billingrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/billing"
	channelrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/channel"
	contentmoderationrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/contentmoderation"
	conversationrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	knowledgebaserepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/knowledgebase"
	logcleanuprepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/logcleanup"
	mcprepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/mcp"
	memoryrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/memory"
	promptpresetrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/promptpreset"
	settingsrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/settings"
	skillrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/skill"
	systemeventrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/systemevent"
	uicomponentrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/uicomponent"
	userrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/user"
	usersettingsrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/usersettings"
	platformruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/runtime"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/lifecycle"
	platformhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http"
	adminhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/admin"
	announcementhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/announcement"
	authhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/auth"
	billinghttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/billing"
	channelhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/channel"
	contentmoderationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/contentmoderation"
	conversationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/conversation"
	knowledgebasehttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/knowledgebase"
	mcphttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/mcp"
	memoryhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/memory"
	promptpresethttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/promptpreset"
	settingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/settings"
	skillhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/skill"
	uicomponenthttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/uicomponent"
	userhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/user"
	usersettingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/usersettings"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 维护应用运行依赖。
type App struct {
	cfg                    config.Config
	engine                 *gin.Engine
	logger                 *zap.Logger
	db                     *gorm.DB
	cache                  cache.Backend
	geoResolver            *geoip.Client
	identityProviderClient *identityprovider.Client
	llmClient              *llm.Client
	mcpClient              *mcp.Client
	embeddingClient        *embedding.Client
	mediaArtifactClient    *mediaartifact.Client
	moderationClient       *moderationclient.Client
	conversationService    *conversation.Service
	contentModeration      *appcontentmoderation.Service
	authService            *auth.Service
	runtimeCfg             *config.Runtime
	tracingShutdown        platformtracing.ShutdownFunc
	backgroundCancel       context.CancelFunc
	// shutdown 是进程关停排空信号：翻转就绪探针并断开订阅型长连接。
	shutdown *lifecycle.Shutdown
	// stopCh 由 RequestShutdown 关闭，与 SIGTERM 等价。
	stopCh   chan struct{}
	stopOnce sync.Once
}

type subscriptionGroupAdapter struct {
	billing *billing.Service
}

func (a *subscriptionGroupAdapter) GetUserSubscriptionGroupID(ctx context.Context, userID uint) (*uint, error) {
	snap, err := a.billing.GetCurrentSubscriptionSnapshot(ctx, userID, time.Now())
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, nil
	}
	return snap.PermissionGroupID, nil
}

type avatarContentOpener struct {
	uploads *appupload.Service
}

func (o avatarContentOpener) OpenAvatarFileContent(ctx context.Context, userID uint, fileID string) (*user.AvatarFileContent, error) {
	content, err := o.uploads.OpenFileContent(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	return &user.AvatarFileContent{
		Reader:      content.Reader,
		ContentType: content.ContentType,
		SizeBytes:   content.SizeBytes,
		ModTime:     content.ModTime,
		FileName:    content.File.FileName,
	}, nil
}

// Options 控制应用的运行形态。零值等价于普通服务器部署。
type Options struct {
	// LocalDataDir 非空时以本地 sidecar 模式运行，所有数据落在该目录（见 config.ApplyLocalMode）。
	LocalDataDir string
}

// NewApp 创建普通服务器部署形态的应用。
func NewApp() (*App, error) {
	return NewAppWithOptions(Options{})
}

// NewAppWithOptions 按 Options 创建应用。
func NewAppWithOptions(opts Options) (*App, error) {
	cfg := config.Load()
	if opts.LocalDataDir != "" {
		if err := cfg.ApplyLocalMode(opts.LocalDataDir); err != nil {
			return nil, err
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	runtimeCfg := config.NewRuntime(cfg)

	tracingShutdown, err := platformtracing.Init(context.Background(), platformtracing.Config{
		ServiceName:  cfg.AppName,
		Enabled:      cfg.OTelEnabled,
		Endpoint:     cfg.OTelExporterOTLPEndpoint,
		Headers:      cfg.OTelExporterOTLPHeaders,
		Insecure:     cfg.OTelExporterOTLPInsecure,
		Protocol:     cfg.OTelExporterOTLPProtocol,
		SamplingRate: cfg.OTelSamplingRate,
	})
	if err != nil {
		return nil, fmt.Errorf("init tracing: %w", err)
	}
	keepTracing := false
	defer func() {
		if keepTracing {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tracingShutdown(shutdownCtx)
	}()

	// 本地模式下 stdout 是与父进程的握手通道，日志改走 stderr。
	newLogger := platformlogger.New
	if cfg.LocalMode {
		newLogger = platformlogger.NewStderr
	}
	log, err := newLogger(cfg.Env)
	if err != nil {
		return nil, err
	}

	db, err := persistence.Open(cfg)
	if err != nil {
		return nil, err
	}

	cacheBackend, err := cache.Open(cfg)
	if err != nil {
		return nil, err
	}

	auditRepo := auditrepo.NewRepo(db)
	auditService := audit.NewService(auditRepo, log)
	logCleanupRepo := logcleanuprepo.NewRepo(db)
	logCleanupService := applogcleanup.NewService(logCleanupRepo, auditService)
	systemEventRepo := systemeventrepo.NewRepo(db)
	systemEventService := appsystemevent.NewService(systemEventRepo)

	// 初始化 settings 模块：种子数据 + 动态配置覆盖
	settingsRepo := settingsrepo.NewRepo(db)
	settingsService := settings.NewService(settingsRepo, cfg.DataEncryptionKey)
	settingsService.SetAuditWriter(auditService)
	runtimeService := appruntime.NewService(runtimeCfg, extractprobe.Prober{})
	runtimeService.SetDockerRunner(platformruntime.NewDockerRunner())
	settingsCache := cacheBackend.Settings()
	runtimeSettings := settings.NewRuntimeSettings(settingsRepo, settingsCache, cfg.DataEncryptionKey)
	settingsHandler := settingshttp.NewHandler(settingsService, runtimeSettings, runtimeService, runtimeCfg)
	settingsModule := settingshttp.NewModule(settingsHandler)
	if err = settingsService.Seed(context.Background()); err != nil {
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	if err = runtimeSettings.ApplyTo(context.Background(), runtimeCfg); err != nil {
		return nil, fmt.Errorf("apply settings: %w", err)
	}

	// 启动时补全旧版模型签名以兼容已有向量。后续真正修改模型、
	// 维度或服务地址时，设置处理器会切换到包含服务地址的新空间签名。
	if startCfg := runtimeCfg.Snapshot(); startCfg.EmbeddingModelSignature == "" && startCfg.RAGModel != "" {
		initialSig := appembedding.ComputeModelSignature(startCfg.RAGModel, startCfg.EmbeddingOutputDimensions)
		if _, seedErr := settingsService.BatchUpdate(context.Background(), []settings.PatchItem{
			{Namespace: "file", Key: "embedding_model_signature", Value: initialSig},
		}); seedErr == nil {
			_ = runtimeSettings.ApplyTo(context.Background(), runtimeCfg)
		}
	}

	userRepo := userrepo.NewRepo(db)
	userService := user.NewService(userRepo)
	billingRepo := billingrepo.NewRepo(db)
	billingService := billing.NewService(billingRepo)
	billingService.SetAuditWriter(auditService)
	billingService.SetRedemptionCodeSecret(cfg.DataEncryptionKey)
	officialPricingService := billing.NewOfficialPricingService(
		openrouterpricing.New(cfg.StrictOutboundPolicy()),
		filecache.NewOpenRouterPricingCache(runtimeCfg.Snapshot().StorageRootDir),
	)
	paymentCheckoutService := billing.NewPaymentCheckoutService(stripepayment.New(cfg.StrictOutboundPolicy()), epaypayment.New())
	billingHandler := billinghttp.NewHandler(billingService, settingsService, runtimeCfg, officialPricingService, paymentCheckoutService, log)
	billingModule := billinghttp.NewModule(billingHandler)
	// 对象存储工厂由组合根显式注入，避免服务实例依赖进程级可变状态。
	objectStoreProvider := appstorage.NewRuntimeProvider(runtimeCfg, objectstore.New)
	// 抽取引擎工厂由组合根显式注入；具体客户端构造为 nil 时必须返回 nil 接口，避免 typed-nil 绕过判空。
	extractionFactories := extraction.EngineFactories{
		NewTika: func(cfg config.Config) extraction.DocumentExtractor {
			if client := extractengines.NewTika(cfg); client != nil {
				return client
			}
			return nil
		},
		NewDocling: func(cfg config.Config) extraction.DocumentExtractor {
			if client := extractengines.NewDocling(cfg); client != nil {
				return client
			}
			return nil
		},
		NewMinerU: func(cfg config.Config) extraction.DocumentExtractor {
			if client := extractengines.NewMinerU(cfg); client != nil {
				return client
			}
			return nil
		},
		NewOCR: func(provider string, cfg config.Config) extraction.OCRExtractor {
			if client := extractengines.NewOCR(provider, cfg); client != nil {
				return client
			}
			return nil
		},
		Builtin: extractengines.Builtin{},
	}
	geoResolver := geoip.New(runtimeCfg.Snapshot())
	// GeoIP 关闭时 geoip.New 返回 nil 指针，必须转成 nil 接口再注入，避免 typed-nil 绕过判空。
	var authGeoResolver auth.GeoResolver
	if geoResolver != nil {
		authGeoResolver = geoResolver
	}
	identityProviderClient := identityprovider.New(cfg.StrictOutboundPolicy())
	authService := auth.NewServiceWithRuntime(
		runtimeCfg,
		userRepo,
		authGeoResolver,
		identityProviderClient,
	)
	authService.SetLogger(log)
	authService.SetProviderAuthBridge(cacheBackend.ProviderAuthBridge())
	authService.SetObjectStoreProvider(objectStoreProvider)
	authService.SetAuditWriter(auditService)
	settingsService.SetAuthSafetyService(authService)
	authService.SetSubscriptionResolver(billingService)
	var bootstrapSuperAdmin *auth.BootstrapSuperAdmin
	if cfg.LocalMode {
		// 本地模式：唯一用户无密码、无初始化引导，通过启动握手的一次性 grant 登录。
		if _, err = authService.EnsureLocalOwner(context.Background()); err != nil {
			return nil, err
		}
	} else if bootstrapSuperAdmin, err = authService.EnsureBootstrapSuperAdmin(context.Background()); err != nil {
		return nil, err
	}
	authHandler := authhttp.NewHandler(authService)
	authModule := authhttp.NewModule(authHandler)
	memoryRepo := memoryrepo.NewRepo(db)
	memoryService := memory.NewService(memoryRepo)
	memoryService.SetAuditWriter(auditService)
	memoryHandler := memoryhttp.NewHandler(memoryService)
	memoryModule := memoryhttp.NewModule(memoryHandler)
	channelRepo := channelrepo.NewRepo(db)
	channelCache := cacheBackend.Channel()
	trustedOutboundPolicy := cfg.TrustedOutboundPolicy()
	strictOutboundPolicy := cfg.StrictOutboundPolicy()
	llmClient := llm.NewClient(trustedOutboundPolicy)
	mcpClient := mcp.NewClient(trustedOutboundPolicy)
	mediaArtifactClient := mediaartifact.New(strictOutboundPolicy)
	channelService := channel.NewServiceWithRuntime(runtimeCfg, channelRepo, channelRepo, channelCache, llmClient)
	channelService.SetLogger(log)
	channelService.SetObjectStoreProvider(objectStoreProvider)
	channelService.SetModelIconAssetRepository(channelRepo)
	channelService.SetBillingModelPricingFilter(billingService)
	channelService.SetPermissionGroupRepo(channelRepo)
	channelService.SetSubscriptionGroupResolver(&subscriptionGroupAdapter{billing: billingService})
	billingService.SetGroupRateMultiplierResolver(channelRepo)
	billingService.SetPermissionGroupLookup(channelRepo)
	billingService.SetModelPricingInvalidator(channelService.InvalidateModelCatalog)
	billingService.SetPlatformModelIdentityResolver(channelService)
	billingService.SetModelPricingCatalogProvider(channelService)
	billingService.SetNativeToolCatalogProvider(channelService)
	settingsHandler.SetNativeToolCatalogProvider(channelService)
	channelHandler := channelhttp.NewHandler(channelService)
	channelModule := channelhttp.NewModule(channelHandler)
	conversationRepo := conversationrepo.NewRepo(db)
	settingsService.SetVectorStoreAvailabilityService(conversationRepo)
	conversationCache := cacheBackend.Conversation()
	mcpRepo := mcprepo.NewRepo(db)
	embedClient := embedding.New(trustedOutboundPolicy)
	compactService := compact.NewServiceWithRuntime(runtimeCfg, conversationRepo, log)
	extractionService := extraction.NewServiceWithRuntime(runtimeCfg, extractionFactories)
	extractionService.SetObjectStoreProvider(objectStoreProvider)
	embeddingService := appembedding.NewServiceWithRuntime(runtimeCfg, conversationRepo, extractionService, embedClient, log)
	memoryService.SetEmbeddingProvider(embeddingService)
	settingsHandler.SetEmbeddingService(embeddingService)
	processingService := appprocessing.NewServiceWithRuntime(appprocessing.Dependencies{
		Config:           runtimeCfg,
		Repository:       conversationRepo,
		Cache:            conversationCache,
		ExtractService:   extractionService,
		EmbeddingService: embeddingService,
		Logger:           log,
		ExtractorVersion: appprocessing.DefaultExtractorVersion,
	})
	uploadService := appupload.NewServiceWithRuntime(
		runtimeCfg,
		conversationRepo,
		log,
		appupload.Hooks{InitializeUploadedFile: processingService.InitializeUploadedFile},
		conversation.UploadErrorSet(),
		appprocessing.DefaultExtractorVersion,
	)
	uploadService.SetObjectStoreProvider(objectStoreProvider)
	ragService := apprag.NewServiceWithRuntime(runtimeCfg, conversationRepo, conversationCache, embedClient)
	conversationService := conversation.NewServiceWithRuntime(conversation.Dependencies{
		Config:            runtimeCfg,
		Repository:        conversationRepo,
		Cache:             conversationCache,
		RouteResolver:     channelService,
		MemoryRecorder:    memoryService,
		LLMClient:         llmClient,
		MediaDownloader:   mediaArtifactClient,
		MCPClient:         mcpClient,
		CompactService:    compactService,
		EmbeddingService:  embeddingService,
		ProcessingService: processingService,
		UploadService:     uploadService,
		ExtractService:    extractionService,
		RAGService:        ragService,
		Logger:            log,
	})
	conversationService.SetBillingService(billingService)
	conversationService.SetAuditWriter(auditService)
	conversationService.SetObjectStoreProvider(objectStoreProvider)
	conversationService.SetMCPRepository(mcpRepo)
	contentModerationRepo := contentmoderationrepo.NewRepo(db)
	contentModerationService := appcontentmoderation.NewService(settingsRepo, contentModerationRepo, cfg.DataEncryptionKey, log)
	moderationClient := moderationclient.New(trustedOutboundPolicy)
	contentModerationService.SetProvider(moderationClient)
	contentModerationService.SetAuditWriter(auditService)
	conversationService.SetModerationService(contentModerationService)
	contentModerationHandler := contentmoderationhttp.NewHandler(contentModerationService)
	contentModerationModule := contentmoderationhttp.NewModule(contentModerationHandler)
	userService.SetAvatarContentOpener(avatarContentOpener{uploads: uploadService})
	userService.SetAvatarFileValidator(uploadService)
	userService.SetActivityStatsRepository(billingRepo)
	authService.SetAvatarFileValidator(uploadService)
	memoryService.SetCacheInvalidator(conversationService.InvalidateMemoryCache)
	shutdownSignal := lifecycle.NewShutdown()
	conversationHandler := conversationhttp.NewHandler(conversationService, uploadService, processingService, runtimeCfg, shutdownSignal)
	conversationModule := conversationhttp.NewModule(conversationHandler)
	userHandler := userhttp.NewHandler(userService)
	userModule := userhttp.NewModule(userHandler)
	mcpService := appmcp.NewServiceWithRuntime(runtimeCfg, mcpRepo, mcpClient)
	mcpService.SetSystemEventWriter(systemEventService)
	mcpService.SetBillingModeProvider(billingService)
	mcpHandler := mcphttp.NewHandler(mcpService)
	mcpModule := mcphttp.NewModule(mcpHandler)
	adminService := admin.NewService(userService, auditService)
	adminService.SetAuthSecurityService(authService)
	adminService.SetSystemEventService(systemEventService)
	adminService.SetUsageLogService(billingService)
	adminService.SetUsageStatisticsService(billingService)
	adminService.SetOrderLogService(billingService)
	adminService.SetConversationEventService(conversationService)
	adminService.SetLogCleanupService(logCleanupService)
	adminService.SetSubscriptionResolver(billingService)
	adminService.SetOpenWebUIRowLoader(openwebui.NewRowLoader())
	adminService.SetPermissionGroupRepo(channelRepo)
	adminService.SetPermissionGroupModelLookup(channelRepo)
	adminService.SetPermissionGroupBillingPlanReferenceChecker(billingService)
	adminHandler := adminhttp.NewHandler(adminService)
	adminHandler.SetConversationExporter(conversationService)
	adminModule := adminhttp.NewModule(adminHandler)
	contentModerationHandler.SetUserLabelResolver(adminService)
	userSettingsRepo := usersettingsrepo.NewRepo(db)
	userSettingsService := usersettings.NewService(userSettingsRepo)
	userSettingsService.SetCacheRefresher(conversationService.RefreshUserSettingCache)
	userSettingsHandler := usersettingshttp.NewHandler(userSettingsService)
	userSettingsModule := usersettingshttp.NewModule(userSettingsHandler)
	announcementRepo := announcementrepo.NewRepo(db)
	announcementService := announcement.NewService(announcementRepo)
	announcementHandler := announcementhttp.NewHandler(announcementService)
	announcementModule := announcementhttp.NewModule(announcementHandler)
	promptPresetRepo := promptpresetrepo.NewRepo(db)
	promptPresetService := apppromptpreset.NewService(promptPresetRepo)
	promptPresetService.SetAuditWriter(auditService)
	promptPresetHandler := promptpresethttp.NewHandler(promptPresetService)
	promptPresetModule := promptpresethttp.NewModule(promptPresetHandler)
	skillRepo := skillrepo.NewRepo(db)
	skillService := appskill.NewService(skillRepo)
	skillService.SetAuditWriter(auditService)
	conversationService.SetSkillResolver(skillService)
	skillHandler := skillhttp.NewHandler(skillService)
	skillModule := skillhttp.NewModule(skillHandler)
	uiComponentService := appuicomponent.NewService(uicomponentrepo.NewRepo(db))
	uiComponentService.SetFeatureEnabled(func() bool { return runtimeCfg.Snapshot().UIComponentsEnabled })
	uiComponentService.SetAuditWriter(auditService)
	conversationService.SetUIComponentResolver(uiComponentService)
	uiComponentModule := uicomponenthttp.NewModule(uicomponenthttp.NewHandler(uiComponentService))
	knowledgeBaseRepo := knowledgebaserepo.NewRepo(db)
	knowledgeBaseService := appknowledgebase.NewService(knowledgeBaseRepo)
	knowledgeBaseService.SetAuditWriter(auditService)
	knowledgeBaseService.SetFileCleaner(uploadService)
	knowledgeBaseService.SetFileContentOpener(uploadService)
	knowledgeBaseService.SetFileUploader(uploadService)
	knowledgeBaseService.SetFileEmbeddingSubmitter(processingService)
	knowledgeBaseService.SetLogger(log)
	conversationService.SetKnowledgeBaseResolver(knowledgeBaseService)
	knowledgeBaseHandler := knowledgebasehttp.NewHandler(knowledgeBaseService, runtimeCfg)
	knowledgeBaseModule := knowledgebasehttp.NewModule(knowledgeBaseHandler)

	hc := newHealthChecker(db, cacheBackend)
	rateLimiter := cacheBackend.RateLimiter()
	engine, err := platformhttp.NewEngine(runtimeCfg, log, platformhttp.Modules{
		Auth:              authModule,
		AuthService:       authService,
		Channel:           channelModule,
		Conversation:      conversationModule,
		MCP:               mcpModule,
		Memory:            memoryModule,
		Billing:           billingModule,
		Admin:             adminModule,
		ContentModeration: contentModerationModule,
		Announcement:      announcementModule,
		PromptPreset:      promptPresetModule,
		Skill:             skillModule,
		UIComponent:       uiComponentModule,
		KnowledgeBase:     knowledgeBaseModule,
		Settings:          settingsModule,
		UserSettings:      userSettingsModule,
		User:              userModule,
		Shutdown:          shutdownSignal,
		StartupLog: func(log *zap.Logger) {
			if log == nil || bootstrapSuperAdmin == nil {
				return
			}
			log.Info("bootstrap superadmin created",
				zap.String("username", bootstrapSuperAdmin.Username),
				zap.String("password", bootstrapSuperAdmin.Password),
			)
		},
	}, hc, rateLimiter)
	if err != nil {
		return nil, err
	}

	backgroundCtx, backgroundCancel := context.WithCancel(context.Background())
	if _, reconcileErr := embeddingService.ReconcileIndex(backgroundCtx); reconcileErr != nil {
		log.Warn("embedding index reconciliation failed", zap.Error(reconcileErr))
	}
	embeddingService.StartBackgroundWorkers(backgroundCtx)
	conversationService.StartBackgroundWorkers(backgroundCtx)
	contentModerationService.StartBackgroundWorkers(backgroundCtx)
	channelService.StartModelIconAssetCleanup(backgroundCtx)

	app := &App{
		stopCh:                 make(chan struct{}),
		cfg:                    runtimeCfg.Snapshot(),
		engine:                 engine,
		logger:                 log,
		db:                     db,
		cache:                  cacheBackend,
		geoResolver:            geoResolver,
		identityProviderClient: identityProviderClient,
		llmClient:              llmClient,
		mcpClient:              mcpClient,
		embeddingClient:        embedClient,
		mediaArtifactClient:    mediaArtifactClient,
		moderationClient:       moderationClient,
		conversationService:    conversationService,
		contentModeration:      contentModerationService,
		authService:            authService,
		runtimeCfg:             runtimeCfg,
		tracingShutdown:        tracingShutdown,
		backgroundCancel:       backgroundCancel,
		shutdown:               shutdownSignal,
	}
	keepTracing = true
	return app, nil
}

// Run 启动 HTTP 服务并支持优雅停机。
// IssueLocalGrant 生成本地模式的一次性登录 grant（仅本地模式）。
func (a *App) IssueLocalGrant() (string, error) {
	if !a.cfg.LocalMode {
		return "", errors.New("local grant is only available in local mode")
	}
	return a.authService.IssueLocalGrant()
}

// Listen 绑定监听地址并返回实际地址。本地模式绑定 127.0.0.1:0，端口由系统分配；
// 调用方在 Serve 之前即可据此完成与父进程的握手。
func (a *App) Listen() (net.Listener, error) {
	addr := strings.TrimSpace(a.cfg.HTTPListenAddr)
	if addr == "" {
		addr = fmt.Sprintf(":%s", a.cfg.HTTPPort)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if a.cfg.LocalMode {
		origin := "http://" + listener.Addr().String()
		a.cfg.SetLocalOrigin(origin)
		snapshot := a.runtimeCfg.Snapshot()
		snapshot.SetLocalOrigin(origin)
		a.runtimeCfg.Store(snapshot)
	}
	return listener, nil
}

// Run 监听并服务，直到收到终止信号。
func (a *App) Run() error {
	listener, err := a.Listen()
	if err != nil {
		return err
	}
	return a.Serve(listener)
}

// Serve 在已绑定的监听器上服务，直到收到终止信号；随后分阶段排空。
// RequestShutdown triggers the same graceful drain as SIGTERM. Safe to call
// more than once; used by local mode when the desktop shell goes away.
func (a *App) RequestShutdown() {
	a.stopOnce.Do(func() { close(a.stopCh) })
}

func (a *App) Serve(listener net.Listener) error {
	srv := &http.Server{
		Handler:           a.engine,
		ReadHeaderTimeout: httpTimeoutSeconds(a.cfg.HTTPReadHeaderTimeoutSeconds, 10),
		ReadTimeout:       httpTimeoutSeconds(a.cfg.HTTPReadTimeoutSeconds, 120),
		IdleTimeout:       httpTimeoutSeconds(a.cfg.HTTPIdleTimeoutSeconds, 120),
		MaxHeaderBytes:    httpMaxHeaderBytes(a.cfg.HTTPMaxHeaderBytes),
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("server_starting", zap.String("addr", listener.Addr().String()))
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		a.logger.Info("server_shutting_down", zap.String("signal", sig.String()))
	case <-a.stopCh:
		a.logger.Info("server_shutting_down", zap.String("signal", "parent_exit"))
	}

	// 阶段一：进入排空。就绪探针翻转为 503 引导负载均衡摘流，
	// 订阅型 SSE（run 对账流、run 观看流）立即断开，客户端按既有逻辑重连。
	a.shutdown.BeginDrain()

	// 阶段二：排空 in-flight 请求。消息生成等有价值的流式请求在窗口内自然完成。
	drainTimeout := httpTimeoutSeconds(a.cfg.HTTPShutdownTimeoutSeconds, 10)
	ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		// 阶段三：排空超时，强断剩余连接。被打断的生成已有落盘与前端恢复兜底，
		// 属预期内降级而非故障，进程仍以成功状态退出。
		a.logger.Warn("server_drain_timeout_force_close",
			zap.Duration("drain_timeout", drainTimeout),
			zap.Error(err),
		)
		if closeErr := srv.Close(); closeErr != nil {
			a.logger.Warn("server_force_close_error", zap.Error(closeErr))
		}
	}

	// HTTP 排空完成后再停后台 worker；资源释放由 cli.Run 的 defer Close() 收尾。
	if a.backgroundCancel != nil {
		a.backgroundCancel()
	}
	a.logger.Info("server_stopped")
	return nil
}

func httpTimeoutSeconds(value int, fallback int) time.Duration {
	if value <= 0 {
		value = fallback
	}
	return time.Duration(value) * time.Second
}

func httpMaxHeaderBytes(value int) int {
	if value <= 0 {
		return 1 << 20
	}
	return value
}

// Close 关闭资源。
func (a *App) Close() {
	if a.backgroundCancel != nil {
		a.backgroundCancel()
	}
	// Workers must be drained before their dependencies (cache, database) close.
	if a.contentModeration != nil {
		a.contentModeration.Stop()
	}
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), 5*time.Second)
	if err := background.Wait(drainCtx); err != nil {
		a.logger.Warn("background_tasks_drain_timeout", zap.Error(err))
	}
	cancelDrain()
	if a.conversationService != nil {
		a.conversationService.Close()
	}
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.geoResolver != nil {
		a.geoResolver.Close()
	}
	if a.identityProviderClient != nil {
		a.identityProviderClient.CloseIdleConnections()
	}
	if a.llmClient != nil {
		a.llmClient.CloseIdleConnections()
	}
	if a.mcpClient != nil {
		a.mcpClient.CloseIdleConnections()
	}
	if a.embeddingClient != nil {
		a.embeddingClient.CloseIdleConnections()
	}
	if a.mediaArtifactClient != nil {
		a.mediaArtifactClient.CloseIdleConnections()
	}
	if a.moderationClient != nil {
		a.moderationClient.CloseIdleConnections()
	}
	if a.db != nil {
		if sqlDB, err := a.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if a.tracingShutdown != nil {
		_ = a.tracingShutdown(shutdownCtx)
	}
	a.logger.Sync() //nolint:errcheck
}
