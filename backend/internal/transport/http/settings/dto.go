package settings

import (
	"strings"

	appsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
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
	DefaultNextPath string `json:"defaultNextPath"`
}

type ModelOptionPolicyResponse struct {
	Mode             string                         `json:"mode"`
	AllowedPathsJSON string                         `json:"allowedPathsJSON"`
	DeniedPathsJSON  string                         `json:"deniedPathsJSON"`
	NativeTools      []NativeToolDefinitionResponse `json:"nativeTools"`
}

// NativeToolDefinitionResponse 返回可由后台开启的官方原生工具定义。
type NativeToolDefinitionResponse struct {
	Protocol       string                 `json:"protocol"`
	Provider       string                 `json:"provider"`
	Type           string                 `json:"type"`
	ToolKey        string                 `json:"toolKey"`
	Label          string                 `json:"label"`
	Description    string                 `json:"description"`
	Payload        map[string]interface{} `json:"payload"`
	DefaultEnabled bool                   `json:"defaultEnabled"`
	Billable       bool                   `json:"billable"`
	BillingUnit    string                 `json:"billingUnit"`
	PriceNanousd   int64                  `json:"priceNanousd"`
	PriceLabel     string                 `json:"priceLabel"`
	RiskLevel      string                 `json:"riskLevel"`
	UsageAliases   []string               `json:"usageAliases"`
}

// MCPPolicyResponse 返回聊天侧需要遵守的 MCP 工具运行策略。
type MCPPolicyResponse struct {
	MaxSelectedToolsPerMessage int `json:"maxSelectedToolsPerMessage"`
}

// ChatContextPolicyResponse 返回聊天侧上下文能力策略。
type ChatContextPolicyResponse struct {
	ContextCompactEnabled bool `json:"contextCompactEnabled"`
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

func toNativeToolDefinitionResponses(items []nativetool.Definition) []NativeToolDefinitionResponse {
	results := make([]NativeToolDefinitionResponse, 0, len(items))
	for _, item := range items {
		results = append(results, NativeToolDefinitionResponse{
			Protocol:       item.Protocol,
			Provider:       item.Provider,
			Type:           item.Type,
			ToolKey:        item.Key,
			Label:          item.Label,
			Description:    item.Description,
			Payload:        item.Payload,
			DefaultEnabled: item.DefaultEnabled,
			Billable:       item.Billable,
			BillingUnit:    item.BillingUnit,
			PriceNanousd:   item.PriceNanousd,
			PriceLabel:     item.PriceLabel,
			RiskLevel:      item.RiskLevel,
			UsageAliases:   item.UsageAliases,
		})
	}
	return results
}
