package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// ---------------------------------------------------------------------------
// 上游管理
// ---------------------------------------------------------------------------

// ListUpstreamsInput 定义上游列表筛选排序条件。
type ListUpstreamsInput struct {
	Query      string
	Status     string
	Compatible string
	Sort       string
}

// ListUpstreams 分页查询上游列表。
func (s *Service) ListUpstreams(ctx context.Context, page int, pageSize int, input ListUpstreamsInput) ([]UpstreamView, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	if strings.TrimSpace(input.Status) == "circuit" {
		return s.listCircuitOpenUpstreams(ctx, offset, limit, input)
	}
	items, total, err := s.repo.ListUpstreams(ctx, repository.ListChannelUpstreamsInput{
		Offset:     offset,
		Limit:      limit,
		Query:      input.Query,
		Status:     input.Status,
		Compatible: input.Compatible,
		Sort:       input.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	return s.toUpstreamViews(ctx, items), total, nil
}

func (s *Service) listCircuitOpenUpstreams(ctx context.Context, offset int, limit int, input ListUpstreamsInput) ([]UpstreamView, int64, error) {
	items, _, err := s.repo.ListUpstreams(ctx, repository.ListChannelUpstreamsInput{
		Offset:     0,
		Limit:      5000,
		Query:      input.Query,
		Compatible: input.Compatible,
		Sort:       input.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	views := s.toUpstreamViews(ctx, items)
	filtered := make([]UpstreamView, 0, len(views))
	for _, item := range views {
		if item.CircuitOpen {
			filtered = append(filtered, item)
		}
	}
	total := int64(len(filtered))
	if offset >= len(filtered) {
		return []UpstreamView{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], total, nil
}

func (s *Service) toUpstreamViews(ctx context.Context, items []repository.ChannelUpstreamListRow) []UpstreamView {
	views := make([]UpstreamView, 0, len(items))
	for _, item := range items {
		v := toUpstreamView(item)
		v.APIKeysMasked = s.maskAPIKeysEnc(item.APIKeysEnc)
		v.CircuitOpen, v.CircuitUntil = s.cache.QueryUpstreamCircuitStatus(ctx, item.ID)
		views = append(views, v)
	}
	return views
}

// CreateUpstream 创建上游。
func (s *Service) CreateUpstream(ctx context.Context, input CreateUpstreamInput) (*UpstreamView, error) {
	if err := validateOptionalJSON(strings.TrimSpace(input.HeadersJSON)); err != nil {
		return nil, ErrInvalidHeadersConfig
	}
	if err := s.validateUpstreamBaseURL(input.BaseURL); err != nil {
		return nil, err
	}
	if err := validateAPIKeys(input.APIKeys); err != nil {
		return nil, ErrInvalidAPIKeysConfig
	}
	apiKeysEnc, err := encryptAPIKeys(s.cfg.Snapshot().DataEncryptionKey, input.APIKeys)
	if err != nil {
		return nil, ErrInvalidAPIKeysConfig
	}

	compatible := normalizeCompatible(input.Compatible)
	if compatible == "" {
		return nil, ErrInvalidCompatible
	}
	protocolDefaults := strings.TrimSpace(input.ProtocolDefaultsJSON)
	if protocolDefaults == "" {
		protocolDefaults = protocolDefaultsForCompatible(compatible)
	}
	protocolDefaults, err = normalizeProtocolDefaultsJSON(protocolDefaults)
	if err != nil {
		if errors.Is(err, ErrInvalidJSONConfig) {
			return nil, ErrInvalidProtocolDefaultsConfig
		}
		return nil, err
	}

	item := &domainchannel.Upstream{
		Name:                 strings.TrimSpace(input.Name),
		BaseURL:              strings.TrimSpace(input.BaseURL),
		Compatible:           compatible,
		ProtocolDefaultsJSON: protocolDefaults,
		APIKeysEnc:           apiKeysEnc,
		Status:               normalizeStatus(input.Status),
		ConnectTimeoutMS:     normalizeTimeout(input.ConnectTimeoutMS, 10000),
		ReadTimeoutMS:        normalizeTimeout(input.ReadTimeoutMS, 120000),
		StreamIdleTimeoutMS:  normalizeTimeout(input.StreamIdleTimeoutMS, 60000),
		CbFailureThreshold:   input.CbFailureThreshold,
		CbModelThreshold:     input.CbModelThreshold,
		CbThresholdLogic:     normalizeCbLogic(input.CbThresholdLogic),
		CbDurationMin:        input.CbDurationMin,
		CbWindowMin:          input.CbWindowMin,
		HeadersJSON:          strings.TrimSpace(input.HeadersJSON),
	}
	if err := s.repo.CreateUpstream(ctx, item); err != nil {
		return nil, err
	}
	view := toUpstreamView(repository.ChannelUpstreamListRow{Upstream: *item})
	view.APIKeysMasked = s.maskAPIKeysEnc(item.APIKeysEnc)
	return &view, nil
}

// UpdateUpstream 更新上游配置。
func (s *Service) UpdateUpstream(ctx context.Context, upstreamID uint, input UpdateUpstreamInput) (*UpstreamView, error) {
	updateInput := repository.UpdateChannelUpstreamInput{}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		updateInput.Name = &name
	}
	if input.BaseURL != nil {
		baseURL := strings.TrimSpace(*input.BaseURL)
		if err := s.validateUpstreamBaseURL(baseURL); err != nil {
			return nil, err
		}
		updateInput.BaseURL = &baseURL
	}
	if input.Compatible != nil {
		compatible := normalizeCompatible(*input.Compatible)
		if compatible == "" {
			return nil, ErrInvalidCompatible
		}
		updateInput.Compatible = &compatible
	}
	if input.ProtocolDefaultsJSON != nil {
		protocolDefaults := strings.TrimSpace(*input.ProtocolDefaultsJSON)
		if protocolDefaults == "" && input.Compatible != nil {
			protocolDefaults = protocolDefaultsForCompatible(*input.Compatible)
		}
		normalized, err := normalizeProtocolDefaultsJSON(protocolDefaults)
		if err != nil {
			if errors.Is(err, ErrInvalidJSONConfig) {
				return nil, ErrInvalidProtocolDefaultsConfig
			}
			return nil, err
		}
		updateInput.ProtocolDefaultsJSON = &normalized
	}
	if input.APIKeys != nil {
		if err := validateAPIKeys(*input.APIKeys); err != nil {
			return nil, ErrInvalidAPIKeysConfig
		}
		apiKeysEnc, err := encryptAPIKeys(s.cfg.Snapshot().DataEncryptionKey, *input.APIKeys)
		if err != nil {
			return nil, ErrInvalidAPIKeysConfig
		}
		updateInput.APIKeysEnc = &apiKeysEnc
	}
	if input.Status != nil {
		status := normalizeStatus(*input.Status)
		updateInput.Status = &status
	}
	if input.ConnectTimeoutMS != nil {
		connectTimeoutMS := normalizeTimeout(*input.ConnectTimeoutMS, 10000)
		updateInput.ConnectTimeoutMS = &connectTimeoutMS
	}
	if input.ReadTimeoutMS != nil {
		readTimeoutMS := normalizeTimeout(*input.ReadTimeoutMS, 120000)
		updateInput.ReadTimeoutMS = &readTimeoutMS
	}
	if input.StreamIdleTimeoutMS != nil {
		streamIdleTimeoutMS := normalizeTimeout(*input.StreamIdleTimeoutMS, 60000)
		updateInput.StreamIdleTimeoutMS = &streamIdleTimeoutMS
	}
	if input.CbFailureThreshold != nil {
		updateInput.CbFailureThreshold = input.CbFailureThreshold
	}
	if input.CbModelThreshold != nil {
		updateInput.CbModelThreshold = input.CbModelThreshold
	}
	if input.CbThresholdLogic != nil {
		cbThresholdLogic := normalizeCbLogic(*input.CbThresholdLogic)
		updateInput.CbThresholdLogic = &cbThresholdLogic
	}
	if input.CbDurationMin != nil {
		updateInput.CbDurationMin = input.CbDurationMin
	}
	if input.CbWindowMin != nil {
		updateInput.CbWindowMin = input.CbWindowMin
	}
	if input.HeadersJSON != nil {
		if err := validateOptionalJSON(strings.TrimSpace(*input.HeadersJSON)); err != nil {
			return nil, ErrInvalidHeadersConfig
		}
		headersJSON := strings.TrimSpace(*input.HeadersJSON)
		updateInput.HeadersJSON = &headersJSON
	}

	if !updateInput.IsZero() {
		if err := s.repo.UpdateUpstream(ctx, upstreamID, updateInput); err != nil {
			return nil, err
		}
		if input.Status != nil {
			s.InvalidateModelCatalog()
		}
	}

	item, err := s.repo.GetUpstreamByID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	view := toUpstreamView(repository.ChannelUpstreamListRow{Upstream: *item})
	view.APIKeysMasked = s.maskAPIKeysEnc(item.APIKeysEnc)
	return &view, nil
}

// DeleteUpstream 删除上游及其所有路由绑定，保留模型目录。
func (s *Service) DeleteUpstream(ctx context.Context, upstreamID uint) error {
	if err := s.repo.DeleteUpstreamCascade(ctx, upstreamID); err != nil {
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

// BatchDeleteUpstreams 批量删除上游，逐项返回结果。
func (s *Service) BatchDeleteUpstreams(ctx context.Context, upstreamIDs []uint) *BatchDeleteData {
	result := &BatchDeleteData{
		Total:   len(upstreamIDs),
		Results: make([]BatchDeleteResultView, 0, len(upstreamIDs)),
	}

	for _, upstreamID := range upstreamIDs {
		err := s.DeleteUpstream(ctx, upstreamID)
		switch {
		case err == nil:
			result.SuccessCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{
				ID:     upstreamID,
				Status: BatchDeleteStatusDeleted,
			})
		case errors.Is(err, ErrUpstreamNotFound):
			result.NotFoundCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{
				ID:     upstreamID,
				Status: BatchDeleteStatusNotFound,
			})
		default:
			result.FailedCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{
				ID:     upstreamID,
				Status: BatchDeleteStatusFailed,
				Error:  err.Error(),
			})
		}
	}

	return result
}

func (s *Service) validateUpstreamBaseURL(raw string) error {
	env := ""
	if s != nil && s.cfg != nil {
		env = s.cfg.Snapshot().Env
	}
	if err := security.ValidateOutboundHTTPURL(raw, env); err != nil {
		return ErrInvalidUpstreamBaseURL
	}
	return nil
}

// ---------------------------------------------------------------------------
// 上游熔断管理
// ---------------------------------------------------------------------------

// OpenUpstreamCircuit 手动打开上游熔断。
func (s *Service) OpenUpstreamCircuit(ctx context.Context, upstreamID uint) error {
	if _, err := s.repo.GetUpstreamByID(ctx, upstreamID); err != nil {
		return err
	}
	return s.cache.OpenUpstreamCircuit(ctx, upstreamID)
}

// ResetUpstreamCircuit 重置上游熔断与失败计数。
func (s *Service) ResetUpstreamCircuit(ctx context.Context, upstreamID uint) error {
	if _, err := s.repo.GetUpstreamByID(ctx, upstreamID); err != nil {
		return err
	}
	return s.cache.ResetUpstreamCircuit(ctx, upstreamID)
}

// ---------------------------------------------------------------------------
// API Key 配置工具
// ---------------------------------------------------------------------------

func encryptAPIKeys(secret string, raw string) (string, error) {
	return secretbox.EncryptString(secret, raw)
}

func (s *Service) decryptAPIKeys(encrypted string) (string, error) {
	return secretbox.DecryptString(s.cfg.Snapshot().DataEncryptionKey, encrypted)
}

func (s *Service) maskAPIKeysEnc(encrypted string) string {
	raw, err := s.decryptAPIKeys(encrypted)
	if err != nil {
		return ""
	}
	return maskAPIKeys(raw)
}

func (s *Service) parseAPIKeysConfig(encrypted string) (domainchannel.APIKeysConfig, error) {
	raw, err := s.decryptAPIKeys(encrypted)
	if err != nil {
		return domainchannel.APIKeysConfig{}, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return domainchannel.APIKeysConfig{}, nil
	}
	var parsed apiKeysPayload
	if err = json.Unmarshal([]byte(raw), &parsed); err != nil {
		return domainchannel.APIKeysConfig{}, err
	}
	keys := make([]domainchannel.APIKey, 0, len(parsed.Keys))
	for _, item := range parsed.Keys {
		keys = append(keys, domainchannel.APIKey{
			Key:    item.Key,
			Status: item.Status,
			Note:   item.Note,
		})
	}
	return domainchannel.APIKeysConfig{Strategy: parsed.Strategy, Keys: keys}, nil
}

type apiKeyPayload struct {
	Key    string
	Status string
	Note   string
}

type apiKeysPayload struct {
	Strategy string
	Keys     []apiKeyPayload
}

// maskAPIKeys 将密钥配置中的密钥做脱敏处理用于前端展示。
func maskAPIKeys(raw string) string {
	var cfgMap map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cfgMap); err == nil {
		if keysRaw, ok := cfgMap["keys"]; ok {
			if keys, ok := keysRaw.([]interface{}); ok {
				for _, k := range keys {
					if m, ok := k.(map[string]interface{}); ok {
						if v, ok := m["key"].(string); ok {
							m["key"] = maskSingleKey(v)
						}
					}
				}
				cfgMap["keys"] = keys
			}
		}
		b, _ := json.Marshal(cfgMap)
		return string(b)
	}
	return ""
}

func maskSingleKey(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "****" + v[len(v)-4:]
}

// validateAPIKeys 检查 API keys 配置格式是否有效。
func validateAPIKeys(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("api_keys is required")
	}
	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return err
	}
	if len(payload.Keys) == 0 {
		return fmt.Errorf("api_keys is required")
	}
	return nil
}
