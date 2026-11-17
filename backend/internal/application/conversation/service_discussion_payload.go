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
	Round            int    `json:"round"`
	StepID           string `json:"stepId"`
	Participant      string `json:"participant"`
	Model            string `json:"model"`
	Status           string `json:"status"`
	Content          string `json:"content,omitempty"`
	Error            string `json:"error,omitempty"`
	InputTokens      int64  `json:"inputTokens,omitempty"`
	OutputTokens     int64  `json:"outputTokens,omitempty"`
	CacheReadTokens  int64  `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int64  `json:"cacheWriteTokens,omitempty"`
	ReasoningTokens  int64  `json:"reasoningTokens,omitempty"`
	LatencyMS        int64  `json:"latencyMs,omitempty"`
}

type discussionStatePayload struct {
	Enabled      bool                           `json:"enabled"`
	Status       string                         `json:"status"`
	TemplateID   string                         `json:"templateId"`
	TemplateName string                         `json:"templateName"`
	Placeholders []discussionPlaceholderPayload `json:"placeholders"`
	Rounds       []discussionRoundPayload       `json:"rounds"`
	Summary      discussionSummaryPayload       `json:"summary"`
	Turns        []discussionTurnPayload        `json:"turns"`
	FinalContent string                         `json:"finalContent,omitempty"`
	Error        string                         `json:"error,omitempty"`
	CreatedAt    int64                          `json:"createdAt"`
	UpdatedAt    int64                          `json:"updatedAt"`
}

func discussionStatePayloadOf(state domaindiscussion.State) discussionStatePayload {
	payload := discussionStatePayload{
		Enabled:      state.Enabled,
		Status:       state.Status,
		TemplateID:   state.TemplateID,
		TemplateName: state.TemplateName,
		Summary: discussionSummaryPayload{
			Participant: state.Summary.Participant,
			Model:       state.Summary.Model,
			Prompt:      state.Summary.Prompt,
		},
		Turns:        make([]discussionTurnPayload, 0, len(state.Turns)),
		FinalContent: state.FinalContent,
		Error:        state.Error,
		CreatedAt:    state.CreatedAt,
		UpdatedAt:    state.UpdatedAt,
	}
	for _, item := range state.Placeholders {
		payload.Placeholders = append(payload.Placeholders, discussionPlaceholderPayload{
			Key:          item.Key,
			Label:        item.Label,
			DefaultModel: item.DefaultModel,
			Required:     item.Required,
		})
	}
	for _, round := range state.Rounds {
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
		payload.Rounds = append(payload.Rounds, roundPayload)
	}
	for _, turn := range state.Turns {
		payload.Turns = append(payload.Turns, discussionTurnPayload{
			Round:            turn.Round,
			StepID:           turn.StepID,
			Participant:      turn.Participant,
			Model:            turn.Model,
			Status:           turn.Status,
			Content:          turn.Content,
			Error:            turn.Error,
			InputTokens:      turn.InputTokens,
			OutputTokens:     turn.OutputTokens,
			CacheReadTokens:  turn.CacheReadTokens,
			CacheWriteTokens: turn.CacheWriteTokens,
			ReasoningTokens:  turn.ReasoningTokens,
			LatencyMS:        turn.LatencyMS,
		})
	}
	return payload
}
