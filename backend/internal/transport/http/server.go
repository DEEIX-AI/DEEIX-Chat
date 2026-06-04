package httpx

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	adminhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/admin"
	announcementhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/announcement"
	authhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/auth"
	billinghttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/billing"
	channelhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/channel"
	conversationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/conversation"
	mcphttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/mcp"
	memoryhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	settingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/settings"
	usersettingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/usersettings"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// HealthCheck 表示单个健康检查项的结果。
type HealthCheck struct {
	Name   string
	Status string
}

// HealthChecker 封装服务健康检查能力。
type HealthChecker interface {
	// CheckHealth 执行所有健康检查，返回检查结果列表。
	// 当所有检查均通过时 healthy 为 true。
	CheckHealth(ctx context.Context) (checks []HealthCheck, healthy bool)
}

// Modules 聚合可注册的业务模块。
type Modules struct {
	Auth         *authhttp.Module
	AuthService  middleware.SessionValidator
	Channel      *channelhttp.Module
	Conversation *conversationhttp.Module
	MCP          *mcphttp.Module
	Memory       *memoryhttp.Module
	Billing      *billinghttp.Module
	Admin        *adminhttp.Module
	Announcement *announcementhttp.Module
	Settings     *settingshttp.Module
	UserSettings *usersettingshttp.Module
}

// NewEngine 创建并注册 API 路由。
func NewEngine(cfg *config.Runtime, log *zap.Logger, modules Modules, hc HealthChecker, limiter middleware.RateLimiter) (*gin.Engine, error) {
	snapshot := cfg.Snapshot()
	if snapshot.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.MaxMultipartMemory = 8 << 20
	if err := engine.SetTrustedProxies(snapshot.TrustedProxyList()); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	if err := middleware.ConfigureTrustedProxyHeaders(snapshot.TrustedProxyList()); err != nil {
		return nil, fmt.Errorf("configure trusted proxy headers: %w", err)
	}
	engine.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if log != nil {
			log.Error("http_panic_recovered", zap.Any("error", recovered), zap.ByteString("stack", debug.Stack()))
		}
		response.ErrorWithCode(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		c.Abort()
	}))
	engine.Use(otelgin.Middleware(snapshot.AppName, otelgin.WithFilter(func(req *http.Request) bool {
		return req.URL.Path != "/healthz"
	})))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.AccessLog(log))
	engine.Use(middleware.SecurityHeaders(snapshot.Env))
	engine.Use(middleware.CORS(snapshot.CORSAllowOrigin))

	engine.GET("/healthz", func(c *gin.Context) {
		info := buildinfo.Snapshot()
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": info.Version})
	})
	engine.GET("/readyz", readyzHandler(hc))
	if swaggerEnabled(snapshot.Env) {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := engine.Group("/api/v1")
	api.GET("/version", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.JSON(http.StatusOK, buildinfo.Snapshot())
	})
	if modules.Auth != nil || modules.Settings != nil || modules.Billing != nil || modules.Conversation != nil {
		publicAuth := api.Group("")
		publicAuth.Use(middleware.PublicAuthRateLimit(limiter, cfg))
		if modules.Auth != nil {
			modules.Auth.RegisterPublicRoutes(publicAuth)
		}
		if modules.Conversation != nil {
			modules.Conversation.RegisterPublicRoutes(publicAuth)
		}
		if modules.Settings != nil {
			modules.Settings.RegisterPublicRoutes(publicAuth)
		}
		if modules.Billing != nil {
			modules.Billing.RegisterPublicRoutes(publicAuth)
		}
	}

	authRequired := api.Group("")
	authRequired.Use(middleware.AuthMiddleware(snapshot.JWTSecret, modules.AuthService))
	authRequired.Use(middleware.RateLimit(limiter, cfg))

	if modules.Auth != nil {
		modules.Auth.RegisterProtectedRoutes(authRequired)
	}
	if modules.Conversation != nil {
		modules.Conversation.RegisterRoutes(authRequired)
	}
	if modules.Channel != nil {
		modules.Channel.RegisterRoutes(authRequired)
	}
	if modules.Memory != nil {
		modules.Memory.RegisterRoutes(authRequired)
	}
	if modules.MCP != nil {
		modules.MCP.RegisterRoutes(authRequired)
	}
	if modules.Billing != nil {
		modules.Billing.RegisterRoutes(authRequired)
	}
	if modules.Announcement != nil {
		modules.Announcement.RegisterRoutes(authRequired)
	}
	if modules.UserSettings != nil {
		modules.UserSettings.RegisterRoutes(authRequired)
	}
	if modules.Settings != nil {
		modules.Settings.RegisterRoutes(authRequired)
	}
	if modules.Admin != nil || modules.Auth != nil || modules.Billing != nil || modules.Channel != nil || modules.MCP != nil || modules.Settings != nil || modules.Announcement != nil {
		adminGroup := authRequired.Group("/admin")
		adminGroup.Use(middleware.AdminOnly())
		if modules.Auth != nil {
			modules.Auth.RegisterAdminRoutes(adminGroup)
		}
		if modules.Admin != nil {
			modules.Admin.RegisterRoutes(adminGroup)
		}
		if modules.Billing != nil {
			modules.Billing.RegisterAdminRoutes(adminGroup)
		}
		if modules.Channel != nil {
			modules.Channel.RegisterAdminRoutes(adminGroup)
		}
		if modules.MCP != nil {
			modules.MCP.RegisterAdminRoutes(adminGroup)
		}
		if modules.Settings != nil {
			modules.Settings.RegisterAdminRoutes(adminGroup)
		}
		if modules.Announcement != nil {
			modules.Announcement.RegisterAdminRoutes(adminGroup)
		}
	}

	registerFrontendStatic(engine, snapshot.FrontendDistDir, log)

	return engine, nil
}

func registerFrontendStatic(engine *gin.Engine, distDir string, log *zap.Logger) {
	root := strings.TrimSpace(distDir)
	if root == "" {
		return
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		if log != nil {
			log.Warn("frontend_static_path_invalid", zap.String("path", root), zap.Error(err))
		}
		return
	}

	info, err := os.Stat(absoluteRoot)
	if err != nil || !info.IsDir() {
		if log != nil {
			log.Warn("frontend_static_disabled", zap.String("path", absoluteRoot), zap.Error(err))
		}
		return
	}

	if log != nil {
		log.Info("frontend_static_enabled", zap.String("path", absoluteRoot))
	}

	engine.NoRoute(func(c *gin.Context) {
		requestPath := cleanFrontendPath(c.Request.URL.Path)
		if isBackendOnlyPath(requestPath) {
			response.ErrorWithCode(c, http.StatusNotFound, response.CodeResourceNotFound, "not found")
			return
		}

		if filePath, ok := resolveFrontendStaticFile(absoluteRoot, requestPath); ok {
			applyFrontendCacheHeaders(c, requestPath)
			c.File(filePath)
			return
		}

		if filePath, ok := resolveFrontendPageFile(absoluteRoot, requestPath); ok {
			c.Header("Cache-Control", "no-cache")
			c.File(filePath)
			return
		}

		notFoundPath := filepath.Join(absoluteRoot, "404.html")
		if isRegularFile(notFoundPath) {
			c.Status(http.StatusNotFound)
			c.File(notFoundPath)
			return
		}

		response.ErrorWithCode(c, http.StatusNotFound, response.CodeResourceNotFound, "not found")
	})
}

func swaggerEnabled(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "dev", "development":
		return true
	default:
		return false
	}
}

func cleanFrontendPath(rawPath string) string {
	if rawPath == "" || rawPath == "/" {
		return "/"
	}
	return path.Clean("/" + strings.TrimPrefix(rawPath, "/"))
}

func isBackendOnlyPath(requestPath string) bool {
	return requestPath == "/api" ||
		strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/swagger" ||
		strings.HasPrefix(requestPath, "/swagger/") ||
		requestPath == "/healthz" ||
		requestPath == "/readyz"
}

func resolveFrontendStaticFile(root string, requestPath string) (string, bool) {
	if requestPath == "/" {
		return "", false
	}
	candidate := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(requestPath, "/")))
	if !strings.HasPrefix(candidate, root) {
		return "", false
	}
	if isRegularFile(candidate) {
		return candidate, true
	}
	return "", false
}

func resolveFrontendPageFile(root string, requestPath string) (string, bool) {
	candidates := []string{filepath.Join(root, "index.html")}
	if requestPath != "/" {
		cleanPath := filepath.FromSlash(strings.TrimPrefix(requestPath, "/"))
		candidates = []string{
			filepath.Join(root, cleanPath+".html"),
			filepath.Join(root, cleanPath, "index.html"),
			filepath.Join(root, "index.html"),
		}
	}

	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, root) && isRegularFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isRegularFile(filePath string) bool {
	info, err := os.Stat(filePath)
	return err == nil && !info.IsDir()
}

func applyFrontendCacheHeaders(c *gin.Context, requestPath string) {
	if isImmutableFrontendAsset(requestPath) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	if isVendorIconAsset(requestPath) {
		c.Header("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
		return
	}
	if isNextExportDataAsset(requestPath) {
		c.Header("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
}

func isImmutableFrontendAsset(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/_next/static/") ||
		strings.HasPrefix(requestPath, "/fonts/")
}

func isVendorIconAsset(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/vendor/lobehub-icons/")
}

func isNextExportDataAsset(requestPath string) bool {
	fileName := path.Base(requestPath)
	return strings.HasPrefix(fileName, "__next.") && strings.EqualFold(path.Ext(fileName), ".txt")
}

func readyzHandler(hc HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var healthy bool
		checksMap := gin.H{}

		if hc != nil {
			results, ok := hc.CheckHealth(ctx)
			healthy = ok
			for _, r := range results {
				checksMap[r.Name] = r.Status
			}
		} else {
			healthy = true
		}

		status := http.StatusOK
		if !healthy {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"status": map[bool]string{true: "ok", false: "degraded"}[healthy],
			"checks": checksMap,
		})
	}
}
