package repository

import (
	"context"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

// ContentModerationRepository persists moderation events and daily stats.
type ContentModerationRepository interface {
	CreateEvent(ctx context.Context, event *domaincm.Event) error
	GetEventByPublicID(ctx context.Context, publicID string) (*domaincm.Event, error)
	GetLatestHitEventByRunID(ctx context.Context, runID string) (*domaincm.Event, error)
	ListEvents(ctx context.Context, filter domaincm.EventListFilter) ([]domaincm.Event, int64, error)
	// ClearExpiredContentByPublicIDs clears payloads only for events whose isolated objects were deleted.
	ClearExpiredContentByPublicIDs(ctx context.Context, publicIDs []string) (int64, error)
	ListExpiredContentEvents(ctx context.Context, before time.Time, limit int) ([]domaincm.Event, error)
	DeleteExpiredMetadata(ctx context.Context, before time.Time) (int64, error)
	IncrementDailyStat(ctx context.Context, input DailyStatIncrement) error
	ListDailyStats(ctx context.Context, from, to time.Time) ([]domaincm.DailyStat, error)
	DeleteDailyStatsBefore(ctx context.Context, before time.Time) (int64, error)
	UpdateRunModeration(ctx context.Context, runID string, state string, eventPublicID string, categoriesJSON string) error
	// ApplyRunBlock atomically marks messages/output files blocked, clears assistant content/traces,
	// and updates run state. It returns output file IDs that need physical object cleanup.
	ApplyRunBlock(ctx context.Context, runID string, includeUser bool, eventPublicID string, categoriesJSON string) ([]string, error)
	GetRunModerationState(ctx context.Context, runID string) (state string, err error)
	ListStaleModeratingRuns(ctx context.Context, olderThan time.Time, limit int) ([]string, error)
}

// DailyStatIncrement updates anonymous daily counters.
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
