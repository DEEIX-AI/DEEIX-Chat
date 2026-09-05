package announcement

import (
	"strings"
	"time"
)

const (
	// StatusActive 表示公告启用。
	StatusActive = "active"
	// StatusInactive 表示公告停用。
	StatusInactive = "inactive"

	// TypeCritical 表示紧急公告。
	TypeCritical = "critical"
	// TypeWarning 表示警告公告。
	TypeWarning = "warning"
	// TypeInfo 表示提示公告。
	TypeInfo = "info"
	// TypeNormal 表示普通公告。
	TypeNormal = "normal"
	// TypeGeneral 表示常规公告。
	TypeGeneral = "general"
)

// Announcement 表示一条站点公告。
type Announcement struct {
	ID              uint
	Title           string
	ContentMarkdown string
	Status          string
	Type            string
	Pinned          bool
	Priority        int
	StartsAt        *time.Time
	ExpiresAt       *time.Time
	CreatedByUserID uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ClosedAt        *time.Time
}

// UserState 记录用户对公告版本的展示偏好。
type UserState struct {
	ID                    uint
	AnnouncementID        uint
	UserID                uint
	AnnouncementUpdatedAt time.Time
	DismissedUntil        *time.Time
	ClosedAt              *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// NormalizeStatus 将公告状态规范化为持久化使用的领域值。
func NormalizeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "", StatusActive:
		return StatusActive
	case StatusInactive:
		return StatusInactive
	default:
		return ""
	}
}

// NormalizeType 将公告类型规范化为持久化使用的领域值。
func NormalizeType(value string) string {
	switch strings.TrimSpace(value) {
	case "", TypeGeneral:
		return TypeGeneral
	case TypeCritical:
		return TypeCritical
	case TypeWarning:
		return TypeWarning
	case TypeInfo:
		return TypeInfo
	case TypeNormal:
		return TypeNormal
	default:
		return ""
	}
}
