package conversation

import (
	"context"
	"errors"
	"strconv"
	"time"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repo) CreateFileCreateGrant(ctx context.Context, grant *domainmcp.FileCreateGrant, baseURL string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := fileCreateGrantModel(grant)
		row.MaintainedAt = grant.CreatedAt
		if err := validateFileCreateAuthority(tx, &row); err != nil {
			return err
		}
		var server models.MCPServer
		if err := tx.First(&server, row.ServerID).Error; err != nil || server.BaseURL != baseURL {
			return repository.ErrFileCreateUnauthorized
		}
		return tx.Create(&row).Error
	})
}

func (r *Repo) GetFileCreateGrant(ctx context.Context, tokenHash string, serverID uint) (*domainmcp.FileCreateGrant, error) {
	var grant domainmcp.FileCreateGrant
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := loadFileCreateGrant(tx, tokenHash, serverID)
		if err != nil {
			return err
		}
		grant = fileCreateGrantDomain(row)
		return nil
	})
	return &grant, err
}

func (r *Repo) BindFileCreateGrant(ctx context.Context, tokenHash string, serverID uint, fingerprint string, storagePath string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockFileCreateGrant(tx, tokenHash, serverID)
		if err != nil {
			return err
		}
		if row.Fingerprint != "" {
			if row.Fingerprint != fingerprint {
				return repository.ErrFileCreateConflict
			}
			return nil
		}
		return tx.Model(&row).Updates(map[string]any{"fingerprint": fingerprint, "storage_path": storagePath}).Error
	})
}

func (r *Repo) WithFileCreateGrant(ctx context.Context, tokenHash string, serverID uint, fn func(*domainmcp.FileCreateGrant, repository.UploadRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockFileCreateGrant(tx, tokenHash, serverID)
		if err != nil {
			return err
		}
		grant := fileCreateGrantDomain(row)
		if err = fn(&grant, NewRepo(tx)); err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"file_id": grant.FileID, "reused": grant.Reused}).Error
	})
}

func lockFileCreateGrant(tx *gorm.DB, tokenHash string, serverID uint) (models.MCPFileCreateGrant, error) {
	// SQLite has one writer, not row locks. Acquire its write lock before reading.
	if tx.Dialector.Name() == "sqlite" {
		if err := tx.Model(&models.MCPFileCreateGrant{}).Where("token_hash = ? AND server_id = ?", tokenHash, serverID).
			Update("token_hash", tokenHash).Error; err != nil {
			return models.MCPFileCreateGrant{}, err
		}
	}
	return loadFileCreateGrant(tx, tokenHash, serverID)
}

func loadFileCreateGrant(tx *gorm.DB, tokenHash string, serverID uint) (models.MCPFileCreateGrant, error) {
	var row models.MCPFileCreateGrant
	if err := tx.First(&row, "token_hash = ? AND server_id = ?", tokenHash, serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, repository.ErrFileCreateUnauthorized
		}
		return row, err
	}
	if !row.ExpiresAt.After(time.Now()) {
		return row, repository.ErrFileCreateUnauthorized
	}
	// Lock authority before the receipt, matching disable/update transaction order.
	if err := validateFileCreateAuthority(tx, &row); err != nil {
		return row, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "token_hash = ? AND server_id = ?", tokenHash, serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, repository.ErrFileCreateUnauthorized
		}
		return row, err
	}
	if !row.ExpiresAt.After(time.Now()) {
		return row, repository.ErrFileCreateUnauthorized
	}
	return row, nil
}

func validateFileCreateAuthority(tx *gorm.DB, row *models.MCPFileCreateGrant) error {
	// Read the persisted global switch too; another instance may have a stale runtime snapshot.
	var setting models.SystemSetting
	err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("namespace = ? AND key = ?", "mcp", "mcp_enable").First(&setting).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		enabled, parseErr := strconv.ParseBool(setting.Value)
		if parseErr != nil || !enabled {
			return repository.ErrFileCreateUnauthorized
		}
	}
	var server models.MCPServer
	if err = tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&server, row.ServerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrFileCreateUnauthorized
		}
		return err
	}
	if server.Status != "active" || !server.FileCreateEnabled || server.FileCreateEpoch != row.Epoch {
		return repository.ErrFileCreateUnauthorized
	}
	var tool models.MCPTool
	if err = tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&tool, "id = ? AND server_id = ?", row.ToolID, row.ServerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrFileCreateUnauthorized
		}
		return err
	}
	var user models.User
	if err = tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&user, row.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrFileCreateUnauthorized
		}
		return err
	}
	if row.UserID == 0 || user.Status != "active" || tool.Status != "active" {
		return repository.ErrFileCreateUnauthorized
	}
	return nil
}

func (r *Repo) ListFileCreateMaintenance(ctx context.Context, now time.Time) ([]domainmcp.FileCreateGrant, error) {
	var rows []models.MCPFileCreateGrant
	// Rotate visited receipts so a stuck storage/queue cannot starve cleanup.
	err := r.db.WithContext(ctx).Where("expires_at <= ? OR file_id <> ''", now).Order("maintained_at").Limit(100).Find(&rows).Error
	if err == nil && len(rows) > 0 {
		hashes := make([]string, 0, len(rows))
		for _, row := range rows {
			hashes = append(hashes, row.TokenHash)
		}
		err = r.db.WithContext(ctx).Model(&models.MCPFileCreateGrant{}).Where("token_hash IN ?", hashes).Update("maintained_at", now).Error
	}
	result := make([]domainmcp.FileCreateGrant, 0, len(rows))
	for _, row := range rows {
		result = append(result, fileCreateGrantDomain(row))
	}
	return result, err
}

func (r *Repo) ExpireFileCreateGrant(ctx context.Context, hash string, now time.Time, remove func(string) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Model(&models.MCPFileCreateGrant{}).Where("token_hash = ?", hash).Update("token_hash", hash).Error; err != nil {
				return err
			}
		}
		var row models.MCPFileCreateGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "token_hash = ?", hash).Error; err != nil {
			return err
		}
		if row.ExpiresAt.After(now) {
			return nil
		}
		if row.StoragePath != "" {
			var references int64
			if err := tx.Model(&models.FileObject{}).Where("storage_path = ? AND status = ?", row.StoragePath, "active").Count(&references).Error; err != nil {
				return err
			}
			if references == 0 {
				if err := remove(row.StoragePath); err != nil {
					return err
				}
			}
		}
		return tx.Delete(&row).Error
	})
}

func fileCreateGrantModel(g *domainmcp.FileCreateGrant) models.MCPFileCreateGrant {
	return models.MCPFileCreateGrant{TokenHash: g.TokenHash, ServerID: g.ServerID, ToolID: g.ToolID, UserID: g.UserID, Epoch: g.Epoch, CallID: g.CallID, RequestID: g.RequestID, ExpiresAt: g.ExpiresAt, CreatedAt: g.CreatedAt, Fingerprint: g.Fingerprint, CandidateID: g.CandidateID, StoragePath: g.StoragePath, FileID: g.FileID, Reused: g.Reused}
}

func fileCreateGrantDomain(g models.MCPFileCreateGrant) domainmcp.FileCreateGrant {
	return domainmcp.FileCreateGrant{TokenHash: g.TokenHash, ServerID: g.ServerID, ToolID: g.ToolID, UserID: g.UserID, Epoch: g.Epoch, CallID: g.CallID, RequestID: g.RequestID, ExpiresAt: g.ExpiresAt, CreatedAt: g.CreatedAt, Fingerprint: g.Fingerprint, CandidateID: g.CandidateID, StoragePath: g.StoragePath, FileID: g.FileID, Reused: g.Reused}
}
