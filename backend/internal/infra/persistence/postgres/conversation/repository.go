package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	if isUniqueConstraintError(err) {
		return repository.ErrDuplicate
	}
	return err
}

type sqlStateError interface {
	SQLState() string
}

func isUniqueConstraintError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var stateErr sqlStateError
	if errors.As(err, &stateErr) && stateErr.SQLState() == "23505" {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

func truncateText(value string, maxChars int) string {
	if maxChars <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	return string(runes[:maxChars])
}

// Repo 聚合会话域数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) sqliteDialect() bool {
	return r != nil && r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "sqlite"
}

// CreateConversation 创建会话。
func (r *Repo) CreateConversation(ctx context.Context, item *domainconversation.Conversation) error {
	entity := toConversationModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toConversationDomain(entity)
	return nil
}

// ListConversationsByUser 分页查询用户会话。
func (r *Repo) ListConversationsByUser(
	ctx context.Context,
	userID uint,
	offset int,
	limit int,
	statusFilter string,
	starredFilter string,
	shareFilter string,
	projectFilter string,
) ([]domainconversation.Conversation, int64, error) {
	items := make([]models.Conversation, 0)
	var total int64
	query := r.db.WithContext(ctx).Model(&models.Conversation{}).Where("user_id = ?", userID)

	switch statusFilter {
	case "archived":
		query = query.Where("status = ?", "archived")
	case "all":
		// 保留全部状态。
	default:
		query = query.Where("status <> ?", "archived")
	}

	switch starredFilter {
	case "starred":
		query = query.Where("is_starred = ?", true)
	case "unstarred":
		query = query.Where("is_starred = ?", false)
	}

	activeShareExistsSQL := `EXISTS (
		SELECT 1
		FROM chat_conversation_shares AS shares
		WHERE shares.conversation_id = chat_conversations.id
			AND shares.user_id = chat_conversations.user_id
			AND shares.status = ?
	)`
	switch shareFilter {
	case "shared":
		query = query.Where(activeShareExistsSQL, "active")
	case "unshared":
		query = query.Where("NOT "+activeShareExistsSQL, "active")
	}

	switch normalizedProjectFilter := strings.TrimSpace(projectFilter); normalizedProjectFilter {
	case "", "all":
		// 保留全部项目归属。
	case "unassigned":
		query = query.Where("project_id IS NULL")
	default:
		project, err := r.GetConversationProjectByPublicID(ctx, userID, normalizedProjectFilter)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return []domainconversation.Conversation{}, 0, nil
			}
			return nil, 0, err
		}
		query = query.Where("project_id = ?", project.ID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	orderedQuery := query.Session(&gorm.Session{})
	if starredFilter == "starred" {
		orderedQuery = orderedQuery.
			Order("starred_at DESC").
			Order("id DESC")
	} else {
		orderedQuery = orderedQuery.
			Order("updated_at DESC").
			Order("id DESC")
	}
	if err := orderedQuery.Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	results := toConversationDomains(items)
	if err := r.hydrateConversationShareSummaries(ctx, results); err != nil {
		return nil, 0, err
	}
	if err := r.hydrateConversationProjectSummaries(ctx, results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func (r *Repo) hydrateConversationShareSummaries(ctx context.Context, items []domainconversation.Conversation) error {
	if len(items) == 0 {
		return nil
	}
	conversationIDs := make([]uint, 0, len(items))
	for _, item := range items {
		conversationIDs = append(conversationIDs, item.ID)
	}
	shares := make([]models.ConversationShare, 0)
	if err := r.db.WithContext(ctx).
		Where("conversation_id IN ?", conversationIDs).
		Order("updated_at DESC").
		Order("id DESC").
		Find(&shares).Error; err != nil {
		return translateError(err)
	}
	latestByConversationID := make(map[uint]models.ConversationShare, len(shares))
	for _, share := range shares {
		if _, exists := latestByConversationID[share.ConversationID]; exists {
			continue
		}
		latestByConversationID[share.ConversationID] = share
	}
	for index := range items {
		share, ok := latestByConversationID[items[index].ID]
		if !ok {
			items[index].ShareStatus = "none"
			continue
		}
		items[index].ShareStatus = strings.TrimSpace(share.Status)
		if items[index].ShareStatus == "" {
			items[index].ShareStatus = "none"
		}
		if items[index].ShareStatus == "active" {
			items[index].ShareID = share.ShareID
			sharedAt := share.CreatedAt
			items[index].SharedAt = &sharedAt
			items[index].LastShareAccessedAt = share.LastAccessedAt
		}
	}
	return nil
}

func (r *Repo) hydrateConversationShareSummary(ctx context.Context, item *domainconversation.Conversation) error {
	if item == nil {
		return nil
	}
	items := []domainconversation.Conversation{*item}
	if err := r.hydrateConversationShareSummaries(ctx, items); err != nil {
		return err
	}
	*item = items[0]
	return nil
}

func (r *Repo) hydrateConversationProjectSummaries(ctx context.Context, items []domainconversation.Conversation) error {
	if len(items) == 0 {
		return nil
	}
	projectIDs := make([]uint, 0, len(items))
	seen := make(map[uint]struct{}, len(items))
	for _, item := range items {
		if item.ProjectID == nil || *item.ProjectID == 0 {
			continue
		}
		if _, exists := seen[*item.ProjectID]; exists {
			continue
		}
		seen[*item.ProjectID] = struct{}{}
		projectIDs = append(projectIDs, *item.ProjectID)
	}
	if len(projectIDs) == 0 {
		return nil
	}
	projects := make([]models.ConversationProject, 0, len(projectIDs))
	if err := r.db.WithContext(ctx).
		Where("id IN ?", projectIDs).
		Find(&projects).Error; err != nil {
		return translateError(err)
	}
	byID := make(map[uint]models.ConversationProject, len(projects))
	for _, project := range projects {
		byID[project.ID] = project
	}
	for index := range items {
		if items[index].ProjectID == nil {
			continue
		}
		project, ok := byID[*items[index].ProjectID]
		if !ok {
			continue
		}
		items[index].ProjectPublicID = project.PublicID
		items[index].ProjectName = project.Name
		items[index].ProjectSystemPrompt = project.SystemPrompt
	}
	return nil
}

func (r *Repo) hydrateConversationProjectSummary(ctx context.Context, item *domainconversation.Conversation) error {
	if item == nil {
		return nil
	}
	items := []domainconversation.Conversation{*item}
	if err := r.hydrateConversationProjectSummaries(ctx, items); err != nil {
		return err
	}
	*item = items[0]
	return nil
}

// GetConversationByUser 查询归属用户会话。
func (r *Repo) GetConversationByUser(ctx context.Context, conversationID uint, userID uint) (*domainconversation.Conversation, error) {
	var item models.Conversation
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", conversationID, userID).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toConversationDomain(item)
	if err := r.hydrateConversationShareSummary(ctx, &result); err != nil {
		return nil, err
	}
	if err := r.hydrateConversationProjectSummary(ctx, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConversationByPublicID 查询归属用户的公开会话 ID。
func (r *Repo) GetConversationByPublicID(ctx context.Context, publicID string, userID uint) (*domainconversation.Conversation, error) {
	var item models.Conversation
	if err := r.db.WithContext(ctx).
		Where("public_id = ? AND user_id = ?", publicID, userID).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toConversationDomain(item)
	if err := r.hydrateConversationShareSummary(ctx, &result); err != nil {
		return nil, err
	}
	if err := r.hydrateConversationProjectSummary(ctx, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetActiveConversationShareByConversation 查询会话当前有效分享。
func (r *Repo) GetActiveConversationShareByConversation(ctx context.Context, userID uint, conversationID uint) (*domainconversation.ConversationShare, error) {
	var item models.ConversationShare
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ? AND status = ?", userID, conversationID, "active").
		Order("updated_at DESC").
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toConversationShareDomain(item)
	return &result, nil
}

// GetLatestConversationShareByConversation 查询会话最近一次分享记录。
func (r *Repo) GetLatestConversationShareByConversation(ctx context.Context, userID uint, conversationID uint) (*domainconversation.ConversationShare, error) {
	var item models.ConversationShare
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("updated_at DESC").
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toConversationShareDomain(item)
	return &result, nil
}

// GetActiveConversationShareByShareID 查询公开分享与未删除原会话。
func (r *Repo) GetActiveConversationShareByShareID(ctx context.Context, shareID string) (*domainconversation.ConversationShare, *domainconversation.Conversation, error) {
	var share models.ConversationShare
	if err := r.db.WithContext(ctx).
		Where("share_id = ? AND status = ?", shareID, "active").
		First(&share).Error; err != nil {
		return nil, nil, translateError(err)
	}
	var conversation models.Conversation
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", share.ConversationID, share.UserID).
		First(&conversation).Error; err != nil {
		return nil, nil, translateError(err)
	}
	shareDomain := toConversationShareDomain(share)
	conversationDomain := toConversationDomain(conversation)
	return &shareDomain, &conversationDomain, nil
}

// CreateConversationShare 创建会话公开分享快照。
func (r *Repo) CreateConversationShare(ctx context.Context, item *domainconversation.ConversationShare) error {
	entity := toConversationShareModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toConversationShareDomain(entity)
	return nil
}

// ReplaceActiveConversationShare 在一个事务内撤销旧分享并创建新快照。
func (r *Repo) ReplaceActiveConversationShare(ctx context.Context, item *domainconversation.ConversationShare) error {
	if item == nil {
		return nil
	}
	var created models.ConversationShare
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&models.ConversationShare{}).
			Where("user_id = ? AND conversation_id = ? AND status = ?", item.UserID, item.ConversationID, "active").
			Updates(map[string]interface{}{
				"status":     "revoked",
				"revoked_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return translateError(err)
		}
		created = toConversationShareModel(item)
		if err := tx.Create(&created).Error; err != nil {
			return translateError(err)
		}
		return nil
	})
	if err != nil {
		return translateError(err)
	}
	*item = toConversationShareDomain(created)
	return nil
}

// RevokeActiveConversationShares 撤销会话当前有效分享。
func (r *Repo) RevokeActiveConversationShares(ctx context.Context, userID uint, conversationIDs []uint) error {
	if len(conversationIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return translateError(r.db.WithContext(ctx).
		Model(&models.ConversationShare{}).
		Where("user_id = ? AND conversation_id IN ? AND status = ?", userID, conversationIDs, "active").
		Updates(map[string]interface{}{
			"status":     "revoked",
			"revoked_at": now,
			"updated_at": now,
		}).Error)
}

// TouchConversationShareAccess 记录公开分享访问时间。
func (r *Repo) TouchConversationShareAccess(ctx context.Context, shareID string, accessedAt time.Time) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.ConversationShare{}).
		Where("share_id = ? AND status = ?", shareID, "active").
		Update("last_accessed_at", accessedAt).
		Error)
}

// UpdateConversationTitleByPublicID 更新会话标题。
func (r *Repo) UpdateConversationTitleByPublicID(
	ctx context.Context,
	userID uint,
	publicID string,
	title string,
) (*domainconversation.Conversation, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("user_id = ? AND public_id = ?", userID, publicID).
		Update("title", title)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetConversationByPublicID(ctx, publicID, userID)
}

// UpdateConversationMetadata 更新自动生成的会话元数据。
func (r *Repo) UpdateConversationMetadata(ctx context.Context, conversationID uint, title string, labelsJSON string) (*domainconversation.Conversation, error) {
	updates := map[string]interface{}{}
	if strings.TrimSpace(title) != "" {
		updates["title"] = gorm.Expr(
			"CASE WHEN lower(btrim(title)) IN ? THEN ? ELSE title END",
			[]string{"", "new conversation", "new chat", "untitled", "新会话", "新对话", "新的对话"},
			strings.TrimSpace(title),
		)
	}
	if strings.TrimSpace(labelsJSON) != "" {
		updates["labels_json"] = strings.TrimSpace(labelsJSON)
	}
	if len(updates) == 0 {
		var current models.Conversation
		if err := r.db.WithContext(ctx).Where("id = ?", conversationID).First(&current).Error; err != nil {
			return nil, translateError(err)
		}
		result := toConversationDomain(current)
		return &result, nil
	}
	result := r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", conversationID).
		Updates(updates)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	var current models.Conversation
	if err := r.db.WithContext(ctx).Where("id = ?", conversationID).First(&current).Error; err != nil {
		return nil, translateError(err)
	}
	updated := toConversationDomain(current)
	return &updated, nil
}

// UpdateConversationStarByPublicID 更新会话星标状态。
func (r *Repo) UpdateConversationStarByPublicID(
	ctx context.Context,
	userID uint,
	publicID string,
	starred bool,
) (*domainconversation.Conversation, error) {
	current, err := r.GetConversationByPublicID(ctx, publicID, userID)
	if err != nil {
		return nil, translateError(err)
	}
	if current.IsStarred == starred {
		return current, nil
	}

	var starredAt interface{}
	if starred {
		now := time.Now().UTC()
		starredAt = &now
	} else {
		starredAt = nil
	}

	result := r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("user_id = ? AND public_id = ?", userID, publicID).
		UpdateColumns(map[string]interface{}{
			"is_starred": starred,
			"starred_at": starredAt,
		})
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetConversationByPublicID(ctx, publicID, userID)
}

// UpdateConversationArchiveByPublicID 更新会话归档状态。
func (r *Repo) UpdateConversationArchiveByPublicID(
	ctx context.Context,
	userID uint,
	publicID string,
	archived bool,
) (*domainconversation.Conversation, error) {
	nextStatus := "active"
	if archived {
		nextStatus = "archived"
	}
	result := r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("user_id = ? AND public_id = ?", userID, publicID).
		Update("status", nextStatus)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetConversationByPublicID(ctx, publicID, userID)
}

// DeleteConversationByPublicID 删除会话（软删除），并可返回仅被该会话引用的文件 ID。
func (r *Repo) DeleteConversationByPublicID(ctx context.Context, userID uint, publicID string, deleteFiles bool) ([]string, error) {
	normalizedPublicID := strings.TrimSpace(publicID)
	cleanupFileIDs := make([]string, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND public_id = ?", userID, normalizedPublicID).
			First(&item).Error; err != nil {
			return translateError(err)
		}
		if err := tx.Delete(&item).Error; err != nil {
			return translateError(err)
		}
		if !deleteFiles {
			return nil
		}
		// 候选文件必须在会话软删除后计算，避免仍被其他活跃会话引用的文件被误删。
		fileIDs, err := listConversationFileCleanupCandidates(tx, userID, []uint{item.ID})
		if err != nil {
			return err
		}
		cleanupFileIDs = fileIDs
		return nil
	})
	if err != nil {
		return nil, translateError(err)
	}
	return cleanupFileIDs, nil
}

type conversationFileCleanupCandidate struct {
	FileID string `gorm:"column:file_id"`
}

// listConversationFileCleanupCandidates 返回仅被指定会话集合引用、且仍处于 active 状态的文件 ID。
func listConversationFileCleanupCandidates(tx *gorm.DB, userID uint, conversationIDs []uint) ([]string, error) {
	if len(conversationIDs) == 0 {
		return nil, nil
	}
	activeReferenceQuery := tx.
		Table("chat_attachments AS other_a").
		Select("1").
		Joins("JOIN chat_conversations AS other_c ON other_c.id = other_a.conversation_id AND other_c.user_id = other_a.user_id AND other_c.deleted_at IS NULL").
		Where("other_a.user_id = a.user_id AND other_a.file_id = a.file_id AND other_a.status <> ?", "deleted")

	rows := make([]conversationFileCleanupCandidate, 0)
	if err := tx.
		Table("chat_attachments AS a").
		Select("DISTINCT a.file_id AS file_id").
		Joins("JOIN file_objects AS fo ON fo.user_id = a.user_id AND fo.file_id = a.file_id AND fo.status = ? AND fo.deleted_at IS NULL", "active").
		Where("a.user_id = ? AND a.conversation_id IN ? AND a.status <> ? AND a.file_id <> ''", userID, conversationIDs, "deleted").
		Where("NOT EXISTS (?)", activeReferenceQuery).
		Order("a.file_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	fileIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.FileID) != "" {
			fileIDs = append(fileIDs, row.FileID)
		}
	}
	return fileIDs, nil
}

func lockActiveFileObjectsForAttachments(tx *gorm.DB, userID uint, attachments []domainconversation.Attachment) error {
	fileIDs := make([]string, 0, len(attachments))
	seen := make(map[string]struct{}, len(attachments))
	for i := range attachments {
		fileID := strings.TrimSpace(attachments[i].FileID)
		if fileID == "" {
			continue
		}
		attachmentUserID := attachments[i].UserID
		if userID == 0 {
			userID = attachmentUserID
		}
		if attachmentUserID != 0 && attachmentUserID != userID {
			return repository.ErrInvalidInput
		}
		if _, exists := seen[fileID]; exists {
			continue
		}
		seen[fileID] = struct{}{}
		fileIDs = append(fileIDs, fileID)
	}
	if len(fileIDs) == 0 {
		return nil
	}
	if userID == 0 {
		return repository.ErrInvalidInput
	}

	lockedIDs := make([]uint, 0, len(fileIDs))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&models.FileObject{}).
		Where("user_id = ? AND status = ? AND file_id IN ?", userID, "active", fileIDs).
		Pluck("id", &lockedIDs).Error; err != nil {
		return translateError(err)
	}
	if len(lockedIDs) != len(fileIDs) {
		return repository.ErrNotFound
	}
	return nil
}

func ensureFileObjectUnreferencedByActiveConversations(tx *gorm.DB, userID uint, fileID string) error {
	var activeReferences int64
	if err := tx.Table("chat_attachments AS a").
		Joins("JOIN chat_conversations AS c ON c.id = a.conversation_id AND c.user_id = a.user_id AND c.deleted_at IS NULL").
		Where("a.user_id = ? AND a.file_id = ? AND a.status <> ?", userID, fileID, "deleted").
		Count(&activeReferences).Error; err != nil {
		return translateError(err)
	}
	if activeReferences > 0 {
		return repository.ErrConflict
	}
	return nil
}

// GetUserByID 按 ID 查询用户。
func (r *Repo) GetUserByID(ctx context.Context, userID uint) (*domainuser.User, error) {
	var item models.User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toUserDomain(item)
	return &result, nil
}

// IncrementMessageCount 增加消息计数。
func (r *Repo) IncrementMessageCount(ctx context.Context, conversationID uint, delta int) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", conversationID).
		Update("message_count", gorm.Expr("message_count + ?", delta)).
		Error)
}

// UpdateConversationCompactedAt 更新会话最近压缩时间。
func (r *Repo) UpdateConversationCompactedAt(ctx context.Context, conversationID uint, compactedAt time.Time) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]interface{}{
			"last_compacted_at": compactedAt,
		}).
		Error)
}

// UpdateConversationLastResponseID 更新会话最近响应 ID。
func (r *Repo) UpdateConversationLastResponseID(ctx context.Context, conversationID uint, responseID string) error {
	updates := map[string]interface{}{"last_response_id": responseID}
	if strings.TrimSpace(responseID) == "" {
		updates["last_prompt_fingerprint"] = ""
	}
	return translateError(r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", conversationID).
		Updates(updates).
		Error)
}

// UpdateConversationStatefulResponse 同步更新最新响应 ID 与对应的本地上下文状态指纹。
func (r *Repo) UpdateConversationStatefulResponse(ctx context.Context, conversationID uint, responseID string, promptFingerprint string) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]interface{}{
			"last_response_id":        responseID,
			"last_prompt_fingerprint": promptFingerprint,
		}).
		Error)
}

// UpdateConversationModel 更新会话当前使用模型与提供商。
func (r *Repo) UpdateConversationModel(ctx context.Context, conversationID uint, platformModelName string, provider string) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.Conversation{}).
		Where("id = ?", conversationID).
		Updates(map[string]interface{}{
			"model":    platformModelName,
			"provider": provider,
		}).
		Error)
}

// CreateMessage 创建消息。
func (r *Repo) CreateMessage(ctx context.Context, item *domainconversation.Message) error {
	attachmentSnapshot := item.Attachments
	entity := toMessageModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toMessageDomain(entity)
	item.Attachments = attachmentSnapshot
	return nil
}

// CreateMessagePairWithUserAttachments 原子创建用户消息、助手占位消息、用户附件并递增会话消息数。
func (r *Repo) CreateMessagePairWithUserAttachments(
	ctx context.Context,
	userMessage *domainconversation.Message,
	assistantMessage *domainconversation.Message,
	userAttachments []domainconversation.Attachment,
) error {
	if userMessage == nil || assistantMessage == nil {
		return repository.ErrInvalidInput
	}
	userAttachmentSnapshot := userMessage.Attachments
	assistantAttachmentSnapshot := assistantMessage.Attachments
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userEntity := toMessageModel(userMessage)
		if err := tx.Create(&userEntity).Error; err != nil {
			return err
		}
		*userMessage = toMessageDomain(userEntity)
		userMessage.Attachments = userAttachmentSnapshot

		if len(userAttachments) > 0 {
			if err := lockActiveFileObjectsForAttachments(tx, userMessage.UserID, userAttachments); err != nil {
				return err
			}
			entities := make([]models.Attachment, 0, len(userAttachments))
			for i := range userAttachments {
				item := userAttachments[i]
				item.ConversationID = userMessage.ConversationID
				item.MessageID = userMessage.ID
				item.UserID = userMessage.UserID
				entities = append(entities, toAttachmentModel(&item))
			}
			if err := tx.Create(&entities).Error; err != nil {
				return err
			}
		}

		parentMessageID := userMessage.ID
		assistantMessage.ParentMessageID = &parentMessageID
		assistantEntity := toMessageModel(assistantMessage)
		if err := tx.Create(&assistantEntity).Error; err != nil {
			return err
		}
		*assistantMessage = toMessageDomain(assistantEntity)
		assistantMessage.Attachments = assistantAttachmentSnapshot

		result := tx.Model(&models.Conversation{}).
			Where("id = ?", userMessage.ConversationID).
			Update("message_count", gorm.Expr("message_count + ?", 2))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return nil
	}))
}

// GetMessageByPublicID 查询归属会话的消息。
func (r *Repo) GetMessageByPublicID(
	ctx context.Context,
	conversationID uint,
	userID uint,
	publicID string,
) (*domainconversation.Message, error) {
	var item models.Message
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ? AND public_id = ?", conversationID, userID, publicID).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	single := []models.Message{item}
	if err := r.hydrateMessageRefs(ctx, single); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, single); err != nil {
		return nil, err
	}
	item = single[0]
	result := toMessageDomain(item)
	return &result, nil
}

// GetMessageByPublicIDForUser 查询当前用户可访问的消息。
func (r *Repo) GetMessageByPublicIDForUser(ctx context.Context, userID uint, publicID string) (*domainconversation.Message, error) {
	var item models.Message
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND public_id = ?", userID, publicID).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	single := []models.Message{item}
	if err := r.hydrateMessageRefs(ctx, single); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, single); err != nil {
		return nil, err
	}
	item = single[0]
	result := toMessageDomain(item)
	return &result, nil
}

// UpdateMessageUsage 更新消息 token 使用量字段。
func (r *Repo) UpdateMessageUsage(
	ctx context.Context,
	messageID uint,
	inputTokens int64,
	outputTokens int64,
	cacheReadTokens int64,
	cacheWriteTokens int64,
	reasoningTokens int64,
) error {
	tokenUsage := inputTokens + cacheReadTokens + cacheWriteTokens + outputTokens + reasoningTokens
	if tokenUsage < 0 {
		tokenUsage = 0
	}
	return translateError(r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"token_usage":        tokenUsage,
			"input_tokens":       inputTokens,
			"output_tokens":      outputTokens,
			"cache_read_tokens":  cacheReadTokens,
			"cache_write_tokens": cacheWriteTokens,
			"reasoning_tokens":   reasoningTokens,
		}).
		Error)
}

// UpdateMessageState 更新消息处理状态与错误信息。
func (r *Repo) UpdateMessageState(
	ctx context.Context,
	messageID uint,
	status string,
	errorCode string,
	errorMessage string,
) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"status":        status,
			"error_code":    errorCode,
			"error_message": errorMessage,
		}).
		Error)
}

// UpdateAssistantMessageContent 更新当前用户 assistant 消息正文并标记编辑时间。
func (r *Repo) UpdateAssistantMessageContent(
	ctx context.Context,
	userID uint,
	publicID string,
	content string,
	editedAt time.Time,
) (*domainconversation.Message, error) {
	normalizedPublicID := strings.TrimSpace(publicID)
	if userID == 0 || normalizedPublicID == "" {
		return nil, repository.ErrInvalidInput
	}

	var item models.Message
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("user_id = ? AND public_id = ? AND role = ?", userID, normalizedPublicID, "assistant").
			First(&item).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Message{}).
			Where("id = ?", item.ID).
			Updates(map[string]interface{}{
				"content":   content,
				"edited_at": editedAt,
			}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", item.ID).First(&item).Error
	})
	if err != nil {
		return nil, translateError(err)
	}

	single := []models.Message{item}
	if err = r.hydrateMessageRefs(ctx, single); err != nil {
		return nil, err
	}
	if err = r.hydrateMessageAttachments(ctx, single); err != nil {
		return nil, err
	}
	item = single[0]
	result := toMessageDomain(item)
	return &result, nil
}

// CancelPendingGenerationMessagesByRunID 将用户显式取消的 pending 回合更新为稳定终态。
func (r *Repo) CancelPendingGenerationMessagesByRunID(
	ctx context.Context,
	userID uint,
	runID string,
	errorCode string,
	errorMessage string,
) (bool, error) {
	normalizedRunID := strings.TrimSpace(runID)
	if userID == 0 || normalizedRunID == "" {
		return false, repository.ErrInvalidInput
	}
	normalizedErrorCode := strings.TrimSpace(errorCode)
	normalizedErrorMessage := truncateText(strings.TrimSpace(errorMessage), 255)
	returnedRows := int64(0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userResult := tx.Model(&models.Message{}).
			Where("user_id = ? AND run_id = ? AND role = ? AND status = ?", userID, normalizedRunID, "user", "pending").
			Updates(map[string]interface{}{
				"status":        "success",
				"error_code":    "",
				"error_message": "",
			})
		if userResult.Error != nil {
			return userResult.Error
		}
		assistantResult := tx.Model(&models.Message{}).
			Where("user_id = ? AND run_id = ? AND role = ? AND status = ?", userID, normalizedRunID, "assistant", "pending").
			Updates(map[string]interface{}{
				"status":        "canceled",
				"error_code":    normalizedErrorCode,
				"error_message": normalizedErrorMessage,
			})
		if assistantResult.Error != nil {
			return assistantResult.Error
		}
		returnedRows = userResult.RowsAffected + assistantResult.RowsAffected
		return nil
	})
	if err != nil {
		return false, translateError(err)
	}
	return returnedRows > 0, nil
}

// InterruptPendingAssistantMessageByRunID 将失去活跃生成流的 pending assistant 标记为错误。
func (r *Repo) InterruptPendingAssistantMessageByRunID(
	ctx context.Context,
	userID uint,
	runID string,
	errorCode string,
	errorMessage string,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("user_id = ? AND run_id = ? AND role = ? AND status = ?", userID, strings.TrimSpace(runID), "assistant", "pending").
		Updates(map[string]interface{}{
			"status":        "error",
			"error_code":    strings.TrimSpace(errorCode),
			"error_message": truncateText(strings.TrimSpace(errorMessage), 255),
		})
	if result.Error != nil {
		return false, translateError(result.Error)
	}
	return result.RowsAffected > 0, nil
}

// UpdateAssistantMessageCompletion 回填 assistant 消息正文、用量与状态。
func (r *Repo) UpdateAssistantMessageCompletion(
	ctx context.Context,
	messageID uint,
	content string,
	outputTokens int64,
	reasoningTokens int64,
	latencyMS int64,
	status string,
	errorCode string,
	errorMessage string,
) error {
	tokenUsage := outputTokens + reasoningTokens
	if tokenUsage < 0 {
		tokenUsage = 0
	}
	if latencyMS < 0 {
		latencyMS = 0
	}
	return translateError(r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"content":          content,
			"token_usage":      tokenUsage,
			"output_tokens":    outputTokens,
			"reasoning_tokens": reasoningTokens,
			"latency_ms":       latencyMS,
			"status":           status,
			"error_code":       errorCode,
			"error_message":    errorMessage,
		}).
		Error)
}

// CompleteAssistantMessageWithAttachments 原子写入助手附件，并同步用户用量与助手完成态。
func (r *Repo) CompleteAssistantMessageWithAttachments(
	ctx context.Context,
	userMessageID uint,
	userUsage repository.MessageUsageUpdate,
	assistantMessageID uint,
	assistantCompletion repository.AssistantMessageCompletionUpdate,
	assistantAttachments []domainconversation.Attachment,
) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(assistantAttachments) > 0 {
			if err := lockActiveFileObjectsForAttachments(tx, 0, assistantAttachments); err != nil {
				return err
			}
			entities := make([]models.Attachment, 0, len(assistantAttachments))
			for i := range assistantAttachments {
				item := assistantAttachments[i]
				item.MessageID = assistantMessageID
				entities = append(entities, toAttachmentModel(&item))
			}
			if err := tx.Create(&entities).Error; err != nil {
				return err
			}
		}

		userTokenUsage := userUsage.InputTokens + userUsage.CacheReadTokens + userUsage.CacheWriteTokens + userUsage.OutputTokens + userUsage.ReasoningTokens
		if userTokenUsage < 0 {
			userTokenUsage = 0
		}
		if err := tx.Model(&models.Message{}).
			Where("id = ?", userMessageID).
			Updates(map[string]interface{}{
				"token_usage":        userTokenUsage,
				"input_tokens":       userUsage.InputTokens,
				"output_tokens":      userUsage.OutputTokens,
				"cache_read_tokens":  userUsage.CacheReadTokens,
				"cache_write_tokens": userUsage.CacheWriteTokens,
				"reasoning_tokens":   userUsage.ReasoningTokens,
			}).Error; err != nil {
			return err
		}

		assistantTokenUsage := assistantCompletion.OutputTokens + assistantCompletion.ReasoningTokens
		if assistantTokenUsage < 0 {
			assistantTokenUsage = 0
		}
		latencyMS := assistantCompletion.LatencyMS
		if latencyMS < 0 {
			latencyMS = 0
		}
		updates := map[string]interface{}{
			"content":          assistantCompletion.Content,
			"token_usage":      assistantTokenUsage,
			"output_tokens":    assistantCompletion.OutputTokens,
			"reasoning_tokens": assistantCompletion.ReasoningTokens,
			"latency_ms":       latencyMS,
			"status":           assistantCompletion.Status,
			"error_code":       assistantCompletion.ErrorCode,
			"error_message":    assistantCompletion.ErrorMessage,
		}
		if contentType := strings.TrimSpace(assistantCompletion.ContentType); contentType != "" {
			updates["content_type"] = contentType
		}
		return tx.Model(&models.Message{}).
			Where("id = ?", assistantMessageID).
			Updates(updates).Error
	}))
}

// UpdateMessageBilling 回填消息计费金额与计费快照。
func (r *Repo) UpdateMessageBilling(ctx context.Context, messageID uint, billedCurrency string, billedNanousd int64, pricingSnapshot string) error {
	if billedNanousd < 0 {
		billedNanousd = 0
	}
	return translateError(r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"billed_currency":  billedCurrency,
			"billed_nanousd":   billedNanousd,
			"pricing_snapshot": pricingSnapshot,
		}).
		Error)
}

// SumMessageTokens 统计会话 token 消耗总量。
func (r *Repo) SumMessageTokens(ctx context.Context, conversationID uint) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&models.Message{}).
		Select("COALESCE(SUM(token_usage), 0)").
		Where("conversation_id = ?", conversationID).
		Scan(&total).Error; err != nil {
		return 0, translateError(err)
	}
	return total, nil
}

// ListMessages 查询会话消息。
func (r *Repo) ListMessages(ctx context.Context, conversationID uint, offset int, limit int) ([]domainconversation.Message, int64, error) {
	items := make([]models.Message, 0)
	var total int64

	if err := r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("conversation_id = ?", conversationID).
		Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("id ASC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := r.hydrateMessageRefs(ctx, items); err != nil {
		return nil, 0, err
	}
	if err := r.hydrateMessageAttachments(ctx, items); err != nil {
		return nil, 0, err
	}
	return toMessageDomains(items), total, nil
}

// ListMessagesBeforeID 查询指定消息 ID 之前的一页会话消息（按时间升序返回）。
func (r *Repo) ListMessagesBeforeID(ctx context.Context, conversationID uint, beforeID uint, limit int) ([]domainconversation.Message, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	items := make([]models.Message, 0, limit)
	var total int64

	if err := r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("conversation_id = ?", conversationID).
		Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}

	if err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND id < ?", conversationID, beforeID).
		Order("id DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	if err := r.hydrateMessageRefs(ctx, items); err != nil {
		return nil, 0, err
	}
	if err := r.hydrateMessageAttachments(ctx, items); err != nil {
		return nil, 0, err
	}
	return toMessageDomains(items), total, nil
}

// ListMessagesForShare 查询分享快照可公开展示的消息。
func (r *Repo) ListMessagesForShare(ctx context.Context, conversationID uint, publicIDs []string) ([]domainconversation.Message, error) {
	items := make([]models.Message, 0)
	query := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID)
	if len(publicIDs) > 0 {
		query = query.Where("public_id IN ?", publicIDs)
	}
	if err := query.Order("id ASC").Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	if err := r.hydrateMessageRefs(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, items); err != nil {
		return nil, err
	}
	result := toMessageDomains(items)
	if len(publicIDs) == 0 {
		return result, nil
	}
	byPublicID := make(map[string]domainconversation.Message, len(result))
	for _, item := range result {
		byPublicID[item.PublicID] = item
	}
	ordered := make([]domainconversation.Message, 0, len(publicIDs))
	for _, publicID := range publicIDs {
		item, ok := byPublicID[publicID]
		if !ok {
			continue
		}
		ordered = append(ordered, item)
	}
	return ordered, nil
}

// ListAllMessages 查询会话全部消息。
func (r *Repo) ListAllMessages(ctx context.Context, conversationID uint) ([]domainconversation.Message, error) {
	items := make([]models.Message, 0)
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	if err := r.hydrateMessageRefs(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, items); err != nil {
		return nil, err
	}
	return toMessageDomains(items), nil
}

// UpsertMessageFeedback 写入或更新消息反馈。
func (r *Repo) UpsertMessageFeedback(ctx context.Context, item *domainconversation.MessageFeedback) error {
	entity := toMessageFeedbackModel(item)
	return translateError(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "message_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"conversation_id",
				"feedback",
				"updated_at",
			}),
		}).
		Create(&entity).Error)
}

// DeleteMessageFeedback 删除用户对消息的反馈。
func (r *Repo) DeleteMessageFeedback(ctx context.Context, userID uint, messageID uint) error {
	return translateError(r.db.WithContext(ctx).
		Where("user_id = ? AND message_id = ?", userID, messageID).
		Delete(&models.ConversationMessageFeedback{}).Error)
}

// GetUserMessageFeedbackMap 查询用户对消息列表的反馈映射。
func (r *Repo) GetUserMessageFeedbackMap(
	ctx context.Context,
	userID uint,
	messageIDs []uint,
) (map[uint]string, error) {
	result := make(map[uint]string, len(messageIDs))
	if len(messageIDs) == 0 {
		return result, nil
	}

	items := make([]models.ConversationMessageFeedback, 0, len(messageIDs))
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND message_id IN ?", userID, messageIDs).
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	for _, item := range items {
		result[item.MessageID] = item.Feedback
	}
	return result, nil
}

type messageFeedbackAggregateRow struct {
	MessageID uint   `gorm:"column:message_id"`
	Feedback  string `gorm:"column:feedback"`
	Count     int64  `gorm:"column:count"`
}

// GetMessageFeedbackCounts 查询消息列表的反馈计数。
func (r *Repo) GetMessageFeedbackCounts(
	ctx context.Context,
	messageIDs []uint,
) (map[uint]map[string]int64, error) {
	result := make(map[uint]map[string]int64, len(messageIDs))
	if len(messageIDs) == 0 {
		return result, nil
	}

	rows := make([]messageFeedbackAggregateRow, 0)
	if err := r.db.WithContext(ctx).
		Model(&models.ConversationMessageFeedback{}).
		Select("message_id, feedback, COUNT(*) AS count").
		Where("message_id IN ?", messageIDs).
		Group("message_id, feedback").
		Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}

	for _, row := range rows {
		bucket := result[row.MessageID]
		if bucket == nil {
			bucket = map[string]int64{
				"up":   0,
				"down": 0,
			}
		}
		bucket[row.Feedback] = row.Count
		result[row.MessageID] = bucket
	}
	return result, nil
}

// CreateAttachments 批量创建附件。
func (r *Repo) CreateAttachments(ctx context.Context, items []domainconversation.Attachment) error {
	if len(items) == 0 {
		return nil
	}
	entities := make([]models.Attachment, 0, len(items))
	for i := range items {
		entities = append(entities, toAttachmentModel(&items[i]))
	}
	return translateError(r.db.WithContext(ctx).Create(&entities).Error)
}

const (
	chatRunEventScopeTraceBlock = "trace_block"
	chatRunEventScopeTraceEvent = "trace_event"
	chatRunEventScopeToolCall   = "tool_call"
	chatContextRecordSnapshot   = "snapshot"
	chatContextRecordArtifact   = "artifact"
)

// CreateConversationRun 写入会话运行日志。
func (r *Repo) CreateConversationRun(ctx context.Context, item *domainconversation.Run) error {
	entity := toConversationRunModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toConversationRunDomain(entity)
	return nil
}

// UpsertConversationMessageTrace 写入或更新消息轨迹。
func (r *Repo) UpsertConversationMessageTrace(ctx context.Context, item *domainconversation.MessageTrace) error {
	if item == nil {
		return nil
	}
	entity := toConversationMessageTraceModel(item)
	return translateError(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "run_id"},
				{Name: "event_scope"},
				{Name: "event_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"message_id",
				"conversation_id",
				"user_id",
				"event_type",
				"phase",
				"stage",
				"round_id",
				"parent_event_id",
				"status",
				"title",
				"summary",
				"content_markdown",
				"payload_json",
				"seq",
				"started_at",
				"ended_at",
				"updated_at",
			}),
		}).
		Create(&entity).Error)
}

// ListConversationMessageTracesByMessageIDs 查询消息轨迹。
func (r *Repo) ListConversationMessageTracesByMessageIDs(ctx context.Context, messageIDs []uint) ([]domainconversation.MessageTrace, error) {
	items := make([]models.ChatRunEvent, 0, len(messageIDs))
	if len(messageIDs) == 0 {
		return []domainconversation.MessageTrace{}, nil
	}
	if err := r.db.WithContext(ctx).
		Where("message_id IN ? AND event_scope = ?", messageIDs, chatRunEventScopeTraceBlock).
		Order("message_id ASC, seq ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toConversationMessageTraceDomains(items), nil
}

// UpsertConversationMessageTraceEvent 写入或更新消息轨迹事件。
func (r *Repo) UpsertConversationMessageTraceEvent(ctx context.Context, item *domainconversation.MessageTraceEventRow) error {
	if item == nil {
		return nil
	}
	entity := toConversationMessageTraceEventModel(item)
	return translateError(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "run_id"},
				{Name: "event_scope"},
				{Name: "event_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"message_id",
				"conversation_id",
				"user_id",
				"event_type",
				"phase",
				"stage",
				"round_id",
				"parent_event_id",
				"status",
				"title",
				"summary",
				"content_markdown",
				"payload_json",
				"seq",
				"started_at",
				"ended_at",
				"updated_at",
			}),
		}).
		Create(&entity).Error)
}

// ListConversationMessageTraceEventsByMessageIDs 查询消息轨迹事件。
func (r *Repo) ListConversationMessageTraceEventsByMessageIDs(ctx context.Context, messageIDs []uint) ([]domainconversation.MessageTraceEventRow, error) {
	items := make([]models.ChatRunEvent, 0, len(messageIDs))
	if len(messageIDs) == 0 {
		return []domainconversation.MessageTraceEventRow{}, nil
	}
	if err := r.db.WithContext(ctx).
		Where("message_id IN ? AND event_scope = ?", messageIDs, chatRunEventScopeTraceEvent).
		Order("message_id ASC, seq ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toConversationMessageTraceEventDomains(items), nil
}

// CreateConversationToolCalls 批量写入工具调用日志。
func (r *Repo) CreateConversationToolCalls(ctx context.Context, items []domainconversation.ToolCall) error {
	if len(items) == 0 {
		return nil
	}
	entities := make([]models.ChatRunEvent, 0, len(items))
	for i := range items {
		entities = append(entities, toConversationToolCallModel(&items[i]))
	}
	return translateError(r.db.WithContext(ctx).Create(&entities).Error)
}

// ListConversationRuns 分页查询会话运行日志。
func (r *Repo) ListConversationRuns(
	ctx context.Context,
	userID uint,
	conversationID uint,
	offset int,
	limit int,
) ([]domainconversation.Run, int64, error) {
	items := make([]models.ConversationRun, 0)
	var total int64

	query := r.db.WithContext(ctx).
		Model(&models.ConversationRun{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := query.
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return toConversationRunDomains(items), total, nil
}

// ListConversationRunsByRunIDs 按运行 ID 查询会话运行快照。
func (r *Repo) ListConversationRunsByRunIDs(
	ctx context.Context,
	userID uint,
	conversationID uint,
	runIDs []string,
) ([]domainconversation.Run, error) {
	if len(runIDs) == 0 {
		return []domainconversation.Run{}, nil
	}
	items := make([]models.ConversationRun, 0, len(runIDs))
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ? AND run_id IN ?", userID, conversationID, runIDs).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toConversationRunDomains(items), nil
}

// GetMessageByID 按内部 ID 查询消息。
func (r *Repo) GetMessageByID(ctx context.Context, conversationID uint, messageID uint) (*domainconversation.Message, error) {
	var item models.Message
	if err := r.db.WithContext(ctx).
		Where("id = ? AND conversation_id = ?", messageID, conversationID).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	single := []models.Message{item}
	if err := r.hydrateMessageRefs(ctx, single); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, single); err != nil {
		return nil, err
	}
	item = single[0]
	result := toMessageDomain(item)
	return &result, nil
}

// GetLatestMessage 查询会话最新一条消息。
func (r *Repo) GetLatestMessage(ctx context.Context, conversationID uint) (*domainconversation.Message, error) {
	var item models.Message
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("id DESC").
		Limit(1).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	single := []models.Message{item}
	if err := r.hydrateMessageRefs(ctx, single); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, single); err != nil {
		return nil, err
	}
	item = single[0]
	result := toMessageDomain(item)
	return &result, nil
}

// ListMessageAncestors 从指定消息向上遍历 parent_message_id 链，返回祖先消息（按 id ASC 排列）。
// 使用 WITH RECURSIVE CTE 一次往返代替原来最多 40 次单行查询（N+1 反模式）。
func (r *Repo) ListMessageAncestors(ctx context.Context, conversationID uint, leafMessageID uint, maxDepth int) ([]domainconversation.Message, error) {
	if maxDepth <= 0 {
		maxDepth = 40
	}

	// WITH RECURSIVE：从叶节点沿 parent_message_id 向上递归，_depth 用于限制深度。
	// 外层 SELECT 显式列出所有 DB 列，排除 CTE 内部的 _depth 辅助列，
	// 避免 GORM Scan 遇到未知字段。deleted_at IS NULL 保持软删除语义。
	const cteSQL = `
WITH RECURSIVE ancestors AS (
    SELECT *, 1 AS _depth
    FROM chat_messages
    WHERE id = ? AND conversation_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT m.*, a._depth + 1
    FROM chat_messages m
    INNER JOIN ancestors a ON m.id = a.parent_message_id
    WHERE a.parent_message_id IS NOT NULL
      AND a._depth < ?
      AND m.deleted_at IS NULL
)
SELECT id, conversation_id, user_id, public_id, parent_message_id, run_id,
       role, content_type, content, branch_reason, source_message_id,
       token_usage, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens,
       latency_ms, billed_currency, billed_nanousd, pricing_snapshot,
       status, error_code, error_message, is_compacted, edited_at,
       created_at, updated_at, deleted_at
FROM ancestors
ORDER BY id ASC`

	path := make([]models.Message, 0, maxDepth)
	if err := r.db.WithContext(ctx).Raw(cteSQL, leafMessageID, conversationID, maxDepth).Scan(&path).Error; err != nil {
		return nil, translateError(err)
	}

	if err := r.hydrateMessageRefs(ctx, path); err != nil {
		return nil, err
	}
	if err := r.hydrateMessageAttachments(ctx, path); err != nil {
		return nil, err
	}
	return toMessageDomains(path), nil
}

// ListRecentMessages 查询会话最近消息窗口（按时间升序返回）。
func (r *Repo) ListRecentMessages(ctx context.Context, conversationID uint, limit int) ([]domainconversation.Message, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&models.Message{}).
		Where("conversation_id = ?", conversationID).
		Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	offset := int(total) - limit
	if offset < 0 {
		offset = 0
	}
	items := make([]models.Message, 0, limit)
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("id ASC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := r.hydrateMessageRefs(ctx, items); err != nil {
		return nil, 0, err
	}
	if err := r.hydrateMessageAttachments(ctx, items); err != nil {
		return nil, 0, err
	}
	return toMessageDomains(items), total, nil
}

// CreateContextSnapshot 写入上下文压缩快照。
func (r *Repo) CreateContextSnapshot(ctx context.Context, item *domainconversation.ContextSnapshot) error {
	entity := toContextSnapshotModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toContextSnapshotDomain(entity)
	return nil
}

// GetContextSnapshotByRunID 按运行 ID 查询上下文压缩快照。
func (r *Repo) GetContextSnapshotByRunID(ctx context.Context, runID string) (*domainconversation.ContextSnapshot, error) {
	var item models.ChatContextRecord
	if err := r.db.WithContext(ctx).
		Where("record_type = ? AND run_id = ?", chatContextRecordSnapshot, runID).
		Order("id DESC").
		Limit(1).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toContextSnapshotDomain(item)
	return &result, nil
}

// GetLatestContextSnapshot 查询最近一次上下文压缩快照。
func (r *Repo) GetLatestContextSnapshot(ctx context.Context, conversationID uint) (*domainconversation.ContextSnapshot, error) {
	var item models.ChatContextRecord
	if err := r.db.WithContext(ctx).
		Where("record_type = ? AND conversation_id = ?", chatContextRecordSnapshot, conversationID).
		Order("id DESC").
		Limit(1).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toContextSnapshotDomain(item)
	return &result, nil
}

// ListFileObjectsByUser 分页查询用户文件。
func (r *Repo) ListFileObjectsByUser(ctx context.Context, userID uint, offset int, limit int) ([]domainconversation.FileObject, int64, error) {
	return r.ListFileObjectsByUserWithFilter(ctx, userID, offset, limit, "", "all", "created")
}

func (r *Repo) ListFileObjectsByUserWithFilter(
	ctx context.Context,
	userID uint,
	offset int,
	limit int,
	searchQuery string,
	filterKind string,
	sortBy string,
) ([]domainconversation.FileObject, int64, error) {
	items := make([]models.FileObject, 0)
	var total int64

	query := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND status = ?", userID, "active")
	normalizedQuery := strings.TrimSpace(searchQuery)
	if normalizedQuery != "" {
		pattern := "%" + strings.ToLower(normalizedQuery) + "%"
		query = query.Where(
			"LOWER(file_id) LIKE ? OR LOWER(file_name) LIKE ? OR LOWER(mime_type) LIKE ? OR LOWER(purpose) LIKE ? OR LOWER(sha256) LIKE ?",
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}
	if condition, args := buildFileKindWhereClause(filterKind); condition != "" {
		query = query.Where(condition, args...)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	orderQuery := query
	switch sortBy {
	case "name":
		orderQuery = orderQuery.Order("file_name ASC").Order("id DESC")
	case "size":
		orderQuery = orderQuery.Order("size_bytes DESC").Order("id DESC")
	case "last_used":
		orderQuery = orderQuery.Order("COALESCE(last_accessed_at, created_at) DESC").Order("id DESC")
	default:
		orderQuery = orderQuery.Order("created_at DESC").Order("id DESC")
	}
	if err := orderQuery.
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return toFileObjectDomains(items), total, nil
}

// GetActiveFileObjectsByIDs 查询用户激活文件对象。
func (r *Repo) GetActiveFileObjectsByIDs(ctx context.Context, userID uint, fileIDs []string) ([]domainconversation.FileObject, error) {
	items := make([]models.FileObject, 0)
	if len(fileIDs) == 0 {
		return []domainconversation.FileObject{}, nil
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND file_id IN ?", userID, "active", fileIDs).
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toFileObjectDomains(items), nil
}

// GetActiveFileObjectByID 查询单个用户激活文件对象。
func (r *Repo) GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error) {
	var item models.FileObject
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND file_id = ?", userID, "active", fileID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, translateError(err)
	}
	result := toFileObjectDomain(item)
	return &result, nil
}

// RenameFileObjectByID 更新文件名。
func (r *Repo) RenameFileObjectByID(ctx context.Context, userID uint, fileID string, fileName string) (*domainconversation.FileObject, error) {
	var item models.FileObject
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND file_id = ?", userID, "active", fileID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, translateError(err)
	}

	item.FileName = fileName
	if err := r.db.WithContext(ctx).Save(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toFileObjectDomain(item)
	return &result, nil
}

// UpdateFileObjectRagOptOut 更新文件 RAG 检索开关。
func (r *Repo) UpdateFileObjectRagOptOut(ctx context.Context, userID uint, fileID string, ragOptOut bool) (*domainconversation.FileObject, error) {
	var item models.FileObject
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND file_id = ?", userID, "active", fileID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, translateError(err)
	}
	item.RagOptOut = ragOptOut
	if err := r.db.WithContext(ctx).Save(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toFileObjectDomain(item)
	return &result, nil
}

// TouchFileObjectLastAccessedAt 更新文件最近使用时间。
func (r *Repo) TouchFileObjectLastAccessedAt(ctx context.Context, userID uint, fileID string, accessedAt time.Time) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND status = ? AND file_id = ?", userID, "active", fileID).
		Update("last_accessed_at", accessedAt).Error)
}

// GetLatestActiveFileObjectBySHA 查询用户最近上传的同内容文件（按 SHA256 + Size）。
func (r *Repo) GetLatestActiveFileObjectBySHA(
	ctx context.Context,
	userID uint,
	sha256Value string,
	sizeBytes int64,
) (*domainconversation.FileObject, error) {
	var item models.FileObject
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND sha256 = ? AND size_bytes = ?", userID, "active", sha256Value, sizeBytes).
		Order("id DESC").
		Limit(1).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, translateError(err)
	}
	result := toFileObjectDomain(item)
	return &result, nil
}

func buildFileKindWhereClause(filterKind string) (string, []interface{}) {
	normalized := strings.ToLower(strings.TrimSpace(filterKind))
	if normalized == "" || normalized == "all" {
		return "", nil
	}

	parts := strings.Split(normalized, ",")
	conditions := make([]string, 0, len(parts))
	args := make([]interface{}, 0, len(parts)*8)
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		current := strings.TrimSpace(part)
		if current == "" || current == "all" {
			continue
		}
		if _, exists := seen[current]; exists {
			continue
		}
		seen[current] = struct{}{}

		condition, conditionArgs := buildSingleFileKindWhereClause(current)
		if condition == "" {
			continue
		}
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func buildSingleFileKindWhereClause(filterKind string) (string, []interface{}) {
	switch filterKind {
	case "image":
		return "LOWER(mime_type) LIKE ?", []interface{}{"image/%"}
	case "audio":
		return "LOWER(mime_type) LIKE ?", []interface{}{"audio/%"}
	case "video":
		return "LOWER(mime_type) LIKE ?", []interface{}{"video/%"}
	case "pdf":
		return "(LOWER(mime_type) = ? OR LOWER(file_name) LIKE ?)", []interface{}{"application/pdf", "%.pdf"}
	case "spreadsheet":
		return "(" + strings.Join([]string{
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
			}, " OR ") + ")", []interface{}{
				"%spreadsheet%",
				"%excel%",
				"%csv%",
				"%.xls",
				"%.xlsx",
				"%.csv",
				"%.ods",
			}
	case "presentation":
		return "(" + strings.Join([]string{
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
			}, " OR ") + ")", []interface{}{
				"%presentation%",
				"%powerpoint%",
				"%.ppt",
				"%.pptx",
				"%.odp",
			}
	case "document":
		return "(" + strings.Join([]string{
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
			}, " OR ") + ")", []interface{}{
				"%word%",
				"%rtf%",
				"%opendocument.text%",
				"%.doc",
				"%.docx",
				"%.rtf",
				"%.odt",
				"%.pages",
			}
	case "code":
		return "(" + strings.Join([]string{
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(mime_type) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
				"LOWER(file_name) LIKE ?",
			}, " OR ") + ")", []interface{}{
				"text/%",
				"%json%",
				"%javascript%",
				"%typescript%",
				"%xml%",
				"%html%",
				"%css%",
				"%yaml%",
				"%toml%",
				"%sql%",
				"%markdown%",
				"%.js",
				"%.jsx",
				"%.ts",
				"%.tsx",
				"%.json",
				"%.html",
				"%.css",
				"%.md",
				"%.xml",
				"%.yaml",
				"%.yml",
				"%.toml",
				"%.sql",
				"%.sh",
				"%.py",
			}
	default:
		return "", nil
	}
}

// CreateFileObjectAndConsumeQuota 创建文件对象并扣减配额。
func (r *Repo) CreateFileObjectAndConsumeQuota(
	ctx context.Context,
	item *domainconversation.FileObject,
	defaultQuotaBytes int64,
) (*domainconversation.StorageQuota, error) {
	var updatedQuota models.UserStorageQuota

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity := toFileObjectModel(item)
		quota, err := getOrInitQuotaForUpdate(tx, entity.UserID, defaultQuotaBytes)
		if err != nil {
			return translateError(err)
		}

		nextUsed := quota.UsedBytes + entity.SizeBytes
		if quota.QuotaBytes > 0 && nextUsed+quota.ReservedBytes > quota.QuotaBytes {
			return ErrStorageQuotaExceeded
		}

		if err = tx.Create(&entity).Error; err != nil {
			return translateError(err)
		}
		*item = toFileObjectDomain(entity)

		if err = tx.Model(&models.UserStorageQuota{}).
			Where("id = ?", quota.ID).
			Update("used_bytes", nextUsed).Error; err != nil {
			return translateError(err)
		}

		if err = tx.Where("id = ?", quota.ID).First(&updatedQuota).Error; err != nil {
			return translateError(err)
		}
		return nil
	})
	if err != nil {
		return nil, translateError(err)
	}

	result := toStorageQuotaDomain(updatedQuota)
	return &result, nil
}

// DeleteFileObjectAndReleaseQuota 删除文件对象并释放配额，可按需要求文件未被活跃会话引用。
func (r *Repo) DeleteFileObjectAndReleaseQuota(
	ctx context.Context,
	userID uint,
	fileID string,
	defaultQuotaBytes int64,
	options repository.DeleteFileObjectOptions,
) (*domainconversation.FileObject, *domainconversation.StorageQuota, bool, error) {
	var deletedFile models.FileObject
	var updatedQuota models.UserStorageQuota
	shouldRemovePhysical := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND file_id = ? AND status = ?", userID, fileID, "active").
			First(&deletedFile).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return translateError(err)
		}

		if options.RequireUnreferenced {
			if err := ensureFileObjectUnreferencedByActiveConversations(tx, userID, fileID); err != nil {
				return err
			}
		}

		quota, err := getOrInitQuotaForUpdate(tx, userID, defaultQuotaBytes)
		if err != nil {
			return translateError(err)
		}

		if err = tx.Model(&models.FileObject{}).
			Where("id = ?", deletedFile.ID).
			Updates(map[string]interface{}{
				"status": "deleted",
			}).Error; err != nil {
			return translateError(err)
		}

		var remainingUserRefs int64
		if err = tx.Model(&models.FileObject{}).
			Where("user_id = ? AND status = ? AND storage_path = ? AND id <> ?",
				userID,
				"active",
				deletedFile.StoragePath,
				deletedFile.ID,
			).
			Count(&remainingUserRefs).Error; err != nil {
			return translateError(err)
		}

		nextUsed := quota.UsedBytes
		if remainingUserRefs == 0 {
			nextUsed = quota.UsedBytes - deletedFile.SizeBytes
			if nextUsed < 0 {
				nextUsed = 0
			}
			if err = tx.Model(&models.UserStorageQuota{}).
				Where("id = ?", quota.ID).
				Update("used_bytes", nextUsed).Error; err != nil {
				return translateError(err)
			}
		}

		var remainingPhysicalRefs int64
		if err = tx.Model(&models.FileObject{}).
			Where("status = ? AND storage_path = ? AND id <> ?", "active", deletedFile.StoragePath, deletedFile.ID).
			Count(&remainingPhysicalRefs).Error; err != nil {
			return translateError(err)
		}
		shouldRemovePhysical = remainingPhysicalRefs == 0

		if err = tx.Where("id = ?", quota.ID).First(&updatedQuota).Error; err != nil {
			return translateError(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, false, translateError(err)
	}

	deleted := toFileObjectDomain(deletedFile)
	quota := toStorageQuotaDomain(updatedQuota)
	return &deleted, &quota, shouldRemovePhysical, nil
}

// GetOrInitUserStorageQuota 查询或初始化用户存储配额。
func (r *Repo) GetOrInitUserStorageQuota(
	ctx context.Context,
	userID uint,
	defaultQuotaBytes int64,
) (*domainconversation.StorageQuota, error) {
	var quota models.UserStorageQuota
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, innerErr := getOrInitQuotaForUpdate(tx, userID, defaultQuotaBytes)
		if innerErr != nil {
			return innerErr
		}
		quota = *item
		return nil
	})
	if err != nil {
		return nil, translateError(err)
	}
	result := toStorageQuotaDomain(quota)
	return &result, nil
}

func getOrInitQuotaForUpdate(tx *gorm.DB, userID uint, defaultQuotaBytes int64) (*models.UserStorageQuota, error) {
	var quota models.UserStorageQuota
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		Limit(1).
		Find(&quota)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		if defaultQuotaBytes < 0 {
			defaultQuotaBytes = 0
		}
		quota = models.UserStorageQuota{
			UserID:        userID,
			QuotaBytes:    defaultQuotaBytes,
			UsedBytes:     0,
			ReservedBytes: 0,
		}
		if err := tx.Select("UserID", "QuotaBytes", "UsedBytes", "ReservedBytes").Create(&quota).Error; err != nil {
			return nil, translateError(err)
		}
	} else {
		if defaultQuotaBytes < 0 {
			defaultQuotaBytes = 0
		}
		if quota.QuotaBytes != defaultQuotaBytes {
			if err := tx.Model(&models.UserStorageQuota{}).
				Where("id = ?", quota.ID).
				Update("quota_bytes", defaultQuotaBytes).Error; err != nil {
				return nil, translateError(err)
			}
			quota.QuotaBytes = defaultQuotaBytes
		}
	}
	return &quota, nil
}

// UpdateFileObjectEmbedStatus 更新文件对象的 embedding 状态及分片数量。
func (r *Repo) UpdateFileObjectEmbedStatus(ctx context.Context, userID uint, fileID string, status string, embedErr string) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND file_id = ?", userID, fileID).
		Updates(map[string]interface{}{
			"embed_status": status,
			"embed_error":  embedErr,
		}).Error)
}

// UpdateFileObjectChunkCount 在 embedding 完成后更新分片数量。
func (r *Repo) UpdateFileObjectChunkCount(ctx context.Context, fileObjID uint, chunkCount int) error {
	return translateError(r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("id = ?", fileObjID).
		Update("chunk_count", chunkCount).Error)
}

// CloneFileEmbeddingArtifacts 复用已完成 embedding 的文件分片到新的逻辑别名文件。
// 若目标环境不支持 embedding 列复制，调用方应回退到重新异步 embedding。
func (r *Repo) CloneFileEmbeddingArtifacts(ctx context.Context, source *domainconversation.FileObject, target *domainconversation.FileObject) error {
	if source == nil || target == nil {
		return nil
	}
	sourceEntity := toFileObjectModel(source)
	targetEntity := toFileObjectModel(target)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.FileObject{}).
			Where("id = ?", targetEntity.ID).
			Updates(map[string]interface{}{
				"embed_status": "ready",
				"embed_error":  "",
				"page_count":   sourceEntity.PageCount,
				"chunk_count":  sourceEntity.ChunkCount,
				"extracted_at": sourceEntity.ExtractedAt,
			}).Error; err != nil {
			return translateError(err)
		}
		if r.sqliteDialect() {
			if err := deleteSQLiteFileChunkVectorsByFile(tx, targetEntity.ID); err != nil {
				return err
			}
		}
		if err := tx.Where("file_obj_id = ?", targetEntity.ID).Delete(&models.FileChunk{}).Error; err != nil {
			return translateError(err)
		}
		if r.sqliteDialect() {
			if err := tx.Exec(
				`INSERT INTO "file_chunks" ("file_obj_id", "user_id", "chunk_index", "page_num", "char_offset", "content", "token_count", "created_at")
				 SELECT ?, ?, "chunk_index", "page_num", "char_offset", "content", "token_count", CURRENT_TIMESTAMP
				 FROM "file_chunks"
				 WHERE "file_obj_id" = ?`,
				targetEntity.ID,
				targetEntity.UserID,
				sourceEntity.ID,
			).Error; err != nil {
				return translateError(err)
			}
			result := tx.Exec(
				fmt.Sprintf(`INSERT INTO %s (chunk_id, user_id, file_obj_id, embedding)
					SELECT target_chunks.id, ?, ?, source_vectors.embedding
					FROM "file_chunks" AS source_chunks
					JOIN "file_chunks" AS target_chunks
						ON target_chunks.file_obj_id = ?
						AND target_chunks.chunk_index = source_chunks.chunk_index
					JOIN %s AS source_vectors
						ON source_vectors.chunk_id = source_chunks.id
					WHERE source_chunks.file_obj_id = ?`,
					sqlitevec.FileChunkVectorTable,
					sqlitevec.FileChunkVectorTable,
				),
				targetEntity.UserID,
				targetEntity.ID,
				targetEntity.ID,
				sourceEntity.ID,
			)
			if err := result.Error; err != nil {
				return translateError(err)
			}
			if sourceEntity.ChunkCount > 0 && result.RowsAffected != int64(sourceEntity.ChunkCount) {
				return fmt.Errorf("sqlite file vector copy mismatch: source_chunks=%d copied_vectors=%d", sourceEntity.ChunkCount, result.RowsAffected)
			}
			return nil
		}
		return tx.Exec(
			`INSERT INTO "file_chunks" ("file_obj_id", "user_id", "chunk_index", "page_num", "char_offset", "content", "token_count", "embedding", "created_at")
			 SELECT ?, ?, "chunk_index", "page_num", "char_offset", "content", "token_count", "embedding", NOW()
			 FROM "file_chunks"
			 WHERE "file_obj_id" = ?`,
			targetEntity.ID,
			targetEntity.UserID,
			sourceEntity.ID,
		).Error
	})
}

// ReplaceFileChunks 替换文件的所有分片（删除旧的，插入新的，并用 raw SQL 更新 embedding）。
func (r *Repo) ReplaceFileChunks(ctx context.Context, fileObjID uint, chunks []domainconversation.FileChunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("embedding count mismatch: chunks=%d embeddings=%d", len(chunks), len(embeddings))
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entities := make([]models.FileChunk, 0, len(chunks))
		for i := range chunks {
			entities = append(entities, toFileChunkModel(&chunks[i]))
		}
		if r.sqliteDialect() {
			if err := deleteSQLiteFileChunkVectorsByFile(tx, fileObjID); err != nil {
				return err
			}
		}
		// 删除旧分片
		if err := tx.Where("file_obj_id = ?", fileObjID).Delete(&models.FileChunk{}).Error; err != nil {
			return translateError(err)
		}
		if len(entities) == 0 {
			return nil
		}
		// 插入新分片
		if err := tx.Create(&entities).Error; err != nil {
			return translateError(err)
		}
		if r.sqliteDialect() {
			return insertSQLiteFileChunkVectors(tx, entities, embeddings)
		}
		// 更新 embedding（通过 raw SQL 写入 vector 值）
		for i, chunk := range entities {
			if len(embeddings[i]) == 0 {
				return fmt.Errorf("empty embedding vector at chunk %d", i)
			}
			vec := float32SliceToPostgresVector(embeddings[i])
			if err := tx.Exec(
				`UPDATE "file_chunks" SET embedding = ? WHERE id = ?`,
				vec, chunk.ID,
			).Error; err != nil {
				return translateError(err)
			}
		}
		return nil
	})
}

// GetFirstActiveUpstream 查询第一个激活的上游（用于 embedding API）。
func (r *Repo) GetFirstActiveUpstream(ctx context.Context) (*models.LLMUpstream, error) {
	var item models.LLMUpstream
	if err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Order("id ASC").
		Limit(1).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, translateError(err)
	}
	return &item, nil
}

// GetUpstreamByID 按 ID 查询上游。
func (r *Repo) GetUpstreamByID(ctx context.Context, upstreamID uint) (*models.LLMUpstream, error) {
	var item models.LLMUpstream
	if err := r.db.WithContext(ctx).
		Where("id = ?", upstreamID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, translateError(err)
	}
	return &item, nil
}

// float32SliceToPostgresVector 将 []float32 转为 pgvector 文本格式 "[1.0,2.0,...]"。
func float32SliceToPostgresVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}

// fileChunkSearchRow 是原始 SQL 扫描专用的本地类型，携带 gorm column tag 映射相似度列。
type fileChunkSearchRow struct {
	ID         uint      `gorm:"column:id"`
	FileObjID  uint      `gorm:"column:file_obj_id"`
	UserID     uint      `gorm:"column:user_id"`
	ChunkIndex int       `gorm:"column:chunk_index"`
	PageNum    int       `gorm:"column:page_num"`
	CharOffset int       `gorm:"column:char_offset"`
	Content    string    `gorm:"column:content"`
	TokenCount int       `gorm:"column:token_count"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	Similarity float32   `gorm:"column:similarity"`
}

func deleteSQLiteFileChunkVectorsByFile(tx *gorm.DB, fileObjID uint) error {
	return translateError(tx.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE chunk_id IN (
			SELECT id FROM "file_chunks" WHERE file_obj_id = ?
		)`, sqlitevec.FileChunkVectorTable),
		fileObjID,
	).Error)
}

func insertSQLiteFileChunkVectors(tx *gorm.DB, entities []models.FileChunk, embeddings [][]float32) error {
	if len(entities) != len(embeddings) {
		return fmt.Errorf("embedding count mismatch: chunks=%d embeddings=%d", len(entities), len(embeddings))
	}
	for i, chunk := range entities {
		if len(embeddings[i]) == 0 {
			return fmt.Errorf("empty embedding vector at chunk %d", i)
		}
		vector, err := sqlitevec.SerializeFloat32(embeddings[i])
		if err != nil {
			return err
		}
		if err = tx.Exec(
			fmt.Sprintf(`INSERT INTO %s (chunk_id, user_id, file_obj_id, embedding) VALUES (?, ?, ?, ?)`, sqlitevec.FileChunkVectorTable),
			chunk.ID,
			chunk.UserID,
			chunk.FileObjID,
			vector,
		).Error; err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r *Repo) searchSQLiteFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, queryEmbedding []float32, topK int) ([]domainconversation.FileChunkSearchResult, error) {
	vector, err := sqlitevec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, err
	}
	results := make([]domainconversation.FileChunkSearchResult, 0, topK)
	seenFileObjIDs := make(map[uint]struct{}, len(fileObjIDs))
	for _, fileObjID := range fileObjIDs {
		if _, ok := seenFileObjIDs[fileObjID]; ok {
			continue
		}
		seenFileObjIDs[fileObjID] = struct{}{}
		var rows []fileChunkSearchRow
		query := fmt.Sprintf(`
			SELECT chunks.id, chunks.file_obj_id, chunks.user_id, chunks.chunk_index, chunks.page_num,
			       chunks.char_offset, chunks.content, chunks.token_count, chunks.created_at,
			       (1.0 - vectors.distance) AS similarity
			FROM %s AS vectors
			JOIN "file_chunks" AS chunks
				ON chunks.id = vectors.chunk_id
			WHERE vectors.embedding MATCH ?
				AND vectors.k = ?
				AND vectors.user_id = ?
				AND vectors.file_obj_id = ?
			ORDER BY vectors.distance ASC`,
			sqlitevec.FileChunkVectorTable,
		)
		if err := r.db.WithContext(ctx).Raw(query, vector, topK, userID, fileObjID).Scan(&rows).Error; err != nil {
			return nil, translateError(err)
		}
		for _, row := range rows {
			results = append(results, domainconversation.FileChunkSearchResult{
				FileChunk: domainconversation.FileChunk{
					ID:         row.ID,
					FileObjID:  row.FileObjID,
					UserID:     row.UserID,
					ChunkIndex: row.ChunkIndex,
					PageNum:    row.PageNum,
					CharOffset: row.CharOffset,
					Content:    row.Content,
					TokenCount: row.TokenCount,
					CreatedAt:  row.CreatedAt,
				},
				Similarity: row.Similarity,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// SearchFileChunks 使用向量存储的余弦距离检索最相关的文本分片。
// 返回结果按相似度降序排列，已携带 Similarity 分数以供阈值过滤。
func (r *Repo) SearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, queryEmbedding []float32, topK int) ([]domainconversation.FileChunkSearchResult, error) {
	if len(fileObjIDs) == 0 || len(queryEmbedding) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}
	if r.sqliteDialect() {
		return r.searchSQLiteFileChunks(ctx, userID, fileObjIDs, queryEmbedding, topK)
	}
	vec := float32SliceToPostgresVector(queryEmbedding)
	query := `
		SELECT id, file_obj_id, user_id, chunk_index, page_num, char_offset, content, token_count, created_at,
		       (1 - (embedding <=> ?::vector)) AS similarity
		FROM file_chunks
		WHERE user_id = ? AND file_obj_id IN ? AND embedding IS NOT NULL
		ORDER BY similarity DESC
		LIMIT ?`
	var rows []fileChunkSearchRow
	if err := r.db.WithContext(ctx).Raw(query, vec, userID, fileObjIDs, topK).Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainconversation.FileChunkSearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, domainconversation.FileChunkSearchResult{
			FileChunk: domainconversation.FileChunk{
				ID:         row.ID,
				FileObjID:  row.FileObjID,
				UserID:     row.UserID,
				ChunkIndex: row.ChunkIndex,
				PageNum:    row.PageNum,
				CharOffset: row.CharOffset,
				Content:    row.Content,
				TokenCount: row.TokenCount,
				CreatedAt:  row.CreatedAt,
			},
			Similarity: row.Similarity,
		})
	}
	return results, nil
}

// BM25SearchFileChunks 使用 PostgreSQL tsvector 全文检索文件分片，中文字符以空格切字作为后备分词策略。
// 返回结果按 ts_rank 降序，Similarity 字段存放归一化后的排名得分（0-1）。
func (r *Repo) BM25SearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, query string, topK int) ([]domainconversation.FileChunkSearchResult, error) {
	if len(fileObjIDs) == 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}
	if r.sqliteDialect() {
		return r.keywordSearchFileChunks(ctx, userID, fileObjIDs, query, topK)
	}
	// 中文字符逐字切开，空格分隔后拼成 OR 查询，提高中文召回率
	tsQuery := buildTSQuery(query)
	if tsQuery == "" {
		return nil, nil
	}
	rawQuery := `
		SELECT id, file_obj_id, user_id, chunk_index, page_num, char_offset, content, token_count, created_at,
		       ts_rank(to_tsvector('simple', content), to_tsquery('simple', ?)) AS similarity
		FROM file_chunks
		WHERE user_id = ? AND file_obj_id IN ?
		  AND to_tsvector('simple', content) @@ to_tsquery('simple', ?)
		ORDER BY similarity DESC
		LIMIT ?`
	var rows []fileChunkSearchRow
	if err := r.db.WithContext(ctx).Raw(rawQuery, tsQuery, userID, fileObjIDs, tsQuery, topK).Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainconversation.FileChunkSearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, domainconversation.FileChunkSearchResult{
			FileChunk: domainconversation.FileChunk{
				ID:         row.ID,
				FileObjID:  row.FileObjID,
				UserID:     row.UserID,
				ChunkIndex: row.ChunkIndex,
				PageNum:    row.PageNum,
				CharOffset: row.CharOffset,
				Content:    row.Content,
				TokenCount: row.TokenCount,
				CreatedAt:  row.CreatedAt,
			},
			Similarity: row.Similarity,
		})
	}
	return results, nil
}

func (r *Repo) keywordSearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, query string, topK int) ([]domainconversation.FileChunkSearchResult, error) {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		terms = []string{strings.ToLower(strings.TrimSpace(query))}
	}
	dbq := r.db.WithContext(ctx).
		Model(&models.FileChunk{}).
		Where("user_id = ? AND file_obj_id IN ?", userID, fileObjIDs)
	for _, term := range terms {
		if strings.TrimSpace(term) == "" {
			continue
		}
		dbq = dbq.Where("LOWER(content) LIKE ?", "%"+term+"%")
	}
	rows := make([]models.FileChunk, 0, topK)
	if err := dbq.Order("id ASC").Limit(topK).Find(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainconversation.FileChunkSearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, domainconversation.FileChunkSearchResult{
			FileChunk: domainconversation.FileChunk{
				ID:         row.ID,
				FileObjID:  row.FileObjID,
				UserID:     row.UserID,
				ChunkIndex: row.ChunkIndex,
				PageNum:    row.PageNum,
				CharOffset: row.CharOffset,
				Content:    row.Content,
				TokenCount: row.TokenCount,
				CreatedAt:  row.CreatedAt,
			},
			Similarity: 0.5,
		})
	}
	return results, nil
}

// buildTSQuery 将查询字符串转换为 PostgreSQL tsquery 格式。
// 中文字符逐字展开，ASCII 单词保留，用 | 连接（OR 语义）。
func buildTSQuery(query string) string {
	var tokens []string
	var wordBuf strings.Builder
	for _, r := range strings.TrimSpace(query) {
		if r > 0x2E7F { // CJK 及更宽字符：单字为 token
			if wordBuf.Len() > 0 {
				tokens = append(tokens, wordBuf.String())
				wordBuf.Reset()
			}
			tokens = append(tokens, string(r))
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			wordBuf.WriteRune(r)
		} else {
			if wordBuf.Len() > 0 {
				tokens = append(tokens, wordBuf.String())
				wordBuf.Reset()
			}
		}
	}
	if wordBuf.Len() > 0 {
		tokens = append(tokens, wordBuf.String())
	}
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " | ")
}

// GetUserSettingValue 查询指定用户的单个配置值，不存在时返回 ""。
func (r *Repo) GetUserSettingValue(ctx context.Context, userID uint, key string) (string, error) {
	var item models.UserSetting
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND key = ?", userID, key).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", translateError(err)
	}
	return item.Value, nil
}

// GetFileObjectsByInternalIDs 按内部主键 ID 批量查询文件对象。
func (r *Repo) GetFileObjectsByInternalIDs(ctx context.Context, userID uint, ids []uint) ([]domainconversation.FileObject, error) {
	items := make([]models.FileObject, 0)
	if len(ids) == 0 {
		return []domainconversation.FileObject{}, nil
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND id IN ?", userID, ids).
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toFileObjectDomains(items), nil
}

func (r *Repo) hydrateMessageRefs(ctx context.Context, items []models.Message) error {
	if len(items) == 0 {
		return nil
	}

	publicIDs := make(map[uint]string, len(items))
	for i := range items {
		publicIDs[items[i].ID] = items[i].PublicID
	}

	missingIDs := make(map[uint]struct{})
	for i := range items {
		if items[i].ParentMessageID != nil {
			if _, ok := publicIDs[*items[i].ParentMessageID]; !ok {
				missingIDs[*items[i].ParentMessageID] = struct{}{}
			}
		}
		if items[i].SourceMessageID != nil {
			if _, ok := publicIDs[*items[i].SourceMessageID]; !ok {
				missingIDs[*items[i].SourceMessageID] = struct{}{}
			}
		}
	}

	if len(missingIDs) > 0 {
		ids := make([]uint, 0, len(missingIDs))
		for id := range missingIDs {
			ids = append(ids, id)
		}
		refs := make([]models.Message, 0, len(ids))
		if err := r.db.WithContext(ctx).
			Select("id", "public_id").
			Where("id IN ?", ids).
			Find(&refs).Error; err != nil {
			return translateError(err)
		}
		for i := range refs {
			publicIDs[refs[i].ID] = refs[i].PublicID
		}
	}

	for i := range items {
		if items[i].ParentMessageID != nil {
			items[i].ParentPublicID = publicIDs[*items[i].ParentMessageID]
		}
		if items[i].SourceMessageID != nil {
			items[i].SourcePublicID = publicIDs[*items[i].SourceMessageID]
		}
	}
	return nil
}

type messageAttachmentSnapshotRow struct {
	MessageID              uint   `gorm:"column:message_id"`
	FileID                 string `gorm:"column:file_id"`
	Kind                   string `gorm:"column:kind"`
	FileName               string `gorm:"column:file_name"`
	MimeType               string `gorm:"column:mime_type"`
	DetectedMIME           string `gorm:"column:detected_mime"`
	FileCategory           string `gorm:"column:file_category"`
	FileSize               int64  `gorm:"column:file_size"`
	ProcessingStatus       string `gorm:"column:processing_status"`
	ProcessingReady        bool   `gorm:"column:processing_ready"`
	ProcessingErrorCode    string `gorm:"column:processing_error_code"`
	ProcessingErrorMessage string `gorm:"column:processing_error_message"`
}

func (r *Repo) hydrateMessageAttachments(ctx context.Context, items []models.Message) error {
	if len(items) == 0 {
		return nil
	}
	messageIDs := make([]uint, 0, len(items))
	for i := range items {
		items[i].Attachments = "[]"
		if items[i].ID != 0 {
			messageIDs = append(messageIDs, items[i].ID)
		}
	}
	if len(messageIDs) == 0 {
		return nil
	}

	rows := make([]messageAttachmentSnapshotRow, 0)
	if err := r.db.WithContext(ctx).
		Table("chat_attachments AS a").
		Select(strings.Join([]string{
			"a.message_id",
			"a.file_id",
			"a.kind",
			"a.file_name",
			"a.mime_type",
			"a.file_size",
			"fo.detected_mime",
			"fo.file_category",
			"fo.processing_status",
			"fo.processing_ready",
			"fo.processing_error_code",
			"fo.processing_error_message",
		}, ", ")).
		Joins("LEFT JOIN file_objects fo ON fo.file_id = a.file_id AND fo.user_id = a.user_id").
		Where("a.message_id IN ? AND a.status <> ?", messageIDs, "deleted").
		Order("a.id ASC").
		Scan(&rows).Error; err != nil {
		return translateError(err)
	}

	grouped := make(map[uint][]map[string]interface{}, len(rows))
	for _, row := range rows {
		grouped[row.MessageID] = append(grouped[row.MessageID], map[string]interface{}{
			"file_id":                  row.FileID,
			"kind":                     row.Kind,
			"file_name":                row.FileName,
			"mime_type":                row.MimeType,
			"detected_mime":            row.DetectedMIME,
			"file_category":            row.FileCategory,
			"file_size":                row.FileSize,
			"processing_status":        row.ProcessingStatus,
			"processing_ready":         row.ProcessingReady,
			"processing_error_code":    row.ProcessingErrorCode,
			"processing_error_message": row.ProcessingErrorMessage,
		})
	}
	for i := range items {
		payload := grouped[items[i].ID]
		if len(payload) == 0 {
			continue
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			items[i].Attachments = "[]"
			continue
		}
		items[i].Attachments = string(raw)
	}
	return nil
}

func toConversationDomain(item models.Conversation) domainconversation.Conversation {
	labelsJSON := strings.TrimSpace(item.LabelsJSON)
	if labelsJSON == "" {
		labelsJSON = "[]"
	}
	return domainconversation.Conversation{
		ID:                    item.ID,
		UserID:                item.UserID,
		ProjectID:             item.ProjectID,
		PublicID:              item.PublicID,
		Title:                 item.Title,
		LabelsJSON:            labelsJSON,
		Model:                 item.Model,
		Provider:              item.Provider,
		SessionKey:            item.SessionKey,
		IsStarred:             item.IsStarred,
		StarredAt:             item.StarredAt,
		MessageCount:          item.MessageCount,
		Status:                item.Status,
		ContextPolicy:         item.ContextPolicy,
		LastCompactedAt:       item.LastCompactedAt,
		LastResponseID:        item.LastResponseID,
		LastPromptFingerprint: item.LastPromptFingerprint,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func toConversationDomains(items []models.Conversation) []domainconversation.Conversation {
	results := make([]domainconversation.Conversation, 0, len(items))
	for _, item := range items {
		results = append(results, toConversationDomain(item))
	}
	return results
}

func toConversationShareDomain(item models.ConversationShare) domainconversation.ConversationShare {
	return domainconversation.ConversationShare{
		ID:                    item.ID,
		ShareID:               item.ShareID,
		ConversationID:        item.ConversationID,
		UserID:                item.UserID,
		Status:                item.Status,
		TitleSnapshot:         item.TitleSnapshot,
		ModelSnapshot:         item.ModelSnapshot,
		MessageIDsJSON:        item.MessageIDsJSON,
		DefaultMessageIDsJSON: item.DefaultMessageIDsJSON,
		RevokedAt:             item.RevokedAt,
		RegeneratedAt:         item.RegeneratedAt,
		LastAccessedAt:        item.LastAccessedAt,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func toConversationModel(item *domainconversation.Conversation) models.Conversation {
	if item == nil {
		return models.Conversation{}
	}
	labelsJSON := strings.TrimSpace(item.LabelsJSON)
	if labelsJSON == "" {
		labelsJSON = "[]"
	}
	return models.Conversation{
		UserID:                item.UserID,
		ProjectID:             item.ProjectID,
		PublicID:              item.PublicID,
		Title:                 item.Title,
		LabelsJSON:            labelsJSON,
		Model:                 item.Model,
		Provider:              item.Provider,
		SessionKey:            item.SessionKey,
		IsStarred:             item.IsStarred,
		StarredAt:             item.StarredAt,
		MessageCount:          item.MessageCount,
		Status:                item.Status,
		ContextPolicy:         item.ContextPolicy,
		LastCompactedAt:       item.LastCompactedAt,
		LastResponseID:        item.LastResponseID,
		LastPromptFingerprint: item.LastPromptFingerprint,
	}
}

func toConversationShareModel(item *domainconversation.ConversationShare) models.ConversationShare {
	if item == nil {
		return models.ConversationShare{}
	}
	return models.ConversationShare{
		ShareID:               item.ShareID,
		ConversationID:        item.ConversationID,
		UserID:                item.UserID,
		Status:                item.Status,
		TitleSnapshot:         item.TitleSnapshot,
		ModelSnapshot:         item.ModelSnapshot,
		MessageIDsJSON:        item.MessageIDsJSON,
		DefaultMessageIDsJSON: item.DefaultMessageIDsJSON,
		RevokedAt:             item.RevokedAt,
		RegeneratedAt:         item.RegeneratedAt,
		LastAccessedAt:        item.LastAccessedAt,
	}
}

func toUserDomain(item models.User) domainuser.User {
	return domainuser.User{
		ID:                    item.ID,
		PublicID:              item.PublicID,
		Username:              item.Username,
		DisplayName:           item.DisplayName,
		AvatarURL:             item.AvatarURL,
		Email:                 item.Email,
		Phone:                 item.Phone,
		Role:                  item.Role,
		Status:                item.Status,
		Timezone:              item.Timezone,
		Locale:                item.Locale,
		ProfilePreferences:    item.ProfilePreferences,
		AppearancePreferences: item.AppearancePreferences,
		EmailVerifiedAt:       item.EmailVerifiedAt,
		PhoneVerifiedAt:       item.PhoneVerifiedAt,
		LastLoginAt:           item.LastLoginAt,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func toMessageDomain(item models.Message) domainconversation.Message {
	return domainconversation.Message{
		ID:               item.ID,
		ConversationID:   item.ConversationID,
		UserID:           item.UserID,
		PublicID:         item.PublicID,
		ParentMessageID:  item.ParentMessageID,
		RunID:            item.RunID,
		Role:             item.Role,
		ContentType:      item.ContentType,
		Content:          item.Content,
		BranchReason:     item.BranchReason,
		SourceMessageID:  item.SourceMessageID,
		TokenUsage:       item.TokenUsage,
		InputTokens:      item.InputTokens,
		OutputTokens:     item.OutputTokens,
		CacheReadTokens:  item.CacheReadTokens,
		CacheWriteTokens: item.CacheWriteTokens,
		ReasoningTokens:  item.ReasoningTokens,
		LatencyMS:        item.LatencyMS,
		BilledCurrency:   item.BilledCurrency,
		BilledNanousd:    item.BilledNanousd,
		PricingSnapshot:  item.PricingSnapshot,
		Status:           item.Status,
		ErrorCode:        item.ErrorCode,
		ErrorMessage:     item.ErrorMessage,
		Attachments:      item.Attachments,
		ParentPublicID:   item.ParentPublicID,
		SourcePublicID:   item.SourcePublicID,
		MyFeedback:       item.MyFeedback,
		ThumbsUpCount:    item.ThumbsUpCount,
		ThumbsDownCount:  item.ThumbsDownCount,
		EditedAt:         item.EditedAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toMessageDomains(items []models.Message) []domainconversation.Message {
	results := make([]domainconversation.Message, 0, len(items))
	for _, item := range items {
		results = append(results, toMessageDomain(item))
	}
	return results
}

func toMessageModel(item *domainconversation.Message) models.Message {
	if item == nil {
		return models.Message{}
	}
	return models.Message{
		ConversationID:   item.ConversationID,
		UserID:           item.UserID,
		PublicID:         item.PublicID,
		ParentMessageID:  item.ParentMessageID,
		RunID:            item.RunID,
		Role:             item.Role,
		ContentType:      item.ContentType,
		Content:          item.Content,
		BranchReason:     item.BranchReason,
		SourceMessageID:  item.SourceMessageID,
		TokenUsage:       item.TokenUsage,
		InputTokens:      item.InputTokens,
		OutputTokens:     item.OutputTokens,
		CacheReadTokens:  item.CacheReadTokens,
		CacheWriteTokens: item.CacheWriteTokens,
		ReasoningTokens:  item.ReasoningTokens,
		LatencyMS:        item.LatencyMS,
		BilledCurrency:   item.BilledCurrency,
		BilledNanousd:    item.BilledNanousd,
		PricingSnapshot:  item.PricingSnapshot,
		Status:           item.Status,
		ErrorCode:        item.ErrorCode,
		ErrorMessage:     item.ErrorMessage,
		EditedAt:         item.EditedAt,
	}
}

func toMessageFeedbackModel(item *domainconversation.MessageFeedback) models.ConversationMessageFeedback {
	if item == nil {
		return models.ConversationMessageFeedback{}
	}
	return models.ConversationMessageFeedback{
		UserID:         item.UserID,
		ConversationID: item.ConversationID,
		MessageID:      item.MessageID,
		Feedback:       item.Feedback,
	}
}

func toAttachmentModel(item *domainconversation.Attachment) models.Attachment {
	if item == nil {
		return models.Attachment{}
	}
	return models.Attachment{
		ConversationID: item.ConversationID,
		MessageID:      item.MessageID,
		UserID:         item.UserID,
		FileID:         item.FileID,
		Kind:           item.Kind,
		FileName:       item.FileName,
		MimeType:       item.MimeType,
		FileSize:       item.FileSize,
		SHA256:         item.SHA256,
		StoragePath:    item.StoragePath,
		Status:         item.Status,
		MetaJSON:       item.MetaJSON,
		UploadedAt:     item.UploadedAt,
	}
}

func toConversationRunDomain(item models.ConversationRun) domainconversation.Run {
	return domainconversation.Run{
		ID:                  item.ID,
		RunID:               item.RunID,
		RequestID:           item.RequestID,
		UserID:              item.UserID,
		ConversationID:      item.ConversationID,
		TaskType:            item.TaskType,
		Endpoint:            item.Endpoint,
		Provider:            item.Provider,
		ProviderProtocol:    item.ProviderProtocol,
		UpstreamID:          item.UpstreamID,
		UpstreamModelID:     item.UpstreamModelID,
		UpstreamName:        item.UpstreamName,
		RequestedModelName:  item.RequestedModelName,
		PlatformModelName:   item.PlatformModelName,
		RoutedBindingCode:   item.RoutedBindingCode,
		ModelVendor:         item.ModelVendor,
		ModelIcon:           item.ModelIcon,
		UpstreamModelName:   item.UpstreamModelName,
		InputTokens:         item.InputTokens,
		OutputTokens:        item.OutputTokens,
		CacheReadTokens:     item.CacheReadTokens,
		CacheWriteTokens:    item.CacheWriteTokens,
		ReasoningTokens:     item.ReasoningTokens,
		ToolCallsCount:      item.ToolCallsCount,
		FirstTokenLatencyMS: item.FirstTokenLatencyMS,
		TotalLatencyMS:      item.TotalLatencyMS,
		Status:              item.Status,
		ErrorCode:           item.ErrorCode,
		ErrorMessage:        item.ErrorMessage,
		StartedAt:           item.StartedAt,
		EndedAt:             item.EndedAt,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func toConversationRunDomains(items []models.ConversationRun) []domainconversation.Run {
	results := make([]domainconversation.Run, 0, len(items))
	for _, item := range items {
		results = append(results, toConversationRunDomain(item))
	}
	return results
}

func toConversationRunModel(item *domainconversation.Run) models.ConversationRun {
	if item == nil {
		return models.ConversationRun{}
	}
	return models.ConversationRun{
		RunID:               item.RunID,
		RequestID:           item.RequestID,
		UserID:              item.UserID,
		ConversationID:      item.ConversationID,
		TaskType:            item.TaskType,
		Endpoint:            item.Endpoint,
		Provider:            item.Provider,
		ProviderProtocol:    item.ProviderProtocol,
		UpstreamID:          item.UpstreamID,
		UpstreamModelID:     item.UpstreamModelID,
		UpstreamName:        item.UpstreamName,
		RequestedModelName:  item.RequestedModelName,
		PlatformModelName:   item.PlatformModelName,
		RoutedBindingCode:   item.RoutedBindingCode,
		ModelVendor:         item.ModelVendor,
		ModelIcon:           item.ModelIcon,
		UpstreamModelName:   item.UpstreamModelName,
		InputTokens:         item.InputTokens,
		OutputTokens:        item.OutputTokens,
		CacheReadTokens:     item.CacheReadTokens,
		CacheWriteTokens:    item.CacheWriteTokens,
		ReasoningTokens:     item.ReasoningTokens,
		ToolCallsCount:      item.ToolCallsCount,
		FirstTokenLatencyMS: item.FirstTokenLatencyMS,
		TotalLatencyMS:      item.TotalLatencyMS,
		Status:              item.Status,
		ErrorCode:           item.ErrorCode,
		ErrorMessage:        item.ErrorMessage,
		StartedAt:           item.StartedAt,
		EndedAt:             item.EndedAt,
	}
}

func toConversationMessageTraceDomains(items []models.ChatRunEvent) []domainconversation.MessageTrace {
	results := make([]domainconversation.MessageTrace, 0, len(items))
	for _, item := range items {
		results = append(results, domainconversation.MessageTrace{
			ID:              item.ID,
			MessageID:       item.MessageID,
			ConversationID:  item.ConversationID,
			UserID:          item.UserID,
			RunID:           item.RunID,
			TraceType:       item.EventType,
			Status:          item.Status,
			Stage:           item.Stage,
			RoundID:         item.RoundID,
			ParentEventID:   item.ParentEventID,
			Title:           item.Title,
			Summary:         item.Summary,
			ContentMarkdown: item.ContentMarkdown,
			PayloadJSON:     item.PayloadJSON,
			Seq:             item.Seq,
			StartedAt:       item.StartedAt,
			EndedAt:         item.EndedAt,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}
	return results
}

func toConversationMessageTraceModel(item *domainconversation.MessageTrace) models.ChatRunEvent {
	if item == nil {
		return models.ChatRunEvent{}
	}
	startedAt := item.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return models.ChatRunEvent{
		MessageID:       item.MessageID,
		ConversationID:  item.ConversationID,
		UserID:          item.UserID,
		RunID:           item.RunID,
		EventScope:      chatRunEventScopeTraceBlock,
		EventID:         fmt.Sprintf("message:%d:%s", item.MessageID, item.TraceType),
		EventType:       item.TraceType,
		Phase:           item.TraceType,
		Status:          item.Status,
		Stage:           item.Stage,
		RoundID:         item.RoundID,
		ParentEventID:   item.ParentEventID,
		Title:           item.Title,
		Summary:         item.Summary,
		ContentMarkdown: item.ContentMarkdown,
		PayloadJSON:     item.PayloadJSON,
		Seq:             item.Seq,
		StartedAt:       startedAt,
		EndedAt:         item.EndedAt,
	}
}

func toConversationMessageTraceEventDomains(items []models.ChatRunEvent) []domainconversation.MessageTraceEventRow {
	results := make([]domainconversation.MessageTraceEventRow, 0, len(items))
	for _, item := range items {
		results = append(results, domainconversation.MessageTraceEventRow{
			ID:              item.ID,
			MessageID:       item.MessageID,
			ConversationID:  item.ConversationID,
			UserID:          item.UserID,
			RunID:           item.RunID,
			EventID:         item.EventID,
			EventType:       item.EventType,
			Phase:           item.Phase,
			Stage:           item.Stage,
			RoundID:         item.RoundID,
			ParentEventID:   item.ParentEventID,
			Status:          item.Status,
			Title:           item.Title,
			Summary:         item.Summary,
			ContentMarkdown: item.ContentMarkdown,
			PayloadJSON:     item.PayloadJSON,
			Seq:             item.Seq,
			StartedAt:       item.StartedAt,
			EndedAt:         item.EndedAt,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}
	return results
}

func toConversationMessageTraceEventModel(item *domainconversation.MessageTraceEventRow) models.ChatRunEvent {
	if item == nil {
		return models.ChatRunEvent{}
	}
	startedAt := item.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return models.ChatRunEvent{
		MessageID:       item.MessageID,
		ConversationID:  item.ConversationID,
		UserID:          item.UserID,
		RunID:           item.RunID,
		EventScope:      chatRunEventScopeTraceEvent,
		EventID:         item.EventID,
		EventType:       item.EventType,
		Phase:           item.Phase,
		Stage:           item.Stage,
		RoundID:         item.RoundID,
		ParentEventID:   item.ParentEventID,
		Status:          item.Status,
		Title:           item.Title,
		Summary:         item.Summary,
		ContentMarkdown: item.ContentMarkdown,
		PayloadJSON:     item.PayloadJSON,
		Seq:             item.Seq,
		StartedAt:       startedAt,
		EndedAt:         item.EndedAt,
	}
}

func toConversationToolCallModel(item *domainconversation.ToolCall) models.ChatRunEvent {
	if item == nil {
		return models.ChatRunEvent{}
	}
	startedAt := item.CreatedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	eventID := strings.TrimSpace(item.ToolCallID)
	if eventID == "" {
		eventID = fmt.Sprintf("tool:%s:%d", strings.TrimSpace(item.ToolName), time.Now().UnixNano())
	}
	return models.ChatRunEvent{
		MessageID:      item.MessageID,
		ConversationID: item.ConversationID,
		UserID:         item.UserID,
		RunID:          item.RunID,
		EventScope:     chatRunEventScopeToolCall,
		EventID:        eventID,
		EventType:      item.ToolType,
		ToolCallID:     item.ToolCallID,
		ToolName:       item.ToolName,
		Status:         item.Status,
		LatencyMS:      item.LatencyMS,
		InputJSON:      item.InputJSON,
		OutputJSON:     item.OutputJSON,
		ErrorJSON:      item.ErrorJSON,
		StartedAt:      startedAt,
	}
}

func toContextSnapshotDomain(item models.ChatContextRecord) domainconversation.ContextSnapshot {
	return domainconversation.ContextSnapshot{
		ID:             item.ID,
		ConversationID: item.ConversationID,
		MessageID:      item.MessageID,
		UserID:         item.UserID,
		RunID:          item.RunID,
		FromTurn:       item.FromTurn,
		ToTurn:         item.ToTurn,
		SourceTokens:   item.SourceTokens,
		SummaryTokens:  item.SummaryTokens,
		SummaryText:    item.SummaryText,
		Strategy:       item.Strategy,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toContextSnapshotModel(item *domainconversation.ContextSnapshot) models.ChatContextRecord {
	if item == nil {
		return models.ChatContextRecord{}
	}
	return models.ChatContextRecord{
		RecordType:     chatContextRecordSnapshot,
		ConversationID: item.ConversationID,
		MessageID:      item.MessageID,
		UserID:         item.UserID,
		RunID:          item.RunID,
		FromTurn:       item.FromTurn,
		ToTurn:         item.ToTurn,
		SourceTokens:   item.SourceTokens,
		SummaryTokens:  item.SummaryTokens,
		SummaryText:    item.SummaryText,
		Strategy:       item.Strategy,
	}
}

func toFileObjectDomain(item models.FileObject) domainconversation.FileObject {
	return domainconversation.FileObject{
		ID:                     item.ID,
		FileID:                 item.FileID,
		UserID:                 item.UserID,
		Purpose:                item.Purpose,
		FileName:               item.FileName,
		MimeType:               item.MimeType,
		DetectedMIME:           item.DetectedMIME,
		FileCategory:           item.FileCategory,
		SizeBytes:              item.SizeBytes,
		SHA256:                 item.SHA256,
		StoragePath:            item.StoragePath,
		Status:                 item.Status,
		LastAccessedAt:         item.LastAccessedAt,
		ExpiresAt:              item.ExpiresAt,
		ProcessingStatus:       item.ProcessingStatus,
		ProcessingReady:        item.ProcessingReady,
		ProcessingErrorCode:    item.ProcessingErrorCode,
		ProcessingErrorMessage: item.ProcessingErrorMessage,
		ExtractStatus:          item.ExtractStatus,
		ExtractEngine:          item.ExtractEngine,
		ExtractStoragePath:     item.ExtractStoragePath,
		ExtractChars:           item.ExtractChars,
		ExtractPages:           item.ExtractPages,
		PreviewText:            item.PreviewText,
		OCRUsed:                item.OCRUsed,
		RAGReady:               item.RAGReady,
		RAGReason:              item.RAGReason,
		EmbedStatus:            item.EmbedStatus,
		EmbedError:             item.EmbedError,
		PageCount:              item.PageCount,
		ChunkCount:             item.ChunkCount,
		ExtractorVersion:       item.ExtractorVersion,
		ExtractedAt:            item.ExtractedAt,
		ProcessingPayloadJSON:  item.ProcessingPayloadJSON,
		ProcessingStartedAt:    item.ProcessingStartedAt,
		ProcessingCompletedAt:  item.ProcessingCompletedAt,
		RagOptOut:              item.RagOptOut,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
	}
}

func toFileObjectDomains(items []models.FileObject) []domainconversation.FileObject {
	results := make([]domainconversation.FileObject, 0, len(items))
	for _, item := range items {
		results = append(results, toFileObjectDomain(item))
	}
	return results
}

func toFileObjectModel(item *domainconversation.FileObject) models.FileObject {
	if item == nil {
		return models.FileObject{}
	}
	return models.FileObject{
		BaseModel:              models.BaseModel{ID: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt},
		FileID:                 item.FileID,
		UserID:                 item.UserID,
		Purpose:                item.Purpose,
		FileName:               item.FileName,
		MimeType:               item.MimeType,
		DetectedMIME:           item.DetectedMIME,
		FileCategory:           item.FileCategory,
		SizeBytes:              item.SizeBytes,
		SHA256:                 item.SHA256,
		StoragePath:            item.StoragePath,
		Status:                 item.Status,
		LastAccessedAt:         item.LastAccessedAt,
		ExpiresAt:              item.ExpiresAt,
		ProcessingStatus:       item.ProcessingStatus,
		ProcessingReady:        item.ProcessingReady,
		ProcessingErrorCode:    item.ProcessingErrorCode,
		ProcessingErrorMessage: item.ProcessingErrorMessage,
		ExtractStatus:          item.ExtractStatus,
		ExtractEngine:          item.ExtractEngine,
		ExtractStoragePath:     item.ExtractStoragePath,
		ExtractChars:           item.ExtractChars,
		ExtractPages:           item.ExtractPages,
		PreviewText:            item.PreviewText,
		OCRUsed:                item.OCRUsed,
		RAGReady:               item.RAGReady,
		RAGReason:              item.RAGReason,
		EmbedStatus:            item.EmbedStatus,
		EmbedError:             item.EmbedError,
		PageCount:              item.PageCount,
		ChunkCount:             item.ChunkCount,
		ExtractorVersion:       item.ExtractorVersion,
		ExtractedAt:            item.ExtractedAt,
		ProcessingPayloadJSON:  item.ProcessingPayloadJSON,
		ProcessingStartedAt:    item.ProcessingStartedAt,
		ProcessingCompletedAt:  item.ProcessingCompletedAt,
		RagOptOut:              item.RagOptOut,
	}
}

func toStorageQuotaDomain(item models.UserStorageQuota) domainconversation.StorageQuota {
	return domainconversation.StorageQuota{
		ID:            item.ID,
		UserID:        item.UserID,
		QuotaBytes:    item.QuotaBytes,
		UsedBytes:     item.UsedBytes,
		ReservedBytes: item.ReservedBytes,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toFileChunkModel(item *domainconversation.FileChunk) models.FileChunk {
	if item == nil {
		return models.FileChunk{}
	}
	return models.FileChunk{
		FileObjID:  item.FileObjID,
		UserID:     item.UserID,
		ChunkIndex: item.ChunkIndex,
		PageNum:    item.PageNum,
		CharOffset: item.CharOffset,
		Content:    item.Content,
		TokenCount: item.TokenCount,
		CreatedAt:  item.CreatedAt,
	}
}

func toFileObjectProcessingStateDomain(item models.FileObject) domainconversation.FileObjectProcessing {
	return domainconversation.FileObjectProcessing{
		ID:                 item.ID,
		FileObjectID:       item.ID,
		UserID:             item.UserID,
		DetectedMIME:       item.DetectedMIME,
		FileCategory:       item.FileCategory,
		ProcessingStatus:   item.ProcessingStatus,
		ExtractStatus:      item.ExtractStatus,
		ExtractEngine:      item.ExtractEngine,
		ExtractStoragePath: item.ExtractStoragePath,
		ExtractChars:       item.ExtractChars,
		ExtractPages:       item.ExtractPages,
		PreviewText:        item.PreviewText,
		OCRUsed:            item.OCRUsed,
		RAGReady:           item.RAGReady,
		RAGReason:          item.RAGReason,
		ErrorCode:          item.ProcessingErrorCode,
		ErrorMessage:       item.ProcessingErrorMessage,
		ExtractorVersion:   item.ExtractorVersion,
		PayloadJSON:        item.ProcessingPayloadJSON,
		StartedAt:          item.ProcessingStartedAt,
		CompletedAt:        item.ProcessingCompletedAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func fileObjectProcessingStateUpdates(item *domainconversation.FileObjectProcessing) map[string]interface{} {
	if item == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"detected_mime":            item.DetectedMIME,
		"file_category":            item.FileCategory,
		"processing_status":        item.ProcessingStatus,
		"extract_status":           item.ExtractStatus,
		"extract_engine":           item.ExtractEngine,
		"extract_storage_path":     item.ExtractStoragePath,
		"extract_chars":            item.ExtractChars,
		"extract_pages":            item.ExtractPages,
		"preview_text":             item.PreviewText,
		"ocr_used":                 item.OCRUsed,
		"rag_ready":                item.RAGReady,
		"rag_reason":               item.RAGReason,
		"processing_error_code":    item.ErrorCode,
		"processing_error_message": item.ErrorMessage,
		"extractor_version":        item.ExtractorVersion,
		"processing_payload_json":  item.PayloadJSON,
		"processing_started_at":    item.StartedAt,
		"processing_completed_at":  item.CompletedAt,
		"updated_at":               time.Now(),
	}
}

// ── MessageEmbeddingRepository ─────────────────────────────────────────────

func (r *Repo) VectorStoreAvailable(ctx context.Context) (bool, error) {
	if r.sqliteDialect() {
		return sqlitevec.Available(ctx, r.db)
	}
	checks := []string{
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')`,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = 'file_chunks'
				AND column_name = 'embedding'
				AND udt_name = 'vector'
		)`,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = 'chat_message_chunks'
				AND column_name = 'embedding'
				AND udt_name = 'vector'
		)`,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = 'user_memories'
				AND column_name = 'embedding'
				AND udt_name = 'vector'
		)`,
		`SELECT to_regclass('idx_file_chunks_embedding') IS NOT NULL`,
		`SELECT to_regclass('idx_chat_message_chunks_embedding') IS NOT NULL`,
		`SELECT to_regclass('idx_user_memories_embedding') IS NOT NULL`,
	}
	for _, query := range checks {
		available := false
		if err := r.db.WithContext(ctx).Raw(query).Scan(&available).Error; err != nil {
			return false, translateError(err)
		}
		if !available {
			return false, nil
		}
	}
	return true, nil
}

// UpsertMessageChunks 为指定消息写入向量分片（先删旧后插新，再写 embedding）。
func (r *Repo) UpsertMessageChunks(ctx context.Context, chunks []domainconversation.MessageChunk, embeddings [][]float32) error {
	if len(chunks) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 收集需要删除的 messageID 集合（幂等清理）
		seen := make(map[uint]struct{}, len(chunks))
		messageIDs := make([]uint, 0, len(chunks))
		for _, c := range chunks {
			if _, ok := seen[c.MessageID]; !ok {
				messageIDs = append(messageIDs, c.MessageID)
				seen[c.MessageID] = struct{}{}
			}
		}
		if r.sqliteDialect() {
			if err := deleteSQLiteMessageChunkVectorsByMessages(tx, messageIDs); err != nil {
				return err
			}
		}
		if err := tx.Where("message_id IN ?", messageIDs).Delete(&models.MessageChunk{}).Error; err != nil {
			return translateError(err)
		}
		// 插入新分片
		entities := make([]models.MessageChunk, 0, len(chunks))
		for i := range chunks {
			entities = append(entities, models.MessageChunk{
				ConversationID: chunks[i].ConversationID,
				MessageID:      chunks[i].MessageID,
				UserID:         chunks[i].UserID,
				Role:           chunks[i].Role,
				ChunkIndex:     chunks[i].ChunkIndex,
				Content:        chunks[i].Content,
				TokenCount:     chunks[i].TokenCount,
			})
		}
		if err := tx.Create(&entities).Error; err != nil {
			return translateError(err)
		}
		if r.sqliteDialect() {
			return insertSQLiteMessageChunkVectors(tx, entities, embeddings)
		}
		// 写入 embedding 向量。
		for i, entity := range entities {
			if i >= len(embeddings) || len(embeddings[i]) == 0 {
				continue
			}
			vec := float32SliceToPostgresVector(embeddings[i])
			if err := tx.Exec(`UPDATE "chat_message_chunks" SET embedding = ? WHERE id = ?`, vec, entity.ID).Error; err != nil {
				return translateError(err)
			}
		}
		return nil
	})
}

type messageChunkSearchRow struct {
	ID             uint      `gorm:"column:id"`
	ConversationID uint      `gorm:"column:conversation_id"`
	MessageID      uint      `gorm:"column:message_id"`
	UserID         uint      `gorm:"column:user_id"`
	Role           string    `gorm:"column:role"`
	ChunkIndex     int       `gorm:"column:chunk_index"`
	Content        string    `gorm:"column:content"`
	TokenCount     int       `gorm:"column:token_count"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	Similarity     float64   `gorm:"column:similarity"`
}

func deleteSQLiteMessageChunkVectorsByMessages(tx *gorm.DB, messageIDs []uint) error {
	if len(messageIDs) == 0 {
		return nil
	}
	return translateError(tx.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE chunk_id IN (
			SELECT id FROM "chat_message_chunks" WHERE message_id IN ?
		)`, sqlitevec.MessageChunkVectorTable),
		messageIDs,
	).Error)
}

func insertSQLiteMessageChunkVectors(tx *gorm.DB, entities []models.MessageChunk, embeddings [][]float32) error {
	for i, chunk := range entities {
		if i >= len(embeddings) || len(embeddings[i]) == 0 {
			continue
		}
		vector, err := sqlitevec.SerializeFloat32(embeddings[i])
		if err != nil {
			return err
		}
		if err = tx.Exec(
			fmt.Sprintf(`INSERT INTO %s (chunk_id, user_id, conversation_id, message_id, embedding) VALUES (?, ?, ?, ?, ?)`, sqlitevec.MessageChunkVectorTable),
			chunk.ID,
			chunk.UserID,
			chunk.ConversationID,
			chunk.MessageID,
			vector,
		).Error; err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r *Repo) searchSQLiteMessageChunks(ctx context.Context, conversationID uint, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainconversation.MessageChunk, error) {
	vector, err := sqlitevec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT chunks.id, chunks.conversation_id, chunks.message_id, chunks.user_id, chunks.role,
		       chunks.chunk_index, chunks.content, chunks.token_count, chunks.created_at,
		       (1.0 - vectors.distance) AS similarity
		FROM %s AS vectors
		JOIN "chat_message_chunks" AS chunks
			ON chunks.id = vectors.chunk_id
		WHERE vectors.embedding MATCH ?
			AND vectors.k = ?
			AND vectors.user_id = ?
			AND vectors.conversation_id = ?
		ORDER BY vectors.distance ASC`,
		sqlitevec.MessageChunkVectorTable,
	)
	var rows []messageChunkSearchRow
	if err := r.db.WithContext(ctx).Raw(query, vector, topK, userID, conversationID).Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainconversation.MessageChunk, 0, len(rows))
	for _, row := range rows {
		if row.Similarity < minSimilarity {
			continue
		}
		results = append(results, domainconversation.MessageChunk{
			ID:             row.ID,
			ConversationID: row.ConversationID,
			MessageID:      row.MessageID,
			UserID:         row.UserID,
			Role:           row.Role,
			ChunkIndex:     row.ChunkIndex,
			Content:        row.Content,
			TokenCount:     row.TokenCount,
			Similarity:     row.Similarity,
			CreatedAt:      row.CreatedAt,
		})
	}
	return results, nil
}

// SearchMessageChunks 按查询向量检索最相关的历史消息分片。
func (r *Repo) SearchMessageChunks(ctx context.Context, conversationID uint, userID uint, queryEmbedding []float32, topK int, minSimilarity float64) ([]domainconversation.MessageChunk, error) {
	if len(queryEmbedding) == 0 || topK <= 0 {
		return nil, nil
	}
	if r.sqliteDialect() {
		return r.searchSQLiteMessageChunks(ctx, conversationID, userID, queryEmbedding, topK, minSimilarity)
	}
	vec := float32SliceToPostgresVector(queryEmbedding)
	query := `
		SELECT id, conversation_id, message_id, user_id, role, chunk_index, content, token_count, created_at,
		       (1 - (embedding <=> ?::vector)) AS similarity
		FROM chat_message_chunks
		WHERE conversation_id = ? AND user_id = ? AND embedding IS NOT NULL
		ORDER BY similarity DESC
		LIMIT ?`
	var rows []messageChunkSearchRow
	if err := r.db.WithContext(ctx).Raw(query, vec, conversationID, userID, topK).Scan(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainconversation.MessageChunk, 0, len(rows))
	for _, row := range rows {
		if row.Similarity < minSimilarity {
			continue
		}
		results = append(results, domainconversation.MessageChunk{
			ID:             row.ID,
			ConversationID: row.ConversationID,
			MessageID:      row.MessageID,
			UserID:         row.UserID,
			Role:           row.Role,
			ChunkIndex:     row.ChunkIndex,
			Content:        row.Content,
			TokenCount:     row.TokenCount,
			Similarity:     row.Similarity,
		})
	}
	return results, nil
}

// MarkAllEmbeddedFilesStale 将所有 embed_status=ready 的文件标记为 stale。
func (r *Repo) MarkAllEmbeddedFilesStale(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("embed_status = ? AND status = ?", "ready", "active").
		Updates(map[string]interface{}{
			"embed_status": "stale",
			"embed_error":  "embedding model changed, reindex required",
		})
	return result.RowsAffected, translateError(result.Error)
}

// CountFilesByEmbedStatus 统计指定 embed_status 的文件数量。
func (r *Repo) CountFilesByEmbedStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("embed_status = ? AND status = ?", status, "active").
		Count(&count).Error
	return count, translateError(err)
}

// MarkTimedOutFileEmbeddingsFailed 将长时间停留在向量化中的文件标记为失败。
func (r *Repo) MarkTimedOutFileEmbeddingsFailed(ctx context.Context, userID uint, cutoff time.Time, message string) (int64, error) {
	if message == "" {
		message = "向量化超时"
	}
	result := r.db.WithContext(ctx).
		Model(&models.FileObject{}).
		Where("user_id = ? AND status = ? AND embed_status = ? AND updated_at < ?", userID, "active", "processing", cutoff).
		Updates(map[string]interface{}{
			"embed_status":             "failed",
			"embed_error":              truncateText(message, 255),
			"processing_status":        gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_status END", "embedding", "ready"),
			"processing_ready":         gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_ready END", "embedding", true),
			"processing_error_code":    gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_error_code END", "embedding", "embed_failed"),
			"processing_error_message": gorm.Expr("CASE WHEN processing_status = ? THEN ? ELSE processing_error_message END", "embedding", truncateText(message, 255)),
		})
	return result.RowsAffected, translateError(result.Error)
}

// ListFilesForReindex 分页返回需要重建向量的文件（embed_status 为 stale 或 failed）。
func (r *Repo) ListFilesForReindex(ctx context.Context, limit int, offset int) ([]domainconversation.FileObject, error) {
	if limit <= 0 {
		limit = 50
	}
	var entities []models.FileObject
	err := r.db.WithContext(ctx).
		Where("embed_status IN ? AND status = ?", []string{"stale", "failed"}, "active").
		Order("updated_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&entities).Error
	if err != nil {
		return nil, translateError(err)
	}
	results := make([]domainconversation.FileObject, 0, len(entities))
	for i := range entities {
		results = append(results, toFileObjectDomain(entities[i]))
	}
	return results, nil
}
