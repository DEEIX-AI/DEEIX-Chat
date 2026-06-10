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
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	appmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/memory"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	appruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/runtime"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	appsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/geoip"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
	platformlogger "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/logger"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	announcementrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/announcement"
	auditrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/audit"
	billingrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/billing"
	channelrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/channel"
	conversationrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	mcprepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/mcp"
	memoryrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/memory"
	settingsrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/settings"
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
	conversationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/conversation"
	mcphttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/mcp"
	memoryhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/memory"
	settingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/settings"
	usersettingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/usersettings"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App 维护应用运行依赖。
type App struct {
	cfg              config.Config
	engine           *gin.Engine
	logger           *zap.Logger
	db               *gorm.DB
	redis            *redis.Client
	geoResolver      *geoip.Client
	backgroundCancel context.CancelFunc
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

	// 启动时确保 embedding_model_signature 已写入：首次部署或签名字段为空时自动补全。
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
	billingHandler := billinghttp.NewHandler(billingService, settingsService, runtimeCfg)
	billingModule := billinghttp.NewModule(billingHandler)
	objectStoreProvider := appstorage.NewRuntimeProvider(runtimeCfg, nil)
	geoResolver := geoip.New(runtimeCfg.Snapshot())
	authService := auth.NewServiceWithRuntime(runtimeCfg, userRepo, geoResolver)
	authService.SetLogger(log)
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
	llmClient := llm.NewClientWithEnv(cfg.Env, cfg.SSRFProtectionEnabled)
	mcpClient := mcp.NewClientWithEnv(cfg.Env, cfg.SSRFProtectionEnabled)
	channelService := channel.NewServiceWithRuntime(runtimeCfg, channelRepo, channelCache, llmClient)
	channelService.SetLogger(log)
	channelService.SetBillingModelPricingFilter(billingService)
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
	embedClient := embedding.NewWithEnv(cfg.Env, cfg.SSRFProtectionEnabled)
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
	memoryService.SetCacheInvalidator(conversationService.InvalidateMemoryCache)
	conversationHandler := conversationhttp.NewHandler(conversationService, runtimeCfg)
	conversationModule := conversationhttp.NewModule(conversationHandler)
	mcpService := appmcp.NewServiceWithRuntime(runtimeCfg, mcpRepo, mcpClient)
	mcpService.SetSystemEventWriter(systemEventService)
	mcpHandler := mcphttp.NewHandler(mcpService)
	mcpModule := mcphttp.NewModule(mcpHandler)
	adminService := admin.NewService(userService, auditService)
	adminService.SetAuthSecurityService(authService)
	adminService.SetSystemEventService(systemEventService)
	adminService.SetUsageLogService(billingService)
	adminService.SetOrderLogService(billingService)
	adminService.SetConversationEventService(conversationService)
	adminService.SetSubscriptionResolver(billingService)
	adminHandler := adminhttp.NewHandler(adminService)
	adminModule := adminhttp.NewModule(adminHandler)
	userSettingsRepo := usersettingsrepo.NewRepo(db)
	userSettingsService := usersettings.NewService(userSettingsRepo)
	userSettingsHandler := usersettingshttp.NewHandler(userSettingsService)
	userSettingsModule := usersettingshttp.NewModule(userSettingsHandler)
	announcementRepo := announcementrepo.NewRepo(db)
	announcementService := announcement.NewService(announcementRepo)
	announcementHandler := announcementhttp.NewHandler(announcementService)
	announcementModule := announcementhttp.NewModule(announcementHandler)

	hc := newHealthChecker(db, cfg.CacheDriver, redisClient)
	rateLimiter := buildRateLimiter(cfg, redisClient, memoryCache)
	engine, err := platformhttp.NewEngine(runtimeCfg, log, platformhttp.Modules{
		Auth:         authModule,
		AuthService:  authService,
		Channel:      channelModule,
		Conversation: conversationModule,
		MCP:          mcpModule,
		Memory:       memoryModule,
		Billing:      billingModule,
		Admin:        adminModule,
		Announcement: announcementModule,
		Settings:     settingsModule,
		UserSettings: userSettingsModule,
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
	conversationService.StartBackgroundWorkers(backgroundCtx)

	return &App{
		cfg:              runtimeCfg.Snapshot(),
		engine:           engine,
		logger:           log,
		db:               db,
		redis:            redisClient,
		geoResolver:      geoResolver,
		backgroundCancel: backgroundCancel,
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
