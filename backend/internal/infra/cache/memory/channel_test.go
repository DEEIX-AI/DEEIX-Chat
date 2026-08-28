package memory

import (
	"context"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestRecordCircuitFailureTripsUpstreamByModelThresholdWithOR(t *testing.T) {
	cache := New()
	ctx := context.Background()
	input := repository.CircuitFailureInput{
		UpstreamID:               1,
		ModelKey:                 "model-a",
		ModelWindowSec:           60,
		ModelFailureThreshold:    1,
		ModelDurationSec:         60,
		UpstreamWindowSec:        60,
		UpstreamFailureThreshold: 0,
		UpstreamModelThreshold:   1,
		UpstreamThresholdLogic:   "or",
		UpstreamDurationSec:      60,
		ActiveModelKeys:          []string{"model-a"},
	}

	if err := cache.RecordCircuitFailure(ctx, input); err != nil {
		t.Fatalf("RecordCircuitFailure() error = %v", err)
	}
	open, _ := cache.QueryUpstreamCircuitStatus(ctx, 1)
	if !open {
		t.Fatal("expected upstream circuit to open when model threshold is met with OR")
	}
}

func TestResetAllCircuitStatesPreservesIndependentState(t *testing.T) {
	cache := NewChannelCache(New())
	ctx := t.Context()
	if err := cache.OpenUpstreamCircuit(ctx, 1); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	if err := cache.OpenModelCircuit(ctx, 1, "model"); err != nil {
		t.Fatalf("OpenModelCircuit() error = %v", err)
	}
	cache.RecordFailureMetadata(ctx, 1, "temporary failure")
	if err := cache.RecordRateLimitBackoff(ctx, repository.RateLimitBackoffParams{
		UpstreamID: 1, RouteID: 10, BackoffBaseSec: 60, BackoffMaxSec: 60, BackoffMultiplier: 2,
	}); err != nil {
		t.Fatalf("RecordRateLimitBackoff() error = %v", err)
	}

	if err := cache.ResetAllCircuitStates(ctx); err != nil {
		t.Fatalf("ResetAllCircuitStates() error = %v", err)
	}
	if open, _ := cache.QueryUpstreamCircuitStatus(ctx, 1); open {
		t.Fatal("expected upstream circuit to be cleared")
	}
	if open, _ := cache.QueryModelCircuitStatus(ctx, 1, "model"); open {
		t.Fatal("expected model circuit to be cleared")
	}
	if remaining, err := cache.GetRateLimitBackoff(ctx, 1, 10); err != nil || remaining <= 0 {
		t.Fatal("expected rate-limit state to remain independent")
	}
}

func TestRouteRateLimitBackoffIsScopedAndClearedAfterSuccess(t *testing.T) {
	cache := NewChannelCache(New())
	ctx := t.Context()
	params := repository.RateLimitBackoffParams{
		UpstreamID: 1, RouteID: 10, BackoffBaseSec: 5, BackoffMaxSec: 60, BackoffMultiplier: 2,
	}
	if err := cache.RecordRateLimitBackoff(ctx, params); err != nil {
		t.Fatalf("RecordRateLimitBackoff() error = %v", err)
	}
	if remaining, err := cache.GetRateLimitBackoff(ctx, 1, 10); err != nil || remaining <= 0 {
		t.Fatalf("limited route remaining = %s, error = %v", remaining, err)
	}
	if remaining, err := cache.GetRateLimitBackoff(ctx, 1, 11); err != nil || remaining != 0 {
		t.Fatalf("sibling route remaining = %s, error = %v", remaining, err)
	}
	if err := cache.ClearRateLimitBackoff(ctx, 1, 10); err != nil {
		t.Fatalf("ClearRateLimitBackoff() error = %v", err)
	}
	if remaining, err := cache.GetRateLimitBackoff(ctx, 1, 10); err != nil || remaining != 0 {
		t.Fatalf("cleared route remaining = %s, error = %v", remaining, err)
	}
}

func TestRateLimitBackoffHonorsBoundedRetryAfter(t *testing.T) {
	params := repository.RateLimitBackoffParams{
		BackoffBaseSec: 5, BackoffMaxSec: 60, BackoffMultiplier: 2, RetryAfterSec: 45,
	}
	if got := calculateBackoffSeconds(1, params); got != 45 {
		t.Fatalf("calculateBackoffSeconds() = %d, want 45", got)
	}
	params.RetryAfterSec = 120
	if got := calculateBackoffSeconds(1, params); got != 60 {
		t.Fatalf("bounded calculateBackoffSeconds() = %d, want 60", got)
	}
}

func TestRecordCircuitFailureRequiresBothThresholdsWithAND(t *testing.T) {
	cache := New()
	ctx := context.Background()
	input := repository.CircuitFailureInput{
		UpstreamID:               1,
		ModelKey:                 "model-a",
		ModelWindowSec:           60,
		ModelFailureThreshold:    1,
		ModelDurationSec:         60,
		UpstreamWindowSec:        60,
		UpstreamFailureThreshold: 2,
		UpstreamModelThreshold:   1,
		UpstreamThresholdLogic:   "and",
		UpstreamDurationSec:      60,
		ActiveModelKeys:          []string{"model-a"},
	}

	if err := cache.RecordCircuitFailure(ctx, input); err != nil {
		t.Fatalf("RecordCircuitFailure() error = %v", err)
	}
	open, _ := cache.QueryUpstreamCircuitStatus(ctx, 1)
	if open {
		t.Fatal("expected upstream circuit to stay closed until both AND thresholds are met")
	}

	input.ModelKey = "model-b"
	input.ModelFailureThreshold = 2
	input.ActiveModelKeys = []string{"model-a", "model-b"}
	if err := cache.RecordCircuitFailure(ctx, input); err != nil {
		t.Fatalf("RecordCircuitFailure() error = %v", err)
	}
	open, _ = cache.QueryUpstreamCircuitStatus(ctx, 1)
	if !open {
		t.Fatal("expected upstream circuit to open after both AND thresholds are met")
	}
}

func TestRecordCircuitFailureReopensHalfOpenModelOnProbeFailure(t *testing.T) {
	cache := New()
	ctx := context.Background()

	cache.mu.Lock()
	state := cache.ensureModelCircuitLocked(1, "model-a")
	state.probeUntil = time.Now().Add(circuitProbeTTL)
	cache.mu.Unlock()

	if err := cache.RecordCircuitFailure(ctx, repository.CircuitFailureInput{
		UpstreamID:            1,
		ModelKey:              "model-a",
		ModelWindowSec:        60,
		ModelFailureThreshold: 10,
		ModelDurationSec:      60,
		UpstreamWindowSec:     60,
		UpstreamDurationSec:   60,
		ActiveModelKeys:       []string{"model-a"},
	}); err != nil {
		t.Fatalf("RecordCircuitFailure() error = %v", err)
	}
	open, _ := cache.QueryModelCircuitStatus(ctx, 1, "model-a")
	if !open {
		t.Fatal("expected half-open model probe failure to reopen circuit")
	}
}

func TestReleaseRouteProbesReleasesOnlyRequestedScope(t *testing.T) {
	cache := New()
	ctx := context.Background()

	cache.mu.Lock()
	upstreamState := cache.ensureUpstreamCircuitLocked(1)
	upstreamState.probeUntil = time.Now().Add(circuitProbeTTL)
	modelState := cache.ensureModelCircuitLocked(1, "model-a")
	modelState.probeUntil = time.Now().Add(circuitProbeTTL)
	cache.mu.Unlock()

	if err := cache.ReleaseRouteProbes(ctx, 1, "model-a"); err != nil {
		t.Fatalf("ReleaseRouteProbes(model) error = %v", err)
	}
	cache.mu.Lock()
	if upstreamState.probeUntil.IsZero() {
		t.Fatal("expected model probe release to keep upstream probe")
	}
	if !modelState.probeUntil.IsZero() {
		t.Fatal("expected model probe to be released")
	}
	modelState.probeUntil = time.Now().Add(circuitProbeTTL)
	cache.mu.Unlock()

	if err := cache.ReleaseRouteProbes(ctx, 1, ""); err != nil {
		t.Fatalf("ReleaseRouteProbes(upstream) error = %v", err)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !upstreamState.probeUntil.IsZero() {
		t.Fatal("expected upstream probe to be released")
	}
	if modelState.probeUntil.IsZero() {
		t.Fatal("expected upstream probe release to keep model probe")
	}
}

func TestCheckModelCircuitStateKeepsCircuitOpenDuringDuration(t *testing.T) {
	cache := New()
	ctx := context.Background()

	if err := cache.RecordCircuitFailure(ctx, repository.CircuitFailureInput{
		UpstreamID:            1,
		ModelKey:              "model-a",
		ModelWindowSec:        60,
		ModelFailureThreshold: 1,
		ModelDurationSec:      60,
		UpstreamWindowSec:     60,
		ActiveModelKeys:       []string{"model-a"},
	}); err != nil {
		t.Fatalf("RecordCircuitFailure() error = %v", err)
	}

	state, err := cache.CheckModelCircuitState(ctx, 1, "model-a")
	if err != nil {
		t.Fatalf("CheckModelCircuitState() error = %v", err)
	}
	if state != "open" {
		t.Fatalf("expected circuit to remain open during duration, got %q", state)
	}
}

func TestCheckModelCircuitStateClosesExpiredManualCircuit(t *testing.T) {
	cache := New()
	ctx := context.Background()

	cache.mu.Lock()
	state := cache.ensureModelCircuitLocked(1, "model-a")
	state.openUntil = time.Now().Add(-time.Second)
	state.manualOpen = true
	cache.mu.Unlock()

	got, err := cache.CheckModelCircuitState(ctx, 1, "model-a")
	if err != nil {
		t.Fatalf("CheckModelCircuitState() error = %v", err)
	}
	if got != "closed" {
		t.Fatalf("expected expired manual circuit to close, got %q", got)
	}
}

func TestCheckModelCircuitStateGrantsHalfOpenAfterDuration(t *testing.T) {
	cache := New()
	ctx := context.Background()

	cache.mu.Lock()
	state := cache.ensureModelCircuitLocked(1, "model-a")
	state.openUntil = time.Now().Add(-time.Second)
	cache.mu.Unlock()

	got, err := cache.CheckModelCircuitState(ctx, 1, "model-a")
	if err != nil {
		t.Fatalf("CheckModelCircuitState() error = %v", err)
	}
	if got != "half_open_granted" {
		t.Fatalf("expected half-open probe after duration, got %q", got)
	}
	got, err = cache.CheckModelCircuitState(ctx, 1, "model-a")
	if err != nil {
		t.Fatalf("CheckModelCircuitState() error = %v", err)
	}
	if got != "half_open_denied" {
		t.Fatalf("expected second half-open probe to be denied, got %q", got)
	}
}

func TestCheckModelCircuitStateClosesAfterProbeWindow(t *testing.T) {
	cache := New()
	ctx := context.Background()

	cache.mu.Lock()
	state := cache.ensureModelCircuitLocked(1, "model-a")
	state.openUntil = time.Now().Add(-circuitProbeTTL - time.Second)
	cache.mu.Unlock()

	got, err := cache.CheckModelCircuitState(ctx, 1, "model-a")
	if err != nil {
		t.Fatalf("CheckModelCircuitState() error = %v", err)
	}
	if got != "closed" {
		t.Fatalf("expected circuit to close after probe window, got %q", got)
	}
}
