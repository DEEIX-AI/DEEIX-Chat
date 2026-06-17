package llm

import (
	"encoding/json"
	"errors"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var errDeepSeekDSMLToolCallsIncomplete = errors.New("deepseek dsml tool calls ended before a complete tool call envelope")

var (
	dsmlTokenRE          = `(?:｜DSML｜|｜｜DSML｜｜|\|\|DSML\|\||\|DSML\|)`
	dsmlToolCallsBlockRE = regexp.MustCompile(`(?is)<\s*` + dsmlTokenRE + `\s*tool_calls\s*>(.*?)</\s*` + dsmlTokenRE + `\s*tool_calls\s*>`)
	dsmlInvokeRE         = regexp.MustCompile(`(?is)<\s*` + dsmlTokenRE + `\s*invoke\b([^>]*)>(.*?)</\s*` + dsmlTokenRE + `\s*invoke\s*>`)
	dsmlParameterRE      = regexp.MustCompile(`(?is)<\s*` + dsmlTokenRE + `\s*parameter\b([^>]*)>(.*?)</\s*` + dsmlTokenRE + `\s*parameter\s*>`)
	dsmlAttributeRE      = regexp.MustCompile(`(?is)([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*("([^"]*)"|'([^']*)')`)
)

// applyTextEncodedToolCalls 将 DeepSeek V4 文本编码的工具调用转换为内部结构化 ToolCall。
// 这段兼容只在路由层显式判定为 DeepSeek Chat Completions 时调用，避免影响其他 OpenAI-compatible 模型。
func applyTextEncodedToolCalls(output *GenerateOutput) {
	if output == nil {
		return
	}
	cleanText, toolCalls, ok := parseDSMLToolCalls(output.Text)
	if !ok {
		return
	}
	output.Text = cleanText
	output.ToolCalls = append(output.ToolCalls, toolCalls...)
}

// deepSeekTextEncodedToolCallsEnabled 判断当前路由是否需要启用 DeepSeek DSML 文本工具调用解析。
func deepSeekTextEncodedToolCallsEnabled(route RouteConfig) bool {
	if NormalizeAdapter(route.Protocol) != AdapterOpenAIChatCompletions {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(route.UpstreamModel))
	baseURL := strings.ToLower(strings.TrimSpace(route.BaseURL))
	return strings.Contains(model, "deepseek") || strings.Contains(baseURL, "deepseek")
}

// parseDSMLToolCalls 解析 DeepSeek V4 以 DSML 文本片段返回的工具调用。
// 解析成功时只移除完整 tool_calls 块；无法形成合法工具调用时保留原文，交由调用方按普通文本或格式错误处理。
func parseDSMLToolCalls(text string) (string, []ToolCall, bool) {
	matches := dsmlToolCallsBlockRE.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil, false
	}

	var clean strings.Builder
	toolCalls := make([]ToolCall, 0)
	last := 0
	for _, match := range matches {
		blockStart, blockEnd := match[0], match[1]
		contentStart, contentEnd := match[2], match[3]
		blockToolCalls := parseDSMLInvokeToolCalls(text[contentStart:contentEnd], len(toolCalls))
		if len(blockToolCalls) == 0 {
			continue
		}
		clean.WriteString(text[last:blockStart])
		last = blockEnd
		toolCalls = append(toolCalls, blockToolCalls...)
	}
	if len(toolCalls) == 0 {
		return text, nil, false
	}
	clean.WriteString(text[last:])
	return strings.TrimSpace(clean.String()), toolCalls, true
}

// parseDSMLInvokeToolCalls 将 DSML invoke 节点映射为本地函数工具调用。
func parseDSMLInvokeToolCalls(content string, offset int) []ToolCall {
	matches := dsmlInvokeRE.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make([]ToolCall, 0, len(matches))
	for _, match := range matches {
		attrs := parseDSMLAttributes(match[1])
		toolName := strings.TrimSpace(attrs["name"])
		if toolName == "" {
			continue
		}
		args := parseDSMLParameters(match[2])
		if args == nil {
			continue
		}
		argsJSON, err := json.Marshal(args)
		if err != nil {
			argsJSON = []byte("{}")
		}
		result = append(result, ToolCall{
			ToolCallID:    "dsml_call_" + strconv.Itoa(offset+len(result)+1),
			ToolType:      "function",
			ToolName:      toolName,
			ArgumentsJSON: string(argsJSON),
			Status:        "requested",
		})
	}
	return result
}

// parseDSMLParameters 按 DeepSeek DSML 的 string 标记还原参数类型。
// 参数名重复时放弃本次 invoke，避免同名覆盖造成工具入参歧义。
func parseDSMLParameters(content string) map[string]interface{} {
	params := map[string]interface{}{}
	for _, match := range dsmlParameterRE.FindAllStringSubmatch(content, -1) {
		attrs := parseDSMLAttributes(match[1])
		name := strings.TrimSpace(attrs["name"])
		if name == "" {
			continue
		}
		if _, exists := params[name]; exists {
			return nil
		}
		value := strings.TrimSpace(html.UnescapeString(match[2]))
		if strings.EqualFold(strings.TrimSpace(attrs["string"]), "true") {
			params[name] = value
			continue
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			params[name] = decoded
			continue
		}
		params[name] = value
	}
	return params
}

// parseDSMLAttributes 解析 DSML 标签属性，属性名统一小写以匹配上游格式波动。
func parseDSMLAttributes(raw string) map[string]string {
	attrs := map[string]string{}
	for _, match := range dsmlAttributeRE.FindAllStringSubmatch(raw, -1) {
		value := match[3]
		if value == "" {
			value = match[4]
		}
		attrs[strings.ToLower(strings.TrimSpace(match[1]))] = html.UnescapeString(value)
	}
	return attrs
}
