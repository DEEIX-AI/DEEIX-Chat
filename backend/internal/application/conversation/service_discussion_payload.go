package conversation

import domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"

// Unexported transport payloads keep JSON tags out of domain and exported application types.
type discussionPlaceholderPayload struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	DefaultModel string `json:"defaultModel,omitempty"`
	Required     bool   `json:"required,omitempty"`
}

type discussionStepPayload struct {
	ID          string `json:"id"`
	Participant string `json:"participant"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
}

type discussionRoundPayload struct {
	Index int                     `json:"index"`
	Steps []discussionStepPayload `json:"steps"`
}

type discussionSummaryPayload struct {
	Participant string `json:"participant"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
}

type discussionTurnPayload struct {
	Round       int    `json:"round"`
	StepID      string `json:"stepId"`
	Participant string `json:"participant"`
	Model       string `json:"model"`
	Status      string `json:"status"`
	Content     string `json:"content,omitempty"`
	Error       string `json:"error,omitempty"`
	// Prompt is the exact prompt sent to this model; it is debug output, not a summary.
	Prompt           string `json:"prompt,omitempty"`
	InputTokens      int64  `json:"inputTokens,omitempty"`
	OutputTokens     int64  `json:"outputTokens,omitempty"`
	CacheReadTokens  int64  `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int64  `json:"cacheWriteTokens,omitempty"`
	ReasoningTokens  int64  `json:"reasoningTokens,omitempty"`
	LatencyMS        int64  `json:"latencyMs,omitempty"`
}

type discussionStatePayload struct {
	Enabled       bool                           `json:"enabled"`
	Status        string                         `json:"status"`
	TemplateID    string                         `json:"templateId"`
	TemplateName  string                         `json:"templateName"`
	Placeholders  []discussionPlaceholderPayload `json:"placeholders"`
	Rounds        []discussionRoundPayload       `json:"rounds"`
	Summary       discussionSummaryPayload       `json:"summary"`
	SummaryPrompt string                         `json:"summaryPrompt,omitempty"`
	Turns         []discussionTurnPayload        `json:"turns"`
	FinalContent  string                         `json:"finalContent,omitempty"`
	Error         string                         `json:"error,omitempty"`
	CreatedAt     int64                          `json:"createdAt"`
	UpdatedAt     int64                          `json:"updatedAt"`
}

func discussionPlaceholdersPayloadOf(items []domaindiscussion.Placeholder) []discussionPlaceholderPayload {
	if len(items) == 0 {
		return nil
	}
	payload := make([]discussionPlaceholderPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, discussionPlaceholderPayload{
			Key:          item.Key,
			Label:        item.Label,
			DefaultModel: item.DefaultModel,
			Required:     item.Required,
		})
	}
	return payload
}

func discussionRoundsPayloadOf(rounds []domaindiscussion.RuntimeRound) []discussionRoundPayload {
	if len(rounds) == 0 {
		return nil
	}
	payload := make([]discussionRoundPayload, 0, len(rounds))
	for _, round := range rounds {
		roundPayload := discussionRoundPayload{
			Index: round.Index,
			Steps: make([]discussionStepPayload, 0, len(round.Steps)),
		}
		for _, step := range round.Steps {
			roundPayload.Steps = append(roundPayload.Steps, discussionStepPayload{
				ID:          step.ID,
				Participant: step.Participant,
				Model:       step.Model,
				Prompt:      step.Prompt,
			})
		}
		payload = append(payload, roundPayload)
	}
	return payload
}

func discussionSummaryPayloadOf(summary domaindiscussion.RuntimeSummary) discussionSummaryPayload {
	return discussionSummaryPayload{
		Participant: summary.Participant,
		Model:       summary.Model,
		Prompt:      summary.Prompt,
	}
}

func discussionTurnPayloadOf(turn domaindiscussion.Turn) discussionTurnPayload {
	return discussionTurnPayload{
		Round:            turn.Round,
		StepID:           turn.StepID,
		Participant:      turn.Participant,
		Model:            turn.Model,
		Status:           turn.Status,
		Content:          turn.Content,
		Error:            turn.Error,
		Prompt:           turn.Prompt,
		InputTokens:      turn.InputTokens,
		OutputTokens:     turn.OutputTokens,
		CacheReadTokens:  turn.CacheReadTokens,
		CacheWriteTokens: turn.CacheWriteTokens,
		ReasoningTokens:  turn.ReasoningTokens,
		LatencyMS:        turn.LatencyMS,
	}
}

func discussionTurnsPayloadOf(turns []domaindiscussion.Turn) []discussionTurnPayload {
	payload := make([]discussionTurnPayload, 0, len(turns))
	for _, turn := range turns {
		payload = append(payload, discussionTurnPayloadOf(turn))
	}
	return payload
}

func discussionStatePayloadOf(state domaindiscussion.State) discussionStatePayload {
	return discussionStatePayload{
		Enabled:       state.Enabled,
		Status:        state.Status,
		TemplateID:    state.TemplateID,
		TemplateName:  state.TemplateName,
		Placeholders:  discussionPlaceholdersPayloadOf(state.Placeholders),
		Rounds:        discussionRoundsPayloadOf(state.Rounds),
		Summary:       discussionSummaryPayloadOf(state.Summary),
		SummaryPrompt: state.SummaryPrompt,
		Turns:         discussionTurnsPayloadOf(state.Turns),
		FinalContent:  state.FinalContent,
		Error:         state.Error,
		CreatedAt:     state.CreatedAt,
		UpdatedAt:     state.UpdatedAt,
	}
}
