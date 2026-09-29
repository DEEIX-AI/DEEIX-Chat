package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/token"
	adminhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/admin"
	announcementhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/announcement"
	authhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/auth"
	billinghttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/billing"
	contentmoderationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/contentmoderation"
	conversationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/conversation"
	"github.com/gin-gonic/gin"
)

const gateTestSecret = "test-jwt-secret-value"

// 门禁挂在鉴权之后：请求必须先通过认证，才能证明是门禁而不是 401 拦下了它。
func adminRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	accessToken, err := token.GenerateWithClaims(token.GenerateClaimsInput{
		Secret: gateTestSecret, UserID: 1, Username: "owner", Role: domainuser.RoleSuperAdmin,
		SessionID: "s", TokenID: "t", TokenType: "access", TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	request := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	return request
}

// gatedRoutes 是 docs/ARCHITECTURE.md §4 的清单：本地模式下每一条都必须返回 404 feature.disabled。
var gatedRoutes = []struct {
	feature string
	method  string
	path    string
}{
	{"multiUser", "GET", "/admin/users"},
	{"multiUser", "POST", "/admin/users"},
	{"multiUser", "PATCH", "/admin/users/1"},
	{"multiUser", "DELETE", "/admin/users/1"},
	{"multiUser", "POST", "/admin/users/import/openwebui"},
	{"multiUser", "GET", "/admin/user-auth-events"},
	{"multiUser", "GET", "/admin/permission-groups"},
	{"multiUser", "PUT", "/admin/permission-groups/1/users"},
	{"multiUser", "GET", "/admin/models/1/permission-groups"},

	{"registration", "POST", "/auth/register/email/start"},
	{"registration", "POST", "/auth/register/email/complete"},

	{"identityProviders", "POST", "/auth/providers/github/authorize"},
	{"identityProviders", "GET", "/auth/providers/github/callback"},
	{"identityProviders", "POST", "/auth/providers/github/exchange"},
	{"identityProviders", "GET", "/me/identities"},
	{"identityProviders", "DELETE", "/me/identities/1"},
	{"identityProviders", "POST", "/me/identities/providers/acme/authorize"},
	{"identityProviders", "POST", "/me/identities/providers/acme/exchange"},
	{"identityProviders", "GET", "/admin/auth/providers"},
	{"identityProviders", "POST", "/admin/auth/providers"},

	{"accountSecurity", "POST", "/auth/password/reset/start"},
	{"accountSecurity", "POST", "/auth/password/change/start"},
	{"accountSecurity", "POST", "/me/email/change/start-new"},
	{"accountSecurity", "GET", "/me/2fa"},
	{"accountSecurity", "POST", "/me/delete/start"},
	{"accountSecurity", "DELETE", "/me"},
	{"accountSecurity", "GET", "/auth/sessions"},
	{"accountSecurity", "POST", "/auth/logout-all"},

	{"announcements", "POST", "/announcements/1/close"},
	{"announcements", "POST", "/announcements/1/dismiss-today"},
	{"announcements", "GET", "/admin/announcements"},
	{"announcements", "POST", "/admin/announcements"},

	{"billingGating", "GET", "/billing/plans"},
	{"billingGating", "POST", "/billing/subscriptions"},
	{"billingGating", "POST", "/billing/payments/checkout"},
	{"billingGating", "POST", "/billing/redemptions"},
	{"billingGating", "POST", "/billing/payments/stripe/webhook"},
	{"billingGating", "GET", "/admin/billing/plans"},
	{"billingGating", "PATCH", "/admin/billing/accounts/1/balance"},
	{"billingGating", "GET", "/admin/billing/redemption-codes"},
	{"billingGating", "GET", "/admin/payment-orders"},
	{"billingGating", "GET", "/admin/redemptions"},

	{"contentModeration", "GET", "/admin/content-moderation/config"},
	{"contentModeration", "GET", "/admin/content-moderation/events"},

	{"sharing", "GET", "/conversations/1/share"},
	{"sharing", "POST", "/conversations/1/share"},
	{"sharing", "POST", "/conversations/shares/revoke"},
	{"sharing", "POST", "/shared-conversations/abc/clone"},
	{"sharing", "GET", "/shared-conversations/abc"},
}

// openRoutes 在本地模式下也必须可达：门禁不能误伤。它们没有数据库，所以只断言
// 请求走到了鉴权（401）或处理器，而不是被 feature.disabled 拦下。
var openRoutes = []struct{ method, path string }{
	{"GET", "/auth/login-options"},
	{"POST", "/auth/login"},
	{"POST", "/auth/refresh"},
	{"GET", "/me"},
	{"PATCH", "/me"},
	{"POST", "/auth/logout"},
	{"GET", "/announcements"},
	{"GET", "/billing/config"},
	{"GET", "/billing/usage"},
	{"GET", "/billing/overview"},
	{"GET", "/admin/billing/model-prices"},
	{"GET", "/admin/usage-statistics"},
	{"GET", "/admin/audit-logs"},
	{"GET", "/admin/call-logs"},
	{"GET", "/conversations"},
}

func gatedEngine(t *testing.T, cfg config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// 放行到零值处理器的请求会 panic 并被 recovery 记录；那正是"门禁没拦"的证据，日志无需输出。
	previous := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = io.Discard
	t.Cleanup(func() { gin.DefaultErrorWriter = previous })
	cfg.AppName = "test"
	cfg.JWTSecret = gateTestSecret
	// 零值处理器只用于注册路由；被门禁拦下或被鉴权拦下的请求永远不会调用它们。
	engine, err := NewEngine(config.NewRuntime(cfg), nil, Modules{
		Auth:              &authhttp.Module{Handler: &authhttp.Handler{}},
		Admin:             &adminhttp.Module{Handler: &adminhttp.Handler{}},
		Billing:           &billinghttp.Module{Handler: &billinghttp.Handler{}},
		Announcement:      &announcementhttp.Module{Handler: &announcementhttp.Handler{}},
		ContentModeration: &contentmoderationhttp.Module{Handler: &contentmoderationhttp.Handler{}},
		Conversation:      &conversationhttp.Module{Handler: &conversationhttp.Handler{}},
	}, nil, nil)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	return engine
}

func TestLocalModeRejectsGatedRoutes(t *testing.T) {
	engine := gatedEngine(t, config.Config{LocalMode: true})
	for _, route := range gatedRoutes {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, adminRequest(t, route.method, route.path))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s %s: expected 404, got %d", route.method, route.path, recorder.Code)
			continue
		}
		var body struct {
			ErrorCode string `json:"errorCode"`
			Details   struct {
				Feature string `json:"feature"`
			} `json:"details"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Errorf("%s %s: decode: %v", route.method, route.path, err)
			continue
		}
		if body.ErrorCode != "feature.disabled" || body.Details.Feature != route.feature {
			t.Errorf("%s %s: got %s/%s, want feature.disabled/%s", route.method, route.path, body.ErrorCode, body.Details.Feature, route.feature)
		}
	}
}

func TestLocalModeKeepsOpenRoutes(t *testing.T) {
	engine := gatedEngine(t, config.Config{LocalMode: true})
	for _, route := range openRoutes {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, adminRequest(t, route.method, route.path))
		if strings.Contains(recorder.Body.String(), "feature.disabled") {
			t.Errorf("%s %s must stay available in local mode", route.method, route.path)
		}
	}
}

func TestServerModeNeverReturnsFeatureDisabled(t *testing.T) {
	engine := gatedEngine(t, config.Config{})
	for _, route := range gatedRoutes {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, adminRequest(t, route.method, route.path))
		if strings.Contains(recorder.Body.String(), "feature.disabled") {
			t.Errorf("%s %s: server mode must not report feature.disabled", route.method, route.path)
		}
	}
}
