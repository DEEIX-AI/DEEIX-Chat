package errorlog

import (
	"context"
	"strings"
	"testing"
	"time"

	domainerrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/errorlog"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openErrorLogSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.ErrorLog{}); err != nil {
		t.Fatalf("migrate error logs: %v", err)
	}
	return db
}

func TestErrorLogListFindsByRequestIDAndFilters(t *testing.T) {
	repo := NewRepo(openErrorLogSQLiteTestDB(t))
	ctx := context.Background()
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	for _, item := range []domainerrorlog.Log{
		{RequestID: "req-a", UserID: 1, Route: "/api/v1/conversations/:id/messages/stream", StatusCode: 200, ErrorCode: "upstream.unavailable", Cause: "upstream request failed: status=500", CreatedAt: base},
		{RequestID: "req-b", UserID: 2, Route: "/api/v1/settings", StatusCode: 500, ErrorCode: "internal.error", Cause: "generate failed", Detail: `{"error":{"message":"model not found"}}`, CreatedAt: base.Add(time.Minute)},
		{RequestID: "req-ab", UserID: 2, Route: "/api/v1/files", StatusCode: 503, ErrorCode: "service.unavailable", Cause: "storage offline", CreatedAt: base.Add(2 * time.Minute)},
	} {
		item := item
		if err := repo.Create(ctx, &item); err != nil {
			t.Fatalf("create %s: %v", item.RequestID, err)
		}
	}

	list := func(filter repository.ErrorLogListFilter) []domainerrorlog.Log {
		t.Helper()
		items, total, err := repo.List(ctx, 0, 20, filter)
		if err != nil {
			t.Fatalf("list %+v: %v", filter, err)
		}
		if int(total) != len(items) {
			t.Fatalf("total %d != len %d", total, len(items))
		}
		return items
	}

	// 错误 ID 精确匹配：req-a 不应命中 req-ab。
	if got := list(repository.ErrorLogListFilter{Query: "req-a"}); len(got) != 1 || got[0].RequestID != "req-a" {
		t.Fatalf("query by request id: %#v", got)
	}
	if got := list(repository.ErrorLogListFilter{RequestID: "req-b"}); len(got) != 1 || got[0].Cause != "generate failed" {
		t.Fatalf("request_id filter: %#v", got)
	}
	if got := list(repository.ErrorLogListFilter{Query: "GENERATE FAILED"}); len(got) != 1 || got[0].RequestID != "req-b" {
		t.Fatalf("case-insensitive cause search: %#v", got)
	}
	// 原始上游响应体也要能搜到，否则拿到上游报错片段时无从定位。
	if got := list(repository.ErrorLogListFilter{Query: "model not found"}); len(got) != 1 || got[0].RequestID != "req-b" {
		t.Fatalf("detail search: %#v", got)
	}
	if got := list(repository.ErrorLogListFilter{StatusClass: "5xx"}); len(got) != 2 {
		t.Fatalf("5xx filter: %#v", got)
	}
	if got := list(repository.ErrorLogListFilter{StatusClass: "stream"}); len(got) != 1 || got[0].RequestID != "req-a" {
		t.Fatalf("stream filter: %#v", got)
	}
	if got := list(repository.ErrorLogListFilter{UserID: 2}); len(got) != 2 || got[0].RequestID != "req-ab" {
		t.Fatalf("user filter must be newest first: %#v", got)
	}
}
