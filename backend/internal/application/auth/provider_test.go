package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

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

func TestResolveProviderUserAutoLinksVerifiedEmailBeforeProvisioning(t *testing.T) {
	now := time.Now()
	existing := &domainuser.User{
		ID:              42,
		Email:           "verified@example.com",
		EmailVerifiedAt: &now,
		Status:          domainuser.StatusActive,
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
	usersByID                 map[uint]*domainuser.User
	usersByEmail              map[string]*domainuser.User
	credentialsByUserID       map[uint]*domainuser.Credential
	identities                []domainuser.UserIdentity
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

func (r *providerLoginRepo) DeleteAccountHard(ctx context.Context, userID uint) error {
	r.deletedUserID = userID
	return nil
}
