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

func TestMessageErrorDebugKeepsSnapshotButRemovesUpstreamNames(t *testing.T) {
	err := wrapUpstreamRequestError(&llm.UpstreamError{
		StatusCode: 502,
		Message:    "bad gateway",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/responses",
				Body:   `{"model":"grok-4","upstream_name":"Oi Hub","upstream":{"name":"Oi Hub","id":7},"messages":[{"role":"user","content":"hi"}]}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 502,
				Body:       `{"error":{"message":"bad gateway"},"upstreamName":"Oi Hub","data":{"upstream":{"displayName":"Oi Hub","status":"failed"}}}`,
			},
		},
	})

	debug := MessageErrorDebug(err)
	if debug == nil {
		t.Fatal("expected debug snapshot")
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
