package model

import "time"

// Announcement 记录站点公告。
type Announcement struct {
	BaseModel
	Title           string     `gorm:"size:120;not null;default:'';comment:公告标题"`
	ContentMarkdown string     `gorm:"type:text;not null;default:'';comment:公告 Markdown 内容"`
	Status          string     `gorm:"size:32;not null;default:'active';index:idx_system_announcements_status;comment:状态(active/inactive)"`
	Type            string     `gorm:"size:32;not null;default:'general';index:idx_system_announcements_type;comment:公告类型(critical/warning/info/normal/general)"`
	Pinned          bool       `gorm:"not null;default:false;index:idx_system_announcements_pinned;comment:是否置顶"`
	Priority        int        `gorm:"not null;default:0;index:idx_system_announcements_priority;comment:排序优先级"`
	StartsAt        *time.Time `gorm:"index:idx_system_announcements_starts_at;comment:开始展示时间"`
	ExpiresAt       *time.Time `gorm:"index:idx_system_announcements_expires_at;comment:结束展示时间"`
	CreatedByUserID uint       `gorm:"not null;default:0;index:idx_system_announcements_created_by;comment:创建管理员ID"`
}

// TableName 指定表名。
func (Announcement) TableName() string {
	return "system_announcements"
}

// AnnouncementUserState 记录用户对公告版本的展示状态。
type AnnouncementUserState struct {
	BaseModel
	AnnouncementID        uint       `gorm:"not null;index:idx_announcement_user_states_announcement;uniqueIndex:idx_announcement_user_states_version;comment:公告ID"`
	UserID                uint       `gorm:"not null;index:idx_announcement_user_states_user;uniqueIndex:idx_announcement_user_states_version;comment:用户ID"`
	AnnouncementUpdatedAt time.Time  `gorm:"not null;uniqueIndex:idx_announcement_user_states_version;comment:公告版本更新时间"`
	DismissedUntil        *time.Time `gorm:"index:idx_announcement_user_states_dismissed_until;comment:暂不显示截止时间"`
	ClosedAt              *time.Time `gorm:"index:idx_announcement_user_states_closed_at;comment:关闭时间"`
}

// TableName 指定表名。
func (AnnouncementUserState) TableName() string {
	return "announcement_user_states"
}
