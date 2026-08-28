package channel

import (
	"context"
	"encoding/json"
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
	"gorm.io/gorm/clause"
)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
// channel 包内部的语义错误（ErrUpstreamNotFound 等）优先在调用点直接返回，
// 此函数处理未在调用点明确转换的 gorm 错误。
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	if dberror.IsUniqueConstraint(err) {
		return repository.ErrDuplicate
	}
	return err
}

// Repo 封装上游域数据访问。
type Repo struct {
	db            *gorm.DB
	inTransaction bool
}

// WithinTransaction 在同一数据库事务中执行渠道仓储操作。
func (r *Repo) WithinTransaction(ctx context.Context, fn func(repository.ChannelRepository) error) error {
	if fn == nil {
		return repository.ErrInvalidInput
	}
	return translateError(r.transact(ctx, func(tx *gorm.DB) error {
		return fn(&Repo{db: tx, inTransaction: true})
	}))
}

func (r *Repo) transact(ctx context.Context, fn func(*gorm.DB) error) error {
	if r.inTransaction {
		return fn(r.db.WithContext(ctx))
	}
	return r.db.WithContext(ctx).Transaction(fn)
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
		Joins(upstreamListStatsJoinSQL())
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

func (r *Repo) GetUpstreamListRowByID(ctx context.Context, upstreamID uint) (*UpstreamListRow, error) {
	var item UpstreamListRow
	result := r.db.WithContext(ctx).
		Table("llm_upstreams AS u").
		Select(
			"u.*, COALESCE(stats.models_count, 0) AS models_count, COALESCE(stats.active_models_count, 0) AS active_models_count",
		).
		Joins(upstreamListStatsJoinSQL()).
		Where("u.id = ?", upstreamID).
		Scan(&item)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrUpstreamNotFound
	}
	return &item, nil
}

func upstreamListStatsJoinSQL() string {
	return `LEFT JOIN (
		SELECT um.upstream_id,
			COUNT(DISTINCT CASE WHEN r.id IS NOT NULL THEN um.id END) AS models_count,
			COUNT(DISTINCT CASE WHEN u.status = 'active' AND r.status = 'active' AND um.status = 'active' AND pm.status = 'active' THEN um.id END) AS active_models_count
		FROM llm_upstream_models um
		LEFT JOIN llm_upstreams u ON u.id = um.upstream_id
		LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id
		LEFT JOIN llm_platform_models pm ON pm.id = r.platform_model_id
		GROUP BY um.upstream_id
	) AS stats ON stats.upstream_id = u.id`
}

func applyUpstreamListFilters(query *gorm.DB, input repository.ListChannelUpstreamsInput) *gorm.DB {
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(base_url) LIKE ?", like, like)
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
	if err := r.transact(ctx, func(tx *gorm.DB) error {
		if err := lockModelPresentationReferences(tx, entity.Vendor, entity.DisplayGroupID); err != nil {
			return err
		}
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
	if input.DisplayGroupID != nil {
		if *input.DisplayGroupID == 0 {
			updates["display_group_id"] = nil
		} else {
			updates["display_group_id"] = *input.DisplayGroupID
		}
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
	if input.CbPolicyMode != nil {
		updates["cb_policy_mode"] = *input.CbPolicyMode
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
	if len(updates) == 0 {
		return nil
	}
	return r.transact(ctx, func(tx *gorm.DB) error {
		vendor := ""
		if input.Vendor != nil {
			vendor = *input.Vendor
		}
		var displayGroupID *uint
		if input.DisplayGroupID != nil && *input.DisplayGroupID > 0 {
			displayGroupID = input.DisplayGroupID
		}
		if err := lockModelPresentationReferences(tx, vendor, displayGroupID); err != nil {
			return err
		}
		result := tx.Model(&model.LLMPlatformModel{}).
			Where("id = ?", modelID).
			Updates(updates)
		if result.Error != nil {
			return translateError(result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrModelNotFound
		}
		return nil
	})
}

// ReorderModels 按指定子序列调整模型顺序，仅更新提交的模型。
func (r *Repo) ReorderModels(ctx context.Context, orderedModelIDs []uint) error {
	if len(orderedModelIDs) == 0 {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingRows []model.LLMPlatformModel
		if err := tx.
			Select("id").
			Where("id IN ?", orderedModelIDs).
			Find(&existingRows).Error; err != nil {
			return translateError(err)
		}
		existingIDs := make(map[uint]struct{}, len(existingRows))
		for _, row := range existingRows {
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

		for index, modelID := range orderedModelIDs {
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

// GetActiveRoutableModelKindsJSON 查询存在可用路由的启用模型能力类型。
func (r *Repo) GetActiveRoutableModelKindsJSON(ctx context.Context, platformModelName string) (string, bool, error) {
	var result struct {
		KindsJSON string
	}
	dbResult := r.db.WithContext(ctx).
		Table("llm_platform_models AS pm").
		Select("pm.kinds_json").
		Joins("JOIN llm_model_routes AS r ON r.platform_model_id = pm.id AND r.status = ?", "active").
		Joins("JOIN llm_upstream_models AS um ON um.id = r.upstream_model_id AND um.status = ?", "active").
		Joins("JOIN llm_upstreams AS u ON u.id = um.upstream_id AND u.status = ?", "active").
		Where("pm.name = ? AND pm.status = ?", strings.TrimSpace(platformModelName), "active").
		Limit(1).
		Scan(&result)
	if dbResult.Error != nil {
		return "", false, translateError(dbResult.Error)
	}
	if dbResult.RowsAffected == 0 {
		return "", false, nil
	}
	return result.KindsJSON, true, nil
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
	items := []ModelListRow{item}
	if err := r.applyModelListRouteMetadata(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// ListModels 分页查询平台模型。
func (r *Repo) ListModels(ctx context.Context, input repository.ListChannelModelsInput) ([]ModelListRow, int64, error) {
	items := make([]ModelListRow, 0)
	var total int64

	query := applyModelListFilters(r.modelListBaseQuery(ctx), input)
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
	if err := r.applyModelListRouteMetadata(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repo) modelListQuery(ctx context.Context) *gorm.DB {
	return r.modelListBaseQuery(ctx).
		Select(
			"m.id, m.name AS platform_model_name, m.vendor, m.display_group_id, m.kinds_json, m.icon, m.capabilities_json, m.system_prompt, m.access_scope, m.status, m.description, m.cb_policy_mode, m.cb_failure_threshold, m.cb_duration_min, m.cb_window_min, m.sort_order, m.created_at, m.updated_at, " +
				"COALESCE(v.name, m.vendor) AS vendor_name, COALESCE(v.icon, '') AS vendor_icon, COALESCE(g.name, '') AS display_group_name, COALESCE(g.icon, '') AS display_group_icon, " +
				"COALESCE(stats.source_count, 0) AS source_count, COALESCE(stats.active_source_count, 0) AS active_source_count, '[]' AS protocols_json, '[]' AS upstream_names_json",
		).
		Joins(
			`LEFT JOIN (
				SELECT r.platform_model_id,
					COUNT(r.id) AS source_count,
					SUM(CASE WHEN r.status = 'active' AND um.status = 'active' AND u.status = 'active' THEN 1 ELSE 0 END) AS active_source_count
				FROM llm_model_routes r
				JOIN llm_upstream_models um ON um.id = r.upstream_model_id
				JOIN llm_upstreams u ON u.id = um.upstream_id
				GROUP BY r.platform_model_id
			) AS stats ON stats.platform_model_id = m.id`,
		)
}

func (r *Repo) modelListBaseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("llm_platform_models AS m").
		Joins("LEFT JOIN llm_model_vendors AS v ON v.key = m.vendor").
		Joins("LEFT JOIN llm_model_display_groups AS g ON g.id = m.display_group_id")
}

func (r *Repo) applyModelListRouteMetadata(ctx context.Context, items []ModelListRow) error {
	if len(items) == 0 {
		return nil
	}

	modelIDs := make([]uint, 0, len(items))
	indexByModelID := make(map[uint]int, len(items))
	for index, item := range items {
		modelIDs = append(modelIDs, item.ID)
		indexByModelID[item.ID] = index
		items[index].ProtocolsJSON = "[]"
		items[index].UpstreamNamesJSON = "[]"
	}

	type routeMetadataRow struct {
		PlatformModelID uint
		Protocol        string
		UpstreamName    string
	}
	rows := make([]routeMetadataRow, 0)
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Select("DISTINCT r.platform_model_id, r.protocol, u.name AS upstream_name").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_upstreams u ON u.id = um.upstream_id").
		Where("r.platform_model_id IN ? AND r.status = ? AND um.status = ? AND u.status = ?", modelIDs, "active", "active", "active").
		Order("r.platform_model_id ASC, r.protocol ASC, u.name ASC").
		Scan(&rows).Error; err != nil {
		return translateError(err)
	}

	protocolsByModelID := make(map[uint]map[string]struct{})
	upstreamNamesByModelID := make(map[uint]map[string]struct{})
	for _, row := range rows {
		protocol := strings.TrimSpace(row.Protocol)
		if protocol != "" {
			if protocolsByModelID[row.PlatformModelID] == nil {
				protocolsByModelID[row.PlatformModelID] = make(map[string]struct{})
			}
			protocolsByModelID[row.PlatformModelID][protocol] = struct{}{}
		}

		upstreamName := strings.TrimSpace(row.UpstreamName)
		if upstreamName != "" {
			if upstreamNamesByModelID[row.PlatformModelID] == nil {
				upstreamNamesByModelID[row.PlatformModelID] = make(map[string]struct{})
			}
			upstreamNamesByModelID[row.PlatformModelID][upstreamName] = struct{}{}
		}

	}
	for modelID, values := range protocolsByModelID {
		index, ok := indexByModelID[modelID]
		if !ok {
			continue
		}
		protocols := sortedStringSetValues(values)
		payload, err := json.Marshal(protocols)
		if err != nil {
			return err
		}
		items[index].ProtocolsJSON = string(payload)
	}
	for modelID, values := range upstreamNamesByModelID {
		index, ok := indexByModelID[modelID]
		if !ok {
			continue
		}
		upstreamNames := sortedStringSetValues(values)
		payload, err := json.Marshal(upstreamNames)
		if err != nil {
			return err
		}
		items[index].UpstreamNamesJSON = string(payload)
	}
	return nil
}

func sortedStringSetValues(values map[string]struct{}) []string {
	results := make([]string, 0, len(values))
	for value := range values {
		results = append(results, value)
	}
	sort.Strings(results)
	return results
}

func applyModelListFilters(query *gorm.DB, input repository.ListChannelModelsInput) *gorm.DB {
	if input.OnlyAvailable {
		query = query.Where("m.status = ?", "active")
		query = query.Where("COALESCE(NULLIF(TRIM(m.access_scope), ''), 'public') = ?", "public")
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM llm_model_routes r
				JOIN llm_upstream_models um ON um.id = r.upstream_model_id
				JOIN llm_upstreams u ON u.id = um.upstream_id
				WHERE r.platform_model_id = m.id
					AND r.status = 'active'
					AND um.status = 'active'
					AND u.status = 'active'
			)`,
		)
	} else if input.OnlyActive {
		query = query.Where("m.status = ?", "active")
	} else if status := strings.TrimSpace(input.Status); status == "active" || status == "inactive" {
		query = query.Where("m.status = ?", status)
	}
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			`LOWER(m.name) LIKE ?
				OR LOWER(m.vendor) LIKE ?
				OR LOWER(m.description) LIKE ?
				OR LOWER(v.name) LIKE ?
				OR LOWER(g.name) LIKE ?`,
			like, like, like, like, like,
		)
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
	if input.UpstreamID > 0 {
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM llm_model_routes r
				JOIN llm_upstream_models um ON um.id = r.upstream_model_id
				JOIN llm_upstreams u ON u.id = um.upstream_id
				WHERE r.platform_model_id = m.id
					AND u.id = ?
					AND r.status = 'active'
					AND um.status = 'active'
					AND u.status = 'active'
			)`,
			input.UpstreamID,
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
		return modelDefaultDisplayOrder()
	case "updated_desc":
		return "m.updated_at DESC, m.id DESC"
	default:
		return modelDefaultDisplayOrder()
	}
}

func modelDefaultDisplayOrder() string {
	availabilityRank := modelAvailabilityRankExpression()
	presentationKey := modelPresentationOrderKey("m.")
	presentationGroupOrder := "MIN(m.sort_order) OVER (PARTITION BY " + availabilityRank + ", " + presentationKey + ")"
	return availabilityRank + " ASC, " +
		presentationGroupOrder + " ASC, " +
		presentationKey + " ASC, " +
		"m.sort_order ASC, m.id ASC"
}

func modelAvailabilityRankExpression() string {
	return "CASE WHEN m.status = 'active' AND COALESCE(stats.active_source_count, 0) > 0 THEN 0 WHEN COALESCE(stats.source_count, 0) > 0 THEN 1 ELSE 2 END"
}

func modelVendorOrderKey(prefix string) string {
	return "COALESCE(NULLIF(TRIM(LOWER(" + prefix + "vendor)), ''), LOWER(" + prefix + "name))"
}

func modelPresentationOrderKey(prefix string) string {
	return "CASE WHEN " + prefix + "display_group_id IS NOT NULL " +
		"THEN 'group:' || CAST(" + prefix + "display_group_id AS TEXT) " +
		"ELSE 'vendor:' || " + modelVendorOrderKey(prefix) + " END"
}

// ---------------------------------------------------------------------------
// 上游真实模型与平台路由
// ---------------------------------------------------------------------------

// CreateUpstreamModel 新增上游真实模型，不覆盖同名目录项。
func (r *Repo) CreateUpstreamModel(ctx context.Context, item *domainchannel.UpstreamModel) error {
	entity := toUpstreamModelModel(item)
	if entity.UpstreamID == 0 || strings.TrimSpace(entity.UpstreamModelName) == "" || strings.TrimSpace(entity.BindingCode) == "" {
		return repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "upstream_id"}, {Name: "upstream_model_name"}},
			DoNothing: true,
		}).
		Create(&entity)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrDuplicate
	}
	*item = toUpstreamModelDomain(entity)
	return nil
}

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

type upstreamModelBindingPageKey struct {
	UpstreamModelID uint `gorm:"column:upstream_model_id"`
	PlatformModelID uint `gorm:"column:platform_model_id"`
}

// ListUpstreamModels 查询上游真实模型及其路由绑定。分页单位是平台模型与上游真实模型组成的绑定；
// 返回结果仍为扁平行：每条路由一行，无路由的上游模型单独一行。
func (r *Repo) ListUpstreamModels(ctx context.Context, upstreamID uint, input repository.ListChannelUpstreamModelsInput) ([]UpstreamModelListRow, int64, error) {
	groupedQuery := r.upstreamModelBindingGroupsQuery(ctx, upstreamID, input)
	var total int64
	if err := r.db.WithContext(ctx).
		Table("(?) AS binding_groups", groupedQuery).
		Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}

	pageKeys := make([]upstreamModelBindingPageKey, 0)
	if input.Limit > 0 {
		if err := r.upstreamModelBindingGroupsQuery(ctx, upstreamID, input).
			Order(upstreamModelBindingListOrder(input.Sort)).
			Offset(input.Offset).
			Limit(input.Limit).
			Scan(&pageKeys).Error; err != nil {
			return nil, 0, translateError(err)
		}
	}
	if len(pageKeys) == 0 {
		return []UpstreamModelListRow{}, total, nil
	}

	pagedGroups := r.upstreamModelBindingGroupsQuery(ctx, upstreamID, input).
		Order(upstreamModelBindingListOrder(input.Sort)).
		Offset(input.Offset).
		Limit(input.Limit)
	items := make([]UpstreamModelListRow, 0)
	listQuery := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Select(
			"um.*, r.id AS route_id, r.platform_model_id, pm.name AS platform_model_name, pm.vendor AS model_vendor, pm.kinds_json AS model_kinds_json, pm.icon AS model_icon, "+
				"r.protocol, r.status AS route_status, r.priority, r.weight, r.source AS route_source, "+
				"r.cb_failure_threshold, r.cb_duration_min, r.cb_window_min, r.headers_json",
		).
		Joins("LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("LEFT JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Joins(
			"JOIN (?) AS page_bindings ON page_bindings.upstream_model_id = um.id AND page_bindings.platform_model_id = COALESCE(r.platform_model_id, 0)",
			pagedGroups,
		).
		Where("um.upstream_id = ?", upstreamID)
	if err := listQuery.
		Order("um.id ASC, r.id ASC NULLS LAST").
		Scan(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}

	itemsByKey := make(map[string][]UpstreamModelListRow, len(pageKeys))
	for _, item := range items {
		key := upstreamModelBindingKey(item.UpstreamModel.ID, item.PlatformModelID)
		itemsByKey[key] = append(itemsByKey[key], item)
	}
	orderedItems := make([]UpstreamModelListRow, 0, len(items))
	for _, pageKey := range pageKeys {
		key := upstreamModelBindingKey(pageKey.UpstreamModelID, pageKey.PlatformModelID)
		orderedItems = append(orderedItems, itemsByKey[key]...)
	}
	return orderedItems, total, nil
}

func (r *Repo) upstreamModelBindingGroupsQuery(ctx context.Context, upstreamID uint, input repository.ListChannelUpstreamModelsInput) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table("llm_upstream_models AS um").
		Joins("LEFT JOIN llm_model_routes r ON r.upstream_model_id = um.id").
		Joins("LEFT JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ?", upstreamID)
	return applyUpstreamModelListFilters(query, input).
		Select("um.id AS upstream_model_id, COALESCE(r.platform_model_id, 0) AS platform_model_id").
		Group("um.id, r.platform_model_id, pm.name")
}

func upstreamModelBindingKey(upstreamModelID uint, platformModelID uint) string {
	return strconv.FormatUint(uint64(upstreamModelID), 10) + ":" + strconv.FormatUint(uint64(platformModelID), 10)
}

func upstreamModelBindingListOrder(sortValue string) string {
	switch strings.TrimSpace(sortValue) {
	case "upstream_desc":
		return "um.upstream_model_name DESC, um.id ASC, COALESCE(r.platform_model_id, 0) ASC"
	case "platform_asc":
		return "pm.name ASC NULLS LAST, um.upstream_model_name ASC, um.id ASC, COALESCE(r.platform_model_id, 0) ASC"
	case "platform_desc":
		return "pm.name DESC NULLS LAST, um.upstream_model_name ASC, um.id ASC, COALESCE(r.platform_model_id, 0) ASC"
	case "status_asc":
		return "CASE WHEN COUNT(r.id) = 0 THEN 2 WHEN MAX(CASE WHEN r.status = 'active' THEN 1 ELSE 0 END) = 1 THEN 0 ELSE 1 END ASC, um.upstream_model_name ASC, um.id ASC, COALESCE(r.platform_model_id, 0) ASC"
	case "protocol_asc":
		return "MIN(r.protocol) ASC NULLS LAST, um.upstream_model_name ASC, um.id ASC, COALESCE(r.platform_model_id, 0) ASC"
	case "upstream_asc":
		fallthrough
	default:
		return "um.upstream_model_name ASC, um.id ASC, COALESCE(r.platform_model_id, 0) ASC"
	}
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
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			"LOWER(um.upstream_model_name) LIKE ? OR LOWER(um.binding_code) LIKE ? OR LOWER(pm.name) LIKE ? OR LOWER(r.protocol) LIKE ?",
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

// ReplacePlatformModelRoutes 原子替换一组平台模型与上游真实模型绑定的完整协议集合。
func (r *Repo) ReplacePlatformModelRoutes(
	ctx context.Context,
	inputs []repository.ReplaceChannelPlatformRoutesInput,
) ([]domainchannel.PlatformModelRoute, error) {
	if len(inputs) == 0 {
		return nil, repository.ErrInvalidInput
	}

	replaced := make([]domainchannel.PlatformModelRoute, 0, len(inputs))
	err := r.transact(ctx, func(tx *gorm.DB) error {
		plans, err := loadPlatformRouteReplacementPlans(tx, inputs)
		if err != nil {
			return err
		}
		replaced, err = applyPlatformRouteReplacementPlans(tx, plans)
		return err
	})
	if err != nil {
		return nil, translateError(err)
	}
	return replaced, nil
}

type platformRouteBindingKey struct {
	platformModelID uint
	upstreamModelID uint
}

type platformRouteReplacementPlan struct {
	input      repository.ReplaceChannelPlatformRoutesInput
	sourceKey  platformRouteBindingKey
	targetKey  platformRouteBindingKey
	candidates []model.LLMPlatformModelRoute
}

func loadPlatformRouteReplacementPlans(
	tx *gorm.DB,
	inputs []repository.ReplaceChannelPlatformRoutesInput,
) ([]platformRouteReplacementPlan, error) {
	normalizedInputs := make([]repository.ReplaceChannelPlatformRoutesInput, len(inputs))
	targetBindings := make(map[platformRouteBindingKey]struct{}, len(inputs))
	selectedRouteSet := make(map[uint]struct{})
	selectedRouteIDs := make([]uint, 0)

	for index, rawInput := range inputs {
		input, targetKey, err := normalizePlatformRouteReplacement(rawInput)
		if err != nil {
			return nil, err
		}
		if _, exists := targetBindings[targetKey]; exists {
			return nil, repository.ErrInvalidInput
		}
		targetBindings[targetKey] = struct{}{}
		for _, routeID := range input.ExistingRouteIDs {
			if _, exists := selectedRouteSet[routeID]; exists {
				return nil, repository.ErrInvalidInput
			}
			selectedRouteSet[routeID] = struct{}{}
			selectedRouteIDs = append(selectedRouteIDs, routeID)
		}
		normalizedInputs[index] = input
	}

	selectedRows := make([]model.LLMPlatformModelRoute, 0, len(selectedRouteIDs))
	if len(selectedRouteIDs) > 0 {
		// 此处仅解析来源归属；先锁父模型、再锁完整路由集合，避免并发写入时形成 route -> model / model -> route 的交叉锁顺序。
		if err := tx.Where("id IN ?", selectedRouteIDs).
			Find(&selectedRows).Error; err != nil {
			return nil, err
		}
		if len(selectedRows) != len(selectedRouteIDs) {
			return nil, repository.ErrConflict
		}
	}
	selectedByID := make(map[uint]model.LLMPlatformModelRoute, len(selectedRows))
	for _, row := range selectedRows {
		selectedByID[row.ID] = row
	}

	plans := make([]platformRouteReplacementPlan, len(normalizedInputs))
	platformModelIDs := make(map[uint]struct{})
	upstreamModelIDs := make(map[uint]struct{})
	expectedUpstreamIDs := make(map[uint]uint)
	for index, input := range normalizedInputs {
		targetKey := platformRouteBindingKey{
			platformModelID: input.Routes[0].PlatformModelID,
			upstreamModelID: input.Routes[0].UpstreamModelID,
		}
		sourceKey := targetKey
		if len(input.ExistingRouteIDs) > 0 {
			first := selectedByID[input.ExistingRouteIDs[0]]
			sourceKey = platformRouteBindingKey{platformModelID: first.PlatformModelID, upstreamModelID: first.UpstreamModelID}
			for _, routeID := range input.ExistingRouteIDs[1:] {
				row := selectedByID[routeID]
				if row.PlatformModelID != sourceKey.platformModelID || row.UpstreamModelID != sourceKey.upstreamModelID {
					return nil, repository.ErrConflict
				}
			}
		}
		plans[index] = platformRouteReplacementPlan{input: input, sourceKey: sourceKey, targetKey: targetKey}
		platformModelIDs[sourceKey.platformModelID] = struct{}{}
		platformModelIDs[targetKey.platformModelID] = struct{}{}
		upstreamModelIDs[sourceKey.upstreamModelID] = struct{}{}
		upstreamModelIDs[targetKey.upstreamModelID] = struct{}{}
		if expected, exists := expectedUpstreamIDs[sourceKey.upstreamModelID]; exists && expected != input.UpstreamID {
			return nil, repository.ErrInvalidInput
		}
		expectedUpstreamIDs[sourceKey.upstreamModelID] = input.UpstreamID
		if expected, exists := expectedUpstreamIDs[targetKey.upstreamModelID]; exists && expected != input.UpstreamID {
			return nil, repository.ErrInvalidInput
		}
		expectedUpstreamIDs[targetKey.upstreamModelID] = input.UpstreamID
	}

	platformIDs := uintSetValues(platformModelIDs)
	lockedModels := make([]model.LLMPlatformModel, 0, len(platformIDs))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id IN ?", platformIDs).
		Find(&lockedModels).Error; err != nil {
		return nil, err
	}
	if len(lockedModels) != len(platformIDs) {
		return nil, ErrModelNotFound
	}

	upstreamModelIDList := uintSetValues(upstreamModelIDs)
	lockedUpstreamModels := make([]model.LLMUpstreamModel, 0, len(upstreamModelIDList))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "upstream_id").
		Where("id IN ?", upstreamModelIDList).
		Find(&lockedUpstreamModels).Error; err != nil {
		return nil, err
	}
	if len(lockedUpstreamModels) != len(upstreamModelIDList) {
		return nil, ErrUpstreamModelNotFound
	}
	for _, upstreamModel := range lockedUpstreamModels {
		if upstreamModel.UpstreamID != expectedUpstreamIDs[upstreamModel.ID] {
			return nil, ErrUpstreamModelNotFound
		}
	}

	allRows := make([]model.LLMPlatformModelRoute, 0, len(selectedRows))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("platform_model_id IN ? AND upstream_model_id IN ?", platformIDs, upstreamModelIDList).
		Order("id ASC").
		Find(&allRows).Error; err != nil {
		return nil, err
	}
	rowsByBinding := make(map[platformRouteBindingKey][]model.LLMPlatformModelRoute)
	for _, row := range allRows {
		key := platformRouteBindingKey{platformModelID: row.PlatformModelID, upstreamModelID: row.UpstreamModelID}
		rowsByBinding[key] = append(rowsByBinding[key], row)
	}

	bindingOwners := make(map[platformRouteBindingKey]int, len(plans)*2)
	for index := range plans {
		plan := &plans[index]
		if owner, exists := bindingOwners[plan.sourceKey]; exists && owner != index {
			return nil, repository.ErrInvalidInput
		}
		bindingOwners[plan.sourceKey] = index
		if owner, exists := bindingOwners[plan.targetKey]; exists && owner != index {
			return nil, repository.ErrInvalidInput
		}
		bindingOwners[plan.targetKey] = index

		sourceRows := rowsByBinding[plan.sourceKey]
		if len(plan.input.ExistingRouteIDs) > 0 && !samePlatformRouteIDs(sourceRows, plan.input.ExistingRouteIDs) {
			return nil, repository.ErrConflict
		}
		targetRows := rowsByBinding[plan.targetKey]
		if len(plan.input.ExistingRouteIDs) > 0 && plan.sourceKey != plan.targetKey && len(targetRows) > 0 {
			return nil, repository.ErrDuplicate
		}

		candidateRows := make(map[uint]model.LLMPlatformModelRoute, len(sourceRows)+len(targetRows))
		for _, row := range sourceRows {
			candidateRows[row.ID] = row
		}
		for _, row := range targetRows {
			candidateRows[row.ID] = row
		}
		plan.candidates = make([]model.LLMPlatformModelRoute, 0, len(candidateRows))
		for _, row := range candidateRows {
			plan.candidates = append(plan.candidates, row)
		}
		sort.Slice(plan.candidates, func(i int, j int) bool { return plan.candidates[i].ID < plan.candidates[j].ID })
	}
	return plans, nil
}

func normalizePlatformRouteReplacement(
	input repository.ReplaceChannelPlatformRoutesInput,
) (repository.ReplaceChannelPlatformRoutesInput, platformRouteBindingKey, error) {
	if input.UpstreamID == 0 || len(input.Routes) == 0 {
		return repository.ReplaceChannelPlatformRoutesInput{}, platformRouteBindingKey{}, repository.ErrInvalidInput
	}
	targetKey := platformRouteBindingKey{
		platformModelID: input.Routes[0].PlatformModelID,
		upstreamModelID: input.Routes[0].UpstreamModelID,
	}
	if targetKey.platformModelID == 0 || targetKey.upstreamModelID == 0 {
		return repository.ReplaceChannelPlatformRoutesInput{}, platformRouteBindingKey{}, repository.ErrInvalidInput
	}
	seenProtocols := make(map[string]struct{}, len(input.Routes))
	for _, route := range input.Routes {
		if route.PlatformModelID != targetKey.platformModelID || route.UpstreamModelID != targetKey.upstreamModelID || strings.TrimSpace(route.Protocol) == "" {
			return repository.ReplaceChannelPlatformRoutesInput{}, platformRouteBindingKey{}, repository.ErrInvalidInput
		}
		if _, exists := seenProtocols[route.Protocol]; exists {
			return repository.ReplaceChannelPlatformRoutesInput{}, platformRouteBindingKey{}, repository.ErrInvalidInput
		}
		seenProtocols[route.Protocol] = struct{}{}
	}
	existingRouteIDs, ok := normalizePlatformRouteIDs(input.ExistingRouteIDs)
	if !ok {
		return repository.ReplaceChannelPlatformRoutesInput{}, platformRouteBindingKey{}, repository.ErrInvalidInput
	}
	input.ExistingRouteIDs = existingRouteIDs
	return input, targetKey, nil
}

func applyPlatformRouteReplacementPlans(
	tx *gorm.DB,
	plans []platformRouteReplacementPlan,
) ([]domainchannel.PlatformModelRoute, error) {
	const batchSize = 200
	now := time.Now()
	updatedRows := make([]model.LLMPlatformModelRoute, 0)
	createdRows := make([]model.LLMPlatformModelRoute, 0)
	createdResultIndexes := make([]int, 0)
	staleIDs := make([]uint, 0)
	replaced := make([]domainchannel.PlatformModelRoute, 0)

	for _, plan := range plans {
		usedIDs := make(map[uint]struct{}, len(plan.input.Routes))
		for _, desired := range plan.input.Routes {
			candidateIndex := selectPlatformRouteCandidate(
				plan.candidates,
				usedIDs,
				plan.targetKey.platformModelID,
				plan.targetKey.upstreamModelID,
				desired.Protocol,
			)
			if candidateIndex >= 0 {
				candidate := plan.candidates[candidateIndex]
				entity := toPlatformModelRouteModel(&desired)
				entity.ControlPlaneModel = candidate.ControlPlaneModel
				entity.UpdatedAt = now
				updatedRows = append(updatedRows, entity)
				desired.ID = candidate.ID
				desired.CreatedAt = candidate.CreatedAt
				desired.UpdatedAt = now
				usedIDs[candidate.ID] = struct{}{}
			} else {
				createdRows = append(createdRows, toPlatformModelRouteModel(&desired))
				createdResultIndexes = append(createdResultIndexes, len(replaced))
			}
			replaced = append(replaced, desired)
		}
		for _, candidate := range plan.candidates {
			if _, used := usedIDs[candidate.ID]; !used {
				staleIDs = append(staleIDs, candidate.ID)
			}
		}
	}

	if len(updatedRows) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns(platformRouteMutableColumns),
		}).CreateInBatches(&updatedRows, batchSize).Error; err != nil {
			return nil, err
		}
	}
	if len(createdRows) > 0 {
		if err := tx.CreateInBatches(&createdRows, batchSize).Error; err != nil {
			return nil, err
		}
		for index, row := range createdRows {
			replaced[createdResultIndexes[index]] = toPlatformModelRouteDomain(row)
		}
	}
	if len(staleIDs) > 0 {
		if err := tx.Unscoped().Where("id IN ?", staleIDs).Delete(&model.LLMPlatformModelRoute{}).Error; err != nil {
			return nil, err
		}
	}
	return replaced, nil
}

var platformRouteMutableColumns = []string{
	"platform_model_id",
	"upstream_model_id",
	"protocol",
	"status",
	"priority",
	"weight",
	"source",
	"cb_failure_threshold",
	"cb_duration_min",
	"cb_window_min",
	"headers_json",
	"updated_at",
}

func uintSetValues(values map[uint]struct{}) []uint {
	result := make([]uint, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i int, j int) bool { return result[i] < result[j] })
	return result
}

func samePlatformRouteIDs(rows []model.LLMPlatformModelRoute, expected []uint) bool {
	if len(rows) != len(expected) {
		return false
	}
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		seen[row.ID] = struct{}{}
	}
	for _, routeID := range expected {
		if _, exists := seen[routeID]; !exists {
			return false
		}
	}
	return true
}

func normalizePlatformRouteIDs(routeIDs []uint) ([]uint, bool) {
	seen := make(map[uint]struct{}, len(routeIDs))
	result := make([]uint, 0, len(routeIDs))
	for _, routeID := range routeIDs {
		if routeID == 0 {
			return nil, false
		}
		if _, exists := seen[routeID]; exists {
			continue
		}
		seen[routeID] = struct{}{}
		result = append(result, routeID)
	}
	return result, true
}

func selectPlatformRouteCandidate(
	candidates []model.LLMPlatformModelRoute,
	usedIDs map[uint]struct{},
	targetPlatformModelID uint,
	targetUpstreamModelID uint,
	protocol string,
) int {
	for index, candidate := range candidates {
		if _, used := usedIDs[candidate.ID]; used {
			continue
		}
		if candidate.PlatformModelID == targetPlatformModelID && candidate.UpstreamModelID == targetUpstreamModelID && candidate.Protocol == protocol {
			return index
		}
	}
	for index, candidate := range candidates {
		if _, used := usedIDs[candidate.ID]; used {
			continue
		}
		if candidate.Protocol == protocol {
			return index
		}
	}
	for index, candidate := range candidates {
		if _, used := usedIDs[candidate.ID]; !used {
			return index
		}
	}
	return -1
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
	if err := r.modelUpstreamSourcesBaseQuery(ctx, name).Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := r.modelUpstreamSourcesQuery(ctx, name).
		Order("r.priority ASC, r.id DESC").
		Offset(offset).
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return items, total, nil
}

// ListModelUpstreamSourcesForUpdate 锁定并返回平台模型的完整来源集合，用于原子批量更新。
func (r *Repo) ListModelUpstreamSourcesForUpdate(ctx context.Context, platformModelName string) ([]ModelSourceRow, error) {
	items := make([]ModelSourceRow, 0)
	if err := r.modelUpstreamSourcesQuery(ctx, strings.TrimSpace(platformModelName)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Order("r.id ASC").
		Scan(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return items, nil
}

func (r *Repo) modelUpstreamSourcesBaseQuery(ctx context.Context, platformModelName string) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("pm.name = ?", platformModelName)
}

func (r *Repo) modelUpstreamSourcesQuery(ctx context.Context, platformModelName string) *gorm.DB {
	return r.modelUpstreamSourcesBaseQuery(ctx, platformModelName).
		Select(
			"r.*, um.upstream_id, u.name AS upstream_name, u.status AS upstream_status, " +
				"u.compatible AS upstream_compatible, u.protocol_defaults_json AS upstream_protocol_defaults_json, u.base_url AS base_url, " +
				"um.binding_code, um.upstream_model_name, um.vendor AS upstream_model_vendor, um.icon AS upstream_model_icon, " +
				"um.kinds_json AS upstream_model_kinds_json, um.suggested_protocol, um.status AS upstream_model_status",
		).
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_upstreams u ON u.id = um.upstream_id")
}

// GetModelUpstreamSourceByRouteID 按平台模型名和路由 ID 精确查询模型来源。
func (r *Repo) GetModelUpstreamSourceByRouteID(ctx context.Context, platformModelName string, routeID uint) (*ModelSourceRow, error) {
	var item ModelSourceRow
	if err := r.modelUpstreamSourcesQuery(ctx, strings.TrimSpace(platformModelName)).
		Where("r.id = ?", routeID).
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
	RouteID                         uint
	UpstreamModelID                 uint
	UpstreamID                      uint
	UpstreamName                    string
	PlatformModelID                 uint
	PlatformModelName               string
	ModelVendor                     string
	ModelIcon                       string
	ModelKindsJSON                  string
	ModelCapabilitiesJSON           string
	ModelSystemPrompt               string
	Protocol                        string
	BaseURL                         string
	APIKeysEnc                      string
	ConnectTimeoutMS                int
	ReadTimeoutMS                   int
	StreamIdleTimeoutMS             int
	HeadersJSON                     string
	RouteHeadersJSON                string
	BindingCode                     string
	UpstreamModelName               string
	Weight                          int
	RoutePriority                   int
	UpstreamCbFailureThreshold      int
	UpstreamCbModelThreshold        int
	UpstreamCbThresholdLogic        string
	UpstreamCbDurationMin           int
	UpstreamCbWindowMin             int
	PlatformModelCbPolicyMode       string
	PlatformModelCbFailureThreshold int
	PlatformModelCbDurationMin      int
	PlatformModelCbWindowMin        int
	ModelCbFailureThreshold         int
	ModelCbDurationMin              int
	ModelCbWindowMin                int
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
				"pm.cb_policy_mode AS platform_model_cb_policy_mode, "+
				"pm.cb_failure_threshold AS platform_model_cb_failure_threshold, "+
				"pm.cb_duration_min AS platform_model_cb_duration_min, "+
				"pm.cb_window_min AS platform_model_cb_window_min, "+
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
			RouteID:                         s.RouteID,
			UpstreamModelID:                 s.UpstreamModelID,
			UpstreamID:                      s.UpstreamID,
			UpstreamName:                    s.UpstreamName,
			PlatformModelID:                 s.PlatformModelID,
			PlatformModelName:               s.PlatformModelName,
			ModelVendor:                     s.ModelVendor,
			ModelIcon:                       s.ModelIcon,
			ModelKindsJSON:                  s.ModelKindsJSON,
			ModelCapabilitiesJSON:           s.ModelCapabilitiesJSON,
			ModelSystemPrompt:               s.ModelSystemPrompt,
			Protocol:                        s.Protocol,
			BaseURL:                         s.BaseURL,
			APIKeysEnc:                      s.APIKeysEnc,
			ConnectTimeoutMS:                s.ConnectTimeoutMS,
			ReadTimeoutMS:                   s.ReadTimeoutMS,
			StreamIdleTimeoutMS:             s.StreamIdleTimeoutMS,
			HeadersJSON:                     s.HeadersJSON,
			RouteHeadersJSON:                s.RouteHeadersJSON,
			BindingCode:                     s.BindingCode,
			UpstreamModelName:               s.UpstreamModelName,
			Weight:                          s.Weight,
			RoutePriority:                   s.RoutePriority,
			UpstreamCbFailureThreshold:      s.UpstreamCbFailureThreshold,
			UpstreamCbModelThreshold:        s.UpstreamCbModelThreshold,
			UpstreamCbThresholdLogic:        s.UpstreamCbThresholdLogic,
			UpstreamCbDurationMin:           s.UpstreamCbDurationMin,
			UpstreamCbWindowMin:             s.UpstreamCbWindowMin,
			PlatformModelCbPolicyMode:       s.PlatformModelCbPolicyMode,
			PlatformModelCbFailureThreshold: s.PlatformModelCbFailureThreshold,
			PlatformModelCbDurationMin:      s.PlatformModelCbDurationMin,
			PlatformModelCbWindowMin:        s.PlatformModelCbWindowMin,
			ModelCbFailureThreshold:         s.ModelCbFailureThreshold,
			ModelCbDurationMin:              s.ModelCbDurationMin,
			ModelCbWindowMin:                s.ModelCbWindowMin,
		})
	}
	return rows, nil
}

// ListActiveRouteBindingCodesForUpstream 返回上游下所有启用路由的 bindingCode。
func (r *Repo) ListActiveRouteBindingCodesForUpstream(ctx context.Context, upstreamID uint) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).
		Table("llm_model_routes AS r").
		Distinct("um.binding_code").
		Joins("JOIN llm_upstream_models um ON um.id = r.upstream_model_id").
		Joins("JOIN llm_platform_models pm ON pm.id = r.platform_model_id").
		Where("um.upstream_id = ? AND r.status = ? AND um.status = ? AND pm.status = ?", upstreamID, "active", "active", "active").
		Order("um.binding_code ASC").
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
		if err := tx.Where("rule_type = ? AND value = ?",
			domainchannel.PermissionGroupModelRuleUpstream,
			strconv.FormatUint(uint64(upstreamID), 10),
		).Delete(&model.PermissionGroupModelRule{}).Error; err != nil {
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
		if err := tx.Where("platform_model_id = ?", item.ID).Delete(&model.PermissionGroupModelAccess{}).Error; err != nil {
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
		ID:                 item.ID,
		PlatformModelName:  item.Name,
		Vendor:             item.Vendor,
		DisplayGroupID:     item.DisplayGroupID,
		KindsJSON:          item.KindsJSON,
		Icon:               item.Icon,
		CapabilitiesJSON:   item.CapabilitiesJSON,
		SystemPrompt:       item.SystemPrompt,
		AccessScope:        item.AccessScope,
		Status:             item.Status,
		Description:        item.Description,
		CbPolicyMode:       item.CbPolicyMode,
		CbFailureThreshold: item.CbFailureThreshold,
		CbDurationMin:      item.CbDurationMin,
		CbWindowMin:        item.CbWindowMin,
		SortOrder:          item.SortOrder,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func toPlatformModelModel(item *domainchannel.PlatformModel) model.LLMPlatformModel {
	if item == nil {
		return model.LLMPlatformModel{}
	}
	return model.LLMPlatformModel{
		Name:               item.PlatformModelName,
		Vendor:             item.Vendor,
		DisplayGroupID:     item.DisplayGroupID,
		KindsJSON:          item.KindsJSON,
		Icon:               item.Icon,
		CapabilitiesJSON:   item.CapabilitiesJSON,
		SystemPrompt:       item.SystemPrompt,
		AccessScope:        item.AccessScope,
		Status:             item.Status,
		Description:        item.Description,
		CbPolicyMode:       item.CbPolicyMode,
		CbFailureThreshold: item.CbFailureThreshold,
		CbDurationMin:      item.CbDurationMin,
		CbWindowMin:        item.CbWindowMin,
		SortOrder:          item.SortOrder,
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
