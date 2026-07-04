package channel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	modelProbeStatusSuccess     = "success"
	modelProbeStatusFailed      = "failed"
	modelProbeStatusUnsupported = "unsupported"

	modelProbeDebugBodyLimit = 8 * 1024
	modelProbeReadTimeoutMS  = 30000
	modelProbeMaxConcurrency = 4
)

// TestModel 使用当前活跃路由规则对平台模型执行一次轻量连通性测试。
func (s *Service) TestModel(ctx context.Context, modelID uint, input ModelProbeInput) (*ModelProbeResult, error) {
	model, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}

	taskType := normalizeModelProbeTaskType(input.TaskType, model.KindsJSON, "")
	rows, err := s.repo.ListActiveRoutesByModel(ctx, model.PlatformModelName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return failedModelProbeResult(repository.ChannelUpstreamRouteRow{
			PlatformModelID:   model.ID,
			PlatformModelName: model.PlatformModelName,
			ModelKindsJSON:    model.KindsJSON,
		}, "route_not_found", "no active route is available for this model"), nil
	}

	var firstFailure *ModelProbeResult
	for _, row := range rows {
		if !IsRouteAllowedForTask(taskType, row.ModelKindsJSON, row.Protocol) {
			continue
		}
		result, ok := s.prepareModelProbeRoute(row)
		if !ok {
			if firstFailure == nil {
				firstFailure = result
			}
			continue
		}
		return s.probeRoute(ctx, row)
	}
	if firstFailure != nil {
		return firstFailure, nil
	}
	return failedModelProbeResult(repository.ChannelUpstreamRouteRow{
		PlatformModelID:   model.ID,
		PlatformModelName: model.PlatformModelName,
		ModelKindsJSON:    model.KindsJSON,
	}, "route_not_found", "no active route matches the selected test type"), nil
}

// TestModelAll 对平台模型的全部可测试活跃路由执行轻量连通性测试。
func (s *Service) TestModelAll(ctx context.Context, modelID uint, input ModelProbeInput) (*ModelProbeBatchResult, error) {
	model, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.ListActiveRoutesByModel(ctx, model.PlatformModelName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		result := failedModelProbeResult(repository.ChannelUpstreamRouteRow{
			PlatformModelID:   model.ID,
			PlatformModelName: model.PlatformModelName,
			ModelKindsJSON:    model.KindsJSON,
		}, "route_not_found", "no active route is available for this model")
		return summarizeModelProbeResults([]ModelProbeResult{*result}), nil
	}

	routes := filterModelProbeRows(rows, input.TaskType)
	if len(routes) == 0 {
		result := failedModelProbeResult(repository.ChannelUpstreamRouteRow{
			PlatformModelID:   model.ID,
			PlatformModelName: model.PlatformModelName,
			ModelKindsJSON:    model.KindsJSON,
		}, "route_not_found", "no active route matches the selected test type")
		return summarizeModelProbeResults([]ModelProbeResult{*result}), nil
	}

	results := make([]ModelProbeResult, len(routes))
	sem := make(chan struct{}, modelProbeMaxConcurrency)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for index, row := range routes {
		wg.Add(1)
		go func(index int, row repository.ChannelUpstreamRouteRow) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errMu.Lock()
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				errMu.Unlock()
				return
			}

			result, err := s.probeRoute(ctx, row)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
			}
			results[index] = *result
		}(index, row)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return summarizeModelProbeResults(results), nil
}

// TestUpstreamModelRoute 对指定上游模型路由绑定执行一次轻量连通性测试。
func (s *Service) TestUpstreamModelRoute(ctx context.Context, upstreamID uint, routeID uint, input ModelProbeInput) (*ModelProbeResult, error) {
	row, err := s.buildProbeRouteFromBinding(ctx, upstreamID, routeID)
	if err != nil {
		return nil, err
	}

	taskType := normalizeModelProbeTaskType(input.TaskType, row.ModelKindsJSON, row.Protocol)
	if !IsRouteAllowedForTask(taskType, row.ModelKindsJSON, row.Protocol) {
		return failedModelProbeResult(*row, "protocol_mismatch", "route protocol does not match the model type"), nil
	}
	return s.probeRoute(ctx, *row)
}

func filterModelProbeRows(rows []repository.ChannelUpstreamRouteRow, taskType string) []repository.ChannelUpstreamRouteRow {
	result := make([]repository.ChannelUpstreamRouteRow, 0, len(rows))
	for _, row := range rows {
		resolvedTaskType := normalizeModelProbeTaskType(taskType, row.ModelKindsJSON, row.Protocol)
		if IsRouteAllowedForTask(resolvedTaskType, row.ModelKindsJSON, row.Protocol) {
			result = append(result, row)
		}
	}
	return result
}

func summarizeModelProbeResults(results []ModelProbeResult) *ModelProbeBatchResult {
	summary := &ModelProbeBatchResult{
		TotalCount: len(results),
		Results:    results,
	}
	for _, result := range results {
		switch {
		case result.Success:
			summary.SuccessCount++
		case result.Status == modelProbeStatusUnsupported:
			summary.UnsupportedCount++
			summary.FailedCount++
		default:
			summary.FailedCount++
		}
	}
	return summary
}

// buildProbeRouteFromBinding 将路由绑定、平台模型和上游配置拼成探针所需的完整路由。
func (s *Service) buildProbeRouteFromBinding(ctx context.Context, upstreamID uint, routeID uint) (*repository.ChannelUpstreamRouteRow, error) {
	binding, err := s.repo.GetUpstreamModelRouteByID(ctx, upstreamID, routeID)
	if err != nil {
		return nil, err
	}
	upstream, err := s.repo.GetUpstreamByID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	model, err := s.repo.GetModelByID(ctx, binding.PlatformModelID)
	if err != nil {
		return nil, err
	}

	return &repository.ChannelUpstreamRouteRow{
		RouteID:                    binding.RouteID,
		UpstreamModelID:            binding.ID,
		UpstreamID:                 binding.UpstreamID,
		UpstreamName:               upstream.Name,
		PlatformModelID:            model.ID,
		PlatformModelName:          model.PlatformModelName,
		ModelVendor:                model.Vendor,
		ModelIcon:                  model.Icon,
		ModelKindsJSON:             model.KindsJSON,
		ModelCapabilitiesJSON:      model.CapabilitiesJSON,
		ModelSystemPrompt:          model.SystemPrompt,
		Protocol:                   binding.Protocol,
		BaseURL:                    upstream.BaseURL,
		APIKeysEnc:                 upstream.APIKeysEnc,
		ConnectTimeoutMS:           upstream.ConnectTimeoutMS,
		ReadTimeoutMS:              upstream.ReadTimeoutMS,
		StreamIdleTimeoutMS:        upstream.StreamIdleTimeoutMS,
		HeadersJSON:                upstream.HeadersJSON,
		RouteHeadersJSON:           binding.HeadersJSON,
		BindingCode:                binding.BindingCode,
		UpstreamModelName:          binding.UpstreamModelName,
		Weight:                     binding.Weight,
		RoutePriority:              binding.Priority,
		UpstreamCbFailureThreshold: upstream.CbFailureThreshold,
		UpstreamCbModelThreshold:   upstream.CbModelThreshold,
		UpstreamCbThresholdLogic:   upstream.CbThresholdLogic,
		UpstreamCbDurationMin:      upstream.CbDurationMin,
		UpstreamCbWindowMin:        upstream.CbWindowMin,
		ModelCbFailureThreshold:    binding.CbFailureThreshold,
		ModelCbDurationMin:         binding.CbDurationMin,
		ModelCbWindowMin:           binding.CbWindowMin,
	}, nil
}

// probeRoute 校验并执行一次不会记录计费、消息和熔断状态的轻量模型调用。
func (s *Service) probeRoute(ctx context.Context, row repository.ChannelUpstreamRouteRow) (*ModelProbeResult, error) {
	if result, ok := s.prepareModelProbeRoute(row); !ok {
		return result, nil
	}
	if !isLightweightModelProbeProtocol(row.Protocol) {
		return unsupportedModelProbeResult(row, "this protocol requires media input or may create billable media output"), nil
	}
	if s.llmClient == nil {
		return nil, errors.New("llm client is not configured")
	}

	keyCfg, err := s.parseAPIKeysConfig(row.APIKeysEnc)
	if err != nil {
		return failedModelProbeResult(row, "config_invalid", "api key configuration is invalid"), nil
	}
	apiKey, err := selectProbeAPIKey(keyCfg)
	if err != nil {
		return failedModelProbeResult(row, "no_active_key", "no active api key is configured"), nil
	}

	resolved := buildResolvedRoute(row, apiKey)
	attributionReferer, attributionTitle := s.llmAttribution()
	routeConfig := modelProbeRouteConfig(resolved, attributionReferer, attributionTitle)
	input := llm.GenerateInput{
		Messages: []llm.Message{
			{Role: "user", Content: "Reply with OK."},
		},
		DisableTools: true,
		Options: map[string]interface{}{
			"max_output_tokens":     1,
			"max_completion_tokens": 1,
			"max_tokens":            1,
			"temperature":           0,
		},
	}

	startedAt := time.Now()
	output, err := s.llmClient.Generate(ctx, routeConfig, input)
	latencyMS := time.Since(startedAt).Milliseconds()
	if err != nil {
		return s.failedModelProbeFromError(row, routeConfig, err, latencyMS), nil
	}

	return &ModelProbeResult{
		Success:            true,
		Status:             modelProbeStatusSuccess,
		LatencyMS:          latencyMS,
		Protocol:           row.Protocol,
		Endpoint:           llm.DefaultEndpointForAdapter(row.Protocol),
		PlatformModelID:    row.PlatformModelID,
		PlatformModelName:  strings.TrimSpace(row.PlatformModelName),
		UpstreamID:         row.UpstreamID,
		UpstreamName:       strings.TrimSpace(row.UpstreamName),
		UpstreamModelID:    row.UpstreamModelID,
		UpstreamModelName:  strings.TrimSpace(row.UpstreamModelName),
		RouteID:            row.RouteID,
		BindingCode:        strings.TrimSpace(row.BindingCode),
		UpstreamStatusCode: modelProbeDebugStatusCode(output.Debug),
		Debug:              sanitizeModelProbeDebug(output.Debug, routeConfig),
	}, nil
}

// prepareModelProbeRoute 执行不会触达上游的本地配置校验。
func (s *Service) prepareModelProbeRoute(row repository.ChannelUpstreamRouteRow) (*ModelProbeResult, bool) {
	if strings.TrimSpace(row.Protocol) == "" || !llm.IsImplementedAdapter(row.Protocol) {
		return failedModelProbeResult(row, "unsupported_protocol", "route protocol is not supported"), false
	}
	if strings.TrimSpace(row.UpstreamModelName) == "" {
		return failedModelProbeResult(row, "model_not_found", "upstream model name is empty"), false
	}
	if err := s.validateUpstreamBaseURL(row.BaseURL); err != nil {
		return failedModelProbeResult(row, "config_invalid", "upstream base url is invalid"), false
	}
	keyCfg, err := s.parseAPIKeysConfig(row.APIKeysEnc)
	if err != nil {
		return failedModelProbeResult(row, "config_invalid", "api key configuration is invalid"), false
	}
	if _, err = selectProbeAPIKey(keyCfg); err != nil {
		return failedModelProbeResult(row, "no_active_key", "no active api key is configured"), false
	}
	return nil, true
}

func modelProbeRouteConfig(route *ResolvedRoute, attributionReferer string, attributionTitle string) llm.RouteConfig {
	readTimeoutMS := route.ReadTimeoutMS
	if readTimeoutMS <= 0 || readTimeoutMS > modelProbeReadTimeoutMS {
		readTimeoutMS = modelProbeReadTimeoutMS
	}
	return llm.RouteConfig{
		Protocol:            route.Protocol,
		BaseURL:             route.BaseURL,
		APIKey:              route.APIKey,
		HeadersJSON:         route.HeadersJSON,
		ConnectTimeoutMS:    route.ConnectTimeoutMS,
		ReadTimeoutMS:       readTimeoutMS,
		StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
		Endpoint:            llm.DefaultEndpointForAdapter(route.Protocol),
		UpstreamModel:       route.UpstreamModel,
		AttributionReferer:  attributionReferer,
		AttributionTitle:    attributionTitle,
	}
}

func selectProbeAPIKey(cfg domainchannel.APIKeysConfig) (string, error) {
	for _, item := range cfg.Keys {
		if (item.Status == "" || item.Status == "active") && strings.TrimSpace(item.Key) != "" {
			return item.Key, nil
		}
	}
	return "", ErrNoActiveKey
}

func isLightweightModelProbeProtocol(protocol string) bool {
	switch llm.NormalizeAdapter(protocol) {
	case llm.AdapterOpenAIResponses,
		llm.AdapterOpenRouterChat,
		llm.AdapterOpenRouterResponses,
		llm.AdapterOpenAIChatCompletions,
		llm.AdapterAnthropicMessages,
		llm.AdapterGoogleGenerateContent,
		llm.AdapterGeminiInteractions,
		llm.AdapterXAIResponses:
		return true
	default:
		return false
	}
}

func normalizeModelProbeTaskType(raw string, kindsJSON string, protocol string) string {
	if strings.TrimSpace(raw) != "" {
		return NormalizeTaskType(raw)
	}
	protocol = llm.NormalizeAdapter(protocol)
	kinds := parseKinds(kindsJSON)
	supportsImageGeneration := llm.IsImageGenerationAdapter(protocol)
	supportsImageEdit := llm.IsImageEditAdapter(protocol)
	if supportsImageGeneration && hasModelKind(kinds, modelKindImageGen) {
		return TaskTypeImageGeneration
	}
	if supportsImageEdit && hasModelKind(kinds, modelKindImageEdit) {
		return TaskTypeImageEdit
	}
	if supportsImageGeneration {
		return TaskTypeImageGeneration
	}
	if supportsImageEdit {
		return TaskTypeImageEdit
	}
	if hasModelKind(kinds, modelKindImageGen) {
		return TaskTypeImageGeneration
	}
	return TaskTypeChat
}

func unsupportedModelProbeResult(row repository.ChannelUpstreamRouteRow, message string) *ModelProbeResult {
	result := failedModelProbeResult(row, "unsupported_protocol", message)
	result.Status = modelProbeStatusUnsupported
	return result
}

func failedModelProbeResult(row repository.ChannelUpstreamRouteRow, code string, message string) *ModelProbeResult {
	return &ModelProbeResult{
		Success:           false,
		Status:            modelProbeStatusFailed,
		ErrorCode:         code,
		ErrorMessage:      strings.TrimSpace(message),
		Protocol:          row.Protocol,
		Endpoint:          llm.DefaultEndpointForAdapter(row.Protocol),
		PlatformModelID:   row.PlatformModelID,
		PlatformModelName: strings.TrimSpace(row.PlatformModelName),
		UpstreamID:        row.UpstreamID,
		UpstreamName:      strings.TrimSpace(row.UpstreamName),
		UpstreamModelID:   row.UpstreamModelID,
		UpstreamModelName: strings.TrimSpace(row.UpstreamModelName),
		RouteID:           row.RouteID,
		BindingCode:       strings.TrimSpace(row.BindingCode),
	}
}

func (s *Service) failedModelProbeFromError(row repository.ChannelUpstreamRouteRow, route llm.RouteConfig, err error, latencyMS int64) *ModelProbeResult {
	code, message, statusCode := classifyModelProbeError(err)
	message = sanitizeSensitiveText(message, route.APIKey, route.BaseURL)
	result := failedModelProbeResult(row, code, message)
	result.LatencyMS = latencyMS
	result.UpstreamStatusCode = statusCode

	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		result.Debug = sanitizeModelProbeDebug(upstreamErr.Debug, route)
	}
	return result
}

func classifyModelProbeError(err error) (string, string, int) {
	if err == nil {
		return "", "", 0
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "upstream request timed out", 0
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", "upstream request timed out", 0
	}
	if errors.Is(err, llm.ErrUnsupportedAdapter) {
		return "unsupported_protocol", "route protocol is not supported", 0
	}

	var upstreamErr *llm.UpstreamError
	if errors.As(err, &upstreamErr) {
		summary := strings.TrimSpace(upstreamErr.Message)
		if summary == "" {
			summary = fmt.Sprintf("upstream_status_%d", upstreamErr.StatusCode)
		}
		if upstreamErr.StatusCode >= 200 && upstreamErr.StatusCode < 300 {
			return "response_incompatible", "upstream response format is incompatible: " + summary, upstreamErr.StatusCode
		}
		switch upstreamErr.StatusCode {
		case 401, 403:
			return "auth_failed", "authentication failed: " + summary, upstreamErr.StatusCode
		case 404:
			return "model_not_found", "model or endpoint not found: " + summary, upstreamErr.StatusCode
		case 408, 504:
			return "timeout", "upstream request timed out: " + summary, upstreamErr.StatusCode
		case 429:
			return "rate_limited", "upstream rate limit reached: " + summary, upstreamErr.StatusCode
		case 400, 422:
			return "request_invalid", "upstream rejected the test request: " + summary, upstreamErr.StatusCode
		default:
			if upstreamErr.StatusCode >= 500 {
				return "upstream_unavailable", "upstream service unavailable: " + summary, upstreamErr.StatusCode
			}
			return "upstream_request_failed", "upstream request failed: " + summary, upstreamErr.StatusCode
		}
	}

	message := strings.TrimSpace(err.Error())
	lowerMessage := strings.ToLower(message)
	switch {
	case strings.Contains(lowerMessage, "timeout"):
		return "timeout", "upstream request timed out", 0
	case strings.Contains(lowerMessage, "unsupported"):
		return "request_invalid", "request parameter is not supported by upstream", 0
	case strings.Contains(lowerMessage, "invalid base url"):
		return "config_invalid", "upstream base url is invalid", 0
	case strings.Contains(lowerMessage, "parse") || strings.Contains(lowerMessage, "invalid response") || strings.Contains(lowerMessage, "missing"):
		return "response_incompatible", "upstream response format is incompatible: " + message, 0
	default:
		return "network_error", "upstream request failed: " + message, 0
	}
}

func sanitizeModelProbeDebug(debug *llm.UpstreamDebugSnapshot, route llm.RouteConfig) *ModelProbeDebugView {
	if debug == nil {
		return nil
	}
	return &ModelProbeDebugView{
		Request: ModelProbeDebugRequestView{
			Method:  strings.TrimSpace(debug.Request.Method),
			Path:    sanitizeSensitiveText(debug.Request.Path, route.APIKey, route.BaseURL),
			Headers: sanitizeProbeHeaders(debug.Request.Headers, false),
			Body:    truncateProbeDebugBody(sanitizeSensitiveText(debug.Request.Body, route.APIKey, route.BaseURL)),
		},
		Response: ModelProbeDebugResponseView{
			StatusCode: debug.Response.StatusCode,
			Headers:    sanitizeProbeHeaders(debug.Response.Headers, true),
			Body:       truncateProbeDebugBody(sanitizeSensitiveText(debug.Response.Body, route.APIKey, route.BaseURL)),
		},
	}
}

func modelProbeDebugStatusCode(debug *llm.UpstreamDebugSnapshot) int {
	if debug == nil {
		return 0
	}
	return debug.Response.StatusCode
}

func sanitizeProbeHeaders(headers map[string]string, response bool) map[string]string {
	result := make(map[string]string)
	for key, value := range headers {
		headerKey := strings.TrimSpace(key)
		switch strings.ToLower(headerKey) {
		case "content-type":
			result[headerKey] = strings.TrimSpace(value)
		case "accept":
			if !response {
				result[headerKey] = strings.TrimSpace(value)
			}
		}
	}
	return result
}

func sanitizeSensitiveText(raw string, apiKey string, baseURL string) string {
	text := raw
	if secret := strings.TrimSpace(apiKey); secret != "" {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	if base := strings.TrimSpace(baseURL); base != "" {
		text = strings.ReplaceAll(text, base, "[redacted]")
		if parsed, err := url.Parse(base); err == nil && strings.TrimSpace(parsed.Host) != "" {
			text = strings.ReplaceAll(text, parsed.Host, "[redacted]")
		}
	}
	return text
}

func truncateProbeDebugBody(raw string) string {
	if len(raw) <= modelProbeDebugBodyLimit {
		return raw
	}
	runes := []rune(raw)
	if len(runes) <= modelProbeDebugBodyLimit {
		return raw
	}
	return string(runes[:modelProbeDebugBodyLimit]) + "...[truncated]"
}
