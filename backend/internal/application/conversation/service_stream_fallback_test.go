package conversation

import (
	"errors"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestShouldNotFallbackToNonStreamingForUpstreamParamErrors(t *testing.T) {
	err := &llm.UpstreamError{StatusCode: 400, Message: "Param Incorrect"}
	if shouldFallbackToNonStreaming(err) {
		t.Fatalf("expected upstream param errors to return directly")
	}

	wrapped := errors.Join(ErrUpstreamRequestFailed, &llm.UpstreamError{StatusCode: 422, Message: "invalid stream"})
	if shouldFallbackToNonStreaming(wrapped) {
		t.Fatalf("expected upstream validation errors to return directly")
	}
}

func TestShouldFallbackToNonStreamingForExplicitStreamUnsupportedErrors(t *testing.T) {
	err := &llm.UpstreamError{StatusCode: 400, Message: "stream is not supported by this model"}
	if !shouldFallbackToNonStreaming(err) {
		t.Fatalf("expected explicit stream unsupported errors to fallback to non-streaming")
	}

	statusErr := &llm.UpstreamError{StatusCode: 405, Message: "method not allowed"}
	if !shouldFallbackToNonStreaming(statusErr) {
		t.Fatalf("expected stream transport status errors to fallback to non-streaming")
	}
}

func TestMessageErrorSummaryIncludesUpstreamBody(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 400,
		Message:    "Param Incorrect",
		Body:       `{"error":{"message":"Param Incorrect","param":"tools[0].type"}}`,
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/responses",
				Body:   `{"model":"grok-4"}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 400,
				Body:       `{"error":{"message":"Param Incorrect","param":"tools[0].type"}}`,
			},
		},
	})
	summary := MessageErrorSummary(err)
	if summary != "模型请求失败（HTTP 400）\n错误：Param Incorrect" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if debug := MessageErrorDebug(err); debug == nil || debug.Request.Path != "/v1/responses" {
		t.Fatalf("expected upstream debug snapshot, got %#v", debug)
	}
}

func TestMessageErrorSummaryHidesRawSSEForSuccessfulHTTPStatus(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 200,
		Message:    `HTTP 200, data: {"id":"resp_1","object":"chat.completion.chunk","choices":[{"delta":{"content":"hello"}}]}`,
		Body:       "data: {\"id\":\"resp_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/responses",
				Body:   `{"model":"gpt-5.5"}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 200,
				Body:       "data: {\"id\":\"resp_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n",
			},
		},
	})

	summary := MessageErrorSummary(err)
	if strings.Contains(summary, "data:") || strings.Contains(summary, "chat.completion.chunk") {
		t.Fatalf("summary leaked raw SSE body: %q", summary)
	}
	if summary != "模型响应格式不兼容（HTTP 200）\n错误：上游返回成功状态码，但响应格式与当前协议不兼容" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if debug := MessageErrorDebug(err); debug == nil || !strings.Contains(debug.Response.Body, "data:") {
		t.Fatalf("expected raw SSE body to stay in debug response, got %#v", debug)
	}
}

func TestMessageErrorSummarySuggestsDisablingImageStreamForParseFailure(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 500,
		Message:    "invalid character 'e' looking for beginning of value",
		Body:       `{"error":{"message":"invalid character 'e' looking for beginning of value"}}`,
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

	summary := MessageErrorSummary(err)
	if !strings.Contains(summary, "Tips：当前上游可能不支持流式响应") || !strings.Contains(summary, "image.stream=false") {
		t.Fatalf("expected image stream configuration hint, got %q", summary)
	}
	if code := MessageErrorCode(err); code != MessageErrorCodeMediaImageStreamUnsupported {
		t.Fatalf("MessageErrorCode() = %q, want %q", code, MessageErrorCodeMediaImageStreamUnsupported)
	}
}

func TestMessageErrorSummarySuggestsDisablingGeminiImageStreamForParseFailure(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 500,
		Message:    "invalid character 'e' looking for beginning of value",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1beta/models/nano-banana-pro:streamGenerateContent",
				Body:   `{"contents":[{"role":"user","parts":[{"text":"a cat"}]}],"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 500,
				Body:       `{"error":{"message":"invalid character 'e' looking for beginning of value"}}`,
			},
		},
	})

	summary := MessageErrorSummary(err)
	if !strings.Contains(summary, "Tips：当前上游可能不支持流式响应") || !strings.Contains(summary, "image.stream=false") {
		t.Fatalf("expected gemini image stream configuration hint, got %q", summary)
	}
	if code := MessageErrorCode(err); code != MessageErrorCodeMediaImageStreamUnsupported {
		t.Fatalf("MessageErrorCode() = %q, want %q", code, MessageErrorCodeMediaImageStreamUnsupported)
	}
}

func TestMessageErrorSummaryDoesNotSuggestImageStreamForChatStreamParseFailure(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 500,
		Message:    "invalid character 'e' looking for beginning of value",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/responses",
				Body:   `{"model":"gpt-5","input":"hello","stream":true}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 500,
				Body:       `{"error":{"message":"invalid character 'e' looking for beginning of value"}}`,
			},
		},
	})

	summary := MessageErrorSummary(err)
	if strings.Contains(summary, "图像流式调用") || strings.Contains(summary, "image.stream=false") {
		t.Fatalf("expected no image stream configuration hint, got %q", summary)
	}
	if code := MessageErrorCode(err); code != "" {
		t.Fatalf("MessageErrorCode() = %q, want empty", code)
	}
}

func TestMessageErrorDebugKeepsSnapshotButRemovesUpstreamNames(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 502,
		Message:    "bad gateway",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/responses",
				Headers: map[string]string{
					"Authorization": "[redacted]",
					"Content-Type":  "application/json",
				},
				Body: `{"model":"grok-4","upstream_name":"Oi Hub","upstream":{"name":"Oi Hub","id":7},"messages":[{"role":"user","content":"hi"}]}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 502,
				Headers: map[string]string{
					"Provider":    "ExampleEdge",
					"Server":      "ExampleCDN",
					"X-Client-Ip": "127.0.0.1",
				},
				Body: `{"error":{"message":"bad gateway"},"upstreamName":"Oi Hub","data":{"upstream":{"displayName":"Oi Hub","status":"failed"}}}`,
			},
		},
	})

	debug := MessageErrorDebug(err)
	if debug == nil {
		t.Fatal("expected debug snapshot")
	}
	if debug.Request.Headers != nil || debug.Response.Headers != nil {
		t.Fatalf("expected public debug headers to be omitted, got request=%#v response=%#v", debug.Request.Headers, debug.Response.Headers)
	}
	for _, body := range []string{debug.Request.Body, debug.Response.Body} {
		if strings.Contains(body, "Oi Hub") || strings.Contains(body, "upstream_name") || strings.Contains(body, "upstreamName") || strings.Contains(body, "displayName") {
			t.Fatalf("expected upstream name fields removed, got %s", body)
		}
	}
	if !strings.Contains(debug.Request.Body, `"model":"grok-4"`) || !strings.Contains(debug.Response.Body, `"message":"bad gateway"`) {
		t.Fatalf("expected non-name debug body fields preserved, request=%s response=%s", debug.Request.Body, debug.Response.Body)
	}
}
