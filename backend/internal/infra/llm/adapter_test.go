package llm

import (
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestSupportsStreamingAdapter(t *testing.T) {
	if !portllm.SupportsStreamingAdapter(portllm.AdapterOpenAIImageGenerations) {
		t.Fatalf("expected image generations adapter to support upstream streaming")
	}
	if !portllm.SupportsStreamingAdapter(portllm.AdapterOpenAIImageEdits) {
		t.Fatalf("expected image edits adapter to support upstream streaming")
	}
	if !portllm.SupportsStreamingAdapter(portllm.AdapterOpenAIResponses) {
		t.Fatalf("expected responses adapter to support streaming")
	}
}

func TestSupportsImageGenerationStream(t *testing.T) {
	if !portllm.SupportsImageGenerationStream(portllm.AdapterOpenAIImageGenerations, "gpt-image-1") {
		t.Fatalf("expected gpt-image models to support image generation streaming")
	}
	if !portllm.SupportsImageGenerationStream(portllm.AdapterOpenAIImageGenerations, "gpt-image-2") {
		t.Fatalf("expected gpt-image-2 to support image generation streaming")
	}
	if !portllm.SupportsStreamingAdapter(portllm.AdapterGoogleImageGeneration) {
		t.Fatalf("expected google image generation adapter to support upstream streaming")
	}
	if !portllm.SupportsImageGenerationStream(portllm.AdapterGoogleImageGeneration, "gemini-3-pro-image") {
		t.Fatalf("expected google image generation adapter to support image generation streaming")
	}
	if !portllm.SupportsImageGenerationStream(portllm.AdapterGeminiInteractions, "gemini-3.5-flash") {
		t.Fatalf("expected Gemini Interactions adapter to support image generation streaming")
	}
	if portllm.SupportsStreamingAdapter(portllm.AdapterXAIImage) {
		t.Fatalf("expected xAI image adapter to use non-streaming media flow")
	}
	if portllm.SupportsStreamingAdapter(portllm.AdapterXAIImageEdits) {
		t.Fatalf("expected xAI image edits adapter to use non-streaming media flow")
	}
	if portllm.SupportsImageGenerationStream(portllm.AdapterOpenAIImageGenerations, "dall-e-3") {
		t.Fatalf("expected DALL-E models to remain non-streaming")
	}
	if portllm.SupportsImageGenerationStream(portllm.AdapterOpenAIResponses, "gpt-image-1") {
		t.Fatalf("expected non-image protocol to remain non-streaming for image generation")
	}
	if !portllm.SupportsImageGenerationStream(portllm.AdapterOpenAIImageEdits, "gpt-image-1") {
		t.Fatalf("expected gpt-image edits to support image edit streaming")
	}
	if !portllm.SupportsImageGenerationStream(portllm.AdapterOpenAIImageEdits, "gpt-image-2") {
		t.Fatalf("expected gpt-image-2 edits to support image edit streaming")
	}
}

func TestImageAdapterCapabilities(t *testing.T) {
	if !portllm.IsImageGenerationAdapter(portllm.AdapterGoogleImageGeneration) {
		t.Fatalf("expected google image protocol to support image generation")
	}
	if !portllm.IsImageEditAdapter(portllm.AdapterGoogleImageGeneration) {
		t.Fatalf("expected google image protocol to support image editing")
	}
	if !portllm.IsImageGenerationAdapter(portllm.AdapterXAIImage) {
		t.Fatalf("expected xAI image protocol to support image generation")
	}
	if portllm.IsImageEditAdapter(portllm.AdapterXAIImage) {
		t.Fatalf("expected xAI image protocol to stay generation-only")
	}
	if !portllm.IsImageEditAdapter(portllm.AdapterXAIImageEdits) {
		t.Fatalf("expected xAI image edits protocol to support image editing")
	}
}

func TestXAIVideoAdapterCapabilities(t *testing.T) {
	if !portllm.IsImplementedAdapter(portllm.AdapterXAIVideo) {
		t.Fatalf("expected xAI video adapter to be known and implemented")
	}
	if !portllm.IsVideoGenerationAdapter(portllm.AdapterXAIVideo) {
		t.Fatalf("expected xAI video adapter to support video generation")
	}
	if portllm.SupportsStreamingAdapter(portllm.AdapterXAIVideo) {
		t.Fatalf("expected xAI video adapter to use asynchronous polling instead of streaming")
	}
	if got := portllm.DefaultEndpointForAdapter(portllm.AdapterXAIVideo); got != portllm.EndpointVideoGenerations {
		t.Fatalf("expected xAI video endpoint, got %q", got)
	}
	if !portllm.IsImplementedAdapter(portllm.AdapterXAIVideoExtensions) {
		t.Fatalf("expected xAI video extensions adapter to be known and implemented")
	}
	if !portllm.IsVideoGenerationAdapter(portllm.AdapterXAIVideoExtensions) {
		t.Fatalf("expected xAI video extensions adapter to use the video media pipeline")
	}
	if got := portllm.DefaultEndpointForAdapter(portllm.AdapterXAIVideoExtensions); got != portllm.EndpointVideoExtensions {
		t.Fatalf("expected xAI video extensions endpoint, got %q", got)
	}
}
