package discussion

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrSnapshotConflict   = errors.New("discussion snapshot version conflict")
	ErrPendingNotFound    = errors.New("discussion pending snapshot not found")
	ErrNoEnabledModels    = errors.New("discussion has no enabled models")
	ErrSummaryModelLocked = errors.New("discussion summary model cannot be disabled")
)

// Snapshot 包含一次成功讨论所需的完整模板与绑定，不再依赖后台模板的后续状态。
type Snapshot struct {
	Template Template
	Config   TemplateConfig
	Plan     RuntimePlan
}

// ConversationSnapshot 按会话和模板分别保存已生效配置与一级待验证配置。
type ConversationSnapshot struct {
	ConversationID   uint
	TemplatePublicID string
	Active           *Snapshot
	ActiveVersion    int64
	Pending          *Snapshot
	PendingVersion   int64
	PendingCreatedAt *time.Time
	UpdatedAt        time.Time
}

// PlanBindings 提取完整计划中的占位符绑定；过滤临时关闭模型之前调用。
func PlanBindings(plan RuntimePlan) map[string]string {
	bindings := make(map[string]string)
	for _, round := range plan.Rounds {
		for _, step := range round.Steps {
			if step.Participant != "" && strings.TrimSpace(step.Model) != "" {
				bindings[step.Participant] = strings.TrimSpace(step.Model)
			}
		}
	}
	if plan.Summary.Participant != "" && strings.TrimSpace(plan.Summary.Model) != "" {
		bindings[plan.Summary.Participant] = strings.TrimSpace(plan.Summary.Model)
	}
	return bindings
}

// MergeBindings 以当前快照绑定为基底，仅覆盖本次明确修改的占位符。
func MergeBindings(plan RuntimePlan, overrides map[string]string) map[string]string {
	bindings := PlanBindings(plan)
	for participant, model := range overrides {
		if strings.TrimSpace(model) != "" {
			bindings[participant] = strings.TrimSpace(model)
		}
	}
	return bindings
}

// FilterDisabledModels 只过滤本次执行，不修改持久化快照；空轮次保留原编号后整体跳过。
func FilterDisabledModels(plan RuntimePlan, disabled []string) (RuntimePlan, error) {
	blocked := make(map[string]struct{}, len(disabled))
	for _, participant := range disabled {
		participant = strings.TrimSpace(participant)
		if participant != "" {
			blocked[participant] = struct{}{}
		}
	}
	if _, ok := blocked[plan.Summary.Participant]; ok {
		return RuntimePlan{}, ErrSummaryModelLocked
	}
	filtered := RuntimePlan{Summary: plan.Summary, Rounds: make([]RuntimeRound, 0, len(plan.Rounds))}
	for _, round := range plan.Rounds {
		next := RuntimeRound{Index: round.Index, Parallel: round.Parallel, Steps: make([]RuntimeStep, 0, len(round.Steps))}
		for _, step := range round.Steps {
			if _, ok := blocked[step.Participant]; !ok {
				next.Steps = append(next.Steps, step)
			}
		}
		if len(next.Steps) > 0 {
			filtered.Rounds = append(filtered.Rounds, next)
		}
	}
	if len(filtered.Rounds) == 0 {
		return RuntimePlan{}, ErrNoEnabledModels
	}
	return filtered, nil
}

// SummaryRoundIndex 在跳过空轮后仍保证总结节点排在所有原始轮次之后。
func SummaryRoundIndex(plan RuntimePlan) int {
	index := 1
	for _, round := range plan.Rounds {
		if round.Index >= index {
			index = round.Index + 1
		}
	}
	return index
}
