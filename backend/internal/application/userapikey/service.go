package userapikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	domainuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/userapikey"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/google/uuid"
)

var (
	ErrDisabled       = apperr.New("auth.user_api_keys_disabled", "user API keys are disabled")
	ErrInvalidKey     = apperr.New("auth.invalid_api_key", "invalid API key")
	ErrKeyNotFound    = apperr.New("auth.api_key_not_found", "API key not found")
	ErrInvalidName    = apperr.New("auth.invalid_api_key_name", "invalid API key name")
	ErrUserInactive   = apperr.New("auth.user_inactive", "user is inactive")
	ErrKeyLimitReached = apperr.New("auth.api_key_limit_reached", "API key limit reached")
)

const (
	maxAPIKeysPerUser = 20
	rawKeyBytes       = 32
	keyPrefixDisplay  = 11 // "sk-" + 8 chars
)

type runtimeConfig interface {
	Snapshot() config.Config
}

type userLookup interface {
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
}

// Service 封装用户 API Key 业务。
type Service struct {
	repo   repository.UserAPIKeyRepository
	users  userLookup
	cfg    runtimeConfig
}

// NewService 创建服务。
func NewService(repo repository.UserAPIKeyRepository, users userLookup, cfg runtimeConfig) *Service {
	return &Service{repo: repo, users: users, cfg: cfg}
}

// CreateResult 创建时仅返回一次的明文密钥。
type CreateResult struct {
	Key       domainuserapikey.UserAPIKey
	Plaintext string
}

// Create 创建新的 API Key。
func (s *Service) Create(ctx context.Context, userID uint, name string) (*CreateResult, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || len(normalizedName) > 128 {
		return nil, ErrInvalidName
	}
	existing, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	activeCount := 0
	for _, item := range existing {
		if item.RevokedAt == nil {
			activeCount++
		}
	}
	if activeCount >= maxAPIKeysPerUser {
		return nil, ErrKeyLimitReached
	}
	plaintext, err := generateRawKey()
	if err != nil {
		return nil, err
	}
	item := &domainuserapikey.UserAPIKey{
		PublicID:  conv.NormalizePublicID(uuid.NewString()),
		UserID:    userID,
		Name:      normalizedName,
		KeyPrefix: plaintext[:keyPrefixDisplay],
		KeyHash:   hashKey(plaintext),
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return &CreateResult{Key: *item, Plaintext: plaintext}, nil
}

// List 列出当前用户的 API Key（不含明文）。
func (s *Service) List(ctx context.Context, userID uint) ([]domainuserapikey.UserAPIKey, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	return s.repo.ListByUserID(ctx, userID)
}

// Revoke 吊销 API Key。
func (s *Service) Revoke(ctx context.Context, userID uint, publicID string) error {
	if err := s.requireEnabled(); err != nil {
		return err
	}
	normalized := conv.NormalizePublicID(publicID)
	if normalized == "" {
		return ErrKeyNotFound
	}
	if err := s.repo.Revoke(ctx, userID, normalized, time.Now()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrKeyNotFound
		}
		return err
	}
	return nil
}

// Authenticate 校验 Bearer sk-… 并返回所属用户。
func (s *Service) Authenticate(ctx context.Context, rawKey string) (*domainuserapikey.AuthenticatedKey, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	normalized := strings.TrimSpace(rawKey)
	if !strings.HasPrefix(normalized, "sk-") || len(normalized) < keyPrefixDisplay+8 {
		return nil, ErrInvalidKey
	}
	item, err := s.repo.GetActiveByHash(ctx, hashKey(normalized))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidKey
		}
		return nil, err
	}
	if s.users != nil {
		user, userErr := s.users.GetByID(ctx, item.UserID)
		if userErr != nil || user == nil || user.Status != domainuser.StatusActive {
			return nil, ErrUserInactive
		}
	}
	touchCtx, cancel := background.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = s.repo.TouchLastUsed(touchCtx, item.ID, time.Now())
	return &domainuserapikey.AuthenticatedKey{Key: *item, UserID: item.UserID}, nil
}

func (s *Service) requireEnabled() error {
	if s == nil || s.cfg == nil {
		return ErrDisabled
	}
	if !s.cfg.Snapshot().UserAPIKeysEnabled {
		return ErrDisabled
	}
	return nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func generateRawKey() (string, error) {
	buf := make([]byte, rawKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sk-" + base64.RawURLEncoding.EncodeToString(buf), nil
}
