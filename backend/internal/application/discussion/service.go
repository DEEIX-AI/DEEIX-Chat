package discussion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// Service 管理讨论模板，并为会话侧解析运行时执行计划。
type Service struct {
	repo repository.DiscussionTemplateRepository
}

func NewService(repo repository.DiscussionTemplateRepository) *Service {
	return &Service{repo: repo}
}

type TemplateInput struct {
	Name        string
	Description string
	Icon        string
	Status      string
	MaxRounds   int
	ConfigJSON  string
	SortOrder   int
}

type TemplateView struct {
	ID          uint
	PublicID    string
	Name        string
	Description string
	Icon        string
	Status      string
	MaxRounds   int
	Config      domaindiscussion.TemplateConfig
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Service) ListTemplates(ctx context.Context, status string) ([]TemplateView, error) {
	items, err := s.repo.ListDiscussionTemplates(ctx, status)
	if err != nil {
		return nil, err
	}
	views := make([]TemplateView, 0, len(items))
	for _, item := range items {
		views = append(views, toView(item))
	}
	return views, nil
}

func (s *Service) ListActiveTemplates(ctx context.Context) ([]TemplateView, error) {
	return s.ListTemplates(ctx, domaindiscussion.StatusActive)
}

func (s *Service) GetTemplate(ctx context.Context, publicID string) (*TemplateView, error) {
	item, err := s.repo.GetDiscussionTemplateByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	view := toView(*item)
	return &view, nil
}

func (s *Service) CreateTemplate(ctx context.Context, input TemplateInput) (*TemplateView, error) {
	normalized, err := normalizeTemplateInput(input)
	if err != nil {
		return nil, err
	}
	item := &domaindiscussion.Template{
		PublicID:    newPublicID(),
		Name:        normalized.Name,
		Description: normalized.Description,
		Icon:        normalized.Icon,
		Status:      normalized.Status,
		MaxRounds:   normalized.MaxRounds,
		ConfigJSON:  normalized.ConfigJSON,
		SortOrder:   normalized.SortOrder,
	}
	if err = s.repo.CreateDiscussionTemplate(ctx, item); err != nil {
		return nil, err
	}
	view := toView(*item)
	return &view, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, publicID string, input TemplateInput) (*TemplateView, error) {
	item, err := s.repo.GetDiscussionTemplateByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeTemplateInput(input)
	if err != nil {
		return nil, err
	}
	item.Name = normalized.Name
	item.Description = normalized.Description
	item.Icon = normalized.Icon
	item.Status = normalized.Status
	item.MaxRounds = normalized.MaxRounds
	item.ConfigJSON = normalized.ConfigJSON
	item.SortOrder = normalized.SortOrder
	if err = s.repo.UpdateDiscussionTemplate(ctx, item); err != nil {
		return nil, err
	}
	view := toView(*item)
	return &view, nil
}

func (s *Service) DeleteTemplate(ctx context.Context, publicID string) error {
	item, err := s.repo.GetDiscussionTemplateByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	return s.repo.DeleteDiscussionTemplate(ctx, item.ID)
}

// ResolvePlan 供会话发送路径使用：读取启用模板并合并用户选择。
func (s *Service) ResolvePlan(ctx context.Context, publicID string, overrides map[string]string) (*domaindiscussion.Template, domaindiscussion.TemplateConfig, domaindiscussion.RuntimePlan, error) {
	item, err := s.repo.GetDiscussionTemplateByPublicID(ctx, publicID)
	if err != nil {
		return nil, domaindiscussion.TemplateConfig{}, domaindiscussion.RuntimePlan{}, err
	}
	if item.Status != domaindiscussion.StatusActive {
		return nil, domaindiscussion.TemplateConfig{}, domaindiscussion.RuntimePlan{}, domaindiscussion.ErrTemplateNotFound
	}
	cfg, err := domaindiscussion.ParseConfig(item.ConfigJSON)
	if err != nil {
		return nil, domaindiscussion.TemplateConfig{}, domaindiscussion.RuntimePlan{}, err
	}
	plan, err := domaindiscussion.ResolveRuntimePlan(cfg, overrides)
	if err != nil {
		return nil, domaindiscussion.TemplateConfig{}, domaindiscussion.RuntimePlan{}, err
	}
	return item, cfg, plan, nil
}

func normalizeTemplateInput(input TemplateInput) (TemplateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Icon = strings.TrimSpace(input.Icon)
	input.Status = strings.TrimSpace(strings.ToLower(input.Status))
	if input.Name == "" {
		return input, fmt.Errorf("%w: name is required", domaindiscussion.ErrTemplateInvalid)
	}
	if input.Status == "" {
		input.Status = domaindiscussion.StatusActive
	}
	if input.Status != domaindiscussion.StatusActive && input.Status != domaindiscussion.StatusInactive {
		return input, fmt.Errorf("%w: invalid status", domaindiscussion.ErrTemplateInvalid)
	}
	cfg, err := domaindiscussion.ParseConfig(input.ConfigJSON)
	if err != nil {
		return input, err
	}
	if input.MaxRounds <= 0 {
		input.MaxRounds = len(cfg.Rounds)
	}
	if input.MaxRounds > domaindiscussion.MaxRounds {
		return input, fmt.Errorf("%w: max rounds exceeds %d", domaindiscussion.ErrTemplateInvalid, domaindiscussion.MaxRounds)
	}
	if len(cfg.Rounds) > input.MaxRounds {
		return input, fmt.Errorf("%w: template has more rounds than max rounds", domaindiscussion.ErrTemplateInvalid)
	}
	return input, nil
}

func toView(item domaindiscussion.Template) TemplateView {
	cfg, _ := domaindiscussion.ParseConfig(item.ConfigJSON)
	return TemplateView{
		ID:          item.ID,
		PublicID:    item.PublicID,
		Name:        item.Name,
		Description: item.Description,
		Icon:        item.Icon,
		Status:      item.Status,
		MaxRounds:   item.MaxRounds,
		Config:      cfg,
		SortOrder:   item.SortOrder,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func newPublicID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("dsc_%d", time.Now().UnixNano())
	}
	return "dsc_" + hex.EncodeToString(buf)
}
