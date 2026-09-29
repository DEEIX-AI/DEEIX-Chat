package embedding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestCallAPITrustsConfiguredPrivateEmbeddingOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected embedding path: %s", request.URL.Path)
		}
		var body openAIRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Dimensions == nil || *body.Dimensions != 3 {
			t.Fatalf("request dimensions = %v, want 3", body.Dimensions)
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	result, err := client.CallAPI(context.Background(), portembedding.Request{APIBase: server.URL + "/v1", Model: "test", Inputs: portembedding.TextInputs([]string{"hello"}), Dimensions: 3, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("call configured private embedding endpoint: %v", err)
	}
	if len(result) != 1 || len(result[0]) != 3 {
		t.Fatalf("unexpected embedding result: %#v", result)
	}
}

func TestCallAPIRejectsUnexpectedEmbeddingDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	_, err := client.CallAPI(context.Background(), portembedding.Request{APIBase: server.URL + "/v1", Model: "test", Inputs: portembedding.TextInputs([]string{"hello"}), Dimensions: 4, TimeoutSeconds: 5})
	if err == nil || !strings.Contains(err.Error(), "has 3 dimensions, expected 4") {
		t.Fatalf("expected explicit dimension mismatch, got %v", err)
	}
}

func TestCallAPIOmitsDimensionsWithoutRelaxingResponseValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := body["dimensions"]; ok {
			t.Fatal("request unexpectedly included dimensions")
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	_, err := client.CallAPI(context.Background(), portembedding.Request{
		APIBase:        server.URL + "/v1",
		Model:          "test",
		Inputs:         portembedding.TextInputs([]string{"hello"}),
		Dimensions:     4,
		OmitDimensions: true,
		TimeoutSeconds: 5,
	})
	if err == nil || !strings.Contains(err.Error(), "has 3 dimensions, expected 4") {
		t.Fatalf("expected response dimension validation, got %v", err)
	}
}

func TestCallAPIAcceptsDimensionChangesInBothDirections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var body openAIRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Dimensions == nil {
			t.Fatal("request dimensions unexpectedly omitted")
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"data": []map[string]any{{"index": 0, "embedding": make([]float32, *body.Dimensions)}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	for _, dimensions := range []int{4096, 1536, 4096} {
		result, err := client.CallAPI(context.Background(), portembedding.Request{APIBase: server.URL + "/v1", Model: "test", Inputs: portembedding.TextInputs([]string{"hello"}), Dimensions: dimensions, TimeoutSeconds: 5})
		if err != nil {
			t.Fatalf("CallAPI(%d) error = %v", dimensions, err)
		}
		if len(result) != 1 || len(result[0]) != dimensions {
			t.Fatalf("CallAPI(%d) returned dimensions %d", dimensions, len(result[0]))
		}
	}
}

func TestCallAPIGeminiEncodesTextAndImageBatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1beta/models/gemini-embedding-2:batchEmbedContents" {
			t.Fatalf("unexpected gemini path: %s", request.URL.Path)
		}
		if request.Header.Get("x-goog-api-key") != "secret" {
			t.Fatalf("gemini api key header missing: %q", request.Header.Get("x-goog-api-key"))
		}
		if request.Header.Get("Authorization") != "" {
			t.Fatal("gemini request must not carry a bearer token")
		}
		var body geminiBatchRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Requests) != 2 {
			t.Fatalf("expected 2 requests, got %d", len(body.Requests))
		}
		if body.Requests[0].Model != "models/gemini-embedding-2" || body.Requests[0].Content.Parts[0].Text != "hello" {
			t.Fatalf("unexpected text request: %#v", body.Requests[0])
		}
		if body.Requests[0].TaskType != "RETRIEVAL_DOCUMENT" {
			t.Fatalf("taskType not forwarded: %#v", body.Requests[0])
		}
		image := body.Requests[1].Content.Parts[0].InlineData
		if image == nil || image.MimeType != "image/png" || image.Data != "AQID" {
			t.Fatalf("unexpected image request: %#v", body.Requests[1])
		}
		if body.Requests[1].OutputDimensionality == nil || *body.Requests[1].OutputDimensionality != 3 {
			t.Fatalf("outputDimensionality not forwarded: %#v", body.Requests[1])
		}
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"embeddings": []map[string]any{{"values": []float32{1, 2, 3}}, {"values": []float32{4, 5, 6}}},
		})
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(true))
	result, err := client.CallAPI(context.Background(), portembedding.Request{
		Protocol: portembedding.ProtocolGemini,
		APIBase:  server.URL + "/v1beta",
		APIKey:   "secret",
		Model:    "gemini-embedding-2",
		Purpose:  portembedding.PurposeDocument,
		Inputs: []portembedding.Input{
			{Kind: portembedding.InputText, Text: "hello"},
			{Kind: portembedding.InputImage, MimeType: "image/png", Data: []byte{1, 2, 3}},
		},
		Dimensions:     3,
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("gemini call: %v", err)
	}
	if len(result) != 2 || result[1][0] != 4 {
		t.Fatalf("unexpected gemini result: %#v", result)
	}
}

func TestCallAPIOpenAIRejectsImageInputs(t *testing.T) {
	client := New(security.NewStrictOutboundPolicy(true))
	_, err := client.CallAPI(context.Background(), portembedding.Request{
		APIBase: "http://127.0.0.1:1/v1",
		Model:   "test",
		Inputs:  []portembedding.Input{{Kind: portembedding.InputImage, MimeType: "image/png", Data: []byte{1}}},
	})
	if !errors.Is(err, portembedding.ErrModalityUnsupported) {
		t.Fatalf("expected modality error before any request, got %v", err)
	}
}

func TestCallAPIVoyageAndJinaEncodeImagesAsDataURIs(t *testing.T) {
	cases := []struct {
		protocol portembedding.Protocol
		path     string
		check    func(t *testing.T, body map[string]any)
	}{
		{
			protocol: portembedding.ProtocolVoyage,
			path:     "/v1/multimodalembeddings",
			check: func(t *testing.T, body map[string]any) {
				inputs := body["inputs"].([]any)
				content := inputs[1].(map[string]any)["content"].([]any)[0].(map[string]any)
				if content["type"] != "image_base64" || content["image_base64"] != "data:image/png;base64,AQID" {
					t.Fatalf("unexpected voyage image content: %#v", content)
				}
				if body["output_dimension"] != float64(3) || body["input_type"] != "query" {
					t.Fatalf("output_dimension/input_type not forwarded: %#v", body)
				}
			},
		},
		{
			protocol: portembedding.ProtocolJina,
			path:     "/v1/embeddings",
			check: func(t *testing.T, body map[string]any) {
				inputs := body["input"].([]any)
				if inputs[0].(map[string]any)["text"] != "hello" || inputs[1].(map[string]any)["image"] != "data:image/png;base64,AQID" {
					t.Fatalf("unexpected jina inputs: %#v", inputs)
				}
				if body["dimensions"] != float64(3) {
					t.Fatalf("dimensions not forwarded: %#v", body)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(string(tc.protocol), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path != tc.path {
					t.Fatalf("unexpected path: %s", request.URL.Path)
				}
				if request.Header.Get("Authorization") != "Bearer secret" {
					t.Fatalf("bearer token missing: %q", request.Header.Get("Authorization"))
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				tc.check(t, body)
				_ = json.NewEncoder(responseWriter).Encode(map[string]any{
					"data": []map[string]any{{"index": 0, "embedding": []float32{1, 2, 3}}, {"index": 1, "embedding": []float32{4, 5, 6}}},
				})
			}))
			defer server.Close()

			client := New(security.NewStrictOutboundPolicy(true))
			result, err := client.CallAPI(context.Background(), portembedding.Request{
				Protocol: tc.protocol,
				APIBase:  server.URL + "/v1",
				APIKey:   "secret",
				Model:    "multimodal",
				Purpose:  portembedding.PurposeQuery,
				Inputs: []portembedding.Input{
					{Kind: portembedding.InputText, Text: "hello"},
					{Kind: portembedding.InputImage, MimeType: "image/png", Data: []byte{1, 2, 3}},
				},
				Dimensions:     3,
				TimeoutSeconds: 5,
			})
			if err != nil {
				t.Fatalf("%s call: %v", tc.protocol, err)
			}
			if len(result) != 2 || result[1][2] != 6 {
				t.Fatalf("unexpected result: %#v", result)
			}
		})
	}
}
