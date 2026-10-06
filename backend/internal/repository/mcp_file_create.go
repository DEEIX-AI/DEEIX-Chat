package repository

import (
	"context"
	"errors"
	"time"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
)

var (
	ErrFileCreateUnauthorized = errors.New("file create capability is invalid, expired or revoked")
	ErrFileCreateConflict     = errors.New("file create capability is already bound to another file")
)

// MCPFileCreateRepository is the one-file receipt boundary, not a general permission store.
type MCPFileCreateRepository interface {
	CreateFileCreateGrant(context.Context, *domainmcp.FileCreateGrant, string) error
	GetFileCreateGrant(context.Context, string, uint) (*domainmcp.FileCreateGrant, error)
	BindFileCreateGrant(context.Context, string, uint, string, string) error
	WithFileCreateGrant(context.Context, string, uint, func(*domainmcp.FileCreateGrant, UploadRepository) error) error
	ListFileCreateMaintenance(context.Context, time.Time) ([]domainmcp.FileCreateGrant, error)
	ExpireFileCreateGrant(context.Context, string, time.Time, func(string) error) error
}
