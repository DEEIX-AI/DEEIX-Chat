package conversation

import (
	"encoding/json"
	"net/http"
	"testing"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin/binding"
)

// 错误码是前端 errors.json 依赖的契约。
func TestMessageUIStateErrorsExposeStableContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err     error
		code    string
		message string
	}{
		{appconversation.ErrInvalidMessageUIState, "message.invalid_ui_state", "invalid message ui state"},
		{appconversation.ErrMessageUIStateTargetInvalid, "message.ui_state_target_invalid", "message ui state target invalid"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			t.Parallel()
			got := response.Describe(http.StatusBadRequest, test.err)
			want := response.Description{Status: http.StatusBadRequest, Code: test.code, Message: test.message}
			if got != want {
				t.Fatalf("response.Describe() = %#v, want %#v", got, want)
			}
		})
	}
}

// 请求体以块 id 为键：值为对象表示替换，null 表示清除，空 blocks 没有意义。
func TestSetMessageUIStateRequestBinding(t *testing.T) {
	t.Parallel()

	var req SetMessageUIStateRequest
	if err := json.Unmarshal([]byte(`{"blocks":{"loan":{"amount":300},"quiz":null}}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if err := binding.Validator.ValidateStruct(req); err != nil {
		t.Fatalf("expected request to pass validation: %v", err)
	}

	states := make(map[string]string, len(req.Blocks))
	for _, patch := range toMessageUIStatePatches(req.Blocks) {
		states[patch.BlockID] = patch.StateJSON
	}
	if states["loan"] != `{"amount":300}` {
		t.Fatalf("loan patch = %q", states["loan"])
	}
	if state, exists := states["quiz"]; !exists || state != "" {
		t.Fatalf("null block must become a clear patch, got %q (exists=%v)", state, exists)
	}

	for name, body := range map[string]string{
		"missing_blocks": `{}`,
		"empty_blocks":   `{"blocks":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var invalid SetMessageUIStateRequest
			if err := json.Unmarshal([]byte(body), &invalid); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if err := binding.Validator.ValidateStruct(invalid); err == nil {
				t.Fatal("expected request without blocks to fail validation")
			}
		})
	}
}
