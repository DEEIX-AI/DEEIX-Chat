package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const FileCreateTTL = 10 * time.Minute

func fileCreateError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFileCreateUnauthorized):
		return ErrFileCreateUnauthorized
	case errors.Is(err, repository.ErrFileCreateConflict):
		return ErrFileCreateConflict
	case errors.Is(err, repository.ErrFileNotFound), errors.Is(err, repository.ErrNotFound):
		return ErrFileCreateResultGone
	case errors.Is(err, repository.ErrInvalidInput):
		return ErrFileCreateInvalidInput
	default:
		return err
	}
}

type fileCreateAuditWriter interface {
	Write(context.Context, appaudit.WriteInput)
}

type FileCreateService struct {
	cfg     *config.Runtime
	servers repository.MCPRepository
	repo    repository.MCPFileCreateRepository
	uploads *appupload.Service
	audit   fileCreateAuditWriter
	logger  *zap.Logger
}

func NewFileCreateService(cfg *config.Runtime, servers repository.MCPRepository, repo repository.MCPFileCreateRepository, uploads *appupload.Service, audit fileCreateAuditWriter, logger *zap.Logger) *FileCreateService {
	return &FileCreateService{cfg: cfg, servers: servers, repo: repo, uploads: uploads, audit: audit, logger: logger}
}

type FileCreateCall struct {
	ServerID  uint
	ToolID    uint
	UserID    uint
	BaseURL   string
	CallID    string
	RequestID string
	Epoch     uint
}

// IssueFileCreateGrant is internal-only. No external identity/signature can mint it.
func (s *FileCreateService) IssueFileCreateGrant(ctx context.Context, call FileCreateCall) (string, error) {
	if !s.cfg.Snapshot().MCPEnable {
		return "", nil
	}
	server, err := s.servers.GetServer(ctx, call.ServerID)
	if err != nil {
		return "", err
	}
	if !server.FileCreateEnabled {
		return "", nil
	}
	if server.Status != "active" || call.UserID == 0 || call.ToolID == 0 || call.CallID == "" || server.BaseURL != call.BaseURL || server.FileCreateEpoch != call.Epoch {
		return "", repository.ErrFileCreateUnauthorized
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	token := "dxf1_" + base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now()
	grant := &domainmcp.FileCreateGrant{
		TokenHash: fileCreateTokenHash(token), ServerID: call.ServerID, ToolID: call.ToolID,
		UserID: call.UserID, Epoch: server.FileCreateEpoch, CallID: call.CallID, RequestID: call.RequestID,
		CreatedAt: now, ExpiresAt: now.Add(FileCreateTTL), CandidateID: "file_" + conv.NormalizePublicID(uuid.NewString()),
	}
	if err = s.repo.CreateFileCreateGrant(ctx, grant, call.BaseURL); err != nil {
		return "", err
	}
	return token, nil
}

func fileCreateTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *FileCreateService) Authorize(ctx context.Context, token string, serverID uint) (*domainmcp.FileCreateGrant, error) {
	if !s.cfg.Snapshot().MCPEnable || len(token) != 48 || token[:5] != "dxf1_" {
		return nil, ErrFileCreateUnauthorized
	}
	grant, err := s.repo.GetFileCreateGrant(ctx, fileCreateTokenHash(token), serverID)
	return grant, fileCreateError(err)
}

type FileCreateResult struct {
	File     domainconversation.FileObject
	Reused   bool
	Replayed bool
}

func (s *FileCreateService) Create(ctx context.Context, token string, serverID uint, input appupload.TemporaryFileInput) (result *FileCreateResult, err error) {
	defer func() { err = fileCreateError(err) }()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	grant, err := s.Authorize(ctx, token, serverID)
	if err != nil {
		return nil, err
	}
	prepared, err := s.uploads.PrepareTemporaryFile(ctx, input)
	if err != nil {
		return nil, err
	}
	defer prepared.Cleanup()
	if len(prepared.FileName) > 255 || len(prepared.MimeType) > 128 {
		return nil, repository.ErrInvalidInput
	}
	manifest, err := json.Marshal([]any{prepared.FileName, prepared.MimeType, prepared.SHA256, prepared.SizeBytes})
	if err != nil {
		return nil, err
	}
	fingerprint := sha256.Sum256(manifest)
	key, err := s.uploads.FileCreateStoragePath(ctx, grant, prepared)
	if err != nil {
		return nil, err
	}
	if err = s.repo.BindFileCreateGrant(ctx, grant.TokenHash, serverID, hex.EncodeToString(fingerprint[:]), key); err != nil {
		return nil, err
	}
	result = &FileCreateResult{}
	err = s.repo.WithFileCreateGrant(ctx, grant.TokenHash, serverID, func(locked *domainmcp.FileCreateGrant, uploads repository.UploadRepository) error {
		if locked.FileID != "" {
			file, readErr := uploads.GetActiveFileObjectByID(ctx, locked.UserID, locked.FileID)
			if readErr != nil {
				return readErr
			}
			result.File, result.Reused, result.Replayed = *file, locked.Reused, true
			return nil
		}
		// ponytail: hold a user quota lock during bounded object I/O; use short
		// leases only if measured per-user upload contention warrants the complexity.
		if _, lockErr := uploads.GetOrInitUserStorageQuota(ctx, locked.UserID, s.cfg.Snapshot().UserStorageQuotaBytes); lockErr != nil {
			return lockErr
		}
		created, createErr := s.uploads.CommitFileCreate(ctx, uploads, locked, prepared)
		if createErr != nil {
			return createErr
		}
		locked.FileID, locked.Reused = created.File.FileID, created.Reused
		result.File, result.Reused = created.File, created.Reused
		return nil
	})
	if err != nil {
		return nil, err
	}
	if file, finishErr := s.uploads.FinishFileCreateProcessing(ctx, grant.UserID, result.File.FileID); finishErr == nil {
		result.File = *file
	}
	if s.audit != nil {
		auditCtx, cancelAudit := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelAudit()
		s.audit.Write(auditCtx, appaudit.WriteInput{
			RequestID: grant.RequestID, ActorUserID: grant.UserID, Action: "mcp_create_file", Resource: "file", ResourceID: result.File.FileID,
			Detail: map[string]any{"mcp_server_id": grant.ServerID, "mcp_tool_id": grant.ToolID, "call_id": grant.CallID, "target_user_id": grant.UserID, "size_bytes": result.File.SizeBytes, "reused": result.Reused, "replayed": result.Replayed},
		})
	}
	return result, nil
}

// Maintain resumes receipt-to-processing handoff and removes expired abandoned
// candidates. It uses existing processing workers, not an additional task queue.
func (s *FileCreateService) Maintain(ctx context.Context) {
	now := time.Now()
	grants, err := s.repo.ListFileCreateMaintenance(ctx, now)
	if err != nil {
		s.logger.Warn("mcp_file_create_maintenance_failed", zap.Error(err))
		return
	}
	for _, grant := range grants {
		if grant.FileID != "" {
			file, finishErr := s.uploads.FinishFileCreateProcessing(ctx, grant.UserID, grant.FileID)
			if finishErr != nil && !errors.Is(finishErr, repository.ErrNotFound) && !errors.Is(finishErr, repository.ErrFileNotFound) {
				continue // Keep durable handoff evidence on transient lookup failures.
			}
			if finishErr == nil && (file.ProcessingStatus == "uploaded" || file.ProcessingStatus == "queued") {
				continue
			}
		}
		if !grant.ExpiresAt.After(now) {
			err = s.repo.ExpireFileCreateGrant(ctx, grant.TokenHash, now, func(key string) error { return s.uploads.RemoveFileCreateCandidate(ctx, key) })
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				s.logger.Warn("mcp_file_create_cleanup_failed", zap.Error(err))
			}
		}
	}
}

func (s *FileCreateService) MaxUploadBytes() int64 {
	if n := s.cfg.Snapshot().MaxUploadFileBytes; n > 0 {
		return n
	}
	return 20 * 1024 * 1024
}

func (s *FileCreateService) StartMaintenance(ctx context.Context) {
	go func() {
		startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		s.Maintain(startupCtx)
		cancel()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				maintenanceCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				s.Maintain(maintenanceCtx)
				cancel()
			}
		}
	}()
}
