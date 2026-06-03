package conversation

import (
	"bytes"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestDetectGeneratedImageMIMERejectsNonImageBytes(t *testing.T) {
	_, _, err := validateGeneratedImageBytes([]byte("<html>not an image</html>"), "image/png")
	if err == nil {
		t.Fatal("expected non-image generated output to be rejected")
	}
}

func TestDetectGeneratedImageMIMEUsesActualImageBytes(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}
	got, mimeType, err := validateGeneratedImageBytes(data, "image/png")
	if err != nil {
		t.Fatalf("expected jpeg bytes to pass validation: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("expected validation to return original bytes")
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("expected actual jpeg MIME, got %q", mimeType)
	}
}

func TestStripBase64DataURLPrefix(t *testing.T) {
	got := stripBase64DataURLPrefix("data:image/png;base64, aGVsbG8= ")
	if got != "aGVsbG8=" {
		t.Fatalf("unexpected stripped data URL: %q", got)
	}
}

func TestMediaImageStreamEnabledUsesModelCapabilitiesOverride(t *testing.T) {
	tests := []struct {
		name             string
		protocol         string
		upstreamModel    string
		capabilitiesJSON string
		want             bool
	}{
		{
			name:          "openai gpt image defaults to stream",
			protocol:      llm.AdapterOpenAIImageGenerations,
			upstreamModel: "gpt-image-2",
			want:          true,
		},
		{
			name:             "explicit false disables stream",
			protocol:         llm.AdapterOpenAIImageGenerations,
			upstreamModel:    "gpt-image-2",
			capabilitiesJSON: `{"image":{"stream":false}}`,
			want:             false,
		},
		{
			name:             "explicit true preserves protocol default",
			protocol:         llm.AdapterOpenAIImageGenerations,
			upstreamModel:    "gpt-image-2",
			capabilitiesJSON: `{"image":{"stream":true}}`,
			want:             true,
		},
		{
			name:             "invalid json keeps protocol default",
			protocol:         llm.AdapterGoogleImageGeneration,
			upstreamModel:    "gemini-3-pro-image",
			capabilitiesJSON: `{`,
			want:             true,
		},
		{
			name:             "unsupported protocol cannot be enabled by capabilities",
			protocol:         llm.AdapterXAIImage,
			upstreamModel:    "grok-2-image",
			capabilitiesJSON: `{"image":{"stream":true}}`,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mediaImageStreamEnabled(tt.protocol, tt.upstreamModel, tt.capabilitiesJSON)
			if got != tt.want {
				t.Fatalf("mediaImageStreamEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
