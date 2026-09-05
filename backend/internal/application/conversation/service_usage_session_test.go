package conversation

import (
	"context"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestBeginUsageSessionWithoutBillingServiceUsesSelfMode(t *testing.T) {
	service := &Service{}
	session, err := service.BeginUsageSession(context.TODO(), SendMessageBillingInput{UserID: 7, ClientRunID: "run-1"})
	if err != nil {
		t.Fatalf("begin usage session: %v", err)
	}
	defer session.Close()
	if session.Authorization() == nil || session.Authorization().Mode != "self" {
		t.Fatalf("authorization = %#v, want self mode", session.Authorization())
	}
}

func TestUsageSessionFinishIsIdempotent(t *testing.T) {
	service := &Service{}
	session, err := service.BeginUsageSession(context.Background(), SendMessageBillingInput{UserID: 7})
	if err != nil {
		t.Fatalf("begin usage session: %v", err)
	}
	result := &SendMessageResult{Billable: true}
	if err := session.Finish(context.Background(), result); err != nil {
		t.Fatalf("first finish: %v", err)
	}
	if err := session.Finish(context.Background(), nil); err != nil {
		t.Fatalf("second finish must be a no-op, got %v", err)
	}
	session.Close()
	session.Close()
}

func TestUsageSessionNilReceiverIsSafe(t *testing.T) {
	var session *UsageSession
	if session.Authorization() != nil {
		t.Fatal("nil session must not expose an authorization")
	}
	if err := session.Finish(context.Background(), &SendMessageResult{Billable: true}); err != nil {
		t.Fatalf("finish on nil session: %v", err)
	}
	session.Close()
}

func TestUsageSessionZeroValueIsSafe(t *testing.T) {
	var session UsageSession
	if session.Authorization() != nil {
		t.Fatal("zero-value session must not expose an authorization")
	}
	if err := session.Finish(context.TODO(), &SendMessageResult{Billable: true}); err != nil {
		t.Fatalf("finish on zero-value session: %v", err)
	}
	session.Close()
}

func TestStartUsageAuthorizationRenewalStopIsIdempotentAndWaitsForExit(t *testing.T) {
	service := &Service{}
	stop := service.startUsageAuthorizationRenewal(context.Background(), &domainbilling.UsageAuthorization{
		Mode:        "reserved",
		RefNo:       "ref-1",
		Reservation: &domainbilling.UsageBalanceReservation{UserID: 7, RefNo: "ref-1"},
	})
	done := make(chan struct{})
	go func() {
		stop()
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stopping renewal twice must return promptly")
	}

	if noop := service.startUsageAuthorizationRenewal(context.Background(), &domainbilling.UsageAuthorization{Mode: "self"}); noop == nil {
		t.Fatal("authorizations without reservation must still return a stop function")
	}
}
