package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// modernProtocolVersion 是无握手的当前规范版本：每个请求在 _meta 与请求头中自带版本。
	modernProtocolVersion = "2026-07-28"

	errorCodeMethodNotFound             = -32601
	errorCodeHeaderMismatch             = -32020
	errorCodeMissingRequiredCapability  = -32021
	errorCodeUnsupportedProtocolVersion = -32022

	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientInfo         = "io.modelcontextprotocol/clientInfo"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"

	headerParamPrefix = "Mcp-Param-"
	base64Sentinel    = "=?base64?"
	base64Suffix      = "?="
)

// serverEra 记录服务端支持的协议世代：modern 为无握手版本，legacy 为 initialize 握手版本（2025-11-25 及更早）。
type serverEra int

const (
	eraUnknown serverEra = iota
	eraModern
	eraLegacy
)

var clientInfo = map[string]any{"name": "deeix-chat", "version": "0.1.0"}

// errLegacyServer 表示 modern 请求被 4xx 拒绝且响应体不是 modern 协议错误，按规范应回退到 initialize 握手。
var errLegacyServer = errors.New("mcp server does not speak the modern protocol")

// rpcCall 是一次 JSON-RPC 请求；Name 镜像到 Mcp-Name 头，ParamHeaders 为 x-mcp-header 标注参数生成的 Mcp-Param-* 头。
type rpcCall struct {
	Method       string
	Name         string
	Params       map[string]any
	ParamHeaders map[string]string
}

func (c *Client) cachedEra(baseURL string) serverEra {
	if value, ok := c.eras.Load(strings.TrimSpace(baseURL)); ok {
		return value.(serverEra)
	}
	return eraUnknown
}

func (c *Client) rememberEra(baseURL string, era serverEra) {
	c.eras.Store(strings.TrimSpace(baseURL), era)
}

// call 按服务端世代发送请求：先尝试 modern 请求，遇到 legacy 服务端时回退到 initialize 握手并缓存结论；
// 返回的 session 仅在 legacy 世代下非空，供同一会话内的后续分页请求复用。
func (c *Client) call(ctx context.Context, cfg CallConfig, call rpcCall) (json.RawMessage, session, error) {
	if c.cachedEra(cfg.BaseURL) != eraLegacy {
		result, err := c.modernRequest(ctx, cfg, call)
		if !errors.Is(err, errLegacyServer) {
			if err == nil {
				c.rememberEra(cfg.BaseURL, eraModern)
			}
			return result, session{}, err
		}
		c.rememberEra(cfg.BaseURL, eraLegacy)
	}
	current, err := c.initialize(ctx, cfg)
	if err != nil {
		return nil, session{}, err
	}
	result, current, err := c.rpcWithSession(ctx, cfg, current, call.Method, call.Params, false)
	return result, current, err
}

func (c *Client) modernRequest(ctx context.Context, cfg CallConfig, call rpcCall) (json.RawMessage, error) {
	endpoint, err := buildEndpointURL(cfg)
	if err != nil {
		return nil, err
	}
	params := make(map[string]any, len(call.Params)+1)
	for key, value := range call.Params {
		params[key] = value
	}
	meta := map[string]any{}
	if existing, ok := params["_meta"].(map[string]any); ok {
		for key, value := range existing {
			meta[key] = value
		}
	}
	meta[metaProtocolVersion] = modernProtocolVersion
	meta[metaClientInfo] = clientInfo
	meta[metaClientCapabilities] = map[string]any{}
	params["_meta"] = meta

	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextRequestID(),
		"method":  call.Method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(resolveRequestTimeoutMS(cfg.TimeoutMS))*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	for key, value := range cfg.Headers {
		if headerKey := strings.TrimSpace(key); headerKey != "" {
			req.Header.Set(headerKey, strings.TrimSpace(value))
		}
	}
	if token := strings.TrimSpace(cfg.AuthToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// 协议头放在管理员自定义头之后写入，避免被同名自定义头覆盖而触发服务端 HeaderMismatch。
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", modernProtocolVersion)
	req.Header.Set("Mcp-Method", call.Method)
	if call.Name != "" {
		req.Header.Set("Mcp-Name", encodeHeaderValue(call.Name))
	}
	for name, value := range call.ParamHeaders {
		req.Header.Set(headerParamPrefix+name, value)
	}

	resp, err := c.httpClients.Do(req, cfg.BaseURL, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, modernHTTPError(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	result, err := parseRPCResponse(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, err
	}
	if resultType := resultTypeOf(result); resultType != "" && resultType != "complete" {
		return nil, fmt.Errorf("mcp server returned an unsupported %q result: additional user input is not supported", resultType)
	}
	return result, nil
}

// modernHTTPError 区分 modern 服务端的协议错误与 legacy 服务端的拒绝：只有 400/404/405 且响应体不是
// 可识别的 modern 错误时回退；UnsupportedProtocolVersion 只列出握手版本时同样视为需要握手的服务端。
func modernHTTPError(status int, contentType string, body []byte) error {
	failure := fmt.Errorf("mcp request failed: status=%d", status)
	rpcErr := parseRPCErrorBody(contentType, body)
	switch {
	case rpcErr != nil && rpcErr.Code == errorCodeUnsupportedProtocolVersion:
		supported := rpcErr.supportedVersions()
		for _, version := range supported {
			if version < modernProtocolVersion {
				return errLegacyServer
			}
		}
		return fmt.Errorf("mcp server does not support protocol %s (supported: %s)", modernProtocolVersion, strings.Join(supported, ", "))
	case rpcErr != nil && isModernErrorCode(status, rpcErr.Code):
		return fmt.Errorf("mcp json-rpc error %d: %s", rpcErr.Code, rpcErr.Message)
	case status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusMethodNotAllowed:
		return errLegacyServer
	}
	return failure
}

func isModernErrorCode(status int, code int) bool {
	switch code {
	case errorCodeHeaderMismatch, errorCodeMissingRequiredCapability, errorCodeUnsupportedProtocolVersion:
		return true
	case errorCodeMethodNotFound:
		return status == http.StatusNotFound
	}
	return false
}

func parseRPCErrorBody(contentType string, body []byte) *rpcError {
	payload := strings.TrimSpace(string(body))
	if payload == "" {
		return nil
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		payload = extractSSEDataPayload(payload)
	}
	var response struct {
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		return nil
	}
	return response.Error
}

func resultTypeOf(result json.RawMessage) string {
	var payload struct {
		ResultType string `json:"resultType"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ResultType)
}

// encodeHeaderValue 按 Streamable HTTP 规则编码头部值：非可见 ASCII、首尾空白或形似哨兵格式的值使用 Base64 哨兵。
func encodeHeaderValue(value string) string {
	if headerValueIsPlain(value) {
		return value
	}
	return base64Sentinel + base64.StdEncoding.EncodeToString([]byte(value)) + base64Suffix
}

func headerValueIsPlain(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return value == ""
	}
	if strings.HasPrefix(value, base64Sentinel) && strings.HasSuffix(value, base64Suffix) {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char > 0x7e {
			return false
		}
	}
	return true
}
