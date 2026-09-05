package conversation

import (
	"context"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
)

// AuditInput 描述会话域一次审计写入。
type AuditInput = appaudit.WriteInput

// RecordAudit 记录会话域审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, input)
}
