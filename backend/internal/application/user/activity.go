package user

import (
	"context"
	"errors"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

const (
	// DefaultActivityDays 是未指定天数时的活跃度统计窗口。
	DefaultActivityDays = 365
	// MaxActivityDays 是活跃度统计窗口上限。
	MaxActivityDays = 366
)

// ErrActivityStatsUnavailable 表示活跃度统计仓储未接入。
var ErrActivityStatsUnavailable = errors.New("activity stats repository unavailable")

// activityStatsRepository 封装活跃度统计所需的按日聚合查询。
type activityStatsRepository interface {
	GetDailyActivityByUser(ctx context.Context, userID uint, startDate time.Time, endDate time.Time) ([]domainuser.DailyActivity, error)
}

// SetActivityStatsRepository 注入活跃度统计仓储。
func (s *Service) SetActivityStatsRepository(repo activityStatsRepository) {
	s.activityStatsRepo = repo
}

// GetDailyActivity 查询用户近 N 天真实模型调用活跃度，逐日补零返回。
func (s *Service) GetDailyActivity(ctx context.Context, userID uint, days int, now time.Time) ([]domainuser.DailyActivity, error) {
	if s.activityStatsRepo == nil {
		return nil, ErrActivityStatsUnavailable
	}
	if days <= 0 {
		days = DefaultActivityDays
	}
	if days > MaxActivityDays {
		days = MaxActivityDays
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startDate := today.AddDate(0, 0, -(days - 1))
	endDate := today.AddDate(0, 0, 1)

	rows, err := s.activityStatsRepo.GetDailyActivityByUser(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]domainuser.DailyActivity, len(rows))
	for _, row := range rows {
		byDate[row.Date] = row
	}
	// days 已钳制在 MaxActivityDays 内，容量直接用常量上限：窗口最多 366 条，
	// 也避免静态分析对"用户输入派生容量"的分配告警。
	results := make([]domainuser.DailyActivity, 0, MaxActivityDays)
	for day := startDate; day.Before(endDate); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if item, ok := byDate[key]; ok {
			results = append(results, item)
			continue
		}
		results = append(results, domainuser.DailyActivity{Date: key})
	}
	return results, nil
}
