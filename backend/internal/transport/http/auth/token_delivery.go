package auth

import (
	"errors"
	"net/http"
	"strings"

	appauth "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// refresh token 的投递方式按客户端平台区分：
//
//   - 浏览器（默认）：HttpOnly cookie。JS 无法读取，XSS 拿不到长期凭据。
//   - 原生客户端（桌面 / 移动）：webview 与服务器跨站，SameSite cookie 不会随请求发送，
//     因此客户端携带 X-Client-Platform: desktop|mobile，refresh token 改为经响应体投递，
//     由客户端存入系统 keychain / SecureStore，刷新时经请求体回传。
//
// 两条路径共用同一套轮换与吊销逻辑（appauth.Service.Refresh），后端不为任何单一客户端开特例。
//
// 安全边界：响应体投递把长期凭据暴露给 JS，因此只对非网页 Origin 开放。
// 浏览器页面即使被 XSS，也无法通过伪造该头拿到 refresh token：isNativeClient 会拒绝
// http(s) Origin（Tauri webview 的 Origin 是 tauri://localhost 或 http://tauri.localhost，
// 原生移动端不发 Origin）。

// ClientPlatformHeader 标识原生客户端平台的请求头。
const ClientPlatformHeader = "X-Client-Platform"

// nativeClientPlatforms 是允许经响应体接收 refresh token 的平台标识。
var nativeClientPlatforms = map[string]struct{}{
	"desktop": {},
	"mobile":  {},
}

// tauriWebviewHost 是 Windows 上 Tauri webview 的 Origin 主机名（其他平台为 tauri:// scheme）。
const tauriWebviewHost = "tauri.localhost"

// isNativeClient 判断请求是否来自声明了原生平台、且 Origin 不是普通网页的客户端。
func isNativeClient(c *gin.Context) bool {
	platform := strings.ToLower(strings.TrimSpace(c.GetHeader(ClientPlatformHeader)))
	if _, ok := nativeClientPlatforms[platform]; !ok {
		return false
	}
	return !isWebOrigin(c.GetHeader("Origin"))
}

// isWebOrigin 判断 Origin 是否是浏览器页面（http/https 且不是 Tauri webview 主机）。
// 空 Origin（原生 HTTP 客户端）和自定义 scheme（tauri://）都不算网页。
func isWebOrigin(origin string) bool {
	origin = strings.ToLower(strings.TrimSpace(origin))
	if origin == "" {
		return false
	}
	var host string
	switch {
	case strings.HasPrefix(origin, "https://"):
		host = strings.TrimPrefix(origin, "https://")
	case strings.HasPrefix(origin, "http://"):
		host = strings.TrimPrefix(origin, "http://")
	default:
		return false
	}
	host, _, _ = strings.Cut(host, "/")
	host, _, _ = strings.Cut(host, ":")
	return host != tauriWebviewHost
}

// RefreshTokenRequest 原生客户端刷新请求体；浏览器客户端不携带请求体。
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// readRefreshToken 按平台读取 refresh token：原生客户端只认请求体，浏览器只认 cookie。
// 不做交叉回退，避免两种投递方式混用导致凭据状态不一致。
func readRefreshToken(c *gin.Context) string {
	if isNativeClient(c) {
		var req RefreshTokenRequest
		if err := bindOptionalJSON(c, &req); err != nil {
			return ""
		}
		return strings.TrimSpace(req.RefreshToken)
	}
	token, err := c.Cookie(refreshTokenCookieName)
	if err != nil {
		return ""
	}
	return token
}

// respondWithSession 写出登录/刷新成功响应，并按平台投递 refresh token。
func (h *Handler) respondWithSession(c *gin.Context, result *appauth.LoginResult) {
	resp := toLoginResponse(result)
	if isNativeClient(c) {
		resp.RefreshToken = result.RefreshToken
	} else {
		h.writeRefreshTokenCookie(c, result)
	}
	response.Success(c, resp)
}

// LocalGrantExchangeRequest 本地模式：桌面壳用启动握手拿到的一次性 grant 换取会话。
type LocalGrantExchangeRequest struct {
	Grant string `json:"grant" binding:"required"`
}

// ExchangeLocalGrant godoc
// @Summary 本地模式：一次性 grant 换取会话
// @Description 仅在服务器以本地 sidecar 模式运行时可用；grant 由启动握手交给桌面壳，只能使用一次
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LocalGrantExchangeRequest true "本地登录 grant"
// @Success 200 {object} LoginResponseDoc
// @Failure 401 {object} ErrorDoc
// @Router /auth/local/exchange [post]
func (h *Handler) ExchangeLocalGrant(c *gin.Context) {
	var req LocalGrantExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.ExchangeLocalGrant(
		c.Request.Context(),
		req.Grant,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		if errors.Is(err, appauth.ErrLocalGrantInvalid) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidLocalGrant)
			return
		}
		response.InternalError(c)
		return
	}
	h.respondWithSession(c, result)
}
