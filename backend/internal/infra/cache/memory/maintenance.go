package memory

import "time"

const (
	maintenanceInterval      = 256
	slidingWindowRetention   = 10 * time.Minute
	circuitMetadataRetention = 30 * time.Minute
	// circuitStateRetention 熔断状态在最近一次失败后保留的时长；
	// 取大于常见失败统计窗口的保守值，避免清扫导致失败计数提前归零。
	circuitStateRetention = 2 * time.Hour
	// keyCounterRetention API Key 轮询计数闲置多久后回收；回收仅使轮询从头开始，无正确性影响。
	keyCounterRetention = 24 * time.Hour
)

func (c *Cache) maybeSweepLocked(now time.Time) {
	c.ops++
	if c.ops%maintenanceInterval != 0 {
		return
	}
	c.sweepExpiredLocked(now)
}

func (c *Cache) sweepExpiredLocked(now time.Time) {
	for key, item := range c.settings {
		if now.After(item.expiresAt) {
			delete(c.settings, key)
		}
	}
	for key, item := range c.userSettings {
		if now.After(item.expiresAt) {
			delete(c.userSettings, key)
		}
	}
	for key, item := range c.userSettingVersions {
		if now.After(item.expiresAt) {
			delete(c.userSettingVersions, key)
		}
	}
	for key, item := range c.rag {
		if now.After(item.expiresAt) {
			delete(c.rag, key)
		}
	}
	for key, stream := range c.streams {
		if stream == nil {
			delete(c.streams, key)
			continue
		}
		if stream.ownerExpired(now) && stream.activeExpired(now) && stream.cancelExpired(now) && stream.eventsExpired(now) {
			delete(c.streams, key)
		}
	}
	for key, item := range c.fixedHTTP {
		if now.After(item.expiresAt) {
			delete(c.fixedHTTP, key)
		}
	}
	for key, item := range c.providerAuthTransactions {
		if now.After(item.expiresAt) {
			delete(c.providerAuthTransactions, key)
		}
	}
	for key, item := range c.providerAuthGrants {
		if now.After(item.expiresAt) {
			delete(c.providerAuthGrants, key)
		}
	}
	cutoff := now.Add(-slidingWindowRetention)
	for key, events := range c.slidingHTTP {
		kept := events[:0]
		for _, item := range events {
			if item.After(cutoff) {
				kept = append(kept, item)
			}
		}
		if len(kept) == 0 {
			delete(c.slidingHTTP, key)
			continue
		}
		c.slidingHTTP[key] = kept
	}
	for key, item := range c.rateLimits {
		if now.After(item.backoffUntil) && now.After(item.countExpires) {
			delete(c.rateLimits, key)
		}
	}
	for upstreamID, item := range c.upstreamMeta {
		if item.lastFailure.IsZero() && item.lastSuccess.IsZero() {
			continue
		}
		if latest(item.lastFailure, item.lastSuccess).Before(now.Add(-circuitMetadataRetention)) {
			delete(c.upstreamMeta, upstreamID)
		}
	}
	for upstreamID, state := range c.upstreamCB {
		if circuitStateExpired(state, now) {
			delete(c.upstreamCB, upstreamID)
		}
	}
	for key, state := range c.modelCB {
		if circuitStateExpired(state, now) {
			delete(c.modelCB, key)
		}
	}
	for upstreamID, counter := range c.keyCounters {
		if counter.lastUsed.Before(now.Add(-keyCounterRetention)) {
			delete(c.keyCounters, upstreamID)
		}
	}
}

// circuitStateExpired 判断熔断状态是否可安全回收：
// 未处于打开/半开探测期，且最近一次失败已超出保留窗口。
func circuitStateExpired(state *circuitState, now time.Time) bool {
	if state == nil {
		return true
	}
	if !state.openUntil.IsZero() && now.Before(state.openUntil.Add(circuitProbeTTL)) {
		return false
	}
	if now.Before(state.probeUntil) {
		return false
	}
	if !state.lastFailure.IsZero() && now.Before(state.lastFailure.Add(circuitStateRetention)) {
		return false
	}
	return true
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
