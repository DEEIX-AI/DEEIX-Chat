package repository

import (
	"context"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

// ContentModerationRepository 持久化审核事件与每日统计。
type ContentModerationRepository interface {
	CreateEvent(ctx context.Context, event *domaincm.Event) error
	GetEventByPublicID(ctx context.Context, publicID string) (*domaincm.Event, error)
	GetLatestHitEventByRunID(ctx context.Context, runID string) (*domaincm.Event, error)
	ListEvents(ctx context.Context, filter domaincm.EventListFilter) ([]domaincm.Event, int64, error)
	// ClearExpiredContentByPublicIDs 仅清除其隔离对象已被删除的事件载荷。
	ClearExpiredContentByPublicIDs(ctx context.Context, publicIDs []string) (int64, error)
	ListExpiredContentEvents(ctx context.Context, before time.Time, limit int) ([]domaincm.Event, error)
	DeleteExpiredMetadata(ctx context.Context, before time.Time) (int64, error)
	IncrementDailyStat(ctx context.Context, input DailyStatIncrement) error
	ListDailyStats(ctx context.Context, from, to time.Time) ([]domaincm.DailyStat, error)
	DeleteDailyStatsBefore(ctx context.Context, before time.Time) (int64, error)
	UpdateRunModeration(ctx context.Context, runID string, state string, eventPublicID string, categoriesJSON string) error
	// ApplyRunBlock 原子地将消息/输出文件标记为已拦截、清除助手内容/轨迹，
	// 并更新 run 状态。返回需要物理清理对象的输出文件 ID。
	ApplyRunBlock(ctx context.Context, runID string, includeUser bool, eventPublicID string, categoriesJSON string) ([]string, error)
	GetRunModerationState(ctx context.Context, runID string) (state string, err error)
	ListStaleModeratingRuns(ctx context.Context, olderThan time.Time, limit int) ([]string, error)
}

// DailyStatIncrement 更新匿名每日计数。
type DailyStatIncrement struct {
	StatDate     time.Time
	Direction    string
	Modality     string
	Result       string
	Category     string
	CheckCount   int64
	ContentItems int64
	HitCount     int64
	FailureCount int64
	LatencyMS    int64
}
