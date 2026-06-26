package channel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	views, err := s.toUpstreamViews(ctx, items)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
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
	views, err := s.toUpstreamViews(ctx, items)
	if err != nil {
		return nil, 0, err
	}
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

func (s *Service) toUpstreamViews(ctx context.Context, items []repository.ChannelUpstreamListRow) ([]UpstreamView, error) {
	views := make([]UpstreamView, 0, len(items))
	for _, item := range items {
		v := toUpstreamView(item)
		v.APIKeysMasked = s.maskAPIKeysEnc(item.APIKeysEnc)
		v.APIKeyItems = s.maskAPIKeyViewsEnc(item.APIKeysEnc)
		if s.cache != nil {
			v.CircuitOpen, v.CircuitUntil = s.cache.QueryUpstreamCircuitStatus(ctx, item.ID)
		}
		if err := s.normalizeUpstreamAvailability(ctx, &v); err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *Service) normalizeUpstreamAvailability(ctx context.Context, view *UpstreamView) error {
	if view == nil {
		return nil
	}
	if view.Status != "active" || view.CircuitOpen {
		view.ActiveModelsCount = 0
		return nil
	}
	if s.cache == nil || view.ModelsCount <= 0 || view.ActiveModelsCount <= 0 {
		return nil
	}
	activeBindingCodes, err := s.repo.ListActiveRouteBindingCodesForUpstream(ctx, view.ID)
	if err != nil {
		return err
	}
	active := make(map[string]struct{}, view.ActiveModelsCount)
	for _, bindingCode := range activeBindingCodes {
		key := bindingCircuitKey(bindingCode)
		if key == "" {
			continue
		}
		if open, _ := s.cache.QueryModelCircuitStatus(ctx, view.ID, key); !open {
			active[bindingCode] = struct{}{}
		}
	}
	view.ActiveModelsCount = int64(len(active))
	return nil
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
	view.APIKeyItems = s.maskAPIKeyViewsEnc(item.APIKeysEnc)
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
	} else if input.AddAPIKeys != nil || len(input.DeleteAPIKeyIDs) > 0 {
		item, err := s.repo.GetUpstreamByID(ctx, upstreamID)
		if err != nil {
			return nil, err
		}
		rawAPIKeys, err := s.decryptAPIKeys(item.APIKeysEnc)
		if err != nil {
			return nil, ErrInvalidAPIKeysConfig
		}
		nextAPIKeys, err := updateAPIKeysByIDs(rawAPIKeys, input.DeleteAPIKeyIDs, input.AddAPIKeys, s.cfg.Snapshot().DataEncryptionKey)
		if err != nil {
			return nil, ErrInvalidAPIKeysConfig
		}
		apiKeysEnc, err := encryptAPIKeys(s.cfg.Snapshot().DataEncryptionKey, nextAPIKeys)
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
	view, err := s.getUpstreamView(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *Service) getUpstreamView(ctx context.Context, upstreamID uint) (UpstreamView, error) {
	row, err := s.repo.GetUpstreamListRowByID(ctx, upstreamID)
	if err != nil {
		return UpstreamView{}, err
	}
	views, err := s.toUpstreamViews(ctx, []repository.ChannelUpstreamListRow{*row})
	if err != nil {
		return UpstreamView{}, err
	}
	return views[0], nil
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
	ssrfProtectionEnabled := false
	if s != nil && s.cfg != nil {
		cfg := s.cfg.Snapshot()
		env = cfg.Env
		ssrfProtectionEnabled = cfg.SSRFProtectionEnabled
	}
	if err := security.ValidateOutboundHTTPURL(raw, env, ssrfProtectionEnabled); err != nil {
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

func (s *Service) maskAPIKeyViewsEnc(encrypted string) []UpstreamAPIKeyView {
	raw, err := s.decryptAPIKeys(encrypted)
	if err != nil {
		return nil
	}
	return maskAPIKeyViews(raw, s.cfg.Snapshot().DataEncryptionKey)
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
	Key    string `json:"key"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type apiKeysPayload struct {
	Strategy string          `json:"strategy"`
	Keys     []apiKeyPayload `json:"keys"`
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

func apiKeyID(secret string, index int, raw string) string {
	mac := hmac.New(sha256.New, apiKeyIDHMACKey(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d:", index)))
	_, _ = mac.Write([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(mac.Sum(nil))
}

func apiKeyIDHMACKey(secret string) []byte {
	sum := sha256.Sum256([]byte("llm_upstream_api_key_id:" + secret))
	return sum[:]
}

func maskAPIKeyViews(raw string, secret string) []UpstreamAPIKeyView {
	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	results := make([]UpstreamAPIKeyView, 0, len(payload.Keys))
	for index, item := range payload.Keys {
		results = append(results, UpstreamAPIKeyView{
			ID:        apiKeyID(secret, index, item.Key),
			Index:     index,
			KeyMasked: maskSingleKey(item.Key),
			Status:    item.Status,
			Note:      item.Note,
		})
	}
	return results
}

func deleteAPIKeysByIDs(raw string, ids []string, secret string) (string, error) {
	return updateAPIKeysByIDs(raw, ids, nil, secret)
}

func updateAPIKeysByIDs(raw string, ids []string, addRaw *string, secret string) (string, error) {
	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return "", err
	}
	if len(payload.Keys) == 0 {
		return "", fmt.Errorf("api_keys is required")
	}

	deleteSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return "", fmt.Errorf("api key id is invalid")
		}
		deleteSet[id] = struct{}{}
	}
	if len(deleteSet) == 0 && addRaw == nil {
		return strings.TrimSpace(raw), nil
	}

	nextKeys := make([]apiKeyPayload, 0, len(payload.Keys)-len(deleteSet))
	deletedCount := 0
	for index, item := range payload.Keys {
		if _, deleted := deleteSet[apiKeyID(secret, index, item.Key)]; deleted {
			deletedCount += 1
			continue
		}
		nextKeys = append(nextKeys, item)
	}
	if deletedCount != len(deleteSet) {
		return "", fmt.Errorf("api key id is invalid")
	}
	if addRaw != nil {
		addedKeys, err := parseAPIKeyPayloads(*addRaw)
		if err != nil {
			return "", err
		}
		nextKeys = append(nextKeys, addedKeys...)
	}
	if len(nextKeys) == 0 {
		return "", fmt.Errorf("api_keys is required")
	}
	if !hasActiveAPIKey(nextKeys) {
		return "", fmt.Errorf("api_keys is required")
	}
	payload.Keys = nextKeys

	nextRaw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(nextRaw), nil
}

func parseAPIKeyPayloads(raw string) ([]apiKeyPayload, error) {
	if err := validateAPIKeys(raw); err != nil {
		return nil, err
	}
	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return nil, err
	}
	return payload.Keys, nil
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
	if !hasActiveAPIKey(payload.Keys) {
		return fmt.Errorf("api_keys is required")
	}
	return nil
}

func hasActiveAPIKey(keys []apiKeyPayload) bool {
	for _, item := range keys {
		status := strings.TrimSpace(item.Status)
		if (status == "" || status == "active") && strings.TrimSpace(item.Key) != "" {
			return true
		}
	}
	return false
}
