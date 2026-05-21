package settings

import (
	"strings"

	appsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
)

// ── 请求 DTO ─────────────────────────────────────────────────────────────────

// PatchSettingsRequest 批量更新配置请求。
type PatchSettingsRequest struct {
	Items []PatchItem `json:"items" binding:"required,min=1"`
}

// PatchItem 单个更新项请求。
type PatchItem struct {
	Namespace string `json:"namespace" binding:"required"`
	Key       string `json:"key" binding:"required"`
	Value     string `json:"value"`
	Clear     bool   `json:"clear"`
}

// ── 响应 DTO ─────────────────────────────────────────────────────────────────

// SettingResponse 单个配置项响应。
type SettingResponse struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"valueType"`
	Description string `json:"description"`
	Sensitive   bool   `json:"sensitive"`
	Configured  bool   `json:"configured"`
}

type LoginPageSettingsResponse struct {
	Title           string `json:"title"`
	DefaultNextPath string `json:"defaultNextPath"`
}

type ModelOptionPolicyResponse struct {
	Mode             string `json:"mode"`
	AllowedPathsJSON string `json:"allowedPathsJSON"`
	DeniedPathsJSON  string `json:"deniedPathsJSON"`
}

// ── mapping 函数 ─────────────────────────────────────────────────────────────

func toAppPatchItems(items []PatchItem) []appsettings.PatchItem {
	results := make([]appsettings.PatchItem, 0, len(items))
	for _, item := range items {
		results = append(results, appsettings.PatchItem{
			Namespace: item.Namespace,
			Key:       item.Key,
			Value:     item.Value,
			Clear:     item.Clear,
		})
	}
	return results
}

func sanitizePatchItemsForAudit(items []PatchItem) []PatchItem {
	results := make([]PatchItem, 0, len(items))
	for _, item := range items {
		next := item
		if isSensitiveSettingKey(item.Key) {
			if strings.TrimSpace(item.Value) == "" {
				next.Value = ""
			} else {
				next.Value = "[REDACTED]"
			}
		}
		results = append(results, next)
	}
	return results
}

func isSensitiveSettingKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "auth_token") ||
		strings.Contains(normalized, "api_key") ||
		strings.HasSuffix(normalized, "_key")
}

func toSettingResponseList(items []appsettings.SettingItem) []SettingResponse {
	result := make([]SettingResponse, 0, len(items))
	for _, item := range items {
		result = append(result, SettingResponse{
			Key:         item.Key,
			Value:       item.Value,
			ValueType:   item.ValueType,
			Description: item.Description,
			Sensitive:   item.Sensitive,
			Configured:  item.Configured,
		})
	}
	return result
}

func toSettingResponseMap(groups map[string][]appsettings.SettingItem) map[string][]SettingResponse {
	result := make(map[string][]SettingResponse, len(groups))
	for ns, items := range groups {
		result[ns] = toSettingResponseList(items)
	}
	return result
}
