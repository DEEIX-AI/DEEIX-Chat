package channel

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// 视图转换
// ---------------------------------------------------------------------------

func toUpstreamView(item repository.ChannelUpstreamListRow) UpstreamView {
	return UpstreamView{
		ID:                   item.ID,
		Name:                 item.Name,
		BaseURL:              item.BaseURL,
		Compatible:           item.Compatible,
		ProtocolDefaultsJSON: displayProtocolDefaultsJSON(item.ProtocolDefaultsJSON),
		Status:               item.Status,
		ConnectTimeoutMS:     item.ConnectTimeoutMS,
		ReadTimeoutMS:        item.ReadTimeoutMS,
		StreamIdleTimeoutMS:  item.StreamIdleTimeoutMS,
		CbFailureThreshold:   item.CbFailureThreshold,
		CbModelThreshold:     item.CbModelThreshold,
		CbThresholdLogic:     item.CbThresholdLogic,
		CbDurationMin:        item.CbDurationMin,
		CbWindowMin:          item.CbWindowMin,
		HeadersJSON:          item.HeadersJSON,
		ModelsCount:          item.ModelsCount,
		ActiveModelsCount:    item.ActiveModelsCount,
		CreatedAt:            item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            item.UpdatedAt.Format(time.RFC3339),
	}
}

func displayProtocolDefaultsJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return `{}`
	}
	var payload map[string]*string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload == nil {
		return `{}`
	}

	defaults := make(map[string]string)
	for _, kind := range protocolDefaultKindOrder {
		value, ok := payload[kind]
		if !ok || value == nil {
			continue
		}
		protocol := strings.TrimSpace(strings.ToLower(*value))
		if protocol == "" || !isKnownProtocol(protocol) || !isProtocolAllowedForKind(kind, protocol) {
			continue
		}
		defaults[kind] = protocol
	}
	normalized, _ := json.Marshal(defaults)
	return string(normalized)
}

func toModelView(item repository.ChannelModelListRow) ModelView {
	return ModelView{
		ID:                item.ID,
		PlatformModelName: item.PlatformModelName,
		Vendor:            item.Vendor,
		KindsJSON:         item.KindsJSON,
		Icon:              item.Icon,
		CapabilitiesJSON:  item.CapabilitiesJSON,
		SystemPrompt:      item.SystemPrompt,
		AccessScope:       normalizeModelAccessScopeValue(item.AccessScope),
		Status:            item.Status,
		Description:       item.Description,
		SortOrder:         item.SortOrder,
		SourceCount:       item.SourceCount,
		ActiveSourceCount: item.ActiveSourceCount,
		ProtocolsJSON:     item.ProtocolsJSON,
		CreatedAt:         item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.Format(time.RFC3339),
	}
}

func toUpstreamModelView(item repository.ChannelUpstreamModelListRow) UpstreamModelView {
	return UpstreamModelView{
		ID:                     item.ID,
		RouteID:                item.RouteID,
		UpstreamID:             item.UpstreamID,
		BindingCode:            item.BindingCode,
		PlatformModelID:        item.PlatformModelID,
		PlatformModelName:      item.PlatformModelName,
		ModelVendor:            item.ModelVendor,
		ModelKindsJSON:         item.ModelKindsJSON,
		ModelIcon:              item.ModelIcon,
		UpstreamModelName:      item.UpstreamModelName,
		UpstreamModelVendor:    item.Vendor,
		UpstreamModelIcon:      item.Icon,
		UpstreamModelKindsJSON: item.KindsJSON,
		SuggestedProtocol:      item.SuggestedProtocol,
		Protocol:               item.Protocol,
		UpstreamModelStatus:    item.Status,
		RouteStatus:            item.RouteStatus,
		Priority:               item.Priority,
		Weight:                 item.Weight,
		Source:                 item.RouteSource,
		CbFailureThreshold:     item.CbFailureThreshold,
		CbDurationMin:          item.CbDurationMin,
		CbWindowMin:            item.CbWindowMin,
		HeadersJSON:            item.HeadersJSON,
		CreatedAt:              item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              item.UpdatedAt.Format(time.RFC3339),
	}
}

func toModelUpstreamSourceView(item repository.ChannelModelSourceRow) ModelUpstreamSourceView {
	vendor := normalizeUpstreamModelVendor(item.UpstreamModelVendor, item.UpstreamModelName, item.UpstreamName, item.BaseURL)
	return ModelUpstreamSourceView{
		ID:                     item.ID,
		UpstreamID:             item.UpstreamID,
		UpstreamName:           item.UpstreamName,
		BaseURL:                item.BaseURL,
		BindingCode:            item.BindingCode,
		UpstreamModelName:      item.UpstreamModelName,
		UpstreamModelKindsJSON: item.UpstreamModelKindsJSON,
		UpstreamModelVendor:    vendor,
		UpstreamModelIcon:      normalizeModelIcon(item.UpstreamModelIcon, vendor, item.UpstreamModelName),
		SuggestedProtocol:      item.SuggestedProtocol,
		UpstreamModelStatus:    item.UpstreamModelStatus,
		Protocol:               item.Protocol,
		Status:                 item.Status,
		Priority:               item.Priority,
		Weight:                 item.Weight,
		Source:                 item.Source,
		HeadersJSON:            item.HeadersJSON,
		CreatedAt:              item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              item.UpdatedAt.Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// 规范化辅助
// ---------------------------------------------------------------------------

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	const maxPageSize = 1000
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return offset, pageSize
}

func normalizeStatus(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "active"
	}
	return v
}

func normalizeModelAccessScope(raw string) (string, error) {
	value := normalizeModelAccessScopeValue(raw)
	if value != ModelAccessScopePublic && value != ModelAccessScopeInternal {
		return "", ErrInvalidModelAccessScope
	}
	return value, nil
}

func normalizeModelAccessScopeValue(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ModelAccessScopePublic
	}
	return value
}

func normalizePriority(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func normalizeWeight(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func normalizeTimeout(value int, defaultMS int) int {
	if value <= 0 {
		return defaultMS
	}
	return value
}

func normalizeCbLogic(raw string) string {
	if strings.TrimSpace(raw) == "and" {
		return "and"
	}
	return "or"
}

func normalizeSource(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "manual"
	}
	return v
}

func normalizeProtocol(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "", ErrProtocolRequired
	}
	if !isKnownProtocol(value) {
		return "", ErrInvalidAdapter
	}
	return value, nil
}

func normalizeKindsJSON(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return `["chat"]`, nil
	}
	kinds := parseKinds(v)
	if len(kinds) == 0 && !strings.HasPrefix(v, "[") {
		kinds = []string{v}
	}
	if !validateKinds(kinds) {
		return "", ErrInvalidKinds
	}
	payload, _ := json.Marshal(kinds)
	return string(payload), nil
}

func parseKinds(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var parsed []string
	if strings.HasPrefix(value, "[") && json.Unmarshal([]byte(value), &parsed) == nil {
		return normalizeKindList(parsed)
	}
	return normalizeKindList([]string{value})
}

func normalizeKindList(kinds []string) []string {
	results := make([]string, 0, len(kinds))
	seen := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		value := strings.TrimSpace(strings.ToLower(kind))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		results = append(results, value)
	}
	return results
}

func validateKinds(kinds []string) bool {
	if len(kinds) == 0 {
		return false
	}
	hasPrimary := false
	for _, kind := range kinds {
		switch kind {
		case modelKindChat:
			hasPrimary = true
		case modelKindAudio, modelKindImageGen, modelKindImageEdit, modelKindVideoGen:
			hasPrimary = true
		default:
			return false
		}
	}
	return hasPrimary
}

func canonicalVendorKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "":
		return ""
	case "openai":
		return "openai"
	case "anthropic", "claude":
		return "anthropic"
	case "google", "gemini", "gemma":
		return "google"
	case "meta", "llama":
		return "meta"
	case "microsoft", "phi":
		return "microsoft"
	case "amazon", "aws", "bedrock", "nova", "titan":
		return "amazon"
	case "nvidia", "nemotron":
		return "nvidia"
	case "xai", "grok":
		return "xai"
	case "deepseek":
		return "deepseek"
	case "moonshot", "kimi":
		return "moonshot"
	case "alibaba", "qwen", "qwq", "qvq", "tongyi", "wanx":
		return "alibaba"
	case "mistral", "mixtral", "codestral", "pixtral", "ministral", "devstral":
		return "mistral"
	case "zhipu", "glm", "chatglm", "bigmodel":
		return "zhipu"
	case "minimax", "abab", "hailuo":
		return "minimax"
	case "bytedance", "byte", "volcengine", "doubao", "seed":
		return "bytedance"
	case "tencent", "hunyuan":
		return "tencent"
	case "longcat":
		return "longcat"
	case "xiaomi", "mimo", "xiaomimimo", "xiaomi-mi-mo":
		return "xiaomi"
	case "iflytek", "iflytekcloud", "spark":
		return "iflytek"
	case "stepfun", "step":
		return "stepfun"
	case "baichuan":
		return "baichuan"
	case "baidu", "ernie", "wenxin":
		return "baidu"
	case "openrouter":
		return "openrouter"
	case "copilot", "github":
		return "copilot"
	case "unknown":
		return "unknown"
	default:
		return ""
	}
}

func normalizeModelVendor(raw string, candidates ...string) string {
	if canonical := canonicalVendorKey(raw); canonical != "" {
		return canonical
	}
	if detected := detectModelVendor(candidates...); detected != "" {
		return detected
	}
	return "unknown"
}

func normalizeUpstreamModelVendor(raw string, candidates ...string) string {
	if canonical := canonicalVendorKey(raw); canonical != "" {
		return canonical
	}
	if detected := detectModelVendor(candidates...); detected != "" {
		return detected
	}
	return "unknown"
}

func reasoningContentPassbackRequired(protocol string, candidates ...string) bool {
	if llm.NormalizeAdapter(protocol) != llm.AdapterOpenAIChatCompletions {
		return false
	}
	return detectModelVendor(candidates...) == "deepseek"
}

func detectModelVendor(candidates ...string) string {
	fallback := ""
	for _, candidate := range candidates {
		for _, value := range modelIdentityCandidateValues(candidate) {
			switch {
			case strings.HasPrefix(value, "claude"), strings.Contains(value, "anthropic/"):
				return "anthropic"
			case strings.HasPrefix(value, "nano-banana"), strings.HasPrefix(value, "gemini"), strings.HasPrefix(value, "gemma"), strings.HasPrefix(value, "imagen"),
				strings.HasPrefix(value, "veo"), strings.Contains(value, "google/"):
				return "google"
			case strings.HasPrefix(value, "llama"), strings.Contains(value, "meta/"):
				return "meta"
			case strings.HasPrefix(value, "phi"), strings.Contains(value, "microsoft/"):
				return "microsoft"
			case strings.HasPrefix(value, "nova"), strings.HasPrefix(value, "titan"), strings.Contains(value, "bedrock"),
				strings.Contains(value, "amazon/"), strings.Contains(value, "amazon."), strings.Contains(value, "aws/"), strings.Contains(value, "aws."):
				return "amazon"
			case strings.HasPrefix(value, "nemotron"), strings.Contains(value, "nvidia/"):
				return "nvidia"
			case strings.HasPrefix(value, "grok"), strings.Contains(value, "xai/"):
				return "xai"
			case strings.HasPrefix(value, "deepseek"):
				return "deepseek"
			case strings.HasPrefix(value, "kimi"), strings.Contains(value, "moonshot"), strings.HasPrefix(value, "moonshot"):
				return "moonshot"
			case strings.HasPrefix(value, "qwen"), strings.HasPrefix(value, "qwq"), strings.HasPrefix(value, "qvq"),
				strings.HasPrefix(value, "wanx"), strings.Contains(value, "tongyi"), strings.Contains(value, "alibaba/"):
				return "alibaba"
			case strings.HasPrefix(value, "mistral"), strings.HasPrefix(value, "mixtral"),
				strings.HasPrefix(value, "ministral"), strings.HasPrefix(value, "codestral"),
				strings.HasPrefix(value, "pixtral"), strings.HasPrefix(value, "devstral"),
				strings.Contains(value, "mistral"):
				return "mistral"
			case strings.HasPrefix(value, "glm"), strings.HasPrefix(value, "chatglm"), strings.HasPrefix(value, "cogview"),
				strings.HasPrefix(value, "cogvideo"), strings.Contains(value, "bigmodel"), strings.Contains(value, "zhipu"):
				return "zhipu"
			case strings.HasPrefix(value, "minimax"), strings.HasPrefix(value, "abab"), strings.HasPrefix(value, "hailuo"):
				return "minimax"
			case strings.HasPrefix(value, "doubao"), strings.Contains(value, "bytedance/"), strings.Contains(value, "volcengine/"),
				strings.HasPrefix(value, "seed"):
				return "bytedance"
			case strings.HasPrefix(value, "hunyuan"), strings.Contains(value, "tencent/"):
				return "tencent"
			case strings.HasPrefix(value, "longcat"):
				return "longcat"
			case strings.HasPrefix(value, "mimo"), strings.Contains(value, "xiaomi/"):
				return "xiaomi"
			case strings.HasPrefix(value, "spark"), strings.Contains(value, "iflytek"):
				return "iflytek"
			case strings.HasPrefix(value, "step"), strings.Contains(value, "stepfun"):
				return "stepfun"
			case strings.HasPrefix(value, "baichuan"):
				return "baichuan"
			case strings.HasPrefix(value, "ernie"), strings.HasPrefix(value, "wenxin"), strings.Contains(value, "baidu/"):
				return "baidu"
			case strings.Contains(value, "copilot"), strings.Contains(value, "github/"):
				return "copilot"
			case strings.HasPrefix(value, "gpt-"), strings.HasPrefix(value, "chatgpt"),
				strings.HasPrefix(value, "gpt-image"), strings.HasPrefix(value, "o1"), strings.HasPrefix(value, "o3"),
				strings.HasPrefix(value, "o4"), strings.HasPrefix(value, "codex"),
				strings.HasPrefix(value, "dall-e"), strings.HasPrefix(value, "sora"), strings.Contains(value, "openai/"):
				return "openai"
			case strings.HasPrefix(value, "openrouter/"), strings.HasPrefix(value, "openrouter-"):
				fallback = "openrouter"
			}
		}
	}
	return fallback
}

func resolveVendorIcon(vendor string) string {
	switch canonical := canonicalVendorKey(vendor); canonical {
	case "anthropic":
		return "claude"
	case "google":
		return "gemini"
	case "meta":
		return "meta"
	case "microsoft":
		return "microsoft"
	case "amazon":
		return "aws"
	case "nvidia":
		return "nvidia"
	case "xai":
		return "grok"
	case "deepseek":
		return "deepseek"
	case "moonshot":
		return "moonshot"
	case "alibaba":
		return "alibaba"
	case "mistral":
		return "mistral"
	case "zhipu":
		return "zhipu"
	case "minimax":
		return "minimax"
	case "bytedance":
		return "bytedance"
	case "tencent":
		return "tencent"
	case "longcat":
		return "longcat"
	case "xiaomi":
		return "xiaomimimo"
	case "iflytek":
		return "iflytekcloud"
	case "stepfun":
		return "stepfun"
	case "baichuan":
		return "baichuan"
	case "baidu":
		return "baidu"
	case "openrouter":
		return "openrouter"
	case "copilot":
		return "copilot"
	case "openai":
		return "openai"
	default:
		return ""
	}
}

func detectModelIcon(candidates ...string) string {
	for _, candidate := range candidates {
		for _, value := range modelIdentityCandidateValues(candidate) {
			switch {
			case strings.HasPrefix(value, "claude"):
				return "claude"
			case strings.HasPrefix(value, "nano-banana"), isGeminiImageGenerationModel(value):
				return "nanobanana"
			case strings.HasPrefix(value, "gemma"):
				return "gemma"
			case strings.HasPrefix(value, "gemini"), strings.HasPrefix(value, "imagen"), strings.HasPrefix(value, "veo"):
				return "gemini"
			case strings.HasPrefix(value, "llama"):
				return "meta"
			case strings.HasPrefix(value, "phi"):
				return "microsoft"
			case strings.HasPrefix(value, "nova"):
				return "nova"
			case strings.HasPrefix(value, "titan"), strings.Contains(value, "bedrock"):
				return "bedrock"
			case strings.HasPrefix(value, "nemotron"):
				return "nvidia"
			case strings.HasPrefix(value, "grok"):
				return "grok"
			case strings.HasPrefix(value, "deepseek"):
				return "deepseek"
			case strings.HasPrefix(value, "kimi"), strings.HasPrefix(value, "moonshot"):
				return "kimi"
			case strings.HasPrefix(value, "qwen"), strings.HasPrefix(value, "qwq"), strings.HasPrefix(value, "qvq"), strings.HasPrefix(value, "wanx"):
				return "qwen"
			case strings.HasPrefix(value, "mistral"), strings.HasPrefix(value, "mixtral"), strings.HasPrefix(value, "ministral"),
				strings.HasPrefix(value, "codestral"), strings.HasPrefix(value, "pixtral"), strings.HasPrefix(value, "devstral"):
				return "mistral"
			case strings.HasPrefix(value, "glm"), strings.HasPrefix(value, "chatglm"):
				return "chatglm"
			case strings.HasPrefix(value, "cogview"):
				return "cogview"
			case strings.HasPrefix(value, "minimax"), strings.HasPrefix(value, "abab"):
				return "minimax"
			case strings.HasPrefix(value, "hailuo"):
				return "hailuo"
			case strings.HasPrefix(value, "doubao"), strings.HasPrefix(value, "seed"):
				return "doubao"
			case strings.HasPrefix(value, "hunyuan"):
				return "hunyuan"
			case strings.HasPrefix(value, "longcat"):
				return "longcat"
			case strings.HasPrefix(value, "mimo"):
				return "xiaomimimo"
			case strings.HasPrefix(value, "spark"):
				return "spark"
			case strings.HasPrefix(value, "step"):
				return "stepfun"
			case strings.HasPrefix(value, "baichuan"):
				return "baichuan"
			case strings.HasPrefix(value, "ernie"), strings.HasPrefix(value, "wenxin"):
				return "wenxin"
			case strings.HasPrefix(value, "dall-e"):
				return "dalle"
			case strings.HasPrefix(value, "sora"):
				return "sora"
			case strings.HasPrefix(value, "gpt-"), strings.HasPrefix(value, "chatgpt"), strings.HasPrefix(value, "gpt-image"),
				strings.HasPrefix(value, "o1"), strings.HasPrefix(value, "o3"), strings.HasPrefix(value, "o4"), strings.HasPrefix(value, "codex"):
				return "openai"
			}
		}
	}
	return ""
}

func modelIdentityCandidateValues(raw string) []string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return nil
	}
	values := []string{value}
	if strings.Contains(value, "/") {
		if idx := strings.Index(value, "/"); idx >= 0 && idx+1 < len(value) {
			values = append(values, value[idx+1:])
		}
		if idx := strings.LastIndex(value, "/"); idx >= 0 && idx+1 < len(value) {
			values = append(values, value[idx+1:])
		}
	}
	return values
}

func normalizeModelIcon(raw string, vendor string, candidates ...string) string {
	if v := strings.TrimSpace(raw); v != "" {
		return v
	}
	if icon := detectModelIcon(candidates...); icon != "" {
		return icon
	}
	return resolveVendorIcon(normalizeModelVendor(vendor, candidates...))
}

func shouldRefreshAutoIcon(item *domainchannel.PlatformModel) bool {
	if item == nil || strings.TrimSpace(item.Icon) == "" {
		return true
	}
	expected := normalizeModelIcon("", item.Vendor, item.PlatformModelName)
	return strings.TrimSpace(item.Icon) == expected
}

func normalizePlatformModelName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ErrInvalidPlatformModelName
	}
	if strings.ContainsFunc(value, hasUnsafeModelNameRune) {
		return "", ErrInvalidPlatformModelName
	}
	return value, nil
}

func hasUnsafeModelNameRune(r rune) bool {
	return unicode.IsControl(r) || (unicode.IsSpace(r) && r != ' ')
}

func generateBindingCode() string {
	return "upm_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// defaultUpstreamProtocol 返回默认的上游通信协议。
func defaultUpstreamProtocol() string {
	return llm.AdapterOpenAIChatCompletions
}

func formatOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// JSON 与字符串工具
// ---------------------------------------------------------------------------

func validateOptionalJSON(raw string) error {
	if v := strings.TrimSpace(raw); v != "" {
		var payload json.RawMessage
		return json.Unmarshal([]byte(v), &payload)
	}
	return nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

func mergeHeaderJSON(upstreamHeaders string, routeHeaders string) string {
	merged := make(map[string]interface{})
	mergeIntoJSONMap(strings.TrimSpace(upstreamHeaders), merged)
	mergeIntoJSONMap(strings.TrimSpace(routeHeaders), merged)
	if len(merged) == 0 {
		return ""
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return strings.TrimSpace(upstreamHeaders)
	}
	return string(payload)
}

func mergeIntoJSONMap(raw string, target map[string]interface{}) {
	if target == nil || strings.TrimSpace(raw) == "" {
		return
	}
	parsed := make(map[string]interface{})
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return
	}
	for k, v := range parsed {
		if key := strings.TrimSpace(k); key != "" {
			target[key] = v
		}
	}
}

func truncateMessage(message string, limit int) string {
	v := strings.TrimSpace(message)
	if limit <= 0 || len([]rune(v)) <= limit {
		return v
	}
	return string([]rune(v)[:limit])
}
