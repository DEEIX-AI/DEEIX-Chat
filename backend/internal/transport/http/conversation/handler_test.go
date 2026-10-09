package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestStreamErrorPayloadWithResultPreservesPersistedResult(t *testing.T) {
	result := &appconversation.SendMessageResult{}
	payload := streamErrorPayloadWithResult(nil, errors.New("store generated video"), result)
	if payload["type"] != "error" {
		t.Fatalf("payload type = %#v, want error", payload["type"])
	}
	if _, ok := payload["data"]; !ok {
		t.Fatalf("stream error payload lost persisted result: %#v", payload)
	}
	if _, ok := streamErrorPayloadWithResult(nil, errors.New("no result"), nil)["data"]; ok {
		t.Fatal("stream error payload must not carry data without a result")
	}
}

func TestBillingStreamErrorPayloadCarriesResultWhenPresent(t *testing.T) {
	withResult := streamErrorPayloadWithResult(nil, appbilling.ErrUsageBalanceInsufficient, &appconversation.SendMessageResult{})
	if withResult["type"] != "error" || withResult["errorCode"] != "billing.insufficient_funds" {
		t.Fatalf("billing error payload = %#v", withResult)
	}
	if _, ok := withResult["data"]; !ok {
		t.Fatalf("billing error payload lost persisted result: %#v", withResult)
	}
	if _, ok := streamErrorPayloadWithResult(nil, appbilling.ErrUsageBalanceInsufficient, nil)["data"]; ok {
		t.Fatal("billing error payload must not carry data without a result")
	}
}

func TestSearchConversationsRejectsLongQueryWithStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/conversations/search?q="+strings.Repeat("a", maxConversationSearchQueryRunes+1),
		nil,
	)
	c.Set(middleware.ContextKeyUserID, uint(1))

	(&Handler{}).SearchConversations(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var payload response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ErrorCode != response.CodeRequestInvalidQuery {
		t.Fatalf("errorCode = %q, want %q", payload.ErrorCode, response.CodeRequestInvalidQuery)
	}
}

func TestStreamErrorPayloadIncludesUpstreamDebug(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 401,
		Message:    "google authentication failed",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method:  "POST",
				Path:    "/v1beta/models/nano-banana-pro:streamGenerateContent",
				Headers: map[string]string{"x-goog-api-key": "[redacted]"},
				Body:    `{"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 401,
				Headers:    map[string]string{"Provider": "ExampleEdge"},
				Body:       `{"error":{"message":"unauthorized"}}`,
			},
		},
	})

	payload := streamErrorPayload(nil, err)
	debug, ok := payload["debug"].(*llm.UpstreamDebugSnapshot)
	if !ok || debug == nil {
		t.Fatalf("expected upstream debug payload, got %#v", payload["debug"])
	}
	if debug.Request.Path != "/v1beta/models/nano-banana-pro:streamGenerateContent" {
		t.Fatalf("unexpected request debug: %#v", debug.Request)
	}
	if debug.Response.StatusCode != 401 {
		t.Fatalf("unexpected response debug: %#v", debug.Response)
	}
	if debug.Request.Headers != nil || debug.Response.Headers != nil {
		t.Fatalf("expected public error stream to omit upstream headers, got request=%#v response=%#v", debug.Request.Headers, debug.Response.Headers)
	}
}

func TestDescribeSendMessageErrorDoesNotExposeUpstreamUnauthorizedAsPlatformUnauthorized(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 401,
		Message:    "upstream authentication failed",
	})

	mapped := describeSendMessageError(err)
	if mapped.Status != 502 {
		t.Fatalf("expected upstream 401 to be mapped to gateway failure, got status=%d", mapped.Status)
	}
	if mapped.Code == "auth.unauthorized" || mapped.Code == "auth.invalid_token" || mapped.Code == "auth.session_invalid" {
		t.Fatalf("expected upstream 401 to avoid platform auth codes, got %#v", mapped)
	}
}

func TestDescribeSendMessageErrorPreservesUpstreamRateLimit(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{StatusCode: http.StatusTooManyRequests})

	mapped := describeSendMessageError(err)
	if mapped.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", mapped.Status, http.StatusTooManyRequests)
	}
	if mapped.Code != appconversation.MessageErrorCodeUpstreamRateLimited {
		t.Fatalf("code = %q, want %q", mapped.Code, appconversation.MessageErrorCodeUpstreamRateLimited)
	}
	payload := streamErrorPayload(nil, err)
	if payload["status"] != http.StatusTooManyRequests {
		t.Fatalf("payload status = %#v, want %d", payload["status"], http.StatusTooManyRequests)
	}
}

func TestDescribeSendMessageErrorPreservesEmptyResponseAfterUpstreamWrapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "wrapped", err: fmt.Errorf("generation: %w", appconversation.ErrUpstreamEmptyResponse)},
		{name: "joined", err: errors.Join(appconversation.ErrUpstreamRequestFailed, appconversation.ErrUpstreamEmptyResponse)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := describeSendMessageError(tt.err)
			want := response.Description{
				Status:  http.StatusBadGateway,
				Code:    appconversation.MessageErrorCodeUpstreamEmptyResponse,
				Message: "model returned empty response",
			}
			if mapped != want {
				t.Fatalf("describeSendMessageError() = %#v, want %#v", mapped, want)
			}
		})
	}
}

func TestDescribeSendMessageErrorClassifiesGeneratedMediaArtifactFailure(t *testing.T) {
	mapped := describeSendMessageError(appconversation.ErrGeneratedMediaArtifactUnavailable)
	if mapped.Status != http.StatusBadGateway {
		t.Fatalf("expected artifact failure to be mapped to gateway failure, got status=%d", mapped.Status)
	}
	if mapped.Code != appconversation.MessageErrorCodeMediaArtifactUnavailable {
		t.Fatalf("unexpected artifact error code: %#v", mapped)
	}
	if mapped.Message != appconversation.ErrGeneratedMediaArtifactUnavailable.Error() {
		t.Fatalf("unexpected public artifact message: %#v", mapped)
	}
}

// TestDescribeSendMessageErrorContract 固定消息发送 / 生成 / 计费路径上每个哨兵的 HTTP 状态、错误码与对外文案。
// 错误码是前端本地化依赖的 API 契约（apps/web/i18n/messages/*/errors.json），改动任何一项都必须同步前端。
func TestDescribeSendMessageErrorContract(t *testing.T) {
	cases := []struct {
		err     error
		status  int
		code    string
		message string
	}{
		{appconversation.ErrConversationNotFound, http.StatusNotFound, "conversation.not_found", "conversation not found"},
		{appconversation.ErrInvalidFileReference, http.StatusBadRequest, "file.invalid_reference", "invalid file reference"},
		{appconversation.ErrFileNotFound, http.StatusNotFound, "file.not_found", "file not found"},
		{appconversation.ErrFileTooLarge, http.StatusRequestEntityTooLarge, "file.too_large", "file too large"},
		{appconversation.ErrInvalidMessageBranch, http.StatusBadRequest, "message.invalid_branch", "invalid message branch"},
		{appconversation.ErrTooManyMessageFiles, http.StatusBadRequest, "message.too_many_files", "too many files in one message"},
		{appconversation.ErrTooManySelectedTools, http.StatusBadRequest, "message.too_many_selected_tools", "too many selected tools"},
		{appconversation.ErrMultipleImageAttachmentProcessors, http.StatusBadRequest, "message.multiple_image_processors", "select only one image attachment processor"},
		{appconversation.ErrImageAttachmentProcessingFailed, http.StatusBadGateway, "mcp.image_processing_failed", "image processing tool failed"},
		{appconversation.ErrTooManySelectedSkills, http.StatusBadRequest, "message.too_many_selected_skills", "too many selected skills"},
		{appconversation.ErrSkillNotFound, http.StatusNotFound, "skill.not_found", "skill not found"},
		{appconversation.ErrInvalidSkillUse, http.StatusBadRequest, "request.invalid_skill_use", "invalid skill use"},
		{appconversation.ErrFileProcessingNotReady, http.StatusBadRequest, "file.not_ready", "file processing is not ready"},
		{appconversation.ErrFileTooLargeForFullContext, http.StatusBadRequest, "file.too_large_for_context", "file is too large for full context"},
		{appconversation.ErrEmbeddingUnavailable, http.StatusBadRequest, "file.embedding_unavailable", "embedding is unavailable for current file capability"},
		{appconversation.ErrInvalidKnowledgeBaseReference, http.StatusBadRequest, "knowledge_base.invalid_reference", "invalid knowledge base reference"},
		{appconversation.ErrKnowledgeBaseUnavailable, http.StatusServiceUnavailable, "knowledge_base.unavailable", "knowledge base retrieval is unavailable"},
		{appconversation.ErrKnowledgeBaseNotReady, http.StatusConflict, "knowledge_base.not_ready", "selected knowledge base has no ready files"},
		{appconversation.ErrModelRouteNotConfigured, http.StatusServiceUnavailable, "llm.model_route_not_configured", "model route is not configured"},
		{appconversation.ErrModelAccessDenied, http.StatusForbidden, "llm.model_access_denied", "you do not have access to this model"},
		{appconversation.ErrStorageQuotaExceeded, http.StatusConflict, "quota.exceeded", "quota exceeded"},
		{appconversation.ErrGeneratedMediaArtifactUnavailable, http.StatusBadGateway, "media.artifact_unavailable", "generated media artifact is temporarily unavailable"},
		{appconversation.ErrUpstreamEmptyResponse, http.StatusBadGateway, "llm.empty_response", "model returned empty response"},
		{appconversation.ErrToolRunFinalAnswerMissing, http.StatusBadGateway, "tool_run.final_answer_missing", "tool run ended without a final answer"},
		{appconversation.ErrMessageGenerationCanceled, http.StatusBadRequest, "conversation_run.canceled", "message generation canceled"},
		{appconversation.ErrMessageGenerationInterrupted, http.StatusServiceUnavailable, "conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"},
		{appconversation.ErrMediaImagePromptRequired, http.StatusBadRequest, "media.image_prompt_required", "image prompt is required"},
		{appconversation.ErrMediaImageGenerationRejectsInputs, http.StatusBadRequest, "media.image_generation_rejects_inputs", "image generation does not accept input images"},
		{appconversation.ErrMediaImageEditInputRequired, http.StatusBadRequest, "media.image_edit_input_required", "image edit requires at least one input image"},
		{appconversation.ErrMediaImageEditTooManyInputs, http.StatusBadRequest, "media.image_edit_too_many_inputs", "too many image edit input images"},
		{appconversation.ErrMediaImageEditInputInvalid, http.StatusBadRequest, "media.image_edit_input_invalid", "image edit input image is invalid"},
		{appconversation.ErrMediaVideoPromptRequired, http.StatusBadRequest, "media.video_prompt_required", "video prompt is required"},
		{appconversation.ErrMediaVideoInputInvalid, http.StatusBadRequest, "media.video_input_invalid", "video generation input is invalid"},
		{appconversation.ErrMediaVideoTooManyInputs, http.StatusBadRequest, "media.video_too_many_inputs", "too many video generation input images"},
		{appconversation.ErrMediaRouteProtocolMismatch, http.StatusServiceUnavailable, "media.route_protocol_mismatch", "media route protocol does not match task"},
		{appconversation.ErrInvalidMediaGenerationTask, http.StatusBadRequest, "media.invalid_task", "invalid media generation task"},
		{appconversation.ErrDuplicateMessageGenerationRun, http.StatusConflict, "message_generation_run.already_exists", "message generation run already exists"},
		{appbilling.ErrUsageConcurrencyLimitExceeded, http.StatusTooManyRequests, "billing.concurrency_limit_exceeded", "too many concurrent paid requests"},
		{appbilling.ErrUsageReservationConflict, http.StatusConflict, "billing.reservation_conflict", "usage request already exists"},
		{appbilling.ErrUsageBalanceInsufficient, http.StatusPaymentRequired, "billing.insufficient_funds", "insufficient balance"},
		{appbilling.ErrModelPricingRequired, http.StatusPaymentRequired, "billing.pricing_required", "model pricing is required"},
		{errors.New("pq: duplicate key value already exists"), http.StatusInternalServerError, "internal.error", "internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			want := response.Description{Status: tc.status, Code: tc.code, Message: tc.message}
			if got := describeSendMessageError(tc.err); got != want {
				t.Fatalf("describeSendMessageError(sentinel) = %#v, want %#v", got, want)
			}
			// 应用层通常带上下文包装哨兵；对外契约不能随包装文本变化。
			wrapped := fmt.Errorf("send message for conversation 7: %w", tc.err)
			if got := describeSendMessageError(wrapped); got != want {
				t.Fatalf("describeSendMessageError(wrapped) = %#v, want %#v", got, want)
			}
		})
	}
}

func TestForkErrorsExposeStableContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err     error
		code    string
		message string
	}{
		{appconversation.ErrMessageForkStateInvalid, "conversation.message_fork_state_invalid", "message is still generating"},
		{appconversation.ErrMessageForkTargetInvalid, "conversation.message_fork_target_invalid", "only assistant messages can be forked"},
		{appconversation.ErrMessageForkHistoryIncomplete, "conversation.message_fork_history_incomplete", "message history is too deep or incomplete"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			t.Parallel()
			got := response.Describe(http.StatusBadRequest, test.err)
			want := response.Description{Status: http.StatusBadRequest, Code: test.code, Message: test.message}
			if got != want {
				t.Fatalf("response.Describe() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestDescribeSendMessageErrorDoesNotInferCodesFromInternalText(t *testing.T) {
	// 未归类的内部错误文案可能与文案推断规则撞车（如 "... not found"），必须一律作为内部错误返回。
	for _, err := range []error{
		errors.New("upstream credential not found"),
		errors.New("redis: connection already exists"),
		fmt.Errorf("store generated video: %w", errors.New("payment gateway timeout")),
	} {
		got := describeSendMessageError(err)
		if got.Status != http.StatusInternalServerError || got.Code != response.CodeInternal || got.Message != "internal server error" {
			t.Fatalf("describeSendMessageError(%v) = %#v, want generic internal error", err, got)
		}
	}
}

func TestHandleSendMessageErrorWritesDescribedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/conversations/1/messages", nil)

	handleSendMessageError(c, fmt.Errorf("retrieve: %w", appconversation.ErrKnowledgeBaseUnavailable))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	var payload response.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ErrorCode != appconversation.MessageErrorCodeKnowledgeBaseUnavailable || payload.ErrorMsg != "knowledge base retrieval is unavailable" {
		t.Fatalf("envelope = %#v", payload)
	}
}

func TestStreamAndHTTPErrorContractsAgree(t *testing.T) {
	// NDJSON 终态事件与 HTTP 响应必须给出同一份 errorCode / message。
	for _, err := range []error{
		appconversation.ErrKnowledgeBaseNotReady,
		appconversation.ErrMessageGenerationInterrupted,
		appbilling.ErrUsageConcurrencyLimitExceeded,
		errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{StatusCode: http.StatusTooManyRequests}),
	} {
		described := describeSendMessageError(err)
		payload := streamErrorPayload(nil, err)
		if payload["errorCode"] != described.Code || payload["message"] != described.Message || payload["status"] != described.Status {
			t.Fatalf("stream payload %#v disagrees with description %#v", payload, described)
		}
	}
}

func TestStreamErrorPayloadClassifiesImageStreamConfigurationFailure(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 500,
		Message:    "invalid character 'e' looking for beginning of value",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/images/generations",
				Body:   `{"model":"gpt-image-2","prompt":"a cat","stream":true}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 500,
				Body:       `{"error":{"message":"invalid character 'e' looking for beginning of value"}}`,
			},
		},
	})

	payload := streamErrorPayload(nil, err)
	if got := payload["errorCode"]; got != appconversation.MessageErrorCodeMediaImageStreamUnsupported {
		t.Fatalf("errorCode = %#v, want %q", got, appconversation.MessageErrorCodeMediaImageStreamUnsupported)
	}
}

func TestStreamErrorPayloadRecordsCauseForAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/conversations/1/messages/stream", nil)
	c.Set(middleware.ContextKeyRequestID, "req-stream-1")

	cause := fmt.Errorf("persist assistant message: %w", errors.New("database is locked"))
	payload := streamErrorPayload(c, cause)

	if payload["requestId"] != "req-stream-1" {
		t.Fatalf("expected requestId in stream error event, got %#v", payload["requestId"])
	}
	if strings.Contains(fmt.Sprint(payload), "database is locked") {
		t.Fatalf("raw cause must not reach the client: %#v", payload)
	}
	if got := response.WrittenErrorCode(c); got != payload["errorCode"] {
		t.Fatalf("expected recorded error code %v, got %q", payload["errorCode"], got)
	}
	if got := response.RecordedErrorCause(c); !errors.Is(got, cause) {
		t.Fatalf("expected raw cause recorded for access log, got %v", got)
	}
}

func TestStreamErrorPayloadDoesNotRecordUserCancelAsFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/conversations/1/messages/stream", nil)
	c.Set(middleware.ContextKeyRequestID, "req-stop-1")

	payload := streamErrorPayload(c, fmt.Errorf("media stream: %w", appconversation.ErrMessageGenerationCanceled))

	if payload["errorCode"] == "" || payload["requestId"] != "req-stop-1" {
		t.Fatalf("client still needs the terminal event: %#v", payload)
	}
	if code, cause := response.WrittenErrorCode(c), response.RecordedErrorCause(c); code != "" || cause != nil {
		t.Fatalf("a user stop must not be recorded as a failure, got code=%q cause=%v", code, cause)
	}
}

func TestStreamResumeInterruptedPayloadReferencesOriginalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/conversation-runs/run_1/stream", nil)
	c.Set(middleware.ContextKeyRequestID, "req-resume-2")

	payload := streamResumeInterruptedPayload(c, "run_1", "req-origin-1")
	if payload["requestId"] != "req-origin-1" {
		t.Fatalf("resume error must reference the original generation request, got %#v", payload["requestId"])
	}
	cause := response.RecordedErrorCause(c)
	if !errors.Is(cause, appconversation.ErrMessageGenerationInterrupted) ||
		!strings.Contains(cause.Error(), "run_1") || !strings.Contains(cause.Error(), "req-origin-1") {
		t.Fatalf("resume access log must link run and original request, got %v", cause)
	}

	// 查不到原始请求时退回续传请求自身的 ID，不能输出空 ID。
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/conversation-runs/run_1/stream", nil)
	c2.Set(middleware.ContextKeyRequestID, "req-resume-3")
	if got := streamResumeInterruptedPayload(c2, "run_1", "")["requestId"]; got != "req-resume-3" {
		t.Fatalf("expected fallback to the resume request id, got %#v", got)
	}
}
