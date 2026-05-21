package mcp

import (
	"context"
	"time"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateServer(ctx context.Context, input repository.CreateMCPServerInput) (*domainmcp.Server, error) {
	item := model.MCPServer{
		Name:         input.Name,
		BaseURL:      input.BaseURL,
		AuthTokenEnc: input.AuthTokenEnc,
		HeadersJSON:  input.HeadersJSON,
		Status:       input.Status,
	}
	if err := r.db.WithContext(ctx).Create(&item).Error; err != nil {
		return nil, err
	}
	result := toDomainServer(item)
	return &result, nil
}

func (r *Repo) UpdateServer(ctx context.Context, serverID uint, input repository.UpdateMCPServerInput) (*domainmcp.Server, error) {
	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.BaseURL != nil {
		updates["base_url"] = *input.BaseURL
	}
	if input.AuthTokenEnc != nil {
		updates["auth_token_enc"] = *input.AuthTokenEnc
	}
	if input.HeadersJSON != nil {
		updates["headers_json"] = *input.HeadersJSON
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.LastError != nil {
		updates["last_error"] = *input.LastError
	}
	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&model.MCPServer{}).Where("id = ?", serverID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return r.GetServer(ctx, serverID)
}

func (r *Repo) ListServers(ctx context.Context) ([]domainmcp.Server, error) {
	var rows []model.MCPServer
	if err := r.db.WithContext(ctx).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	activeCounts := map[uint]int{}
	if len(rows) > 0 {
		serverIDs := make([]uint, 0, len(rows))
		for _, row := range rows {
			serverIDs = append(serverIDs, row.ID)
		}
		var counts []struct {
			ServerID uint
			Count    int
		}
		if err := r.db.WithContext(ctx).
			Model(&model.MCPTool{}).
			Select("server_id, count(*) as count").
			Where("server_id IN ? AND status = ?", serverIDs, "active").
			Group("server_id").
			Scan(&counts).Error; err != nil {
			return nil, err
		}
		for _, item := range counts {
			activeCounts[item.ServerID] = item.Count
		}
	}
	items := make([]domainmcp.Server, 0, len(rows))
	for _, row := range rows {
		item := toDomainServer(row)
		item.ActiveToolCount = activeCounts[row.ID]
		items = append(items, item)
	}
	return items, nil
}

func (r *Repo) GetServer(ctx context.Context, serverID uint) (*domainmcp.Server, error) {
	var row model.MCPServer
	if err := r.db.WithContext(ctx).First(&row, "id = ?", serverID).Error; err != nil {
		return nil, err
	}
	item := toDomainServer(row)
	return &item, nil
}

func (r *Repo) DeleteServer(ctx context.Context, serverID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("server_id = ?", serverID).Delete(&model.MCPTool{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.MCPServer{}, "id = ?", serverID).Error
	})
}

func (r *Repo) ReplaceServerTools(ctx context.Context, serverID uint, tools []domainmcp.Tool) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows := make([]model.MCPTool, 0, len(tools))
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			names = append(names, tool.Name)
			rows = append(rows, model.MCPTool{
				ServerID:        serverID,
				Name:            tool.Name,
				DisplayName:     tool.DisplayName,
				Description:     tool.Description,
				InputSchemaJSON: tool.InputSchemaJSON,
				Status:          tool.Status,
			})
		}
		if len(rows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "server_id"}, {Name: "name"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"input_schema_json",
					"updated_at",
				}),
			}).Create(&rows).Error; err != nil {
				return err
			}
		}
		deleteQuery := tx.Where("server_id = ?", serverID)
		if len(names) > 0 {
			deleteQuery = deleteQuery.Where("name NOT IN ?", names)
		}
		if err := deleteQuery.Delete(&model.MCPTool{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.MCPServer{}).Where("id = ?", serverID).Updates(map[string]interface{}{
			"tool_count":     len(tools),
			"last_synced_at": &now,
			"last_error":     "",
		}).Error
	})
}

func (r *Repo) ListTools(ctx context.Context, serverID uint, onlyActive bool) ([]domainmcp.Tool, error) {
	query := r.db.WithContext(ctx).Where("server_id = ?", serverID).Order("name asc")
	if onlyActive {
		query = query.Where("status = ?", "active")
	}
	var rows []model.MCPTool
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domainmcp.Tool, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainTool(row))
	}
	return items, nil
}

func (r *Repo) ListToolsByIDs(ctx context.Context, toolIDs []uint) ([]domainmcp.Tool, error) {
	if len(toolIDs) == 0 {
		return []domainmcp.Tool{}, nil
	}
	var rows []model.MCPTool
	if err := r.db.WithContext(ctx).Where("id IN ?", toolIDs).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domainmcp.Tool, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainTool(row))
	}
	return items, nil
}

func (r *Repo) UpdateTool(ctx context.Context, toolID uint, input repository.UpdateMCPToolInput) (*domainmcp.Tool, error) {
	updates := map[string]interface{}{}
	if input.DisplayName != nil {
		updates["display_name"] = *input.DisplayName
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&model.MCPTool{}).Where("id = ?", toolID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var row model.MCPTool
	if err := r.db.WithContext(ctx).First(&row, "id = ?", toolID).Error; err != nil {
		return nil, err
	}
	item := toDomainTool(row)
	return &item, nil
}

func (r *Repo) UpdateServerToolsStatus(ctx context.Context, serverID uint, toolIDs []uint, status string) ([]domainmcp.Tool, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.MCPTool{}).
		Where("server_id = ? AND id IN ?", serverID, toolIDs).
		Update("status", status).Error; err != nil {
		return nil, err
	}
	return r.ListTools(ctx, serverID, false)
}

func toDomainServer(row model.MCPServer) domainmcp.Server {
	return domainmcp.Server{
		ID:              row.ID,
		Name:            row.Name,
		BaseURL:         row.BaseURL,
		AuthTokenEnc:    row.AuthTokenEnc,
		HeadersJSON:     row.HeadersJSON,
		Status:          row.Status,
		ToolCount:       row.ToolCount,
		ActiveToolCount: 0,
		LastSyncedAt:    row.LastSyncedAt,
		LastError:       row.LastError,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func toDomainTool(row model.MCPTool) domainmcp.Tool {
	return domainmcp.Tool{
		ID:              row.ID,
		ServerID:        row.ServerID,
		Name:            row.Name,
		DisplayName:     row.DisplayName,
		Description:     row.Description,
		InputSchemaJSON: row.InputSchemaJSON,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
