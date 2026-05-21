package db

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestProductionGORMLoggerIgnoresRecordNotFound(t *testing.T) {
	if output := captureProductionGORMTrace(t, gorm.ErrRecordNotFound); output != "" {
		t.Fatalf("expected record not found to be ignored, got %q", output)
	}

	output := captureProductionGORMTrace(t, errors.New("connection failed"))
	if !strings.Contains(output, "connection failed") {
		t.Fatalf("expected non-not-found error to be logged, got %q", output)
	}
}

func captureProductionGORMTrace(t *testing.T, traceErr error) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	logger := productionGORMLogger()
	os.Stdout = originalStdout

	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT 1", 0
	}, traceErr)

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return string(output)
}
