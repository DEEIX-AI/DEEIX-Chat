package memory

import "time"

const (
	maintenanceInterval      = 256
	slidingWindowRetention   = 10 * time.Minute
	circuitMetadataRetention = 30 * time.Minute
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
	for upstreamID, item := range c.rateLimits {
		if now.After(item.backoffUntil) && now.After(item.countExpires) {
			delete(c.rateLimits, upstreamID)
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
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
