// Package personalprovider 实现用户自带 Key 的模型服务：添加、检测、管理与按用户解析路由。
// 个人服务与平台上游完全隔离：只对所属用户可见，请求走强制 SSRF 防护的独立客户端，
// Key 加密保存且没有任何读取明文的接口。
package personalprovider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// probeTimeout 限制一次检测（拉取模型列表）的总耗时。
	probeTimeout = 20 * time.Second
	// probeConnectTimeoutMS 是检测与对话请求的建连超时。
	probeConnectTimeoutMS = 10_000
	// maxLastErrorRunes 限制保存的错误摘要长度，且只保存服务端生成的固定文案。
	maxLastErrorRunes = 200
)

type modelLister interface {
	ListModels(ctx context.Context, route llm.RouteConfig) ([]llm.ModelItem, error)
}

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// Service 封装个人模型服务业务。
type Service struct {
	cfg         *config.Runtime
	repo        repository.PersonalProviderRepository
	llm         modelLister
	auditWriter auditWriter
	logger      *zap.Logger
	now         func() time.Time
}

// NewService 创建服务。
func NewService(cfg *config.Runtime, repo repository.PersonalProviderRepository, llmClient modelLister) *Service {
	return &Service{cfg: cfg, repo: repo, llm: llmClient, logger: zap.NewNop(), now: time.Now}
}

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// SetLogger 注入日志器。
func (s *Service) SetLogger(logger *zap.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

// RequestMeta 描述写审计所需的请求上下文。
type RequestMeta struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

// Access 描述当前用户能否使用个人模型服务。
type Access struct {
	Enabled    bool
	MaxPerUser int
	Protocols  []string
}

// GetAccess 返回当前用户的可用性；功能关闭时 Enabled 为 false。
func (s *Service) GetAccess(ctx context.Context, userID uint) (Access, error) {
	policy := s.policy()
	access := Access{MaxPerUser: policy.MaxPerUser, Protocols: AllowedProtocols()}
	if err := s.requireEnabled(userID, policy); err != nil {
		if errors.Is(err, ErrFeatureDisabled) {
			return access, nil
		}
		return access, err
	}
	access.Enabled = true
	return access, nil
}

// List 返回当前用户的服务。
func (s *Service) List(ctx context.Context, userID uint) ([]domainpersonalprovider.Provider, error) {
	if err := s.requireEnabled(userID, s.policy()); err != nil {
		return nil, err
	}
	return s.repo.ListByOwner(ctx, userID)
}

// CandidateInput 描述待检测的服务配置。
type CandidateInput struct {
	Protocol string
	BaseURL  string
	APIKey   string
}

// Probe 使用候选配置拉取模型列表，不落库；用于添加前确认 Key 与地址可用。
func (s *Service) Probe(ctx context.Context, userID uint, input CandidateInput) ([]string, error) {
	if err := s.requireEnabled(userID, s.policy()); err != nil {
		return nil, err
	}
	candidate, err := s.normalizeCandidate(input)
	if err != nil {
		return nil, err
	}
	return s.fetchModels(ctx, candidate.protocol, candidate.endpoint.BaseURL, candidate.apiKey)
}

// CreateInput 描述新建服务的输入。
type CreateInput struct {
	Name     string
	Icon     string
	Protocol string
	BaseURL  string
	APIKey   string
	Models   []string
	// FromLink 为 true 表示来自一键导入链接，只用于记录来源；功能开启即可导入。
	FromLink bool
}

// Create 校验、检测并保存服务。Key 只在检测通过后加密落库，不会以明文形式返回。
func (s *Service) Create(ctx context.Context, userID uint, input CreateInput, meta RequestMeta) (*domainpersonalprovider.Provider, error) {
	policy := s.policy()
	if err := s.requireEnabled(userID, policy); err != nil {
		return nil, err
	}
	candidate, err := s.normalizeCandidate(CandidateInput{Protocol: input.Protocol, BaseURL: input.BaseURL, APIKey: input.APIKey})
	if err != nil {
		return nil, err
	}
	icon, err := normalizeIcon(input.Icon)
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(input.Name, candidate.endpoint.Host)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= int64(policy.MaxPerUser) {
		return nil, ErrLimitReached
	}
	if err := s.ensureNotDuplicate(ctx, userID, candidate); err != nil {
		return nil, err
	}
	available, err := s.fetchModels(ctx, candidate.protocol, candidate.endpoint.BaseURL, candidate.apiKey)
	if err != nil {
		return nil, err
	}
	models, err := selectModels(input.Models, available)
	if err != nil {
		return nil, err
	}
	publicID := conv.NormalizePublicID(uuid.NewString())
	encrypted, err := s.encryptKey(candidate.apiKey, userID, publicID)
	if err != nil {
		return nil, err
	}
	source := domainpersonalprovider.SourceManual
	if input.FromLink {
		source = domainpersonalprovider.SourceLink
	}
	checkedAt := s.now()
	item, err := s.repo.Create(ctx, &domainpersonalprovider.Provider{
		PublicID:      publicID,
		OwnerUserID:   userID,
		Name:          name,
		Icon:          icon,
		Protocol:      candidate.protocol,
		BaseURL:       candidate.endpoint.BaseURL,
		Host:          candidate.endpoint.Host,
		APIKeyEnc:     encrypted,
		KeyHint:       domainpersonalprovider.KeyHint(candidate.apiKey),
		Models:        models,
		Status:        domainpersonalprovider.StatusActive,
		Source:        source,
		LastCheckedAt: &checkedAt,
	})
	if err != nil {
		return nil, err
	}
	s.audit(ctx, userID, meta, "personal_provider.create", item, map[string]any{
		"source":      source,
		"model_count": len(models),
	})
	return item, nil
}

// UpdateInput 描述可更新字段；nil 表示不修改。地址与协议不可修改，需要删除后重新添加。
type UpdateInput struct {
	Name    *string
	Icon    *string
	APIKey  *string
	Models  *[]string
	Enabled *bool
}

// Update 更新服务。更换 Key 或调整模型时会重新检测，确保保存的配置可用。
func (s *Service) Update(ctx context.Context, userID uint, publicID string, input UpdateInput, meta RequestMeta) (*domainpersonalprovider.Provider, error) {
	if err := s.requireEnabled(userID, s.policy()); err != nil {
		return nil, err
	}
	current, err := s.getOwned(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	patch := repository.PersonalProviderPatch{}
	if input.Name != nil {
		name, err := normalizeName(*input.Name, "")
		if err != nil {
			return nil, err
		}
		patch.Name = &name
	}
	if input.Icon != nil {
		icon, err := normalizeIcon(*input.Icon)
		if err != nil {
			return nil, err
		}
		patch.Icon = &icon
	}
	if input.Enabled != nil {
		if current.Status == domainpersonalprovider.StatusSuspended {
			return nil, ErrSuspended
		}
		status := domainpersonalprovider.StatusDisabled
		if *input.Enabled {
			status = domainpersonalprovider.StatusActive
		}
		patch.Status = &status
	}

	apiKey := ""
	if input.APIKey != nil {
		apiKey, err = normalizeAPIKey(*input.APIKey)
		if err != nil {
			return nil, err
		}
	}
	if apiKey != "" || input.Models != nil {
		// 地址在保存后可能被加入黑名单；重新检测前再按当前策略校验一次。
		if _, err := normalizeBaseURL(current.BaseURL, s.policy()); err != nil {
			return nil, err
		}
		probeKey := apiKey
		if probeKey == "" {
			probeKey, err = s.decryptKey(current)
			if err != nil {
				return nil, err
			}
		}
		available, err := s.fetchModels(ctx, current.Protocol, current.BaseURL, probeKey)
		if err != nil {
			return nil, err
		}
		requested := current.Models
		if input.Models != nil {
			requested = *input.Models
		}
		models, err := selectModels(requested, available)
		if err != nil {
			return nil, err
		}
		patch.Models = &models
		checkedAt := s.now()
		patch.LastCheckedAt = &checkedAt
		empty := ""
		patch.LastError = &empty
	}
	if apiKey != "" {
		encrypted, err := s.encryptKey(apiKey, current.OwnerUserID, current.PublicID)
		if err != nil {
			return nil, err
		}
		hint := domainpersonalprovider.KeyHint(apiKey)
		patch.APIKeyEnc = &encrypted
		patch.KeyHint = &hint
	}
	item, err := s.repo.UpdateByOwner(ctx, userID, publicID, patch)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.audit(ctx, userID, meta, "personal_provider.update", item, map[string]any{
		"key_rotated":     apiKey != "",
		"models_changed":  input.Models != nil,
		"enabled_changed": input.Enabled != nil,
	})
	return item, nil
}

// Delete 删除服务。功能关闭后用户仍可删除自己的服务，以便清除保存的 Key。
func (s *Service) Delete(ctx context.Context, userID uint, publicID string, meta RequestMeta) error {
	current, err := s.getOwned(ctx, userID, publicID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteByOwner(ctx, userID, publicID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	s.audit(ctx, userID, meta, "personal_provider.delete", current, nil)
	return nil
}

// ListAvailableModels 拉取服务当前可用的模型，用于用户调整启用的模型。
func (s *Service) ListAvailableModels(ctx context.Context, userID uint, publicID string) ([]string, error) {
	if err := s.requireEnabled(userID, s.policy()); err != nil {
		return nil, err
	}
	current, err := s.getOwned(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	if _, err := normalizeBaseURL(current.BaseURL, s.policy()); err != nil {
		return nil, err
	}
	apiKey, err := s.decryptKey(current)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchModels(ctx, current.Protocol, current.BaseURL, apiKey)
	s.recordCheck(ctx, userID, publicID, err)
	return models, err
}

// ---------------------------------------------------------------------------
// 运行时：模型目录与路由解析
// ---------------------------------------------------------------------------

// ModelEntry 是一个可供选择的个人模型。
type ModelEntry struct {
	Ref          string
	Model        string
	ProviderID   string
	ProviderName string
	ProviderIcon string
	ProviderHost string
	Protocol     string
}

// ListActiveModels 返回当前用户可用的个人模型；功能关闭或无权限时返回空列表而不是错误，
// 避免影响平台模型目录。
func (s *Service) ListActiveModels(ctx context.Context, userID uint) ([]ModelEntry, error) {
	policy := s.policy()
	if err := s.requireEnabled(userID, policy); err != nil {
		if errors.Is(err, ErrFeatureDisabled) {
			return []ModelEntry{}, nil
		}
		return nil, err
	}
	providers, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	entries := make([]ModelEntry, 0)
	for _, provider := range providers {
		if provider.Status != domainpersonalprovider.StatusActive || policy.isBlockedHost(provider.Host) {
			continue
		}
		for _, model := range provider.Models {
			entries = append(entries, ModelEntry{
				Ref:          domainpersonalprovider.FormatModelRef(provider.PublicID, model),
				Model:        model,
				ProviderID:   provider.PublicID,
				ProviderName: provider.Name,
				ProviderIcon: provider.Icon,
				ProviderHost: provider.Host,
				Protocol:     provider.Protocol,
			})
		}
	}
	return entries, nil
}

// ResolvedRoute 是个人模型的调用配置。APIKey 为明文，只能在内存中传给 LLM 客户端。
type ResolvedRoute struct {
	Ref          string
	Model        string
	ProviderID   string
	ProviderName string
	Protocol     string
	BaseURL      string
	APIKey       string
}

// Resolve 把个人模型引用解析为调用配置。每次调用都会重新校验：功能开关、权限组、归属、
// 服务状态、地址策略与模型是否启用；任一不满足都返回 ErrModelUnavailable，且绝不回退到平台上游。
func (s *Service) Resolve(ctx context.Context, userID uint, ref string) (*ResolvedRoute, error) {
	providerID, model, ok := domainpersonalprovider.ParseModelRef(ref)
	if !ok || userID == 0 {
		return nil, ErrModelUnavailable
	}
	policy := s.policy()
	if err := s.requireEnabled(userID, policy); err != nil {
		if errors.Is(err, ErrFeatureDisabled) {
			return nil, ErrModelUnavailable
		}
		return nil, err
	}
	provider, err := s.repo.GetByOwner(ctx, userID, providerID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrModelUnavailable
		}
		return nil, err
	}
	if provider.Status != domainpersonalprovider.StatusActive || !provider.HasModel(model) {
		return nil, ErrModelUnavailable
	}
	if _, err := normalizeBaseURL(provider.BaseURL, policy); err != nil {
		return nil, ErrModelUnavailable
	}
	apiKey, err := s.decryptKey(provider)
	if err != nil {
		return nil, ErrModelUnavailable
	}
	return &ResolvedRoute{
		Ref:          domainpersonalprovider.FormatModelRef(provider.PublicID, model),
		Model:        model,
		ProviderID:   provider.PublicID,
		ProviderName: provider.Name,
		Protocol:     provider.Protocol,
		BaseURL:      provider.BaseURL,
		APIKey:       apiKey,
	}, nil
}

// ---------------------------------------------------------------------------
// 管理员治理
// ---------------------------------------------------------------------------

// AdminListInput 描述管理员列表查询。
type AdminListInput struct {
	Query       string
	Status      string
	Host        string
	OwnerUserID uint
	Page        int
	PageSize    int
}

// AdminList 分页列出所有用户的服务（不含 Key）。
func (s *Service) AdminList(ctx context.Context, input AdminListInput) ([]domainpersonalprovider.Provider, int64, error) {
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListForAdmin(ctx, repository.PersonalProviderAdminFilter{
		Query:       input.Query,
		Status:      input.Status,
		Host:        input.Host,
		OwnerUserID: input.OwnerUserID,
	}, offset, limit)
}

// AdminSetSuspended 停用或恢复指定服务；停用后用户不能自行启用。
func (s *Service) AdminSetSuspended(ctx context.Context, actorUserID uint, publicIDs []string, suspended bool, meta RequestMeta) (int64, error) {
	status := domainpersonalprovider.StatusActive
	action := "personal_provider.admin_resume"
	if suspended {
		status = domainpersonalprovider.StatusSuspended
		action = "personal_provider.admin_suspend"
	}
	affected, err := s.repo.SetStatusByPublicIDs(ctx, publicIDs, status)
	if err != nil {
		return 0, err
	}
	s.auditAdmin(ctx, actorUserID, meta, action, strings.Join(publicIDs, ","), map[string]any{"affected": affected})
	return affected, nil
}

// AdminSuspendHost 停用某个域名下的全部服务。
func (s *Service) AdminSuspendHost(ctx context.Context, actorUserID uint, host string, meta RequestMeta) (int64, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return 0, ErrInvalidBaseURL
	}
	affected, err := s.repo.SetStatusByHost(ctx, host, domainpersonalprovider.StatusSuspended)
	if err != nil {
		return 0, err
	}
	s.auditAdmin(ctx, actorUserID, meta, "personal_provider.admin_suspend_host", host, map[string]any{"affected": affected})
	return affected, nil
}

// AdminDelete 删除指定服务（连同加密 Key）。
func (s *Service) AdminDelete(ctx context.Context, actorUserID uint, publicIDs []string, meta RequestMeta) (int64, error) {
	affected, err := s.repo.DeleteByPublicIDs(ctx, publicIDs)
	if err != nil {
		return 0, err
	}
	s.auditAdmin(ctx, actorUserID, meta, "personal_provider.admin_delete", strings.Join(publicIDs, ","), map[string]any{"affected": affected})
	return affected, nil
}

// ---------------------------------------------------------------------------
// 内部工具
// ---------------------------------------------------------------------------

func (s *Service) policy() Policy {
	if s.cfg == nil {
		return Policy{}
	}
	return policyFromConfig(s.cfg.Snapshot())
}

func (s *Service) requireEnabled(userID uint, policy Policy) error {
	if userID == 0 || !policy.Enabled {
		return ErrFeatureDisabled
	}
	return nil
}

func (s *Service) getOwned(ctx context.Context, userID uint, publicID string) (*domainpersonalprovider.Provider, error) {
	if !domainpersonalprovider.IsValidPublicID(strings.TrimSpace(publicID)) {
		return nil, ErrNotFound
	}
	item, err := s.repo.GetByOwner(ctx, userID, strings.TrimSpace(publicID))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

type normalizedCandidate struct {
	protocol string
	endpoint normalizedEndpoint
	apiKey   string
}

func (s *Service) normalizeCandidate(input CandidateInput) (normalizedCandidate, error) {
	protocol := strings.TrimSpace(strings.ToLower(input.Protocol))
	if !isAllowedProtocol(protocol) {
		return normalizedCandidate{}, ErrInvalidProtocol
	}
	endpoint, err := normalizeBaseURL(input.BaseURL, s.policy())
	if err != nil {
		return normalizedCandidate{}, err
	}
	apiKey, err := normalizeAPIKey(input.APIKey)
	if err != nil {
		return normalizedCandidate{}, err
	}
	return normalizedCandidate{protocol: protocol, endpoint: endpoint, apiKey: apiKey}, nil
}

// ensureNotDuplicate 拒绝重复添加同一地址的同一个 Key。密文带随机 nonce 无法直接比较，
// 因此只解密同一主机下的已有服务逐一比对（数量受每人上限约束）。
func (s *Service) ensureNotDuplicate(ctx context.Context, userID uint, candidate normalizedCandidate) error {
	existing, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		return err
	}
	for _, item := range existing {
		if item.BaseURL != candidate.endpoint.BaseURL {
			continue
		}
		key, err := s.decryptKey(&item)
		if err == nil && key == candidate.apiKey {
			return ErrDuplicate
		}
	}
	return nil
}

// encryptKey 加密 API Key，并把密文绑定到所属用户与服务：挪到其他记录上的密文无法解密。
func (s *Service) encryptKey(apiKey string, ownerUserID uint, publicID string) (string, error) {
	keyring, err := s.cfg.Snapshot().Keyring()
	if err != nil {
		return "", err
	}
	return keyring.EncryptStringBound(apiKey, domainpersonalprovider.APIKeyBinding(ownerUserID, publicID))
}

func (s *Service) decryptKey(item *domainpersonalprovider.Provider) (string, error) {
	if item == nil {
		return "", ErrModelUnavailable
	}
	keyring, err := s.cfg.Snapshot().Keyring()
	if err != nil {
		return "", ErrModelUnavailable
	}
	key, err := keyring.DecryptStringBound(item.APIKeyEnc, domainpersonalprovider.APIKeyBinding(item.OwnerUserID, item.PublicID))
	if err != nil || strings.TrimSpace(key) == "" {
		return "", ErrModelUnavailable
	}
	return key, nil
}

// fetchModels 通过不受信端点通道拉取模型目录，并把上游错误归类为不含上游原文的固定错误。
func (s *Service) fetchModels(ctx context.Context, protocol string, baseURL string, apiKey string) ([]string, error) {
	if s.llm == nil {
		return nil, ErrUpstreamUnavailable
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	items, err := s.llm.ListModels(probeCtx, llm.RouteConfig{
		Protocol:          protocol,
		BaseURL:           baseURL,
		APIKey:            apiKey,
		ConnectTimeoutMS:  probeConnectTimeoutMS,
		ReadTimeoutMS:     int(probeTimeout / time.Millisecond),
		UntrustedEndpoint: true,
	})
	if err != nil {
		classified := classifyUpstreamError(err)
		s.logger.Info("personal_provider_probe_failed", zap.String("protocol", protocol), zap.Error(redactedError(err)))
		return nil, classified
	}
	seen := make(map[string]struct{}, len(items))
	models := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if !domainpersonalprovider.IsValidModelName(id) {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	sort.Strings(models)
	return models, nil
}

func (s *Service) recordCheck(ctx context.Context, userID uint, publicID string, cause error) {
	checkedAt := s.now()
	// 只保存稳定错误码（前端据此本地化），绝不保存上游原文。
	message := ""
	if cause != nil {
		message = truncateRunes(apperr.Code(cause), maxLastErrorRunes)
		if message == "" {
			message = apperr.Code(ErrUpstreamUnavailable)
		}
	}
	if _, err := s.repo.UpdateByOwner(ctx, userID, publicID, repository.PersonalProviderPatch{
		LastError:     &message,
		LastCheckedAt: &checkedAt,
	}); err != nil {
		s.logger.Warn("personal_provider_record_check_failed", zap.Error(err))
	}
}

// classifyUpstreamError 把上游错误映射为固定错误码；上游响应原文可能包含 Key 片段，不返回给调用方。
func classifyUpstreamError(err error) error {
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		switch upstreamErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrUpstreamRejected
		}
	}
	return ErrUpstreamUnavailable
}

// redactedError 只保留错误类型与状态码，避免把上游响应体（可能回显 Key）写进日志。
func redactedError(err error) error {
	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		return fmt.Errorf("upstream status %d", upstreamErr.StatusCode)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return errors.New("request failed")
}

// selectModels 校验用户选择的模型都在上游目录中；未选择时默认不启用任何模型，由用户显式勾选。
func selectModels(requested []string, available []string) ([]string, error) {
	if len(requested) > maxModelsCount {
		return nil, ErrInvalidModels
	}
	availableSet := make(map[string]struct{}, len(available))
	for _, model := range available {
		availableSet[model] = struct{}{}
	}
	seen := make(map[string]struct{}, len(requested))
	models := make([]string, 0, len(requested))
	for _, raw := range requested {
		model := strings.TrimSpace(raw)
		if !domainpersonalprovider.IsValidModelName(model) {
			return nil, ErrInvalidModels
		}
		if _, ok := availableSet[model]; !ok {
			return nil, ErrInvalidModels
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	return models, nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

// audit 记录用户操作；只记录主机名与协议。不记录 Key 或其打码提示：审计会同时写入应用日志，
// 日志的留存与可见范围都比数据库更广，Key 的任何片段都不应出现在其中。
func (s *Service) audit(ctx context.Context, userID uint, meta RequestMeta, action string, item *domainpersonalprovider.Provider, extra map[string]any) {
	if s.auditWriter == nil || item == nil {
		return
	}
	detail := map[string]any{
		"host":     item.Host,
		"protocol": item.Protocol,
	}
	for key, value := range extra {
		detail[key] = value
	}
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   meta.RequestID,
		ActorUserID: userID,
		Action:      action,
		Resource:    "personal_providers",
		ResourceID:  item.PublicID,
		IP:          meta.ClientIP,
		UserAgent:   meta.UserAgent,
		Detail:      detail,
	})
}

func (s *Service) auditAdmin(ctx context.Context, actorUserID uint, meta RequestMeta, action string, resourceID string, detail map[string]any) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   meta.RequestID,
		ActorUserID: actorUserID,
		Action:      action,
		Resource:    "personal_providers",
		ResourceID:  truncateRunes(resourceID, 255),
		IP:          meta.ClientIP,
		UserAgent:   meta.UserAgent,
		Detail:      detail,
	})
}
