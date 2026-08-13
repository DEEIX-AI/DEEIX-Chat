package contentmoderation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
)

const (
	settingsNamespace = "content_moderation"

	keyEnabled        = "enabled"
	keyBaseURL        = "base_url"
	keyAPIKey         = "api_key"
	keyModel          = "model"
	keyTimeoutSeconds = "timeout_seconds"
	keyMaxConcurrency = "max_concurrency"
	keyQueueCapacity  = "queue_capacity"
	keyPolicyJSON     = "policy_json"
	keyPolicyVersion  = "policy_version"

	defaultBaseURL        = "https://api.openai.com/v1"
	defaultModel          = "omni-moderation-latest"
	defaultTimeoutSeconds = 10
	defaultMaxConcurrency = 4
	defaultQueueCapacity  = 256
)

// ServiceConfig is the saved moderation service configuration.
type ServiceConfig struct {
	Enabled        bool
	BaseURL        string
	APIKeyMasked   string
	HasAPIKey      bool
	Model          string
	TimeoutSeconds int
	MaxConcurrency int
	QueueCapacity  int
	Policy         Policy
}

// UpdateConfigInput is the super-admin PUT body.
type UpdateConfigInput struct {
	Enabled        *bool
	BaseURL        *string
	APIKey         *string
	ClearAPIKey    bool
	Model          *string
	TimeoutSeconds *int
	MaxConcurrency *int
	QueueCapacity  *int
	Policy         *Policy
}

type runtimeConfig struct {
	Enabled        bool
	BaseURL        string
	APIKey         string
	Model          string
	Timeout        time.Duration
	MaxConcurrency int
	QueueCapacity  int
	Policy         Policy
}

func (s *Service) loadRuntimeConfig(ctx context.Context) (runtimeConfig, error) {
	s.configMu.RLock()
	if s.cachedConfig != nil && time.Since(s.cachedAt) < 2*time.Second {
		cfg := *s.cachedConfig
		s.configMu.RUnlock()
		return cfg, nil
	}
	s.configMu.RUnlock()

	cfg, err := s.readRuntimeConfig(ctx)
	if err != nil {
		return runtimeConfig{}, err
	}
	s.configMu.Lock()
	s.cachedConfig = &cfg
	s.cachedAt = time.Now()
	s.configMu.Unlock()
	return cfg, nil
}

func (s *Service) invalidateConfigCache() {
	s.configMu.Lock()
	s.cachedConfig = nil
	s.configMu.Unlock()
}

func (s *Service) readRuntimeConfig(ctx context.Context) (runtimeConfig, error) {
	items, err := s.settingsRepo.ListByNamespace(ctx, settingsNamespace)
	if err != nil {
		return runtimeConfig{}, err
	}
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.Key] = item.Value
	}

	apiKey := ""
	if encrypted := strings.TrimSpace(values[keyAPIKey]); encrypted != "" {
		decrypted, decErr := secretbox.DecryptString(s.dataEncryptionKey, encrypted)
		if decErr != nil {
			return runtimeConfig{}, decErr
		}
		apiKey = decrypted
	}

	policy := Policy{}
	if raw := strings.TrimSpace(values[keyPolicyJSON]); raw != "" {
		var policyDocument policyJSON
		if err := json.Unmarshal([]byte(raw), &policyDocument); err != nil {
			return runtimeConfig{}, err
		}
		policy = policyDocument.toPolicy()
	}
	policy.Version = parseInt64(values[keyPolicyVersion], 0)
	policy, err = NormalizePolicy(policy)
	if err != nil {
		return runtimeConfig{}, err
	}
	enabled := false
	if raw, exists := values[keyEnabled]; exists {
		if parsed, parseErr := strconv.ParseBool(strings.TrimSpace(raw)); parseErr == nil {
			enabled = parsed
		}
	}

	timeoutSec := parseInt(values[keyTimeoutSeconds], defaultTimeoutSeconds)
	if timeoutSec < 1 {
		timeoutSec = defaultTimeoutSeconds
	}
	maxConc := parseInt(values[keyMaxConcurrency], defaultMaxConcurrency)
	if maxConc < 1 {
		maxConc = defaultMaxConcurrency
	}
	queueCap := parseInt(values[keyQueueCapacity], defaultQueueCapacity)
	if queueCap < 1 {
		queueCap = defaultQueueCapacity
	}
	baseURL := strings.TrimSpace(values[keyBaseURL])
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := strings.TrimSpace(values[keyModel])
	if model == "" {
		model = defaultModel
	}

	return runtimeConfig{
		Enabled:        enabled,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          model,
		Timeout:        time.Duration(timeoutSec) * time.Second,
		MaxConcurrency: maxConc,
		QueueCapacity:  queueCap,
		Policy:         policy,
	}, nil
}

// GetConfig returns masked config for super-admin UI.
func (s *Service) GetConfig(ctx context.Context, actorRole string) (*ServiceConfig, error) {
	if !isSuperAdmin(actorRole) {
		return nil, ErrSuperAdminRequired
	}
	cfg, err := s.readRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}
	return toServiceConfig(cfg), nil
}

// UpdateConfig atomically saves configuration.
func (s *Service) UpdateConfig(ctx context.Context, actorRole string, input UpdateConfigInput) (*ServiceConfig, error) {
	if !isSuperAdmin(actorRole) {
		return nil, ErrSuperAdminRequired
	}
	current, err := s.readRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}
	next := current
	if input.Enabled != nil {
		next.Enabled = *input.Enabled
	}
	if input.BaseURL != nil {
		base := strings.TrimSpace(*input.BaseURL)
		if base == "" {
			base = defaultBaseURL
		}
		if s.provider == nil || s.provider.ValidateBaseURL(base) != nil {
			return nil, ErrInvalidBaseURL
		}
		next.BaseURL = base
	}
	if input.ClearAPIKey {
		next.APIKey = ""
	} else if input.APIKey != nil {
		next.APIKey = strings.TrimSpace(*input.APIKey)
	}
	if input.Model != nil {
		model := strings.TrimSpace(*input.Model)
		if model == "" {
			return nil, ErrInvalidModel
		}
		next.Model = model
	}
	if input.TimeoutSeconds != nil {
		if *input.TimeoutSeconds < 1 || *input.TimeoutSeconds > 60 {
			return nil, ErrInvalidTimeout
		}
		next.Timeout = time.Duration(*input.TimeoutSeconds) * time.Second
	}
	if input.MaxConcurrency != nil {
		if *input.MaxConcurrency < 1 || *input.MaxConcurrency > 64 {
			return nil, ErrInvalidConcurrency
		}
		next.MaxConcurrency = *input.MaxConcurrency
	}
	if input.QueueCapacity != nil {
		if *input.QueueCapacity < 1 || *input.QueueCapacity > 4096 {
			return nil, ErrInvalidQueueCapacity
		}
		next.QueueCapacity = *input.QueueCapacity
	}
	if input.Policy != nil {
		policy, normErr := NormalizePolicy(*input.Policy)
		if normErr != nil {
			return nil, normErr
		}
		if !policyEqualCategories(current.Policy, policy) {
			policy.Version = current.Policy.Version + 1
		} else {
			policy.Version = current.Policy.Version
		}
		next.Policy = policy
	}
	if next.Enabled {
		if !next.Policy.Enabled() || strings.TrimSpace(next.BaseURL) == "" || strings.TrimSpace(next.Model) == "" || strings.TrimSpace(next.APIKey) == "" {
			return nil, ErrServiceConfigRequired
		}
		if s.provider == nil || s.provider.ValidateBaseURL(next.BaseURL) != nil {
			return nil, ErrInvalidBaseURL
		}
	}

	items, err := buildSettingItems(next, s.dataEncryptionKey)
	if err != nil {
		return nil, err
	}
	if err := s.settingsRepo.Upsert(ctx, items); err != nil {
		return nil, err
	}
	s.invalidateConfigCache()
	s.resizeWorker(next.MaxConcurrency, next.QueueCapacity)
	return toServiceConfig(next), nil
}

func policyEqualCategories(a, b Policy) bool {
	return stringSlicesEqual(a.InputTextCategories, b.InputTextCategories) &&
		stringSlicesEqual(a.OutputTextCategories, b.OutputTextCategories) &&
		stringSlicesEqual(a.InputImageCategories, b.InputImageCategories) &&
		stringSlicesEqual(a.OutputImageCategories, b.OutputImageCategories)
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toServiceConfig(cfg runtimeConfig) *ServiceConfig {
	return &ServiceConfig{
		Enabled:        cfg.Enabled,
		BaseURL:        cfg.BaseURL,
		APIKeyMasked:   maskAPIKey(cfg.APIKey),
		HasAPIKey:      strings.TrimSpace(cfg.APIKey) != "",
		Model:          cfg.Model,
		TimeoutSeconds: int(cfg.Timeout / time.Second),
		MaxConcurrency: cfg.MaxConcurrency,
		QueueCapacity:  cfg.QueueCapacity,
		Policy:         cfg.Policy,
	}
}

func buildSettingItems(cfg runtimeConfig, encryptionKey string) ([]domainsettings.SystemSetting, error) {
	policyJSON, err := json.Marshal(newPolicyJSON(cfg.Policy))
	if err != nil {
		return nil, err
	}
	encryptedKey := ""
	if strings.TrimSpace(cfg.APIKey) != "" {
		encryptedKey, err = secretbox.EncryptString(encryptionKey, cfg.APIKey)
		if err != nil {
			return nil, err
		}
	}
	return []domainsettings.SystemSetting{
		{Namespace: settingsNamespace, Key: keyEnabled, Value: strconv.FormatBool(cfg.Enabled), ValueType: "bool"},
		{Namespace: settingsNamespace, Key: keyBaseURL, Value: cfg.BaseURL, ValueType: "string"},
		{Namespace: settingsNamespace, Key: keyAPIKey, Value: encryptedKey, ValueType: "string"},
		{Namespace: settingsNamespace, Key: keyModel, Value: cfg.Model, ValueType: "string"},
		{Namespace: settingsNamespace, Key: keyTimeoutSeconds, Value: strconv.Itoa(int(cfg.Timeout / time.Second)), ValueType: "int"},
		{Namespace: settingsNamespace, Key: keyMaxConcurrency, Value: strconv.Itoa(cfg.MaxConcurrency), ValueType: "int"},
		{Namespace: settingsNamespace, Key: keyQueueCapacity, Value: strconv.Itoa(cfg.QueueCapacity), ValueType: "int"},
		{Namespace: settingsNamespace, Key: keyPolicyJSON, Value: string(policyJSON), ValueType: "json"},
		{Namespace: settingsNamespace, Key: keyPolicyVersion, Value: strconv.FormatInt(cfg.Policy.Version, 10), ValueType: "int"},
	}, nil
}

func parseInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func parseInt64(raw string, fallback int64) int64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func isSuperAdmin(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "superadmin")
}

func isAdminRole(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == "admin" || r == "superadmin"
}
