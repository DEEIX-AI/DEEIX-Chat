package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	protocolVersion         = "2025-06-18"
	defaultRequestTimeoutMS = 10000
	defaultConnectTimeout   = 10 * time.Second
)

// Client 封装 MCP Streamable HTTP JSON-RPC 客户端。
type Client struct {
	httpClient *http.Client
	nextID     atomic.Int64
}

// CallConfig 定义 MCP 调用配置。
type CallConfig struct {
	BaseURL   string
	AuthToken string
	TimeoutMS int
	Headers   map[string]string
}

// CallInput 定义 MCP 工具调用入参。
type CallInput struct {
	ToolName       string
	ArgumentsJSON  string
	UserID         uint
	ConversationID uint
	RequestID      string
}

// Tool 定义 MCP 工具元数据。
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// NewClient 创建 MCP 客户端。
func NewClient() *Client {
	return NewClientWithEnv("", false)
}

// NewClientWithEnv 创建带运行环境的 MCP 客户端。
func NewClientWithEnv(env string, ssrfProtectionEnabled bool) *Client {
	transport := security.NewOutboundHTTPTransport(env, ssrfProtectionEnabled, defaultConnectTimeout)
	return &Client{
		httpClient: &http.Client{
			Transport: platformtracing.NewHTTPTransport(transport),
		},
	}
}

// ListTools 读取 MCP 服务暴露的工具列表。
func (c *Client) ListTools(ctx context.Context, cfg CallConfig) ([]Tool, error) {
	session, err := c.initialize(ctx, cfg)
	if err != nil {
		return nil, err
	}
	result, err := c.rpc(ctx, cfg, session, "tools/list", map[string]interface{}{}, false)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Tools []Tool `json:"tools"`
	}
	if err = json.Unmarshal(result, &payload); err != nil {
		return nil, err
	}
	return payload.Tools, nil
}

// CallTool 执行远端 MCP 工具。
func (c *Client) CallTool(ctx context.Context, cfg CallConfig, input CallInput) (string, error) {
	toolName := strings.TrimSpace(input.ToolName)
	if toolName == "" {
		return "", fmt.Errorf("mcp tool name is empty")
	}
	args, err := decodeArguments(input.ArgumentsJSON)
	if err != nil {
		return "", err
	}
	session, err := c.initialize(ctx, cfg)
	if err != nil {
		return "", err
	}
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
		"_meta": map[string]interface{}{
			"user_id":         input.UserID,
			"conversation_id": input.ConversationID,
			"request_id":      strings.TrimSpace(input.RequestID),
		},
	}
	result, err := c.rpc(ctx, cfg, session, "tools/call", params, false)
	if err != nil {
		return "", err
	}
	return normalizeToolCallResult(result)
}

func (c *Client) initialize(ctx context.Context, cfg CallConfig) (string, error) {
	params := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "deeix-chat",
			"version": "0.1.0",
		},
	}
	_, sessionID, err := c.rpcWithSession(ctx, cfg, "", "initialize", params, false)
	if err != nil {
		return "", err
	}
	_, _, err = c.rpcWithSession(ctx, cfg, sessionID, "notifications/initialized", nil, true)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func (c *Client) rpc(ctx context.Context, cfg CallConfig, sessionID string, method string, params interface{}, notification bool) (json.RawMessage, error) {
	result, _, err := c.rpcWithSession(ctx, cfg, sessionID, method, params, notification)
	return result, err
}

func (c *Client) rpcWithSession(
	ctx context.Context,
	cfg CallConfig,
	sessionID string,
	method string,
	params interface{},
	notification bool,
) (json.RawMessage, string, error) {
	endpoint, err := buildEndpointURL(cfg)
	if err != nil {
		return nil, sessionID, err
	}
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	if !notification {
		payload["id"] = c.nextRequestID()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, sessionID, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(resolveRequestTimeoutMS(cfg.TimeoutMS))*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, sessionID, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token := strings.TrimSpace(cfg.AuthToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if value := strings.TrimSpace(sessionID); value != "" {
		req.Header.Set("Mcp-Session-Id", value)
	}
	for key, value := range cfg.Headers {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		req.Header.Set(headerKey, strings.TrimSpace(value))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, sessionID, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if nextSessionID := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); nextSessionID != "" {
		sessionID = nextSessionID
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, sessionID, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, sessionID, fmt.Errorf("mcp request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if notification {
		return nil, sessionID, nil
	}
	result, err := parseRPCResponse(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, sessionID, err
	}
	return result, sessionID, nil
}

func resolveRequestTimeoutMS(timeoutMS int) int {
	if timeoutMS <= 0 {
		return defaultRequestTimeoutMS
	}
	return timeoutMS
}

func (c *Client) nextRequestID() int64 {
	return c.nextID.Add(1)
}

func buildEndpointURL(cfg CallConfig) (string, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return "", fmt.Errorf("mcp base url is empty")
	}
	return baseURL, nil
}

func parseRPCResponse(contentType string, body []byte) (json.RawMessage, error) {
	payload := strings.TrimSpace(string(body))
	if payload == "" {
		return nil, fmt.Errorf("mcp response is empty")
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if strings.EqualFold(mediaType, "text/event-stream") {
		payload = extractSSEDataPayload(payload)
		if payload == "" {
			return nil, fmt.Errorf("mcp event stream response is empty")
		}
	}
	response := rpcResponse{}
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("mcp json-rpc error %d: %s", response.Error.Code, response.Error.Message)
	}
	if len(response.Result) == 0 {
		return json.RawMessage("{}"), nil
	}
	return response.Result, nil
}

func extractSSEDataPayload(payload string) string {
	reader := bufio.NewReader(strings.NewReader(payload))
	dataLines := make([]string, 0, 4)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) > 0 {
				break
			}
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimPrefix(data, " ")
		dataLines = append(dataLines, data)
	}
	return strings.TrimSpace(strings.Join(dataLines, "\n"))
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallResult struct {
	Content []toolContent   `json:"content,omitempty"`
	IsError bool            `json:"isError,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

type toolContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

func normalizeToolCallResult(raw json.RawMessage) (string, error) {
	output := strings.TrimSpace(string(raw))
	if output == "" {
		return "{}", nil
	}

	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return output, nil
	}
	result.Raw = raw
	if result.IsError {
		return "", fmt.Errorf("mcp tool error: %s", result.errorText())
	}
	if errText := result.protocolErrorText(); errText != "" {
		return "", fmt.Errorf("mcp tool error: %s", errText)
	}
	return output, nil
}

func (r toolCallResult) errorText() string {
	text := strings.TrimSpace(r.textContent())
	if text != "" {
		return text
	}
	raw := strings.TrimSpace(string(r.Raw))
	if raw != "" {
		return raw
	}
	return "tool returned an error"
}

func (r toolCallResult) textContent() string {
	parts := make([]string, 0, len(r.Content))
	for _, item := range r.Content {
		if text := strings.TrimSpace(item.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func (r toolCallResult) protocolErrorText() string {
	text := strings.TrimSpace(r.textContent())
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "MCP error ") || strings.HasPrefix(text, "MCP error:") {
		return text
	}
	return ""
}

func decodeArguments(raw string) (map[string]interface{}, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]interface{}{}, nil
	}
	var parsed interface{}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("mcp tool arguments must be a valid JSON object")
	}
	object, ok := normalizeJSONNumber(parsed).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("mcp tool arguments must be a JSON object")
	}
	return object, nil
}

func normalizeJSONNumber(value interface{}) interface{} {
	switch typed := value.(type) {
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(typed.String(), 64); err == nil {
			return parsed
		}
		return typed.String()
	case map[string]interface{}:
		for key, item := range typed {
			typed[key] = normalizeJSONNumber(item)
		}
		return typed
	case []interface{}:
		for index, item := range typed {
			typed[index] = normalizeJSONNumber(item)
		}
		return typed
	default:
		return value
	}
}
