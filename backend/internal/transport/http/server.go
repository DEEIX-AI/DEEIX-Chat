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
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/lifecycle"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
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
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	promptpresethttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/promptpreset"
	settingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/settings"
	skillhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/skill"
	systemhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/system"
	uicomponenthttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/uicomponent"
	userhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/user"
	usersettingshttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/usersettings"
	"github.com/gin-gonic/gin"
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
	Auth              *authhttp.Module
	AuthService       middleware.SessionValidator
	Channel           *channelhttp.Module
	Conversation      *conversationhttp.Module
	MCP               *mcphttp.Module
	Memory            *memoryhttp.Module
	Billing           *billinghttp.Module
	Admin             *adminhttp.Module
	ContentModeration *contentmoderationhttp.Module
	Announcement      *announcementhttp.Module
	PromptPreset      *promptpresethttp.Module
	Skill             *skillhttp.Module
	UIComponent       *uicomponenthttp.Module
	KnowledgeBase     *knowledgebasehttp.Module
	Settings          *settingshttp.Module
	User              *userhttp.Module
	UserSettings      *usersettingshttp.Module
	StartupLog        func(*zap.Logger)
	// Shutdown 是进程关停排空信号；排空期间就绪探针返回 503，引导负载均衡摘除流量。
	Shutdown *lifecycle.Shutdown
}

// hasAdminRoutes 报告是否存在需要注册到 /admin 路由组的模块；新增管理端模块时须同步更新此处。
func (m Modules) hasAdminRoutes() bool {
	return m.Admin != nil ||
		m.Auth != nil ||
		m.Billing != nil ||
		m.Channel != nil ||
		m.MCP != nil ||
		m.Settings != nil ||
		m.Announcement != nil ||
		m.PromptPreset != nil ||
		m.Skill != nil ||
		m.UIComponent != nil ||
		m.KnowledgeBase != nil ||
		m.ContentModeration != nil
}

// NewEngine 创建并注册 API 路由。
func NewEngine(cfg *config.Runtime, log *zap.Logger, modules Modules, hc HealthChecker, limiter middleware.RateLimiter) (*gin.Engine, error) {
	snapshot := cfg.Snapshot()
	if snapshot.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	if snapshot.LocalMode {
		// stdout 是 sidecar 与父进程的握手通道，框架自身的输出一律走 stderr。
		gin.DefaultWriter = os.Stderr
		gin.DefaultErrorWriter = os.Stderr
	}

	engine := gin.New()
	engine.MaxMultipartMemory = 8 << 20
	if err := engine.SetTrustedProxies(snapshot.TrustedProxyList()); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	trustedProxyHeaders, err := middleware.TrustedProxyHeaders(snapshot.TrustedProxyList())
	if err != nil {
		return nil, fmt.Errorf("configure trusted proxy headers: %w", err)
	}
	engine.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		if log != nil {
			log.Error("http_panic_recovered", zap.Any("error", recovered), zap.ByteString("stack", debug.Stack()))
		}
		response.ErrorWithCode(c, http.StatusInternalServerError, response.CodeInternal)
		c.Abort()
	}))
	engine.Use(otelgin.Middleware(snapshot.AppName, otelgin.WithFilter(func(req *http.Request) bool {
		return req.URL.Path != "/healthz"
	})))
	engine.Use(middleware.RequestID())
	engine.Use(trustedProxyHeaders)
	engine.Use(middleware.AccessLog(log))
	engine.Use(middleware.SecurityHeaders())
	engine.Use(middleware.CORS(snapshot.CORSAllowOrigin))

	engine.GET("/healthz", func(c *gin.Context) {
		info := buildinfo.Snapshot()
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": info.Version})
	})
	engine.GET("/readyz", readyzHandler(hc, modules.Shutdown))
	if swaggerEnabled(snapshot.Env) {
		mountSwagger(engine)
	}

	api := engine.Group("/api/v1")
	// 能力位关闭的功能整组返回 404 feature.disabled；见 docs/ARCHITECTURE.md §4。
	gate := middleware.NewFeatureGate(cfg)
	api.GET("/version", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.JSON(http.StatusOK, buildinfo.Snapshot())
	})
	{
		publicAuth := api.Group("")
		publicAuth.Use(middleware.PublicAuthRateLimit(limiter, cfg))
		systemhttp.NewModule(systemhttp.NewHandler(cfg)).RegisterPublicRoutes(publicAuth)
		if modules.Auth != nil {
			modules.Auth.RegisterPublicRoutes(publicAuth, gate)
			if snapshot.LocalMode {
				modules.Auth.RegisterLocalRoutes(publicAuth)
			}
		}
		if modules.User != nil {
			modules.User.RegisterPublicRoutes(publicAuth)
		}
		if modules.Channel != nil {
			modules.Channel.RegisterPublicRoutes(publicAuth)
		}
		if modules.Conversation != nil {
			modules.Conversation.RegisterPublicRoutes(publicAuth, gate)
		}
		if modules.Settings != nil {
			modules.Settings.RegisterPublicRoutes(publicAuth)
		}
		if modules.Billing != nil {
			modules.Billing.RegisterPublicRoutes(publicAuth, gate)
		}
	}

	authRequired := api.Group("")
	authRequired.Use(middleware.AuthMiddleware(snapshot.JWTSecret, modules.AuthService))
	authRequired.Use(middleware.RateLimit(limiter, cfg))

	if modules.Auth != nil {
		modules.Auth.RegisterProtectedRoutes(authRequired, gate)
	}
	if modules.Conversation != nil {
		modules.Conversation.RegisterRoutes(authRequired, gate)
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
		modules.Billing.RegisterRoutes(authRequired, gate)
	}
	if modules.Announcement != nil {
		modules.Announcement.RegisterRoutes(authRequired, gate)
	}
	if modules.PromptPreset != nil {
		modules.PromptPreset.RegisterRoutes(authRequired)
	}
	if modules.Skill != nil {
		modules.Skill.RegisterRoutes(authRequired)
	}
	if modules.UIComponent != nil {
		modules.UIComponent.RegisterRoutes(authRequired)
	}
	if modules.KnowledgeBase != nil {
		modules.KnowledgeBase.RegisterRoutes(authRequired)
	}
	if modules.UserSettings != nil {
		modules.UserSettings.RegisterRoutes(authRequired)
	}
	if modules.Settings != nil {
		modules.Settings.RegisterRoutes(authRequired)
	}
	if modules.User != nil {
		modules.User.RegisterRoutes(authRequired)
	}
	if modules.hasAdminRoutes() {
		adminGroup := authRequired.Group("/admin")
		adminGroup.Use(middleware.AdminOnly())
		if modules.Auth != nil {
			modules.Auth.RegisterAdminRoutes(adminGroup, gate)
		}
		if modules.Admin != nil {
			modules.Admin.RegisterRoutes(adminGroup, gate)
		}
		if modules.ContentModeration != nil {
			modules.ContentModeration.RegisterRoutes(adminGroup, gate)
		}
		if modules.Billing != nil {
			modules.Billing.RegisterAdminRoutes(adminGroup, gate)
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
			modules.Announcement.RegisterAdminRoutes(adminGroup, gate)
		}
		if modules.PromptPreset != nil {
			modules.PromptPreset.RegisterAdminRoutes(adminGroup)
		}
		if modules.Skill != nil {
			modules.Skill.RegisterAdminRoutes(adminGroup)
		}
		if modules.UIComponent != nil {
			modules.UIComponent.RegisterAdminRoutes(adminGroup)
		}
		if modules.KnowledgeBase != nil {
			modules.KnowledgeBase.RegisterAdminRoutes(adminGroup)
		}
	}

	if modules.StartupLog != nil {
		modules.StartupLog(log)
	}
	if modules.Settings != nil {
		modules.Settings.RegisterFrontendRoutes(engine)
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
			response.ErrorWithCode(c, http.StatusNotFound, response.CodeResourceNotFound)
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

		response.ErrorWithCode(c, http.StatusNotFound, response.CodeResourceNotFound)
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
		// 图标按包版本放在 /vendor/lobehub-icons/<version>/ 下，升级即换路径，可以永久缓存。
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	if isNextExportDataAsset(requestPath) {
		// 导出的 RSC 载荷携带 buildId，与 HTML 页面一同失效；缓存旧载荷会让客户端路由退化为整页刷新。
		c.Header("Cache-Control", "no-cache")
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
}

func isImmutableFrontendAsset(requestPath string) bool {
	return strings.HasPrefix(requestPath, "/_next/static/") ||
		strings.HasPrefix(requestPath, "/fonts/")
}

// isVendorIconAsset 只匹配带版本目录的内置图标（/vendor/lobehub-icons/<version>/<slug>.svg）。
func isVendorIconAsset(requestPath string) bool {
	rest, ok := strings.CutPrefix(requestPath, "/vendor/lobehub-icons/")
	if !ok {
		return false
	}
	version, file, found := strings.Cut(rest, "/")
	return found && version != "" && file != "" && !strings.Contains(file, "/") &&
		version[0] >= '0' && version[0] <= '9'
}

func isNextExportDataAsset(requestPath string) bool {
	// output: "export" 为每个页面写出同名 .txt（如 /settings/general.txt），并在根目录写出 __next.*.txt。
	return strings.EqualFold(path.Ext(requestPath), ".txt")
}

func readyzHandler(hc HealthChecker, shutdown *lifecycle.Shutdown) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 排空期间立即返回未就绪，让负载均衡停止派发新流量；存量请求继续处理。
		if shutdown.Draining() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "draining"})
			return
		}
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
