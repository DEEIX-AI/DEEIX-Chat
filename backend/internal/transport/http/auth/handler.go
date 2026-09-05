package auth

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	appauth "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const refreshTokenCookieName = "deeix_chat_refresh_token"

// Handler 封装认证 HTTP 处理。
type Handler struct {
	service *appauth.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appauth.Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) recordAudit(c *gin.Context, userID uint, action string, resource string, resourceID string, detail any) {
	h.service.RecordAudit(c.Request.Context(), appauth.AuditInput{
		ActorUserID: userID,
		RequestID:   middleware.MustRequestID(c),
		Action:      action,
		Resource:    resource,
		ResourceID:  resourceID,
		IP:          c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
		Detail:      detail,
	})
}

func bindOptionalJSON(c *gin.Context, req any) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(req); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (h *Handler) writeRefreshTokenCookie(c *gin.Context, result *appauth.LoginResult) {
	if result == nil || result.RefreshToken == "" || result.RefreshExpiresAt.IsZero() {
		return
	}
	maxAge := int(time.Until(result.RefreshExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth",
		Expires:  result.RefreshExpiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.shouldUseSecureCookie(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearRefreshTokenCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.shouldUseSecureCookie(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func isHTTPSRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return c.GetHeader("X-Forwarded-Proto") == "https"
}

func (h *Handler) shouldUseSecureCookie(c *gin.Context) bool {
	return isHTTPSRequest(c) || (h.service != nil && h.service.ShouldUseSecureCookies())
}

// LoginOptions godoc
// @Summary 获取登录入口配置
// @Description 获取用户名、邮箱、OAuth/OIDC 登录入口，以及邮箱注册 Turnstile 公共配置
// @Tags auth
// @Produce json
// @Success 200 {object} LoginOptionsResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/login-options [get]
func (h *Handler) LoginOptions(c *gin.Context) {
	result, err := h.service.GetLoginOptions(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, toLoginOptionsResponse(result))
}

// StartEmailRegistration godoc
// @Summary 发送邮箱注册验证码
// @Description 邮箱验证码注册开启时发送验证码；启用 Turnstile 后需要提交 turnstileToken
// @Tags auth
// @Accept json
// @Produce json
// @Param body body EmailRegistrationStartRequest true "邮箱注册验证码请求"
// @Success 200 {object} EmailRegistrationStartResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /auth/register/email/start [post]
func (h *Handler) StartEmailRegistration(c *gin.Context) {
	var req EmailRegistrationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestEmailRegistration(
		c.Request.Context(),
		req.Email,
		req.TurnstileToken,
		c.ClientIP(),
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailRegistrationStartResponse(result))
}

// CompleteEmailRegistration godoc
// @Summary 完成邮箱注册
// @Description 使用邮箱、密码和验证码完成注册；未开启邮箱验证码但启用 Turnstile 时需要提交 turnstileToken
// @Tags auth
// @Accept json
// @Produce json
// @Param body body EmailRegistrationCompleteRequest true "邮箱注册完成请求"
// @Success 200 {object} LoginResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /auth/register/email/complete [post]
func (h *Handler) CompleteEmailRegistration(c *gin.Context) {
	var req EmailRegistrationCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RegisterWithEmail(
		c.Request.Context(),
		appauth.RegisterWithEmailInput{
			Email:          req.Email,
			Password:       req.Password,
			Code:           req.Code,
			TurnstileToken: req.TurnstileToken,
			RemoteIP:       c.ClientIP(),
			RequestID:      middleware.MustRequestID(c),
			AuditContext:   middleware.ResolveSessionAuditContext(c),
		},
	)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	h.writeRefreshTokenCookie(c, result)
	response.Success(c, toLoginResponse(result))
}

// StartPasswordReset godoc
// @Summary 发送密码重置验证码
// @Description SMTP 配置可用时，向已验证邮箱发送密码重置验证码；失败时返回通用错误，避免暴露账号状态
// @Tags auth
// @Accept json
// @Produce json
// @Param body body PasswordResetStartRequest true "密码重置验证码请求"
// @Success 200 {object} PasswordResetStartResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Router /auth/password/reset/start [post]
func (h *Handler) StartPasswordReset(c *gin.Context) {
	var req PasswordResetStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestPasswordReset(
		c.Request.Context(),
		req.Email,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		if errors.Is(err, appauth.ErrPasswordResetFailed) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.ErrorFrom(c, http.StatusInternalServerError, errPasswordResetFailed)
		return
	}
	response.Success(c, toPasswordResetStartResponse(result))
}

// CompletePasswordReset godoc
// @Summary 完成密码重置
// @Description 使用邮箱、验证码和新密码完成密码重置；失败时返回通用错误，避免暴露账号状态
// @Tags auth
// @Accept json
// @Produce json
// @Param body body PasswordResetCompleteRequest true "密码重置完成请求"
// @Success 200 {object} PasswordResetCompleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Router /auth/password/reset/complete [post]
func (h *Handler) CompletePasswordReset(c *gin.Context) {
	var req PasswordResetCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.CompletePasswordReset(
		c.Request.Context(),
		req.Email,
		req.Code,
		req.NewPassword,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	); err != nil {
		if errors.Is(err, appauth.ErrPasswordResetFailed) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, PasswordResetCompleteResponse{Changed: true})
}

func (h *Handler) StartPasswordChangeVerification(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req SecurityVerificationStartRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestPasswordChangeVerification(
		c.Request.Context(),
		userID,
		req.VerificationMethod,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toPasswordChangeVerificationStartResponse(result))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	err := h.service.ChangePassword(
		c.Request.Context(),
		appauth.ChangePasswordInput{
			UserID:             userID,
			CurrentPassword:    req.CurrentPassword,
			NewPassword:        req.NewPassword,
			VerificationMethod: req.VerificationMethod,
			Code:               req.Code,
			RequestID:          middleware.MustRequestID(c),
			AuditContext:       middleware.ResolveSessionAuditContext(c),
		},
	)
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidCurrentPassword)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	h.clearRefreshTokenCookie(c)
	response.Success(c, ChangePasswordResponse{Changed: true})
}

func (h *Handler) StartEmailBootstrap(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req EmailVerificationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestEmailBootstrapVerification(c.Request.Context(), userID, req.Email, middleware.MustRequestID(c), middleware.ResolveSessionAuditContext(c))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailVerificationStartResponse(result))
}

func (h *Handler) CompleteEmailBootstrap(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req EmailBootstrapCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CompleteEmailBootstrap(c.Request.Context(), userID, req.Email, req.Code, middleware.MustRequestID(c), middleware.ResolveSessionAuditContext(c))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, MeResponse{User: toUserResponse(view)})
}

func (h *Handler) StartCurrentEmailVerification(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	result, err := h.service.RequestCurrentEmailVerification(c.Request.Context(), userID, middleware.MustRequestID(c), middleware.ResolveSessionAuditContext(c))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailVerificationStartResponse(result))
}

func (h *Handler) CompleteCurrentEmailVerification(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req EmailVerificationCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CompleteCurrentEmailVerification(c.Request.Context(), userID, req.Code, middleware.MustRequestID(c), middleware.ResolveSessionAuditContext(c))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, MeResponse{User: toUserResponse(view)})
}

func (h *Handler) StartCurrentEmailChange(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req SecurityVerificationStartRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestCurrentEmailChangeVerification(c.Request.Context(), userID, req.VerificationMethod, middleware.MustRequestID(c), middleware.ResolveSessionAuditContext(c))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailVerificationStartResponse(result))
}

func (h *Handler) StartNewEmailChange(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req EmailVerificationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestNewEmailChangeVerification(c.Request.Context(), userID, req.Email, middleware.MustRequestID(c), middleware.ResolveSessionAuditContext(c))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailVerificationStartResponse(result))
}

func (h *Handler) CompleteEmailChange(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req EmailChangeCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CompleteEmailChange(c.Request.Context(), appauth.CompleteEmailChangeInput{
		UserID:                    userID,
		NewEmail:                  req.Email,
		CurrentVerificationMethod: req.CurrentVerificationMethod,
		CurrentCode:               req.CurrentCode,
		NewCode:                   req.NewCode,
		RequestID:                 middleware.MustRequestID(c),
		AuditContext:              middleware.ResolveSessionAuditContext(c),
	})
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidCurrentPassword)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, MeResponse{User: toUserResponse(view)})
}

func (h *Handler) ListCurrentUserIdentities(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	items, err := h.service.ListCurrentUserIdentities(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, UserIdentityListResponse{Results: toUserIdentityResponses(items)})
}

func (h *Handler) DeleteCurrentUserIdentity(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	rawID := c.Param("identity_id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidIdentityID)
		return
	}
	if err = h.service.UnlinkCurrentUserIdentity(c.Request.Context(), userID, uint(parsedID)); err != nil {
		if errors.Is(err, appauth.ErrIdentityNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, appauth.ErrLastLoginMethodNotAllowed) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, DeleteUserIdentityResponse{Deleted: true})
}

func (h *Handler) CompleteProviderBind(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req CompleteProviderBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	identity, err := h.service.CompleteProviderBind(
		c.Request.Context(),
		appauth.CompleteProviderBindInput{
			UserID:       userID,
			Slug:         c.Param("slug"),
			Code:         req.Code,
			State:        req.State,
			RedirectURI:  req.RedirectURI,
			CodeVerifier: req.CodeVerifier,
			RequestID:    middleware.MustRequestID(c),
			AuditContext: middleware.ResolveSessionAuditContext(c),
		},
	)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, UserIdentityResponseData{Identity: toUserIdentityResponse(*identity)})
}

func (h *Handler) StartProviderLogin(c *gin.Context) {
	slug := c.Param("slug")
	callback := c.Query("redirect_uri")
	target, err := h.service.BuildProviderAuthURL(c.Request.Context(), slug, callback, c.Query("next"), c.Query("code_challenge"), c.Query("intent"))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	c.Redirect(http.StatusFound, target)
}

// StartProviderAuthBridge godoc
// @Summary 创建第三方登录授权桥事务
// @Description 为 Web、App 或桌面公共客户端创建 PKCE 保护的 OAuth 授权事务；外部身份源仅回调当前 DEEIX 实例
// @Tags auth
// @Accept json
// @Produce json
// @Param slug path string true "身份源 slug"
// @Param body body ProviderAuthBridgeStartRequest true "授权桥参数"
// @Success 200 {object} ProviderAuthBridgeStartResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /auth/providers/{slug}/authorize [post]
func (h *Handler) StartProviderAuthBridge(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req ProviderAuthBridgeStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.StartProviderAuthBridge(c.Request.Context(), c.Param("slug"), appauth.ProviderAuthBridgeStartInput{
		ClientID:      req.ClientID,
		RedirectURI:   req.RedirectURI,
		CodeChallenge: req.CodeChallenge,
		ClientState:   req.ClientState,
		Intent:        req.Intent,
		Next:          req.Next,
	})
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, ProviderAuthBridgeStartResponse{
		AuthorizationURL: result.AuthorizationURL,
		ExpiresAt:        result.ExpiresAt,
	})
}

func (h *Handler) ProviderCallback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, err := h.service.CompleteProviderAuthBridgeCallback(c.Request.Context(), c.Param("slug"), appauth.ProviderAuthBridgeCallbackInput{
		Code:          c.Query("code"),
		State:         c.Query("state"),
		ProviderError: c.Query("error"),
	})
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	c.Redirect(http.StatusFound, result.RedirectURI)
}

// ExchangeProviderAuthBridgeGrant godoc
// @Summary 兑换第三方登录一次性授权码
// @Description 使用客户端 PKCE verifier 原子兑换服务端回调签发的一次性授权码，并进入统一 2FA/会话流程
// @Tags auth
// @Accept json
// @Produce json
// @Param slug path string true "身份源 slug"
// @Param body body ProviderAuthBridgeExchangeRequest true "授权码兑换参数"
// @Success 200 {object} LoginResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Router /auth/providers/{slug}/exchange [post]
func (h *Handler) ExchangeProviderAuthBridgeGrant(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req ProviderAuthBridgeExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.ExchangeProviderAuthBridgeGrant(
		c.Request.Context(),
		c.Param("slug"),
		appauth.ProviderAuthBridgeExchangeInput{
			ClientID:     req.ClientID,
			Grant:        req.Grant,
			CodeVerifier: req.CodeVerifier,
		},
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		var emailConflictErr *appauth.ProviderEmailConflictError
		if errors.As(err, &emailConflictErr) {
			response.ErrorWithDetails(
				c,
				http.StatusConflict,
				"auth.provider_email_conflict",
				gin.H{
					"providerSlug": emailConflictErr.ProviderSlug,
					"email":        emailConflictErr.Email,
					"action":       emailConflictErr.Action,
				},
			)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	h.writeRefreshTokenCookie(c, result)
	response.Success(c, toLoginResponse(result))
}

func (h *Handler) CompleteProviderLogin(c *gin.Context) {
	var req CompleteProviderLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.CompleteProviderLogin(
		c.Request.Context(),
		appauth.CompleteProviderLoginInput{
			Slug:         c.Param("slug"),
			Code:         req.Code,
			State:        req.State,
			RedirectURI:  req.RedirectURI,
			CodeVerifier: req.CodeVerifier,
			Intent:       req.Intent,
			RequestID:    middleware.MustRequestID(c),
			AuditContext: middleware.ResolveSessionAuditContext(c),
		},
	)
	if err != nil {
		var emailConflictErr *appauth.ProviderEmailConflictError
		if errors.As(err, &emailConflictErr) {
			response.ErrorWithDetails(
				c,
				http.StatusConflict,
				"auth.provider_email_conflict",
				gin.H{
					"providerSlug": emailConflictErr.ProviderSlug,
					"email":        emailConflictErr.Email,
					"action":       emailConflictErr.Action,
				},
			)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	h.writeRefreshTokenCookie(c, result)
	response.Success(c, toLoginResponse(result))
}

// Login godoc
// @Summary 用户登录
// @Description 登录后返回JWT访问令牌
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "登录参数"
// @Success 200 {object} LoginResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Router /auth/login [post]
// Login 登录。
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	auditCtx := middleware.ResolveSessionAuditContext(c)
	result, err := h.service.Login(
		c.Request.Context(),
		req.Username,
		req.Password,
		middleware.MustRequestID(c),
		auditCtx,
	)
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, err)
			return
		}
		if errors.Is(err, appauth.ErrAccountLocked) {
			response.ErrorFrom(c, http.StatusUnauthorized, err)
			return
		}
		response.InternalError(c)
		return
	}

	if !result.TwoFactorRequired {
		h.service.RecordAudit(c.Request.Context(), appauth.AuditInput{
			ActorUserID: result.User.ID,
			RequestID:   middleware.MustRequestID(c),
			Action:      "login",
			Resource:    "user",
			ResourceID:  req.Username,
			IP:          auditCtx.ClientIP,
			UserAgent:   auditCtx.UserAgent,
			Detail:      map[string]string{"event": "user_login"},
		})
	}

	h.writeRefreshTokenCookie(c, result)
	response.Success(c, toLoginResponse(result))
}

func (h *Handler) VerifyTwoFactorLogin(c *gin.Context) {
	var req TwoFactorVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	auditCtx := middleware.ResolveSessionAuditContext(c)
	result, err := h.service.VerifyLoginTwoFactor(
		c.Request.Context(),
		req.ChallengeToken,
		req.VerificationMethod,
		req.Code,
		middleware.MustRequestID(c),
		auditCtx,
	)
	if err != nil {
		if errors.Is(err, appauth.ErrTwoFactorChallengeExpired) {
			response.ErrorFrom(c, http.StatusUnauthorized, err)
			return
		}
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidTwoFactorCode)
			return
		}
		response.InternalError(c)
		return
	}
	h.writeRefreshTokenCookie(c, result)
	response.Success(c, toLoginResponse(result))
}

func (h *Handler) StartTwoFactorEmailVerification(c *gin.Context) {
	var req TwoFactorEmailStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestLoginEmailVerification(
		c.Request.Context(),
		req.ChallengeToken,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		if errors.Is(err, appauth.ErrTwoFactorChallengeExpired) {
			response.ErrorFrom(c, http.StatusUnauthorized, err)
			return
		}
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidTwoFactorChallenge)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailVerificationStartResponse(result))
}

func (h *Handler) CurrentTwoFactorStatus(c *gin.Context) {
	userID := middleware.MustUserID(c)
	result, err := h.service.GetCurrentTwoFactorStatus(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, toTwoFactorStatusResponse(result))
}

func (h *Handler) StartCurrentTwoFactorSetup(c *gin.Context) {
	userID := middleware.MustUserID(c)
	result, err := h.service.StartCurrentTwoFactorSetup(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, TwoFactorSetupStartResponse{Secret: result.Secret, OTPAuthURL: result.OTPAuthURL, ExpiresAt: result.ExpiresAt})
}

func (h *Handler) ConfirmCurrentTwoFactorSetup(c *gin.Context) {
	var req TwoFactorCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	userID := middleware.MustUserID(c)
	result, err := h.service.ConfirmCurrentTwoFactorSetup(c.Request.Context(), userID, req.Code)
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidTwoFactorCode)
			return
		}
		if errors.Is(err, appauth.ErrTwoFactorSetupExpired) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, appauth.ErrTwoFactorSetupNotStarted) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, appauth.ErrTwoFactorSetupNotPersisted) {
			response.InternalError(c)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, TwoFactorRecoveryCodesResponse{
		RecoveryCodes: result.RecoveryCodes,
		Status:        toTwoFactorStatusResponse(&result.Status),
	})
}

func (h *Handler) CancelCurrentTwoFactorSetup(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if err := h.service.CancelCurrentTwoFactorSetup(c.Request.Context(), userID); err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, TwoFactorSetupCancelResponse{Canceled: true})
}

func (h *Handler) DisableCurrentTwoFactor(c *gin.Context) {
	var req TwoFactorCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	userID := middleware.MustUserID(c)
	if err := h.service.DisableCurrentTwoFactor(c.Request.Context(), userID, req.Code); err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidTwoFactorCode)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, TwoFactorDisableResponse{Disabled: true})
}

func (h *Handler) RegenerateCurrentTwoFactorRecoveryCodes(c *gin.Context) {
	var req TwoFactorCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	userID := middleware.MustUserID(c)
	result, err := h.service.RegenerateCurrentTwoFactorRecoveryCodes(c.Request.Context(), userID, req.Code)
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidTwoFactorCode)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, TwoFactorRecoveryCodesResponse{
		RecoveryCodes: result.RecoveryCodes,
		Status:        toTwoFactorStatusResponse(&result.Status),
	})
}

// ListIdentityProviders godoc
// @Summary 获取第三方身份源列表
// @Description 管理员查看已配置的 OIDC 和 OAuth2 身份源
// @Tags admin-auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} IdentityProviderListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/auth/providers [get]
func (h *Handler) ListIdentityProviders(c *gin.Context) {
	items, err := h.service.ListIdentityProviders(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, IdentityProviderListResponse{Results: toIdentityProviderResponses(items), Total: len(items)})
}

// CreateIdentityProvider godoc
// @Summary 创建第三方身份源
// @Description 管理员创建一个 OIDC 或 OAuth2 身份源
// @Tags admin-auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpsertIdentityProviderRequest true "身份源配置"
// @Success 200 {object} IdentityProviderResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Router /admin/auth/providers [post]
func (h *Handler) CreateIdentityProvider(c *gin.Context) {
	var req UpsertIdentityProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateIdentityProvider(c.Request.Context(), toUpsertIdentityProviderInput(req, middleware.MustUserRole(c)))
	if err != nil {
		if errors.Is(err, appauth.ErrIdentityProviderSuperAdminDefaultRoleNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toIdentityProviderResponse(*item))
}

// UpdateIdentityProvider godoc
// @Summary 更新第三方身份源
// @Description 管理员更新一个 OIDC 或 OAuth2 身份源
// @Tags admin-auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider_id path string true "身份源 ID"
// @Param body body UpsertIdentityProviderRequest true "身份源配置"
// @Success 200 {object} IdentityProviderResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Router /admin/auth/providers/{provider_id} [patch]
func (h *Handler) UpdateIdentityProvider(c *gin.Context) {
	var req UpsertIdentityProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateIdentityProvider(c.Request.Context(), c.Param("provider_id"), toUpsertIdentityProviderInput(req, middleware.MustUserRole(c)))
	if err != nil {
		if errors.Is(err, appauth.ErrIdentityProviderSuperAdminDefaultRoleNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toIdentityProviderResponse(*item))
}

// ReorderIdentityProviders godoc
// @Summary 调整第三方身份源顺序
// @Description 管理员保存第三方身份源的展示顺序
// @Tags admin-auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ReorderIdentityProvidersRequest true "身份源顺序"
// @Success 200 {object} IdentityProviderReorderResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/auth/provider-order [patch]
func (h *Handler) ReorderIdentityProviders(c *gin.Context) {
	var req ReorderIdentityProvidersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.ReorderIdentityProviders(c.Request.Context(), req.ProviderIDs); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, IdentityProviderReorderResponse{Updated: true})
}

// DeleteIdentityProvider godoc
// @Summary 删除第三方身份源
// @Description 管理员删除第三方身份源；force=true 时允许删除仍有关联用户的身份源
// @Tags admin-auth
// @Produce json
// @Security BearerAuth
// @Param provider_id path string true "身份源 ID"
// @Param force query bool false "是否强制删除"
// @Success 200 {object} IdentityProviderDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Router /admin/auth/providers/{provider_id} [delete]
func (h *Handler) DeleteIdentityProvider(c *gin.Context) {
	force := c.Query("force") == "true"
	if err := h.service.DeleteIdentityProvider(c.Request.Context(), c.Param("provider_id"), force); err != nil {
		var dependentErr *appauth.IdentityProviderDeleteConflictError
		if errors.As(err, &dependentErr) {
			response.ErrorWithDetails(
				c,
				http.StatusConflict,
				"identity_provider.delete_conflict",
				gin.H{"dependentUsers": dependentErr.DependentUsers},
			)
			return
		}
		if errors.Is(err, appauth.ErrIdentityProviderDeleteConflict) {
			response.ErrorFrom(c, http.StatusConflict, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, IdentityProviderDeleteResponse{Deleted: true})
}

// RefreshToken godoc
// @Summary 刷新访问令牌
// @Description 使用 HttpOnly refresh cookie 轮换并签发新的 access token
// @Tags auth
// @Produce json
// @Success 200 {object} RefreshTokenResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/refresh [post]
func (h *Handler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshTokenCookieName)
	if err != nil || refreshToken == "" {
		h.clearRefreshTokenCookie(c)
		response.ErrorFrom(c, http.StatusUnauthorized, errInvalidRefreshToken)
		return
	}

	auditCtx := middleware.ResolveSessionAuditContext(c)
	result, err := h.service.Refresh(
		c.Request.Context(),
		refreshToken,
		middleware.MustRequestID(c),
		auditCtx,
	)
	if err != nil {
		h.clearRefreshTokenCookie(c)
		if errors.Is(err, appauth.ErrInvalidRefreshToken) || errors.Is(err, appauth.ErrSessionRevoked) {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidRefreshToken)
			return
		}
		response.InternalError(c)
		return
	}

	h.writeRefreshTokenCookie(c, result)
	response.Success(c, toLoginResponse(result))
}

// Me godoc
// @Summary 当前用户信息
// @Description 查询当前登录用户资料
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MeResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me [get]
// Me 获取当前用户资料。
func (h *Handler) Me(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	item, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c)
		return
	}
	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}

	response.Success(c, MeResponse{User: toUserResponse(view)})
}

// CurrentSessions godoc
// @Summary 当前活跃会话
// @Description 查询当前登录用户仍然有效的活跃会话列表
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ActiveSessionListResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/sessions [get]
func (h *Handler) CurrentSessions(c *gin.Context) {
	userID := middleware.MustUserID(c)
	sessionID := middleware.MustSessionID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	results, err := h.service.ListCurrentActiveSessions(c.Request.Context(), userID, sessionID)
	if err != nil {
		response.InternalError(c)
		return
	}

	listData := &appauth.ActiveSessionListResult{
		Total:   int64(len(results)),
		Results: results,
	}
	response.Success(c, toActiveSessionListResponse(listData))
}

// UpdateCurrentSessionLocation godoc
// @Summary 更新当前会话精确位置
// @Description 用户授权后，用浏览器定位能力补充当前登录会话的精确位置
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateCurrentSessionLocationRequest true "精确位置参数"
// @Success 200 {object} UpdateCurrentSessionLocationResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/sessions/current/location [put]
func (h *Handler) UpdateCurrentSessionLocation(c *gin.Context) {
	userID := middleware.MustUserID(c)
	sessionID := middleware.MustSessionID(c)
	if userID == 0 || sessionID == "" {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	var req UpdateCurrentSessionLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateCurrentSessionLocation(
		c.Request.Context(),
		userID,
		sessionID,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
		toUpdateCurrentSessionLocationInput(req),
	)
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidLocation) || errors.Is(err, appauth.ErrInvalidTimeZone) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, appauth.ErrSessionRevoked) {
			response.ErrorFrom(c, http.StatusUnauthorized, err)
			return
		}
		response.InternalError(c)
		return
	}

	response.Success(c, toActiveSessionResponse(*item))
}

// PatchMe godoc
// @Summary 更新当前用户资料
// @Description 更新当前登录用户的头像、昵称、时区、对话偏好
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body PatchMeRequest true "用户资料更新参数"
// @Success 200 {object} PatchMeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me [patch]
func (h *Handler) PatchMe(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	var req PatchMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateProfile(c.Request.Context(), userID, toUpdateProfileInput(req))
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidTimeZone) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, appauth.ErrInvalidLocale) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, appauth.ErrInvalidAvatarURL) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, user.ErrInvalidDisplayName) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, appauth.ErrInvalidAppearancePreferences) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.InternalError(c)
		return
	}

	updatedFields := make([]string, 0, 5)
	if req.AvatarURL != nil {
		updatedFields = append(updatedFields, "avatar_url")
	}
	if req.DisplayName != nil {
		updatedFields = append(updatedFields, "display_name")
	}
	if req.Timezone != nil {
		updatedFields = append(updatedFields, "timezone")
	}
	if req.Locale != nil {
		updatedFields = append(updatedFields, "locale")
	}
	if req.ProfilePreferences != nil {
		updatedFields = append(updatedFields, "profile_preferences")
	}
	if req.AppearancePreferences != nil {
		updatedFields = append(updatedFields, "appearance_preferences")
	}
	h.recordAudit(
		c,
		userID,
		"update_profile",
		"user",
		strconv.FormatUint(uint64(userID), 10),
		map[string]any{"fields": updatedFields},
	)

	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}

	response.Success(c, MeResponse{User: toUserResponse(view)})
}

// PatchUsername godoc
// @Summary 修改当前用户用户名
// @Description 当前用户仅可自主修改一次登录用户名
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body PatchUsernameRequest true "用户名更新参数"
// @Success 200 {object} PatchMeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me/username [patch]
func (h *Handler) PatchUsername(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	var req PatchUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateUsernameOnce(c.Request.Context(), userID, toUpdateUsernameInput(req))
	if err != nil {
		switch {
		case errors.Is(err, appauth.ErrUsernameChangeRequired):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appauth.ErrInvalidUsername):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appauth.ErrUsernameTaken):
			response.ErrorFrom(c, http.StatusConflict, err)
		case errors.Is(err, appauth.ErrUsernameChangeUsed):
			response.ErrorFrom(c, http.StatusConflict, err)
		default:
			response.InternalError(c)
		}
		return
	}

	h.recordAudit(
		c,
		userID,
		"update_username",
		"user",
		strconv.FormatUint(uint64(userID), 10),
		map[string]any{"username": item.Username},
	)

	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}

	response.Success(c, MeResponse{User: toUserResponse(view)})
}

// CompleteOnboarding godoc
// @Summary 完成首次引导
// @Description 标记当前用户已完成首次引导
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PatchMeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me/onboarding/complete [post]
func (h *Handler) CompleteOnboarding(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	var req CompleteOnboardingRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.InvalidRequestBody(c, err)
			return
		}
	}

	item, passwordChanged, err := h.service.CompleteOnboarding(
		c.Request.Context(),
		userID,
		req.NewPassword,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.InternalError(c)
		return
	}
	if passwordChanged {
		h.clearRefreshTokenCookie(c)
	}

	response.Success(c, MeResponse{User: toUserResponse(view)})
}

// StartAccountDeleteVerification godoc
// @Summary 开始删除账号验证
// @Description 发送删除当前账号前所需的邮箱验证码，或返回可用的两步验证方式
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body SecurityVerificationStartRequest false "验证方式"
// @Success 200 {object} EmailVerificationStartResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me/delete/start [post]
func (h *Handler) StartAccountDeleteVerification(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req SecurityVerificationStartRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.RequestAccountDeleteVerification(
		c.Request.Context(),
		userID,
		req.VerificationMethod,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	)
	if err != nil {
		if errors.Is(err, appauth.ErrDeleteSuperAdminNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, toEmailVerificationStartResponse(result))
}

// DeleteMe godoc
// @Summary 删除当前用户账户
// @Description 删除当前登录用户账户及主要用户域数据
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body DeleteAccountRequest true "删除账号验证"
// @Success 200 {object} DeleteAccountResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me [delete]
func (h *Handler) DeleteMe(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}
	var req DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	if err := h.service.DeleteAccount(
		c.Request.Context(),
		userID,
		req.VerificationMethod,
		req.Code,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	); err != nil {
		if errors.Is(err, appauth.ErrDeleteSuperAdminNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, appauth.ErrAccountDeleteVerificationRequired) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, domainknowledgebase.ErrBuiltinFileOwnerDeleteBlocked) {
			response.ErrorWithCode(c, http.StatusConflict, "knowledge_base.owner_file_reference")
			return
		}
		if errors.Is(err, appauth.ErrSecurityVerificationMethodUnavailable) ||
			errors.Is(err, appauth.ErrSecurityVerificationEmailInvalid) ||
			errors.Is(err, appauth.ErrSecurityVerificationCodeInvalid) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.InternalError(c)
		return
	}

	h.recordAudit(
		c,
		userID,
		"delete_account",
		"user",
		strconv.FormatUint(uint64(userID), 10),
		map[string]bool{"deleted": true},
	)

	h.clearRefreshTokenCookie(c)
	response.Success(c, DeleteAccountResponse{Deleted: true})
}

// Logout godoc
// @Summary 登出当前会话
// @Description 吊销当前 access token 对应会话
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} LogoutResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	userID := middleware.MustUserID(c)
	sessionID := middleware.MustSessionID(c)
	if userID == 0 || sessionID == "" {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	if err := h.service.Logout(
		c.Request.Context(),
		userID,
		sessionID,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	); err != nil {
		response.InternalError(c)
		return
	}

	h.clearRefreshTokenCookie(c)
	response.Success(c, LogoutResponse{Revoked: true})
}

// LogoutAll godoc
// @Summary 登出全部会话
// @Description 吊销当前用户所有活跃会话
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} LogoutResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/logout-all [post]
func (h *Handler) LogoutAll(c *gin.Context) {
	userID := middleware.MustUserID(c)
	if userID == 0 {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	if err := h.service.LogoutAll(
		c.Request.Context(),
		userID,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	); err != nil {
		response.InternalError(c)
		return
	}

	h.clearRefreshTokenCookie(c)
	response.Success(c, LogoutResponse{Revoked: true})
}

// LogoutSession godoc
// @Summary 登出指定会话
// @Description 吊销当前用户指定 session_id 对应的活跃会话
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "会话ID"
// @Success 200 {object} LogoutResponseDoc
// @Failure 401 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /auth/sessions/{session_id}/logout [post]
func (h *Handler) LogoutSession(c *gin.Context) {
	userID := middleware.MustUserID(c)
	targetSessionID := c.Param("session_id")
	if userID == 0 || targetSessionID == "" {
		response.ErrorFrom(c, http.StatusUnauthorized, errUnauthorized)
		return
	}

	if err := h.service.Logout(
		c.Request.Context(),
		userID,
		targetSessionID,
		middleware.MustRequestID(c),
		middleware.ResolveSessionAuditContext(c),
	); err != nil {
		response.InternalError(c)
		return
	}

	response.Success(c, LogoutResponse{Revoked: true})
}
