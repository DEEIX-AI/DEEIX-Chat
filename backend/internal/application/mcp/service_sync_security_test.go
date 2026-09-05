package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	systemeventapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	portmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type syncSecurityRepo struct {
	repository.MCPRepository
	lastError string
}

func (r *syncSecurityRepo) GetServer(context.Context, uint) (*domainmcp.Server, error) {
	return &domainmcp.Server{ID: 1, BaseURL: "https://mcp.example/mcp", HeadersJSON: "{}"}, nil
}

func (r *syncSecurityRepo) UpdateServer(_ context.Context, _ uint, input repository.UpdateMCPServerInput) (*domainmcp.Server, error) {
	if input.LastError != nil {
		r.lastError = *input.LastError
	}
	return &domainmcp.Server{ID: 1}, nil
}

type syncSecurityClient struct {
	err error
}

func (c syncSecurityClient) ListTools(context.Context, portmcp.CallConfig) ([]portmcp.Tool, error) {
	return nil, c.err
}

type syncSecurityEventWriter struct {
	input systemeventapp.WriteInput
}

func (w *syncSecurityEventWriter) Write(_ context.Context, input systemeventapp.WriteInput) {
	w.input = input
}

func TestSyncServerToolsDoesNotPersistProviderErrorDetails(t *testing.T) {
	const secret = "https://internal.example/mcp?token=secret"
	repo := &syncSecurityRepo{}
	writer := &syncSecurityEventWriter{}
	service := NewServiceWithRuntime(
		config.NewRuntime(config.Config{}),
		repo,
		syncSecurityClient{err: errors.New(secret)},
	)
	service.SetSystemEventWriter(writer)

	if _, err := service.SyncServerTools(context.Background(), SyncServerToolsInput{ServerID: 1}); err == nil {
		t.Fatal("expected tool synchronization to fail")
	}
	if repo.lastError != "MCP 工具同步失败" {
		t.Fatalf("last error = %q, want stable public summary", repo.lastError)
	}
	detail, err := json.Marshal(writer.input.Detail)
	if err != nil {
		t.Fatalf("marshal event detail: %v", err)
	}
	if strings.Contains(string(detail), secret) || strings.Contains(string(detail), "token=secret") {
		t.Fatalf("provider details leaked into system event: %s", detail)
	}
}
