package conversation

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRequiredConversationBooleanFieldsAcceptExplicitFalse(t *testing.T) {
	falseValue := false

	for name, request := range map[string]any{
		"star":    SetConversationStarRequest{Starred: &falseValue},
		"archive": SetConversationArchiveRequest{Archived: &falseValue},
	} {
		t.Run(name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(request); err != nil {
				t.Fatalf("expected explicit false to pass validation: %v", err)
			}
		})
	}
}

func TestRequiredConversationBooleanFieldsRejectMissingValues(t *testing.T) {
	for name, request := range map[string]any{
		"star":    SetConversationStarRequest{},
		"archive": SetConversationArchiveRequest{},
	} {
		t.Run(name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(request); err == nil {
				t.Fatal("expected missing required boolean to fail validation")
			}
		})
	}
}

func TestConversationLabelsRequestAllowsExplicitEmptyList(t *testing.T) {
	labels := []string{}
	request := UpdateConversationLabelsRequest{Labels: &labels}
	if err := binding.Validator.ValidateStruct(request); err != nil {
		t.Fatalf("expected explicit empty labels to pass validation: %v", err)
	}
}

func TestConversationLabelsRequestRejectsMissingOrOversizedValues(t *testing.T) {
	tooMany := []string{"1", "2", "3", "4", "5", "6", "7"}
	tooLong := []string{"1234567890123456789012345"}
	for name, request := range map[string]UpdateConversationLabelsRequest{
		"missing":  {},
		"too_many": {Labels: &tooMany},
		"too_long": {Labels: &tooLong},
	} {
		t.Run(name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(request); err == nil {
				t.Fatal("expected invalid labels request to fail validation")
			}
		})
	}
}

// 组件状态是发送请求的旁路上下文：内容是否合法由应用层判定并静默丢弃，
// 绑定层只限制条数与字段长度，避免一个坏状态让整条消息发不出去。
func TestSendMessageRequestUIStatesValidation(t *testing.T) {
	base := func(states []MessageUIStateInputRequest) SendMessageRequest {
		return SendMessageRequest{ContentType: "text", Content: "hi", UIStates: states}
	}
	tooMany := make([]MessageUIStateInputRequest, 33)

	for name, test := range map[string]struct {
		request SendMessageRequest
		valid   bool
	}{
		"absent":            {base(nil), true},
		"malformed_state":   {base([]MessageUIStateInputRequest{{MessagePublicID: "m1", BlockID: "bad id", State: []byte(`[1]`)}}), true},
		"null_state":        {base([]MessageUIStateInputRequest{{MessagePublicID: "m1", BlockID: "loan", State: []byte(`null`)}}), true},
		"too_many":          {base(tooMany), false},
		"oversized_title":   {base([]MessageUIStateInputRequest{{MessagePublicID: "m1", BlockID: "loan", Title: string(make([]byte, 201))}}), false},
		"oversized_blockID": {base([]MessageUIStateInputRequest{{MessagePublicID: "m1", BlockID: string(make([]byte, 129))}}), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := binding.Validator.ValidateStruct(test.request)
			if test.valid && err != nil {
				t.Fatalf("expected request to pass validation: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected request to fail validation")
			}
		})
	}
}

func TestTemporaryChatRequestUIStatesValidation(t *testing.T) {
	base := func(states []TemporaryChatUIStateRequest) TemporaryChatMessageRequest {
		return TemporaryChatMessageRequest{
			SessionID: "s", ClientRunID: "r", Model: "m",
			Messages: []TemporaryChatHistoryMessage{{Role: "user", Content: "hi"}},
			UIStates: states,
		}
	}
	for name, test := range map[string]struct {
		request TemporaryChatMessageRequest
		valid   bool
	}{
		"absent":         {base(nil), true},
		"first_message":  {base([]TemporaryChatUIStateRequest{{MessageIndex: 0, BlockID: "loan", State: []byte(`{"v":1}`)}}), true},
		"negative_index": {base([]TemporaryChatUIStateRequest{{MessageIndex: -1, BlockID: "loan"}}), false},
		"index_too_big":  {base([]TemporaryChatUIStateRequest{{MessageIndex: 100, BlockID: "loan"}}), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := binding.Validator.ValidateStruct(test.request)
			if test.valid && err != nil {
				t.Fatalf("expected request to pass validation: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected request to fail validation")
			}
		})
	}
}

// 块格式版本只接受当前已知的 1、2；缺省等同 1，新版本需要后端先支持对应的提示词。
func TestUIBlockVersionValidation(t *testing.T) {
	for version, valid := range map[int]bool{0: true, 1: true, 2: true, 3: false, -1: false} {
		send := SendMessageRequest{ContentType: "text", Content: "hi", UIBlockVersion: version}
		temporary := TemporaryChatMessageRequest{
			SessionID: "s", ClientRunID: "r", Model: "m",
			Messages:       []TemporaryChatHistoryMessage{{Role: "user", Content: "hi"}},
			UIBlockVersion: version,
		}
		for name, request := range map[string]any{"send": send, "temporary": temporary} {
			err := binding.Validator.ValidateStruct(request)
			if valid != (err == nil) {
				t.Fatalf("%s version %d: valid=%v, err=%v", name, version, valid, err)
			}
		}
	}
}
