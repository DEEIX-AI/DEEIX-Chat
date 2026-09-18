package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/mcpauth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
)

// ExecuteToolInput 定义工具执行入参。
type ExecuteToolInput struct {
	UserID         uint
	ConversationID uint
	RequestID      string
	ToolName       string
	ArgumentsJSON  string
	MCPConfig      *mcp.CallConfig
}

func (s *Service) executeToolCall(ctx context.Context, input ExecuteToolInput) (string, error) {
	toolName := strings.TrimSpace(input.ToolName)
	if toolName == "" {
		return "", fmt.Errorf("tool name is required")
	}
	if input.MCPConfig == nil {
		return "", fmt.Errorf("tool %s is not enabled for this run", toolName)
	}
	if s.mcpClient == nil {
		return "", fmt.Errorf("mcp client is not configured")
	}
	cfg := s.cfg.Snapshot()
	mcpCfg, err := applySignedUserContext(cfg, *input.MCPConfig, input)
	if err != nil {
		return "", err
	}

	limit := cfg.MCPMaxConcurrentCalls
	if limit <= 0 {
		limit = 8
	}

	return s.executeWithToolLimiter(ctx, limit, func() (string, error) {
		return s.callMCPWithRetry(ctx, mcpCfg, mcp.CallInput{
			ToolName:       toolName,
			ArgumentsJSON:  strings.TrimSpace(input.ArgumentsJSON),
			UserID:         input.UserID,
			ConversationID: input.ConversationID,
			RequestID:      strings.TrimSpace(input.RequestID),
		}, cfg.MCPToolRetryCount)
	})
}

// applySignedUserContext 将请求头中值等于 ${DEEIX_SIGNED_USER_CONTEXT} 占位符的项
// 替换为本次工具调用签名的用户上下文。未配置占位符时原样返回，不改变现有行为；
// 签名失败时拒绝调用，避免 MCP 服务端收到没有用户上下文的请求。
func applySignedUserContext(cfg config.Config, base mcp.CallConfig, input ExecuteToolInput) (mcp.CallConfig, error) {
	if len(base.Headers) == 0 {
		return base, nil
	}
	matched := false
	for _, value := range base.Headers {
		if strings.TrimSpace(value) == mcpauth.TemplateSignedUserContext {
			matched = true
			break
		}
	}
	if !matched {
		return base, nil
	}
	token, err := mcpauth.Sign(cfg.MCPUserContextSecret, mcpauth.Payload{
		UserID:         input.UserID,
		ConversationID: input.ConversationID,
		RequestID:      strings.TrimSpace(input.RequestID),
		ExpiresAt:      time.Now().Add(mcpauth.DefaultTTL).Unix(),
	})
	if err != nil || token == "" {
		return mcp.CallConfig{}, fmt.Errorf("mcp user context signing failed")
	}
	expanded := make(map[string]string, len(base.Headers))
	for key, value := range base.Headers {
		if strings.TrimSpace(value) != mcpauth.TemplateSignedUserContext {
			expanded[key] = value
			continue
		}
		expanded[key] = token
	}
	base.Headers = expanded
	return base, nil
}

func (s *Service) resolveMaxToolCallsPerRun() int {
	maxCalls := s.cfg.Snapshot().MCPMaxToolCallsPerRun
	if maxCalls <= 0 {
		maxCalls = 8
	}
	if maxCalls > 64 {
		maxCalls = 64
	}
	return maxCalls
}

func (s *Service) resolveMaxSelectedToolsPerMessage() int {
	maxTools := s.cfg.Snapshot().MCPMaxSelectedToolsPerMessage
	if maxTools <= 0 {
		maxTools = config.DefaultMCPMaxSelectedToolsPerMessage
	}
	if maxTools > config.MaxMCPSelectedToolsPerMessage {
		maxTools = config.MaxMCPSelectedToolsPerMessage
	}
	return maxTools
}

// ValidateSelectedToolIDs 校验单次消息选择的 MCP 工具数量。
func (s *Service) ValidateSelectedToolIDs(toolIDs []uint) error {
	if len(toolIDs) > s.resolveMaxSelectedToolsPerMessage() {
		return ErrTooManySelectedTools
	}
	return nil
}

func (s *Service) resolveMaxLLMCallsPerRun() int {
	maxCalls := s.cfg.Snapshot().MCPMaxLLMCallsPerRun
	if maxCalls <= 0 {
		maxCalls = 5
	}
	if maxCalls < 2 {
		maxCalls = 2
	}
	if maxCalls > 32 {
		maxCalls = 32
	}
	return maxCalls
}

func (s *Service) executeWithToolLimiter(
	ctx context.Context,
	limit int,
	fn func() (string, error),
) (string, error) {
	if fn == nil {
		return "", fmt.Errorf("tool execution function is nil")
	}
	if limit <= 0 {
		return fn()
	}

	limiter := s.getToolLimiter(limit)
	select {
	case limiter <- struct{}{}:
		defer func() { <-limiter }()
		return fn()
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *Service) getToolLimiter(limit int) chan struct{} {
	if limit <= 0 {
		limit = 1
	}
	if value, ok := s.toolLimiters.Load(limit); ok {
		if limiter, castOK := value.(chan struct{}); castOK {
			return limiter
		}
	}
	created := make(chan struct{}, limit)
	actual, _ := s.toolLimiters.LoadOrStore(limit, created)
	limiter, ok := actual.(chan struct{})
	if !ok {
		return created
	}
	return limiter
}

func (s *Service) callMCPWithRetry(
	ctx context.Context,
	cfg mcp.CallConfig,
	input mcp.CallInput,
	retryCount int,
) (string, error) {
	if retryCount < 0 {
		retryCount = 0
	}

	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		output, err := s.mcpClient.CallTool(ctx, cfg, input)
		if err == nil {
			return output, nil
		}
		lastErr = err
		if attempt >= retryCount {
			break
		}

		backoff := time.Duration(100*(attempt+1)) * time.Millisecond
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", lastErr
}
