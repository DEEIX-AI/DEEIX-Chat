package contentmoderation

import (
	"context"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
)

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// ReviewAuditInput contains request metadata for a privileged retained-content read.
type ReviewAuditInput struct {
	ActorUserID uint
	RequestID   string
	Action      string
	EventID     string
	ClientIP    string
	UserAgent   string
	Detail      any
}

// RecordReviewAudit records which administrator viewed retained moderation content.
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
