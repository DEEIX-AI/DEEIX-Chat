package billing

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// 时段倍率（峰谷计费）：在模型基础单价之上按服务器本地时间叠加倍率百分比，与计费模式正交，
// token / call / duration / tiered 都适用。同一模型的时段不允许重叠，因此任一时刻最多命中一条。

const (
	maxSchedulePeriods       = 32
	maxSchedulePeriodName    = 32
	minScheduleRatePercent   = 1
	maxScheduleRatePercent   = 10000
	minutesPerDay            = 24 * 60
	schedulePeriodTimeLayout = "15:04"
)

type schedulePricingConfig struct {
	Periods []schedulePeriod `json:"periods"`
}

// schedulePeriod 是一条时段规则。Start/End 为 "HH:MM"，End 不大于 Start 表示跨过午夜；
// Weekdays 为 0（周日）到 6（周六），按 Start 所在的自然日判断。
type schedulePeriod struct {
	Name        string `json:"name"`
	Weekdays    []int  `json:"weekdays"`
	Start       string `json:"start"`
	End         string `json:"end"`
	RatePercent int    `json:"ratePercent"`

	startMinute int
	endMinute   int
}

// ResolvedSchedulePeriod 是某个时刻命中的时段，供账本快照与用户展示。
type ResolvedSchedulePeriod struct {
	Name        string
	RatePercent int
}

func parseClockMinute(value string) (int, error) {
	parsed, err := time.Parse(schedulePeriodTimeLayout, strings.TrimSpace(value))
	if err != nil {
		return 0, repository.ErrInvalidInput
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

// parseSchedulePeriods 解析并校验时段配置；空配置表示未启用。
func parseSchedulePeriods(raw string) ([]schedulePeriod, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil, nil
	}
	var config schedulePricingConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, repository.ErrInvalidInput
	}
	if len(config.Periods) == 0 {
		return nil, nil
	}
	if len(config.Periods) > maxSchedulePeriods {
		return nil, repository.ErrInvalidInput
	}
	for index := range config.Periods {
		period := &config.Periods[index]
		period.Name = strings.TrimSpace(period.Name)
		if period.Name == "" || len([]rune(period.Name)) > maxSchedulePeriodName {
			return nil, repository.ErrInvalidInput
		}
		if period.RatePercent < minScheduleRatePercent || period.RatePercent > maxScheduleRatePercent {
			return nil, repository.ErrInvalidInput
		}
		if len(period.Weekdays) == 0 {
			return nil, repository.ErrInvalidInput
		}
		seen := make(map[int]struct{}, len(period.Weekdays))
		for _, weekday := range period.Weekdays {
			if weekday < 0 || weekday > 6 {
				return nil, repository.ErrInvalidInput
			}
			if _, duplicate := seen[weekday]; duplicate {
				return nil, repository.ErrInvalidInput
			}
			seen[weekday] = struct{}{}
		}
		sort.Ints(period.Weekdays)
		start, err := parseClockMinute(period.Start)
		if err != nil {
			return nil, err
		}
		end, err := parseClockMinute(period.End)
		if err != nil {
			return nil, err
		}
		if start == end {
			return nil, repository.ErrInvalidInput
		}
		period.Start = fmt.Sprintf("%02d:%02d", start/60, start%60)
		period.End = fmt.Sprintf("%02d:%02d", end/60, end%60)
		period.startMinute = start
		period.endMinute = end
	}
	if schedulePeriodsOverlap(config.Periods) {
		return nil, repository.ErrInvalidInput
	}
	return config.Periods, nil
}

// weekMinuteSpans 把一条时段展开为一周内的分钟区间 [from, to)，跨午夜的拆成两段，周日晚跨到周一。
func weekMinuteSpans(period schedulePeriod) [][2]int {
	spans := make([][2]int, 0, len(period.Weekdays)*2)
	for _, weekday := range period.Weekdays {
		base := weekday * minutesPerDay
		if period.startMinute < period.endMinute {
			spans = append(spans, [2]int{base + period.startMinute, base + period.endMinute})
			continue
		}
		spans = append(spans, [2]int{base + period.startMinute, base + minutesPerDay})
		next := ((weekday + 1) % 7) * minutesPerDay
		spans = append(spans, [2]int{next, next + period.endMinute})
	}
	return spans
}

func schedulePeriodsOverlap(periods []schedulePeriod) bool {
	type span struct {
		from, to int
	}
	all := make([]span, 0, len(periods)*4)
	for _, period := range periods {
		for _, item := range weekMinuteSpans(period) {
			if item[0] < item[1] {
				all = append(all, span{from: item[0], to: item[1]})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].from < all[j].from })
	for index := 1; index < len(all); index++ {
		if all[index].from < all[index-1].to {
			return true
		}
	}
	return false
}

// normalizeSchedulePricingJSON 空配置落库为 "{}"，与列默认值一致。
func normalizeSchedulePricingJSON(raw string) (string, error) {
	periods, err := parseSchedulePeriods(raw)
	if err != nil {
		return "", err
	}
	if len(periods) == 0 {
		return "{}", nil
	}
	payload, err := json.Marshal(schedulePricingConfig{Periods: periods})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// resolveSchedulePeriod 返回 at（按服务器本地时间）命中的时段；未命中返回 nil。
func resolveSchedulePeriod(periods []schedulePeriod, at time.Time) *ResolvedSchedulePeriod {
	if len(periods) == 0 {
		return nil
	}
	local := at.Local()
	weekMinute := int(local.Weekday())*minutesPerDay + local.Hour()*60 + local.Minute()
	for _, period := range periods {
		for _, span := range weekMinuteSpans(period) {
			if weekMinute >= span[0] && weekMinute < span[1] {
				return &ResolvedSchedulePeriod{Name: period.Name, RatePercent: period.RatePercent}
			}
		}
	}
	return nil
}

// resolveSchedulePeriodJSON 是账本与预估共用的入口：解析失败时视为无时段，不阻断计费。
func resolveSchedulePeriodJSON(raw string, at time.Time) *ResolvedSchedulePeriod {
	periods, err := parseSchedulePeriods(raw)
	if err != nil {
		return nil
	}
	return resolveSchedulePeriod(periods, at)
}
