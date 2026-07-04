package llm

import (
	"testing"
)

func TestSupportsStreamingAdapter(t *testing.T) {
	if !SupportsStreamingAdapter(AdapterOpenAIImageGenerations) {
		t.Fatalf("expected image generations adapter to support upstream streaming")
	}
	if !SupportsStreamingAdapter(AdapterOpenAIImageEdits) {
		t.Fatalf("expected image edits adapter to support upstream streaming")
	}
	if !SupportsStreamingAdapter(AdapterOpenAIResponses) {
		t.Fatalf("expected responses adapter to support streaming")
	}
}

func TestSupportsImageGenerationStream(t *testing.T) {
	if !SupportsImageGenerationStream(AdapterOpenAIImageGenerations, "gpt-image-1") {
		t.Fatalf("expected gpt-image models to support image generation streaming")
	}
	if !SupportsImageGenerationStream(AdapterOpenAIImageGenerations, "gpt-image-2") {
		t.Fatalf("expected gpt-image-2 to support image generation streaming")
	}
	if !SupportsStreamingAdapter(AdapterGoogleImageGeneration) {
		t.Fatalf("expected google image generation adapter to support upstream streaming")
	}
	if !SupportsImageGenerationStream(AdapterGoogleImageGeneration, "gemini-3-pro-image") {
		t.Fatalf("expected google image generation adapter to support image generation streaming")
	}
	if !SupportsImageGenerationStream(AdapterGeminiInteractions, "gemini-3.5-flash") {
		t.Fatalf("expected Gemini Interactions adapter to support image generation streaming")
	}
	if SupportsStreamingAdapter(AdapterXAIImage) {
		t.Fatalf("expected xAI image adapter to use non-streaming media flow")
	}
	if SupportsStreamingAdapter(AdapterXAIImageEdits) {
		t.Fatalf("expected xAI image edits adapter to use non-streaming media flow")
	}
	if SupportsImageGenerationStream(AdapterOpenAIImageGenerations, "dall-e-3") {
		t.Fatalf("expected DALL-E models to remain non-streaming")
	}
	if SupportsImageGenerationStream(AdapterOpenAIResponses, "gpt-image-1") {
		t.Fatalf("expected non-image protocol to remain non-streaming for image generation")
	}
	if !SupportsImageGenerationStream(AdapterOpenAIImageEdits, "gpt-image-1") {
		t.Fatalf("expected gpt-image edits to support image edit streaming")
	}
	if !SupportsImageGenerationStream(AdapterOpenAIImageEdits, "gpt-image-2") {
		t.Fatalf("expected gpt-image-2 edits to support image edit streaming")
	}
}

func TestImageAdapterCapabilities(t *testing.T) {
	if !IsImageGenerationAdapter(AdapterGoogleImageGeneration) {
		t.Fatalf("expected google image protocol to support image generation")
	}
	if !IsImageEditAdapter(AdapterGoogleImageGeneration) {
		t.Fatalf("expected google image protocol to support image editing")
	}
	if !IsImageGenerationAdapter(AdapterXAIImage) {
		t.Fatalf("expected xAI image protocol to support image generation")
	}
	if IsImageEditAdapter(AdapterXAIImage) {
		t.Fatalf("expected xAI image protocol to stay generation-only")
	}
	if !IsImageEditAdapter(AdapterXAIImageEdits) {
		t.Fatalf("expected xAI image edits protocol to support image editing")
	}
}
