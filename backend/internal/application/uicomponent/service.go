package uicomponent

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
)

const (
	maxNameLength          = 64
	maxDescriptionLength   = 256
	maxPropsSummaryLength  = 1024
	maxPropsSchemaBytes    = 16 * 1024
	maxRendererSourceBytes = 256 * 1024
	maxCatalogComponents   = 32
)

// 名称同时是提示词里的标记和前端分发键，限制为 kebab-case 标识符。
var componentNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Service 封装交互式组件业务逻辑。
type Service struct {
	repo        repository.UIComponentRepository
	auditWriter auditWriter
	// featureEnabled 读取全局开关 chat.ui_components_enabled；为 nil 时视为开启。
	featureEnabled func() bool
}

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// NewService 创建组件服务。
func NewService(repo repository.UIComponentRepository) *Service {
	return &Service{repo: repo}
}

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// SetFeatureEnabled 注入全局开关读取函数。关闭时用户侧目录为空、会话解析不到任何组件，
// 客户端因此不再显示选择器也不渲染组件块；管理端列表不受影响，便于关着的时候仍能维护。
func (s *Service) SetFeatureEnabled(fn func() bool) {
	s.featureEnabled = fn
}

func (s *Service) enabled() bool {
	return s.featureEnabled == nil || s.featureEnabled()
}

// AuditInput 描述组件审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     any
}

// RecordAudit 记录组件审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.UserID,
		Action:      input.Action,
		Resource:    "ui_components",
		ResourceID:  input.ResourceID,
		IP:          input.ClientIP,
		UserAgent:   input.UserAgent,
		Detail:      input.Detail,
	})
}

// ListInput 定义组件列表入参。
type ListInput struct {
	IDs      []uint
	Query    string
	Scope    string
	Enabled  *bool
	Page     int
	PageSize int
}

// WriteInput 定义组件创建入参。
type WriteInput struct {
	Name           string
	Version        int
	Description    string
	PropsSummary   string
	PropsSchema    string
	RendererSource string
	Enabled        bool
	SortOrder      int
}

// PatchInput 定义组件更新入参。
type PatchInput struct {
	Name           *string
	Version        *int
	Description    *string
	PropsSummary   *string
	PropsSchema    *string
	RendererSource *string
	Enabled        *bool
	SortOrder      *int
}

// ListVisible 查询当前用户可使用的组件（已启用的内置、平台组件与本人组件）。
func (s *Service) ListVisible(ctx context.Context, userID uint, input ListInput) ([]domainuicomponent.Component, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	if !s.enabled() {
		return []domainuicomponent.Component{}, 0, nil
	}
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListUIComponents(ctx, repository.UIComponentListFilter{
		IDs:           input.IDs,
		Query:         strings.TrimSpace(input.Query),
		VisibleUserID: &userID,
	}, offset, limit)
}

// ListMine 查询当前用户自定义组件。
func (s *Service) ListMine(ctx context.Context, userID uint, input ListInput) ([]domainuicomponent.Component, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListUIComponents(ctx, repository.UIComponentListFilter{
		Query:       strings.TrimSpace(input.Query),
		Scope:       domainuicomponent.ScopeUser,
		OwnerUserID: &userID,
		Enabled:     input.Enabled,
	}, offset, limit)
}

// ListAdmin 查询管理员视角的内置与平台组件。
func (s *Service) ListAdmin(ctx context.Context, input ListInput) ([]domainuicomponent.Component, int64, error) {
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	scope := strings.TrimSpace(input.Scope)
	if scope != "" && scope != domainuicomponent.ScopeBuiltin && scope != domainuicomponent.ScopePlatform {
		return nil, 0, ErrInvalidComponent
	}
	filter := repository.UIComponentListFilter{
		Query:   strings.TrimSpace(input.Query),
		Scope:   scope,
		Enabled: input.Enabled,
	}
	if scope == "" {
		zero := uint(0)
		filter.OwnerUserID = &zero
	}
	return s.repo.ListUIComponents(ctx, filter, offset, limit)
}

// ResolveVisible 返回当前用户可用且已启用的组件，按传入 ID 顺序去重；不可见的 ID 被忽略。
func (s *Service) ResolveVisible(ctx context.Context, userID uint, ids []uint) ([]domainuicomponent.Component, error) {
	if userID == 0 || len(ids) == 0 || !s.enabled() {
		return nil, nil
	}
	unique := uniqueIDs(ids)
	items, _, err := s.repo.ListUIComponents(ctx, repository.UIComponentListFilter{IDs: unique, VisibleUserID: &userID}, 0, len(unique))
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	byID := make(map[uint]domainuicomponent.Component, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ordered := make([]domainuicomponent.Component, 0, len(unique))
	for _, id := range unique {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
}

// CatalogPrompt 生成给模型的组件目录说明。每个组件一行，渲染实现不进入提示词。
// catalogPromptHeader 是目录层的固定部分。结构化标签与视觉排版层保持同一风格；
// 最先说明组件是回复正文里的 Markdown 代码块、不是工具，避免模型把目录当成函数签名去调用。
const catalogPromptHeader = `<nature>交互式组件是回复正文里的一段 Markdown 代码块，由客户端渲染成界面。它是输出格式，不是工具：不要通过 function calling / tool call 调用组件，也不要等待它返回结果。</nature>
<when>只在组件明显比纯 Markdown 更清楚、更好用时使用（需要筛选、排序、计算、绘图、排期、逐步引导、对比、判分）；否则用 Markdown。数量由内容决定，可以为零。</when>
<syntax>语言标识为 ` + "`" + domainuicomponent.FenceLanguage + "`" + ` 的代码块，内容是合法 JSON，只有 component、id（简短英文、对话内唯一）、props 三个字段；字符串内的英文双引号写成 \"，不要尾随逗号、注释或目录外的字段。代码块放在正文中它所属的位置，前后用自然语言承接，不要在组件外重复组件里的数据。
` + "```" + domainuicomponent.FenceLanguage + `
{"component": "stat-grid", "id": "kpi-week", "props": {"title": "本周指标", "items": [{"label": "日活", "value": "128,430", "delta": "+4.2%", "trend": "up"}]}}
` + "```" + `</syntax>
<catalog>`

// 内置描述形如「定位。适用：…。交互：…」；交互说明是给选择器看的，提示词里只保留定位与适用场景。
const (
	propsRulesSeparator     = "。约定："
	descriptionInteractions = "。交互："
)

const catalogPromptFooter = `
</catalog>`

// CatalogPrompt 生成注入系统提示词的组件目录层。
func CatalogPrompt(components []domainuicomponent.Component) string {
	if len(components) == 0 {
		return ""
	}
	if len(components) > maxCatalogComponents {
		components = components[:maxCatalogComponents]
	}
	var builder strings.Builder
	builder.WriteString(catalogPromptHeader)
	for _, component := range components {
		builder.WriteString("\n  <component name=\"")
		builder.WriteString(component.Name)
		builder.WriteString("\"")
		if component.Version > 1 {
			builder.WriteString(" version=\"")
			builder.WriteString(strconv.Itoa(component.Version))
			builder.WriteString("\"")
		}
		builder.WriteString(">\n    <use>")
		use, _, _ := strings.Cut(component.Description, descriptionInteractions)
		builder.WriteString(use)
		builder.WriteString("</use>\n    <props>")
		props, rules, hasRules := strings.Cut(component.PropsSummary, propsRulesSeparator)
		builder.WriteString(props)
		builder.WriteString("</props>")
		if hasRules {
			builder.WriteString("\n    <rules>")
			builder.WriteString(rules)
			builder.WriteString("</rules>")
		}
		builder.WriteString("\n  </component>")
	}
	builder.WriteString(catalogPromptFooter)
	return builder.String()
}

// CreateUser 创建用户自定义组件。
func (s *Service) CreateUser(ctx context.Context, userID uint, input WriteInput) (*domainuicomponent.Component, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainuicomponent.ScopeUser, userID, userID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// CreatePlatform 创建管理员平台组件。
func (s *Service) CreatePlatform(ctx context.Context, actorUserID uint, input WriteInput) (*domainuicomponent.Component, error) {
	if actorUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainuicomponent.ScopePlatform, 0, actorUserID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// UpdateUser 更新当前用户自定义组件。
func (s *Service) UpdateUser(ctx context.Context, userID uint, id uint, input PatchInput) (*domainuicomponent.Component, error) {
	if userID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if item.Scope != domainuicomponent.ScopeUser || item.OwnerUserID != userID {
		return nil, ErrComponentNotFound
	}
	return s.update(ctx, item, userID, input)
}

// UpdateAdmin 更新内置或平台组件。内置组件只允许修改启用状态、描述与排序。
func (s *Service) UpdateAdmin(ctx context.Context, actorUserID uint, id uint, input PatchInput) (*domainuicomponent.Component, error) {
	if actorUserID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	switch item.Scope {
	case domainuicomponent.ScopePlatform:
		return s.update(ctx, item, actorUserID, input)
	case domainuicomponent.ScopeBuiltin:
		// 内置组件的目录内容由代码定义并在启动时同步，管理员只能启停。
		if input.Name != nil || input.Version != nil || input.Description != nil || input.PropsSummary != nil || input.PropsSchema != nil || input.RendererSource != nil {
			return nil, ErrBuiltinProtected
		}
		return s.update(ctx, item, actorUserID, input)
	default:
		return nil, ErrComponentNotFound
	}
}

// DeleteUser 删除当前用户自定义组件。
func (s *Service) DeleteUser(ctx context.Context, userID uint, id uint) error {
	if userID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if item.Scope != domainuicomponent.ScopeUser || item.OwnerUserID != userID {
		return ErrComponentNotFound
	}
	return mapRepositoryError(s.repo.DeleteUIComponent(ctx, id))
}

// DeleteAdmin 删除平台组件。内置组件受保护。
func (s *Service) DeleteAdmin(ctx context.Context, actorUserID uint, id uint) error {
	if actorUserID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	switch item.Scope {
	case domainuicomponent.ScopeBuiltin:
		return ErrBuiltinProtected
	case domainuicomponent.ScopePlatform:
		return mapRepositoryError(s.repo.DeleteUIComponent(ctx, id))
	default:
		return ErrComponentNotFound
	}
}

func (s *Service) create(ctx context.Context, item *domainuicomponent.Component) (*domainuicomponent.Component, error) {
	result, err := s.repo.CreateUIComponent(ctx, item)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return result, nil
}

func (s *Service) update(ctx context.Context, current *domainuicomponent.Component, actorUserID uint, input PatchInput) (*domainuicomponent.Component, error) {
	patch, err := normalizePatchInput(input, current, actorUserID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.PatchUIComponent(ctx, current.ID, patch)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func normalizeWriteInput(input WriteInput, scope string, ownerUserID uint, actorUserID uint) (*domainuicomponent.Component, error) {
	name := strings.TrimSpace(input.Name)
	if !validName(name) {
		return nil, ErrInvalidComponent
	}
	version := input.Version
	if version <= 0 {
		version = 1
	}
	description := strings.TrimSpace(input.Description)
	summary := strings.TrimSpace(input.PropsSummary)
	schema := strings.TrimSpace(input.PropsSchema)
	source := input.RendererSource
	if description == "" || runeCount(description) > maxDescriptionLength || summary == "" || runeCount(summary) > maxPropsSummaryLength {
		return nil, ErrInvalidComponent
	}
	if err := validSchema(schema); err != nil {
		return nil, err
	}
	if strings.TrimSpace(source) == "" || len(source) > maxRendererSourceBytes {
		return nil, ErrInvalidComponent
	}
	return &domainuicomponent.Component{
		Scope:           scope,
		OwnerUserID:     ownerUserID,
		Name:            name,
		Version:         version,
		Description:     description,
		PropsSummary:    summary,
		PropsSchema:     schema,
		RendererKind:    domainuicomponent.RendererSandbox,
		RendererSource:  source,
		Enabled:         input.Enabled,
		SortOrder:       input.SortOrder,
		CreatedByUserID: actorUserID,
		UpdatedByUserID: actorUserID,
	}, nil
}

func normalizePatchInput(input PatchInput, current *domainuicomponent.Component, actorUserID uint) (repository.UIComponentPatch, error) {
	patch := repository.UIComponentPatch{UpdatedByUserIDSet: true, UpdatedByUserID: actorUserID}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if !validName(name) {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.Name = &name
	}
	if input.Version != nil {
		if *input.Version <= 0 {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.Version = input.Version
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if description == "" || runeCount(description) > maxDescriptionLength {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.Description = &description
	}
	if input.PropsSummary != nil {
		summary := strings.TrimSpace(*input.PropsSummary)
		if summary == "" || runeCount(summary) > maxPropsSummaryLength {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.PropsSummary = &summary
	}
	if input.PropsSchema != nil {
		schema := strings.TrimSpace(*input.PropsSchema)
		if err := validSchema(schema); err != nil {
			return repository.UIComponentPatch{}, err
		}
		patch.PropsSchema = &schema
	}
	if input.RendererSource != nil {
		if current.RendererKind != domainuicomponent.RendererSandbox {
			return repository.UIComponentPatch{}, ErrBuiltinProtected
		}
		if strings.TrimSpace(*input.RendererSource) == "" || len(*input.RendererSource) > maxRendererSourceBytes {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.RendererSource = input.RendererSource
	}
	patch.Enabled = input.Enabled
	patch.SortOrder = input.SortOrder
	return patch, nil
}

func validName(name string) bool {
	return name != "" && runeCount(name) <= maxNameLength && componentNamePattern.MatchString(name)
}

// validSchema 只要求是合法 JSON 对象且不超长；结构校验在前端按 JSON Schema 执行。
func validSchema(schema string) error {
	if schema == "" {
		return nil
	}
	if len(schema) > maxPropsSchemaBytes {
		return ErrInvalidComponent
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(schema), &payload); err != nil {
		return ErrInvalidComponent
	}
	return nil
}

func uniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func runeCount(value string) int {
	return len([]rune(value))
}

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ErrComponentNotFound
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return ErrComponentConflict
	}
	if errors.Is(err, repository.ErrInvalidInput) {
		return ErrInvalidComponent
	}
	return err
}
