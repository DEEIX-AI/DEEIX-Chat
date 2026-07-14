package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

func boolPtr(value bool) *bool {
	return &value
}

func TestResolveProviderUserLoginAutoRegistersWhenProviderRegistrationEnabled(t *testing.T) {
	repo := &providerLoginRepo{}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		DefaultRole:         domainuser.RoleUser,
	}

	userItem, err := service.resolveProviderUser(context.Background(), provider, "sub-1", "new@example.com", "New User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err != nil {
		t.Fatalf("expected login to auto-register, got %v", err)
	}
	if userItem.ID == 0 {
		t.Fatalf("expected created user id to be assigned")
	}
	if repo.createUserCount != 1 {
		t.Fatalf("expected one user to be created, got %d", repo.createUserCount)
	}
	if len(repo.identities) != 1 {
		t.Fatalf("expected one identity to be created, got %d", len(repo.identities))
	}
	if repo.identities[0].ProviderSubject != "sub-1" || repo.identities[0].UserID != userItem.ID {
		t.Fatalf("created identity does not match user: %#v", repo.identities[0])
	}
}

func TestNormalizeProviderInputAllowsAdminDefaultRole(t *testing.T) {
	service := NewService(config.Config{JWTSecret: "test-secret"}, &providerLoginRepo{}, nil)

	provider, err := service.normalizeProviderInput(UpsertIdentityProviderInput{
		ActorRole:           domainuser.RoleAdmin,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		ClientID:            "client",
		ClientSecret:        "secret",
		DiscoveryURL:        "https://example.com/.well-known/openid-configuration",
		RegistrationEnabled: boolPtr(true),
		DefaultRole:         domainuser.RoleAdmin,
	}, nil)
	if err != nil {
		t.Fatalf("expected admin default role to be accepted, got %v", err)
	}
	if provider.DefaultRole != domainuser.RoleAdmin {
		t.Fatalf("expected default role %q, got %q", domainuser.RoleAdmin, provider.DefaultRole)
	}
}

func TestNormalizeProviderInputProtectsSuperAdminDefaultRole(t *testing.T) {
	service := NewService(config.Config{JWTSecret: "test-secret"}, &providerLoginRepo{}, nil)

	_, err := service.normalizeProviderInput(UpsertIdentityProviderInput{
		ActorRole:           domainuser.RoleAdmin,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		ClientID:            "client",
		ClientSecret:        "secret",
		DiscoveryURL:        "https://example.com/.well-known/openid-configuration",
		RegistrationEnabled: boolPtr(true),
		DefaultRole:         domainuser.RoleSuperAdmin,
	}, nil)
	if !errors.Is(err, ErrIdentityProviderSuperAdminDefaultRoleNotAllowed) {
		t.Fatalf("expected superadmin default role protection, got %v", err)
	}
}

func TestNormalizeProviderInputValidatesLogoURL(t *testing.T) {
	service := NewService(config.Config{JWTSecret: "test-secret"}, &providerLoginRepo{}, nil)

	cases := []struct {
		name    string
		logoURL string
		wantErr bool
	}{
		{name: "https url", logoURL: "https://example.com/logo.svg"},
		{name: "http url", logoURL: "http://example.com/logo.svg"},
		{name: "absolute path", logoURL: "/identity-providers/acme.svg"},
		{name: "protocol relative url", logoURL: "//example.com/logo.svg", wantErr: true},
		{name: "data url", logoURL: "data:image/svg+xml,<svg/>", wantErr: true},
		{name: "javascript url", logoURL: "javascript:alert(1)", wantErr: true},
		{name: "relative path", logoURL: "identity-providers/acme.svg", wantErr: true},
		{name: "backslash path", logoURL: `/\example.svg`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.normalizeProviderInput(UpsertIdentityProviderInput{
				ActorRole:           domainuser.RoleAdmin,
				Type:                domainuser.IdentityProviderTypeOIDC,
				Name:                "Acme SSO",
				LogoURL:             tc.logoURL,
				ClientID:            "client",
				ClientSecret:        "secret",
				DiscoveryURL:        "https://example.com/.well-known/openid-configuration",
				RegistrationEnabled: boolPtr(true),
				DefaultRole:         domainuser.RoleUser,
			}, nil)
			if tc.wantErr && err == nil {
				t.Fatalf("expected logo URL %q to be rejected", tc.logoURL)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected logo URL %q to be accepted, got %v", tc.logoURL, err)
			}
		})
	}
}

func TestResolveProviderUserAutoRegistrationAddsUsernameSuffixOnCollision(t *testing.T) {
	repo := &providerLoginRepo{duplicateUsernameAttempts: 1}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		DefaultRole:         domainuser.RoleUser,
	}

	userItem, err := service.resolveProviderUser(context.Background(), provider, "sub-1", "new@example.com", "New User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err != nil {
		t.Fatalf("expected login to retry with suffixed username, got %v", err)
	}
	if !strings.HasSuffix(userItem.Username, "-2") {
		t.Fatalf("expected suffixed username, got %q", userItem.Username)
	}
	if repo.createUserCount != 1 {
		t.Fatalf("expected one successful user create, got %d", repo.createUserCount)
	}
}

func TestResolveProviderUserLoginRequiresRegistrationEnabledForNewAccount(t *testing.T) {
	repo := &providerLoginRepo{}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: false,
		DefaultRole:         domainuser.RoleUser,
	}

	_, err := service.resolveProviderUser(context.Background(), provider, "sub-1", "new@example.com", "New User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err == nil || err.Error() != "provider account is not registered" {
		t.Fatalf("expected not registered error, got %v", err)
	}
	if repo.createUserCount != 0 || len(repo.identities) != 0 {
		t.Fatalf("expected no provisioning, users=%d identities=%d", repo.createUserCount, len(repo.identities))
	}
}

func TestResolveProviderUserAutoLinksVerifiedProviderEmailBeforeProvisioning(t *testing.T) {
	existing := &domainuser.User{
		ID:     42,
		Email:  "verified@example.com",
		Status: domainuser.StatusActive,
	}
	repo := &providerLoginRepo{usersByEmail: map[string]*domainuser.User{existing.Email: existing}}
	service := NewService(config.Config{JWTSecret: "test-secret", AutoLinkVerifiedEmail: true}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: false,
		DefaultRole:         domainuser.RoleUser,
	}

	userItem, err := service.resolveProviderUser(context.Background(), provider, "sub-1", existing.Email, "Verified User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err != nil {
		t.Fatalf("expected verified email to auto-link, got %v", err)
	}
	if userItem.ID != existing.ID {
		t.Fatalf("expected existing user %d, got %d", existing.ID, userItem.ID)
	}
	if repo.createUserCount != 0 {
		t.Fatalf("expected no new user to be created, got %d", repo.createUserCount)
	}
	if len(repo.identities) != 1 || repo.identities[0].UserID != existing.ID {
		t.Fatalf("expected identity linked to existing user, got %#v", repo.identities)
	}
}

func TestResolveProviderUserNormalizesProviderEmailBeforeAutoLink(t *testing.T) {
	existing := &domainuser.User{
		ID:     42,
		Email:  "verified@example.com",
		Status: domainuser.StatusActive,
	}
	repo := &providerLoginRepo{usersByEmail: map[string]*domainuser.User{existing.Email: existing}}
	service := NewService(config.Config{JWTSecret: "test-secret", AutoLinkVerifiedEmail: true}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: false,
		DefaultRole:         domainuser.RoleUser,
	}

	userItem, err := service.resolveProviderUser(context.Background(), provider, "sub-1", "Verified@Example.com", "Verified User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err != nil {
		t.Fatalf("expected normalized provider email to auto-link, got %v", err)
	}
	if userItem.ID != existing.ID {
		t.Fatalf("expected existing user %d, got %d", existing.ID, userItem.ID)
	}
	if len(repo.identities) != 1 || repo.identities[0].Email != existing.Email {
		t.Fatalf("expected normalized linked identity email, got %#v", repo.identities)
	}
}

func TestResolveProviderEmailVerifiedUsesConfiguredField(t *testing.T) {
	provider := domainuser.IdentityProvider{EmailVerifiedField: "verified"}
	profile := map[string]interface{}{
		"email":    "verified@example.com",
		"verified": true,
	}

	if !resolveProviderEmailVerified(profile, provider) {
		t.Fatalf("expected configured email verified field to be recognized")
	}
}

func TestResolveProviderEmailVerifiedUsesDiscordVerifiedField(t *testing.T) {
	provider := domainuser.IdentityProvider{Slug: "discord", EmailVerifiedField: "email_verified"}
	profile := map[string]interface{}{
		"email":    "verified@example.com",
		"verified": true,
	}

	if !resolveProviderEmailVerified(profile, provider) {
		t.Fatalf("expected discord verified field to be recognized as email verification")
	}
}

func TestResolveProviderEmailVerifiedDoesNotUseGenericVerifiedField(t *testing.T) {
	provider := domainuser.IdentityProvider{Slug: "x", EmailVerifiedField: "email_verified"}
	profile := map[string]interface{}{
		"email":    "verified@example.com",
		"verified": true,
	}

	if resolveProviderEmailVerified(profile, provider) {
		t.Fatalf("expected generic verified field to be ignored for non-discord providers")
	}
}

func TestCompleteProviderBindAllowsSameAccountWithoutProviderEmailVerification(t *testing.T) {
	dataKey := "test-data-key"
	clientSecret, err := secretbox.EncryptString(dataKey, "client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer"}`))
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"sub":"sub-1","email":"user@example.com","name":"Provider User"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := &domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOAuth2,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: false,
		ClientID:            "client",
		ClientSecret:        clientSecret,
		AuthURL:             server.URL + "/auth",
		TokenURL:            server.URL + "/token",
		UserInfoURL:         server.URL + "/userinfo",
		SubjectField:        "sub",
		EmailField:          "email",
		EmailVerifiedField:  "email_verified",
		NameField:           "name",
		AvatarField:         "picture",
	}
	repo := &providerLoginRepo{
		providersBySlug: map[string]*domainuser.IdentityProvider{"acme": provider},
		usersByEmail: map[string]*domainuser.User{
			"user@example.com": {ID: 42, Email: "user@example.com", Status: domainuser.StatusActive},
		},
	}
	service := NewService(config.Config{
		JWTSecret:              "test-secret",
		DataEncryptionKey:      dataKey,
		ThirdPartyLoginEnabled: true,
	}, repo, nil)
	redirectURI := "http://localhost/auth/callback?provider=acme"
	codeVerifier := strings.Repeat("a", 43)
	state, err := service.signProviderState(providerOAuthState{
		Provider:      "acme",
		RedirectURI:   redirectURI,
		Intent:        providerIntentBind,
		CodeChallenge: providerCodeChallenge(codeVerifier),
		ExpiresAt:     time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("sign provider state: %v", err)
	}

	identity, err := service.CompleteProviderBind(context.Background(), 42, "acme", "code", state, redirectURI, codeVerifier, "request-id", requestmeta.SessionAuditContext{})
	if err != nil {
		t.Fatalf("expected manual bind to succeed without provider email verification claim, got %v", err)
	}
	if identity.EmailVerified {
		t.Fatalf("expected linked identity to remain unverified")
	}
	if len(repo.identities) != 1 || repo.identities[0].UserID != 42 || repo.identities[0].EmailVerified {
		t.Fatalf("expected identity linked to current user without verified email, got %#v", repo.identities)
	}
}

func TestCompleteProviderLoginAutoLinksGitHubVerifiedPrimaryEmail(t *testing.T) {
	dataKey := "test-data-key"
	clientSecret, err := secretbox.EncryptString(dataKey, "client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer"}`))
		case "/user":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Fatalf("unexpected user authorization header %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"id":123,"login":"octocat","email":null,"avatar_url":"https://example.com/avatar.png"}`))
		case "/user/emails":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Fatalf("unexpected emails authorization header %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`[
				{"email":"secondary@example.com","primary":false,"verified":true},
				{"email":"Verified@Example.com","primary":true,"verified":true}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := &domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOAuth2,
		Name:                "GitHub",
		Slug:                "github",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		ClientID:            "client",
		ClientSecret:        clientSecret,
		AuthURL:             server.URL + "/login/oauth/authorize",
		TokenURL:            server.URL + "/login/oauth/access_token",
		UserInfoURL:         server.URL + "/user",
		SubjectField:        "id",
		EmailField:          "email",
		EmailVerifiedField:  "email_verified",
		NameField:           "login",
		AvatarField:         "avatar_url",
		DefaultRole:         domainuser.RoleUser,
	}
	existing := &domainuser.User{
		ID:          42,
		Email:       "verified@example.com",
		DisplayName: "Existing User",
		Status:      domainuser.StatusActive,
		Role:        domainuser.RoleUser,
	}
	repo := &providerLoginRepo{
		providersBySlug: map[string]*domainuser.IdentityProvider{"github": provider},
		usersByEmail:    map[string]*domainuser.User{existing.Email: existing},
	}
	service := NewService(config.Config{
		JWTSecret:              "test-secret",
		DataEncryptionKey:      dataKey,
		ThirdPartyLoginEnabled: true,
		AutoLinkVerifiedEmail:  true,
	}, repo, nil)
	redirectURI := "http://localhost/auth/callback?provider=github"
	codeVerifier := strings.Repeat("a", 43)
	state, err := service.signProviderState(providerOAuthState{
		Provider:      "github",
		RedirectURI:   redirectURI,
		Intent:        providerIntentLogin,
		CodeChallenge: providerCodeChallenge(codeVerifier),
		ExpiresAt:     time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("sign provider state: %v", err)
	}

	result, err := service.CompleteProviderLogin(context.Background(), "github", "code", state, redirectURI, codeVerifier, providerIntentLogin, "request-id", requestmeta.SessionAuditContext{})
	if err != nil {
		t.Fatalf("expected github login to auto-link existing email, got %v", err)
	}
	if result.User.ID != existing.ID || result.User.Email != existing.Email {
		t.Fatalf("expected existing user login result, got %#v", result.User)
	}
	if repo.createUserCount != 0 {
		t.Fatalf("expected no new user to be created, got %d", repo.createUserCount)
	}
	if len(repo.identities) != 1 || repo.identities[0].UserID != existing.ID || repo.identities[0].Email != existing.Email || !repo.identities[0].EmailVerified {
		t.Fatalf("expected verified github identity linked to existing user, got %#v", repo.identities)
	}
	if repo.createSessionCount != 1 || repo.updateLastLoginUserID != existing.ID {
		t.Fatalf("expected session and last login for existing user, sessions=%d lastLogin=%d", repo.createSessionCount, repo.updateLastLoginUserID)
	}
}

func TestCompleteProviderLoginReturnsErrorWhenGitHubEmailsUnavailable(t *testing.T) {
	dataKey := "test-data-key"
	clientSecret, err := secretbox.EncryptString(dataKey, "client-secret")
	if err != nil {
		t.Fatalf("encrypt client secret: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer"}`))
		case "/user":
			_, _ = w.Write([]byte(`{"id":123,"login":"octocat","email":null}`))
		case "/user/emails":
			http.Error(w, `{"message":"Requires user:email scope"}`, http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := &domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOAuth2,
		Name:                "GitHub",
		Slug:                "github",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		ClientID:            "client",
		ClientSecret:        clientSecret,
		AuthURL:             server.URL + "/login/oauth/authorize",
		TokenURL:            server.URL + "/login/oauth/access_token",
		UserInfoURL:         server.URL + "/user",
		SubjectField:        "id",
		EmailField:          "email",
		EmailVerifiedField:  "email_verified",
		NameField:           "login",
		DefaultRole:         domainuser.RoleUser,
	}
	repo := &providerLoginRepo{
		providersBySlug: map[string]*domainuser.IdentityProvider{"github": provider},
	}
	service := NewService(config.Config{
		JWTSecret:              "test-secret",
		DataEncryptionKey:      dataKey,
		ThirdPartyLoginEnabled: true,
		AutoLinkVerifiedEmail:  true,
	}, repo, nil)
	redirectURI := "http://localhost/auth/callback?provider=github"
	codeVerifier := strings.Repeat("a", 43)
	state, err := service.signProviderState(providerOAuthState{
		Provider:      "github",
		RedirectURI:   redirectURI,
		Intent:        providerIntentLogin,
		CodeChallenge: providerCodeChallenge(codeVerifier),
		ExpiresAt:     time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("sign provider state: %v", err)
	}

	_, err = service.CompleteProviderLogin(context.Background(), "github", "code", state, redirectURI, codeVerifier, providerIntentLogin, "request-id", requestmeta.SessionAuditContext{})
	if err == nil || !strings.Contains(err.Error(), "github provider emails failed") {
		t.Fatalf("expected github email lookup error, got %v", err)
	}
	if repo.createUserCount != 0 || len(repo.identities) != 0 {
		t.Fatalf("expected no user or identity side effect, users=%d identities=%#v", repo.createUserCount, repo.identities)
	}
}

func TestResolveProviderUserReturnsStructuredEmailConflict(t *testing.T) {
	existing := &domainuser.User{
		ID:     42,
		Email:  "existing@example.com",
		Status: domainuser.StatusActive,
	}
	repo := &providerLoginRepo{usersByEmail: map[string]*domainuser.User{existing.Email: existing}}
	service := NewService(config.Config{JWTSecret: "test-secret", AutoLinkVerifiedEmail: true}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOAuth2,
		Name:                "Consumer OAuth",
		Slug:                "consumer",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		DefaultRole:         domainuser.RoleUser,
	}

	_, err := service.resolveProviderUser(context.Background(), provider, "sub-1", existing.Email, "Consumer User", "", false, `{"sub":"sub-1"}`, providerIntentLogin)
	var conflictErr *ProviderEmailConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected structured email conflict, got %v", err)
	}
	if conflictErr.ProviderSlug != provider.Slug || conflictErr.Email != existing.Email || conflictErr.Action != ProviderEmailConflictActionSignInThenBind {
		t.Fatalf("unexpected conflict details: %#v", conflictErr)
	}
	if repo.createUserCount != 0 || len(repo.identities) != 0 {
		t.Fatalf("expected no user or identity side effect, users=%d identities=%#v", repo.createUserCount, repo.identities)
	}
}

func TestResolveProviderUserRejectsInactiveBoundUserWithoutUpdatingIdentity(t *testing.T) {
	repo := &providerLoginRepo{
		usersByID: map[uint]*domainuser.User{
			42: {ID: 42, Status: domainuser.StatusSuspended},
		},
		identities: []domainuser.UserIdentity{
			{ID: 7, UserID: 42, ProviderID: 10, ProviderSubject: "sub-1"},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		DefaultRole:         domainuser.RoleUser,
	}

	_, err := service.resolveProviderUser(context.Background(), provider, "sub-1", "bound@example.com", "Bound User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err == nil || err.Error() != ErrInvalidCredentials.Error() {
		t.Fatalf("expected inactive account rejection, got %v", err)
	}
	if repo.updateIdentityLoginCount != 0 {
		t.Fatalf("expected identity login not to be updated, got %d", repo.updateIdentityLoginCount)
	}
}

func TestResolveProviderUserRejectsInactiveAutoLinkUserWithoutBinding(t *testing.T) {
	now := time.Now()
	existing := &domainuser.User{
		ID:              42,
		Email:           "suspended@example.com",
		EmailVerifiedAt: &now,
		Status:          domainuser.StatusSuspended,
	}
	repo := &providerLoginRepo{usersByEmail: map[string]*domainuser.User{existing.Email: existing}}
	service := NewService(config.Config{JWTSecret: "test-secret", AutoLinkVerifiedEmail: true}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		DefaultRole:         domainuser.RoleUser,
	}

	_, err := service.resolveProviderUser(context.Background(), provider, "sub-1", existing.Email, "Suspended User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err == nil || err.Error() != ErrInvalidCredentials.Error() {
		t.Fatalf("expected inactive account rejection, got %v", err)
	}
	if len(repo.identities) != 0 {
		t.Fatalf("expected no auto-link side effect, got %#v", repo.identities)
	}
}

func TestResolveProviderUserReturnsIdentityCreateErrorWithoutCleanupCompensation(t *testing.T) {
	repo := &providerLoginRepo{createIdentityErr: errors.New("duplicate identity")}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)
	provider := domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOIDC,
		Name:                "Acme SSO",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		DefaultRole:         domainuser.RoleUser,
	}

	_, err := service.resolveProviderUser(context.Background(), provider, "sub-1", "new@example.com", "New User", "", true, `{"sub":"sub-1"}`, providerIntentLogin)
	if err == nil || err.Error() != "duplicate identity" {
		t.Fatalf("expected identity creation error, got %v", err)
	}
	if repo.createUserCount != 0 {
		t.Fatalf("expected transaction to avoid standalone user provisioning, got %d", repo.createUserCount)
	}
	if repo.deletedUserID != 0 {
		t.Fatalf("expected no cleanup compensation, got deleted id %d", repo.deletedUserID)
	}
}

func TestUnlinkCurrentUserIdentityRejectsLastPasswordlessLoginMethod(t *testing.T) {
	repo := &providerLoginRepo{
		credentialsByUserID: map[uint]*domainuser.Credential{
			42: {UserID: 42, PasswordEnabled: false},
		},
		identities: []domainuser.UserIdentity{
			{ID: 7, UserID: 42, ProviderID: 10, ProviderSubject: "sub-1"},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)

	err := service.UnlinkCurrentUserIdentity(context.Background(), 42, 7)
	if !errors.Is(err, ErrLastLoginMethodNotAllowed) {
		t.Fatalf("expected last login method rejection, got %v", err)
	}
	if repo.deletedIdentityID != 0 {
		t.Fatalf("expected identity not to be deleted, got %d", repo.deletedIdentityID)
	}
}

func TestGetIdentityProviderLogoFetchesConfiguredImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "" {
			t.Fatal("expected image accept header")
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer server.Close()

	repo := &providerLoginRepo{
		providersBySlug: map[string]*domainuser.IdentityProvider{
			"acme": {
				Slug:    "acme",
				LogoURL: server.URL + "/logo.png",
			},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)

	asset, err := service.GetIdentityProviderLogo(context.Background(), "acme")
	if err != nil {
		t.Fatalf("expected logo asset, got %v", err)
	}
	if asset.ContentType != "image/png" {
		t.Fatalf("expected image/png, got %q", asset.ContentType)
	}
	if string(asset.Content) != string([]byte{0x89, 0x50, 0x4e, 0x47}) {
		t.Fatalf("unexpected logo content: %#v", asset.Content)
	}
}

func TestGetIdentityProviderLogoRejectsHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<script>alert(1)</script>"))
	}))
	defer server.Close()

	repo := &providerLoginRepo{
		providersBySlug: map[string]*domainuser.IdentityProvider{
			"acme": {
				Slug:    "acme",
				LogoURL: server.URL + "/logo.html",
			},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)

	if _, err := service.GetIdentityProviderLogo(context.Background(), "acme"); !errors.Is(err, ErrIdentityProviderLogoUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestUnlinkCurrentUserIdentityAllowsLastIdentityWhenPasswordEnabled(t *testing.T) {
	repo := &providerLoginRepo{
		credentialsByUserID: map[uint]*domainuser.Credential{
			42: {UserID: 42, PasswordEnabled: true},
		},
		identities: []domainuser.UserIdentity{
			{ID: 7, UserID: 42, ProviderID: 10, ProviderSubject: "sub-1"},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)

	if err := service.UnlinkCurrentUserIdentity(context.Background(), 42, 7); err != nil {
		t.Fatalf("expected unlink to succeed, got %v", err)
	}
	if repo.deletedIdentityID != 7 {
		t.Fatalf("expected identity to be deleted, got %d", repo.deletedIdentityID)
	}
}

func TestUnlinkCurrentUserIdentityAllowsOneOfMultiplePasswordlessLoginMethods(t *testing.T) {
	repo := &providerLoginRepo{
		credentialsByUserID: map[uint]*domainuser.Credential{
			42: {UserID: 42, PasswordEnabled: false},
		},
		identities: []domainuser.UserIdentity{
			{ID: 7, UserID: 42, ProviderID: 10, ProviderSubject: "sub-1"},
			{ID: 8, UserID: 42, ProviderID: 11, ProviderSubject: "sub-2"},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)

	if err := service.UnlinkCurrentUserIdentity(context.Background(), 42, 7); err != nil {
		t.Fatalf("expected unlink to succeed, got %v", err)
	}
	if repo.deletedIdentityID != 7 {
		t.Fatalf("expected identity to be deleted, got %d", repo.deletedIdentityID)
	}
	if len(repo.identities) != 1 || repo.identities[0].ID != 8 {
		t.Fatalf("expected remaining identity 8, got %#v", repo.identities)
	}
}

type providerLoginRepo struct {
	repository.AuthRepository

	nextUserID                uint
	nextIdentityID            uint
	createUserCount           int
	updateIdentityLoginCount  int
	deletedUserID             uint
	deletedIdentityID         uint
	createIdentityErr         error
	duplicateUsernameAttempts int
	createSessionCount        int
	updateLastLoginUserID     uint
	usersByID                 map[uint]*domainuser.User
	usersByEmail              map[string]*domainuser.User
	credentialsByUserID       map[uint]*domainuser.Credential
	identities                []domainuser.UserIdentity
	providersBySlug           map[string]*domainuser.IdentityProvider
}

func (r *providerLoginRepo) GetIdentityProviderBySlug(ctx context.Context, slug string) (*domainuser.IdentityProvider, error) {
	if r.providersBySlug == nil {
		return nil, repository.ErrNotFound
	}
	provider, ok := r.providersBySlug[slug]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return provider, nil
}

func (r *providerLoginRepo) GetUserIdentityByProviderSubject(ctx context.Context, providerID uint, subject string) (*domainuser.UserIdentity, error) {
	for index := range r.identities {
		identity := r.identities[index]
		if identity.ProviderID == providerID && identity.ProviderSubject == subject {
			return &identity, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *providerLoginRepo) GetByID(ctx context.Context, userID uint) (*domainuser.User, error) {
	if r.usersByID == nil {
		return nil, repository.ErrNotFound
	}
	userItem, ok := r.usersByID[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return userItem, nil
}

func (r *providerLoginRepo) GetByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	if r.usersByEmail == nil {
		return nil, repository.ErrNotFound
	}
	userItem, ok := r.usersByEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return userItem, nil
}

func (r *providerLoginRepo) CreateWithCredential(
	ctx context.Context,
	item *domainuser.User,
	credential domainuser.Credential,
	subscriptionPlanID uint,
	subscriptionPriceID uint,
	subscriptionEndAt *time.Time,
	autoRenew bool,
) error {
	r.createUserCount++
	if r.nextUserID == 0 {
		r.nextUserID = 100
	}
	item.ID = r.nextUserID
	r.nextUserID++
	return nil
}

func (r *providerLoginRepo) CreateWithCredentialAndIdentity(
	ctx context.Context,
	item *domainuser.User,
	credential domainuser.Credential,
	identity *domainuser.UserIdentity,
	subscriptionPlanID uint,
	subscriptionPriceID uint,
	subscriptionEndAt *time.Time,
	autoRenew bool,
) error {
	if r.createIdentityErr != nil {
		return r.createIdentityErr
	}
	if r.duplicateUsernameAttempts > 0 {
		r.duplicateUsernameAttempts--
		return repository.ErrDuplicateUsername
	}
	r.createUserCount++
	if r.nextUserID == 0 {
		r.nextUserID = 100
	}
	item.ID = r.nextUserID
	r.nextUserID++
	if identity != nil {
		if r.nextIdentityID == 0 {
			r.nextIdentityID = 200
		}
		identity.ID = r.nextIdentityID
		identity.UserID = item.ID
		r.nextIdentityID++
		r.identities = append(r.identities, *identity)
	}
	return nil
}

func (r *providerLoginRepo) CreateUserIdentity(ctx context.Context, identity *domainuser.UserIdentity) (*domainuser.UserIdentity, error) {
	if r.createIdentityErr != nil {
		return nil, r.createIdentityErr
	}
	if r.nextIdentityID == 0 {
		r.nextIdentityID = 200
	}
	identity.ID = r.nextIdentityID
	r.nextIdentityID++
	r.identities = append(r.identities, *identity)
	return identity, nil
}

func (r *providerLoginRepo) CreateSession(ctx context.Context, item *domainuser.Session) error {
	r.createSessionCount++
	return nil
}

func (r *providerLoginRepo) UpdateLastLogin(ctx context.Context, userID uint) error {
	r.updateLastLoginUserID = userID
	return nil
}

func (r *providerLoginRepo) ListUserIdentitiesByUserID(ctx context.Context, userID uint) ([]domainuser.UserIdentity, error) {
	results := make([]domainuser.UserIdentity, 0)
	for _, identity := range r.identities {
		if identity.UserID == userID {
			results = append(results, identity)
		}
	}
	return results, nil
}

func (r *providerLoginRepo) GetCredentialByUserID(ctx context.Context, userID uint) (*domainuser.Credential, error) {
	if r.credentialsByUserID == nil {
		return nil, repository.ErrNotFound
	}
	credential, ok := r.credentialsByUserID[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return credential, nil
}

func (r *providerLoginRepo) GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error) {
	return nil, repository.ErrNotFound
}

func (r *providerLoginRepo) DeleteUserIdentity(ctx context.Context, userID uint, identityID uint) error {
	credential, err := r.GetCredentialByUserID(ctx, userID)
	if err != nil {
		return err
	}
	identityIndex := -1
	userIdentityCount := 0
	for index, identity := range r.identities {
		if identity.UserID != userID {
			continue
		}
		userIdentityCount++
		if identity.ID == identityID {
			identityIndex = index
		}
	}
	if identityIndex < 0 {
		return repository.ErrNotFound
	}
	if !credential.PasswordEnabled && userIdentityCount <= 1 {
		return repository.ErrConflict
	}
	r.deletedIdentityID = identityID
	r.identities = append(r.identities[:identityIndex], r.identities[identityIndex+1:]...)
	return nil
}

func (r *providerLoginRepo) UpdateUserIdentityLogin(ctx context.Context, identityID uint, profileJSON string, providerDisplayName string, email string, emailVerified bool) error {
	r.updateIdentityLoginCount++
	return nil
}

func (r *providerLoginRepo) RecordAuthEvent(ctx context.Context, userID uint, requestID string, eventType string, result string, reason string, clientIP string, userAgent string, detailJSON string) error {
	return nil
}

func (r *providerLoginRepo) DeleteAccountHard(ctx context.Context, userID uint) error {
	r.deletedUserID = userID
	return nil
}
