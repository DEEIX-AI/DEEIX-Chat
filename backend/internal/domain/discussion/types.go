package discussion

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"

	MaxPlaceholders  = 12
	MaxRounds        = 10
	MaxStepsPerRound = 8
)

var (
	ErrTemplateNotFound = errors.New("discussion template not found")
	ErrTemplateInvalid  = errors.New("discussion template invalid")
)

// Template 是后台定义的讨论模型：前台把它当成一个可选模型，发送时由后端按模板编排。
type Template struct {
	ID          uint
	PublicID    string
	Name        string
	Description string
	Icon        string
	Status      string
	MaxRounds   int
	ConfigJSON  string
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Placeholder 是模板中的模型占位符，运行时由用户绑定真实平台模型。
type Placeholder struct {
	Key          string
	Label        string
	DefaultModel string
	Required     bool
}

// Step 是一轮中的一次模型发言。
type Step struct {
	ID          string
	Participant string
	Prompt      string
}

// Round 是一轮讨论；同轮步骤并行，轮次之间串行。
type Round struct {
	Index    int
	Parallel bool
	Steps    []Step
}

// Summary 是全部轮次结束后的总结节点。
type Summary struct {
	Participant string
	Prompt      string
}

// TemplateConfig 是模板的完整编排定义。
type TemplateConfig struct {
	Placeholders []Placeholder
	Rounds       []Round
	Summary      Summary
}

// RuntimeStep 是运行时解析后的一次发言。
type RuntimeStep struct {
	ID          string
	Participant string
	Model       string
	Prompt      string
}

// RuntimeRound 是运行时解析后的一轮。
type RuntimeRound struct {
	Index int
	Steps []RuntimeStep
}

// RuntimeSummary 是运行时解析后的总结节点。
type RuntimeSummary struct {
	Participant string
	Model       string
	Prompt      string
}

// RuntimePlan 是用户确认后的实际编排。
type RuntimePlan struct {
	Rounds  []RuntimeRound
	Summary RuntimeSummary
}

// Turn 是一次模型调用的结果。
type Turn struct {
	Round       int
	StepID      string
	Participant string
	Model       string
	Status      string
	Content     string
	Error       string
	// Prompt 是本次调用实际发送给模型的完整提示词，仅供调试查看。
	Prompt           string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
	LatencyMS        int64
}

// State 是持久化到 assistant 消息里的完整讨论记录。
type State struct {
	Enabled      bool
	Status       string
	TemplateID   string
	TemplateName string
	Placeholders []Placeholder
	Rounds       []RuntimeRound
	Summary      RuntimeSummary
	// SummaryPrompt 是总结调用实际发送的完整提示词，仅供调试查看。
	SummaryPrompt string
	Turns         []Turn
	FinalContent  string
	Error         string
	CreatedAt     int64
	UpdatedAt     int64
}

// ParseConfig 解析并校验模板配置。
func ParseConfig(raw string) (TemplateConfig, error) {
	var cfg TemplateConfig
	if strings.TrimSpace(raw) == "" {
		return cfg, fmt.Errorf("%w: config is empty", ErrTemplateInvalid)
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrTemplateInvalid, err)
	}
	normalizeConfig(&cfg)
	if err := ValidateConfig(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// ValidateConfig 校验模板结构：占位符唯一、轮次和步骤有界、引用存在。
func ValidateConfig(cfg *TemplateConfig) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrTemplateInvalid)
	}
	if len(cfg.Placeholders) == 0 {
		return fmt.Errorf("%w: at least one placeholder is required", ErrTemplateInvalid)
	}
	if len(cfg.Placeholders) > MaxPlaceholders {
		return fmt.Errorf("%w: too many placeholders", ErrTemplateInvalid)
	}
	keys := make(map[string]struct{}, len(cfg.Placeholders))
	for index := range cfg.Placeholders {
		item := &cfg.Placeholders[index]
		item.Key = strings.TrimSpace(item.Key)
		item.Label = strings.TrimSpace(item.Label)
		item.DefaultModel = strings.TrimSpace(item.DefaultModel)
		if item.Key == "" {
			return fmt.Errorf("%w: placeholder key is required", ErrTemplateInvalid)
		}
		if _, exists := keys[item.Key]; exists {
			return fmt.Errorf("%w: duplicate placeholder %s", ErrTemplateInvalid, item.Key)
		}
		keys[item.Key] = struct{}{}
		if item.Label == "" {
			item.Label = item.Key
		}
	}
	if len(cfg.Rounds) == 0 {
		return fmt.Errorf("%w: at least one round is required", ErrTemplateInvalid)
	}
	if len(cfg.Rounds) > MaxRounds {
		return fmt.Errorf("%w: too many rounds", ErrTemplateInvalid)
	}
	for index := range cfg.Rounds {
		round := &cfg.Rounds[index]
		round.Index = index + 1
		if len(round.Steps) == 0 {
			return fmt.Errorf("%w: round %d has no steps", ErrTemplateInvalid, round.Index)
		}
		if len(round.Steps) > MaxStepsPerRound {
			return fmt.Errorf("%w: round %d has too many steps", ErrTemplateInvalid, round.Index)
		}
		for stepIndex := range round.Steps {
			step := &round.Steps[stepIndex]
			step.Participant = strings.TrimSpace(step.Participant)
			step.Prompt = strings.TrimSpace(step.Prompt)
			if step.ID == "" {
				step.ID = fmt.Sprintf("r%ds%d", round.Index, stepIndex+1)
			}
			if _, ok := keys[step.Participant]; !ok {
				return fmt.Errorf("%w: round %d references unknown placeholder %s", ErrTemplateInvalid, round.Index, step.Participant)
			}
		}
	}
	cfg.Summary.Participant = strings.TrimSpace(cfg.Summary.Participant)
	cfg.Summary.Prompt = strings.TrimSpace(cfg.Summary.Prompt)
	if _, ok := keys[cfg.Summary.Participant]; !ok {
		return fmt.Errorf("%w: summary references unknown placeholder %s", ErrTemplateInvalid, cfg.Summary.Participant)
	}
	return nil
}

func normalizeConfig(cfg *TemplateConfig) {
	for index := range cfg.Rounds {
		round := &cfg.Rounds[index]
		round.Index = index + 1
		if len(round.Steps) > 0 {
			round.Parallel = true
		}
		for stepIndex := range round.Steps {
			if strings.TrimSpace(round.Steps[stepIndex].ID) == "" {
				round.Steps[stepIndex].ID = fmt.Sprintf("r%ds%d", round.Index, stepIndex+1)
			}
		}
	}
}

// ResolveRuntimePlan 把模板与用户运行时绑定合并成实际执行计划。
// overrides 只包含用户显式选择的模型；未选择时回落到模板默认模型。
func ResolveRuntimePlan(cfg TemplateConfig, overrides map[string]string) (RuntimePlan, error) {
	modelFor := func(participant string) string {
		if value := strings.TrimSpace(overrides[participant]); value != "" {
			return value
		}
		for _, item := range cfg.Placeholders {
			if item.Key == participant {
				return strings.TrimSpace(item.DefaultModel)
			}
		}
		return ""
	}
	plan := RuntimePlan{
		Rounds: make([]RuntimeRound, 0, len(cfg.Rounds)),
	}
	for _, round := range cfg.Rounds {
		runtimeRound := RuntimeRound{Index: round.Index, Steps: make([]RuntimeStep, 0, len(round.Steps))}
		for _, step := range round.Steps {
			model := modelFor(step.Participant)
			if model == "" {
				return RuntimePlan{}, fmt.Errorf("%w: model is not selected for %s", ErrTemplateInvalid, step.Participant)
			}
			runtimeRound.Steps = append(runtimeRound.Steps, RuntimeStep{
				ID:          step.ID,
				Participant: step.Participant,
				Model:       model,
				Prompt:      step.Prompt,
			})
		}
		plan.Rounds = append(plan.Rounds, runtimeRound)
	}
	summaryModel := modelFor(cfg.Summary.Participant)
	if summaryModel == "" {
		return RuntimePlan{}, fmt.Errorf("%w: summary model is not selected", ErrTemplateInvalid)
	}
	plan.Summary = RuntimeSummary{
		Participant: cfg.Summary.Participant,
		Model:       summaryModel,
		Prompt:      cfg.Summary.Prompt,
	}
	return plan, nil
}
