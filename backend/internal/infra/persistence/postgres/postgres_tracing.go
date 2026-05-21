package db

import (
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const gormTraceSpanKey = "deeix-chat:postgres-trace-span"

func configureTracing(db *gorm.DB, cfg config.Config) error {
	attrs := postgresTraceAttributes(cfg)
	registrations := []error{
		db.Callback().Create().Before("gorm:create").Register("deeix-chat:trace-before-create", beginGORMTrace("create", attrs)),
		db.Callback().Create().After("gorm:create").Register("deeix-chat:trace-after-create", endGORMTrace),
		db.Callback().Query().Before("gorm:query").Register("deeix-chat:trace-before-query", beginGORMTrace("query", attrs)),
		db.Callback().Query().After("gorm:query").Register("deeix-chat:trace-after-query", endGORMTrace),
		db.Callback().Update().Before("gorm:update").Register("deeix-chat:trace-before-update", beginGORMTrace("update", attrs)),
		db.Callback().Update().After("gorm:update").Register("deeix-chat:trace-after-update", endGORMTrace),
		db.Callback().Delete().Before("gorm:delete").Register("deeix-chat:trace-before-delete", beginGORMTrace("delete", attrs)),
		db.Callback().Delete().After("gorm:delete").Register("deeix-chat:trace-after-delete", endGORMTrace),
		db.Callback().Row().Before("gorm:row").Register("deeix-chat:trace-before-row", beginGORMTrace("row", attrs)),
		db.Callback().Row().After("gorm:row").Register("deeix-chat:trace-after-row", endGORMTrace),
		db.Callback().Raw().Before("gorm:raw").Register("deeix-chat:trace-before-raw", beginGORMTrace("raw", attrs)),
		db.Callback().Raw().After("gorm:raw").Register("deeix-chat:trace-after-raw", endGORMTrace),
	}
	for _, err := range registrations {
		if err != nil {
			return err
		}
	}
	return nil
}

func beginGORMTrace(operation string, baseAttrs []attribute.KeyValue) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		attrs := make([]attribute.KeyValue, 0, len(baseAttrs)+3)
		attrs = append(attrs, baseAttrs...)
		attrs = append(attrs, attribute.String("db.operation", operation))
		if table := strings.TrimSpace(tx.Statement.Table); table != "" {
			attrs = append(attrs, attribute.String("db.table", table))
		}
		ctx, span := platformtracing.Start(
			tx.Statement.Context,
			"db.postgresql."+operation,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attrs...),
		)
		tx.Statement.Context = ctx
		tx.InstanceSet(gormTraceSpanKey, span)
	}
}

func endGORMTrace(tx *gorm.DB) {
	if tx == nil {
		return
	}
	value, ok := tx.InstanceGet(gormTraceSpanKey)
	if !ok {
		return
	}
	span, ok := value.(trace.Span)
	if !ok {
		return
	}
	if tx.RowsAffected >= 0 {
		span.SetAttributes(attribute.Int64("db.rows_affected", tx.RowsAffected))
	}
	platformtracing.RecordError(span, tx.Error)
	span.End()
}

func postgresTraceAttributes(cfg config.Config) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "PostgreSQL"),
	}
	parsed, err := url.Parse(strings.TrimSpace(cfg.PostgresDSN))
	if err != nil {
		return attrs
	}
	if parsed.Host != "" {
		attrs = append(attrs, attribute.String("server.address", parsed.Host))
	}
	if database := strings.TrimPrefix(parsed.Path, "/"); database != "" {
		attrs = append(attrs, attribute.String("db.name", database))
	}
	return attrs
}
