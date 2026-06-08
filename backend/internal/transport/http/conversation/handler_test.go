package conversation

import (
	"errors"
	"net/http/httptest"
	"testing"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/gin-gonic/gin"
)

func TestSafeFileContentTypeDowngradesActiveContent(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{contentType: "text/html; charset=utf-8", want: "text/plain; charset=utf-8"},
		{contentType: "application/javascript", want: "text/plain; charset=utf-8"},
		{contentType: "image/svg+xml", want: "text/plain; charset=utf-8"},
		{contentType: "application/pdf", want: "application/pdf"},
	}
	for _, tt := range tests {
		if got := safeFileContentType(tt.contentType); got != tt.want {
			t.Fatalf("safeFileContentType(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}

func TestMessagePageParamsAllowsRestoreWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/messages?page=1&page_size=1000", nil)

	_, pageSize := messagePageParams(c)
	if pageSize != 1000 {
		t.Fatalf("messagePageParams page size = %d, want 1000", pageSize)
	}

	_, normalPageSize := pageParams(c)
	if normalPageSize != maxHTTPPageSize {
		t.Fatalf("pageParams page size = %d, want %d", normalPageSize, maxHTTPPageSize)
	}
}

func TestBuildContentDispositionDefaultsToAttachment(t *testing.T) {
	got := buildContentDisposition("report.html", false)
	want := `attachment; filename="report.html"; filename*=UTF-8''report.html`
	if got != want {
		t.Fatalf("unexpected disposition: got %q want %q", got, want)
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

	payload := streamErrorPayload(err)
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

func TestMapStreamErrorDoesNotExposeUpstreamUnauthorizedAsPlatformUnauthorized(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 401,
		Message:    "upstream authentication failed",
	})

	mapped := mapStreamError(err)
	if mapped.Status != 502 {
		t.Fatalf("expected upstream 401 to be mapped to gateway failure, got status=%d", mapped.Status)
	}
	if mapped.Code == "auth.unauthorized" || mapped.Code == "auth.invalid_token" || mapped.Code == "auth.session_invalid" {
		t.Fatalf("expected upstream 401 to avoid platform auth codes, got %#v", mapped)
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

	payload := streamErrorPayload(err)
	if got := payload["errorCode"]; got != appconversation.MessageErrorCodeMediaImageStreamUnsupported {
		t.Fatalf("errorCode = %#v, want %q", got, appconversation.MessageErrorCodeMediaImageStreamUnsupported)
	}
}
