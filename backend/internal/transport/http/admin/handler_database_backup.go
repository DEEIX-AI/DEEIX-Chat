package admin

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func (h *Handler) DatabaseBackupInfo(c *gin.Context) {
	if !h.requireSuperAdmin(c) || h.databaseBackup == nil {
		return
	}
	response.Success(c, gin.H{"driver": h.databaseBackup.driver(), "maxRestoreBytes": maxDatabaseRestoreBytes})
}

func (h *Handler) DownloadDatabaseBackup(c *gin.Context) {
	if !h.requireSuperAdmin(c) || h.databaseBackup == nil {
		return
	}
	backup, err := h.databaseBackup.Backup(c.Request.Context())
	if err != nil {
		h.handleDatabaseOperationError(c, err, "database backup failed")
		return
	}
	defer os.Remove(backup.Path)
	c.Header("Cache-Control", "no-store")
	c.FileAttachment(backup.Path, backup.Name)
}

func (h *Handler) RestoreDatabaseBackup(c *gin.Context) {
	if !h.requireSuperAdmin(c) || h.databaseBackup == nil {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDatabaseRestoreBytes+1024*1024)
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "database backup file is required")
		return
	}
	if file.Size <= 0 || file.Size > maxDatabaseRestoreBytes {
		response.Error(c, http.StatusBadRequest, "database backup file size is invalid")
		return
	}
	expected := ".dump"
	if h.databaseBackup.driver() == "sqlite" {
		expected = ".sqlite3"
	}
	if strings.ToLower(filepath.Ext(file.Filename)) != expected {
		response.Error(c, http.StatusBadRequest, "database backup file type does not match current driver")
		return
	}

	upload, err := os.CreateTemp("", "deeix-chat-restore-*"+expected)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "prepare database restore failed")
		return
	}
	path := upload.Name()
	defer os.Remove(path)
	source, err := file.Open()
	var copied int64
	if err == nil {
		copied, err = copyRestoreUpload(upload, source)
		_ = source.Close()
	}
	closeErr := upload.Close()
	if err != nil || closeErr != nil || copied > maxDatabaseRestoreBytes {
		response.Error(c, http.StatusBadRequest, "read database backup failed")
		return
	}
	if err := h.databaseBackup.Restore(c.Request.Context(), path); err != nil {
		h.handleDatabaseOperationError(c, err, "database restore failed")
		return
	}
	response.Success(c, gin.H{"restored": true})
}

func (h *Handler) requireSuperAdmin(c *gin.Context) bool {
	if middleware.MustUserRole(c) != string(domainuser.RoleSuperAdmin) {
		response.Error(c, http.StatusForbidden, "superadmin permission required")
		return false
	}
	return true
}

func (h *Handler) handleDatabaseOperationError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, errDatabaseOperationBusy) {
		response.Error(c, http.StatusConflict, errDatabaseOperationBusy.Error())
		return
	}
	response.Error(c, http.StatusInternalServerError, fallback)
}
