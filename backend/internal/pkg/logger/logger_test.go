package logger

import (
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestMessageOnlyCoreMovesFieldsIntoMessage(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	item := zap.New(newMessageOnlyCore(core)).With(zap.String("service", "deeix-chat"))

	item.Info("request", zap.String("http.method", "GET"), zap.Int("http.status_code", 200))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	if len(entries[0].Context) != 0 {
		t.Fatalf("expected context fields to be empty, got %#v", entries[0].Context)
	}
	for _, part := range []string{"request", "service deeix-chat", "method GET", "status_code 200"} {
		if !strings.Contains(entries[0].Message, part) {
			t.Fatalf("expected message to contain %q, got %q", part, entries[0].Message)
		}
	}
}

func TestMessageOnlyCoreKeepsErrorInMessage(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	item := zap.New(newMessageOnlyCore(core))

	item.Error("persist failed", zap.Error(errors.New("record not found")))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	if len(entries[0].Context) != 0 {
		t.Fatalf("expected context fields to be empty, got %#v", entries[0].Context)
	}
	if !strings.Contains(entries[0].Message, "error record not found") {
		t.Fatalf("expected error in message, got %q", entries[0].Message)
	}
}
