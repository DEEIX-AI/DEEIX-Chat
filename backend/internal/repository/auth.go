package repository

import (
	"context"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// AuthRepository 定义认证流程依赖的持久化能力。
type AuthRepository interface {
	CountSuperAdmins(ctx context.Context) (int64, error)
	CreateWithCredential(
		ctx context.Context,
		item *domainuser.User,
		credential domainuser.Credential,
		subscriptionPlanID uint,
		subscriptionPriceID uint,
		subscriptionEndAt *time.Time,
		autoRenew bool,
	) error
	CreateWithCredentialAndIdentity(
		ctx context.Context,
		user *domainuser.User,
		credential domainuser.Credential,
		identity *domainuser.UserIdentity,
		subscriptionPlanID uint,
		subscriptionPriceID uint,
		subscriptionEndAt *time.Time,
		autoRenew bool,
	) error
	GetByUsername(ctx context.Context, username string) (*domainuser.User, error)
	GetByEmail(ctx context.Context, email string) (*domainuser.User, error)
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
	UpdateProfile(ctx context.Context, userID uint, input UpdateUserFieldsInput) (*domainuser.User, error)
	UpdateUsernameOnce(ctx context.Context, userID uint, username string, changedAt time.Time) (*domainuser.User, error)
	GetCredentialByUserID(ctx context.Context, userID uint) (*domainuser.Credential, error)
	GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error)
	UpsertUserTwoFactor(ctx context.Context, item *domainuser.UserTwoFactor) (*domainuser.UserTwoFactor, error)
	UpdateUserTwoFactor(ctx context.Context, userID uint, input UpdateUserTwoFactorInput) (*domainuser.UserTwoFactor, error)
	DeleteUserTwoFactor(ctx context.Context, userID uint) error
	MarkLoginFailure(ctx context.Context, userID uint, lockThreshold int, lockUntil time.Time) (*domainuser.Credential, error)
	ResetLoginFailure(ctx context.Context, userID uint) error
	UpdateUserStatus(ctx context.Context, userID uint, status string) error
	UpdatePassword(ctx context.Context, userID uint, passwordHash string, passwordOrigin string, mustResetPassword bool) error
	ResetPasswordByAdmin(ctx context.Context, userID uint, passwordHash string, mustResetPassword bool) error
	MarkBootstrapSuperAdminPasswordResetRequired(ctx context.Context, username string) error
	UpdateLastLogin(ctx context.Context, userID uint) error
	DeleteAccountHard(ctx context.Context, userID uint) error
	ListDistinctFileStoragePathsByUserID(ctx context.Context, userID uint) ([]string, error)
	RecordAuthEvent(
		ctx context.Context,
		userID uint,
		requestID string,
		eventType string,
		result string,
		reason string,
		clientIP string,
		userAgent string,
		detailJSON string,
	) error
	CreateSession(ctx context.Context, item *domainuser.Session) error
	GetSessionByUserAndSessionID(ctx context.Context, userID uint, sessionID string) (*domainuser.Session, error)
	RotateSessionTokens(ctx context.Context, input RotateSessionTokensInput) error
	TouchSessionActivity(ctx context.Context, userID uint, sessionID string, input UpdateSessionActivityInput) error
	RevokeSession(ctx context.Context, userID uint, sessionID string, reason string) error
	RevokeAllSessions(ctx context.Context, userID uint, reason string) error
	ListActiveSessionsByUserID(ctx context.Context, userID uint, now time.Time) ([]domainuser.Session, error)
	HasActiveSuperAdminIdentity(ctx context.Context) (bool, error)
	ListIdentityProviders(ctx context.Context, includeDisabled bool) ([]domainuser.IdentityProvider, error)
	GetIdentityProviderByPublicID(ctx context.Context, publicID string) (*domainuser.IdentityProvider, error)
	GetIdentityProviderBySlug(ctx context.Context, slug string) (*domainuser.IdentityProvider, error)
	CreateIdentityProvider(ctx context.Context, provider *domainuser.IdentityProvider) (*domainuser.IdentityProvider, error)
	UpdateIdentityProvider(ctx context.Context, publicID string, input UpdateIdentityProviderInput) (*domainuser.IdentityProvider, error)
	UpdateIdentityProviderSortOrders(ctx context.Context, publicIDs []string) error
	DeleteIdentityProvider(ctx context.Context, publicID string, force bool) error
	ListUserIdentitiesByUserID(ctx context.Context, userID uint) ([]domainuser.UserIdentity, error)
	GetUserIdentityByProviderSubject(ctx context.Context, providerID uint, subject string) (*domainuser.UserIdentity, error)
	CreateUserIdentity(ctx context.Context, identity *domainuser.UserIdentity) (*domainuser.UserIdentity, error)
	UpdateUserIdentityLogin(ctx context.Context, identityID uint, profileJSON string, providerDisplayName string, email string, emailVerified bool) error
	DeleteUserIdentity(ctx context.Context, userID uint, identityID uint) error
	CancelPendingContactVerifications(ctx context.Context, channel string, purpose string, target string) error
	CancelPendingContactVerificationsForUser(ctx context.Context, userID uint, channel string, purpose string, target string) error
	CreateContactVerification(ctx context.Context, item *domainuser.ContactVerification) (*domainuser.ContactVerification, error)
	GetPendingContactVerification(ctx context.Context, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error)
	GetPendingContactVerificationForUser(ctx context.Context, userID uint, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error)
	IncrementContactVerificationAttempt(ctx context.Context, verificationID uint) error
	MarkContactVerificationVerified(ctx context.Context, verificationID uint, now time.Time) error
}
