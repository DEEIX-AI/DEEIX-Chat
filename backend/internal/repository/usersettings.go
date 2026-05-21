package repository

import (
	"context"

	domainusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/usersettings"
)

// UserSettingsRepository 定义用户个人配置读写能力。
type UserSettingsRepository interface {
	ListByUserID(ctx context.Context, userID uint) ([]domainusersettings.UserSetting, error)
	Upsert(ctx context.Context, items []domainusersettings.UserSetting) error
}
