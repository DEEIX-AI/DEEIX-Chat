package billing

import "testing"

func TestBuildMCPToolServiceItemsBillsSnapshotPricePerCall(t *testing.T) {
	input := UsagePricingInput{
		PlatformModelName: "gpt-test",
		ProviderProtocol:  "openai",
		MCPToolUsage: []MCPToolUsageInput{
			{ServerID: 1, ServerName: "exa", ToolName: "search", CallCount: 3, PriceNanousd: 5_000_000},
			{ServerID: 1, ServerName: "exa", ToolName: "contents", CallCount: 1, PriceNanousd: 0},
			{ServerID: 2, ServerName: "serp", ToolName: "search", CallCount: 0, PriceNanousd: 2_000_000},
		},
	}

	items, total := buildMCPToolServiceItems(input, "balance")

	if len(items) != 1 {
		t.Fatalf("expected only priced positive-count usage to be billed, got %#v", items)
	}
	item := items[0]
	if item.ServiceCode != "mcp_tool.exa.search" || item.ServiceName != "exa / search" {
		t.Fatalf("unexpected service identity: %#v", item)
	}
	if item.CallCount != 3 || item.CallNanousdPerCall != 5_000_000 || item.BilledNanousd != 15_000_000 {
		t.Fatalf("unexpected billing amounts: %#v", item)
	}
	if total != 15_000_000 {
		t.Fatalf("expected total 15000000, got %d", total)
	}
}

func TestBuildMCPToolServiceItemsSkipsSelfModeOnly(t *testing.T) {
	input := UsagePricingInput{
		MCPToolUsage: []MCPToolUsageInput{
			{ServerID: 1, ServerName: "exa", ToolName: "search", CallCount: 2, PriceNanousd: 5_000_000},
		},
	}

	if items, total := buildMCPToolServiceItems(input, "self"); len(items) != 0 || total != 0 {
		t.Fatalf("expected self mode to skip billing, got %#v total=%d", items, total)
	}
	// MCP 调用是外部上游成本，免费模型不豁免。
	if items, total := buildMCPToolServiceItems(input, "balance"); len(items) != 1 || total != 10_000_000 {
		t.Fatalf("expected free-model requests to still bill MCP tool calls, got %#v total=%d", items, total)
	}
}

func TestMCPToolUsageSnapshotsKeepUnpricedUsageVisible(t *testing.T) {
	snapshots := mcpToolUsageSnapshots([]MCPToolUsageInput{
		{ServerID: 1, ServerName: " exa ", ToolName: "search", CallCount: 2, PriceNanousd: 0},
		{ServerID: 2, ServerName: "serp", ToolName: "search", CallCount: 0, PriceNanousd: 1},
	})

	if len(snapshots) != 1 {
		t.Fatalf("expected zero-count usage to be dropped, got %#v", snapshots)
	}
	if snapshots[0]["server_name"] != "exa" || snapshots[0]["call_count"] != int64(2) {
		t.Fatalf("unexpected snapshot: %#v", snapshots[0])
	}
}
