package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	moderationclient "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/geoip"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/identityprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mediaartifact"
	openrouterpricing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelpricing/openrouter"
	platformlogger "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/logger"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/openwebui"
	epaypayment "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/payment/epay"
	stripepayment "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/payment/stripe"
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
	userrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/user"
	usersettingsrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/usersettings"
	platformruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/runtime"
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
	userhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/user"
	usersettingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/usersettings"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 维护应用运行依赖。
type App struct {
	cfg                    config.Config
	engine                 *gin.Engine
	logger                 *zap.Logger
	db                     *gorm.DB
	redis                  *redis.Client
	geoResolver            *geoip.Client
	identityProviderClient *identityprovider.Client
	llmClient              *llm.Client
	mcpClient              *mcp.Client
	embeddingClient        *embedding.Client
	mediaArtifactClient    *mediaartifact.Client
	moderationClient       *moderationclient.Client
	backgroundCancel       context.CancelFunc
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
	conversationService *conversation.Service
}

func (o avatarContentOpener) OpenAvatarFileContent(ctx context.Context, userID uint, fileID string) (*user.AvatarFileContent, error) {
	content, err := o.conversationService.OpenFileContent(ctx, userID, fileID)
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

// NewApp 创建应用。
func NewApp() (*App, error) {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	runtimeCfg := config.NewRuntime(cfg)

	if err := platformtracing.Init(context.Background(), platformtracing.Config{
		ServiceName:  cfg.AppName,
		Enabled:      cfg.OTelEnabled,
		Endpoint:     cfg.OTelExporterOTLPEndpoint,
		Headers:      cfg.OTelExporterOTLPHeaders,
		Insecure:     cfg.OTelExporterOTLPInsecure,
		Protocol:     cfg.OTelExporterOTLPProtocol,
		SamplingRate: cfg.OTelSamplingRate,
	}); err != nil {
		return nil, fmt.Errorf("init tracing: %w", err)
	}

	log, err := platformlogger.New(cfg.Env)
	if err != nil {
		return nil, err
	}

	db, err := openDatabase(cfg)
	if err != nil {
		return nil, err
	}

	redisClient, memoryCache, err := openCache(cfg)
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
	runtimeService := appruntime.NewService(runtimeCfg)
	runtimeService.SetDockerRunner(platformruntime.NewDockerRunner())
	settingsCache := buildSettingsCache(cfg, redisClient, memoryCache)
	runtimeSettings := settings.NewRuntimeSettings(settingsRepo, settingsCache, cfg.DataEncryptionKey)
	settingsHandler := settingshttp.NewHandler(settingsService, runtimeSettings, runtimeService, runtimeCfg)
	settingsModule := settingshttp.NewModule(settingsHandler)
	if err = settingsService.Seed(context.Background(), cfg); err != nil {
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
	objectStoreProvider := appstorage.NewRuntimeProvider(runtimeCfg, nil)
	geoResolver := geoip.New(runtimeCfg.Snapshot())
	identityProviderClient := identityprovider.New(cfg.StrictOutboundPolicy())
	authService := auth.NewServiceWithRuntime(
		runtimeCfg,
		userRepo,
		geoResolver,
		identityProviderClient,
	)
	authService.SetLogger(log)
	authService.SetProviderAuthBridge(buildProviderAuthBridge(cfg, redisClient, memoryCache))
	authService.SetObjectStoreProvider(objectStoreProvider)
	authService.SetAuditWriter(auditService)
	settingsService.SetAuthSafetyService(authService)
	authService.SetSubscriptionResolver(billingService)
	bootstrapSuperAdmin, err := authService.EnsureBootstrapSuperAdmin(context.Background())
	if err != nil {
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
	channelCache := buildChannelCache(cfg, redisClient, memoryCache)
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
	conversationCache := buildConversationCache(cfg, redisClient, memoryCache)
	mcpRepo := mcprepo.NewRepo(db)
	embedClient := embedding.New(trustedOutboundPolicy)
	compactService := compact.NewServiceWithRuntime(runtimeCfg, conversationRepo, log)
	extractionService := extraction.NewServiceWithRuntime(runtimeCfg)
	extractionService.SetObjectStoreProvider(objectStoreProvider)
	embeddingService := appembedding.NewServiceWithRuntime(runtimeCfg, conversationRepo, extractionService, embedClient, log)
	memoryService.SetEmbeddingProvider(embeddingService)
	settingsHandler.SetEmbeddingService(embeddingService)
	processingService := appprocessing.NewServiceWithRuntime(runtimeCfg, conversationRepo, conversationCache, extractionService, embeddingService, log, appprocessing.DefaultExtractorVersion)
	ragService := apprag.NewServiceWithRuntime(runtimeCfg, conversationRepo, conversationCache, embedClient)
	conversationService := conversation.NewServiceWithRuntime(
		runtimeCfg,
		conversationRepo,
		conversationCache,
		channelService,
		memoryService,
		llmClient,
		mediaArtifactClient,
		mcpClient,
		embedClient,
		nil,
		compactService,
		embeddingService,
		processingService,
		extractionService,
		ragService,
		log,
	)
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
	userService.SetAvatarContentOpener(avatarContentOpener{conversationService: conversationService})
	userService.SetAvatarFileValidator(conversationService)
	authService.SetAvatarFileValidator(conversationService)
	memoryService.SetCacheInvalidator(conversationService.InvalidateMemoryCache)
	conversationHandler := conversationhttp.NewHandler(conversationService, runtimeCfg)
	conversationModule := conversationhttp.NewModule(conversationHandler)
	userHandler := userhttp.NewHandler(userService)
	userModule := userhttp.NewModule(userHandler)
	mcpService := appmcp.NewServiceWithRuntime(runtimeCfg, mcpRepo, mcpClient)
	mcpService.SetSystemEventWriter(systemEventService)
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
	knowledgeBaseRepo := knowledgebaserepo.NewRepo(db)
	knowledgeBaseService := appknowledgebase.NewService(knowledgeBaseRepo)
	knowledgeBaseService.SetAuditWriter(auditService)
	knowledgeBaseService.SetFileCleaner(conversationService)
	knowledgeBaseService.SetFileContentOpener(conversationService)
	knowledgeBaseService.SetFileUploader(conversationService)
	knowledgeBaseService.SetLogger(log)
	conversationService.SetKnowledgeBaseResolver(knowledgeBaseService)
	knowledgeBaseHandler := knowledgebasehttp.NewHandler(knowledgeBaseService, runtimeCfg)
	knowledgeBaseModule := knowledgebasehttp.NewModule(knowledgeBaseHandler)

	hc := newHealthChecker(db, cfg.CacheDriver, redisClient)
	rateLimiter := buildRateLimiter(cfg, redisClient, memoryCache)
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
		KnowledgeBase:     knowledgeBaseModule,
		Settings:          settingsModule,
		UserSettings:      userSettingsModule,
		User:              userModule,
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

	return &App{
		cfg:                    runtimeCfg.Snapshot(),
		engine:                 engine,
		logger:                 log,
		db:                     db,
		redis:                  redisClient,
		geoResolver:            geoResolver,
		identityProviderClient: identityProviderClient,
		llmClient:              llmClient,
		mcpClient:              mcpClient,
		embeddingClient:        embedClient,
		mediaArtifactClient:    mediaArtifactClient,
		moderationClient:       moderationClient,
		backgroundCancel:       backgroundCancel,
	}, nil
}

// Run 启动 HTTP 服务并支持优雅停机。
func (a *App) Run() error {
	addr := fmt.Sprintf(":%s", a.cfg.HTTPPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.engine,
		ReadHeaderTimeout: httpTimeoutSeconds(a.cfg.HTTPReadHeaderTimeoutSeconds, 10),
		ReadTimeout:       httpTimeoutSeconds(a.cfg.HTTPReadTimeoutSeconds, 120),
		IdleTimeout:       httpTimeoutSeconds(a.cfg.HTTPIdleTimeoutSeconds, 120),
		MaxHeaderBytes:    httpMaxHeaderBytes(a.cfg.HTTPMaxHeaderBytes),
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("server_starting", zap.String("port", a.cfg.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
	}

	if a.backgroundCancel != nil {
		a.backgroundCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		a.logger.Error("server_shutdown_error", zap.Error(err))
		return err
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
	if a.redis != nil {
		_ = a.redis.Close()
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
	platformtracing.Shutdown(shutdownCtx)
	a.logger.Sync() //nolint:errcheck
}
