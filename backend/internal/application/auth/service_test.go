package auth

import (
	"context"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

type validateAccessSessionRepo struct {
	repository.AuthRepository
	session     *domainuser.Session
	touchInputs []repository.UpdateSessionActivityInput
}

func TestCleanupDeletedAccountFilesWithoutProviderKeepsFailuresSilent(t *testing.T) {
	paths := []string{"avatars/one.png", "uploads/two.txt"}
	failed := (&Service{}).cleanupDeletedAccountFiles(t.Context(), paths)
	if len(failed) != len(paths) || failed[0] != paths[0] || failed[1] != paths[1] {
		t.Fatalf("cleanup failures = %#v, want %#v", failed, paths)
	}
}

func (r *validateAccessSessionRepo) GetSessionByUserAndSessionID(_ context.Context, userID uint, sessionID string) (*domainuser.Session, error) {
	if r.session == nil || r.session.UserID != userID || r.session.SessionID != sessionID {
		return nil, repository.ErrNotFound
	}
	return r.session, nil
}

func (r *validateAccessSessionRepo) TouchSessionActivity(_ context.Context, _ uint, _ string, input repository.UpdateSessionActivityInput) error {
	r.touchInputs = append(r.touchInputs, input)
	return nil
}

func (r *validateAccessSessionRepo) ListActiveSessionsByUserID(_ context.Context, userID uint, _ time.Time) ([]domainuser.Session, error) {
	if r.session == nil || r.session.UserID != userID {
		return nil, nil
	}
	return []domainuser.Session{*r.session}, nil
}

type validateAccessSessionGeoResolver struct {
	result requestmeta.SessionAuditContext
	err    error
	inputs []string
}

func (r *validateAccessSessionGeoResolver) Lookup(_ context.Context, rawIP string) (requestmeta.SessionAuditContext, error) {
	r.inputs = append(r.inputs, rawIP)
	return r.result, r.err
}

func newValidateAccessSessionWithGeo(now time.Time) *domainuser.Session {
	latitude := 31.2304
	longitude := 121.4737
	return &domainuser.Session{
		SessionID:    "session-id",
		UserID:       1,
		ClientIP:     "203.0.113.10",
		GeoSource:    "geoip_api",
		GeoAccuracy:  "ip",
		CountryCode:  "CN",
		RegionName:   "Shanghai",
		CityName:     "Shanghai",
		TimezoneName: "Asia/Shanghai",
		IPLatitude:   &latitude,
		IPLongitude:  &longitude,
		CreatedAt:    now.Add(-30 * time.Minute),
		LastSeenAt:   &now,
		ExpiresAt:    now.Add(24 * time.Hour),
	}
}

func TestNormalizeAppearancePreferencesAllowsFontSize(t *testing.T) {
	for _, fontSize := range []string{"small", "standard", "medium", "large"} {
		payload := `{"theme":"system","preset":"default","chatFont":"default","chatFontWeight":"regular","fontSize":"` + fontSize + `"}`

		if _, err := normalizeAppearancePreferences(payload); err != nil {
			t.Fatalf("expected fontSize %q appearance preference to be valid, got %v", fontSize, err)
		}
	}
}

func TestNormalizeAppearancePreferencesDefaultsInvalidFontSize(t *testing.T) {
	payload := `{"fontSize":"huge"}`

	normalized, err := normalizeAppearancePreferences(payload)
	if err != nil {
		t.Fatalf("expected invalid fontSize appearance preference to fall back, got %v", err)
	}
	if normalized != `{"fontSize":"standard"}` {
		t.Fatalf("expected invalid fontSize to fall back to standard, got %s", normalized)
	}
}

func TestNormalizeAppearancePreferencesRejectsUnknownKey(t *testing.T) {
	payload := `{"fontSize":"standard","unknown":"value"}`

	if _, err := normalizeAppearancePreferences(payload); err == nil {
		t.Fatal("expected unknown appearance preference key to be rejected")
	}
}

func TestShouldRequireInitialUsernameOnlyForBootstrapSuperAdmin(t *testing.T) {
	if !shouldRequireInitialUsername(domainuser.User{
		Username: "admin",
		Role:     domainuser.RoleSuperAdmin,
	}, "admin") {
		t.Fatal("expected bootstrap superadmin username to require initialization change")
	}

	if shouldRequireInitialUsername(domainuser.User{
		Username:          "admin",
		Role:              domainuser.RoleSuperAdmin,
		UsernameChangedAt: ptrTime(time.Now()),
	}, "admin") {
		t.Fatal("expected changed superadmin username to remain optional")
	}

	if shouldRequireInitialUsername(domainuser.User{
		Username:    "user",
		Role:        domainuser.RoleUser,
		EmailSource: domainuser.EmailSourceLocalRegister,
	}, "admin") {
		t.Fatal("expected local registration user username to remain optional")
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func TestValidateAccessSessionAllowsTokenIssuedBeforeLatestRefresh(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-30 * time.Minute)
	lastSeenAt := now
	service := &Service{
		repo: &validateAccessSessionRepo{
			session: &domainuser.Session{
				SessionID:  "session-id",
				UserID:     1,
				AccessJTI:  "latest-access-jti",
				CreatedAt:  createdAt,
				IssuedAt:   now,
				LastSeenAt: &lastSeenAt,
				ExpiresAt:  now.Add(24 * time.Hour),
			},
		},
	}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		createdAt.Add(5*time.Minute),
		requestmeta.SessionAuditContext{},
	)
	if err != nil {
		t.Fatalf("expected access token issued before latest refresh to remain valid, got %v", err)
	}
}

func TestValidateAccessSessionRejectsTokenBeforeSessionCreation(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-30 * time.Minute)
	lastSeenAt := now
	service := &Service{
		repo: &validateAccessSessionRepo{
			session: &domainuser.Session{
				SessionID:  "session-id",
				UserID:     1,
				CreatedAt:  createdAt,
				LastSeenAt: &lastSeenAt,
				ExpiresAt:  now.Add(24 * time.Hour),
			},
		},
	}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		createdAt.Add(-accessTokenSessionClockSkew-time.Second),
		requestmeta.SessionAuditContext{},
	)
	if err == nil {
		t.Fatal("expected access token issued before session creation to be rejected")
	}
}

func TestValidateAccessSessionDoesNotOverwriteStoredGeoForSameIP(t *testing.T) {
	now := time.Now()
	lastSeenAt := now.Add(-2 * time.Minute)
	session := newValidateAccessSessionWithGeo(now)
	session.LastSeenAt = &lastSeenAt
	repo := &validateAccessSessionRepo{session: session}
	resolver := &validateAccessSessionGeoResolver{}
	service := &Service{repo: repo, geoResolver: resolver}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		session.CreatedAt.Add(time.Minute),
		requestmeta.SessionAuditContext{ClientIP: "203.0.113.10"},
	)
	if err != nil {
		t.Fatalf("expected session validation to succeed, got %v", err)
	}
	if len(resolver.inputs) != 0 {
		t.Fatalf("expected no GeoIP lookup for unchanged IP, got %d", len(resolver.inputs))
	}
	if len(repo.touchInputs) != 1 {
		t.Fatalf("expected one activity update, got %d", len(repo.touchInputs))
	}
	input := repo.touchInputs[0]
	if input.GeoSource != nil || input.GeoAccuracy != nil || input.CountryCode != nil ||
		input.RegionName != nil || input.CityName != nil || input.TimezoneName != nil ||
		input.IPLatitude != nil || input.IPLongitude != nil {
		t.Fatalf("expected same-IP activity updates to leave stored location untouched, got %+v", input)
	}
}

func TestValidateAccessSessionUpdatesExplicitGeoForSameIP(t *testing.T) {
	now := time.Now()
	session := newValidateAccessSessionWithGeo(now)
	repo := &validateAccessSessionRepo{session: session}
	service := &Service{repo: repo}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		session.CreatedAt.Add(time.Minute),
		requestmeta.SessionAuditContext{
			ClientIP:     session.ClientIP,
			GeoSource:    "proxy_header_trusted",
			GeoAccuracy:  "ip",
			CountryCode:  "CN",
			RegionName:   "Jiangsu",
			CityName:     "Nanjing",
			TimezoneName: "Asia/Shanghai",
		},
	)
	if err != nil {
		t.Fatalf("expected session validation to succeed, got %v", err)
	}
	if len(repo.touchInputs) != 1 {
		t.Fatalf("expected one activity update, got %d", len(repo.touchInputs))
	}
	input := repo.touchInputs[0]
	if input.GeoSource == nil || *input.GeoSource != "proxy_header_trusted" ||
		input.RegionName == nil || *input.RegionName != "Jiangsu" ||
		input.CityName == nil || *input.CityName != "Nanjing" {
		t.Fatalf("expected explicit request location to update the session, got %+v", input)
	}
}

func TestValidateAccessSessionInvalidatesGeoWhenClientIPChanges(t *testing.T) {
	now := time.Now()
	session := newValidateAccessSessionWithGeo(now)
	repo := &validateAccessSessionRepo{session: session}
	resolver := &validateAccessSessionGeoResolver{}
	service := &Service{repo: repo, geoResolver: resolver}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		session.CreatedAt.Add(time.Minute),
		requestmeta.SessionAuditContext{ClientIP: "198.51.100.20"},
	)
	if err != nil {
		t.Fatalf("expected session validation to succeed, got %v", err)
	}
	if len(resolver.inputs) != 0 {
		t.Fatalf("expected access validation not to perform external GeoIP lookups, got %#v", resolver.inputs)
	}
	if len(repo.touchInputs) != 1 {
		t.Fatalf("expected one activity update, got %d", len(repo.touchInputs))
	}
	input := repo.touchInputs[0]
	if input.ClientIP == nil || *input.ClientIP != "198.51.100.20" ||
		input.CountryCode == nil || *input.CountryCode != "" ||
		input.RegionName == nil || *input.RegionName != "" ||
		input.CityName == nil || *input.CityName != "" ||
		input.IPLatitude == nil || *input.IPLatitude != nil ||
		input.IPLongitude == nil || *input.IPLongitude != nil {
		t.Fatalf("expected stale location to be invalidated for the new IP, got %+v", input)
	}
}

func TestListCurrentActiveSessionsResolvesMissingGeo(t *testing.T) {
	now := time.Now()
	session := newValidateAccessSessionWithGeo(now)
	session.ClientIP = "198.51.100.20"
	session.GeoSource = ""
	session.GeoAccuracy = ""
	session.CountryCode = ""
	session.RegionName = ""
	session.CityName = ""
	session.TimezoneName = ""
	session.IPLatitude = nil
	session.IPLongitude = nil
	repo := &validateAccessSessionRepo{session: session}
	resolver := &validateAccessSessionGeoResolver{
		result: requestmeta.SessionAuditContext{
			GeoSource:    "geoip_api",
			GeoAccuracy:  "ip",
			CountryCode:  "US",
			RegionName:   "California",
			CityName:     "San Francisco",
			TimezoneName: "America/Los_Angeles",
		},
	}
	service := &Service{repo: repo, geoResolver: resolver}

	results, err := service.ListCurrentActiveSessions(context.Background(), 1, "session-id")
	if err != nil {
		t.Fatalf("expected active sessions to load, got %v", err)
	}
	if len(resolver.inputs) != 1 || resolver.inputs[0] != "198.51.100.20" {
		t.Fatalf("expected one lazy lookup for the missing location, got %#v", resolver.inputs)
	}
	if len(results) != 1 || results[0].CountryCode != "US" || results[0].CityName != "San Francisco" {
		t.Fatalf("expected the active session to contain the resolved location, got %+v", results)
	}
	if len(repo.touchInputs) != 1 || repo.touchInputs[0].CountryCode == nil || *repo.touchInputs[0].CountryCode != "US" {
		t.Fatalf("expected the resolved location to be persisted, got %+v", repo.touchInputs)
	}
}
