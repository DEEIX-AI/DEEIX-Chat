package contentmoderation

import (
	"context"
	"strings"
)

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

// ReviewAuditInput contains request metadata for a privileged retained-content read.
type ReviewAuditInput struct {
	ActorUserID uint
	RequestID   string
	Action      string
	EventID     string
	ClientIP    string
	UserAgent   string
	Detail      interface{}
}

// RecordReviewAudit records which administrator viewed retained moderation content.
func (s *Service) RecordReviewAudit(ctx context.Context, input ReviewAuditInput) {
	if s == nil || s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.ActorUserID,
		strings.TrimSpace(input.Action),
		"content_moderation_event",
		strings.TrimSpace(input.EventID),
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		input.Detail,
	)
}
