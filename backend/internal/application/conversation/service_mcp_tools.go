package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"strconv"
	"strings"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

type selectedToolRuntime struct {
	definitions         []llm.ToolDefinition
	nameMap             map[string]string
	mcpBindings         map[string]mcpToolCallBinding
	schemas             map[string]json.RawMessage
	attachmentProcessor *selectedAttachmentProcessor
}

// mcpToolCallBinding 绑定模型侧工具名对应的 MCP 调用配置与计量元数据。
// 服务器归属与价格在解析选中工具时快照，保证同名工具跨服务器可区分、计费按调用时价格结算。
type mcpToolCallBinding struct {
	Config       mcp.CallConfig
	ServerID     uint
	ServerName   string
	ToolName     string
	PriceNanousd int64
}

type selectedAttachmentProcessor struct {
	toolID         uint
	modelName      string
	toolName       string
	displayName    string
	argument       string
	encoding       string
	promptArgument string
}

func injectMCPToolGuidance(messages []llm.Message, runtime selectedToolRuntime, customPrompt string) []llm.Message {
	if len(runtime.definitions) == 0 {
		return messages
	}

	content := strings.TrimSpace(customPrompt)
	if content == "" {
		content = defaultMCPToolGuidancePrompt()
	}

	insertAt := 0
	for insertAt < len(messages) && messages[insertAt].Role == "system" {
		insertAt++
	}
	next := make([]llm.Message, 0, len(messages)+1)
	next = append(next, messages[:insertAt]...)
	next = append(next, llm.Message{Role: "system", Content: content})
	next = append(next, messages[insertAt:]...)
	return next
}

func defaultMCPToolGuidancePrompt() string {
	var builder strings.Builder
	builder.WriteString("# tool_use\n")
	builder.WriteString("- Tools are declared separately via the API schema; follow that schema exactly.\n")
	builder.WriteString("- Use tools only for external, realtime, private, or explicitly requested data.\n")
	builder.WriteString("- Use the fewest useful calls; each call must add new information.\n")
	builder.WriteString("- Do not repeat an identical failed call. Adjust arguments, use another tool, or answer from available evidence.\n")
	builder.WriteString("- If tools fail or lack enough data, state the gap in the final answer.\n")
	builder.WriteString("- Do not expose raw tool JSON, internal fields, or tool logs unless the user asks.\n")
	return strings.TrimSpace(builder.String())
}

func (s *Service) resolveSelectedToolRuntime(ctx context.Context, toolIDs []uint) (selectedToolRuntime, error) {
	if len(toolIDs) == 0 || !s.cfg.Snapshot().MCPEnable {
		return selectedToolRuntime{}, nil
	}
	if s.mcpRepo == nil {
		return selectedToolRuntime{}, fmt.Errorf("resolve selected MCP tools: repository unavailable")
	}
	tools, err := s.mcpRepo.ListToolsByIDs(ctx, uniqueToolIDs(toolIDs))
	if err != nil {
		return selectedToolRuntime{}, fmt.Errorf("resolve selected MCP tools: %w", err)
	}
	if len(tools) == 0 {
		return selectedToolRuntime{}, nil
	}

	cfg := s.cfg.Snapshot()
	result := selectedToolRuntime{
		definitions: make([]llm.ToolDefinition, 0, len(tools)),
		nameMap:     map[string]string{},
		mcpBindings: map[string]mcpToolCallBinding{},
		schemas:     map[string]json.RawMessage{},
	}
	usedNames := map[string]int{}
	serverCache := map[uint]*domainmcp.Server{}
	for _, tool := range tools {
		if tool.Status != "active" {
			continue
		}
		isAttachmentProcessor := strings.EqualFold(strings.TrimSpace(tool.AttachmentInputMode), domainmcp.AttachmentInputModeImage)
		server, ok := serverCache[tool.ServerID]
		if !ok {
			server, err = s.mcpRepo.GetServer(ctx, tool.ServerID)
			if err != nil {
				return selectedToolRuntime{}, fmt.Errorf("resolve MCP server %d: %w", tool.ServerID, err)
			}
			if server == nil || server.Status != "active" {
				if isAttachmentProcessor {
					return selectedToolRuntime{}, fmt.Errorf("%w: processor server is unavailable", ErrImageAttachmentProcessingFailed)
				}
				continue
			}
			if validateErr := security.ValidateTrustedOutboundHTTPURL(server.BaseURL); validateErr != nil {
				if isAttachmentProcessor {
					return selectedToolRuntime{}, fmt.Errorf("%w: processor server URL is not allowed", ErrImageAttachmentProcessingFailed)
				}
				continue
			}
			serverCache[tool.ServerID] = server
		}
		modelName := uniqueModelToolName(llm.NormalizeToolName(tool.Name), usedNames)
		if modelName == "" {
			continue
		}
		schema := json.RawMessage(strings.TrimSpace(tool.InputSchemaJSON))
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		token, err := secretbox.DecryptString(cfg.DataEncryptionKey, server.AuthTokenEnc)
		if err != nil {
			if isAttachmentProcessor {
				return selectedToolRuntime{}, fmt.Errorf("%w: processor credentials are unavailable", ErrImageAttachmentProcessingFailed)
			}
			continue
		}
		headers := parseMCPHeaders(server.HeadersJSON)
		result.definitions = append(result.definitions, llm.ToolDefinition{
			Name:        modelName,
			Description: strings.TrimSpace(tool.Description),
			InputSchema: schema,
		})
		result.nameMap[modelName] = tool.Name
		result.schemas[modelName] = schema
		result.mcpBindings[modelName] = mcpToolCallBinding{
			Config: mcp.CallConfig{
				BaseURL:   server.BaseURL,
				AuthToken: token,
				TimeoutMS: cfg.MCPToolTimeoutSeconds * 1000,
				Headers:   headers,
			},
			ServerID:     server.ID,
			ServerName:   server.Name,
			ToolName:     tool.Name,
			PriceNanousd: tool.PriceNanousd,
		}
		if isAttachmentProcessor {
			if bindErr := result.bindAttachmentProcessor(selectedAttachmentProcessor{
				toolID:         tool.ID,
				modelName:      modelName,
				toolName:       tool.Name,
				displayName:    textutil.FirstNonEmpty(tool.DisplayName, tool.Name),
				argument:       strings.TrimSpace(tool.AttachmentArgument),
				encoding:       strings.TrimSpace(tool.AttachmentEncoding),
				promptArgument: strings.TrimSpace(tool.AttachmentPromptArgument),
			}); bindErr != nil {
				return selectedToolRuntime{}, bindErr
			}
		}
	}
	return result, nil
}

func (r *selectedToolRuntime) bindAttachmentProcessor(processor selectedAttachmentProcessor) error {
	if r.attachmentProcessor != nil {
		return ErrMultipleImageAttachmentProcessors
	}
	r.attachmentProcessor = &processor
	return nil
}

func (r selectedToolRuntime) withoutAttachmentProcessor() selectedToolRuntime {
	processor := r.attachmentProcessor
	if processor == nil {
		return r
	}
	definitions := make([]llm.ToolDefinition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		if definition.Name != processor.modelName {
			definitions = append(definitions, definition)
		}
	}
	r.definitions = definitions
	delete(r.nameMap, processor.modelName)
	delete(r.mcpBindings, processor.modelName)
	delete(r.schemas, processor.modelName)
	r.attachmentProcessor = nil
	return r
}

func (r selectedToolRuntime) withoutDefinitions() selectedToolRuntime {
	r.definitions = nil
	r.nameMap = nil
	r.mcpBindings = nil
	r.schemas = nil
	r.attachmentProcessor = nil
	return r
}

func uniqueToolIDs(items []uint) []uint {
	seen := make(map[uint]struct{}, len(items))
	result := make([]uint, 0, len(items))
	for _, item := range items {
		if item == 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func uniqueModelToolName(base string, used map[string]int) string {
	value := strings.TrimSpace(base)
	if value == "" {
		return ""
	}
	count := used[value]
	used[value] = count + 1
	if count == 0 {
		return value
	}
	suffix := "_" + strconv.Itoa(count+1)
	if len(value)+len(suffix) > 64 {
		value = value[:64-len(suffix)]
	}
	return value + suffix
}

func parseMCPHeaders(raw string) map[string]string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]string{}
	}
	payload := map[string]string{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(payload))
	for key, item := range payload {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		result[headerKey] = strings.TrimSpace(item)
	}
	return result
}
