package conversation

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// uiStateRepositoryStub 只实现消息组件状态相关的仓储方法，其余能力沿用嵌入接口。
type uiStateRepositoryStub struct {
	repository.ConversationRepository
	conversation model.Conversation
	message      model.Message
	blocks       map[string]string
	upsertCalls  int
	lastBlocks   map[string]string
}

func (r *uiStateRepositoryStub) GetConversationByPublicID(context.Context, string, uint) (*model.Conversation, error) {
	item := r.conversation
	return &item, nil
}

func (r *uiStateRepositoryStub) GetMessageByPublicIDForUser(context.Context, uint, string) (*model.Message, error) {
	item := r.message
	return &item, nil
}

func (r *uiStateRepositoryStub) UpsertMessageUIStates(
	_ context.Context,
	_ uint,
	_ uint,
	_ uint,
	blocks map[string]string,
) error {
	r.upsertCalls++
	r.lastBlocks = blocks
	for blockID, stateJSON := range blocks {
		if strings.TrimSpace(stateJSON) == "" {
			delete(r.blocks, blockID)
			continue
		}
		r.blocks[blockID] = stateJSON
	}
	return nil
}

func (r *uiStateRepositoryStub) GetMessageUIStateMap(context.Context, uint, []uint) (map[uint]map[string]string, error) {
	if len(r.blocks) == 0 {
		return map[uint]map[string]string{}, nil
	}
	return map[uint]map[string]string{r.message.ID: r.blocks}, nil
}

func newUIStateStub(role string, status string, blocks map[string]string) *uiStateRepositoryStub {
	if blocks == nil {
		blocks = map[string]string{}
	}
	return &uiStateRepositoryStub{
		conversation: model.Conversation{ID: 7, PublicID: "conv_ui"},
		message:      model.Message{ID: 11, ConversationID: 7, PublicID: "msg_ui", Role: role, Status: status},
		blocks:       blocks,
	}
}

func TestSetMessageUIStatePersistsNormalizedDiff(t *testing.T) {
	repo := newUIStateStub("assistant", "success", map[string]string{"quiz": `{"answers":[1]}`})
	service := &Service{repo: repo}

	result, err := service.SetMessageUIState(context.Background(), 1, "conv_ui", "msg_ui", []MessageUIStateBlockPatch{
		{BlockID: " loan ", StateJSON: `{ "amount": 300, "years": 20 }`},
		{BlockID: "quiz", StateJSON: "{}"},
	})
	if err != nil {
		t.Fatalf("SetMessageUIState() error = %v", err)
	}
	if result.MessagePublicID != "msg_ui" {
		t.Fatalf("message public id = %q", result.MessagePublicID)
	}
	if len(result.Blocks) != 1 {
		t.Fatalf("expected only the loan block to remain, got %v", result.Blocks)
	}
	// 状态被规范化成紧凑 JSON 后再入库与返回。
	if got := string(result.Blocks["loan"]); got != `{"amount":300,"years":20}` {
		t.Fatalf("normalized loan state = %q", got)
	}
	if got := repo.lastBlocks["quiz"]; got != "" {
		t.Fatalf("quiz block should be cleared, got %q", got)
	}
}

func TestSetMessageUIStateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		status  string
		patches []MessageUIStateBlockPatch
		want    error
	}{
		{
			name:    "block id with space",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "bad id", StateJSON: `{"v":1}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "block id too long",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: strings.Repeat("a", maxUIStateBlockIDChars+1), StateJSON: `{"v":1}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "duplicated block id",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `{"v":1}`}, {BlockID: "loan", StateJSON: `{"v":2}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "state key with markup",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `{"<script>":1}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "too deep state",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `{"a":{"b":{"c":{"d":{"e":1}}}}}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "block id with dot",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan.v2", StateJSON: `{"v":1}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "state key too long",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `{"` + strings.Repeat("k", maxUIStateKeyChars+1) + `":1}`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "not a json object",
			role:    "assistant",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `[1,2]`}},
			want:    ErrInvalidMessageUIState,
		},
		{
			name:    "user message target",
			role:    "user",
			status:  "success",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `{"v":1}`}},
			want:    ErrMessageUIStateTargetInvalid,
		},
		{
			name:    "pending message target",
			role:    "assistant",
			status:  "pending",
			patches: []MessageUIStateBlockPatch{{BlockID: "loan", StateJSON: `{"v":1}`}},
			want:    ErrMessageUIStateTargetInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newUIStateStub(test.role, test.status, nil)
			service := &Service{repo: repo}
			_, err := service.SetMessageUIState(context.Background(), 1, "conv_ui", "msg_ui", test.patches)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if repo.upsertCalls != 0 {
				t.Fatalf("invalid input must not reach the repository, calls = %d", repo.upsertCalls)
			}
		})
	}
}

func TestSetMessageUIStateRejectsOversizedState(t *testing.T) {
	repo := newUIStateStub("assistant", "success", nil)
	service := &Service{repo: repo}

	oversized := `{"v":"` + strings.Repeat("x", maxUIStateStateBytes) + `"}`
	_, err := service.SetMessageUIState(context.Background(), 1, "conv_ui", "msg_ui", []MessageUIStateBlockPatch{
		{BlockID: "loan", StateJSON: oversized},
	})
	if !errors.Is(err, ErrInvalidMessageUIState) {
		t.Fatalf("error = %v, want invalid ui state", err)
	}
}

func TestSetMessageUIStateMapsRepositoryLimitError(t *testing.T) {
	repo := &uiStateLimitRepositoryStub{}
	service := &Service{repo: repo}

	_, err := service.SetMessageUIState(context.Background(), 1, "conv_ui", "msg_ui", []MessageUIStateBlockPatch{
		{BlockID: "loan", StateJSON: `{"v":1}`},
	})
	if !errors.Is(err, ErrInvalidMessageUIState) {
		t.Fatalf("error = %v, want invalid ui state", err)
	}
}

// uiStateLimitRepositoryStub 模拟仓储层在事务内发现块数超限的情景。
type uiStateLimitRepositoryStub struct {
	repository.ConversationRepository
}

func (r *uiStateLimitRepositoryStub) GetConversationByPublicID(context.Context, string, uint) (*model.Conversation, error) {
	return &model.Conversation{ID: 7, PublicID: "conv_ui"}, nil
}

func (r *uiStateLimitRepositoryStub) GetMessageByPublicIDForUser(context.Context, uint, string) (*model.Message, error) {
	return &model.Message{ID: 11, ConversationID: 7, PublicID: "msg_ui", Role: "assistant", Status: "success"}, nil
}

func (r *uiStateLimitRepositoryStub) UpsertMessageUIStates(context.Context, uint, uint, uint, map[string]string) error {
	return repository.ErrMessageUIStateLimitExceeded
}

func TestHydrateMessageUIStatesSkipsNonAssistantMessages(t *testing.T) {
	repo := newUIStateStub("assistant", "success", map[string]string{"loan": `{"amount":300}`})
	service := &Service{repo: repo}

	items := []model.Message{
		{ID: 11, PublicID: "msg_ui", Role: "assistant"},
		{ID: 11, PublicID: "msg_user", Role: "user"},
	}
	if err := service.hydrateMessageUIStates(context.Background(), 1, items); err != nil {
		t.Fatalf("hydrateMessageUIStates() error = %v", err)
	}
	if items[0].UIState["loan"] != `{"amount":300}` {
		t.Fatalf("assistant message ui state = %v", items[0].UIState)
	}
	if len(items[1].UIState) != 0 {
		t.Fatalf("user message must not carry ui state, got %v", items[1].UIState)
	}
}

func TestMergedMessageUIStateSkipsInvalidJSON(t *testing.T) {
	merged := MergedMessageUIState(map[string]string{
		"loan": `{"amount": 300}`,
		"bad":  `{`,
	})
	if len(merged) != 1 {
		t.Fatalf("merged = %v", merged)
	}
	if string(merged["loan"]) != `{"amount":300}` {
		t.Fatalf("compacted state = %s", merged["loan"])
	}
}

// 状态会原样进入提示词：不做 HTML 转义，键按字典序输出，保证同一状态每次写入的内容一致。
func TestNormalizeUIStateJSONKeepsMarkupReadable(t *testing.T) {
	got, err := normalizeUIStateJSON(`{"query":"a<b>&c","amount":300}`)
	if err != nil {
		t.Fatalf("normalizeUIStateJSON() error = %v", err)
	}
	if got != `{"amount":300,"query":"a<b>&c"}` {
		t.Fatalf("normalized = %q", got)
	}
}
