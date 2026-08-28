package memory

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	circuitManualOpenDuration = 24 * time.Hour
	circuitProbeTTL           = 30 * time.Second
)

type circuitState struct {
	openUntil   time.Time
	probeUntil  time.Time
	failures    []time.Time
	manualOpen  bool
	lastFailure time.Time
}

type upstreamMetadata struct {
	lastError   string
	lastFailure time.Time
	lastSuccess time.Time
}

type rateLimitState struct {
	backoffUntil time.Time
	backoffCount int64
	countExpires time.Time
}

// apiKeyCounter 记录 API Key 轮询计数与最近使用时间，供周期清扫回收闲置条目。
type apiKeyCounter struct {
	next     int64
	lastUsed time.Time
}

type routeRateLimitKey struct {
	upstreamID uint
	routeID    uint
}

func (c *Cache) CheckUpstreamCircuitState(ctx context.Context, upstreamID uint) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checkCircuitStateLocked(c.upstreamCB[upstreamID]), nil
}

func (c *Cache) CheckModelCircuitState(ctx context.Context, upstreamID uint, modelKey string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checkCircuitStateLocked(c.modelCB[modelCircuitKey(upstreamID, modelKey)]), nil
}

func (c *Cache) checkCircuitStateLocked(state *circuitState) string {
	if state == nil {
		return "closed"
	}
	now := time.Now()
	if state.manualOpen {
		if now.Before(state.openUntil) {
			return "open"
		}
		state.openUntil = time.Time{}
		state.probeUntil = time.Time{}
		state.manualOpen = false
		return "closed"
	}
	if now.Before(state.openUntil) {
		return "open"
	}
	if !state.openUntil.IsZero() {
		if now.After(state.openUntil.Add(circuitProbeTTL)) {
			state.openUntil = time.Time{}
			state.probeUntil = time.Time{}
			state.manualOpen = false
			return "closed"
		}
		if now.Before(state.probeUntil) {
			return "half_open_denied"
		}
		state.probeUntil = now.Add(circuitProbeTTL)
		return "half_open_granted"
	}
	return "closed"
}

func (c *Cache) RecordCircuitFailure(ctx context.Context, input repository.CircuitFailureInput) error {
	if input.UpstreamID == 0 || strings.TrimSpace(input.ModelKey) == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	modelState := c.ensureModelCircuitLocked(input.UpstreamID, input.ModelKey)
	modelProbeFailed := now.Before(modelState.probeUntil)
	modelState.probeUntil = time.Time{}
	recordFailure(modelState, now, time.Duration(input.ModelWindowSec)*time.Second)
	if (modelProbeFailed || shouldTrip(modelState, input.ModelFailureThreshold)) && input.ModelDurationSec > 0 {
		modelState.openUntil = now.Add(time.Duration(input.ModelDurationSec) * time.Second)
		modelState.manualOpen = false
	}
	upstreamState := c.ensureUpstreamCircuitLocked(input.UpstreamID)
	upstreamProbeFailed := now.Before(upstreamState.probeUntil)
	upstreamState.probeUntil = time.Time{}
	recordFailure(upstreamState, now, time.Duration(input.UpstreamWindowSec)*time.Second)

	upstreamFailMet := upstreamProbeFailed || shouldTrip(upstreamState, input.UpstreamFailureThreshold)
	upstreamModelMet := input.UpstreamModelThreshold > 0 &&
		c.openModelCountLocked(input.UpstreamID, input.ActiveModelKeys, now) >= input.UpstreamModelThreshold
	upstreamShouldTrip := shouldTripUpstream(upstreamFailMet, input.UpstreamFailureThreshold, upstreamModelMet, input.UpstreamModelThreshold, input.UpstreamThresholdLogic)
	if upstreamShouldTrip && input.UpstreamDurationSec > 0 {
		upstreamState.openUntil = now.Add(time.Duration(input.UpstreamDurationSec) * time.Second)
		upstreamState.manualOpen = false
	}
	c.maybeSweepLocked(now)
	return nil
}

func (c *Cache) RecordFailureMetadata(ctx context.Context, upstreamID uint, lastError string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.upstreamMeta[upstreamID] = upstreamMetadata{lastError: lastError, lastFailure: now}
	c.maybeSweepLocked(now)
}

func (c *Cache) RecordSuccessMetadata(ctx context.Context, upstreamID uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	meta := c.upstreamMeta[upstreamID]
	now := time.Now()
	meta.lastSuccess = now
	c.upstreamMeta[upstreamID] = meta
	c.maybeSweepLocked(now)
}

func (c *Cache) ClearUpstreamCircuitKeys(ctx context.Context, upstreamID uint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.upstreamCB, upstreamID)
	return nil
}

func (c *Cache) ClearModelCircuitKeys(ctx context.Context, upstreamID uint, modelKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.modelCB, modelCircuitKey(upstreamID, modelKey))
	return nil
}

// ResetAllCircuitStates 清除全部上游与模型熔断状态和失败计数。
func (c *Cache) ResetAllCircuitStates(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.upstreamCB = make(map[uint]*circuitState)
	c.modelCB = make(map[string]*circuitState)
	return nil
}

func (c *Cache) ReleaseRouteProbes(ctx context.Context, upstreamID uint, modelKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(modelKey) == "" {
		if state := c.upstreamCB[upstreamID]; state != nil {
			state.probeUntil = time.Time{}
		}
		return nil
	}
	if state := c.modelCB[modelCircuitKey(upstreamID, modelKey)]; state != nil {
		state.probeUntil = time.Time{}
	}
	return nil
}

func (c *Cache) OpenUpstreamCircuit(ctx context.Context, upstreamID uint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.ensureUpstreamCircuitLocked(upstreamID)
	now := time.Now()
	state.openUntil = now.Add(circuitManualOpenDuration)
	state.manualOpen = true
	c.maybeSweepLocked(now)
	return nil
}

func (c *Cache) ResetUpstreamCircuit(ctx context.Context, upstreamID uint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.upstreamCB, upstreamID)
	delete(c.upstreamMeta, upstreamID)
	return nil
}

func (c *Cache) OpenModelCircuit(ctx context.Context, upstreamID uint, modelKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.ensureModelCircuitLocked(upstreamID, modelKey)
	now := time.Now()
	state.openUntil = now.Add(circuitManualOpenDuration)
	state.manualOpen = true
	c.maybeSweepLocked(now)
	return nil
}

func (c *Cache) ResetModelCircuit(ctx context.Context, upstreamID uint, modelKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.modelCB, modelCircuitKey(upstreamID, modelKey))
	return nil
}

func (c *Cache) QueryUpstreamCircuitStatus(ctx context.Context, upstreamID uint) (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return queryCircuitStatus(c.upstreamCB[upstreamID])
}

func (c *Cache) QueryModelCircuitStatus(ctx context.Context, upstreamID uint, modelKey string) (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return queryCircuitStatus(c.modelCB[modelCircuitKey(upstreamID, modelKey)])
}

func (c *Cache) GetRateLimitBackoff(ctx context.Context, upstreamID uint, routeID uint) (time.Duration, error) {
	if upstreamID == 0 || routeID == 0 {
		return 0, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := time.Until(c.rateLimits[routeRateLimitKey{upstreamID: upstreamID, routeID: routeID}].backoffUntil)
	if remaining <= 0 {
		return 0, nil
	}
	return remaining, nil
}

func (c *Cache) RecordRateLimitBackoff(ctx context.Context, params repository.RateLimitBackoffParams) error {
	if params.UpstreamID == 0 || params.RouteID == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := routeRateLimitKey{upstreamID: params.UpstreamID, routeID: params.RouteID}
	state := c.rateLimits[key]
	now := time.Now()
	if now.After(state.countExpires) {
		state.backoffCount = 0
	}
	state.backoffCount++
	state.countExpires = now.Add(5 * time.Minute)
	state.backoffUntil = now.Add(time.Duration(calculateBackoffSeconds(state.backoffCount, params)) * time.Second)
	c.rateLimits[key] = state
	c.maybeSweepLocked(now)
	return nil
}

func (c *Cache) ClearRateLimitBackoff(ctx context.Context, upstreamID uint, routeID uint) error {
	if upstreamID == 0 || routeID == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rateLimits, routeRateLimitKey{upstreamID: upstreamID, routeID: routeID})
	return nil
}

func (c *Cache) IncrAPIKeyCounter(ctx context.Context, upstreamID uint) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	counter := c.keyCounters[upstreamID]
	next := counter.next
	c.keyCounters[upstreamID] = apiKeyCounter{next: next + 1, lastUsed: now}
	c.maybeSweepLocked(now)
	return next, true
}

func (c *Cache) ensureUpstreamCircuitLocked(upstreamID uint) *circuitState {
	state := c.upstreamCB[upstreamID]
	if state == nil {
		state = &circuitState{}
		c.upstreamCB[upstreamID] = state
	}
	return state
}

func (c *Cache) ensureModelCircuitLocked(upstreamID uint, modelKey string) *circuitState {
	key := modelCircuitKey(upstreamID, modelKey)
	state := c.modelCB[key]
	if state == nil {
		state = &circuitState{}
		c.modelCB[key] = state
	}
	return state
}

func modelCircuitKey(upstreamID uint, modelKey string) string {
	return fmt.Sprintf("%d:%s", upstreamID, strings.TrimSpace(modelKey))
}

func recordFailure(state *circuitState, now time.Time, window time.Duration) {
	if window <= 0 {
		window = 5 * time.Minute
	}
	cutoff := now.Add(-window)
	failures := state.failures[:0]
	for _, item := range state.failures {
		if item.After(cutoff) {
			failures = append(failures, item)
		}
	}
	state.failures = append(failures, now)
	state.lastFailure = now
}

func shouldTrip(state *circuitState, threshold int) bool {
	return threshold > 0 && len(state.failures) >= threshold
}

func (c *Cache) openModelCountLocked(upstreamID uint, modelKeys []string, now time.Time) int {
	count := 0
	seen := make(map[string]struct{}, len(modelKeys))
	for _, modelKey := range modelKeys {
		key := modelCircuitKey(upstreamID, modelKey)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		state := c.modelCB[key]
		if state != nil && now.Before(state.openUntil) {
			count++
		}
	}
	return count
}

func shouldTripUpstream(failMet bool, failThreshold int, modelMet bool, modelThreshold int, logic string) bool {
	if strings.TrimSpace(logic) == "and" {
		switch {
		case failThreshold > 0 && modelThreshold > 0:
			return failMet && modelMet
		case failThreshold > 0:
			return failMet
		case modelThreshold > 0:
			return modelMet
		default:
			return false
		}
	}
	return failMet || modelMet
}

func queryCircuitStatus(state *circuitState) (bool, string) {
	if state == nil || !time.Now().Before(state.openUntil) {
		return false, ""
	}
	return true, strconv.FormatInt(state.openUntil.Unix(), 10)
}

func calculateBackoffSeconds(attempt int64, params repository.RateLimitBackoffParams) int {
	base := params.BackoffBaseSec
	if base <= 0 {
		base = 5
	}
	maxSec := params.BackoffMaxSec
	if maxSec <= 0 {
		maxSec = 60
	}
	multiplier := params.BackoffMultiplier
	if multiplier <= 1 {
		multiplier = 2
	}
	backoff := base
	for i := int64(1); i < attempt; i++ {
		if backoff >= maxSec {
			backoff = maxSec
			break
		}
		backoff *= multiplier
		if backoff >= maxSec {
			backoff = maxSec
			break
		}
	}
	if params.RetryAfterSec > backoff {
		backoff = params.RetryAfterSec
	}
	if backoff > maxSec {
		return maxSec
	}
	return backoff
}
