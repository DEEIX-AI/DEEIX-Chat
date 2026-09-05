package conversation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/skill"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	conversationProjectNameMaxChars         = 80
	conversationProjectDescriptionMaxChars  = 255
	conversationProjectSystemPromptMaxChars = 12000
	conversationProjectModelMaxChars        = 128
	conversationProjectMetaMaxChars         = 32
)

// ConversationProjectInput 定义新建项目分组输入。
type ConversationProjectInput struct {
	Name                    string
	Description             string
	SystemPrompt            string
	DefaultModel            string
	MCPDefaultMode          string
	DefaultMCPToolIDs       []uint
	DefaultSkillIDs         []uint
	DefaultKnowledgeBaseIDs []string
	Color                   string
	Icon                    string
}

// ConversationProjectPatchInput 定义项目分组局部更新输入。
type ConversationProjectPatchInput struct {
	Name                    *string
	Description             *string
	SystemPrompt            *string
	DefaultModel            *string
	MCPDefaultMode          *string
	DefaultMCPToolIDs       *[]uint
	DefaultSkillIDs         *[]uint
	DefaultKnowledgeBaseIDs *[]string
	Color                   *string
	Icon                    *string
	Status                  *string
}

type conversationProjectDefaultsValidationInput struct {
	UserID           uint
	DefaultModel     string
	MCPDefaultMode   string
	MCPToolIDs       []uint
	SkillIDs         []uint
	KnowledgeBaseIDs []string
	Current          *model.ConversationProject
}

// CreateConversationProject 创建当前用户的会话项目分组。
func (s *Service) CreateConversationProject(ctx context.Context, userID uint, input ConversationProjectInput) (*model.ConversationProject, error) {
	normalized, err := normalizeConversationProjectInput(input)
	if err != nil {
		return nil, err
	}
	if err = s.validateConversationProjectDefaults(ctx, conversationProjectDefaultsValidationInput{
		UserID:           userID,
		DefaultModel:     normalized.DefaultModel,
		MCPDefaultMode:   normalized.MCPDefaultMode,
		MCPToolIDs:       normalized.DefaultMCPToolIDs,
		SkillIDs:         normalized.DefaultSkillIDs,
		KnowledgeBaseIDs: normalized.DefaultKnowledgeBaseIDs,
	}); err != nil {
		return nil, err
	}
	item := &model.ConversationProject{
		UserID:                  userID,
		PublicID:                normalizePublicID(uuid.NewString()),
		Name:                    normalized.Name,
		Description:             normalized.Description,
		SystemPrompt:            normalized.SystemPrompt,
		DefaultModel:            normalized.DefaultModel,
		MCPDefaultMode:          normalized.MCPDefaultMode,
		DefaultMCPToolIDs:       normalized.DefaultMCPToolIDs,
		DefaultSkillIDs:         normalized.DefaultSkillIDs,
		DefaultKnowledgeBaseIDs: normalized.DefaultKnowledgeBaseIDs,
		Color:                   normalized.Color,
		Icon:                    normalized.Icon,
		Status:                  "active",
	}
	if err = s.repo.CreateConversationProject(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// ListConversationProjects 查询当前用户项目分组。
func (s *Service) ListConversationProjects(ctx context.Context, userID uint, statusFilter string) ([]model.ConversationProject, error) {
	return s.repo.ListConversationProjects(ctx, userID, normalizeConversationProjectStatusFilter(statusFilter))
}

// UpdateConversationProject 更新当前用户项目分组。
func (s *Service) UpdateConversationProject(
	ctx context.Context,
	userID uint,
	publicID string,
	input ConversationProjectPatchInput,
) (*model.ConversationProject, error) {
	patch, err := normalizeConversationProjectPatch(input)
	if err != nil {
		return nil, err
	}
	if patch.DefaultModel != nil || patch.MCPDefaultMode != nil || patch.DefaultMCPToolIDs != nil || patch.DefaultSkillIDs != nil || patch.DefaultKnowledgeBaseIDs != nil {
		current, currentErr := s.repo.GetConversationProjectByPublicID(ctx, userID, strings.TrimSpace(publicID))
		if currentErr != nil {
			if errors.Is(currentErr, repository.ErrNotFound) {
				return nil, ErrConversationProjectNotFound
			}
			return nil, currentErr
		}
		defaultModel := current.DefaultModel
		mode := current.MCPDefaultMode
		mcpToolIDs := current.DefaultMCPToolIDs
		skillIDs := current.DefaultSkillIDs
		knowledgeBaseIDs := current.DefaultKnowledgeBaseIDs
		if patch.DefaultModel != nil {
			defaultModel = *patch.DefaultModel
		}
		if patch.MCPDefaultMode != nil {
			mode = *patch.MCPDefaultMode
		}
		if patch.DefaultMCPToolIDs != nil {
			mcpToolIDs = *patch.DefaultMCPToolIDs
		}
		if patch.DefaultSkillIDs != nil {
			skillIDs = *patch.DefaultSkillIDs
		}
		if patch.DefaultKnowledgeBaseIDs != nil {
			knowledgeBaseIDs = *patch.DefaultKnowledgeBaseIDs
		}
		if mode == model.ConversationProjectMCPDefaultModeInherit {
			mcpToolIDs = []uint{}
		}
		if err = s.validateConversationProjectDefaults(ctx, conversationProjectDefaultsValidationInput{
			UserID:           userID,
			DefaultModel:     defaultModel,
			MCPDefaultMode:   mode,
			MCPToolIDs:       mcpToolIDs,
			SkillIDs:         skillIDs,
			KnowledgeBaseIDs: knowledgeBaseIDs,
			Current:          current,
		}); err != nil {
			return nil, err
		}
		patch.MCPDefaultMode = &mode
		patch.DefaultMCPToolIDs = &mcpToolIDs
		patch.DefaultSkillIDs = &skillIDs
		patch.DefaultKnowledgeBaseIDs = &knowledgeBaseIDs
	}
	item, err := s.repo.UpdateConversationProjectMetadataByPublicID(ctx, userID, strings.TrimSpace(publicID), patch)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationProjectNotFound
		}
		return nil, err
	}
	return item, nil
}

// DeleteConversationProject 删除当前用户项目分组。
func (s *Service) DeleteConversationProject(
	ctx context.Context,
	userID uint,
	publicID string,
	deleteConversations bool,
	options DeleteConversationOptions,
) (*DeleteConversationResult, error) {
	cleanupFileIDs, err := s.repo.DeleteConversationProjectByPublicID(
		ctx,
		userID,
		strings.TrimSpace(publicID),
		repository.DeleteConversationProjectOptions{
			DeleteConversations: deleteConversations,
			DeleteFiles:         deleteConversations && options.DeleteFiles,
		},
	)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationProjectNotFound
		}
		return nil, err
	}
	result := &DeleteConversationResult{Deleted: true}
	if deleteConversations && options.DeleteFiles {
		result.DeletedFileCount, result.Quota = s.deleteConversationFiles(ctx, userID, cleanupFileIDs)
	}
	return result, nil
}

// ReorderConversationProjects 更新当前用户项目展示顺序。
func (s *Service) ReorderConversationProjects(ctx context.Context, userID uint, publicIDs []string) error {
	normalizedIDs := normalizeProjectPublicIDs(publicIDs)
	if len(normalizedIDs) == 0 || len(normalizedIDs) != len(publicIDs) {
		return ErrInvalidConversationProject
	}
	if err := s.repo.ReorderConversationProjects(ctx, userID, normalizedIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrConversationProjectNotFound
		}
		return err
	}
	return nil
}

// SetConversationProject 设置当前用户单个会话的项目归属，空项目 ID 表示解除归属。
func (s *Service) SetConversationProject(
	ctx context.Context,
	userID uint,
	conversationPublicID string,
	projectPublicID string,
) (*model.Conversation, error) {
	projectID, err := s.resolveConversationProjectID(ctx, userID, projectPublicID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.UpdateConversationProjectAssignmentByPublicID(ctx, userID, strings.TrimSpace(conversationPublicID), projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return item, nil
}

// BatchSetConversationProject 批量设置当前用户会话项目归属。
func (s *Service) BatchSetConversationProject(
	ctx context.Context,
	userID uint,
	conversationPublicIDs []string,
	projectPublicID string,
) (int64, error) {
	normalizedConversationIDs := normalizeProjectPublicIDs(conversationPublicIDs)
	if len(normalizedConversationIDs) == 0 || len(normalizedConversationIDs) != len(conversationPublicIDs) {
		return 0, ErrInvalidConversationProject
	}
	projectID, err := s.resolveConversationProjectID(ctx, userID, projectPublicID)
	if err != nil {
		return 0, err
	}
	updated, err := s.repo.BatchUpdateConversationProjectByPublicIDs(ctx, userID, normalizedConversationIDs, projectID)
	if err != nil {
		return 0, err
	}
	if updated != int64(len(normalizedConversationIDs)) {
		return updated, ErrConversationNotFound
	}
	return updated, nil
}

func (s *Service) resolveConversationProjectID(ctx context.Context, userID uint, publicID string) (*uint, error) {
	normalizedPublicID := strings.TrimSpace(publicID)
	if normalizedPublicID == "" || normalizedPublicID == "unassigned" {
		return nil, nil
	}
	project, err := s.repo.GetConversationProjectByPublicID(ctx, userID, normalizedPublicID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationProjectNotFound
		}
		return nil, err
	}
	return &project.ID, nil
}

func normalizeConversationProjectInput(input ConversationProjectInput) (ConversationProjectInput, error) {
	mcpDefaultMode := normalizeConversationProjectMCPDefaultMode(input.MCPDefaultMode)
	if mcpDefaultMode == "" {
		if strings.TrimSpace(input.MCPDefaultMode) != "" {
			return ConversationProjectInput{}, ErrInvalidConversationProject
		}
		mcpDefaultMode = model.ConversationProjectMCPDefaultModeInherit
	}
	normalized := ConversationProjectInput{
		Name:                    strings.TrimSpace(input.Name),
		Description:             strings.TrimSpace(input.Description),
		SystemPrompt:            strings.TrimSpace(input.SystemPrompt),
		DefaultModel:            strings.TrimSpace(input.DefaultModel),
		MCPDefaultMode:          mcpDefaultMode,
		DefaultMCPToolIDs:       uniqueToolIDs(input.DefaultMCPToolIDs),
		DefaultSkillIDs:         normalizeSelectedSkillIDs(input.DefaultSkillIDs),
		DefaultKnowledgeBaseIDs: normalizeProjectPublicIDs(input.DefaultKnowledgeBaseIDs),
		Color:                   strings.TrimSpace(input.Color),
		Icon:                    strings.TrimSpace(input.Icon),
	}
	if normalized.MCPDefaultMode == model.ConversationProjectMCPDefaultModeInherit {
		normalized.DefaultMCPToolIDs = []uint{}
	}
	if normalized.Name == "" || exceedsRuneLimit(normalized.Name, conversationProjectNameMaxChars) {
		return ConversationProjectInput{}, ErrInvalidConversationProject
	}
	if len(normalized.DefaultKnowledgeBaseIDs) != len(input.DefaultKnowledgeBaseIDs) || len(normalized.DefaultKnowledgeBaseIDs) > 8 {
		return ConversationProjectInput{}, ErrInvalidConversationProject
	}
	if exceedsRuneLimit(normalized.Description, conversationProjectDescriptionMaxChars) ||
		exceedsRuneLimit(normalized.SystemPrompt, conversationProjectSystemPromptMaxChars) ||
		exceedsRuneLimit(normalized.DefaultModel, conversationProjectModelMaxChars) ||
		exceedsRuneLimit(normalized.Color, conversationProjectMetaMaxChars) ||
		exceedsRuneLimit(normalized.Icon, conversationProjectMetaMaxChars) {
		return ConversationProjectInput{}, ErrInvalidConversationProject
	}
	return normalized, nil
}

func normalizeConversationProjectPatch(input ConversationProjectPatchInput) (model.ConversationProjectPatch, error) {
	var patch model.ConversationProjectPatch
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || exceedsRuneLimit(value, conversationProjectNameMaxChars) {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.Name = &value
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if exceedsRuneLimit(value, conversationProjectDescriptionMaxChars) {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.Description = &value
	}
	if input.SystemPrompt != nil {
		value := strings.TrimSpace(*input.SystemPrompt)
		if exceedsRuneLimit(value, conversationProjectSystemPromptMaxChars) {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.SystemPrompt = &value
	}
	if input.DefaultModel != nil {
		value := strings.TrimSpace(*input.DefaultModel)
		if exceedsRuneLimit(value, conversationProjectModelMaxChars) {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.DefaultModel = &value
	}
	if input.MCPDefaultMode != nil {
		value := normalizeConversationProjectMCPDefaultMode(*input.MCPDefaultMode)
		if value == "" {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.MCPDefaultMode = &value
	}
	if input.DefaultMCPToolIDs != nil {
		value := uniqueToolIDs(*input.DefaultMCPToolIDs)
		patch.DefaultMCPToolIDs = &value
	}
	if input.DefaultSkillIDs != nil {
		value := normalizeSelectedSkillIDs(*input.DefaultSkillIDs)
		patch.DefaultSkillIDs = &value
	}
	if input.DefaultKnowledgeBaseIDs != nil {
		value := normalizeProjectPublicIDs(*input.DefaultKnowledgeBaseIDs)
		if len(value) != len(*input.DefaultKnowledgeBaseIDs) || len(value) > 8 {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.DefaultKnowledgeBaseIDs = &value
	}
	if input.Color != nil {
		value := strings.TrimSpace(*input.Color)
		if exceedsRuneLimit(value, conversationProjectMetaMaxChars) {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.Color = &value
	}
	if input.Icon != nil {
		value := strings.TrimSpace(*input.Icon)
		if exceedsRuneLimit(value, conversationProjectMetaMaxChars) {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.Icon = &value
	}
	if input.Status != nil {
		value := normalizeConversationProjectStatus(*input.Status)
		if value == "" {
			return model.ConversationProjectPatch{}, ErrInvalidConversationProject
		}
		patch.Status = &value
	}
	if patch.Name == nil && patch.Description == nil && patch.SystemPrompt == nil && patch.DefaultModel == nil && patch.MCPDefaultMode == nil &&
		patch.DefaultMCPToolIDs == nil && patch.DefaultSkillIDs == nil && patch.DefaultKnowledgeBaseIDs == nil && patch.Color == nil && patch.Icon == nil && patch.Status == nil {
		return model.ConversationProjectPatch{}, ErrInvalidConversationProject
	}
	return patch, nil
}

// validateConversationProjectDefaults 校验项目默认能力的数量和新增关联的可用性。
func (s *Service) validateConversationProjectDefaults(ctx context.Context, input conversationProjectDefaultsValidationInput) error {
	if normalizeConversationProjectMCPDefaultMode(input.MCPDefaultMode) == "" {
		return ErrInvalidConversationProject
	}
	normalizedDefaultModel := strings.TrimSpace(input.DefaultModel)
	defaultModelChanged := input.Current == nil || normalizedDefaultModel != strings.TrimSpace(input.Current.DefaultModel)
	if defaultModelChanged && normalizedDefaultModel != "" {
		available, err := s.isAvailableConversationProjectDefaultModel(ctx, input.UserID, normalizedDefaultModel)
		if err != nil {
			return err
		}
		if !available {
			return ErrInvalidConversationProject
		}
	}
	mcpSelectionChanged := input.Current == nil ||
		input.MCPDefaultMode != input.Current.MCPDefaultMode ||
		!slices.Equal(input.MCPToolIDs, input.Current.DefaultMCPToolIDs)
	skillSelectionChanged := input.Current == nil || !slices.Equal(input.SkillIDs, input.Current.DefaultSkillIDs)
	knowledgeBaseSelectionChanged := input.Current == nil || !slices.Equal(input.KnowledgeBaseIDs, input.Current.DefaultKnowledgeBaseIDs)
	if (mcpSelectionChanged && len(input.MCPToolIDs) > s.resolveMaxSelectedToolsPerMessage()) ||
		(skillSelectionChanged && len(input.SkillIDs) > s.resolveMaxSelectedSkillsPerMessage()) {
		return ErrInvalidConversationProject
	}
	mcpToolIDsToValidate := input.MCPToolIDs
	skillIDsToValidate := input.SkillIDs
	knowledgeBaseIDsToValidate := input.KnowledgeBaseIDs
	if input.Current != nil {
		mcpToolIDsToValidate = newProjectDefaultIDs(input.MCPToolIDs, input.Current.DefaultMCPToolIDs)
		skillIDsToValidate = newProjectDefaultIDs(input.SkillIDs, input.Current.DefaultSkillIDs)
		knowledgeBaseIDsToValidate = newProjectDefaultPublicIDs(input.KnowledgeBaseIDs, input.Current.DefaultKnowledgeBaseIDs)
	}
	var selectedToolsByID map[uint]domainmcp.Tool
	if input.MCPDefaultMode == model.ConversationProjectMCPDefaultModeCustom &&
		len(input.MCPToolIDs) > 0 &&
		(mcpSelectionChanged || len(mcpToolIDsToValidate) > 0) {
		if s.mcpRepo == nil {
			return ErrInvalidConversationProject
		}
		tools, err := s.mcpRepo.ListToolsByIDs(ctx, input.MCPToolIDs)
		if err != nil {
			return err
		}
		selectedToolsByID = make(map[uint]domainmcp.Tool, len(tools))
		imageProcessorCount := 0
		for _, tool := range tools {
			selectedToolsByID[tool.ID] = tool
			if tool.AttachmentInputMode == domainmcp.AttachmentInputModeImage {
				imageProcessorCount++
			}
		}
		if imageProcessorCount > 1 {
			return ErrInvalidConversationProject
		}
	}
	if input.MCPDefaultMode == model.ConversationProjectMCPDefaultModeCustom && len(mcpToolIDsToValidate) > 0 {
		for _, toolID := range mcpToolIDsToValidate {
			if _, ok := selectedToolsByID[toolID]; !ok {
				return ErrInvalidConversationProject
			}
		}
		servers, err := s.mcpRepo.ListServers(ctx)
		if err != nil {
			return err
		}
		activeServerIDs := make(map[uint]struct{}, len(servers))
		for _, server := range servers {
			if server.Status == "active" {
				activeServerIDs[server.ID] = struct{}{}
			}
		}
		for _, toolID := range mcpToolIDsToValidate {
			tool := selectedToolsByID[toolID]
			if tool.Status != "active" {
				return ErrInvalidConversationProject
			}
			if _, active := activeServerIDs[tool.ServerID]; !active {
				return ErrInvalidConversationProject
			}
		}
	}
	if len(skillIDsToValidate) > 0 {
		if s.skillResolver == nil {
			return ErrInvalidConversationProject
		}
		_, total, err := s.skillResolver.ListVisible(ctx, input.UserID, appskill.ListInput{
			IDs:      skillIDsToValidate,
			Page:     1,
			PageSize: 1,
		})
		if err != nil {
			return err
		}
		if total != int64(len(skillIDsToValidate)) {
			return ErrInvalidConversationProject
		}
	}
	if knowledgeBaseSelectionChanged && len(input.KnowledgeBaseIDs) > 0 && !s.cfg.Snapshot().KnowledgeBaseEnabled {
		return ErrInvalidConversationProject
	}
	if knowledgeBaseSelectionChanged && len(input.KnowledgeBaseIDs) > 8 {
		return ErrInvalidConversationProject
	}
	if len(knowledgeBaseIDsToValidate) > 0 {
		if s.knowledgeBaseResolver == nil {
			return ErrInvalidConversationProject
		}
		bases, _, err := s.knowledgeBaseResolver.ResolveFiles(ctx, input.UserID, knowledgeBaseIDsToValidate)
		if err != nil {
			if errors.Is(err, domainknowledgebase.ErrReferenceUnavailable) {
				return ErrInvalidConversationProject
			}
			return err
		}
		for _, base := range bases {
			if base.ReadyFileCount == 0 {
				return ErrInvalidConversationProject
			}
		}
	}
	return nil
}

func (s *Service) isAvailableConversationProjectDefaultModel(ctx context.Context, userID uint, platformModelName string) (bool, error) {
	resolver, ok := s.routeResolver.(activeModelCatalogResolver)
	if !ok {
		return false, nil
	}
	models, err := resolver.ListActiveModels(ctx, userID)
	if err != nil {
		return false, err
	}
	name := strings.TrimSpace(platformModelName)
	for _, item := range models {
		if strings.TrimSpace(item.PlatformModelName) == name && channel.ModelSupportsTask(item.KindsJSON, channel.TaskTypeChat) {
			return true, nil
		}
	}
	return false, nil
}

func newProjectDefaultPublicIDs(selectedIDs []string, existingIDs []string) []string {
	existing := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = struct{}{}
	}
	added := make([]string, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		if _, ok := existing[id]; !ok {
			added = append(added, id)
		}
	}
	return added
}

// newProjectDefaultIDs 返回本次更新新增的默认能力 ID。
func newProjectDefaultIDs(selectedIDs []uint, existingIDs []uint) []uint {
	existing := make(map[uint]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = struct{}{}
	}
	added := make([]uint, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		if _, ok := existing[id]; !ok {
			added = append(added, id)
		}
	}
	return added
}

// normalizeConversationProjectMCPDefaultMode 规范项目 MCP 默认模式。
func normalizeConversationProjectMCPDefaultMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case model.ConversationProjectMCPDefaultModeInherit:
		return model.ConversationProjectMCPDefaultModeInherit
	case model.ConversationProjectMCPDefaultModeCustom:
		return model.ConversationProjectMCPDefaultModeCustom
	default:
		return ""
	}
}

func normalizeProjectPublicIDs(values []string) []string {
	results := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		results = append(results, normalized)
	}
	return results
}

func normalizeConversationProjectStatusFilter(value string) string {
	switch normalizeConversationProjectStatus(value) {
	case "archived":
		return "archived"
	case "active":
		return "active"
	default:
		if strings.TrimSpace(value) == "all" {
			return "all"
		}
		return "active"
	}
}

func normalizeConversationProjectStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active":
		return "active"
	case "archived":
		return "archived"
	default:
		return ""
	}
}

func normalizeConversationProjectFilter(value string) string {
	normalized := strings.TrimSpace(value)
	switch normalized {
	case "", "all":
		return "all"
	case "unassigned":
		return "unassigned"
	default:
		return normalized
	}
}

func exceedsRuneLimit(value string, limit int) bool {
	return limit >= 0 && utf8.RuneCountInString(value) > limit
}
