package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

type selectedToolRuntime struct {
	definitions []llm.ToolDefinition
	nameMap     map[string]string
	mcpConfigs  map[string]mcp.CallConfig
	schemas     map[string]json.RawMessage
}

func injectMCPToolGuidance(messages []llm.Message, runtime selectedToolRuntime) []llm.Message {
	if len(runtime.definitions) == 0 {
		return messages
	}

	var builder strings.Builder
	builder.WriteString("# tool_use\n")
	builder.WriteString("- Tools are declared separately via the API schema; follow that schema exactly.\n")
	builder.WriteString("- Use tools only for external, realtime, private, or explicitly requested data.\n")
	builder.WriteString("- Use the fewest useful calls; each call must add new information.\n")
	builder.WriteString("- Do not repeat an identical failed call. Adjust arguments, use another tool, or answer from available evidence.\n")
	builder.WriteString("- If tools fail or lack enough data, state the gap in the final answer.\n")
	builder.WriteString("- Do not expose raw tool JSON, internal fields, or tool logs unless the user asks.\n")

	insertAt := 0
	for insertAt < len(messages) && messages[insertAt].Role == "system" {
		insertAt++
	}
	next := make([]llm.Message, 0, len(messages)+1)
	next = append(next, messages[:insertAt]...)
	next = append(next, llm.Message{Role: "system", Content: strings.TrimSpace(builder.String())})
	next = append(next, messages[insertAt:]...)
	return next
}

func summarizeToolInputSchema(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return ""
	}
	properties, _ := schema["properties"].(map[string]interface{})
	if len(properties) == 0 {
		return "无需参数"
	}
	required := map[string]struct{}{}
	if items, ok := schema["required"].([]interface{}); ok {
		for _, item := range items {
			if name, ok := item.(string); ok && strings.TrimSpace(name) != "" {
				required[strings.TrimSpace(name)] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		prop, _ := properties[name].(map[string]interface{})
		fieldType := schemaFieldType(prop)
		label := name
		if fieldType != "" {
			label = fmt.Sprintf("%s:%s", name, fieldType)
		}
		if _, ok := required[name]; ok {
			label += " 必填"
		}
		parts = append(parts, label)
	}
	if len(parts) > 6 {
		parts = append(parts[:6], fmt.Sprintf("等 %d 个字段", len(parts)))
	}
	return "参数 " + strings.Join(parts, "，")
}

func schemaFieldType(prop map[string]interface{}) string {
	if len(prop) == 0 {
		return ""
	}
	if value, ok := prop["type"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if items, ok := prop["type"].([]interface{}); ok && len(items) > 0 {
		types := make([]string, 0, len(items))
		for _, item := range items {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				types = append(types, strings.TrimSpace(value))
			}
		}
		if len(types) > 0 {
			return strings.Join(types, "|")
		}
	}
	if _, ok := prop["enum"].([]interface{}); ok {
		return "enum"
	}
	return ""
}

func (s *Service) resolveSelectedToolRuntime(ctx context.Context, toolIDs []uint) selectedToolRuntime {
	if s.mcpRepo == nil || len(toolIDs) == 0 || !s.cfg.Snapshot().MCPEnable {
		return selectedToolRuntime{}
	}
	tools, err := s.mcpRepo.ListToolsByIDs(ctx, uniqueToolIDs(toolIDs))
	if err != nil || len(tools) == 0 {
		return selectedToolRuntime{}
	}

	cfg := s.cfg.Snapshot()
	result := selectedToolRuntime{
		definitions: make([]llm.ToolDefinition, 0, len(tools)),
		nameMap:     map[string]string{},
		mcpConfigs:  map[string]mcp.CallConfig{},
		schemas:     map[string]json.RawMessage{},
	}
	usedNames := map[string]int{}
	serverCache := map[uint]*domainmcp.Server{}
	for _, tool := range tools {
		if tool.Status != "active" {
			continue
		}
		server, ok := serverCache[tool.ServerID]
		if !ok {
			server, err = s.mcpRepo.GetServer(ctx, tool.ServerID)
			if err != nil || server == nil || server.Status != "active" {
				continue
			}
			if validateErr := security.ValidateOutboundHTTPURL(server.BaseURL, cfg.Env); validateErr != nil {
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
		result.mcpConfigs[modelName] = mcp.CallConfig{
			BaseURL:   server.BaseURL,
			AuthToken: token,
			TimeoutMS: cfg.MCPToolTimeoutSeconds * 1000,
			Headers:   headers,
		}
	}
	return result
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
