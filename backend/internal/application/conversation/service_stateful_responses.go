package conversation

import (
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
	if route == nil || !llm.SupportsPreviousResponseID(route.Protocol) {
		return ""
	}
	if !isOfficialOpenAIBaseURL(route.BaseURL) {
		return ""
	}
	return responseID
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
