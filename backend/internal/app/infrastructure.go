package app

import (
	"context"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache"
	platformhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http"
	"gorm.io/gorm"
)

type healthChecker struct {
	db    *gorm.DB
	cache cache.Backend
}

func newHealthChecker(db *gorm.DB, cache cache.Backend) platformhttp.HealthChecker {
	return &healthChecker{db: db, cache: cache}
}

func (h *healthChecker) CheckHealth(ctx context.Context) ([]platformhttp.HealthCheck, bool) {
	checks := make([]platformhttp.HealthCheck, 0, 2)
	healthy := true

	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "error"})
			healthy = false
		} else {
			dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err = sqlDB.PingContext(dbCtx); err != nil {
				checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "error"})
				healthy = false
			} else {
				checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "ok"})
			}
		}
	} else {
		checks = append(checks, platformhttp.HealthCheck{Name: "db", Status: "not_configured"})
	}

	if h.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := h.cache.Ping(cacheCtx); err != nil {
			checks = append(checks, platformhttp.HealthCheck{Name: h.cache.Name(), Status: "error"})
			healthy = false
		} else {
			checks = append(checks, platformhttp.HealthCheck{Name: h.cache.Name(), Status: "ok"})
		}
	} else {
		checks = append(checks, platformhttp.HealthCheck{Name: "cache", Status: "not_configured"})
		healthy = false
	}

	return checks, healthy
}
