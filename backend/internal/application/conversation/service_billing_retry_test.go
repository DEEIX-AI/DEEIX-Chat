package conversation

import (
	"context"
	"errors"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
)

func TestRetryUsageBillingOperationRecoversTransientFailure(t *testing.T) {
	attempts := 0
	err := retryUsageBillingOperation(context.Background(), func() error {
		attempts++
		if attempts < usageBillingRetryAttempts {
			return errors.New("temporary database failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry usage billing operation: %v", err)
	}
	if attempts != usageBillingRetryAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, usageBillingRetryAttempts)
	}
}

func TestRetryUsageBillingOperationSkipsSemanticFailure(t *testing.T) {
	attempts := 0
	err := retryUsageBillingOperation(context.Background(), func() error {
		attempts++
		return appbilling.ErrModelPricingRequired
	})
	if !errors.Is(err, appbilling.ErrModelPricingRequired) {
		t.Fatalf("retry usage billing operation error = %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryUsageBillingOperationStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := retryUsageBillingOperation(ctx, func() error {
		attempts++
		return errors.New("temporary database failure")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("retry usage billing operation error = %v, want context canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
