package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestProcessImageAttachmentsRoutesOnlyTextToMainModelContext(t *testing.T) {
	var receivedImage string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var rpcRequest struct {
			ID     interface{} `json:"id"`
			Method string      `json:"method"`
			Params struct {
				Arguments map[string]interface{} `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(request.Body).Decode(&rpcRequest); err != nil {
			t.Errorf("decode MCP request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch rpcRequest.Method {
		case "initialize":
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      rpcRequest.ID,
				"result":  map[string]interface{}{"protocolVersion": "2025-06-18", "capabilities": map[string]interface{}{}},
			})
		case "notifications/initialized":
			writer.WriteHeader(http.StatusAccepted)
		case "tools/call":
			receivedImage, _ = rpcRequest.Params.Arguments["image"].(string)
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      rpcRequest.ID,
				"result": map[string]interface{}{
					"content":           []map[string]interface{}{{"type": "text", "text": "画面中有一辆红色汽车。"}},
					"structuredContent": map[string]interface{}{"echo": receivedImage},
				},
			})
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	store := objectstore.NewLocal(t.TempDir())
	imageData := testPNG(t)
	if _, err := store.Put(t.Context(), "images/current.png", bytes.NewReader(imageData), objectstore.PutOptions{ContentType: "image/png"}); err != nil {
		t.Fatalf("put image: %v", err)
	}
	if _, err := store.Put(t.Context(), "images/second.png", bytes.NewReader(imageData), objectstore.PutOptions{ContentType: "image/png"}); err != nil {
		t.Fatalf("put second image: %v", err)
	}
	service := &Service{
		cfg:           config.NewRuntime(config.Config{MCPMaxToolCallsPerRun: 8, MCPMaxConcurrentCalls: 8}),
		mcpClient:     mcp.NewClient(security.OutboundPolicy{}),
		storeProvider: &conversationTestStoreProvider{store: store},
	}
	runtime := selectedToolRuntime{
		nameMap: map[string]string{"vision_analyze": "vision_analyze"},
		mcpBindings: map[string]mcpToolCallBinding{
			"vision_analyze": {
				Config:       mcp.CallConfig{BaseURL: server.URL, TimeoutMS: 5000},
				ServerID:     11,
				ServerName:   "vision-server",
				ToolName:     "vision_analyze",
				PriceNanousd: 2_000_000,
			},
		},
		schemas: map[string]json.RawMessage{
			"vision_analyze": json.RawMessage(`{"type":"object","properties":{"image":{"type":"string"},"prompt":{"type":"string"}},"required":["image"]}`),
		},
		attachmentProcessor: &selectedAttachmentProcessor{
			toolID:         7,
			modelName:      "vision_analyze",
			toolName:       "vision_analyze",
			displayName:    "图片分析",
			argument:       "image",
			encoding:       domainmcp.AttachmentEncodingDataURL,
			promptArgument: "prompt",
		},
	}

	result, err := service.processImageAttachments(t.Context(), imageAttachmentProcessingInput{
		UserID:         1,
		ConversationID: 2,
		MessageID:      3,
		RequestID:      "request-1",
		RunID:          "run-1",
		UserPrompt:     "图中有什么？",
		Attachments: []AttachmentInput{{
			FileID: "file-1", FileName: "current.png", Kind: "image", MimeType: "image/png",
			StoragePath: "images/current.png", Current: true,
		}, {
			FileID: "file-2", FileName: "second.png", Kind: "image", MimeType: "image/png",
			StoragePath: "images/second.png", Current: true,
		}},
		Runtime: runtime,
	})
	if err != nil {
		t.Fatalf("process image attachment: %v", err)
	}
	if !result.Routed || len(result.Analyses) != 2 || result.Analyses[0].Content != "画面中有一辆红色汽车。" {
		t.Fatalf("unexpected processing result: %#v", result)
	}
	if !strings.HasPrefix(receivedImage, "data:image/png;base64,") {
		t.Fatalf("expected a PNG data URL, got prefix %q", receivedImage[:min(len(receivedImage), 32)])
	}
	if len(result.Rows) != 2 || result.Rows[0].ToolType != "mcp_attachment" || result.Rows[0].Status != "success" || result.Rows[1].Status != "success" {
		t.Fatalf("unexpected tool audit row: %#v", result.Rows)
	}
	if strings.Contains(result.Rows[0].InputJSON, "base64") || strings.Contains(result.Rows[0].InputJSON, receivedImage) {
		t.Fatalf("tool audit input must not persist image bytes: %s", result.Rows[0].InputJSON)
	}
	if strings.Contains(result.Rows[0].OutputJSON, receivedImage) || strings.Contains(result.Rows[0].OutputJSON, ";base64,") {
		t.Fatalf("tool audit output must not persist echoed image bytes: %s", result.Rows[0].OutputJSON)
	}

	messages := injectUserContext(t.Context(), []llm.Message{{Role: "user", Content: "图中有什么？"}}, userContextInput{
		ImageAnalyses: result.Analyses,
	}, config.Config{}, nil)
	if len(messages) != 1 || len(messages[0].Parts) != 0 || !strings.Contains(messages[0].Content, "画面中有一辆红色汽车。") {
		t.Fatalf("expected text-only analysis context, got %#v", messages)
	}
}

type selectedToolRuntimeMCPRepositoryStub struct {
	repository.MCPRepository
	listToolsByIDs func(context.Context, []uint) ([]domainmcp.Tool, error)
	getServer      func(context.Context, uint) (*domainmcp.Server, error)
}

func (s selectedToolRuntimeMCPRepositoryStub) ListToolsByIDs(ctx context.Context, toolIDs []uint) ([]domainmcp.Tool, error) {
	return s.listToolsByIDs(ctx, toolIDs)
}

func (s selectedToolRuntimeMCPRepositoryStub) GetServer(ctx context.Context, serverID uint) (*domainmcp.Server, error) {
	return s.getServer(ctx, serverID)
}

func TestResolveSelectedToolRuntimePropagatesRepositoryFailure(t *testing.T) {
	expected := errors.New("database unavailable")
	service := &Service{
		cfg: config.NewRuntime(config.Config{MCPEnable: true}),
		mcpRepo: selectedToolRuntimeMCPRepositoryStub{
			listToolsByIDs: func(context.Context, []uint) ([]domainmcp.Tool, error) {
				return nil, expected
			},
		},
	}
	_, err := service.resolveSelectedToolRuntime(t.Context(), []uint{1})
	if !errors.Is(err, expected) {
		t.Fatalf("expected repository error to be propagated, got %v", err)
	}
}

func TestResolveSelectedToolRuntimeFailsClosedForUnavailableAttachmentProcessor(t *testing.T) {
	service := &Service{
		cfg: config.NewRuntime(config.Config{MCPEnable: true}),
		mcpRepo: selectedToolRuntimeMCPRepositoryStub{
			listToolsByIDs: func(context.Context, []uint) ([]domainmcp.Tool, error) {
				return []domainmcp.Tool{{
					ID:                  1,
					ServerID:            2,
					Status:              "active",
					AttachmentInputMode: domainmcp.AttachmentInputModeImage,
				}}, nil
			},
			getServer: func(context.Context, uint) (*domainmcp.Server, error) {
				return nil, nil
			},
		},
	}
	_, err := service.resolveSelectedToolRuntime(t.Context(), []uint{1})
	if !errors.Is(err, ErrImageAttachmentProcessingFailed) {
		t.Fatalf("expected unavailable processor to fail closed, got %v", err)
	}
}

func TestSelectedToolRuntimeRejectsMultipleImageProcessors(t *testing.T) {
	runtime := selectedToolRuntime{}
	if err := runtime.bindAttachmentProcessor(selectedAttachmentProcessor{toolID: 1}); err != nil {
		t.Fatalf("bind first processor: %v", err)
	}
	if err := runtime.bindAttachmentProcessor(selectedAttachmentProcessor{toolID: 2}); !errors.Is(err, ErrMultipleImageAttachmentProcessors) {
		t.Fatalf("expected multiple processor error, got %v", err)
	}
}

func TestBuildMessageRoutePromptSkipsRawImagesAfterProcessorRouting(t *testing.T) {
	service := &Service{}
	plan, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{UpstreamModel: "text-only"}, messageRoutePromptInput{
		UserContent: "继续分析",
		DomainMessages: []domainconversation.Message{{
			Role:        "user",
			Content:     "描述图片",
			Attachments: `[{"file_id":"image-1","kind":"image","mime_type":"image/png"}]`,
		}},
		StableAttachments: []AttachmentInput{{
			FileID: "image-1", Kind: "image", MimeType: "image/png", ContextMode: fileContextModeDirectImage,
		}},
		SkipImageAttachments: true,
		Config:               config.Config{},
	})
	if err != nil {
		t.Fatalf("build routed prompt: %v", err)
	}
	for _, message := range plan.Messages {
		for _, part := range message.Parts {
			if part.Kind == llm.ContentPartImage {
				t.Fatalf("raw image leaked into routed prompt: %#v", plan.Messages)
			}
		}
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 30, B: 20, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buffer.Bytes()
}
