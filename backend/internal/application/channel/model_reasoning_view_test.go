package channel

import (
	"reflect"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
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
			if got := (ModelCapabilityResolver{}).Resolve(tt.view).View; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestModelCapabilityResolverResolvesDisplaySpecs 校验展示规格取自模态目录，其中能力 JSON 的上下文窗口优先于目录值。
func TestModelCapabilityResolverResolvesDisplaySpecs(t *testing.T) {
	catalog := domainchannel.NewModelCatalog([]domainchannel.ModelCatalogEntry{{
		Provider:         "google",
		ModelID:          "gemini-3-pro-image-preview",
		CanonicalID:      "gemini-3-pro-image-preview",
		InputModalities:  []string{"text", "image"},
		OutputModalities: []string{"image", "text"},
		ContextWindow:    65_536,
	}})
	resolver := ModelCapabilityResolver{modalities: catalog}
	view := ModelView{PlatformModelName: "gemini-3-pro-image-preview", Vendor: "google", ProtocolsJSON: `["gemini_generate_content"]`}

	info := resolver.Resolve(view)
	if !reflect.DeepEqual(info.OutputModalities, []string{"text", "image"}) {
		t.Fatalf("OutputModalities = %v, want [text image]", info.OutputModalities)
	}
	if info.ContextWindow != 65_536 {
		t.Fatalf("ContextWindow = %d, want catalog value 65536", info.ContextWindow)
	}

	view.CapabilitiesJSON = `{"contextWindow":32768}`
	if got := resolver.Resolve(view).ContextWindow; got != 32_768 {
		t.Fatalf("ContextWindow = %d, want explicit value 32768", got)
	}
}
