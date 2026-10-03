package channel

import (
	"reflect"
	"testing"
)

func TestModelReasoningResolverWithoutCatalog(t *testing.T) {
	tests := []struct {
		name string
		view ModelView
		want *ModelReasoningView
	}{
		{
			name: "no capability",
			view: ModelView{ProtocolsJSON: `["openai_responses"]`, CapabilitiesJSON: `{"defaultOptions":{"temperature":1}}`},
			want: nil,
		},
		{
			name: "explicit",
			view: ModelView{ProtocolsJSON: `["anthropic_messages"]`, CapabilitiesJSON: `{"reasoning":{"format":"anthropic_effort","levels":["max","low","high"],"default":"high"}}`},
			want: &ModelReasoningView{Levels: []string{"low", "high", "max"}, Default: "high", Source: "explicit"},
		},
		{
			name: "explicit locked by native path",
			view: ModelView{ProtocolsJSON: `["anthropic_messages"]`, CapabilitiesJSON: `{"reasoning":{"format":"anthropic_effort","levels":["low","high"],"default":"high"},"lockedOptionPaths":["output_config.effort"]}`},
			want: &ModelReasoningView{Levels: []string{"low", "high"}, Default: "high", Source: "explicit", Locked: true},
		},
		{
			name: "explicit locked by canonical key",
			view: ModelView{ProtocolsJSON: `["google_generate_content"]`, CapabilitiesJSON: `{"reasoning":{"format":"gemini_budget","levels":["none","low"],"default":"low","budgets":{"low":1024}},"lockedOptionPaths":["reasoning_effort"]}`},
			want: &ModelReasoningView{Levels: []string{"none", "low"}, Default: "low", Source: "explicit", Locked: true},
		},
		{
			name: "inferred from legacy control via google alias",
			view: ModelView{ProtocolsJSON: `["google_generate_content"]`, CapabilitiesJSON: `{
				"defaultOptions":{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}},
				"optionControls":[{"path":"generationConfig.thinkingConfig.thinkingLevel","type":"select","options":["low","medium","high"]}],
				"lockedOptionPaths":["generationConfig.thinkingConfig.thinkingLevel"]
			}`},
			want: &ModelReasoningView{Levels: []string{"low", "medium", "high"}, Default: "high", ControlPath: "generationConfig.thinkingConfig.thinkingLevel", Source: "inferred", Locked: true},
		},
		{
			name: "invalid protocols json",
			view: ModelView{ProtocolsJSON: `oops`, CapabilitiesJSON: `{"optionControls":[{"path":"reasoning_effort","type":"select","options":["low"]}]}`},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (ModelReasoningResolver{}).Resolve(tt.view).View; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}
