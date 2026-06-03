package channel

import (
	"context"
	"errors"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
// channel 包内部的语义错误（ErrUpstreamNotFound 等）优先在调用点直接返回，
// 此函数处理未在调用点明确转换的 gorm 错误。
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

// Repo 封装上游域数据访问。
type Repo struct {
	db *gorm.DB
}

// UpstreamRouteRow 是上游路由查询结果。
type UpstreamRouteRow = repository.ChannelUpstreamRouteRow
type UpstreamListRow = repository.ChannelUpstreamListRow
type ModelListRow = repository.ChannelModelListRow
type UpstreamModelListRow = repository.ChannelUpstreamModelListRow
type ModelSourceRow = repository.ChannelModelSourceRow

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ---------------------------------------------------------------------------
// 上游管理
// ---------------------------------------------------------------------------

// CreateUpstream 创建上游。
func (r *Repo) CreateUpstream(ctx context.Context, item *domainchannel.Upstream) error {
	entity := toUpstreamModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toUpstreamDomain(entity)
	return nil
}

// UpdateUpstream 更新上游。
func (r *Repo) UpdateUpstream(ctx context.Context, upstreamID uint, input repository.UpdateChannelUpstreamInput) error {
	updates := upstreamUpdates(input)
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&model.LLMUpstream{}).
		Where("id = ?", upstreamID).
		Updates(updates)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUpstreamNotFound
	}
	return nil
}

func upstreamUpdates(input repository.UpdateChannelUpstreamInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.BaseURL != nil {
		updates["base_url"] = *input.BaseURL
	}
	if input.Compatible != nil {
		updates["compatible"] = *input.Compatible
	}
	if input.ProtocolDefaultsJSON != nil {
		updates["protocol_defaults_json"] = *input.ProtocolDefaultsJSON
	}
	if input.APIKeysEnc != nil {
		updates["api_keys_enc"] = *input.APIKeysEnc
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.ConnectTimeoutMS != nil {
		updates["connect_timeout_ms"] = *input.ConnectTimeoutMS
	}
	if input.ReadTimeoutMS != nil {
		updates["read_timeout_ms"] = *input.ReadTimeoutMS
	}
	if input.StreamIdleTimeoutMS != nil {
		updates["stream_idle_timeout_ms"] = *input.StreamIdleTimeoutMS
	}
	if input.CbFailureThreshold != nil {
		updates["cb_failure_threshold"] = *input.CbFailureThreshold
	}
	if input.CbModelThreshold != nil {
		updates["cb_model_threshold"] = *input.CbModelThreshold
	}
	if input.CbThresholdLogic != nil {
		updates["cb_threshold_logic"] = *input.CbThresholdLogic
	}
	if input.CbDurationMin != nil {
		updates["cb_duration_min"] = *input.CbDurationMin
	}
	if input.CbWindowMin != nil {
		updates["cb_window_min"] = *input.CbWindowMin
	}
	if input.HeadersJSON != nil {
		updates["headers_json"] = *input.HeadersJSON
	}
	return updates
}

// GetUpstreamByID 按 ID 获取上游。
func (r *Repo) GetUpstreamByID(ctx context.Context, upstreamID uint) (*domainchannel.Upstream, error) {
	var item model.LLMUpstream
	if err := r.db.WithContext(ctx).Where("id = ?", upstreamID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamNotFound
		}
		return nil, translateError(err)
	}
	result := toUpstreamDomain(item)
	return &result, nil
}

// ListUpstreams 分页查询上游。
func (r *Repo) ListUpstreams(ctx context.Context, input repository.ListChannelUpstreamsInput) ([]UpstreamListRow, int64, error) {
	items := make([]UpstreamListRow, 0)
	var total int64

	query := applyUpstreamListFilters(r.db.WithContext(ctx).Model(&model.LLMUpstream{}), input)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	listQuery := r.db.WithContext(ctx).
		Table("llm_upstreams AS u").
		Select(
			"u.*, COALESCE(stats.models_count, 0) AS models_count, COALESCE(stats.active_models_count, 0) AS active_models_count",
		).
		Joins(
			`LEFT JOIN (
				SELECT um.upstream_id,
					COUNT(r.id) AS models_count,
					COUNT(r.id) FILTER (WHERE r.status = 'active' AND um.status = 'active') AS active_models_count
				FROM llm_upstream_models um
				LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id
				GROUP BY um.upstream_id
			) AS stats ON stats.upstream_id = u.id`,
		)
	listQuery = applyUpstreamListFilters(listQuery, input)
	if err := listQuery.
		Order(upstreamListOrder(input.Sort)).
		Offset(input.Offset).
		Limit(input.Limit).
		Scan(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return items, total, nil
}

func applyUpstreamListFilters(query *gorm.DB, input repository.ListChannelUpstreamsInput) *gorm.DB {
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name ILIKE ? OR base_url ILIKE ?", like, like)
	}
	if status := strings.TrimSpace(input.Status); status == "active" || status == "inactive" {
		query = query.Where("status = ?", status)
	}
	if compatible := strings.TrimSpace(input.Compatible); compatible != "" {
		query = query.Where("compatible = ?", compatible)
	}
	return query
}

func upstreamListOrder(sort string) string {
	switch strings.TrimSpace(sort) {
	case "id_asc":
		return "u.id ASC"
	case "name_asc":
		return "u.name ASC, u.id DESC"
	case "updated_desc":
		return "u.updated_at DESC, u.id DESC"
	case "id_desc":
		fallthrough
	default:
		return "u.id DESC"
	}
}

// ---------------------------------------------------------------------------
// 模型管理
// ---------------------------------------------------------------------------

// CreateModel 创建平台模型。
func (r *Repo) CreateModel(ctx context.Context, item *domainchannel.PlatformModel) error {
	if item == nil {
		return repository.ErrInvalidInput
	}
	entity := toPlatformModelModel(item)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entity.SortOrder == 0 {
			var maxSortOrder int
			if err := tx.
				Model(&model.LLMPlatformModel{}).
				Select("COALESCE(MAX(sort_order), 0)").
				Scan(&maxSortOrder).Error; err != nil {
				return translateError(err)
			}
			entity.SortOrder = maxSortOrder + 100
		}
		if err := tx.Create(&entity).Error; err != nil {
			return translateError(err)
		}
		return nil
	}); err != nil {
		return err
	}
	*item = toPlatformModelDomain(entity)
	return nil
}

// UpdateModel 更新平台模型。
func (r *Repo) UpdateModel(ctx context.Context, modelID uint, input repository.UpdateChannelModelInput) error {
	updates := make(map[string]interface{})
	if input.PlatformModelName != nil {
		updates["name"] = *input.PlatformModelName
	}
	if input.Vendor != nil {
		updates["vendor"] = *input.Vendor
	}
	if input.KindsJSON != nil {
		updates["kinds_json"] = *input.KindsJSON
	}
	if input.Icon != nil {
		updates["icon"] = *input.Icon
	}
	if input.CapabilitiesJSON != nil {
		updates["capabilities_json"] = *input.CapabilitiesJSON
	}
	if input.SystemPrompt != nil {
		updates["system_prompt"] = *input.SystemPrompt
	}
	if input.AccessScope != nil {
		updates["access_scope"] = *input.AccessScope
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&model.LLMPlatformModel{}).
		Where("id = ?", modelID).
		Updates(updates)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrModelNotFound
	}
	return nil
}

// ReorderModels 按指定子序列调整模型顺序，并归一化全量 sort_order。
func (r *Repo) ReorderModels(ctx context.Context, orderedModelIDs []uint) error {
	if len(orderedModelIDs) == 0 {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []model.LLMPlatformModel
		if err := tx.
			Select("id").
			Order("sort_order ASC, id ASC").
			Find(&rows).Error; err != nil {
			return translateError(err)
		}

		allIDs := make([]uint, 0, len(rows))
		existingIDs := make(map[uint]struct{}, len(rows))
		for _, row := range rows {
			allIDs = append(allIDs, row.ID)
			existingIDs[row.ID] = struct{}{}
		}

		reorderedIDs := make(map[uint]struct{}, len(orderedModelIDs))
		for _, modelID := range orderedModelIDs {
			if _, exists := reorderedIDs[modelID]; exists {
				return repository.ErrInvalidInput
			}
			if _, exists := existingIDs[modelID]; !exists {
				return ErrModelNotFound
			}
			reorderedIDs[modelID] = struct{}{}
		}

		nextIDs := make([]uint, 0, len(allIDs))
		reorderedIndex := 0
		for _, modelID := range allIDs {
			if _, exists := reorderedIDs[modelID]; exists {
				nextIDs = append(nextIDs, orderedModelIDs[reorderedIndex])
				reorderedIndex++
				continue
			}
			nextIDs = append(nextIDs, modelID)
		}
		if reorderedIndex != len(orderedModelIDs) {
			return repository.ErrInvalidInput
		}

		for index, modelID := range nextIDs {
			sortOrder := (index + 1) * 100
			if err := tx.
				Model(&model.LLMPlatformModel{}).
				Where("id = ?", modelID).
				Update("sort_order", sortOrder).Error; err != nil {
				return translateError(err)
			}
		}
		return nil
	})
}

// GetModelByID 按 ID 获取平台模型。
func (r *Repo) GetModelByID(ctx context.Context, modelID uint) (*domainchannel.PlatformModel, error) {
	var item model.LLMPlatformModel
	if err := r.db.WithContext(ctx).Where("id = ?", modelID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelNotFound
		}
		return nil, translateError(err)
	}
	result := toPlatformModelDomain(item)
	return &result, nil
}

// GetModelByName 按平台模型名获取平台模型。
func (r *Repo) GetModelByName(ctx context.Context, platformModelName string) (*domainchannel.PlatformModel, error) {
	var item model.LLMPlatformModel
	if err := r.db.WithContext(ctx).
		Where("name = ?", strings.TrimSpace(platformModelName)).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelNotFound
		}
		return nil, translateError(err)
	}
	result := toPlatformModelDomain(item)
	return &result, nil
}

// GetActiveModelByName 按平台模型名获取启用平台模型。
func (r *Repo) GetActiveModelByName(ctx context.Context, platformModelName string) (*domainchannel.PlatformModel, error) {
	var item model.LLMPlatformModel
	if err := r.db.WithContext(ctx).
		Where("name = ? AND status = ?", strings.TrimSpace(platformModelName), "active").
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelNotFound
		}
		return nil, translateError(err)
	}
	result := toPlatformModelDomain(item)
	return &result, nil
}

// GetModelListRowByID 按 ID 获取带来源统计的平台模型列表行。
func (r *Repo) GetModelListRowByID(ctx context.Context, modelID uint) (*ModelListRow, error) {
	var item ModelListRow
	if err := r.modelListQuery(ctx).Where("m.id = ?", modelID).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModelNotFound
		}
		return nil, translateError(err)
	}
	return &item, nil
}

// ListModels 分页查询平台模型。
func (r *Repo) ListModels(ctx context.Context, input repository.ListChannelModelsInput) ([]ModelListRow, int64, error) {
	items := make([]ModelListRow, 0)
	var total int64

	query := applyModelListFilters(r.db.WithContext(ctx).Table("llm_platform_models AS m"), input)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}

	listQuery := r.modelListQuery(ctx)
	listQuery = applyModelListFilters(listQuery, input)
	if err := listQuery.
		Order(modelListOrder(input.Sort)).
		Offset(input.Offset).
		Limit(input.Limit).
		Scan(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return items, total, nil
}

func (r *Repo) modelListQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("llm_platform_models AS m").
		Select(
			"m.id, m.name AS platform_model_name, m.vendor, m.kinds_json, m.icon, m.capabilities_json, m.system_prompt, m.access_scope, m.status, m.description, m.sort_order, m.created_at, m.updated_at, " +
				"COALESCE(stats.source_count, 0) AS source_count, COALESCE(stats.active_source_count, 0) AS active_source_count, COALESCE(stats.protocols_json, '[]') AS protocols_json",
		).
		Joins(
			`LEFT JOIN (
				SELECT r.platform_model_id,
					COUNT(*) AS source_count,
					COUNT(*) FILTER (WHERE r.status = 'active' AND um.status = 'active' AND u.status = 'active') AS active_source_count,
					COALESCE(json_agg(DISTINCT r.protocol) FILTER (WHERE r.status = 'active' AND um.status = 'active' AND u.status = 'active' AND r.protocol != ''), '[]') AS protocols_json
				FROM llm_model_routes r
				JOIN llm_upstream_models um ON um.id = r.upstream_model_id
				JOIN llm_upstreams u ON u.id = um.upstream_id
				GROUP BY r.platform_model_id
			) AS stats ON stats.platform_model_id = m.id`,
		)
}

func applyModelListFilters(query *gorm.DB, input repository.ListChannelModelsInput) *gorm.DB {
	if input.OnlyActive {
		query = query.Where("m.status = ?", "active")
	} else if status := strings.TrimSpace(input.Status); status == "active" || status == "inactive" {
		query = query.Where("m.status = ?", status)
	}
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("m.name ILIKE ? OR m.vendor ILIKE ? OR m.description ILIKE ?", like, like, like)
	}
	if vendor := strings.TrimSpace(input.Vendor); vendor != "" {
		query = query.Where("m.vendor = ?", vendor)
	}
	if protocol := strings.TrimSpace(input.Protocol); protocol != "" {
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM llm_model_routes r
				JOIN llm_upstream_models um ON um.id = r.upstream_model_id
				JOIN llm_upstreams u ON u.id = um.upstream_id
				WHERE r.platform_model_id = m.id
					AND r.protocol = ?
					AND r.status = 'active'
					AND um.status = 'active'
					AND u.status = 'active'
			)`,
			protocol,
		)
	}
	return query
}

func modelListOrder(sort string) string {
	switch strings.TrimSpace(sort) {
	case "id_desc":
		return "m.id DESC"
	case "platformModelName_asc":
		return "m.name ASC, m.id DESC"
	case "sourceCount_desc":
		return "source_count DESC, m.id DESC"
	case "sortOrder_asc":
		return "m.sort_order ASC, m.id ASC"
	case "updated_desc":
		return "m.updated_at DESC, m.id DESC"
	default:
		return "m.sort_order ASC, m.id ASC"
	}
}

// ---------------------------------------------------------------------------
// 上游真实模型与平台路由
// ---------------------------------------------------------------------------

// UpsertUpstreamModel 新增或更新上游真实模型。
func (r *Repo) UpsertUpstreamModel(ctx context.Context, item *domainchannel.UpstreamModel) error {
	entity := toUpstreamModelModel(item)
	if entity.UpstreamID == 0 || strings.TrimSpace(entity.UpstreamModelName) == "" || strings.TrimSpace(entity.BindingCode) == "" {
		return repository.ErrInvalidInput
	}
	var existing model.LLMUpstreamModel
	query := r.db.WithContext(ctx).
		Where("upstream_id = ? AND upstream_model_name = ?", entity.UpstreamID, entity.UpstreamModelName).
		Limit(1).
		Find(&existing)
	if query.Error != nil {
		return translateError(query.Error)
	}
	if query.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
			return translateError(err)
		}
		*item = toUpstreamModelDomain(entity)
		return nil
	}

	entity.ID = existing.ID
	if strings.TrimSpace(entity.BindingCode) == "" {
		entity.BindingCode = existing.BindingCode
	}

	if err := r.db.WithContext(ctx).
		Model(&model.LLMUpstreamModel{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"binding_code":        entity.BindingCode,
			"upstream_model_name": entity.UpstreamModelName,
			"vendor":              entity.Vendor,
			"icon":                entity.Icon,
			"suggested_protocol":  entity.SuggestedProtocol,
			"kinds_json":          entity.KindsJSON,
			"status":              entity.Status,
			"source":              entity.Source,
			"last_synced_at":      entity.LastSyncedAt,
			"raw_json":            entity.RawJSON,
		}).
		Error; err != nil {
		return translateError(err)
	}
	entity.ID = existing.ID
	*item = toUpstreamModelDomain(entity)
	return nil
}

// GetUpstreamModelByID 查询单条上游真实模型。
func (r *Repo) GetUpstreamModelByID(ctx context.Context, sourceID uint, upstreamID uint) (*domainchannel.UpstreamModel, error) {
	var item model.LLMUpstreamModel
	if err := r.db.WithContext(ctx).
		Where("id = ? AND upstream_id = ?", sourceID, upstreamID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamModelNotFound
		}
		return nil, translateError(err)
	}
	result := toUpstreamModelDomain(item)
	return &result, nil
}

// GetUpstreamModelByUpstreamName 查询单条上游真实模型。
func (r *Repo) GetUpstreamModelByUpstreamName(ctx context.Context, upstreamID uint, upstreamModelName string) (*domainchannel.UpstreamModel, error) {
	var item model.LLMUpstreamModel
	if err := r.db.WithContext(ctx).
		Where("upstream_id = ? AND upstream_model_name = ?", upstreamID, strings.TrimSpace(upstreamModelName)).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamModelNotFound
		}
		return nil, translateError(err)
	}
	result := toUpstreamModelDomain(item)
	return &result, nil
}

// UpdateUpstreamModelByID 更新单条上游真实模型。
func (r *Repo) UpdateUpstreamModelByID(
	ctx context.Context,
	sourceID uint,
	upstreamID uint,
	input repository.UpdateChannelUpstreamModelInput,
) error {
	updates := upstreamModelUpdates(input)
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&model.LLMUpstreamModel{}).
		Where("id = ? AND upstream_id = ?", sourceID, upstreamID).
		Updates(updates)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUpstreamModelNotFound
	}
	return nil
}

func upstreamModelUpdates(input repository.UpdateChannelUpstreamModelInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.UpstreamModelName != nil {
		updates["upstream_model_name"] = *input.UpstreamModelName
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Source != nil {
		updates["source"] = *input.Source
	}
	if input.SuggestedProtocol != nil {
		updates["suggested_protocol"] = *input.SuggestedProtocol
	}
	if input.KindsJSON != nil {
		updates["kinds_json"] = *input.KindsJSON
	}
	if input.LastSyncedAt != nil {
		updates["last_synced_at"] = *input.LastSyncedAt
	}
	if input.RawJSON != nil {
		updates["raw_json"] = *input.RawJSON
	}
	return updates
}

// DeleteUpstreamModel 硬删除单条上游真实模型及其平台路由。
func (r *Repo) DeleteUpstreamModel(ctx context.Context, sourceID uint, upstreamID uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.LLMUpstreamModel
		if err := tx.Where("id = ? AND upstream_id = ?", sourceID, upstreamID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUpstreamModelNotFound
			}
			return err
		}
		if err := tx.Where("upstream_model_id = ?", item.ID).Delete(&model.LLMPlatformModelRoute{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.LLMUpstreamModel{}, item.ID).Error
	}))
}

// MarkMissingSyncedUpstreamModelsInactive 将本次同步未返回的同步来源模型置为停用。
func (r *Repo) MarkMissingSyncedUpstreamModelsInactive(ctx context.Context, upstreamID uint, activeNames []string) (int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.LLMUpstreamModel{}).
		Where("upstream_id = ? AND source = ? AND status = ?", upstreamID, "sync", "active")
	if len(activeNames) > 0 {
		query = query.Where("upstream_model_name NOT IN ?", activeNames)
	}
	result := query.Update("status", "inactive")
	if result.Error != nil {
		return 0, translateError(result.Error)
	}
	return result.RowsAffected, nil
}

// ---------------------------------------------------------------------------
// 上游模型列表与查询
// ---------------------------------------------------------------------------

// ListUpstreamModels 查询上游真实模型及其路由绑定。结果为扁平行：每条路由一行，无路由的上游模型单独一行。
func (r *Repo) ListUpstreamModels(ctx context.Context, upstreamID uint, input repository.ListChannelUpstreamModelsInput) ([]UpstreamModelListRow, int64, error) {
	items := make([]UpstreamModelListRow, 0)
	var total int64

	countQuery := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Joins("LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("LEFT JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ?", upstreamID)
	countQuery = applyUpstreamModelListFilters(countQuery, input)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	listQuery := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Select(
			"um.*, r.id AS route_id, r.platform_model_id, pm.name AS platform_model_name, pm.vendor AS model_vendor, pm.kinds_json AS model_kinds_json, pm.icon AS model_icon, "+
				"r.protocol, r.status AS route_status, r.priority, r.weight, r.source AS route_source, "+
				"r.cb_failure_threshold, r.cb_duration_min, r.cb_window_min, r.headers_json",
		).
		Joins("LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("LEFT JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ?", upstreamID)
	listQuery = applyUpstreamModelListFilters(listQuery, input)
	if err := listQuery.
		Order(upstreamModelListOrder(input.Sort)).
		Offset(input.Offset).
		Limit(input.Limit).
		Scan(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return items, total, nil
}

// ListUpstreamModelsByNames 按远端模型名集合查询已有上游模型和绑定快照。
func (r *Repo) ListUpstreamModelsByNames(ctx context.Context, upstreamID uint, upstreamModelNames []string) ([]UpstreamModelListRow, error) {
	names := make([]string, 0, len(upstreamModelNames))
	seen := make(map[string]struct{}, len(upstreamModelNames))
	for _, raw := range upstreamModelNames {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return []UpstreamModelListRow{}, nil
	}
	items := make([]UpstreamModelListRow, 0)
	if err := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Select(
			"um.*, r.id AS route_id, r.platform_model_id, pm.name AS platform_model_name, pm.vendor AS model_vendor, pm.kinds_json AS model_kinds_json, pm.icon AS model_icon, "+
				"r.protocol, r.status AS route_status, r.priority, r.weight, r.source AS route_source, "+
				"r.cb_failure_threshold, r.cb_duration_min, r.cb_window_min, r.headers_json",
		).
		Joins("LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("LEFT JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ? AND um.upstream_model_name IN ?", upstreamID, names).
		Order("um.upstream_model_name ASC, r.id ASC NULLS LAST").
		Scan(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return items, nil
}

// GetUpstreamModelRouteByID 按路由 ID 精确查询上游模型绑定行。
func (r *Repo) GetUpstreamModelRouteByID(ctx context.Context, upstreamID uint, routeID uint) (*UpstreamModelListRow, error) {
	var item UpstreamModelListRow
	if err := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Select(
			"um.*, r.id AS route_id, r.platform_model_id, pm.name AS platform_model_name, pm.vendor AS model_vendor, pm.kinds_json AS model_kinds_json, pm.icon AS model_icon, "+
				"r.protocol, r.status AS route_status, r.priority, r.weight, r.source AS route_source, "+
				"r.cb_failure_threshold, r.cb_duration_min, r.cb_window_min, r.headers_json",
		).
		Joins("JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ? AND r.id = ?", upstreamID, routeID).
		Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamModelNotFound
		}
		return nil, translateError(err)
	}
	return &item, nil
}

// GetUpstreamModelRouteByNames 按平台模型、上游模型和协议精确查询绑定行。
func (r *Repo) GetUpstreamModelRouteByNames(
	ctx context.Context,
	upstreamID uint,
	platformModelName string,
	upstreamModelName string,
	protocol string,
) (*UpstreamModelListRow, error) {
	var item UpstreamModelListRow
	query := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Select(
			"um.*, r.id AS route_id, r.platform_model_id, pm.name AS platform_model_name, pm.vendor AS model_vendor, pm.kinds_json AS model_kinds_json, pm.icon AS model_icon, "+
				"r.protocol, r.status AS route_status, r.priority, r.weight, r.source AS route_source, "+
				"r.cb_failure_threshold, r.cb_duration_min, r.cb_window_min, r.headers_json",
		).
		Joins("JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ? AND pm.name = ? AND um.upstream_model_name = ?", upstreamID, strings.TrimSpace(platformModelName), strings.TrimSpace(upstreamModelName))
	if normalizedProtocol := strings.TrimSpace(protocol); normalizedProtocol != "" {
		query = query.Where("r.protocol = ?", normalizedProtocol)
	}
	if err := query.Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamModelNotFound
		}
		return nil, translateError(err)
	}
	return &item, nil
}

func applyUpstreamModelListFilters(query *gorm.DB, input repository.ListChannelUpstreamModelsInput) *gorm.DB {
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"um.upstream_model_name ILIKE ? OR um.binding_code ILIKE ? OR pm.name ILIKE ? OR r.protocol ILIKE ?",
			like,
			like,
			like,
			like,
		)
	}
	switch strings.TrimSpace(input.RouteStatus) {
	case "bound":
		query = query.Where("r.id IS NOT NULL")
	case "active", "inactive":
		query = query.Where("r.id IS NOT NULL AND r.status = ?", input.RouteStatus)
	}
	if status := strings.TrimSpace(input.UpstreamStatus); status == "active" || status == "inactive" {
		query = query.Where("um.status = ?", status)
	}
	if protocol := strings.TrimSpace(input.Protocol); protocol != "" {
		query = query.Where("r.protocol = ?", protocol)
	}
	return query
}

func upstreamModelListOrder(sort string) string {
	switch strings.TrimSpace(sort) {
	case "upstream_desc":
		return "um.upstream_model_name DESC, r.id ASC NULLS LAST"
	case "platform_asc":
		return "pm.name ASC NULLS LAST, um.upstream_model_name ASC, r.id ASC NULLS LAST"
	case "platform_desc":
		return "pm.name DESC NULLS LAST, um.upstream_model_name ASC, r.id ASC NULLS LAST"
	case "status_asc":
		return "CASE WHEN r.id IS NULL THEN 2 WHEN r.status = 'active' THEN 0 ELSE 1 END ASC, um.upstream_model_name ASC, r.id ASC NULLS LAST"
	case "protocol_asc":
		return "r.protocol ASC NULLS LAST, um.upstream_model_name ASC, r.id ASC NULLS LAST"
	case "upstream_asc":
		fallthrough
	default:
		return "um.upstream_model_name ASC, r.id ASC NULLS LAST"
	}
}

// UpsertPlatformModelRoute 新增或更新平台模型到上游真实模型的路由绑定。
func (r *Repo) UpsertPlatformModelRoute(ctx context.Context, item *domainchannel.PlatformModelRoute) error {
	if item == nil || item.PlatformModelID == 0 || item.UpstreamModelID == 0 {
		return repository.ErrInvalidInput
	}
	entity := toPlatformModelRouteModel(item)
	var existing model.LLMPlatformModelRoute
	query := r.db.WithContext(ctx).
		Where(
			"platform_model_id = ? AND upstream_model_id = ? AND protocol = ?",
			entity.PlatformModelID,
			entity.UpstreamModelID,
			entity.Protocol,
		).
		Limit(1).
		Find(&existing)
	if query.Error != nil {
		return translateError(query.Error)
	}
	if query.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
			return translateError(err)
		}
		*item = toPlatformModelRouteDomain(entity)
		return nil
	}
	entity.ID = existing.ID
	if err := r.db.WithContext(ctx).
		Model(&model.LLMPlatformModelRoute{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"protocol":             entity.Protocol,
			"status":               entity.Status,
			"priority":             entity.Priority,
			"weight":               entity.Weight,
			"source":               entity.Source,
			"cb_failure_threshold": entity.CbFailureThreshold,
			"cb_duration_min":      entity.CbDurationMin,
			"cb_window_min":        entity.CbWindowMin,
			"headers_json":         entity.HeadersJSON,
		}).Error; err != nil {
		return translateError(err)
	}
	*item = toPlatformModelRouteDomain(entity)
	return nil
}

// ListPlatformModelRoutesByPair 查询同一平台模型和同一上游真实模型之间的全部协议绑定。
func (r *Repo) ListPlatformModelRoutesByPair(
	ctx context.Context,
	upstreamID uint,
	platformModelID uint,
	upstreamModelID uint,
) ([]domainchannel.PlatformModelRoute, error) {
	items := make([]model.LLMPlatformModelRoute, 0)
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("r.*").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Where("um.upstream_id = ? AND r.platform_model_id = ? AND r.upstream_model_id = ?", upstreamID, platformModelID, upstreamModelID).
		Order("r.id ASC").
		Scan(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainchannel.PlatformModelRoute, 0, len(items))
	for _, item := range items {
		results = append(results, toPlatformModelRouteDomain(item))
	}
	return results, nil
}

func (r *Repo) GetPlatformModelRouteByID(ctx context.Context, routeID uint, upstreamID uint) (*domainchannel.PlatformModelRoute, error) {
	var item model.LLMPlatformModelRoute
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("r.*").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Where("r.id = ? AND um.upstream_id = ?", routeID, upstreamID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamModelNotFound
		}
		return nil, translateError(err)
	}
	result := toPlatformModelRouteDomain(item)
	return &result, nil
}

func (r *Repo) UpdatePlatformModelRouteByID(ctx context.Context, routeID uint, upstreamID uint, input repository.UpdateChannelPlatformRouteInput) error {
	updates := platformRouteUpdates(input)
	if len(updates) == 0 {
		return nil
	}
	sub := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("r.id").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Where("r.id = ? AND um.upstream_id = ?", routeID, upstreamID)
	result := r.db.WithContext(ctx).
		Model(&model.LLMPlatformModelRoute{}).
		Where("id IN (?)", sub).
		Updates(updates)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUpstreamModelNotFound
	}
	return nil
}

func platformRouteUpdates(input repository.UpdateChannelPlatformRouteInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.PlatformModelID != nil {
		updates["platform_model_id"] = *input.PlatformModelID
	}
	if input.UpstreamModelID != nil {
		updates["upstream_model_id"] = *input.UpstreamModelID
	}
	if input.Protocol != nil {
		updates["protocol"] = *input.Protocol
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.Priority != nil {
		updates["priority"] = *input.Priority
	}
	if input.Weight != nil {
		updates["weight"] = *input.Weight
	}
	if input.Source != nil {
		updates["source"] = *input.Source
	}
	if input.CbFailureThreshold != nil {
		updates["cb_failure_threshold"] = *input.CbFailureThreshold
	}
	if input.CbDurationMin != nil {
		updates["cb_duration_min"] = *input.CbDurationMin
	}
	if input.CbWindowMin != nil {
		updates["cb_window_min"] = *input.CbWindowMin
	}
	if input.HeadersJSON != nil {
		updates["headers_json"] = *input.HeadersJSON
	}
	return updates
}

func (r *Repo) DeletePlatformModelRoute(ctx context.Context, routeID uint, upstreamID uint) error {
	sub := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("r.id").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Where("r.id = ? AND um.upstream_id = ?", routeID, upstreamID)
	result := r.db.WithContext(ctx).
		Unscoped().
		Where("id IN (?)", sub).
		Delete(&model.LLMPlatformModelRoute{})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUpstreamModelNotFound
	}
	return nil
}

// ListModelUpstreamSources 查询平台模型下的上游来源。
func (r *Repo) ListModelUpstreamSources(ctx context.Context, platformModelName string, offset int, limit int) ([]ModelSourceRow, int64, error) {
	items := make([]ModelSourceRow, 0)
	var total int64

	name := strings.TrimSpace(platformModelName)
	query := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("pm.name = ?", name)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select(
			"r.*, um.upstream_id, u.name AS upstream_name, u.base_url AS base_url, "+
				"um.binding_code, um.upstream_model_name, um.vendor AS upstream_model_vendor, um.icon AS upstream_model_icon, "+
				"um.kinds_json AS upstream_model_kinds_json, um.suggested_protocol, um.status AS upstream_model_status",
		).
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_upstreams u ON u.id = um.upstream_id").
		Where("pm.name = ?", name).
		Order("r.priority ASC, r.id DESC").
		Offset(offset).
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return items, total, nil
}

// GetModelUpstreamSourceByRouteID 按平台模型名和路由 ID 精确查询模型来源。
func (r *Repo) GetModelUpstreamSourceByRouteID(ctx context.Context, platformModelName string, routeID uint) (*ModelSourceRow, error) {
	var item ModelSourceRow
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select(
			"r.*, um.upstream_id, u.name AS upstream_name, u.base_url AS base_url, "+
				"um.binding_code, um.upstream_model_name, um.vendor AS upstream_model_vendor, um.icon AS upstream_model_icon, "+
				"um.kinds_json AS upstream_model_kinds_json, um.suggested_protocol, um.status AS upstream_model_status",
		).
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_upstreams u ON u.id = um.upstream_id").
		Where("pm.name = ? AND r.id = ?", strings.TrimSpace(platformModelName), routeID).
		Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUpstreamModelNotFound
		}
		return nil, translateError(err)
	}
	return &item, nil
}

// routeScanRow 是 ListActiveRoutesByModel 查询的原始扫描结构体。
// 仅限 infra 层内部使用，扫描后映射到 UpstreamRouteRow。
type routeScanRow struct {
	RouteID                    uint
	UpstreamModelID            uint
	UpstreamID                 uint
	UpstreamName               string
	PlatformModelID            uint
	PlatformModelName          string
	ModelVendor                string
	ModelIcon                  string
	ModelKindsJSON             string
	ModelCapabilitiesJSON      string
	ModelSystemPrompt          string
	Protocol                   string
	BaseURL                    string
	APIKeysEnc                 string
	ConnectTimeoutMS           int
	ReadTimeoutMS              int
	StreamIdleTimeoutMS        int
	HeadersJSON                string
	RouteHeadersJSON           string
	BindingCode                string
	UpstreamModelName          string
	Weight                     int
	RoutePriority              int
	UpstreamCbFailureThreshold int
	UpstreamCbModelThreshold   int
	UpstreamCbThresholdLogic   string
	UpstreamCbDurationMin      int
	UpstreamCbWindowMin        int
	ModelCbFailureThreshold    int
	ModelCbDurationMin         int
	ModelCbWindowMin           int
}

// ListActiveRoutesByModel 按平台模型名查询可用路由。
func (r *Repo) ListActiveRoutesByModel(ctx context.Context, platformModelName string) ([]UpstreamRouteRow, error) {
	scanned := make([]routeScanRow, 0)
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select(
			"r.id AS route_id, um.id AS upstream_model_id, u.id AS upstream_id, u.name AS upstream_name, "+
				"pm.id AS platform_model_id, pm.name AS platform_model_name, pm.vendor AS model_vendor, pm.icon AS model_icon, pm.kinds_json AS model_kinds_json, pm.capabilities_json AS model_capabilities_json, pm.system_prompt AS model_system_prompt, "+
				"r.protocol, u.base_url, u.api_keys_enc, "+
				"u.connect_timeout_ms, u.read_timeout_ms, u.stream_idle_timeout_ms, "+
				"u.headers_json, r.headers_json AS route_headers_json, "+
				"um.binding_code, um.upstream_model_name, r.weight, r.priority AS route_priority, "+
				"u.cb_failure_threshold AS upstream_cb_failure_threshold, "+
				"u.cb_model_threshold AS upstream_cb_model_threshold, "+
				"u.cb_threshold_logic AS upstream_cb_threshold_logic, "+
				"u.cb_duration_min AS upstream_cb_duration_min, "+
				"u.cb_window_min AS upstream_cb_window_min, "+
				"r.cb_failure_threshold AS model_cb_failure_threshold, "+
				"r.cb_duration_min AS model_cb_duration_min, "+
				"r.cb_window_min AS model_cb_window_min",
		).
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_upstreams u ON u.id = um.upstream_id").
		Where("pm.name = ? AND pm.status = ? AND r.status = ? AND um.status = ? AND u.status = ?", strings.TrimSpace(platformModelName), "active", "active", "active", "active").
		Order("r.priority ASC, r.id ASC").
		Scan(&scanned).Error; err != nil {
		return nil, translateError(err)
	}

	rows := make([]UpstreamRouteRow, 0, len(scanned))
	for _, s := range scanned {
		rows = append(rows, UpstreamRouteRow{
			RouteID:                    s.RouteID,
			UpstreamModelID:            s.UpstreamModelID,
			UpstreamID:                 s.UpstreamID,
			UpstreamName:               s.UpstreamName,
			PlatformModelID:            s.PlatformModelID,
			PlatformModelName:          s.PlatformModelName,
			ModelVendor:                s.ModelVendor,
			ModelIcon:                  s.ModelIcon,
			ModelKindsJSON:             s.ModelKindsJSON,
			ModelCapabilitiesJSON:      s.ModelCapabilitiesJSON,
			ModelSystemPrompt:          s.ModelSystemPrompt,
			Protocol:                   s.Protocol,
			BaseURL:                    s.BaseURL,
			APIKeysEnc:                 s.APIKeysEnc,
			ConnectTimeoutMS:           s.ConnectTimeoutMS,
			ReadTimeoutMS:              s.ReadTimeoutMS,
			StreamIdleTimeoutMS:        s.StreamIdleTimeoutMS,
			HeadersJSON:                s.HeadersJSON,
			RouteHeadersJSON:           s.RouteHeadersJSON,
			BindingCode:                s.BindingCode,
			UpstreamModelName:          s.UpstreamModelName,
			Weight:                     s.Weight,
			RoutePriority:              s.RoutePriority,
			UpstreamCbFailureThreshold: s.UpstreamCbFailureThreshold,
			UpstreamCbModelThreshold:   s.UpstreamCbModelThreshold,
			UpstreamCbThresholdLogic:   s.UpstreamCbThresholdLogic,
			UpstreamCbDurationMin:      s.UpstreamCbDurationMin,
			UpstreamCbWindowMin:        s.UpstreamCbWindowMin,
			ModelCbFailureThreshold:    s.ModelCbFailureThreshold,
			ModelCbDurationMin:         s.ModelCbDurationMin,
			ModelCbWindowMin:           s.ModelCbWindowMin,
		})
	}
	return rows, nil
}

// ListActiveRouteBindingCodesForUpstream 返回上游下所有启用路由的 bindingCode。
func (r *Repo) ListActiveRouteBindingCodesForUpstream(ctx context.Context, upstreamID uint) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("um.binding_code").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Where("um.upstream_id = ? AND r.status = ? AND um.status = ?", upstreamID, "active", "active").
		Pluck("um.binding_code", &codes).Error; err != nil {
		return nil, translateError(err)
	}
	return codes, nil
}

// ---------------------------------------------------------------------------
// 全局设置
// ---------------------------------------------------------------------------

// GetLLMSetting 按 key 获取 LLM 全局设置。
func (r *Repo) GetLLMSetting(ctx context.Context, key string) (*domainchannel.LLMSetting, error) {
	var item model.SystemSetting
	if err := r.db.WithContext(ctx).
		Where("namespace = ? AND key = ?", "llm", strings.TrimSpace(key)).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLLMSettingNotFound
		}
		return nil, translateError(err)
	}
	result := toLLMSettingDomain(item)
	return &result, nil
}

// ListLLMSettings 列出 LLM 全局设置。
func (r *Repo) ListLLMSettings(ctx context.Context) ([]domainchannel.LLMSetting, error) {
	items := make([]model.SystemSetting, 0)
	if err := r.db.WithContext(ctx).
		Where("namespace = ?", "llm").
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainchannel.LLMSetting, 0, len(items))
	for _, item := range items {
		results = append(results, toLLMSettingDomain(item))
	}
	return results, nil
}

// UpsertLLMSetting 新增或更新 LLM 全局设置。
func (r *Repo) UpsertLLMSetting(ctx context.Context, item *domainchannel.LLMSetting) error {
	entity := toLLMSettingModel(item)
	var existing model.SystemSetting
	query := r.db.WithContext(ctx).
		Where("namespace = ? AND key = ?", "llm", entity.Key).
		Limit(1).
		Find(&existing)
	if query.Error != nil {
		return translateError(query.Error)
	}
	if query.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
			return translateError(err)
		}
		*item = toLLMSettingDomain(entity)
		return nil
	}

	entity.ID = existing.ID

	if err := r.db.WithContext(ctx).
		Model(&model.SystemSetting{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"value":       entity.Value,
			"description": entity.Description,
		}).
		Error; err != nil {
		return translateError(err)
	}
	entity.ID = existing.ID
	*item = toLLMSettingDomain(entity)
	return nil
}

// DeleteUpstreamCascade 硬删除上游及其全部绑定，保留模型目录。
func (r *Repo) DeleteUpstreamCascade(ctx context.Context, upstreamID uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.LLMUpstream
		if err := tx.Where("id = ?", upstreamID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUpstreamNotFound
			}
			return err
		}
		upstreamModelIDs := tx.Model(&model.LLMUpstreamModel{}).
			Select("id").
			Where("upstream_id = ?", upstreamID)
		if err := tx.Where("upstream_model_id IN (?)", upstreamModelIDs).Delete(&model.LLMPlatformModelRoute{}).Error; err != nil {
			return err
		}
		if err := tx.Where("upstream_id = ?", upstreamID).Delete(&model.LLMUpstreamModel{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.LLMUpstream{}, upstreamID).Error; err != nil {
			return err
		}
		return nil
	}))
}

// DeleteModelCascade 硬删除平台模型及其全部路由绑定，保留上游真实模型清单。
func (r *Repo) DeleteModelCascade(ctx context.Context, modelID uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.LLMPlatformModel
		if err := tx.Where("id = ?", modelID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrModelNotFound
			}
			return err
		}
		if err := tx.Where("platform_model_id = ?", item.ID).Delete(&model.LLMPlatformModelRoute{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.LLMPlatformModel{}, modelID).Error; err != nil {
			return err
		}
		return nil
	}))
}

func toUpstreamDomain(item model.LLMUpstream) domainchannel.Upstream {
	return domainchannel.Upstream{
		ID:                   item.ID,
		Name:                 item.Name,
		BaseURL:              item.BaseURL,
		Compatible:           item.Compatible,
		ProtocolDefaultsJSON: item.ProtocolDefaultsJSON,
		Status:               item.Status,
		ConnectTimeoutMS:     item.ConnectTimeoutMS,
		ReadTimeoutMS:        item.ReadTimeoutMS,
		StreamIdleTimeoutMS:  item.StreamIdleTimeoutMS,
		APIKeysEnc:           item.APIKeysEnc,
		CbFailureThreshold:   item.CbFailureThreshold,
		CbModelThreshold:     item.CbModelThreshold,
		CbThresholdLogic:     item.CbThresholdLogic,
		CbDurationMin:        item.CbDurationMin,
		CbWindowMin:          item.CbWindowMin,
		HeadersJSON:          item.HeadersJSON,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func toUpstreamModel(item *domainchannel.Upstream) model.LLMUpstream {
	if item == nil {
		return model.LLMUpstream{}
	}
	return model.LLMUpstream{
		Name:                 item.Name,
		BaseURL:              item.BaseURL,
		Compatible:           item.Compatible,
		ProtocolDefaultsJSON: item.ProtocolDefaultsJSON,
		Status:               item.Status,
		ConnectTimeoutMS:     item.ConnectTimeoutMS,
		ReadTimeoutMS:        item.ReadTimeoutMS,
		StreamIdleTimeoutMS:  item.StreamIdleTimeoutMS,
		APIKeysEnc:           item.APIKeysEnc,
		CbFailureThreshold:   item.CbFailureThreshold,
		CbModelThreshold:     item.CbModelThreshold,
		CbThresholdLogic:     item.CbThresholdLogic,
		CbDurationMin:        item.CbDurationMin,
		CbWindowMin:          item.CbWindowMin,
		HeadersJSON:          item.HeadersJSON,
	}
}

func toPlatformModelDomain(item model.LLMPlatformModel) domainchannel.PlatformModel {
	return domainchannel.PlatformModel{
		ID:                item.ID,
		PlatformModelName: item.Name,
		Vendor:            item.Vendor,
		KindsJSON:         item.KindsJSON,
		Icon:              item.Icon,
		CapabilitiesJSON:  item.CapabilitiesJSON,
		SystemPrompt:      item.SystemPrompt,
		AccessScope:       item.AccessScope,
		Status:            item.Status,
		Description:       item.Description,
		SortOrder:         item.SortOrder,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toPlatformModelModel(item *domainchannel.PlatformModel) model.LLMPlatformModel {
	if item == nil {
		return model.LLMPlatformModel{}
	}
	return model.LLMPlatformModel{
		Name:             item.PlatformModelName,
		Vendor:           item.Vendor,
		KindsJSON:        item.KindsJSON,
		Icon:             item.Icon,
		CapabilitiesJSON: item.CapabilitiesJSON,
		SystemPrompt:     item.SystemPrompt,
		AccessScope:      item.AccessScope,
		Status:           item.Status,
		Description:      item.Description,
		SortOrder:        item.SortOrder,
	}
}

func toUpstreamModelDomain(item model.LLMUpstreamModel) domainchannel.UpstreamModel {
	return domainchannel.UpstreamModel{
		ID:                item.ID,
		UpstreamID:        item.UpstreamID,
		BindingCode:       item.BindingCode,
		UpstreamModelName: item.UpstreamModelName,
		Vendor:            item.Vendor,
		Icon:              item.Icon,
		SuggestedProtocol: item.SuggestedProtocol,
		KindsJSON:         item.KindsJSON,
		Status:            item.Status,
		Source:            item.Source,
		LastSyncedAt:      item.LastSyncedAt,
		RawJSON:           item.RawJSON,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toUpstreamModelModel(item *domainchannel.UpstreamModel) model.LLMUpstreamModel {
	if item == nil {
		return model.LLMUpstreamModel{}
	}
	return model.LLMUpstreamModel{
		UpstreamID:        item.UpstreamID,
		BindingCode:       item.BindingCode,
		UpstreamModelName: item.UpstreamModelName,
		Vendor:            item.Vendor,
		Icon:              item.Icon,
		SuggestedProtocol: item.SuggestedProtocol,
		KindsJSON:         item.KindsJSON,
		Status:            item.Status,
		Source:            item.Source,
		LastSyncedAt:      item.LastSyncedAt,
		RawJSON:           item.RawJSON,
	}
}

func toPlatformModelRouteDomain(item model.LLMPlatformModelRoute) domainchannel.PlatformModelRoute {
	return domainchannel.PlatformModelRoute{
		ID:                 item.ID,
		PlatformModelID:    item.PlatformModelID,
		UpstreamModelID:    item.UpstreamModelID,
		Protocol:           item.Protocol,
		Status:             item.Status,
		Priority:           item.Priority,
		Weight:             item.Weight,
		Source:             item.Source,
		CbFailureThreshold: item.CbFailureThreshold,
		CbDurationMin:      item.CbDurationMin,
		CbWindowMin:        item.CbWindowMin,
		HeadersJSON:        item.HeadersJSON,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func toPlatformModelRouteModel(item *domainchannel.PlatformModelRoute) model.LLMPlatformModelRoute {
	if item == nil {
		return model.LLMPlatformModelRoute{}
	}
	return model.LLMPlatformModelRoute{
		PlatformModelID:    item.PlatformModelID,
		UpstreamModelID:    item.UpstreamModelID,
		Protocol:           item.Protocol,
		Status:             item.Status,
		Priority:           item.Priority,
		Weight:             item.Weight,
		Source:             item.Source,
		CbFailureThreshold: item.CbFailureThreshold,
		CbDurationMin:      item.CbDurationMin,
		CbWindowMin:        item.CbWindowMin,
		HeadersJSON:        item.HeadersJSON,
	}
}

func toLLMSettingDomain(item model.SystemSetting) domainchannel.LLMSetting {
	return domainchannel.LLMSetting{
		ID:          item.ID,
		Key:         item.Key,
		Value:       item.Value,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toLLMSettingModel(item *domainchannel.LLMSetting) model.SystemSetting {
	if item == nil {
		return model.SystemSetting{}
	}
	return model.SystemSetting{
		Namespace:   "llm",
		Key:         item.Key,
		Value:       item.Value,
		ValueType:   "json",
		Description: item.Description,
	}
}
