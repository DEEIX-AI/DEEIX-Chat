package conversation

import "testing"

func TestMergeMCPToolUsageAggregatesByServerToolAndPrice(t *testing.T) {
	got := mergeMCPToolUsage(
		[]MCPToolUsageItem{
			{ServerID: 1, ServerName: "exa", ToolName: "search", CallCount: 1, PriceNanousd: 5},
			{ServerID: 2, ServerName: "serp", ToolName: "search", CallCount: 1, PriceNanousd: 3},
		},
		[]MCPToolUsageItem{
			{ServerID: 1, ServerName: "exa", ToolName: "search", CallCount: 2, PriceNanousd: 5},
			{ServerID: 1, ServerName: "exa", ToolName: "search", CallCount: 1, PriceNanousd: 7},
			{ServerID: 1, ServerName: "exa", ToolName: "search", CallCount: 0, PriceNanousd: 5},
		},
	)

	if len(got) != 3 {
		t.Fatalf("expected 3 aggregated entries, got %#v", got)
	}
	if got[0].ServerID != 1 || got[0].CallCount != 3 || got[0].PriceNanousd != 5 {
		t.Fatalf("expected same server/tool/price to merge counts, got %#v", got[0])
	}
	if got[1].ServerID != 2 || got[1].CallCount != 1 {
		t.Fatalf("expected different server with same tool name to stay separate, got %#v", got[1])
	}
	if got[2].PriceNanousd != 7 || got[2].CallCount != 1 {
		t.Fatalf("expected price change to produce a separate entry, got %#v", got[2])
	}
}

func TestMergeMCPToolUsageReturnsNilWhenEmpty(t *testing.T) {
	if got := mergeMCPToolUsage(nil, nil); got != nil {
		t.Fatalf("expected nil for empty input, got %#v", got)
	}
	if got := mergeMCPToolUsage(nil, []MCPToolUsageItem{{CallCount: 0}}); got != nil {
		t.Fatalf("expected nil when all counts are non-positive, got %#v", got)
	}
}
