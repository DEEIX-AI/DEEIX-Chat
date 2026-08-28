package mcp

import (
	"context"
	"testing"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type availableToolsRepoFake struct {
	repository.MCPRepository
}

func (availableToolsRepoFake) ListServers(context.Context) ([]domainmcp.Server, error) {
	return []domainmcp.Server{{ID: 1, Name: "exa", Status: "active"}}, nil
}

func (availableToolsRepoFake) ListTools(context.Context, uint, bool) ([]domainmcp.Tool, error) {
	return []domainmcp.Tool{
		{ID: 11, ServerID: 1, Name: "web_search_exa", PriceNanousd: 5_000_000, Status: "active"},
		{ID: 12, ServerID: 1, Name: "web_fetch_exa", PriceNanousd: 0, Status: "active"},
	}, nil
}

type billingModeProviderFake struct {
	mode string
}

func (f billingModeProviderFake) GetBillingMode(context.Context) (string, error) {
	return f.mode, nil
}

func newAvailableToolsService(mode string) *Service {
	service := NewServiceWithRuntime(config.NewRuntime(config.Config{MCPEnable: true}), availableToolsRepoFake{}, nil)
	if mode != "" {
		service.SetBillingModeProvider(billingModeProviderFake{mode: mode})
	}
	return service
}

func TestListAvailableToolsHidesPriceInSelfBillingMode(t *testing.T) {
	tools, err := newAvailableToolsService("self").ListAvailableTools(context.Background())
	if err != nil {
		t.Fatalf("list available tools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.PriceNanousd != 0 {
			t.Fatalf("tool %s price must be hidden in self mode, got %d", tool.Name, tool.PriceNanousd)
		}
	}
}

func TestListAvailableToolsKeepsPriceInBilledModes(t *testing.T) {
	for _, mode := range []string{"usage", "period"} {
		tools, err := newAvailableToolsService(mode).ListAvailableTools(context.Background())
		if err != nil {
			t.Fatalf("list available tools failed in %s mode: %v", mode, err)
		}
		if tools[0].PriceNanousd != 5_000_000 {
			t.Fatalf("tool price must be kept in %s mode, got %d", mode, tools[0].PriceNanousd)
		}
	}
}

func TestListAvailableToolsKeepsPriceWithoutBillingModeProvider(t *testing.T) {
	tools, err := newAvailableToolsService("").ListAvailableTools(context.Background())
	if err != nil {
		t.Fatalf("list available tools failed: %v", err)
	}
	if tools[0].PriceNanousd != 5_000_000 {
		t.Fatalf("tool price must be kept without billing mode provider, got %d", tools[0].PriceNanousd)
	}
}
