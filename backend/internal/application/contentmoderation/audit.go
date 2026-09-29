package contentmoderation

import (
	"context"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
)

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// ReviewAuditInput 包含特权读取保留内容时的请求元数据。
type ReviewAuditInput struct {
	ActorUserID uint
	RequestID   string
	Action      string
	EventID     string
	ClientIP    string
	UserAgent   string
	Detail      any
}

// RecordReviewAudit 记录查看保留审核内容的管理员。
func (s *Service) RecordReviewAudit(ctx context.Context, input ReviewAuditInput) {
	if s == nil || s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.ActorUserID,
		Action:      input.Action,
		Resource:    "content_moderation_event",
		ResourceID:  input.EventID,
		IP:          input.ClientIP,
		UserAgent:   input.UserAgent,
		Detail:      input.Detail,
	})
}
