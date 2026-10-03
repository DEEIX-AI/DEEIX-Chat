package conversation

import (
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestMessageSendRunStateAppliesReasoningEffort(t *testing.T) {
	assistant := &model.Message{Role: "assistant"}
	state := &messageSendRunState{run: &model.Run{}, assistantMessage: &assistant}

	state.applyReasoningEffort("high")
	if state.run.ReasoningEffort == nil || *state.run.ReasoningEffort != "high" {
		t.Fatalf("expected run reasoning effort, got %#v", state.run.ReasoningEffort)
	}
	if assistant.ReasoningEffort == nil || *assistant.ReasoningEffort != "high" {
		t.Fatalf("expected assistant reasoning effort, got %#v", assistant.ReasoningEffort)
	}

	// 故障转移到无推理能力的路由后应清空，避免记录未实际生效的档位。
	state.applyReasoningEffort("")
	if state.run.ReasoningEffort != nil || assistant.ReasoningEffort != nil {
		t.Fatalf("expected reasoning effort cleared, got run=%#v assistant=%#v", state.run.ReasoningEffort, assistant.ReasoningEffort)
	}
}
