package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appauth "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/gin-gonic/gin"
)

func newTestContext(t *testing.T, method, body string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/api/v1/auth/refresh", reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	c.Request = req
	return c, recorder
}

func TestIsNativeClient(t *testing.T) {
	cases := map[string]bool{
		"":         false,
		"web":      false,
		"browser":  false,
		"desktop":  true,
		"Desktop":  true,
		" mobile ": true,
		"tv":       false,
	}
	for header, want := range cases {
		c, _ := newTestContext(t, http.MethodPost, "", map[string]string{ClientPlatformHeader: header})
		if got := isNativeClient(c); got != want {
			t.Fatalf("isNativeClient(%q) = %v, want %v", header, got, want)
		}
	}
}

// 网页 Origin 即使带了原生平台头也不能拿到响应体里的 refresh token，
// 否则同源 XSS 就能把 HttpOnly cookie 升级为可外传的长期凭据。
func TestIsNativeClientRejectsWebOrigins(t *testing.T) {
	cases := map[string]bool{
		"":                             true, // 原生 HTTP 客户端不发 Origin
		"tauri://localhost":            true, // macOS / Linux webview
		"http://tauri.localhost":       true, // Windows webview
		"HTTP://TAURI.LOCALHOST/":      true,
		"http://localhost:3000":        false, // 开发页面
		"https://chat.example.com":     false, // 生产页面
		"https://tauri.localhost.evil": false, // 主机名必须精确匹配
		"https://evil.tauri.localhost": false,
	}
	for origin, want := range cases {
		c, _ := newTestContext(t, http.MethodPost, "", map[string]string{ClientPlatformHeader: "desktop", "Origin": origin})
		if got := isNativeClient(c); got != want {
			t.Fatalf("isNativeClient(origin=%q) = %v, want %v", origin, got, want)
		}
	}
}

func TestReadRefreshTokenUsesCookieForBrowsers(t *testing.T) {
	c, _ := newTestContext(t, http.MethodPost, `{"refreshToken":"from-body"}`, nil)
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: "from-cookie"})

	if got := readRefreshToken(c); got != "from-cookie" {
		t.Fatalf("browser refresh token = %q, want cookie value", got)
	}
}

func TestReadRefreshTokenUsesBodyForNativeClients(t *testing.T) {
	c, _ := newTestContext(t, http.MethodPost, `{"refreshToken":" from-body "}`, map[string]string{ClientPlatformHeader: "desktop"})
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: "from-cookie"})

	if got := readRefreshToken(c); got != "from-body" {
		t.Fatalf("native refresh token = %q, want trimmed body value", got)
	}
}

func TestReadRefreshTokenNativeClientDoesNotFallBackToCookie(t *testing.T) {
	c, _ := newTestContext(t, http.MethodPost, "", map[string]string{ClientPlatformHeader: "mobile"})
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: "from-cookie"})

	if got := readRefreshToken(c); got != "" {
		t.Fatalf("native client without body token got %q, want empty", got)
	}
}

func sampleLoginResult() *appauth.LoginResult {
	return &appauth.LoginResult{
		AccessToken:      "access",
		RefreshToken:     "refresh-secret",
		SessionID:        "session-1",
		ExpiresAt:        time.Now().Add(time.Hour),
		RefreshExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func decodeLoginResponse(t *testing.T, recorder *httptest.ResponseRecorder) LoginResponse {
	t.Helper()
	var envelope struct {
		Data LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return envelope.Data
}

func TestRespondWithSessionBrowserUsesCookieOnly(t *testing.T) {
	c, recorder := newTestContext(t, http.MethodPost, "", nil)
	h := &Handler{}

	h.respondWithSession(c, sampleLoginResult())

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != refreshTokenCookieName || cookies[0].Value != "refresh-secret" {
		t.Fatalf("expected refresh cookie, got %+v", cookies)
	}
	if !cookies[0].HttpOnly {
		t.Fatalf("refresh cookie must be HttpOnly")
	}
	if body := decodeLoginResponse(t, recorder); body.RefreshToken != "" {
		t.Fatalf("browser response must not expose refresh token in body, got %q", body.RefreshToken)
	}
	if strings.Contains(recorder.Body.String(), "refresh-secret") {
		t.Fatalf("refresh token leaked into browser response body")
	}
}

func TestRespondWithSessionNativeUsesBodyOnly(t *testing.T) {
	c, recorder := newTestContext(t, http.MethodPost, "", map[string]string{ClientPlatformHeader: "desktop"})
	h := &Handler{}

	h.respondWithSession(c, sampleLoginResult())

	if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("native response must not set cookies, got %+v", cookies)
	}
	body := decodeLoginResponse(t, recorder)
	if body.RefreshToken != "refresh-secret" {
		t.Fatalf("native response refreshToken = %q, want body delivery", body.RefreshToken)
	}
	if body.AccessToken != "access" || body.SessionID != "session-1" {
		t.Fatalf("unexpected login payload: %+v", body)
	}
}

func TestRespondWithSessionNativeTwoFactorChallengeHasNoRefreshToken(t *testing.T) {
	c, recorder := newTestContext(t, http.MethodPost, "", map[string]string{ClientPlatformHeader: "desktop"})
	h := &Handler{}

	h.respondWithSession(c, &appauth.LoginResult{TwoFactorRequired: true, TwoFactorChallengeToken: "challenge"})

	body := decodeLoginResponse(t, recorder)
	if body.RefreshToken != "" || !body.TwoFactorRequired {
		t.Fatalf("2FA challenge must not carry a refresh token: %+v", body)
	}
}
