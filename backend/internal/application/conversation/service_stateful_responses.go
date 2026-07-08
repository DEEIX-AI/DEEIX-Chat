package conversation

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type statefulResponseDecision struct {
	PreviousResponseID string
	DisabledReason     string
}

func resolveStatefulPreviousResponseID(
	route *channel.ResolvedRoute,
	branchReason string,
	lastResponseID string,
	lastPromptFingerprint string,
	currentPrefixFingerprint string,
) statefulResponseDecision {
	responseID := resolvePreviousResponseID(route, branchReason, lastResponseID)
	if responseID == "" {
		return statefulResponseDecision{DisabledReason: "route_or_branch_not_eligible"}
	}
	storedFingerprint := strings.TrimSpace(lastPromptFingerprint)
	currentFingerprint := strings.TrimSpace(currentPrefixFingerprint)
	if storedFingerprint == "" {
		return statefulResponseDecision{DisabledReason: "missing_stored_fingerprint"}
	}
	if currentFingerprint == "" {
		return statefulResponseDecision{DisabledReason: "missing_current_fingerprint"}
	}
	if storedFingerprint != currentFingerprint {
		return statefulResponseDecision{DisabledReason: "prompt_fingerprint_mismatch"}
	}
	return statefulResponseDecision{PreviousResponseID: responseID}
}

func resolvePreviousResponseID(route *channel.ResolvedRoute, branchReason string, lastResponseID string) string {
	responseID := strings.TrimSpace(lastResponseID)
	if responseID == "" || !strings.EqualFold(strings.TrimSpace(branchReason), "default") {
		return ""
	}
	if !supportsPreviousResponseIDRoute(route) {
		return ""
	}
	return responseID
}

func supportsPreviousResponseIDRoute(route *channel.ResolvedRoute) bool {
	return route != nil &&
		llm.SupportsPreviousResponseID(route.Protocol) &&
		isOfficialOpenAIBaseURL(route.BaseURL)
}

func supportsOpenAIResponsesBackgroundMode(route *channel.ResolvedRoute) bool {
	if route == nil ||
		!strings.EqualFold(strings.TrimSpace(route.Protocol), llm.AdapterOpenAIResponses) ||
		!isOfficialOpenAIBaseURL(route.BaseURL) {
		return false
	}
	capabilities := decodeModelCapabilities(route.ModelCapabilitiesJSON)
	return boolCapability(capabilities, "responsesBackgroundMode") ||
		nestedBoolCapability(capabilities, "responses", "backgroundMode")
}

func decodeModelCapabilities(raw string) map[string]interface{} {
	parsed := make(map[string]interface{})
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return nil
	}
	return parsed
}

func boolCapability(capabilities map[string]interface{}, key string) bool {
	value, ok := capabilities[key]
	if !ok {
		return false
	}
	enabled, ok := value.(bool)
	return ok && enabled
}

func nestedBoolCapability(capabilities map[string]interface{}, parentKey string, childKey string) bool {
	parent, ok := capabilities[parentKey].(map[string]interface{})
	if !ok {
		return false
	}
	enabled, ok := parent[childKey].(bool)
	return ok && enabled
}

func isOfficialOpenAIBaseURL(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.openai.com"
}

func buildStatefulResponseMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == "user" {
			return []llm.Message{messages[index]}
		}
	}
	return nil
}

func applyOpenAIResponsesInstructions(route *channel.ResolvedRoute, endpoint string, input *llm.GenerateInput) {
	if input == nil || endpoint != llm.EndpointResponses || !supportsPreviousResponseIDRoute(route) {
		return
	}
	instructions, messages := extractOpenAIResponsesInstructions(input.Messages)
	if strings.TrimSpace(instructions) == "" {
		return
	}
	input.Instructions = instructions
	input.Messages = messages
}

func extractOpenAIResponsesInstructions(messages []llm.Message) (string, []llm.Message) {
	if len(messages) == 0 {
		return "", nil
	}
	var builder strings.Builder
	result := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		if message.Role != "system" {
			result = append(result, message)
			continue
		}
		text := strings.TrimSpace(systemInstructionText(message))
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(text)
	}
	if builder.Len() == 0 {
		return "", cloneLLMMessages(messages)
	}
	return builder.String(), result
}

func systemInstructionText(message llm.Message) string {
	if strings.TrimSpace(message.Content) != "" || len(message.Parts) == 0 {
		return message.Content
	}
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		if strings.TrimSpace(part.Text) != "" {
			parts = append(parts, strings.TrimSpace(part.Text))
		}
	}
	return strings.Join(parts, "\n\n")
}

func shouldRetryWithoutPreviousResponseID(err error) bool {
	if err == nil {
		return false
	}
	var upstreamErr *llm.UpstreamError
	if !errors.As(err, &upstreamErr) {
		return false
	}
	if upstreamErr.StatusCode != 400 && upstreamErr.StatusCode != 404 && upstreamErr.StatusCode != 422 {
		return false
	}
	text := strings.ToLower(upstreamErr.Message + "\n" + upstreamErr.Body)
	return strings.Contains(text, "previous_response") ||
		strings.Contains(text, "previous response") ||
		strings.Contains(text, "response_id") ||
		strings.Contains(text, "unknown parameter")
}

func shouldRetryWithoutResponsesBackground(err error) bool {
	if err == nil {
		return false
	}
	var upstreamErr *llm.UpstreamError
	if !errors.As(err, &upstreamErr) {
		return false
	}
	if upstreamErr.StatusCode != 400 && upstreamErr.StatusCode != 409 && upstreamErr.StatusCode != 422 {
		return false
	}
	text := strings.ToLower(upstreamErr.Message + "\n" + upstreamErr.Body)
	if !strings.Contains(text, "background") && !strings.Contains(text, "store") &&
		!strings.Contains(text, "zero data retention") && !strings.Contains(text, "zdr") {
		return false
	}
	return strings.Contains(text, "unsupported") ||
		strings.Contains(text, "not supported") ||
		strings.Contains(text, "unknown parameter") ||
		strings.Contains(text, "unrecognized") ||
		strings.Contains(text, "invalid") ||
		strings.Contains(text, "zero data retention") ||
		strings.Contains(text, "zdr")
}
