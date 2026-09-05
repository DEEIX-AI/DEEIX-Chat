package billing

import (
	"context"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
)

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// AuditInput 描述计费域审计写入。
type AuditInput = appaudit.WriteInput

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// RecordAudit 记录计费域审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, input)
}
