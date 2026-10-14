package conversation

import (
	"fmt"
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func uiStateBranch() []model.Message {
	return []model.Message{
		{ID: 1, PublicID: "msg_user_1", Role: "user", Content: "算一下房贷", Status: "success"},
		{ID: 2, PublicID: "msg_bot_1", Role: "assistant", Content: "计算器", Status: "success"},
		{ID: 3, PublicID: "msg_user_2", Role: "user", Content: "再出个测验", Status: "success"},
		{ID: 4, PublicID: "msg_bot_2", Role: "assistant", Content: "测验", Status: "success"},
		{ID: 5, PublicID: "msg_user_3", Role: "user", Content: "按我现在的设置算", Status: "pending"},
	}
}

func TestResolveBranchUIStatesKeepsOnlyCurrentBranchAssistantMessages(t *testing.T) {
	cfg := config.Config{UIComponentsEnabled: true}
	got := resolveBranchUIStates(cfg, uiStateBranch(), []MessageUIStateInput{
		{MessagePublicID: "msg_bot_2", BlockID: "quiz", Title: "小测验", StateJSON: `{"answers":[1,0]}`},
		{MessagePublicID: "msg_bot_1", BlockID: "loan", Title: "房贷计算器", StateJSON: `{ "amount": 300 }`},
		// 其它分支或他人会话的消息：不在本轮历史里，丢弃。
		{MessagePublicID: "msg_other_branch", BlockID: "loan2", StateJSON: `{"amount":1}`},
		// 用户消息上不会有组件。
		{MessagePublicID: "msg_user_1", BlockID: "fake", StateJSON: `{"amount":1}`},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 blocks, got %#v", got)
	}
	// 结果按时间正序：先出现的组件在前。
	if got[0].BlockID != "loan" || got[1].BlockID != "quiz" {
		t.Fatalf("unexpected order: %#v", got)
	}
	if got[0].StateJSON != `{"amount":300}` {
		t.Fatalf("state should be normalized, got %q", got[0].StateJSON)
	}
	if got[0].MessageRef != "msg_bot_1" || got[0].Title != "房贷计算器" {
		t.Fatalf("unexpected block: %#v", got[0])
	}
}

func TestResolveBranchUIStatesSkipsWhenComponentsDisabled(t *testing.T) {
	inputs := []MessageUIStateInput{{MessagePublicID: "msg_bot_1", BlockID: "loan", StateJSON: `{"amount":300}`}}
	if got := resolveBranchUIStates(config.Config{UIComponentsEnabled: false}, uiStateBranch(), inputs); got != nil {
		t.Fatalf("disabled components must not inject ui state, got %#v", got)
	}
}

func TestResolveBranchUIStatesSkipsBlockedMessages(t *testing.T) {
	branch := uiStateBranch()
	branch[1].Status = "blocked"
	got := resolveBranchUIStates(config.Config{UIComponentsEnabled: true}, branch, []MessageUIStateInput{
		{MessagePublicID: "msg_bot_1", BlockID: "loan", StateJSON: `{"amount":300}`},
	})
	if got != nil {
		t.Fatalf("blocked message is not part of the prompt, got %#v", got)
	}
}

func TestSelectTurnUIStatesDropsInvalidEntries(t *testing.T) {
	got := selectTurnUIStates([]uiStateCandidate{
		{Order: 1, BlockID: "bad id", StateJSON: `{"v":1}`},
		{Order: 2, BlockID: "empty", StateJSON: `{}`},
		{Order: 3, BlockID: "array", StateJSON: `[1]`},
		{Order: 4, BlockID: "badkey", StateJSON: `{"</ui_state>":1}`},
		{Order: 5, BlockID: "ok", Title: "  多行\n标题\t ", StateJSON: `{"v":1}`},
	})
	if len(got) != 1 || got[0].BlockID != "ok" {
		t.Fatalf("expected only the valid block, got %#v", got)
	}
	if got[0].Title != "多行 标题" {
		t.Fatalf("title should be collapsed to one line, got %q", got[0].Title)
	}
}

func TestSelectTurnUIStatesPrefersMostRecentAndCapsCount(t *testing.T) {
	candidates := make([]uiStateCandidate, 0, maxTurnUIStateBlocks+3)
	for index := 0; index < maxTurnUIStateBlocks+3; index++ {
		candidates = append(candidates, uiStateCandidate{
			Order:     index,
			BlockID:   fmt.Sprintf("block-%d", index),
			StateJSON: `{"v":1}`,
		})
	}
	got := selectTurnUIStates(candidates)
	if len(got) != maxTurnUIStateBlocks {
		t.Fatalf("expected %d blocks, got %d", maxTurnUIStateBlocks, len(got))
	}
	// 最早的 3 个被挤掉，保留下来的仍按时间正序。
	if got[0].BlockID != "block-3" || got[len(got)-1].BlockID != fmt.Sprintf("block-%d", maxTurnUIStateBlocks+2) {
		t.Fatalf("unexpected selection: first=%s last=%s", got[0].BlockID, got[len(got)-1].BlockID)
	}
}

func TestSelectTurnUIStatesKeepsLatestForDuplicatedBlockID(t *testing.T) {
	got := selectTurnUIStates([]uiStateCandidate{
		{Order: 1, MessageRef: "old", BlockID: "loan", StateJSON: `{"amount":100}`},
		{Order: 3, MessageRef: "new", BlockID: "loan", StateJSON: `{"amount":300}`},
	})
	if len(got) != 1 || got[0].MessageRef != "new" || got[0].StateJSON != `{"amount":300}` {
		t.Fatalf("expected the most recent block to win, got %#v", got)
	}
}

func TestSelectTurnUIStatesRespectsByteBudget(t *testing.T) {
	large := `{"v":"` + strings.Repeat("x", maxUIStateStateBytes-64) + `"}`
	got := selectTurnUIStates([]uiStateCandidate{
		{Order: 1, BlockID: "small", StateJSON: `{"v":1}`},
		{Order: 2, BlockID: "large-old", StateJSON: large},
		{Order: 3, BlockID: "large-new", StateJSON: large},
	})
	// 两个大块放不下：保留最近的那个，再往前找还放得下的小块。
	if len(got) != 2 || got[0].BlockID != "small" || got[1].BlockID != "large-new" {
		ids := make([]string, 0, len(got))
		for _, block := range got {
			ids = append(ids, block.BlockID)
		}
		t.Fatalf("unexpected selection: %v", ids)
	}
}

func TestTruncatedUIStateTitle(t *testing.T) {
	got := selectTurnUIStates([]uiStateCandidate{
		{Order: 1, BlockID: "loan", Title: strings.Repeat("标", maxUIStateTitleRunes+10), StateJSON: `{"v":1}`},
	})
	if len(got) != 1 || len([]rune(got[0].Title)) != maxUIStateTitleRunes {
		t.Fatalf("title should be truncated to %d runes, got %d", maxUIStateTitleRunes, len([]rune(got[0].Title)))
	}
}

func TestResolveTemporaryUIStatesLocatesAssistantMessageByIndex(t *testing.T) {
	messages := []TemporaryChatMessage{
		{Role: "user", Content: "算一下"},
		{Role: "assistant", Content: "计算器"},
		{Role: "user", Content: "按现在的设置"},
	}
	got := resolveTemporaryUIStates(config.Config{UIComponentsEnabled: true}, messages, []TemporaryChatUIStateInput{
		{MessageIndex: 1, BlockID: "loan", StateJSON: `{"amount":300}`},
		{MessageIndex: 0, BlockID: "on-user", StateJSON: `{"v":1}`},
		{MessageIndex: 9, BlockID: "out-of-range", StateJSON: `{"v":1}`},
		{MessageIndex: -1, BlockID: "negative", StateJSON: `{"v":1}`},
	})
	if len(got) != 1 || got[0].BlockID != "loan" || got[0].MessageRef != "1" {
		t.Fatalf("unexpected temporary ui states: %#v", got)
	}
}

func TestInjectUserContextIncludesUIState(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "按我现在的设置算"}}
	got := injectUserContext(t.Context(), messages, userContextInput{UIStates: []uiStateContextBlock{
		{MessageRef: "msg_bot_1", BlockID: "loan", Title: `房贷 "计算器" <b>`, StateJSON: `{"note":"a<b>&c","amount":300}`},
	}}, config.Config{}, nil)

	question, dynamic := splitUserContextParts(t, got[0])
	if question != "按我现在的设置算" {
		t.Fatalf("question must stay untouched, got %q", question)
	}
	for _, want := range []string{
		"<ui_state>\n<notice>" + uiStateContextNotice + "</notice>\n",
		`<block id="loan" title="房贷 &#34;计算器&#34; &lt;b&gt;">`,
		`{"note":"a&lt;b&gt;&amp;c","amount":300}</block>`,
		"</ui_state>\n</ctx>",
	} {
		if !strings.Contains(dynamic, want) {
			t.Fatalf("expected dynamic context to contain %q, got %q", want, dynamic)
		}
	}
	// 用户输入的文本不能以原样标签的形式出现在上下文里。
	if strings.Contains(dynamic, "a<b>") {
		t.Fatalf("state text must be escaped, got %q", dynamic)
	}
}

func TestDynamicContextSourceRefsIncludeUIState(t *testing.T) {
	refs := dynamicContextSourceRefs(userContextInput{UIStates: []uiStateContextBlock{
		{MessageRef: "msg_bot_1", BlockID: "loan", Title: "房贷计算器", StateJSON: `{"amount":300}`},
		{BlockID: "quiz", StateJSON: `{"answers":[1]}`},
	}})
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %#v", refs)
	}
	if refs[0].SourceType != promptSourceUIState || refs[0].SourceID != "msg_bot_1#loan" || refs[0].Title != "房贷计算器" {
		t.Fatalf("unexpected ref: %#v", refs[0])
	}
	// 没有标题时用块 id 兜底，面板里总能看到是哪个组件。
	if refs[1].SourceID != "quiz" || refs[1].Title != "quiz" {
		t.Fatalf("unexpected ref: %#v", refs[1])
	}
}

// 只有组件状态、没有其它动态上下文时，本轮仍应生成动态上下文块并登记来源，
// 否则上下文面板看不到这一轮带了界面状态。
func TestBuildPromptPlanTracesUIStateAsDynamicContext(t *testing.T) {
	plan := buildPromptPlan(t.Context(), promptPlanInput{
		BaseMessages: []llm.Message{
			{Role: "user", Content: "算一下房贷"},
			{Role: "assistant", Content: "计算器"},
			{Role: "user", Content: "按我现在的设置算"},
		},
		DynamicContext: userContextInput{UIStates: []uiStateContextBlock{
			{MessageRef: "msg_bot_1", BlockID: "loan", Title: "房贷计算器", StateJSON: `{"amount":300}`},
		}},
		Config: config.Config{},
	})

	block := promptTraceBlock(plan.Trace, PromptBlockDynamicContext)
	if block == nil {
		t.Fatalf("expected a dynamic context block, got %#v", plan.Trace.Blocks)
	}
	if block.SourceCount != 1 || len(block.SourceRefs) != 1 || block.SourceRefs[0].SourceType != promptSourceUIState {
		t.Fatalf("unexpected dynamic context trace: %#v", block)
	}
	if block.Cacheable {
		t.Fatalf("ui state belongs to this turn only and must not be cacheable")
	}

	// 状态只进最后一条用户消息，历史轮次保持原样。
	last := plan.Messages[len(plan.Messages)-1]
	_, dynamic := splitUserContextParts(t, last)
	if !strings.Contains(dynamic, `<block id="loan" title="房贷计算器">{"amount":300}</block>`) {
		t.Fatalf("expected ui state in the last user turn, got %q", dynamic)
	}
	if plan.Messages[0].Content != "算一下房贷" || len(plan.Messages[0].Parts) != 0 {
		t.Fatalf("earlier turns must stay untouched, got %#v", plan.Messages[0])
	}
}
