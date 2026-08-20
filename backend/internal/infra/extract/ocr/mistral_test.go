package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestMistralOCRPDFRequestAndResponse(t *testing.T) {
	pdf := []byte("pdf-content")
	filePath := writeMistralOCRTestFile(t, "document.pdf", pdf)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Fatalf("Authorization = %q, want Bearer token", got)
		}
		for _, header := range []string{"X-API-Key", "token"} {
			if got := r.Header.Get(header); got != "" {
				t.Fatalf("%s = %q, want absent", header, got)
			}
		}

		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		if r.ContentLength != int64(len(rawBody)) {
			t.Fatalf("Content-Length = %d, body length = %d", r.ContentLength, len(rawBody))
		}
		if len(r.TransferEncoding) != 0 {
			t.Fatalf("Transfer-Encoding = %v, want none", r.TransferEncoding)
		}
		var payload mistralOCRRequest
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "mistral-ocr-latest" {
			t.Fatalf("model = %q", payload.Model)
		}
		if payload.Document.Type != "document_url" {
			t.Fatalf("document.type = %q", payload.Document.Type)
		}
		if got := payload.Document.DocumentURL; got != "data:application/pdf;base64,"+base64.StdEncoding.EncodeToString(pdf) {
			t.Fatalf("document_url = %q", got)
		}
		if !reflect.DeepEqual(payload.Pages, []int{0, 1, 3}) {
			t.Fatalf("pages = %v, want [0 1 3]", payload.Pages)
		}
		if payload.IncludeImageBase64 {
			t.Fatal("include_image_base64 must be false")
		}
		if payload.IncludeBlocks {
			t.Fatal("include_blocks must be false")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pages":[{"index":3,"markdown":"fourth"},{"index":0,"markdown":"first"},{"index":1,"markdown":"second"},{"index":2,"markdown":"  "}]}`))
	}))
	defer server.Close()

	client := newMistralOCRTestClient(t, server.URL)
	result, err := client.ExtractText(context.Background(), Request{
		AbsolutePath: filePath,
		FileName:     "document.pdf",
		MimeType:     "application/pdf",
		PageRanges: []PageRange{
			{Start: 1, End: 2},
			{Start: 4, End: 4},
		},
	})
	if err != nil {
		t.Fatalf("extract text: %v", err)
	}
	if result.Text != "first\n\nsecond\n\nfourth" {
		t.Fatalf("text = %q", result.Text)
	}
	if result.RenderedPages != 3 {
		t.Fatalf("rendered pages = %d, want 3", result.RenderedPages)
	}
	if got := []int{result.Pages[0].PageNumber, result.Pages[1].PageNumber, result.Pages[2].PageNumber}; !reflect.DeepEqual(got, []int{1, 2, 4}) {
		t.Fatalf("page numbers = %v, want [1 2 4]", got)
	}
}

func TestParseMistralOCRResponsePreservesMarkdown(t *testing.T) {
	markdown := "    indented code\n\n# Heading\n\n- parent\n  - child\n\n```go\n  fmt.Println(\"value\")\n```\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n"
	body, err := json.Marshal(mistralOCRResponse{Pages: []mistralOCRPage{{Index: 0, Markdown: markdown}}})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	result, err := parseMistralOCRResponse(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if result.Text != markdown {
		t.Fatalf("text = %q, want exact Markdown %q", result.Text, markdown)
	}
	if len(result.Pages) != 1 || result.Pages[0].Text != markdown {
		t.Fatalf("pages = %+v, want exact page Markdown", result.Pages)
	}
}

func TestMistralOCRRequestBodyReplaysAcrossRedirect(t *testing.T) {
	pdf := []byte("redirected-pdf-content")
	filePath := writeMistralOCRTestFile(t, "document.pdf", pdf)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/redirect":
			w.Header().Set("Location", "/ocr")
			w.WriteHeader(http.StatusTemporaryRedirect)
		case "/ocr":
			var payload mistralOCRRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode redirected request: %v", err)
			}
			wantURL := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(pdf)
			if payload.Document.DocumentURL != wantURL {
				t.Fatalf("redirected document_url = %q, want %q", payload.Document.DocumentURL, wantURL)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"pages":[{"index":0,"markdown":"redirected"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := newMistralOCRTestClient(t, server.URL+"/redirect").ExtractText(context.Background(), Request{
		AbsolutePath: filePath,
		FileName:     "document.pdf",
		MimeType:     "application/pdf",
	})
	if err != nil {
		t.Fatalf("extract redirected document: %v", err)
	}
	if result.Text != "redirected" {
		t.Fatalf("text = %q, want redirected", result.Text)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
}

func TestMistralOCRImageRequestOmitsPages(t *testing.T) {
	image := []byte("image-content")
	filePath := writeMistralOCRTestFile(t, "image.png", image)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := payload["pages"]; ok {
			t.Fatal("image OCR request must not include pages")
		}
		var document mistralOCRDocument
		if err := json.Unmarshal(payload["document"], &document); err != nil {
			t.Fatalf("decode document: %v", err)
		}
		if document.Type != "image_url" {
			t.Fatalf("document.type = %q", document.Type)
		}
		wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
		if document.ImageURL != wantURL {
			t.Fatalf("image_url = %q, want %q", document.ImageURL, wantURL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pages":[{"index":0,"markdown":"recognized image"}]}`))
	}))
	defer server.Close()

	result, err := newMistralOCRTestClient(t, server.URL).ExtractText(context.Background(), Request{
		AbsolutePath: filePath,
		FileName:     "image.png",
		MimeType:     "image/png",
	})
	if err != nil {
		t.Fatalf("extract image text: %v", err)
	}
	if result.Text != "recognized image" {
		t.Fatalf("text = %q", result.Text)
	}
}

func TestMistralOCRErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "empty content", status: http.StatusOK, body: `{"pages":[{"index":0,"markdown":" "}]}`, want: "ocr_empty_content"},
		{name: "unauthorized", status: http.StatusUnauthorized, want: "ocr_unauthorized"},
		{name: "forbidden", status: http.StatusForbidden, want: "ocr_forbidden"},
		{name: "unprocessable", status: http.StatusUnprocessableEntity, want: "ocr_unprocessable"},
		{name: "http error", status: http.StatusInternalServerError, want: "ocr_http_500"},
	}

	filePath := writeMistralOCRTestFile(t, "document.pdf", []byte("pdf-content"))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := newMistralOCRTestClient(t, server.URL).ExtractText(context.Background(), Request{AbsolutePath: filePath, FileName: "document.pdf"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestNewMistralAllowsConfiguredEndpointWithStrictOutboundPolicy(t *testing.T) {
	filePath := writeMistralOCRTestFile(t, "document.pdf", []byte("pdf-content"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pages":[{"index":0,"markdown":"text"}]}`))
	}))
	defer server.Close()

	client := NewMistral(ClientConfig{
		BaseURL:        server.URL,
		AuthToken:      "test-api-key",
		Model:          "mistral-ocr-latest",
		TimeoutSeconds: 60,
		OutboundPolicy: security.NewStrictOutboundPolicy(true),
	})
	if client == nil {
		t.Fatal("NewMistral returned nil for an explicitly configured endpoint")
	}
	if _, err := client.ExtractText(context.Background(), Request{AbsolutePath: filePath, FileName: "document.pdf"}); err != nil {
		t.Fatalf("configured endpoint request failed: %v", err)
	}
}

func newMistralOCRTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client := NewMistral(ClientConfig{
		BaseURL:        baseURL,
		AuthToken:      "test-api-key",
		Model:          "mistral-ocr-latest",
		TimeoutSeconds: 60,
		OutboundPolicy: security.NewStrictOutboundPolicy(true),
	})
	if client == nil {
		t.Fatal("NewMistral returned nil")
	}
	return client
}

func writeMistralOCRTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}
