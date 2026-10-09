package response

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecordErrorDeduplicatesAndKeepsRicherChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	cause := errors.New("database is locked")
	RecordError(c, cause)
	// 错误映射 helper 入口先记录，随后 ErrorFrom 再次传入同一错误：不应重复拼接。
	RecordError(c, cause)
	if got := RecordedErrorCause(c).Error(); got != "database is locked" {
		t.Fatalf("duplicate record must be ignored, got %q", got)
	}

	// 补充了上下文的包裹错误替换原记录。
	wrapped := fmt.Errorf("persist message: %w", cause)
	RecordError(c, wrapped)
	if got := RecordedErrorCause(c).Error(); got != "persist message: database is locked" {
		t.Fatalf("wrapping error should replace the recorded cause, got %q", got)
	}

	// 无关的第二个错误与已有原因合并保留。
	RecordError(c, errors.New("rollback failed"))
	if got := RecordedErrorCause(c).Error(); !strings.Contains(got, "persist message") || !strings.Contains(got, "rollback failed") {
		t.Fatalf("unrelated errors must both be kept, got %q", got)
	}

	RecordError(c, nil)
	if RecordedErrorCause(c) == nil {
		t.Fatal("nil must not clear the recorded cause")
	}
}
