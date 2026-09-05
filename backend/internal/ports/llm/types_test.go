package llm

import "testing"

func TestUsageHasObservedInput(t *testing.T) {
	tests := map[string]struct {
		usage Usage
		want  bool
	}{
		"empty":            {usage: Usage{}, want: false},
		"output only":      {usage: Usage{OutputTokens: 12, ReasoningTokens: 3}, want: false},
		"non-cached input": {usage: Usage{InputTokens: 12}, want: true},
		"fully cached":     {usage: Usage{CacheReadTokens: 3355}, want: true},
		"cache write only": {usage: Usage{CacheWriteTokens: 128}, want: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := test.usage.HasObservedInput(); got != test.want {
				t.Fatalf("HasObservedInput() = %v, want %v for %#v", got, test.want, test.usage)
			}
		})
	}
}

func TestUsageHasObservedOutput(t *testing.T) {
	tests := map[string]struct {
		usage Usage
		want  bool
	}{
		"empty":          {usage: Usage{}, want: false},
		"input only":     {usage: Usage{InputTokens: 12, CacheReadTokens: 40}, want: false},
		"visible output": {usage: Usage{OutputTokens: 12}, want: true},
		"reasoning only": {usage: Usage{ReasoningTokens: 3}, want: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := test.usage.HasObservedOutput(); got != test.want {
				t.Fatalf("HasObservedOutput() = %v, want %v for %#v", got, test.want, test.usage)
			}
		})
	}
}
