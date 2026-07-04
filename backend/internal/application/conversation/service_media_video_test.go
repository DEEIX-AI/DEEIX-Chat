package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestGeminiGeneratedFileURLsBuildsMetadataAndDownloadURLs(t *testing.T) {
	metadataURL, downloadURL, ok := geminiGeneratedFileURLs("https://generativelanguage.googleapis.com/v1beta/files/video_123?alt=media&key=secret")
	if !ok {
		t.Fatal("expected Gemini Files URL to be recognized")
	}
	if metadataURL != "https://generativelanguage.googleapis.com/v1beta/files/video_123" {
		t.Fatalf("unexpected metadata URL: %s", metadataURL)
	}
	if downloadURL != "https://generativelanguage.googleapis.com/v1beta/files/video_123:download?alt=media" {
		t.Fatalf("unexpected download URL: %s", downloadURL)
	}
}

func TestGeminiGeneratedFileURLsNormalizesDownloadURL(t *testing.T) {
	metadataURL, downloadURL, ok := geminiGeneratedFileURLs("https://generativelanguage.googleapis.com/v1beta/files/video_123:download?alt=media")
	if !ok {
		t.Fatal("expected Gemini download URL to be recognized")
	}
	if metadataURL != "https://generativelanguage.googleapis.com/v1beta/files/video_123" {
		t.Fatalf("unexpected metadata URL: %s", metadataURL)
	}
	if downloadURL != "https://generativelanguage.googleapis.com/v1beta/files/video_123:download?alt=media" {
		t.Fatalf("unexpected download URL: %s", downloadURL)
	}
}

func TestGeminiGeneratedFileURLsRejectsNonFileURLs(t *testing.T) {
	if _, _, ok := geminiGeneratedFileURLs("https://generativelanguage.googleapis.com/v1beta/files/video_123"); !ok {
		t.Fatal("expected Gemini Files URL to be recognized")
	}
	if _, _, ok := geminiGeneratedFileURLs("https://example.com/v1beta/files/video_123"); ok {
		t.Fatal("expected non-Gemini host to be ignored")
	}
	if _, _, ok := geminiGeneratedFileURLs("https://generativelanguage.googleapis.com/v1beta/models/gemini"); ok {
		t.Fatal("expected non-Files Gemini URL to be ignored")
	}
}

func TestGeminiGeneratedFileStateHelpers(t *testing.T) {
	if !geminiGeneratedFileReady("ACTIVE") {
		t.Fatal("expected ACTIVE to be accepted")
	}
	if geminiGeneratedFileReady("READY") || geminiGeneratedFileReady("PROCESSED") {
		t.Fatal("expected non-Files ready aliases to stay pending")
	}
	if !geminiGeneratedFileFailed("FAILED") {
		t.Fatal("expected FAILED to be rejected")
	}
	if geminiGeneratedFileFailed("ERROR") || geminiGeneratedFileFailed("CANCELLED") {
		t.Fatal("expected non-Files failure aliases to stay pending")
	}
	if geminiGeneratedFileReady("PROCESSING") || geminiGeneratedFileFailed("PROCESSING") {
		t.Fatal("expected processing to stay pending")
	}
}

func TestReadGeneratedVideoRequiresAPIKeyForGeminiFilesURL(t *testing.T) {
	service := &Service{}
	_, _, err := service.readGeneratedVideo(context.Background(), llm.GeneratedVideo{
		URL:      "https://generativelanguage.googleapis.com/v1beta/files/video_123:download?alt=media",
		MIMEType: "video/mp4",
	}, "")
	if err == nil || !strings.Contains(err.Error(), "requires an API key") {
		t.Fatalf("expected missing Gemini API key to be rejected, got %v", err)
	}
}

func TestValidateGeneratedVideoBytesRejectsUndetectedContent(t *testing.T) {
	if _, _, err := validateGeneratedVideoBytes([]byte("not a video"), "video/mp4"); err == nil {
		t.Fatal("expected declared video MIME to be insufficient without a supported video header")
	}
}
