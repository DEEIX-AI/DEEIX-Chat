package db

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/gorm"
)

// postgresTimeZonePattern matches the timezone handling in gorm.io/driver/postgres.
var postgresTimeZonePattern = regexp.MustCompile(`(time_zone|TimeZone|timezone)=(.*?)($|&| )`)

// openPostgres matches gorm's pgx setup and only rewrites client_encoding when
// the server reports a UTF-8 spelling other than the exact token "UTF8".
// Ordinary Postgres already reports "UTF8", so the value is left untouched.
func openPostgres(dsn string) (gorm.ConnPool, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	var timestampLocation *time.Location
	if match := postgresTimeZonePattern.FindStringSubmatch(dsn); len(match) > 2 {
		config.RuntimeParams["timezone"] = match[2]
		timestampLocation, err = time.LoadLocation(match[2])
		if err != nil {
			return nil, err
		}
	}

	return stdlib.OpenDB(*config, stdlib.OptionAfterConnect(func(ctx context.Context, conn *pgx.Conn) error {
		normalizeReportedClientEncoding(conn)
		if timestampLocation == nil {
			return nil
		}
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "timestamp",
			OID:   pgtype.TimestampOID,
			Codec: &pgtype.TimestampCodec{ScanLocation: timestampLocation},
		})
		return nil
	})), nil
}

func normalizeReportedClientEncoding(conn *pgx.Conn) {
	if conn == nil || conn.PgConn() == nil {
		return
	}
	field := reflect.ValueOf(conn.PgConn()).Elem().FieldByName("parameterStatuses")
	if !field.IsValid() || field.Kind() != reflect.Map || field.IsNil() || !field.CanAddr() {
		return
	}
	statuses := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	key := reflect.ValueOf("client_encoding")
	current := statuses.MapIndex(key)
	if !current.IsValid() || current.Kind() != reflect.String {
		return
	}
	normalized := canonicalClientEncoding(current.String())
	if normalized == "" || normalized == current.String() {
		return
	}
	statuses.SetMapIndex(key, reflect.ValueOf(normalized))
}

func canonicalClientEncoding(value string) string {
	compact := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
	if compact == "UTF8" {
		return "UTF8"
	}
	return ""
}
