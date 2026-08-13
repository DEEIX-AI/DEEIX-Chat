package model

import "time"

// ContentModerationEvent stores hit and failed-open moderation records only.
type ContentModerationEvent struct {
	BaseModel
	PublicID            string    `gorm:"size:40;not null;default:'';uniqueIndex:idx_content_moderation_events_public_id;comment:公开事件编号"`
	UserID              uint      `gorm:"not null;default:0;index:idx_content_moderation_events_user_id;comment:用户ID"`
	ConversationID      uint      `gorm:"not null;default:0;index:idx_content_moderation_events_conversation_id;comment:会话ID"`
	RunID               string    `gorm:"size:64;not null;default:'';index:idx_content_moderation_events_run_id;comment:运行ID"`
	MessageID           uint      `gorm:"not null;default:0;index:idx_content_moderation_events_message_id;comment:消息ID"`
	MessagePublicID     string    `gorm:"size:32;not null;default:'';index:idx_content_moderation_events_message_public_id;comment:消息公开ID"`
	Direction           string    `gorm:"size:16;not null;default:'';index:idx_content_moderation_events_direction;comment:方向(input/output)"`
	Modality            string    `gorm:"size:16;not null;default:'';index:idx_content_moderation_events_modality;comment:模态(text/image)"`
	Model               string    `gorm:"size:128;not null;default:'';comment:审核模型"`
	PolicyVersion       int64     `gorm:"not null;default:0;comment:策略版本"`
	Result              string    `gorm:"size:32;not null;default:'';index:idx_content_moderation_events_result;comment:结果(passed/hit/failed_open)"`
	CategoriesJSON      string    `gorm:"type:text;not null;default:'[]';comment:命中分类JSON"`
	CategoryScoresJSON  string    `gorm:"type:text;not null;default:'{}';comment:分类分数JSON"`
	LatencyMS           int64     `gorm:"not null;default:0;comment:审核延迟毫秒"`
	ErrorCode           string    `gorm:"size:64;not null;default:'';index:idx_content_moderation_events_error_code;comment:错误码"`
	ErrorMessage        string    `gorm:"size:255;not null;default:'';comment:错误信息"`
	ContentLocationJSON string    `gorm:"type:text;not null;default:'{}';comment:内容位置JSON"`
	ContentSummary      string    `gorm:"size:255;not null;default:'';comment:内容摘要"`
	EncryptedText       string    `gorm:"type:text;not null;default:'';comment:命中文本AES-GCM密文"`
	ImageCount          int       `gorm:"not null;default:0;comment:隔离图片数量"`
	ImageMetaJSON       string    `gorm:"type:text;not null;default:'[]';comment:隔离图片元数据JSON"`
	ContentExpiresAt    time.Time `gorm:"not null;index:idx_content_moderation_events_content_expires_at;comment:原文密文过期时间"`
	MetadataExpiresAt   time.Time `gorm:"not null;index:idx_content_moderation_events_metadata_expires_at;comment:元数据过期时间"`
}

// TableName 指定表名。
func (ContentModerationEvent) TableName() string {
	return "content_moderation_events"
}

// ContentModerationDailyStat stores anonymous daily aggregates.
type ContentModerationDailyStat struct {
	BaseModel
	StatDate     time.Time `gorm:"type:date;not null;uniqueIndex:uk_content_moderation_daily_stats,priority:1;comment:统计日期"`
	Direction    string    `gorm:"size:16;not null;default:'';uniqueIndex:uk_content_moderation_daily_stats,priority:2;comment:方向"`
	Modality     string    `gorm:"size:16;not null;default:'';uniqueIndex:uk_content_moderation_daily_stats,priority:3;comment:模态"`
	Result       string    `gorm:"size:32;not null;default:'';uniqueIndex:uk_content_moderation_daily_stats,priority:4;comment:结果"`
	Category     string    `gorm:"size:64;not null;default:'';uniqueIndex:uk_content_moderation_daily_stats,priority:5;comment:分类(空表示汇总)"`
	CheckCount   int64     `gorm:"not null;default:0;comment:检查次数"`
	ContentItems int64     `gorm:"not null;default:0;comment:内容项数"`
	HitCount     int64     `gorm:"not null;default:0;comment:命中次数"`
	FailureCount int64     `gorm:"not null;default:0;comment:失败开放次数"`
	LatencySumMS int64     `gorm:"not null;default:0;comment:延迟合计毫秒"`
	LatencyCount int64     `gorm:"not null;default:0;comment:延迟样本数"`
}

// TableName 指定表名。
func (ContentModerationDailyStat) TableName() string {
	return "content_moderation_daily_stats"
}
