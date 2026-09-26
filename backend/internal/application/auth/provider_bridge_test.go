package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	memorycache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

func TestStartProviderAuthBridgeKeepsProviderCallbackAndPKCEOnServer(t *testing.T) {
	service, store := newProviderAuthBridgeTestService()
	clientVerifier := strings.Repeat("c", 43)
	clientState := strings.Repeat("s", 43)

	result, err := service.StartProviderAuthBridge(context.Background(), "acme", ProviderAuthBridgeStartInput{
		ClientID:      ProviderAuthNativeClientID,
		RedirectURI:   providerAuthNativeRedirect,
		CodeChallenge: providerCodeChallenge(clientVerifier),
		ClientState:   clientState,
		Intent:        providerIntentLogin,
		Next:          "/chat",
	})
	if err != nil {
		t.Fatalf("start provider auth bridge: %v", err)
	}
	authorizationURL, err := url.Parse(result.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorization url: %v", err)
	}
	if got := authorizationURL.Query().Get("redirect_uri"); got != "https://api.example.com/api/v1/auth/providers/acme/callback" {
		t.Fatalf("expected instance callback, got %q", got)
	}
	if got := authorizationURL.Query().Get("code_challenge"); got == "" || got == providerCodeChallenge(clientVerifier) {
		t.Fatalf("expected an independent server-side provider PKCE challenge, got %q", got)
	}
	state, err := service.verifyProviderAuthBridgeState("acme", authorizationURL.Query().Get("state"))
	if err != nil {
		t.Fatalf("verify bridge state: %v", err)
	}
	transaction, err := store.ConsumeProviderAuthTransaction(context.Background(), state.TransactionID)
	if err != nil {
		t.Fatalf("consume transaction: %v", err)
	}
	if transaction.ClientCodeChallenge != providerCodeChallenge(clientVerifier) || transaction.ProviderCodeVerifier == clientVerifier {
		t.Fatalf("client and provider PKCE values were not separated: %#v", transaction)
	}
}

func TestProviderAuthBridgeReturnsProviderDenialThroughOneTimeGrant(t *testing.T) {
	service, _ := newProviderAuthBridgeTestService()
	clientVerifier := strings.Repeat("v", 43)
	clientState := strings.Repeat("t", 43)
	start, err := service.StartProviderAuthBridge(context.Background(), "acme", ProviderAuthBridgeStartInput{
		ClientID:      ProviderAuthNativeClientID,
		RedirectURI:   providerAuthNativeRedirect,
		CodeChallenge: providerCodeChallenge(clientVerifier),
		ClientState:   clientState,
	})
	if err != nil {
		t.Fatalf("start provider auth bridge: %v", err)
	}
	authorizationURL, _ := url.Parse(start.AuthorizationURL)
	callback, err := service.CompleteProviderAuthBridgeCallback(context.Background(), "acme", ProviderAuthBridgeCallbackInput{
		State:         authorizationURL.Query().Get("state"),
		ProviderError: "access_denied",
	})
	if err != nil {
		t.Fatalf("complete denied callback: %v", err)
	}
	redirect, err := url.Parse(callback.RedirectURI)
	if err != nil {
		t.Fatalf("parse client redirect: %v", err)
	}
	if redirect.Scheme != "com.deeix.chat" || redirect.Query().Get("state") != clientState {
		t.Fatalf("unexpected client redirect %q", callback.RedirectURI)
	}
	input := ProviderAuthBridgeExchangeInput{
		ClientID:     ProviderAuthNativeClientID,
		Grant:        redirect.Query().Get("grant"),
		CodeVerifier: clientVerifier,
	}
	if _, err = service.ExchangeProviderAuthBridgeGrant(context.Background(), "acme", input, "request-id", requestmeta.SessionAuditContext{}); err == nil || err.Error() != "provider authorization was denied" {
		t.Fatalf("expected provider denial, got %v", err)
	}
	if _, err = service.ExchangeProviderAuthBridgeGrant(context.Background(), "acme", input, "request-id", requestmeta.SessionAuditContext{}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected one-time grant to be consumed, got %v", err)
	}
}

func TestExchangeProviderAuthBridgeGrantRequiresOriginalPKCEVerifier(t *testing.T) {
	service, store := newProviderAuthBridgeTestService()
	userItem := &domainuser.User{ID: 42, PublicID: "user-42", Username: "alice", DisplayName: "Alice", Role: domainuser.RoleUser, Status: domainuser.StatusActive}
	repo := service.repo.(*providerLoginRepo)
	repo.usersByID = map[uint]*domainuser.User{userItem.ID: userItem}
	verifier := strings.Repeat("p", 43)
	rawGrant := strings.Repeat("g", 43)
	grant := repository.ProviderAuthGrant{
		ProviderSlug: "acme",
		ClientID:     ProviderAuthNativeClientID,
		UserID:       userItem.ID,
		Subject:      "subject-42",
		ExpiresAt:    time.Now().Add(time.Minute),
	}
	if err := store.PutProviderAuthGrant(context.Background(), providerAuthGrantKey(rawGrant, providerCodeChallenge(verifier)), grant, time.Minute); err != nil {
		t.Fatalf("put grant: %v", err)
	}

	wrongInput := ProviderAuthBridgeExchangeInput{ClientID: ProviderAuthNativeClientID, Grant: rawGrant, CodeVerifier: strings.Repeat("x", 43)}
	if _, err := service.ExchangeProviderAuthBridgeGrant(context.Background(), "acme", wrongInput, "request-id", requestmeta.SessionAuditContext{}); err == nil {
		t.Fatal("expected wrong verifier to fail")
	}
	correctInput := ProviderAuthBridgeExchangeInput{ClientID: ProviderAuthNativeClientID, Grant: rawGrant, CodeVerifier: verifier}
	result, err := service.ExchangeProviderAuthBridgeGrant(context.Background(), "acme", correctInput, "request-id", requestmeta.SessionAuditContext{})
	if err != nil {
		t.Fatalf("exchange grant: %v", err)
	}
	if result.User.ID != userItem.ID || result.AccessToken == "" || repo.createSessionCount != 1 {
		t.Fatalf("expected the standard session flow, got result=%#v sessions=%d", result, repo.createSessionCount)
	}
}

func TestProviderAuthBridgeRejectsUnregisteredNativeRedirect(t *testing.T) {
	service, _ := newProviderAuthBridgeTestService()
	_, err := service.StartProviderAuthBridge(context.Background(), "acme", ProviderAuthBridgeStartInput{
		ClientID:      ProviderAuthNativeClientID,
		RedirectURI:   "evil.app:/oauth/callback",
		CodeChallenge: providerCodeChallenge(strings.Repeat("c", 43)),
		ClientState:   strings.Repeat("s", 43),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid native redirect") {
		t.Fatalf("expected redirect allowlist rejection, got %v", err)
	}
}

// 桌面端 (RFC 8252) 只接受回环地址上的临时端口，其他一律拒绝。
func TestProviderAuthBridgeDesktopRedirectMustBeLoopbackWithPort(t *testing.T) {
	service, _ := newProviderAuthBridgeTestService()
	start := func(redirect string) error {
		_, err := service.StartProviderAuthBridge(context.Background(), "acme", ProviderAuthBridgeStartInput{
			ClientID:      ProviderAuthDesktopClientID,
			RedirectURI:   redirect,
			CodeChallenge: providerCodeChallenge(strings.Repeat("c", 43)),
			ClientState:   strings.Repeat("s", 43),
		})
		return err
	}
	for _, ok := range []string{"http://127.0.0.1:49152/oauth/callback", "http://localhost:8123/oauth/callback", "http://[::1]:5000/oauth/callback"} {
		if err := start(ok); err != nil {
			t.Fatalf("expected %q to be accepted, got %v", ok, err)
		}
	}
	for _, bad := range []string{
		"https://127.0.0.1:49152/oauth/callback", // must be plain http on loopback
		"http://127.0.0.1/oauth/callback",        // ephemeral port is mandatory
		"http://127.0.0.1:49152/other",           // fixed path
		"http://127.0.0.1:49152/oauth/callback?x=1",
		"http://evil.example.com:49152/oauth/callback",
		"http://127.0.0.1.evil.com:49152/oauth/callback",
		"deeix-chat://oauth/callback",
	} {
		if err := start(bad); err == nil || !strings.Contains(err.Error(), "desktop redirect") {
			t.Fatalf("expected %q to be rejected as desktop redirect, got %v", bad, err)
		}
	}
}

func newProviderAuthBridgeTestService() (*Service, *memorycache.Cache) {
	provider := &domainuser.IdentityProvider{
		ID:                  10,
		Type:                domainuser.IdentityProviderTypeOAuth2,
		Name:                "Acme",
		Slug:                "acme",
		LoginEnabled:        true,
		RegistrationEnabled: true,
		ClientID:            "provider-client",
		AuthURL:             "https://idp.example.com/authorize",
		TokenURL:            "https://idp.example.com/token",
		UserInfoURL:         "https://idp.example.com/userinfo",
		Scopes:              "openid profile email",
	}
	repo := &providerLoginRepo{providersBySlug: map[string]*domainuser.IdentityProvider{"acme": provider}}
	service := newTestService(config.Config{
		JWTSecret:              "test-secret",
		PublicAPIBaseURL:       "https://api.example.com",
		ThirdPartyLoginEnabled: true,
		TokenTTLHours:          1,
		RefreshTokenTTLHours:   720,
	}, repo, nil)
	store := memorycache.New()
	service.SetProviderAuthBridge(memorycache.NewProviderAuthBridge(store))
	return service, store
}
