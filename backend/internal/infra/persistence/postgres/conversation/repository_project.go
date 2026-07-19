package conversation

import (
	"context"
	"strings"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// CreateConversationProject 创建会话项目分组。
func (r *Repo) CreateConversationProject(ctx context.Context, item *domainconversation.ConversationProject) error {
	entity := toConversationProjectModel(item)
	mcpToolIDs := append([]uint(nil), item.DefaultMCPToolIDs...)
	skillIDs := append([]uint(nil), item.DefaultSkillIDs...)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entity).Error; err != nil {
			return err
		}
		if err := replaceConversationProjectMCPTools(tx, entity.ID, mcpToolIDs); err != nil {
			return err
		}
		return replaceConversationProjectSkills(tx, entity.ID, skillIDs)
	}); err != nil {
		return translateError(err)
	}
	*item = toConversationProjectDomain(entity)
	item.DefaultMCPToolIDs = mcpToolIDs
	item.DefaultSkillIDs = skillIDs
	return nil
}

// ListConversationProjects 查询用户项目分组。
func (r *Repo) ListConversationProjects(ctx context.Context, userID uint, statusFilter string) ([]domainconversation.ConversationProject, error) {
	items := make([]models.ConversationProject, 0)
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID)
	switch strings.TrimSpace(statusFilter) {
	case "archived":
		query = query.Where("status = ?", "archived")
	case "all":
		// 保留全部状态。
	default:
		query = query.Where("status = ?", "active")
	}
	if err := query.
		Order("sort_order ASC").
		Order("id DESC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := toConversationProjectDomains(items)
	if err := r.hydrateConversationProjectDefaults(ctx, results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetConversationProjectByPublicID 查询用户项目分组。
func (r *Repo) GetConversationProjectByPublicID(ctx context.Context, userID uint, publicID string) (*domainconversation.ConversationProject, error) {
	var item models.ConversationProject
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND public_id = ?", userID, strings.TrimSpace(publicID)).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toConversationProjectDomain(item)
	projects := []domainconversation.ConversationProject{result}
	if err := r.hydrateConversationProjectDefaults(ctx, projects); err != nil {
		return nil, err
	}
	result = projects[0]
	return &result, nil
}

// UpdateConversationProjectMetadataByPublicID 更新项目分组元信息。
func (r *Repo) UpdateConversationProjectMetadataByPublicID(
	ctx context.Context,
	userID uint,
	publicID string,
	patch domainconversation.ConversationProjectPatch,
) (*domainconversation.ConversationProject, error) {
	updates := make(map[string]interface{})
	if patch.Name != nil {
		updates["name"] = *patch.Name
	}
	if patch.Description != nil {
		updates["description"] = *patch.Description
	}
	if patch.SystemPrompt != nil {
		updates["system_prompt"] = *patch.SystemPrompt
	}
	if patch.MCPDefaultMode != nil {
		updates["mcp_default_mode"] = *patch.MCPDefaultMode
	}
	if patch.Color != nil {
		updates["color"] = *patch.Color
	}
	if patch.Icon != nil {
		updates["icon"] = *patch.Icon
	}
	if patch.Status != nil {
		updates["status"] = *patch.Status
	}
	if len(updates) == 0 && patch.DefaultMCPToolIDs == nil && patch.DefaultSkillIDs == nil {
		return r.GetConversationProjectByPublicID(ctx, userID, publicID)
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project models.ConversationProject
		if err := tx.Where("user_id = ? AND public_id = ?", userID, strings.TrimSpace(publicID)).First(&project).Error; err != nil {
			return err
		}
		if len(updates) > 0 {
			if err := tx.Model(&project).Updates(updates).Error; err != nil {
				return err
			}
		}
		if patch.DefaultMCPToolIDs != nil {
			if err := replaceConversationProjectMCPTools(tx, project.ID, *patch.DefaultMCPToolIDs); err != nil {
				return err
			}
		}
		if patch.DefaultSkillIDs != nil {
			if err := replaceConversationProjectSkills(tx, project.ID, *patch.DefaultSkillIDs); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, translateError(err)
	}
	return r.GetConversationProjectByPublicID(ctx, userID, publicID)
}

// DeleteConversationProjectByPublicID 删除项目分组，可选择一并软删除其下会话并返回可清理文件 ID。
func (r *Repo) DeleteConversationProjectByPublicID(
	ctx context.Context,
	userID uint,
	publicID string,
	deleteConversations bool,
	deleteFiles bool,
) ([]string, error) {
	normalizedPublicID := strings.TrimSpace(publicID)
	cleanupFileIDs := make([]string, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project models.ConversationProject
		if err := tx.Where("user_id = ? AND public_id = ?", userID, normalizedPublicID).First(&project).Error; err != nil {
			return translateError(err)
		}
		// 项目删除与会话归属处理必须保持原子性，避免项目删除后留下不可见的项目引用。
		if deleteConversations {
			conversationIDs := make([]uint, 0)
			if deleteFiles {
				if err := tx.Model(&models.Conversation{}).
					Where("user_id = ? AND project_id = ?", userID, project.ID).
					Pluck("id", &conversationIDs).Error; err != nil {
					return translateError(err)
				}
			}
			if err := tx.
				Where("user_id = ? AND project_id = ?", userID, project.ID).
				Delete(&models.Conversation{}).Error; err != nil {
				return translateError(err)
			}
			if deleteFiles {
				fileIDs, err := listConversationFileCleanupCandidates(tx, userID, conversationIDs)
				if err != nil {
					return err
				}
				cleanupFileIDs = fileIDs
			}
		} else {
			if err := tx.Model(&models.Conversation{}).
				Where("user_id = ? AND project_id = ?", userID, project.ID).
				Update("project_id", nil).Error; err != nil {
				return translateError(err)
			}
		}
		if err := tx.Where("project_id = ?", project.ID).Delete(&models.ConversationProjectMCPTool{}).Error; err != nil {
			return translateError(err)
		}
		if err := tx.Where("project_id = ?", project.ID).Delete(&models.ConversationProjectSkill{}).Error; err != nil {
			return translateError(err)
		}
		if err := tx.Delete(&project).Error; err != nil {
			return translateError(err)
		}
		return nil
	})
	if err != nil {
		return nil, translateError(err)
	}
	return cleanupFileIDs, nil
}

// ReorderConversationProjects 更新项目展示顺序。
func (r *Repo) ReorderConversationProjects(ctx context.Context, userID uint, publicIDs []string) error {
	if len(publicIDs) == 0 {
		return nil
	}
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, publicID := range publicIDs {
			result := tx.Model(&models.ConversationProject{}).
				Where("user_id = ? AND public_id = ?", userID, strings.TrimSpace(publicID)).
				Update("sort_order", index+1)
			if result.Error != nil {
				return translateError(result.Error)
			}
			if result.RowsAffected == 0 {
				return repository.ErrNotFound
			}
		}
		return nil
	}))
}

// UpdateConversationProjectAssignmentByPublicID 更新单个会话的项目归属。
func (r *Repo) UpdateConversationProjectAssignmentByPublicID(
	ctx context.Context,
	userID uint,
	conversationPublicID string,
	projectID *uint,
) (*domainconversation.Conversation, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("user_id = ? AND public_id = ?", userID, strings.TrimSpace(conversationPublicID)).
		Update("project_id", projectID)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetConversationByPublicID(ctx, conversationPublicID, userID)
}

// BatchUpdateConversationProjectByPublicIDs 批量更新会话项目归属。
func (r *Repo) BatchUpdateConversationProjectByPublicIDs(
	ctx context.Context,
	userID uint,
	conversationPublicIDs []string,
	projectID *uint,
) (int64, error) {
	if len(conversationPublicIDs) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("user_id = ? AND public_id IN ?", userID, conversationPublicIDs).
		Update("project_id", projectID)
	if result.Error != nil {
		return 0, translateError(result.Error)
	}
	return result.RowsAffected, nil
}

func toConversationProjectDomain(item models.ConversationProject) domainconversation.ConversationProject {
	mcpDefaultMode := strings.TrimSpace(item.MCPDefaultMode)
	if mcpDefaultMode != domainconversation.ConversationProjectMCPDefaultModeCustom {
		mcpDefaultMode = domainconversation.ConversationProjectMCPDefaultModeInherit
	}
	return domainconversation.ConversationProject{
		ID:             item.ID,
		UserID:         item.UserID,
		PublicID:       item.PublicID,
		Name:           item.Name,
		Description:    item.Description,
		SystemPrompt:   item.SystemPrompt,
		MCPDefaultMode: mcpDefaultMode,
		Color:          item.Color,
		Icon:           item.Icon,
		SortOrder:      item.SortOrder,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toConversationProjectDomains(items []models.ConversationProject) []domainconversation.ConversationProject {
	results := make([]domainconversation.ConversationProject, 0, len(items))
	for _, item := range items {
		results = append(results, toConversationProjectDomain(item))
	}
	return results
}

func toConversationProjectModel(item *domainconversation.ConversationProject) models.ConversationProject {
	if item == nil {
		return models.ConversationProject{}
	}
	return models.ConversationProject{
		UserID:         item.UserID,
		PublicID:       item.PublicID,
		Name:           item.Name,
		Description:    item.Description,
		SystemPrompt:   item.SystemPrompt,
		MCPDefaultMode: item.MCPDefaultMode,
		Color:          item.Color,
		Icon:           item.Icon,
		SortOrder:      item.SortOrder,
		Status:         item.Status,
	}
}

// hydrateConversationProjectDefaults 批量装载项目默认 MCP 与 Skill 关联，避免列表查询产生 N+1。
func (r *Repo) hydrateConversationProjectDefaults(ctx context.Context, items []domainconversation.ConversationProject) error {
	if len(items) == 0 {
		return nil
	}
	projectIDs := make([]uint, 0, len(items))
	for _, item := range items {
		projectIDs = append(projectIDs, item.ID)
	}

	mcpRows := make([]models.ConversationProjectMCPTool, 0)
	if err := r.db.WithContext(ctx).
		Where("project_id IN ?", projectIDs).
		Order("project_id ASC, sort_order ASC, tool_id ASC").
		Find(&mcpRows).Error; err != nil {
		return translateError(err)
	}
	skillRows := make([]models.ConversationProjectSkill, 0)
	if err := r.db.WithContext(ctx).
		Where("project_id IN ?", projectIDs).
		Order("project_id ASC, sort_order ASC, skill_id ASC").
		Find(&skillRows).Error; err != nil {
		return translateError(err)
	}

	mcpIDsByProject := make(map[uint][]uint, len(items))
	for _, row := range mcpRows {
		mcpIDsByProject[row.ProjectID] = append(mcpIDsByProject[row.ProjectID], row.ToolID)
	}
	skillIDsByProject := make(map[uint][]uint, len(items))
	for _, row := range skillRows {
		skillIDsByProject[row.ProjectID] = append(skillIDsByProject[row.ProjectID], row.SkillID)
	}
	for index := range items {
		items[index].DefaultMCPToolIDs = mcpIDsByProject[items[index].ID]
		items[index].DefaultSkillIDs = skillIDsByProject[items[index].ID]
	}
	return nil
}

// replaceConversationProjectMCPTools 在事务内替换项目默认 MCP 工具关联。
func replaceConversationProjectMCPTools(tx *gorm.DB, projectID uint, toolIDs []uint) error {
	if err := tx.Where("project_id = ?", projectID).Delete(&models.ConversationProjectMCPTool{}).Error; err != nil {
		return err
	}
	rows := make([]models.ConversationProjectMCPTool, 0, len(toolIDs))
	for index, toolID := range toolIDs {
		rows = append(rows, models.ConversationProjectMCPTool{ProjectID: projectID, ToolID: toolID, SortOrder: index + 1})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

// replaceConversationProjectSkills 在事务内替换项目默认 Skill 关联。
func replaceConversationProjectSkills(tx *gorm.DB, projectID uint, skillIDs []uint) error {
	if err := tx.Where("project_id = ?", projectID).Delete(&models.ConversationProjectSkill{}).Error; err != nil {
		return err
	}
	rows := make([]models.ConversationProjectSkill, 0, len(skillIDs))
	for index, skillID := range skillIDs {
		rows = append(rows, models.ConversationProjectSkill{ProjectID: projectID, SkillID: skillID, SortOrder: index + 1})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}
