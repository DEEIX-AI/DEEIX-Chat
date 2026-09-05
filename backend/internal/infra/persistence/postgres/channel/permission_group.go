package channel

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 权限组（模型访问控制）
// ---------------------------------------------------------------------------

// ListPermissionGroups 返回全部权限组，默认组优先。
func (r *Repo) ListPermissionGroups(ctx context.Context) ([]domainchannel.PermissionGroup, error) {
	rows := make([]model.PermissionGroup, 0)
	if err := r.db.WithContext(ctx).
		Order("is_default DESC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	modelStats, err := r.listPermissionGroupModelStats(ctx)
	if err != nil {
		return nil, err
	}
	totalUsers, err := r.countPermissionGroupUsers(ctx)
	if err != nil {
		return nil, err
	}
	manualUserIDsByGroup, err := r.listManualPermissionGroupUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	subscriptionUserIDsByGroup, err := r.listSubscriptionPermissionGroupUserIDs(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	results := make([]domainchannel.PermissionGroup, 0, len(rows))
	for _, row := range rows {
		item := toPermissionGroupDomain(row)
		item.ModelCount = modelStats.total[item.ID]
		item.ManualModelCount = modelStats.manual[item.ID]
		item.RuleModelCount = modelStats.rule[item.ID]
		if item.IsDefault {
			item.UserCount = totalUsers
		} else {
			item.ManualUserCount = int64(len(manualUserIDsByGroup[item.ID]))
			item.SubscriptionUserCount = int64(len(subscriptionUserIDsByGroup[item.ID]))
			item.UserCount = int64(len(mergeUserIDSets(manualUserIDsByGroup[item.ID], subscriptionUserIDsByGroup[item.ID])))
		}
		results = append(results, item)
	}
	return results, nil
}

// GetPermissionGroup 按 ID 获取权限组。
func (r *Repo) GetPermissionGroup(ctx context.Context, id uint) (*domainchannel.PermissionGroup, error) {
	var item model.PermissionGroup
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, dberror.Translate(err)
	}
	result := toPermissionGroupDomain(item)
	return &result, nil
}

// PermissionGroupExists 判断权限组是否存在。
func (r *Repo) PermissionGroupExists(ctx context.Context, id uint) (bool, error) {
	if id == 0 {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("id = ?", id).
		Count(&count).Error; err != nil {
		return false, dberror.Translate(err)
	}
	return count > 0, nil
}

// CreatePermissionGroup 创建权限组。
func (r *Repo) CreatePermissionGroup(ctx context.Context, item *domainchannel.PermissionGroup) error {
	if item == nil {
		return repository.ErrInvalidInput
	}
	entity := model.PermissionGroup{
		Name:                  strings.TrimSpace(item.Name),
		Description:           strings.TrimSpace(item.Description),
		IsDefault:             item.IsDefault,
		RateMultiplierPercent: normalizeRateMultiplierPercent(item.RateMultiplierPercent),
	}
	if entity.Name == "" {
		return repository.ErrInvalidInput
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return dberror.Translate(err)
	}
	*item = toPermissionGroupDomain(entity)
	return nil
}

// UpdatePermissionGroup 更新权限组名称、说明与计费倍率。
func (r *Repo) UpdatePermissionGroup(ctx context.Context, id uint, name string, description string, rateMultiplierPercent int) (*domainchannel.PermissionGroup, error) {
	result := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":                    strings.TrimSpace(name),
			"description":             strings.TrimSpace(description),
			"rate_multiplier_percent": normalizeRateMultiplierPercent(rateMultiplierPercent),
		})
	if result.Error != nil {
		return nil, dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetPermissionGroup(ctx, id)
}

// DeletePermissionGroup 硬删除权限组及其模型/用户关联。
func (r *Repo) DeletePermissionGroup(ctx context.Context, id uint) error {
	return dberror.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.PermissionGroup
		if err := tx.Where("id = ?", id).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrNotFound
			}
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&model.PermissionGroupModelAccess{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&model.PermissionGroupModelRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&model.PermissionGroupUserAccess{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PermissionGroup{}, id).Error
	}))
}

// GetPermissionGroupDeleteSummary 返回删除权限组前会清理的关联规模。
func (r *Repo) GetPermissionGroupDeleteSummary(ctx context.Context, id uint) (domainchannel.PermissionGroupDeleteSummary, error) {
	var summary domainchannel.PermissionGroupDeleteSummary
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelAccess{}).
		Where("group_id = ?", id).
		Count(&summary.ManualModelCount).Error; err != nil {
		return summary, dberror.Translate(err)
	}
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelRule{}).
		Where("group_id = ?", id).
		Count(&summary.RuleCount).Error; err != nil {
		return summary, dberror.Translate(err)
	}
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupUserAccess{}).
		Where("group_id = ?", id).
		Count(&summary.ManualUserCount).Error; err != nil {
		return summary, dberror.Translate(err)
	}
	return summary, nil
}

// ---------------------------------------------------------------------------
// 权限组 - 模型关联
// ---------------------------------------------------------------------------

// ListGroupModelIDs 返回权限组已授权的平台模型 ID。
func (r *Repo) ListGroupModelIDs(ctx context.Context, groupID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelAccess{}).
		Where("group_id = ?", groupID).
		Order("platform_model_id ASC").
		Pluck("platform_model_id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return ids, nil
}

// ListGroupModelRules 返回权限组动态模型访问规则。
func (r *Repo) ListGroupModelRules(ctx context.Context, groupID uint) ([]domainchannel.PermissionGroupModelRule, error) {
	rows := make([]model.PermissionGroupModelRule, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelRule{}).
		Where("group_id = ?", groupID).
		Order("rule_type ASC, value ASC").
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	results := make([]domainchannel.PermissionGroupModelRule, 0, len(rows))
	for _, row := range rows {
		results = append(results, toPermissionGroupModelRuleDomain(row))
	}
	return results, nil
}

// SetGroupModelAccess 全量替换权限组授权的平台模型集合与动态规则。
func (r *Repo) SetGroupModelAccess(ctx context.Context, groupID uint, modelIDs []uint, rules []domainchannel.PermissionGroupModelRule) error {
	return dberror.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.PermissionGroupModelAccess{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&model.PermissionGroupModelRule{}).Error; err != nil {
			return err
		}
		modelRows := dedupeAccessModelRows(groupID, modelIDs)
		if len(modelRows) > 0 {
			if err := tx.Create(&modelRows).Error; err != nil {
				return err
			}
		}
		ruleRows := dedupeAccessRuleRows(groupID, rules)
		if len(ruleRows) > 0 {
			if err := tx.Create(&ruleRows).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

// ListModelManualGroupIDs 返回手动授权某平台模型的权限组 ID，不包含动态规则命中的权限组。
func (r *Repo) ListModelManualGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelAccess{}).
		Where("platform_model_id = ?", platformModelID).
		Order("group_id ASC").
		Pluck("group_id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return ids, nil
}

// ListModelRuleGroupIDs 返回动态规则命中某平台模型的权限组 ID。
func (r *Repo) ListModelRuleGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error) {
	return r.listModelRuleGroupIDs(ctx, platformModelID)
}

// SetModelManualGroups 全量替换某平台模型的手动权限组，不影响动态规则。
func (r *Repo) SetModelManualGroups(ctx context.Context, platformModelID uint, groupIDs []uint) error {
	return dberror.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("platform_model_id = ?", platformModelID).Delete(&model.PermissionGroupModelAccess{}).Error; err != nil {
			return err
		}
		rows := dedupeModelAccessGroupRows(platformModelID, groupIDs)
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	}))
}

// ---------------------------------------------------------------------------
// 权限组 - 用户关联
// ---------------------------------------------------------------------------

// ListGroupUserIDs 返回权限组内的用户 ID。
func (r *Repo) ListGroupUserIDs(ctx context.Context, groupID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupUserAccess{}).
		Where("group_id = ?", groupID).
		Order("user_id ASC").
		Pluck("user_id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return ids, nil
}

// SetGroupUsers 全量替换权限组内的用户集合。
func (r *Repo) SetGroupUsers(ctx context.Context, groupID uint, userIDs []uint) error {
	return dberror.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.PermissionGroupUserAccess{}).Error; err != nil {
			return err
		}
		rows := dedupeAccessUserRows(groupID, userIDs)
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	}))
}

func (r *Repo) countPermissionGroupUsers(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Count(&count).Error; err != nil {
		return 0, dberror.Translate(err)
	}
	return count, nil
}

type permissionGroupUserIDRow struct {
	GroupID uint
	UserID  uint
}

func (r *Repo) listManualPermissionGroupUserIDs(ctx context.Context) (map[uint]map[uint]struct{}, error) {
	rows := make([]permissionGroupUserIDRow, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupUserAccess{}).
		Select("group_id, user_id").
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return groupUserIDsByGroup(rows), nil
}

func (r *Repo) listSubscriptionPermissionGroupUserIDs(ctx context.Context, now time.Time) (map[uint]map[uint]struct{}, error) {
	rows := make([]permissionGroupUserIDRow, 0)
	if err := r.db.WithContext(ctx).
		Table("billing_subscriptions AS subscription").
		Select("plan.permission_group_id AS group_id, subscription.user_id").
		Joins("JOIN billing_plans AS plan ON plan.id = subscription.plan_id").
		Where(`
			plan.permission_group_id IS NOT NULL
			AND plan.is_active = ?
			AND subscription.status = ?
			AND subscription.current_period_start_at <= ?
			AND (subscription.current_period_end_at IS NULL OR subscription.current_period_end_at > ?)
		`, true, "active", now, now).
		Scan(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return groupUserIDsByGroup(rows), nil
}

func groupUserIDsByGroup(rows []permissionGroupUserIDRow) map[uint]map[uint]struct{} {
	results := make(map[uint]map[uint]struct{})
	for _, row := range rows {
		if row.GroupID == 0 || row.UserID == 0 {
			continue
		}
		if _, ok := results[row.GroupID]; !ok {
			results[row.GroupID] = make(map[uint]struct{})
		}
		results[row.GroupID][row.UserID] = struct{}{}
	}
	return results
}

func mergeUserIDSets(left map[uint]struct{}, right map[uint]struct{}) map[uint]struct{} {
	results := make(map[uint]struct{}, len(left)+len(right))
	for id := range left {
		results[id] = struct{}{}
	}
	for id := range right {
		results[id] = struct{}{}
	}
	return results
}

func addModelIDToGroupSet(items map[uint]map[uint]struct{}, groupID uint, modelID uint) {
	if groupID == 0 || modelID == 0 {
		return
	}
	if _, ok := items[groupID]; !ok {
		items[groupID] = make(map[uint]struct{})
	}
	items[groupID][modelID] = struct{}{}
}

func countGroupModelSets(items map[uint]map[uint]struct{}) map[uint]int64 {
	results := make(map[uint]int64, len(items))
	for groupID, modelIDs := range items {
		results[groupID] = int64(len(modelIDs))
	}
	return results
}

// ---------------------------------------------------------------------------
// 访问判定
// ---------------------------------------------------------------------------

// ListUserGroupIDs 返回用户显式归属的权限组 ID。
func (r *Repo) ListUserGroupIDs(ctx context.Context, userID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupUserAccess{}).
		Where("user_id = ?", userID).
		Order("group_id ASC").
		Pluck("group_id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return ids, nil
}

func (r *Repo) listEffectiveUserGroupIDs(ctx context.Context, userID uint, extraGroupIDs []uint) ([]uint, error) {
	ids := make([]uint, 0, len(extraGroupIDs)+4)
	appendID := func(id uint) {
		ids = appendUniqueGroupID(ids, id)
	}
	if userID > 0 {
		directIDs, err := r.ListUserGroupIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, id := range directIDs {
			appendID(id)
		}
	}
	defaultIDs, err := r.ListDefaultGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range defaultIDs {
		appendID(id)
	}
	for _, id := range extraGroupIDs {
		appendID(id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// ListModelGroupIDs 返回授权某平台模型的权限组 ID。
func (r *Repo) ListModelGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelAccess{}).
		Where("platform_model_id = ?", platformModelID).
		Order("group_id ASC").
		Pluck("group_id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	ruleIDs, err := r.listModelRuleGroupIDs(ctx, platformModelID)
	if err != nil {
		return nil, err
	}
	return mergeGroupIDLists(ids, ruleIDs), nil
}

// ListModelsWithGroupAccess 返回所有已配置权限组的模型到权限组 ID 的映射。
func (r *Repo) ListModelsWithGroupAccess(ctx context.Context) (map[uint][]uint, error) {
	var rows []model.PermissionGroupModelAccess
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := make(map[uint][]uint)
	for _, row := range rows {
		result[row.PlatformModelID] = append(result[row.PlatformModelID], row.GroupID)
	}
	ruleRows := make([]model.PermissionGroupModelRule, 0)
	if err := r.db.WithContext(ctx).Find(&ruleRows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	if len(ruleRows) == 0 {
		return sortModelGroupMap(result), nil
	}
	contexts, err := r.listModelAccessRuleContexts(ctx)
	if err != nil {
		return nil, err
	}
	for _, ctxItem := range contexts {
		for _, rule := range ruleRows {
			if !modelRuleMatchesContext(rule, ctxItem) {
				continue
			}
			result[ctxItem.PlatformModelID] = appendUniqueGroupID(result[ctxItem.PlatformModelID], rule.GroupID)
		}
	}
	return sortModelGroupMap(result), nil
}

type permissionGroupModelStats struct {
	total  map[uint]int64
	manual map[uint]int64
	rule   map[uint]int64
}

func (r *Repo) listPermissionGroupModelStats(ctx context.Context) (permissionGroupModelStats, error) {
	manualRows := make([]model.PermissionGroupModelAccess, 0)
	if err := r.db.WithContext(ctx).Find(&manualRows).Error; err != nil {
		return permissionGroupModelStats{}, dberror.Translate(err)
	}
	manualSets := make(map[uint]map[uint]struct{})
	for _, row := range manualRows {
		addModelIDToGroupSet(manualSets, row.GroupID, row.PlatformModelID)
	}

	ruleSets := make(map[uint]map[uint]struct{})
	ruleRows := make([]model.PermissionGroupModelRule, 0)
	if err := r.db.WithContext(ctx).Find(&ruleRows).Error; err != nil {
		return permissionGroupModelStats{}, dberror.Translate(err)
	}
	if len(ruleRows) > 0 {
		contexts, err := r.listModelAccessRuleContexts(ctx)
		if err != nil {
			return permissionGroupModelStats{}, err
		}
		for _, ctxItem := range contexts {
			for _, rule := range ruleRows {
				if modelRuleMatchesContext(rule, ctxItem) {
					addModelIDToGroupSet(ruleSets, rule.GroupID, ctxItem.PlatformModelID)
				}
			}
		}
	}

	totalSets := make(map[uint]map[uint]struct{}, len(manualSets)+len(ruleSets))
	for groupID, modelIDs := range manualSets {
		for modelID := range modelIDs {
			addModelIDToGroupSet(totalSets, groupID, modelID)
		}
	}
	for groupID, modelIDs := range ruleSets {
		for modelID := range modelIDs {
			addModelIDToGroupSet(totalSets, groupID, modelID)
		}
	}
	return permissionGroupModelStats{
		total:  countGroupModelSets(totalSets),
		manual: countGroupModelSets(manualSets),
		rule:   countGroupModelSets(ruleSets),
	}, nil
}

func (r *Repo) listModelRuleGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error) {
	rules := make([]model.PermissionGroupModelRule, 0)
	if err := r.db.WithContext(ctx).Find(&rules).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	if len(rules) == 0 {
		return []uint{}, nil
	}
	ctxItem, err := r.getModelAccessRuleContext(ctx, platformModelID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0)
	for _, rule := range rules {
		if modelRuleMatchesContext(rule, ctxItem) {
			ids = appendUniqueGroupID(ids, rule.GroupID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

type modelAccessRuleContext struct {
	PlatformModelID uint
	Vendor          string
	Protocols       map[string]struct{}
	UpstreamIDs     map[string]struct{}
}

func (r *Repo) listModelAccessRuleContexts(ctx context.Context) (map[uint]modelAccessRuleContext, error) {
	type modelRow struct {
		ID     uint
		Vendor string
	}
	modelRows := make([]modelRow, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.LLMPlatformModel{}).
		Select("id, vendor").
		Find(&modelRows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	contexts := make(map[uint]modelAccessRuleContext, len(modelRows))
	for _, row := range modelRows {
		contexts[row.ID] = modelAccessRuleContext{
			PlatformModelID: row.ID,
			Vendor:          strings.TrimSpace(row.Vendor),
			Protocols:       make(map[string]struct{}),
			UpstreamIDs:     make(map[string]struct{}),
		}
	}
	if err := r.populateModelAccessRouteContext(ctx, contexts); err != nil {
		return nil, err
	}
	return contexts, nil
}

func (r *Repo) getModelAccessRuleContext(ctx context.Context, platformModelID uint) (modelAccessRuleContext, error) {
	var item model.LLMPlatformModel
	if err := r.db.WithContext(ctx).
		Select("id, vendor").
		Where("id = ?", platformModelID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return modelAccessRuleContext{}, repository.ErrNotFound
		}
		return modelAccessRuleContext{}, dberror.Translate(err)
	}
	contexts := map[uint]modelAccessRuleContext{
		item.ID: {
			PlatformModelID: item.ID,
			Vendor:          strings.TrimSpace(item.Vendor),
			Protocols:       make(map[string]struct{}),
			UpstreamIDs:     make(map[string]struct{}),
		},
	}
	if err := r.populateModelAccessRouteContext(ctx, contexts); err != nil {
		return modelAccessRuleContext{}, err
	}
	return contexts[item.ID], nil
}

func (r *Repo) populateModelAccessRouteContext(ctx context.Context, contexts map[uint]modelAccessRuleContext) error {
	if len(contexts) == 0 {
		return nil
	}
	modelIDs := make([]uint, 0, len(contexts))
	for modelID := range contexts {
		modelIDs = append(modelIDs, modelID)
	}
	type routeRow struct {
		PlatformModelID uint
		Protocol        string
		UpstreamID      uint
	}
	rows := make([]routeRow, 0)
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("r.platform_model_id, r.protocol, um.upstream_id").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_upstreams u ON u.id = um.upstream_id").
		Where("r.platform_model_id IN ? AND r.status = ? AND um.status = ? AND u.status = ?", modelIDs, "active", "active", "active").
		Scan(&rows).Error; err != nil {
		return dberror.Translate(err)
	}
	for _, row := range rows {
		ctxItem, ok := contexts[row.PlatformModelID]
		if !ok {
			continue
		}
		if protocol := strings.TrimSpace(row.Protocol); protocol != "" {
			ctxItem.Protocols[protocol] = struct{}{}
		}
		if row.UpstreamID > 0 {
			ctxItem.UpstreamIDs[strconv.FormatUint(uint64(row.UpstreamID), 10)] = struct{}{}
		}
		contexts[row.PlatformModelID] = ctxItem
	}
	return nil
}

func modelRuleMatchesContext(rule model.PermissionGroupModelRule, ctxItem modelAccessRuleContext) bool {
	ruleType := strings.TrimSpace(rule.RuleType)
	value := strings.TrimSpace(rule.Value)
	switch ruleType {
	case domainchannel.PermissionGroupModelRuleAll:
		return true
	case domainchannel.PermissionGroupModelRuleVendor:
		return value != "" && ctxItem.Vendor == value
	case domainchannel.PermissionGroupModelRuleProtocol:
		_, ok := ctxItem.Protocols[value]
		return value != "" && ok
	case domainchannel.PermissionGroupModelRuleUpstream:
		_, ok := ctxItem.UpstreamIDs[value]
		return value != "" && ok
	default:
		return false
	}
}

func mergeGroupIDLists(left []uint, right []uint) []uint {
	result := make([]uint, 0, len(left)+len(right))
	for _, id := range left {
		result = appendUniqueGroupID(result, id)
	}
	for _, id := range right {
		result = appendUniqueGroupID(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func appendUniqueGroupID(ids []uint, id uint) []uint {
	if id == 0 {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func sortModelGroupMap(items map[uint][]uint) map[uint][]uint {
	for modelID, ids := range items {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		items[modelID] = ids
	}
	return items
}

// ListDefaultGroupIDs 返回内置默认权限组 ID（所有用户隐式归属）。
func (r *Repo) ListDefaultGroupIDs(ctx context.Context) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("is_default = ?", true).
		Order("id ASC").
		Pluck("id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// 映射与辅助
// ---------------------------------------------------------------------------

func toPermissionGroupDomain(item model.PermissionGroup) domainchannel.PermissionGroup {
	return domainchannel.PermissionGroup{
		ID:                    item.ID,
		Name:                  item.Name,
		Description:           item.Description,
		IsDefault:             item.IsDefault,
		RateMultiplierPercent: normalizeRateMultiplierPercent(item.RateMultiplierPercent),
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func toPermissionGroupModelRuleDomain(item model.PermissionGroupModelRule) domainchannel.PermissionGroupModelRule {
	return domainchannel.PermissionGroupModelRule{
		GroupID:  item.GroupID,
		RuleType: item.RuleType,
		Value:    item.Value,
	}
}

func normalizeRateMultiplierPercent(value int) int {
	if value <= 0 {
		return 100
	}
	return value
}

// GetUserModelGroupRateMultiplierPercent 返回用户当前模型命中权限组的计费倍率百分比。
//
// 用户有效权限组由手动成员、默认权限组和订阅绑定权限组共同组成；当模型访问控制已启用时，
// 只在用户有效权限组与当前模型权限组的交集中取最低倍率。
func (r *Repo) GetUserModelGroupRateMultiplierPercent(ctx context.Context, userID uint, platformModelID uint, extraGroupIDs []uint) (int, error) {
	userGroupIDs, err := r.listEffectiveUserGroupIDs(ctx, userID, extraGroupIDs)
	if err != nil {
		return 100, dberror.Translate(err)
	}
	if len(userGroupIDs) == 0 {
		return 100, nil
	}
	if platformModelID == 0 {
		return 100, nil
	}
	groupIDs := userGroupIDs
	if platformModelID > 0 {
		modelGroupIDs, err := r.ListModelGroupIDs(ctx, platformModelID)
		if err != nil {
			return 100, dberror.Translate(err)
		}
		if len(modelGroupIDs) == 0 {
			return 100, nil
		}
		groupIDs = intersectGroupIDLists(userGroupIDs, modelGroupIDs)
	}
	return r.minGroupRateMultiplierPercent(ctx, groupIDs)
}

func (r *Repo) minGroupRateMultiplierPercent(ctx context.Context, groupIDs []uint) (int, error) {
	if len(groupIDs) == 0 {
		return 100, nil
	}
	var minPercent int
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("id IN ?", groupIDs).
		Select("COALESCE(MIN(rate_multiplier_percent), 100)").
		Scan(&minPercent).Error; err != nil {
		return 100, dberror.Translate(err)
	}
	return normalizeRateMultiplierPercent(minPercent), nil
}

func intersectGroupIDLists(left []uint, right []uint) []uint {
	if len(left) == 0 || len(right) == 0 {
		return []uint{}
	}
	rightSet := make(map[uint]struct{}, len(right))
	for _, id := range right {
		if id > 0 {
			rightSet[id] = struct{}{}
		}
	}
	result := make([]uint, 0)
	for _, id := range left {
		if _, ok := rightSet[id]; ok {
			result = appendUniqueGroupID(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func dedupeAccessModelRows(groupID uint, modelIDs []uint) []model.PermissionGroupModelAccess {
	seen := make(map[uint]struct{}, len(modelIDs))
	rows := make([]model.PermissionGroupModelAccess, 0, len(modelIDs))
	for _, id := range modelIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, model.PermissionGroupModelAccess{GroupID: groupID, PlatformModelID: id})
	}
	return rows
}

func dedupeModelAccessGroupRows(platformModelID uint, groupIDs []uint) []model.PermissionGroupModelAccess {
	seen := make(map[uint]struct{}, len(groupIDs))
	rows := make([]model.PermissionGroupModelAccess, 0, len(groupIDs))
	for _, id := range groupIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, model.PermissionGroupModelAccess{GroupID: id, PlatformModelID: platformModelID})
	}
	return rows
}

func dedupeAccessRuleRows(groupID uint, rules []domainchannel.PermissionGroupModelRule) []model.PermissionGroupModelRule {
	seen := make(map[string]struct{}, len(rules))
	rows := make([]model.PermissionGroupModelRule, 0, len(rules))
	for _, rule := range rules {
		ruleType := strings.TrimSpace(rule.RuleType)
		value := strings.TrimSpace(rule.Value)
		if ruleType == domainchannel.PermissionGroupModelRuleAll {
			value = ""
		}
		if ruleType == "" {
			continue
		}
		key := ruleType + "\x00" + value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		rows = append(rows, model.PermissionGroupModelRule{
			GroupID:  groupID,
			RuleType: ruleType,
			Value:    value,
		})
	}
	return rows
}

func dedupeAccessUserRows(groupID uint, userIDs []uint) []model.PermissionGroupUserAccess {
	seen := make(map[uint]struct{}, len(userIDs))
	rows := make([]model.PermissionGroupUserAccess, 0, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, model.PermissionGroupUserAccess{GroupID: groupID, UserID: id})
	}
	return rows
}
