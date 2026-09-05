package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	systemeventapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	portmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

var (
	ErrInvalidServerName           = apperr.New("mcp.invalid_server_name", "invalid mcp server name")
	ErrInvalidServerBaseURL        = apperr.New("mcp.invalid_server_base_url", "invalid mcp server base url")
	ErrInvalidServerStatus         = apperr.New("mcp.invalid_server_status", "invalid mcp server status")
	ErrInvalidServerHeaders        = apperr.New("mcp.invalid_server_headers", "invalid mcp server headers json")
	ErrInvalidToolStatus           = apperr.New("mcp.invalid_tool_status", "invalid mcp tool status")
	ErrInvalidToolName             = apperr.New("mcp.invalid_tool_name", "invalid mcp tool display name")
	ErrInvalidToolDesc             = apperr.New("mcp.invalid_tool_description", "invalid mcp tool description")
	ErrInvalidToolAttachmentConfig = apperr.NewMasked("mcp.invalid_attachment_configuration", "invalid MCP tool attachment configuration", "invalid mcp tool attachment configuration")
	ErrInvalidToolSelection        = apperr.New("mcp.invalid_tool_selection", "invalid mcp tool selection")
	ErrInvalidToolPrice            = apperr.New("request.invalid_mcp_tool_price", "invalid mcp tool price")
	ErrMCPClientUnavailable        = apperr.New("mcp.client_unavailable", "mcp client unavailable")
	// ErrServerLimitExceeded MCP 服务数量超限。
	ErrServerLimitExceeded = apperr.New("mcp.server_limit_exceeded", "mcp server limit exceeded")
)

const mcpServerToolListTimeoutMS = 10000

type Service struct {
	cfg                 *config.Runtime
	repo                repository.MCPRepository
	client              toolLister
	systemEventWriter   systemEventWriter
	billingModeProvider billingModeProvider
}

// toolLister 列出远端 MCP 服务暴露的工具。
type toolLister interface {
	ListTools(ctx context.Context, cfg portmcp.CallConfig) ([]portmcp.Tool, error)
}

// billingModeProvider 查询当前计费模式，用于决定用户侧是否下发工具价格。
type billingModeProvider interface {
	GetBillingMode(ctx context.Context) (string, error)
}

type ReorderServerInput struct {
	ServerID uint
	ToolIDs  []uint
}

type systemEventWriter interface {
	Write(ctx context.Context, input systemeventapp.WriteInput)
}

type ServerInput struct {
	Name        string
	BaseURL     string
	AuthToken   string
	HeadersJSON string
	Status      string
}

type ToolInput struct {
	DisplayName              *string
	Description              *string
	AttachmentInputMode      *string
	AttachmentArgument       *string
	AttachmentEncoding       *string
	AttachmentPromptArgument *string
	// PriceNanousd 单次调用价格（nano USD），0 表示不单独计费。
	PriceNanousd *int64
	Status       *string
}

// SyncServerToolsInput 描述一次 MCP 工具同步请求。
type SyncServerToolsInput struct {
	ServerID                    uint
	RequestID                   string
	OverwriteCustomizedMetadata bool
}

// NewServiceWithRuntime 创建 MCP 应用服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.MCPRepository, client toolLister) *Service {
	return &Service{cfg: cfg, repo: repo, client: client}
}

// SetSystemEventWriter 注入系统事件写入器。
func (s *Service) SetSystemEventWriter(writer systemEventWriter) {
	s.systemEventWriter = writer
}

// SetBillingModeProvider 注入计费模式查询器。
func (s *Service) SetBillingModeProvider(provider billingModeProvider) {
	s.billingModeProvider = provider
}

func (s *Service) ListServers(ctx context.Context) ([]domainmcp.Server, error) {
	return s.repo.ListServers(ctx)
}

func (s *Service) CreateServer(ctx context.Context, input ServerInput) (*domainmcp.Server, error) {
	normalized, err := s.normalizeServerInput(input, true)
	if err != nil {
		return nil, err
	}
	tokenEnc, err := s.encryptToken(normalized.AuthToken)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.CreateServer(ctx, repository.CreateMCPServerInput{
		Name:         normalized.Name,
		BaseURL:      normalized.BaseURL,
		AuthTokenEnc: tokenEnc,
		HeadersJSON:  normalized.HeadersJSON,
		Status:       normalized.Status,
	})
	if errors.Is(err, repository.ErrMCPServerLimitExceeded) {
		return nil, ErrServerLimitExceeded
	}
	return item, err
}

func (s *Service) UpdateServer(ctx context.Context, serverID uint, input ServerInput) (*domainmcp.Server, error) {
	normalized, err := s.normalizeServerInput(input, false)
	if err != nil {
		return nil, err
	}
	update := repository.UpdateMCPServerInput{
		Name:        &normalized.Name,
		BaseURL:     &normalized.BaseURL,
		HeadersJSON: &normalized.HeadersJSON,
		Status:      &normalized.Status,
	}
	if normalized.AuthToken != "" {
		tokenEnc, encryptErr := s.encryptToken(normalized.AuthToken)
		if encryptErr != nil {
			return nil, encryptErr
		}
		update.AuthTokenEnc = &tokenEnc
	}
	return s.repo.UpdateServer(ctx, serverID, update)
}

func (s *Service) DeleteServer(ctx context.Context, serverID uint) error {
	return s.repo.DeleteServer(ctx, serverID)
}

func (s *Service) SyncServerTools(ctx context.Context, input SyncServerToolsInput) ([]domainmcp.Tool, error) {
	serverID := input.ServerID
	fail := func(err error) ([]domainmcp.Tool, error) {
		s.writeToolSyncEvent(ctx, systemeventapp.WriteInput{
			RequestID:  input.RequestID,
			Level:      "error",
			Source:     "mcp",
			Event:      "mcp.tools_sync_failed",
			Resource:   "mcp_server",
			ResourceID: fmt.Sprintf("%d", serverID),
			Message:    "MCP 工具同步失败",
			Detail: map[string]any{
				"server_id":  serverID,
				"error":      "MCP 工具同步失败",
				"error_code": mcpSyncErrorCode(err),
			},
		})
		return nil, err
	}

	server, err := s.repo.GetServer(ctx, serverID)
	if err != nil {
		return fail(err)
	}
	if err = s.validateServerBaseURL(server.BaseURL); err != nil {
		return fail(err)
	}
	if s.client == nil {
		return fail(ErrMCPClientUnavailable)
	}
	token, err := s.decryptToken(server.AuthTokenEnc)
	if err != nil {
		return fail(err)
	}
	headers, err := parseHeadersJSON(server.HeadersJSON)
	if err != nil {
		return fail(err)
	}
	tools, err := s.client.ListTools(ctx, portmcp.CallConfig{
		BaseURL:   server.BaseURL,
		AuthToken: token,
		TimeoutMS: mcpServerToolListTimeoutMS,
		Headers:   headers,
	})
	if err != nil {
		message := "MCP 工具同步失败"
		_, _ = s.repo.UpdateServer(ctx, serverID, repository.UpdateMCPServerInput{LastError: &message})
		return fail(err)
	}
	existingTools, err := s.repo.ListTools(ctx, serverID, false)
	if err != nil {
		return fail(err)
	}
	existingToolsByName := make(map[string]domainmcp.Tool, len(existingTools))
	for _, tool := range existingTools {
		existingToolsByName[tool.Name] = tool
	}
	items := make([]domainmcp.Tool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		schema := strings.TrimSpace(string(tool.InputSchema))
		if schema == "" {
			schema = "{}"
		}
		displayName := strings.TrimSpace(tool.Title)
		if displayName == "" {
			displayName = name
		}
		item := domainmcp.Tool{
			ServerID:            serverID,
			Name:                name,
			DisplayName:         displayName,
			Description:         strings.TrimSpace(tool.Description),
			InputSchemaJSON:     schema,
			AttachmentInputMode: domainmcp.AttachmentInputModeNone,
			Status:              "active",
		}
		if existing, ok := existingToolsByName[name]; ok {
			preserveCompatibleToolAttachmentConfig(&item, existing)
		}
		items = append(items, item)
	}
	if err = s.repo.ReplaceServerTools(ctx, serverID, items, input.OverwriteCustomizedMetadata); err != nil {
		return fail(err)
	}
	result, err := s.repo.ListTools(ctx, serverID, false)
	if err != nil {
		return fail(err)
	}
	s.writeToolSyncEvent(ctx, systemeventapp.WriteInput{
		RequestID:  input.RequestID,
		Level:      "info",
		Source:     "mcp",
		Event:      "mcp.tools_synced",
		Resource:   "mcp_server",
		ResourceID: fmt.Sprintf("%d", serverID),
		Message:    "MCP 工具已同步",
		Detail: map[string]any{
			"server_id":                     serverID,
			"tool_count":                    len(result),
			"overwrite_customized_metadata": input.OverwriteCustomizedMetadata,
		},
	})
	return result, nil
}

func mcpSyncErrorCode(err error) string {
	if code := apperr.Code(err); code != "" {
		return code
	}
	return "mcp.tools_sync_failed"
}

func preserveCompatibleToolAttachmentConfig(discovered *domainmcp.Tool, existing domainmcp.Tool) {
	if discovered == nil {
		return
	}
	config := toolAttachmentConfig{
		Mode:           strings.TrimSpace(existing.AttachmentInputMode),
		Argument:       strings.TrimSpace(existing.AttachmentArgument),
		Encoding:       strings.TrimSpace(existing.AttachmentEncoding),
		PromptArgument: strings.TrimSpace(existing.AttachmentPromptArgument),
	}
	if validateToolAttachmentConfig(config, discovered.InputSchemaJSON) != nil {
		return
	}
	discovered.AttachmentInputMode = config.Mode
	discovered.AttachmentArgument = config.Argument
	discovered.AttachmentEncoding = config.Encoding
	discovered.AttachmentPromptArgument = config.PromptArgument
}

func (s *Service) writeToolSyncEvent(ctx context.Context, input systemeventapp.WriteInput) {
	if s.systemEventWriter == nil {
		return
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	s.systemEventWriter.Write(ctx, input)
}

func (s *Service) ListTools(ctx context.Context, serverID uint, onlyActive bool) ([]domainmcp.Tool, error) {
	return s.repo.ListTools(ctx, serverID, onlyActive)
}

func (s *Service) ListAvailableTools(ctx context.Context) ([]domainmcp.Tool, error) {
	if !s.cfg.Snapshot().MCPEnable {
		return []domainmcp.Tool{}, nil
	}
	hideToolPrice, err := s.shouldHideToolPrice(ctx)
	if err != nil {
		return nil, err
	}
	servers, err := s.repo.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domainmcp.Tool, 0)
	for _, server := range servers {
		if server.Status != "active" {
			continue
		}
		tools, err := s.repo.ListTools(ctx, server.ID, true)
		if err != nil {
			return nil, err
		}
		for _, tool := range tools {
			tool.ServerName = server.Name
			if hideToolPrice {
				tool.PriceNanousd = 0
			}
			result = append(result, tool)
		}
	}
	return result, nil
}

// shouldHideToolPrice 判断用户侧工具列表是否隐藏价格；自用模式只记录用量，与模型列表保持一致不下发定价。
func (s *Service) shouldHideToolPrice(ctx context.Context) (bool, error) {
	if s.billingModeProvider == nil {
		return false, nil
	}
	mode, err := s.billingModeProvider.GetBillingMode(ctx)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(mode) == "self", nil
}

func (s *Service) UpdateTool(ctx context.Context, toolID uint, input ToolInput) (*domainmcp.Tool, error) {
	update, err := normalizeToolInput(input)
	if err != nil {
		return nil, err
	}
	if toolAttachmentConfigChanged(input) {
		tools, listErr := s.repo.ListToolsByIDs(ctx, []uint{toolID})
		if listErr != nil {
			return nil, listErr
		}
		if len(tools) != 1 || tools[0].ID != toolID {
			return nil, repository.ErrNotFound
		}
		config := mergedToolAttachmentConfig(tools[0], update)
		if validationErr := validateToolAttachmentConfig(config, tools[0].InputSchemaJSON); validationErr != nil {
			return nil, validationErr
		}
		if config.Mode == domainmcp.AttachmentInputModeNone {
			empty := ""
			update.AttachmentArgument = &empty
			update.AttachmentEncoding = &empty
			update.AttachmentPromptArgument = &empty
		}
	}
	return s.repo.UpdateTool(ctx, toolID, update)
}

func (s *Service) UpdateServerToolsStatus(ctx context.Context, serverID uint, toolIDs []uint, status string) ([]domainmcp.Tool, error) {
	normalized, err := normalizeToolStatus(status)
	if err != nil {
		return nil, err
	}
	if len(toolIDs) == 0 {
		return nil, ErrInvalidToolSelection
	}
	return s.repo.UpdateServerToolsStatus(ctx, serverID, toolIDs, normalized)
}

func (s *Service) ReorderServersWithTools(ctx context.Context, order []ReorderServerInput) ([]domainmcp.ServerWithTools, error) {
	if len(order) == 0 {
		return nil, ErrInvalidToolSelection
	}
	currentServers, err := s.repo.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	if len(order) != len(currentServers) {
		return nil, ErrInvalidToolSelection
	}

	allowedServers := make(map[uint]struct{}, len(currentServers))
	for _, server := range currentServers {
		allowedServers[server.ID] = struct{}{}
	}
	seenServers := make(map[uint]struct{}, len(order))
	for _, item := range order {
		if item.ServerID == 0 {
			return nil, ErrInvalidToolSelection
		}
		if _, ok := allowedServers[item.ServerID]; !ok {
			return nil, ErrInvalidToolSelection
		}
		if _, ok := seenServers[item.ServerID]; ok {
			return nil, ErrInvalidToolSelection
		}
		seenServers[item.ServerID] = struct{}{}

		currentTools, err := s.repo.ListTools(ctx, item.ServerID, false)
		if err != nil {
			return nil, err
		}
		if len(item.ToolIDs) != len(currentTools) {
			return nil, ErrInvalidToolSelection
		}
		allowedTools := make(map[uint]struct{}, len(currentTools))
		for _, tool := range currentTools {
			allowedTools[tool.ID] = struct{}{}
		}
		seenTools := make(map[uint]struct{}, len(item.ToolIDs))
		for _, toolID := range item.ToolIDs {
			if toolID == 0 {
				return nil, ErrInvalidToolSelection
			}
			if _, ok := allowedTools[toolID]; !ok {
				return nil, ErrInvalidToolSelection
			}
			if _, ok := seenTools[toolID]; ok {
				return nil, ErrInvalidToolSelection
			}
			seenTools[toolID] = struct{}{}
		}
	}
	repoOrder := make([]repository.ReorderMCPServerInput, 0, len(order))
	for _, item := range order {
		repoOrder = append(repoOrder, repository.ReorderMCPServerInput{
			ServerID: item.ServerID,
			ToolIDs:  item.ToolIDs,
		})
	}
	return s.repo.ReorderServersWithTools(ctx, repoOrder)
}

func (s *Service) normalizeServerInput(input ServerInput, requireToken bool) (ServerInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 128 {
		return ServerInput{}, ErrInvalidServerName
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return ServerInput{}, ErrInvalidServerBaseURL
	}
	if err = s.validateServerBaseURL(baseURL); err != nil {
		return ServerInput{}, ErrInvalidServerBaseURL
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "inactive":
	default:
		return ServerInput{}, ErrInvalidServerStatus
	}
	headersJSON := strings.TrimSpace(input.HeadersJSON)
	if headersJSON == "" {
		headersJSON = "{}"
	}
	if _, err = parseHeadersJSON(headersJSON); err != nil {
		return ServerInput{}, ErrInvalidServerHeaders
	}
	if requireToken {
		input.AuthToken = strings.TrimSpace(input.AuthToken)
	}
	return ServerInput{
		Name:        name,
		BaseURL:     baseURL,
		AuthToken:   strings.TrimSpace(input.AuthToken),
		HeadersJSON: headersJSON,
		Status:      status,
	}, nil
}

func (s *Service) validateServerBaseURL(raw string) error {
	if s == nil {
		return ErrInvalidServerBaseURL
	}
	return security.ValidateTrustedOutboundHTTPURL(raw)
}

func normalizeToolInput(input ToolInput) (repository.UpdateMCPToolInput, error) {
	update := repository.UpdateMCPToolInput{}
	if input.DisplayName != nil {
		displayName := strings.TrimSpace(*input.DisplayName)
		if len([]rune(displayName)) > 160 {
			return update, ErrInvalidToolName
		}
		update.DisplayName = &displayName
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len([]rune(description)) > 4096 {
			return update, ErrInvalidToolDesc
		}
		update.Description = &description
	}
	if input.PriceNanousd != nil {
		price := *input.PriceNanousd
		if price < 0 {
			return update, ErrInvalidToolPrice
		}
		update.PriceNanousd = &price
	}
	if input.Status != nil {
		status, err := normalizeToolStatus(*input.Status)
		if err != nil {
			return update, err
		}
		update.Status = &status
	}
	if input.AttachmentInputMode != nil {
		mode := strings.ToLower(strings.TrimSpace(*input.AttachmentInputMode))
		switch mode {
		case domainmcp.AttachmentInputModeNone, domainmcp.AttachmentInputModeImage:
			update.AttachmentInputMode = &mode
		default:
			return update, ErrInvalidToolAttachmentConfig
		}
	}
	if input.AttachmentArgument != nil {
		value := strings.TrimSpace(*input.AttachmentArgument)
		if len([]rune(value)) > 128 {
			return update, ErrInvalidToolAttachmentConfig
		}
		update.AttachmentArgument = &value
	}
	if input.AttachmentEncoding != nil {
		encoding := strings.ToLower(strings.TrimSpace(*input.AttachmentEncoding))
		switch encoding {
		case "", domainmcp.AttachmentEncodingBase64, domainmcp.AttachmentEncodingDataURL:
			update.AttachmentEncoding = &encoding
		default:
			return update, ErrInvalidToolAttachmentConfig
		}
	}
	if input.AttachmentPromptArgument != nil {
		value := strings.TrimSpace(*input.AttachmentPromptArgument)
		if len([]rune(value)) > 128 {
			return update, ErrInvalidToolAttachmentConfig
		}
		update.AttachmentPromptArgument = &value
	}
	return update, nil
}

type toolAttachmentConfig struct {
	Mode           string
	Argument       string
	Encoding       string
	PromptArgument string
}

func toolAttachmentConfigChanged(input ToolInput) bool {
	return input.AttachmentInputMode != nil ||
		input.AttachmentArgument != nil ||
		input.AttachmentEncoding != nil ||
		input.AttachmentPromptArgument != nil
}

func mergedToolAttachmentConfig(tool domainmcp.Tool, update repository.UpdateMCPToolInput) toolAttachmentConfig {
	config := toolAttachmentConfig{
		Mode:           strings.TrimSpace(tool.AttachmentInputMode),
		Argument:       strings.TrimSpace(tool.AttachmentArgument),
		Encoding:       strings.TrimSpace(tool.AttachmentEncoding),
		PromptArgument: strings.TrimSpace(tool.AttachmentPromptArgument),
	}
	if update.AttachmentInputMode != nil {
		config.Mode = *update.AttachmentInputMode
	}
	if update.AttachmentArgument != nil {
		config.Argument = *update.AttachmentArgument
	}
	if update.AttachmentEncoding != nil {
		config.Encoding = *update.AttachmentEncoding
	}
	if update.AttachmentPromptArgument != nil {
		config.PromptArgument = *update.AttachmentPromptArgument
	}
	if update.AttachmentInputMode != nil && *update.AttachmentInputMode == domainmcp.AttachmentInputModeNone {
		config.Argument = ""
		config.Encoding = ""
		config.PromptArgument = ""
	}
	return config
}

func validateToolAttachmentConfig(config toolAttachmentConfig, schemaJSON string) error {
	if config.Mode == domainmcp.AttachmentInputModeNone {
		if config.Argument != "" || config.Encoding != "" || config.PromptArgument != "" {
			return ErrInvalidToolAttachmentConfig
		}
		return nil
	}
	if config.Mode != domainmcp.AttachmentInputModeImage || config.Argument == "" {
		return ErrInvalidToolAttachmentConfig
	}
	switch config.Encoding {
	case domainmcp.AttachmentEncodingBase64, domainmcp.AttachmentEncodingDataURL:
	default:
		return ErrInvalidToolAttachmentConfig
	}
	if config.PromptArgument == config.Argument {
		return ErrInvalidToolAttachmentConfig
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(schemaJSON)), &schema); err != nil || len(schema.Properties) == 0 {
		return ErrInvalidToolAttachmentConfig
	}
	if !toolSchemaPropertyAcceptsString(schemaJSON, config.Argument) {
		return ErrInvalidToolAttachmentConfig
	}
	if config.PromptArgument != "" && !toolSchemaPropertyAcceptsString(schemaJSON, config.PromptArgument) {
		return ErrInvalidToolAttachmentConfig
	}
	for _, name := range schema.Required {
		name = strings.TrimSpace(name)
		if name != "" && name != config.Argument && name != config.PromptArgument {
			return ErrInvalidToolAttachmentConfig
		}
	}
	return nil
}

func normalizeToolStatus(status string) (string, error) {
	normalized := strings.TrimSpace(status)
	switch normalized {
	case "active", "inactive":
		return normalized, nil
	default:
		return "", ErrInvalidToolStatus
	}
}

func (s *Service) encryptToken(token string) (string, error) {
	return secretbox.EncryptString(s.cfg.Snapshot().DataEncryptionKey, token)
}

func (s *Service) decryptToken(encrypted string) (string, error) {
	return secretbox.DecryptString(s.cfg.Snapshot().DataEncryptionKey, encrypted)
}

func parseHeadersJSON(raw string) (map[string]string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]string{}, nil
	}
	payload := map[string]string{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidServerHeaders, err)
	}
	result := make(map[string]string, len(payload))
	for key, item := range payload {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		result[headerKey] = strings.TrimSpace(item)
	}
	return result, nil
}
