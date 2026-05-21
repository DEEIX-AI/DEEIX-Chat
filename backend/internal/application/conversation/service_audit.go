package conversation

import (
	"context"
	"strings"
)

// AuditInput 描述会话域一次审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	Resource   string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     interface{}
}

// RecordAudit 记录会话域审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.UserID,
		strings.TrimSpace(input.Action),
		strings.TrimSpace(input.Resource),
		strings.TrimSpace(input.ResourceID),
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		input.Detail,
	)
}
