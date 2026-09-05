package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// geminiInteractionsAdapter 实现 Google Gemini Interactions API。
type geminiInteractionsAdapter struct {
	client *Client
}

func (a *geminiInteractionsAdapter) Name() string { return portllm.AdapterGeminiInteractions }

func (a *geminiInteractionsAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	return a.client.generateGeminiInteraction(ctx, route, input)
}

func (a *geminiInteractionsAdapter) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return a.client.generateGeminiInteractionStream(ctx, route, input, onEvent)
}

func (a *geminiInteractionsAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	return a.client.listModelsGemini(ctx, route)
}

func (c *Client) generateGeminiInteraction(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	base := geminiBaseURL(route)
	requestURL := buildGeminiInteractionsURL(base)
	requestBody, err := buildGeminiInteractionRequestBody(route, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := c.newGeminiRequest(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route, &input)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRouteGenerationRequest(route, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, portllm.MarkRequestAccepted(err)
		}
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseGeminiError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		return nil, portllm.MarkRequestAccepted(err)
	}
	return output, nil
}

func (c *Client) generateGeminiInteractionStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	base := geminiBaseURL(route)
	requestURL := buildGeminiInteractionsURL(base)
	requestBody, err := buildGeminiInteractionRequestBody(route, input)
	if err != nil {
		return nil, err
	}
	requestBody["stream"] = true
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	firstByteCtx, firstByteCancel := context.WithCancel(ctx)
	defer firstByteCancel()

	firstByteTimer := time.AfterFunc(resolveReadTimeout(route.ReadTimeoutMS), firstByteCancel)
	defer firstByteTimer.Stop()

	req, err := c.newGeminiRequest(firstByteCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route, &input)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.doRouteGenerationRequest(route, req)
	firstByteTimer.Stop()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := readUpstreamBody(resp.Body)
		return nil, parseGeminiError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	result := &portllm.GenerateOutput{
		ToolCalls: make([]portllm.ToolCall, 0),
	}
	idleReader := newIdleTimeoutReader(resp.Body, resolveStreamIdleTimeout(route.StreamIdleTimeoutMS))
	streamBody := newUpstreamBodyRecorder(idleReader)
	if err = consumeGeminiInteractionStream(streamBody, result, onEvent); err != nil {
		return nil, portllm.MarkRequestAccepted(attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err))))
	}
	for index := range result.GeneratedImages {
		if result.GeneratedImages[index].RevisedPrompt == "" {
			result.GeneratedImages[index].RevisedPrompt = result.Text
		}
	}
	return result, nil
}

func buildGeminiInteractionsURL(base string) string {
	return buildGeminiEndpointURL(base, "/interactions")
}

func buildGeminiInteractionRequestBody(route portllm.RouteConfig, input portllm.GenerateInput) (map[string]any, error) {
	model := strings.TrimSpace(route.UpstreamModel)
	if model == "" {
		return nil, fmt.Errorf("interaction model required")
	}
	interactionInput := buildGeminiInteractionInput(input.Messages)
	if geminiInteractionInputEmpty(interactionInput) {
		return nil, fmt.Errorf("interaction input required")
	}
	payload := map[string]any{
		"model": model,
		"input": interactionInput,
	}
	if format := buildGeminiInteractionResponseFormat(route, input.Options); !geminiInteractionInputEmpty(format) {
		payload["response_format"] = format
	}
	if config := buildGeminiInteractionGenerationConfig(input.Options); len(config) > 0 {
		payload["generation_config"] = config
	}
	if instructions := strings.TrimSpace(input.Instructions); instructions != "" {
		payload["system_instruction"] = instructions
	}
	if previousID := strings.TrimSpace(input.PreviousResponseID); previousID != "" {
		payload["previous_interaction_id"] = previousID
	}
	providerTools, toolDefinitions, toolsEnabled, err := toolDeclarationsForInput(input)
	if err != nil {
		return nil, err
	}
	if toolsEnabled {
		appendToolDeclarations(payload, providerTools, buildGeminiInteractionTools(toolDefinitions))
	}
	applyProviderOptions(payload, input.Options, geminiInteractionsProtectedProviderOptionKeys()...)
	return payload, nil
}

func buildGeminiInteractionInput(messages []portllm.Message) any {
	steps := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		stepType := geminiInteractionStepType(message.Role)
		contentMessage := message
		contentMessage.ToolCalls = nil
		contentMessage.ToolResults = nil
		if stepType != "" {
			content := buildGeminiInteractionContent(contentMessage)
			if !geminiInteractionInputEmpty(content) {
				steps = append(steps, map[string]any{
					"type":    stepType,
					"content": content,
				})
			}
		}
		for _, call := range message.ToolCalls {
			if functionCall := buildGeminiInteractionFunctionCall(call); len(functionCall) > 0 {
				steps = append(steps, functionCall)
			}
		}
		for _, result := range message.ToolResults {
			if functionResult := buildGeminiInteractionFunctionResult(result); len(functionResult) > 0 {
				steps = append(steps, functionResult)
			}
		}
	}
	if len(steps) == 1 && steps[0]["type"] == "user_input" {
		return steps[0]["content"]
	}
	return steps
}

func geminiInteractionStepType(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "assistant", "model":
		return "model_output"
	case "user", "tool":
		return "user_input"
	default:
		return ""
	}
}

func buildGeminiInteractionContent(message portllm.Message) any {
	if len(message.Parts) == 0 {
		return strings.TrimSpace(message.Content)
	}
	items := make([]map[string]any, 0, len(message.Parts)+1)
	if text := strings.TrimSpace(message.Content); text != "" {
		items = append(items, map[string]any{"type": "text", "text": text})
	}
	for _, part := range message.Parts {
		switch part.Kind {
		case portllm.ContentPartText, portllm.ContentPartFile:
			text := strings.TrimSpace(part.Text)
			if text == "" {
				continue
			}
			items = append(items, map[string]any{"type": "text", "text": text})
		case portllm.ContentPartImage:
			if len(part.Data) == 0 {
				continue
			}
			mimeType := strings.TrimSpace(part.MimeType)
			if mimeType == "" {
				mimeType = "image/png"
			}
			items = append(items, map[string]any{
				"type":      "image",
				"mime_type": mimeType,
				"data":      base64.StdEncoding.EncodeToString(part.Data),
			})
		}
	}
	if len(items) == 1 && items[0]["type"] == "text" {
		return strings.TrimSpace(getString(items[0]["text"]))
	}
	return items
}

func buildGeminiInteractionFunctionCall(call portllm.ToolCall) map[string]any {
	name := strings.TrimSpace(call.ToolName)
	if name == "" {
		return nil
	}
	args := strings.TrimSpace(call.ArgumentsJSON)
	if args == "" {
		args = "{}"
	}
	arguments := map[string]any{}
	if err := json.Unmarshal([]byte(args), &arguments); err != nil {
		arguments = map[string]any{"arguments": args}
	}
	item := map[string]any{
		"type":      "function_call",
		"name":      name,
		"arguments": arguments,
	}
	if id := strings.TrimSpace(call.ToolCallID); id != "" {
		item["id"] = id
	}
	return item
}

func buildGeminiInteractionFunctionResult(result portllm.ToolResult) map[string]any {
	name := strings.TrimSpace(result.ToolName)
	if name == "" {
		return nil
	}
	item := map[string]any{
		"type":   "function_result",
		"name":   name,
		"result": geminiInteractionFunctionResultContent(result),
	}
	if id := strings.TrimSpace(result.ToolCallID); id != "" {
		item["call_id"] = id
	}
	return item
}

func geminiInteractionFunctionResultContent(result portllm.ToolResult) []map[string]any {
	output := map[string]any{}
	raw := strings.TrimSpace(result.OutputJSON)
	if raw != "" {
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			if payload, ok := decoded.(map[string]any); ok {
				output = payload
			} else {
				output["content"] = decoded
			}
		} else {
			output["content"] = raw
		}
	}
	if errText := strings.TrimSpace(result.Error); errText != "" {
		output["error"] = errText
	}
	if len(output) == 0 {
		output["content"] = ""
	}
	text, err := json.Marshal(output)
	if err != nil {
		text = []byte(`{"content":""}`)
	}
	return []map[string]any{{
		"type": "text",
		"text": string(text),
	}}
}

func geminiInteractionInputEmpty(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case map[string]any:
		return len(typed) == 0
	case []map[string]any:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return value == nil
	}
}

func buildGeminiInteractionResponseFormat(route portllm.RouteConfig, options map[string]any) any {
	if rawFormats, ok := firstGeminiInteractionResponseFormatList(options); ok {
		items := make([]any, 0, len(rawFormats))
		for _, rawFormat := range rawFormats {
			if format := normalizeGeminiInteractionResponseFormat(route, rawFormat); len(format) > 0 {
				items = append(items, format)
			}
		}
		if len(items) > 0 {
			return items
		}
	}
	raw := modelParamMap(options, "response_format")
	return normalizeGeminiInteractionResponseFormat(route, raw)
}

func normalizeGeminiInteractionResponseFormat(route portllm.RouteConfig, raw map[string]any) map[string]any {
	responseType := geminiInteractionResponseType(getString(raw["type"]))
	if responseType == "" {
		switch normalizeEndpoint(route.Endpoint) {
		case portllm.EndpointImageGenerations, portllm.EndpointImageEdits:
			responseType = "image"
		}
	}
	if responseType == "" {
		return nil
	}
	format := map[string]any{
		"type": responseType,
	}
	if responseType == "video" {
		format["delivery"] = "uri"
	}
	if aspectRatio := geminiInteractionAspectRatio(getString(raw["aspect_ratio"]), responseType); aspectRatio != "" {
		format["aspect_ratio"] = aspectRatio
	}
	if imageSize := geminiInteractionImageSize(getString(raw["image_size"])); imageSize != "" {
		format["image_size"] = imageSize
	}
	if mimeType := geminiInteractionMIMEType(getString(raw["mime_type"]), responseType); mimeType != "" {
		format["mime_type"] = mimeType
	}
	if responseType == "text" && format["mime_type"] == "application/json" {
		if schema := asMap(raw["schema"]); len(schema) > 0 {
			format["schema"] = schema
		}
	}
	return format
}

func firstGeminiInteractionResponseFormatList(options map[string]any) ([]map[string]any, bool) {
	value, ok := options["response_format"]
	if !ok {
		return nil, false
	}
	switch typed := value.(type) {
	case []map[string]any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if len(item) > 0 {
				items = append(items, item)
			}
		}
		return items, len(items) > 0
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, raw := range typed {
			item := asMap(raw)
			if len(item) > 0 {
				items = append(items, item)
			}
		}
		return items, len(items) > 0
	default:
		return nil, false
	}
}

func geminiInteractionResponseType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "image":
		return "image"
	case "video":
		return "video"
	case "text":
		return "text"
	default:
		return ""
	}
}

func buildGeminiInteractionGenerationConfig(options map[string]any) map[string]any {
	config := map[string]any{}
	raw := modelParamMap(options, "generation_config")
	for key, value := range raw {
		if strings.TrimSpace(key) != "" && key != "video_config" {
			config[key] = value
		}
	}
	if videoConfig := buildGeminiInteractionVideoConfig(modelParamMap(raw, "video_config")); len(videoConfig) > 0 {
		config["video_config"] = videoConfig
	}
	return config
}

func buildGeminiInteractionVideoConfig(raw map[string]any) map[string]any {
	config := map[string]any{}
	if task := geminiInteractionVideoTask(getString(raw["task"])); task != "" {
		config["task"] = task
	}
	return config
}

func geminiInteractionAspectRatio(value string, responseType string) string {
	normalized := strings.TrimSpace(value)
	switch responseType {
	case "video":
		switch normalized {
		case "9:16", "16:9":
			return normalized
		default:
			return ""
		}
	default:
		switch normalized {
		case "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9", "1:4", "4:1", "1:8", "8:1":
			return normalized
		default:
			return ""
		}
	}
}

func geminiInteractionVideoTask(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "text_to_video":
		return "text_to_video"
	case "image_to_video":
		return "image_to_video"
	case "reference_to_video":
		return "reference_to_video"
	case "edit":
		return "edit"
	default:
		return ""
	}
}

func geminiInteractionImageSize(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "512", "1K", "2K", "4K":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func geminiInteractionMIMEType(value string, responseType string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	switch responseType {
	case "text":
		switch normalized {
		case "application/json", "text/plain":
			return normalized
		}
	case "image":
		if normalized == "image/jpeg" {
			return normalized
		}
	}
	return ""
}

func buildGeminiInteractionTools(tools []portllm.ToolDefinition) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		items = append(items, map[string]any{
			"type":        "function",
			"name":        name,
			"description": strings.TrimSpace(tool.Description),
			"parameters":  decodeToolSchema(tool.InputSchema),
		})
	}
	return items
}

func geminiInteractionsProtectedProviderOptionKeys() []string {
	return []string{
		"input",
		"model",
		"response_format",
		"generation_config",
		"previous_interaction_id",
		"stream",
		"system_instruction",
		"tools",
	}
}

// consumeGeminiInteractionStream 按 SSE 事件边界消费 Interactions 流，并保留跨事件的工具步骤关联状态。
func consumeGeminiInteractionStream(
	reader io.Reader,
	result *portllm.GenerateOutput,
	onEvent func(portllm.GenerateStreamEvent) error,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxUpstreamBodyBytes)
	streamState := newGeminiInteractionStreamState()

	var dataLines []string
	flush := func() error {
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if data == "" || data == "[DONE]" {
			return nil
		}
		parsed := make(map[string]any)
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return nil
		}
		if err := parseStreamUpstreamError(parsed, data); err != nil {
			return err
		}
		return applyGeminiInteractionStreamEvent(parsed, result, streamState, onEvent)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(line[len("data:"):], " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

type geminiInteractionStreamState struct {
	toolCallIndexes      map[int64]int
	argumentDeltaStarted map[int64]bool
	serverToolCallIDs    map[int64]string
}

func newGeminiInteractionStreamState() *geminiInteractionStreamState {
	return &geminiInteractionStreamState{
		toolCallIndexes:      make(map[int64]int),
		argumentDeltaStarted: make(map[int64]bool),
		serverToolCallIDs:    make(map[int64]string),
	}
}

// applyGeminiInteractionStreamEvent 将单个官方 event_type 事件归并到统一生成结果并向会话层发送增量。
func applyGeminiInteractionStreamEvent(
	parsed map[string]any,
	result *portllm.GenerateOutput,
	streamState *geminiInteractionStreamState,
	onEvent func(portllm.GenerateStreamEvent) error,
) error {
	if result == nil {
		return nil
	}
	eventType := strings.TrimSpace(getString(parsed["event_type"]))
	if responseID := geminiInteractionStreamResponseID(parsed, eventType); responseID != "" {
		result.ResponseID = responseID
	}
	if serviceTier := geminiInteractionStreamServiceTier(parsed); serviceTier != "" {
		result.Usage.ServiceTier = serviceTier
	}
	if finalPayload := geminiInteractionStreamFinalPayload(parsed, eventType); len(finalPayload) > 0 {
		return mergeGeminiInteractionStreamFinal(result, finalPayload, onEvent)
	}
	if reasoning := geminiInteractionStreamReasoningDelta(parsed, eventType); reasoning != nil {
		mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Reasoning:  reasoning,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	if delta := geminiInteractionStreamText(parsed, eventType); delta != "" {
		result.Text += delta
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	if err := applyGeminiInteractionStreamMedia(parsed, eventType, result, onEvent); err != nil {
		return err
	}
	updateGeminiInteractionStreamToolCall(result, streamState, parsed, eventType)
	if call, ok := updateGeminiInteractionStreamServerToolCall(streamState, parsed, eventType); ok {
		appendUniqueToolCall(&result.ServerToolCalls, call)
		result.ServerSideToolUsage = geminiInteractionServerToolUsage(result.ServerToolCalls)
		result.Citations = geminiInteractionServerToolCitations(result.ServerToolCalls)
		if onEvent != nil {
			merged := geminiInteractionServerToolCall(result.ServerToolCalls, call.ToolCallID)
			if err := onEvent(portllm.GenerateStreamEvent{
				ServerToolCall: &merged,
				ResponseID:     result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	if usage := parseGeminiInteractionUsage(parsed); usage != (portllm.Usage{}) {
		if usage.ServiceTier == "" {
			usage.ServiceTier = result.Usage.ServiceTier
		}
		result.Usage = usage
		if onEvent != nil {
			return onEvent(portllm.GenerateStreamEvent{
				Usage:      usage,
				ResponseID: result.ResponseID,
			})
		}
	}
	return nil
}

func geminiInteractionStreamServiceTier(parsed map[string]any) string {
	if interaction := asMap(parsed["interaction"]); len(interaction) > 0 {
		return strings.TrimSpace(getString(interaction["service_tier"]))
	}
	return strings.TrimSpace(getString(parsed["service_tier"]))
}

func geminiInteractionStreamResponseID(parsed map[string]any, eventType string) string {
	if interaction := asMap(parsed["interaction"]); len(interaction) > 0 {
		return strings.TrimSpace(getString(interaction["id"]))
	}
	if strings.EqualFold(strings.TrimSpace(eventType), "interaction.status_update") {
		return strings.TrimSpace(getString(parsed["interaction_id"]))
	}
	return ""
}

func geminiInteractionStreamFinalPayload(parsed map[string]any, eventType string) map[string]any {
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	if eventType != "interaction.completed" {
		return nil
	}
	return parsed
}

// mergeGeminiInteractionStreamFinal 使用完成事件补齐文本、用量与工具轨迹，不重复发送已累计的文本增量。
func mergeGeminiInteractionStreamFinal(
	result *portllm.GenerateOutput,
	payload map[string]any,
	onEvent func(portllm.GenerateStreamEvent) error,
) error {
	finalOutput := parseGeminiInteractionPayload(payload)
	if finalOutput.ResponseID != "" {
		result.ResponseID = finalOutput.ResponseID
	}
	if finalOutput.Text != "" {
		delta := ""
		if result.Text == "" {
			delta = finalOutput.Text
		} else if strings.HasPrefix(finalOutput.Text, result.Text) && len(finalOutput.Text) > len(result.Text) {
			delta = finalOutput.Text[len(result.Text):]
		}
		result.Text = finalOutput.Text
		if delta != "" && onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	if finalOutput.Usage != (portllm.Usage{}) {
		usage := finalOutput.Usage
		if usage.ServiceTier == "" {
			usage.ServiceTier = result.Usage.ServiceTier
		}
		result.Usage = usage
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Usage:      usage,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	mergeReasoningOutput(&result.Reasoning, finalOutput.Reasoning)
	for _, call := range finalOutput.ToolCalls {
		appendUniqueToolCall(&result.ToolCalls, call)
	}
	result.ServerToolCalls = mergeGeminiInteractionFinalServerToolCalls(result.ServerToolCalls, finalOutput.ServerToolCalls)
	result.ServerSideToolUsage = geminiInteractionServerToolUsage(result.ServerToolCalls)
	result.Citations = appendUniqueStrings(result.Citations, finalOutput.Citations...)
	result.GeneratedImages = dedupeGeminiInteractionImages(append(result.GeneratedImages, finalOutput.GeneratedImages...))
	result.GeneratedVideos = dedupeGeminiInteractionVideos(append(result.GeneratedVideos, finalOutput.GeneratedVideos...))
	return nil
}

func applyGeminiInteractionStreamMedia(
	parsed map[string]any,
	eventType string,
	result *portllm.GenerateOutput,
	onEvent func(portllm.GenerateStreamEvent) error,
) error {
	if result == nil {
		return nil
	}
	images, videos := geminiInteractionStreamMedia(parsed, eventType)
	for _, image := range images {
		if geminiInteractionImageExists(result.GeneratedImages, image) {
			continue
		}
		image.RevisedPrompt = result.Text
		imageIndex := int64(len(result.GeneratedImages))
		result.GeneratedImages = append(result.GeneratedImages, image)
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				GeneratedImage:        &image,
				GeneratedImageIndex:   imageIndex,
				GeneratedImagePartial: true,
				ResponseID:            result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	result.GeneratedVideos = dedupeGeminiInteractionVideos(append(result.GeneratedVideos, videos...))
	return nil
}

func geminiInteractionStreamMedia(parsed map[string]any, eventType string) ([]portllm.GeneratedImage, []portllm.GeneratedVideo) {
	var value any
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "step.start":
		step := asMap(parsed["step"])
		if strings.ToLower(strings.TrimSpace(getString(step["type"]))) != "model_output" {
			return nil, nil
		}
		value = step["content"]
	case "step.delta":
		delta := asMap(parsed["delta"])
		switch strings.ToLower(strings.TrimSpace(getString(delta["type"]))) {
		case "image", "video":
			value = delta
		default:
			return nil, nil
		}
	default:
		return nil, nil
	}
	images := make([]portllm.GeneratedImage, 0)
	videos := make([]portllm.GeneratedVideo, 0)
	walkGeminiInteractionImages(value, &images)
	walkGeminiInteractionVideos(value, &videos)
	return dedupeGeminiInteractionImages(images), dedupeGeminiInteractionVideos(videos)
}

func geminiInteractionImageExists(images []portllm.GeneratedImage, candidate portllm.GeneratedImage) bool {
	key := strings.TrimSpace(candidate.URL)
	if key == "" {
		key = strings.TrimSpace(candidate.B64JSON)
	}
	if key == "" {
		return true
	}
	for _, image := range images {
		current := strings.TrimSpace(image.URL)
		if current == "" {
			current = strings.TrimSpace(image.B64JSON)
		}
		if current == key {
			return true
		}
	}
	return false
}

func geminiInteractionStreamText(parsed map[string]any, eventType string) string {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "step.start":
		step := asMap(parsed["step"])
		if strings.ToLower(strings.TrimSpace(getString(step["type"]))) != "model_output" {
			return ""
		}
		var text strings.Builder
		for _, rawContent := range asSlice(step["content"]) {
			content := asMap(rawContent)
			if strings.ToLower(strings.TrimSpace(getString(content["type"]))) == "text" {
				text.WriteString(getString(content["text"]))
			}
		}
		return text.String()
	case "step.delta":
		delta := asMap(parsed["delta"])
		if strings.ToLower(strings.TrimSpace(getString(delta["type"]))) == "text" {
			return getString(delta["text"])
		}
	}
	return ""
}

func geminiInteractionStreamReasoningDelta(parsed map[string]any, eventType string) *portllm.ReasoningDelta {
	result := &portllm.ReasoningDelta{
		EventType: eventType,
		ItemID:    fmt.Sprintf("%v", parsed["index"]),
		Status:    "streaming",
	}
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "step.start":
		step := asMap(parsed["step"])
		if strings.ToLower(strings.TrimSpace(getString(step["type"]))) != "thought" {
			return nil
		}
		result.Kind = "summary_text"
		result.Text = geminiInteractionSummaryText(step["summary"])
		result.Signature = strings.TrimSpace(getString(step["signature"]))
		if result.Text == "" && result.Signature == "" {
			return nil
		}
	case "step.delta":
		delta := asMap(parsed["delta"])
		switch strings.ToLower(strings.TrimSpace(getString(delta["type"]))) {
		case "thought_summary":
			content := asMap(delta["content"])
			if strings.ToLower(strings.TrimSpace(getString(content["type"]))) != "text" {
				return nil
			}
			result.Kind = "summary_text"
			result.Text = getString(content["text"])
			if result.Text == "" {
				return nil
			}
		case "thought_signature":
			result.Signature = strings.TrimSpace(getString(delta["signature"]))
			if result.Signature == "" {
				return nil
			}
		default:
			return nil
		}
	default:
		return nil
	}
	return result
}

func parseGeminiInteractionOutput(body []byte) (*portllm.GenerateOutput, error) {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	output := parseGeminiInteractionPayload(parsed)
	output.RawJSON = string(body)
	return output, nil
}

// parseGeminiInteractionPayload 将 Interactions 完整响应映射为项目统一的生成结果。
func parseGeminiInteractionPayload(parsed map[string]any) *portllm.GenerateOutput {
	payload := parsed
	if interaction := asMap(parsed["interaction"]); len(interaction) > 0 {
		payload = interaction
	}
	usage := parseGeminiInteractionUsage(parsed)
	output := &portllm.GenerateOutput{
		ResponseID:      strings.TrimSpace(getString(payload["id"])),
		Text:            geminiInteractionTextFromSteps(payload["steps"]),
		Reasoning:       parseGeminiInteractionReasoning(payload),
		Usage:           usage,
		ToolCalls:       parseGeminiInteractionFunctionCalls(payload),
		ServerToolCalls: parseGeminiInteractionServerToolCalls(payload),
		GeneratedImages: extractGeminiInteractionGeneratedImages(payload),
		GeneratedVideos: extractGeminiInteractionGeneratedVideos(payload),
	}
	output.ServerSideToolUsage = geminiInteractionServerToolUsage(output.ServerToolCalls)
	output.Citations = geminiInteractionServerToolCitations(output.ServerToolCalls)
	for i := range output.GeneratedImages {
		if output.GeneratedImages[i].RevisedPrompt == "" {
			output.GeneratedImages[i].RevisedPrompt = output.Text
		}
	}
	return output
}

// parseGeminiInteractionUsage 按 Interactions 官方 usage 字段拆分缓存输入、输出和思考 token。
// 官方文档示例满足 total_tokens = total_input_tokens + total_output_tokens + total_thought_tokens，
// 即 total_output_tokens 不含思考 token，与 generateContent 的 candidatesTokenCount/thoughtsTokenCount
// 语义一致；输出与思考分别记账、相加计费，不会重复计费。
func parseGeminiInteractionUsage(parsed map[string]any) portllm.Usage {
	payload := parsed
	if interaction := asMap(parsed["interaction"]); len(interaction) > 0 {
		payload = interaction
	}
	usage := asMap(payload["usage"])
	if len(usage) == 0 {
		usage = asMap(asMap(parsed["metadata"])["total_usage"])
	}
	if len(usage) == 0 {
		return portllm.Usage{}
	}
	totalInputTokens := toInt64(usage["total_input_tokens"])
	cacheReadTokens := toInt64(usage["total_cached_tokens"])
	return portllm.Usage{
		InputTokens:     nonCachedInputTokens(totalInputTokens, cacheReadTokens),
		OutputTokens:    toInt64(usage["total_output_tokens"]),
		CacheReadTokens: cacheReadTokens,
		ReasoningTokens: toInt64(usage["total_thought_tokens"]),
		ServiceTier:     strings.TrimSpace(getString(payload["service_tier"])),
		RawUsageJSON:    rawJSONFromValue(usage),
	}
}

func parseGeminiInteractionReasoning(parsed map[string]any) *portllm.ReasoningOutput {
	result := &portllm.ReasoningOutput{}
	summaryParts := make([]string, 0)
	for _, rawStep := range asSlice(parsed["steps"]) {
		step := asMap(rawStep)
		if strings.ToLower(strings.TrimSpace(getString(step["type"]))) != "thought" {
			continue
		}
		if summary := geminiInteractionSummaryText(step["summary"]); summary != "" {
			summaryParts = append(summaryParts, summary)
		}
		if signature := strings.TrimSpace(getString(step["signature"])); signature != "" {
			result.Signature = signature
		}
	}
	result.Summary = strings.Join(summaryParts, "\n\n")
	if result.Summary == "" && result.Signature == "" {
		return nil
	}
	return result
}

func geminiInteractionSummaryText(raw any) string {
	parts := make([]string, 0)
	for _, rawContent := range asSlice(raw) {
		content := asMap(rawContent)
		if strings.ToLower(strings.TrimSpace(getString(content["type"]))) != "text" {
			continue
		}
		if text := strings.TrimSpace(getString(content["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func rawJSONFromValue(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func parseGeminiInteractionFunctionCalls(parsed map[string]any) []portllm.ToolCall {
	calls := make([]portllm.ToolCall, 0)
	for _, rawStep := range asSlice(parsed["steps"]) {
		if call, ok := geminiInteractionToolCallFromMap(asMap(rawStep)); ok {
			calls = append(calls, call)
		}
	}
	return dedupeGeminiInteractionToolCalls(calls)
}

func parseGeminiInteractionServerToolCalls(parsed map[string]any) []portllm.ToolCall {
	calls := make([]portllm.ToolCall, 0)
	for _, rawStep := range asSlice(parsed["steps"]) {
		if call, ok := parseGeminiInteractionServerToolCall(asMap(rawStep), false); ok {
			appendUniqueToolCall(&calls, call)
		}
	}
	return calls
}

func updateGeminiInteractionStreamServerToolCall(
	state *geminiInteractionStreamState,
	parsed map[string]any,
	eventType string,
) (portllm.ToolCall, bool) {
	if state == nil {
		return portllm.ToolCall{}, false
	}
	index, ok := geminiInteractionStreamStepIndex(parsed)
	if !ok {
		return portllm.ToolCall{}, false
	}
	var payload map[string]any
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "step.start":
		payload = asMap(parsed["step"])
	case "step.delta":
		payload = asMap(parsed["delta"])
	default:
		return portllm.ToolCall{}, false
	}
	call, ok := parseGeminiInteractionServerToolCall(payload, true)
	if !ok {
		return portllm.ToolCall{}, false
	}
	if call.ToolCallID == "" {
		call.ToolCallID = state.serverToolCallIDs[index]
	}
	if call.ToolCallID == "" {
		_, isResult := geminiInteractionServerToolName(getString(payload["type"]))
		if isResult {
			return portllm.ToolCall{}, false
		}
		call.ToolCallID = geminiInteractionStreamToolCallID(call.ToolName, index)
	}
	if state.serverToolCallIDs == nil {
		state.serverToolCallIDs = make(map[int64]string)
	}
	state.serverToolCallIDs[index] = call.ToolCallID
	return call, true
}

func parseGeminiInteractionServerToolCall(item map[string]any, streaming bool) (portllm.ToolCall, bool) {
	itemType := strings.ToLower(strings.TrimSpace(getString(item["type"])))
	toolName, isResult := geminiInteractionServerToolName(itemType)
	if toolName == "" {
		return portllm.ToolCall{}, false
	}
	callID := strings.TrimSpace(getString(item["id"]))
	if isResult {
		callID = strings.TrimSpace(getString(item["call_id"]))
	}
	status := "in_progress"
	outputJSON := ""
	errorJSON := ""
	if isResult {
		_, hasResult := item["result"]
		if !streaming || hasResult {
			status = "completed"
		}
		outputJSON = rawJSONFromValue(item["result"])
		if isError, _ := item["is_error"].(bool); isError {
			status = "error"
			errorJSON = outputJSON
		}
	}
	return portllm.ToolCall{
		ToolCallID:       callID,
		ToolType:         toolName,
		ToolName:         toolName,
		ArgumentsJSON:    rawJSONFromValue(item["arguments"]),
		ThoughtSignature: strings.TrimSpace(getString(item["signature"])),
		Status:           status,
		OutputJSON:       outputJSON,
		ErrorJSON:        errorJSON,
	}, true
}

func geminiInteractionStreamToolCallID(name string, index int64) string {
	return fmt.Sprintf("gemini_interaction_%s_%d", name, index)
}

// mergeGeminiInteractionFinalServerToolCalls uses the completed interaction to fill streamed
// native-tool traces while preserving any stable ID already emitted to conversation consumers.
func mergeGeminiInteractionFinalServerToolCalls(current []portllm.ToolCall, final []portllm.ToolCall) []portllm.ToolCall {
	if len(current) == 0 {
		return final
	}
	merged := append([]portllm.ToolCall(nil), current...)
	matched := make([]bool, len(merged))
	for _, incoming := range final {
		matchIndex := -1
		if incoming.ToolCallID != "" {
			for index, existing := range merged {
				if !matched[index] && existing.ToolCallID == incoming.ToolCallID {
					matchIndex = index
					break
				}
			}
		}
		if matchIndex < 0 {
			for index, existing := range merged {
				if !matched[index] && existing.ToolName == incoming.ToolName {
					matchIndex = index
					break
				}
			}
		}
		if matchIndex < 0 {
			merged = append(merged, incoming)
			matched = append(matched, true)
			continue
		}
		stableID := merged[matchIndex].ToolCallID
		merged[matchIndex] = mergeToolCall(merged[matchIndex], incoming)
		if stableID != "" {
			merged[matchIndex].ToolCallID = stableID
		}
		matched[matchIndex] = true
	}
	return merged
}

func geminiInteractionServerToolName(itemType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "google_search_call":
		return "google_search", false
	case "google_search_result":
		return "google_search", true
	case "code_execution_call":
		return "code_execution", false
	case "code_execution_result":
		return "code_execution", true
	case "url_context_call":
		return "url_context", false
	case "url_context_result":
		return "url_context", true
	default:
		return "", false
	}
}

func geminiInteractionServerToolCall(calls []portllm.ToolCall, callID string) portllm.ToolCall {
	for _, call := range calls {
		if strings.TrimSpace(call.ToolCallID) == strings.TrimSpace(callID) {
			return call
		}
	}
	return portllm.ToolCall{ToolCallID: strings.TrimSpace(callID)}
}

func geminiInteractionServerToolUsage(calls []portllm.ToolCall) map[string]int64 {
	if len(calls) == 0 {
		return nil
	}
	usage := make(map[string]int64)
	for _, call := range calls {
		name := strings.TrimSpace(call.ToolName)
		if name == "" {
			name = strings.TrimSpace(call.ToolType)
		}
		if name != "" {
			usage[name]++
		}
	}
	if len(usage) == 0 {
		return nil
	}
	return usage
}

func geminiInteractionServerToolCitations(calls []portllm.ToolCall) []string {
	citations := make([]string, 0)
	for _, call := range calls {
		if call.ToolName != "google_search" && call.ToolName != "url_context" {
			continue
		}
		var output any
		if err := json.Unmarshal([]byte(call.OutputJSON), &output); err != nil {
			continue
		}
		walkGeminiInteractionCitationURLs(output, &citations)
	}
	return appendUniqueStrings(nil, citations...)
}

func walkGeminiInteractionCitationURLs(value any, citations *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "url" || key == "uri" {
				if citation := strings.TrimSpace(getString(child)); citation != "" {
					*citations = append(*citations, citation)
				}
			}
			walkGeminiInteractionCitationURLs(child, citations)
		}
	case []any:
		for _, child := range typed {
			walkGeminiInteractionCitationURLs(child, citations)
		}
	}
}

func updateGeminiInteractionStreamToolCall(
	result *portllm.GenerateOutput,
	state *geminiInteractionStreamState,
	parsed map[string]any,
	eventType string,
) {
	if result == nil || state == nil {
		return
	}
	index, ok := geminiInteractionStreamStepIndex(parsed)
	if !ok {
		return
	}
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "step.start":
		step := asMap(parsed["step"])
		if strings.ToLower(strings.TrimSpace(getString(step["type"]))) != "function_call" {
			return
		}
		name := strings.TrimSpace(getString(step["name"]))
		if name == "" {
			return
		}
		if state.toolCallIndexes == nil {
			state.toolCallIndexes = make(map[int64]int)
		}
		arguments := normalizeJSONString(step["arguments"])
		result.ToolCalls = append(result.ToolCalls, portllm.ToolCall{
			ToolCallID:    strings.TrimSpace(getString(step["id"])),
			ToolType:      "function",
			ToolName:      name,
			ArgumentsJSON: arguments,
			Status:        "requested",
		})
		state.toolCallIndexes[index] = len(result.ToolCalls) - 1
	case "step.delta":
		delta := asMap(parsed["delta"])
		if strings.ToLower(strings.TrimSpace(getString(delta["type"]))) != "arguments_delta" {
			return
		}
		partial := getString(delta["arguments"])
		if partial == "" {
			return
		}
		callIndex, ok := geminiInteractionStreamToolCallIndex(result, state, index)
		if !ok {
			return
		}
		if !state.argumentDeltaStarted[index] {
			if state.argumentDeltaStarted == nil {
				state.argumentDeltaStarted = make(map[int64]bool)
			}
			result.ToolCalls[callIndex].ArgumentsJSON = ""
			state.argumentDeltaStarted[index] = true
		}
		result.ToolCalls[callIndex].ArgumentsJSON += partial
	case "step.stop":
		callIndex, ok := geminiInteractionStreamToolCallIndex(result, state, index)
		if !ok || strings.TrimSpace(result.ToolCalls[callIndex].ArgumentsJSON) != "" {
			return
		}
		result.ToolCalls[callIndex].ArgumentsJSON = "{}"
	}
}

func geminiInteractionStreamStepIndex(parsed map[string]any) (int64, bool) {
	rawIndex, ok := parsed["index"]
	if !ok {
		return 0, false
	}
	index := toInt64(rawIndex)
	return index, index >= 0
}

func geminiInteractionStreamToolCallIndex(result *portllm.GenerateOutput, state *geminiInteractionStreamState, index int64) (int, bool) {
	if result == nil || state == nil {
		return 0, false
	}
	callIndex, ok := state.toolCallIndexes[index]
	if !ok || callIndex >= len(result.ToolCalls) {
		return 0, false
	}
	return callIndex, true
}

func geminiInteractionToolCallFromMap(item map[string]any) (portllm.ToolCall, bool) {
	if strings.TrimSpace(strings.ToLower(getString(item["type"]))) != "function_call" {
		return portllm.ToolCall{}, false
	}
	name := strings.TrimSpace(getString(item["name"]))
	if name == "" {
		return portllm.ToolCall{}, false
	}
	arguments := normalizeJSONString(item["arguments"])
	if arguments == "" {
		arguments = "{}"
	}
	return portllm.ToolCall{
		ToolCallID:    strings.TrimSpace(getString(item["id"])),
		ToolType:      "function",
		ToolName:      name,
		ArgumentsJSON: arguments,
		Status:        "requested",
	}, true
}

func dedupeGeminiInteractionToolCalls(calls []portllm.ToolCall) []portllm.ToolCall {
	if len(calls) <= 1 {
		return calls
	}
	result := make([]portllm.ToolCall, 0, len(calls))
	seen := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		key := strings.TrimSpace(call.ToolCallID)
		if key == "" {
			key = call.ToolName + "\x00" + call.ArgumentsJSON
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, call)
	}
	return result
}

func extractGeminiInteractionGeneratedImages(parsed map[string]any) []portllm.GeneratedImage {
	images := make([]portllm.GeneratedImage, 0)
	walkGeminiInteractionModelOutputContent(parsed["steps"], func(content any) {
		walkGeminiInteractionImages(content, &images)
	})
	return dedupeGeminiInteractionImages(images)
}

func dedupeGeminiInteractionImages(images []portllm.GeneratedImage) []portllm.GeneratedImage {
	if len(images) <= 1 {
		return images
	}
	deduped := make([]portllm.GeneratedImage, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
		key := strings.TrimSpace(image.URL)
		if key == "" {
			key = strings.TrimSpace(image.B64JSON)
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, image)
	}
	return deduped
}

func walkGeminiInteractionModelOutputContent(raw any, walk func(any)) {
	if walk == nil {
		return
	}
	for _, rawStep := range asSlice(raw) {
		step := asMap(rawStep)
		if strings.TrimSpace(strings.ToLower(getString(step["type"]))) != "model_output" {
			continue
		}
		walk(step["content"])
	}
}

func walkGeminiInteractionImages(value any, images *[]portllm.GeneratedImage) {
	switch typed := value.(type) {
	case map[string]any:
		if image, ok := geminiImageFromInteractionMap(typed); ok {
			*images = append(*images, image)
		}
	case []any:
		for _, child := range typed {
			if image, ok := geminiImageFromInteractionMap(asMap(child)); ok {
				*images = append(*images, image)
			}
		}
	}
}

func geminiImageFromInteractionMap(item map[string]any) (portllm.GeneratedImage, bool) {
	if strings.TrimSpace(strings.ToLower(getString(item["type"]))) != "image" {
		return portllm.GeneratedImage{}, false
	}
	mimeType := strings.TrimSpace(getString(item["mime_type"]))
	if mimeType != "" && !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return portllm.GeneratedImage{}, false
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	url := strings.TrimSpace(getString(item["uri"]))
	b64 := strings.TrimSpace(getString(item["data"]))
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") || (url == "" && b64 == "") {
		return portllm.GeneratedImage{}, false
	}
	return portllm.GeneratedImage{
		URL:      url,
		B64JSON:  b64,
		MIMEType: mimeType,
	}, true
}

func extractGeminiInteractionGeneratedVideos(parsed map[string]any) []portllm.GeneratedVideo {
	videos := make([]portllm.GeneratedVideo, 0)
	walkGeminiInteractionModelOutputContent(parsed["steps"], func(content any) {
		walkGeminiInteractionVideos(content, &videos)
	})
	return dedupeGeminiInteractionVideos(videos)
}

func dedupeGeminiInteractionVideos(videos []portllm.GeneratedVideo) []portllm.GeneratedVideo {
	if len(videos) <= 1 {
		return videos
	}
	deduped := make([]portllm.GeneratedVideo, 0, len(videos))
	seen := make(map[string]struct{}, len(videos))
	for _, video := range videos {
		key := strings.TrimSpace(video.URL)
		if key == "" {
			key = strings.TrimSpace(video.B64JSON)
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, video)
	}
	return deduped
}

func walkGeminiInteractionVideos(value any, videos *[]portllm.GeneratedVideo) {
	switch typed := value.(type) {
	case map[string]any:
		if video, ok := geminiVideoFromMap(typed); ok {
			*videos = append(*videos, video)
		}
	case []any:
		for _, child := range typed {
			if video, ok := geminiVideoFromMap(asMap(child)); ok {
				*videos = append(*videos, video)
			}
		}
	}
}

func geminiVideoFromMap(item map[string]any) (portllm.GeneratedVideo, bool) {
	if strings.TrimSpace(strings.ToLower(getString(item["type"]))) != "video" {
		return portllm.GeneratedVideo{}, false
	}
	mimeType := strings.TrimSpace(getString(item["mime_type"]))
	if mimeType != "" && !strings.HasPrefix(strings.ToLower(mimeType), "video/") {
		return portllm.GeneratedVideo{}, false
	}
	url := strings.TrimSpace(getString(item["uri"]))
	b64 := strings.TrimSpace(getString(item["data"]))
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	if !strings.HasPrefix(strings.ToLower(mimeType), "video/") || (url == "" && b64 == "") {
		return portllm.GeneratedVideo{}, false
	}
	return portllm.GeneratedVideo{
		URL:      url,
		B64JSON:  b64,
		MIMEType: mimeType,
	}, true
}

func geminiInteractionTextFromSteps(raw any) string {
	parts := make([]string, 0)
	for _, rawStep := range asSlice(raw) {
		step := asMap(rawStep)
		if strings.TrimSpace(strings.ToLower(getString(step["type"]))) != "model_output" {
			continue
		}
		for _, rawContent := range asSlice(step["content"]) {
			content := asMap(rawContent)
			if strings.TrimSpace(strings.ToLower(getString(content["type"]))) != "text" {
				continue
			}
			if text := strings.TrimSpace(getString(content["text"])); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}
