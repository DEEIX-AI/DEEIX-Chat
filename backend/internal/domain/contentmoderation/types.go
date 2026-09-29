package contentmoderation

import (
	"sort"
	"strings"
	"time"
)

// Direction 表示内容是用户输入还是模型输出。
const (
	DirectionInput  = "input"
	DirectionOutput = "output"
)

// Modality 表示文本或图片内容。
const (
	ModalityText  = "text"
	ModalityImage = "image"
)

// 事件与每日统计的结果取值。
const (
	ResultHit        = "hit"
	ResultFailedOpen = "failed_open"
	ResultPassed     = "passed"
)

// Run 的 moderation_state 取值。
const (
	ModerationStateNotRequired = "not_required"
	ModerationStatePending     = "pending"
	ModerationStateModerating  = "moderating"
	ModerationStatePassed      = "passed"
	ModerationStateBlocked     = "blocked"
	ModerationStateFailedOpen  = "failed_open"
)

// 被拦截轮次的消息/run 状态。
const (
	StatusBlocked = "blocked"
)

// 失败事件中存储的错误码。
const (
	ErrorCodeTimeout       = "timeout"
	ErrorCodeRateLimited   = "rate_limited"
	ErrorCodeQueueFull     = "queue_full"
	ErrorCodeServiceError  = "service_error"
	ErrorCodeInvalidResp   = "invalid_response"
	ErrorCodeWorkerLost    = "worker_lost"
	ErrorCodeNetworkError  = "network_error"
	ErrorCodeConfigMissing = "config_missing"
)

// Event 是一条审核检查记录（通过、命中或 failed-open）。
type Event struct {
	ID                  uint
	PublicID            string
	UserID              uint
	ConversationID      uint
	RunID               string
	MessageID           uint
	MessagePublicID     string
	Direction           string
	Modality            string
	Model               string
	PolicyVersion       int64
	Result              string
	CategoriesJSON      string
	CategoryScoresJSON  string
	LatencyMS           int64
	ErrorCode           string
	ErrorMessage        string
	ContentLocationJSON string
	ContentSummary      string
	EncryptedText       string
	ImageCount          int
	ImageMetaJSON       string
	ContentExpiresAt    time.Time
	MetadataExpiresAt   time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// DailyStat 按自然日聚合匿名计数。
type DailyStat struct {
	ID           uint
	StatDate     time.Time
	Direction    string
	Modality     string
	Result       string
	Category     string
	CheckCount   int64
	ContentItems int64
	HitCount     int64
	FailureCount int64
	LatencySumMS int64
	LatencyCount int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// EventListFilter 用于过滤超级管理员的事件查询。
type EventListFilter struct {
	Query     string
	Direction string
	Modality  string
	Result    string
	Category  string
	UserID    uint
	RunID     string
	From      *time.Time
	To        *time.Time
	Offset    int
	Limit     int
}

// IsolatedImageMeta 描述一份为审核而保留的加密图片副本。
type IsolatedImageMeta struct {
	Index        int
	SHA256       string
	MimeType     string
	SizeBytes    int64
	StoragePath  string
	SourceFileID string
}

// ContentLocation 描述被审核内容的来源位置。
type ContentLocation struct {
	Field      string
	FileID     string
	Attachment int
	ChunkIndex int
	ChunkCount int
}

// ProviderConfig 包含审核提供方所需的运行时参数。
type ProviderConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// ProviderImage 是提交给审核提供方的图片。
type ProviderImage struct {
	Data     []byte
	MimeType string
}

// CategoryResult 是与提供方无关的分类结果。
type CategoryResult struct {
	Flagged                   bool
	Categories                map[string]bool
	CategoryScores            map[string]float64
	CategoryAppliedInputTypes map[string][]string
}

// ProviderResponse 是与提供方无关的审核结果。
type ProviderResponse struct {
	ID      string
	Model   string
	Results []CategoryResult
}

// HitEvaluation 是针对单个提供方响应、结合策略得出的判定。
type HitEvaluation struct {
	Hit        bool
	Categories []string
	Scores     map[string]float64
}

// EvaluateHit 仅依据适用于预期模态的已选分类来决定是否拦截。
func EvaluateHit(response *ProviderResponse, selected []string, expectedModality string) HitEvaluation {
	evaluation := HitEvaluation{
		Categories: make([]string, 0),
		Scores:     make(map[string]float64),
	}
	if response == nil || len(selected) == 0 {
		return evaluation
	}
	selectedSet := make(map[string]struct{}, len(selected))
	for _, category := range selected {
		selectedSet[category] = struct{}{}
	}
	expected := "text"
	if strings.TrimSpace(expectedModality) == ModalityImage {
		expected = "image"
	}
	for _, item := range response.Results {
		for category := range selectedSet {
			if !item.Categories[category] || !categoryAppliesToModality(item.CategoryAppliedInputTypes, category, expected) {
				continue
			}
			if _, exists := evaluation.Scores[category]; exists {
				continue
			}
			evaluation.Categories = append(evaluation.Categories, category)
			if item.CategoryScores != nil {
				evaluation.Scores[category] = item.CategoryScores[category]
			}
		}
	}
	if len(evaluation.Categories) > 0 {
		sort.Strings(evaluation.Categories)
		evaluation.Hit = true
	}
	return evaluation
}

func categoryAppliesToModality(applied map[string][]string, category, expected string) bool {
	if applied == nil {
		return true
	}
	types, ok := applied[category]
	if !ok || len(types) == 0 {
		return true
	}
	for _, inputType := range types {
		if strings.EqualFold(strings.TrimSpace(inputType), expected) {
			return true
		}
	}
	return false
}
