package conversation

import "testing"

func TestSplitThinkingContentOnlyAcceptsLeadingClosedBlock(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVisible string
		wantThink   string
	}{
		{
			name:        "leading think block",
			input:       "<think>hidden</think>visible",
			wantVisible: "visible",
			wantThink:   "hidden",
		},
		{
			name:        "leading thinking block with attributes",
			input:       "\n<thinking data-source=\"model\">hidden</thinking>\nvisible",
			wantVisible: "visible",
			wantThink:   "hidden",
		},
		{
			name:        "middle think remains visible",
			input:       "visible <think>not hidden</think> tail",
			wantVisible: "visible <think>not hidden</think> tail",
			wantThink:   "",
		},
		{
			name:        "unclosed think remains visible",
			input:       "<think>not closed",
			wantVisible: "<think>not closed",
			wantThink:   "",
		},
		{
			name:        "plain thinking word remains visible",
			input:       "stream JSON uses isThinking to describe state",
			wantVisible: "stream JSON uses isThinking to describe state",
			wantThink:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, think := splitThinkingContent(tt.input)
			if visible != tt.wantVisible || think != tt.wantThink {
				t.Fatalf("unexpected split: visible=%q think=%q", visible, think)
			}
		})
	}
}

func TestThinkingDeltaRouterParsesEachAssistantOutputStart(t *testing.T) {
	router := &thinkingDeltaRouter{}
	visible, think := router.consume("<thi")
	if visible != "" || think != "" {
		t.Fatalf("partial opening tag should be buffered, got visible=%q think=%q", visible, think)
	}
	visible, think = router.consume("nk>hid")
	if visible != "" || think != "hid" {
		t.Fatalf("completed opening tag should enter thinking immediately: visible=%q think=%q", visible, think)
	}
	visible, think = router.consume("den</think>visible")
	if visible != "visible" || think != "den" {
		t.Fatalf("unexpected completed block tail split: visible=%q think=%q", visible, think)
	}
	visible, think = router.consume(" with <think>literal</think>")
	if visible != " with <think>literal</think>" || think != "" {
		t.Fatalf("post-resolution tags should stay visible: visible=%q think=%q", visible, think)
	}

	nextRouter := &thinkingDeltaRouter{}
	visible, think = nextRouter.consume("<thinking>sec")
	if visible != "" || think != "sec" {
		t.Fatalf("new assistant output should enter thinking immediately: visible=%q think=%q", visible, think)
	}
	visible, think = nextRouter.consume("ond</thinking>answer")
	if visible != "answer" || think != "ond" {
		t.Fatalf("new assistant output should parse its own leading block: visible=%q think=%q", visible, think)
	}
}

func TestThinkingDeltaRouterHoldsPartialClosingTag(t *testing.T) {
	router := &thinkingDeltaRouter{}
	visible, think := router.consume("<think>hidden</thi")
	if visible != "" || think != "hidden" {
		t.Fatalf("partial closing tag should be buffered outside thinking text: visible=%q think=%q", visible, think)
	}
	visible, think = router.consume("nk>visible")
	if visible != "visible" || think != "" {
		t.Fatalf("completed closing tag should exit thinking: visible=%q think=%q", visible, think)
	}
}

func TestThinkingDeltaRouterKeepsInvalidLeadingTagVisible(t *testing.T) {
	router := &thinkingDeltaRouter{}
	visible, think := router.consume("<thinkingg")
	if visible != "<thinkingg" || think != "" {
		t.Fatalf("invalid leading tag should stay visible: visible=%q think=%q", visible, think)
	}
}

func TestThinkingDeltaRouterFlushesUnclosedBlockAsThinking(t *testing.T) {
	router := &thinkingDeltaRouter{}
	visible, think := router.consume("<thinking>not closed")
	if visible != "" || think != "not closed" {
		t.Fatalf("leading block should stream as thinking after opening tag, got visible=%q think=%q", visible, think)
	}
	visible, think = router.flush()
	if visible != "" || think != "" {
		t.Fatalf("flushed unclosed block should not duplicate streamed thinking, got visible=%q think=%q", visible, think)
	}
}
