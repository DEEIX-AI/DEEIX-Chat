package contentmoderation

import (
	"context"
	"testing"
)

type reviewAuditCall struct {
	requestID   string
	actorUserID uint
	action      string
	resource    string
	resourceID  string
}

type reviewAuditWriter struct {
	call reviewAuditCall
}

func (writer *reviewAuditWriter) Write(
	_ context.Context,
	requestID string,
	actorUserID uint,
	action string,
	resource string,
	resourceID string,
	_ string,
	_ string,
	_ interface{},
) {
	writer.call = reviewAuditCall{
		requestID:   requestID,
		actorUserID: actorUserID,
		action:      action,
		resource:    resource,
		resourceID:  resourceID,
	}
}

func TestRecordReviewAuditIdentifiesActorAndEvent(t *testing.T) {
	writer := &reviewAuditWriter{}
	service := NewService(nil, nil, "", nil)
	service.SetAuditWriter(writer)
	service.RecordReviewAudit(context.Background(), ReviewAuditInput{
		ActorUserID: 42,
		RequestID:   " req_1 ",
		Action:      " content_moderation.event.view ",
		EventID:     " cme_1 ",
	})
	if writer.call.actorUserID != 42 || writer.call.requestID != "req_1" ||
		writer.call.action != "content_moderation.event.view" ||
		writer.call.resource != "content_moderation_event" || writer.call.resourceID != "cme_1" {
		t.Fatalf("unexpected audit call: %#v", writer.call)
	}
}
