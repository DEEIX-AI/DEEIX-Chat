package billing

import (
	"context"
	"strings"
)

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

// AuditInput 描述计费域审计写入。
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

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// RecordAudit 记录计费域审计日志。
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
