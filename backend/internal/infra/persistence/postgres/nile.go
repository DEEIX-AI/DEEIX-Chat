package db

import (
	"strings"

	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"
)

const nileInformationSchemaCatalog = "nile"

// postgresHostIsNile reports whether the DSN host is a Nile database endpoint.
// The database name is ignored: a local database named "nile" is not Nile.
func postgresHostIsNile(dsn string) bool {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.Trim(strings.TrimSpace(cfg.Host), "."))
	return host == "thenile.dev" || strings.HasSuffix(host, ".thenile.dev")
}

func registerNileConstraintRewrite(db *gorm.DB) {
	rewrite := func(tx *gorm.DB) {
		rewritten, vars, ok := rewriteNileConstraintQuery(tx.Statement.SQL.String(), tx.Statement.Vars)
		if !ok {
			return
		}
		tx.Statement.SQL.Reset()
		tx.Statement.SQL.WriteString(rewritten)
		tx.Statement.Vars = vars
	}
	_ = db.Callback().Row().Before("gorm:row").Register("deeix:nile_constraint", rewrite)
	_ = db.Callback().Query().Before("gorm:query").Register("deeix:nile_constraint", rewrite)
	_ = db.Callback().Raw().Before("gorm:raw").Register("deeix:nile_constraint", rewrite)
}

// rewriteNileConstraintQuery replaces GORM's two information_schema constraint
// joins. On Nile those joins take about 18s and, when filtered by
// current_database(), return no rows. The replacement reads pg_constraint,
// which is the same set: primary keys and unique constraints, one row per
// key column, and not unique indexes that are not constraints.
func rewriteNileConstraintQuery(sql string, vars []any) (string, []any, bool) {
	compact := strings.Join(strings.Fields(sql), " ")
	switch {
	case isGORMUniqueConstraintNameQuery(compact):
		return buildNileTableQuery(compact, vars, nileUniqueConstraintNamesSQL, true, true)
	case isGORMConstraintColumnQuery(compact):
		return buildNileTableQuery(compact, vars, nileConstraintColumnsSQL, false, true)
	case isGORMColumnMetadataQuery(compact):
		return buildNileTableQuery(compact, vars, nileColumnMetadataSQL, false, true)
	case isGORMFormatTypeQuery(compact):
		return buildNileTableQuery(compact, vars, nileFormatTypeSQL, false, false)
	default:
		return "", nil, false
	}
}

func isGORMUniqueConstraintNameQuery(compact string) bool {
	return strings.Contains(compact, "SELECT constraint_name FROM information_schema.table_constraints") &&
		strings.Contains(compact, "constraint_column_usage") &&
		strings.Contains(compact, "constraint_type =")
}

func isGORMConstraintColumnQuery(compact string) bool {
	return strings.Contains(compact, "SELECT c.column_name, constraint_name, constraint_type FROM information_schema.table_constraints") &&
		strings.Contains(compact, "constraint_column_usage")
}

func isGORMColumnMetadataQuery(compact string) bool {
	return strings.Contains(compact, "FROM information_schema.columns") &&
		strings.Contains(compact, "8 * typlen") &&
		strings.Contains(compact, "c.identity_increment")
}

func isGORMFormatTypeQuery(compact string) bool {
	return strings.Contains(compact, "format_type(a.atttypid, a.atttypmod)") &&
		strings.Contains(compact, "pg_attribute a JOIN pg_class b") &&
		strings.Contains(compact, "b.relname")
}

const nileUniqueConstraintNamesSQL = `
SELECT con.conname
FROM pg_constraint AS con
JOIN pg_attribute AS a ON a.attrelid = con.conrelid AND a.attnum = ANY (con.conkey) AND NOT a.attisdropped AND a.attnum > 0
WHERE con.conrelid = format('%I.%I', %s, %s)::regclass
  AND con.contype = 'u'`

const nileConstraintColumnsSQL = `
SELECT a.attname, con.conname,
       CASE con.contype WHEN 'p' THEN 'PRIMARY KEY' WHEN 'u' THEN 'UNIQUE' END
FROM pg_constraint AS con
JOIN pg_attribute AS a ON a.attrelid = con.conrelid AND a.attnum = ANY (con.conkey) AND NOT a.attisdropped AND a.attnum > 0
WHERE con.conrelid = format('%I.%I', %s, %s)::regclass
  AND con.contype IN ('p', 'u')`

const nileFormatTypeSQL = `
SELECT a.attname AS column_name, format_type(a.atttypid, a.atttypmod) AS data_type
FROM pg_attribute AS a
WHERE a.attrelid = format('%I.%I', %s, %s)::regclass
  AND a.attnum > 0
  AND NOT a.attisdropped`

// nileColumnMetadataSQL returns the same twelve values GORM scans from
// information_schema.columns. Serial columns keep their nextval() default;
// identity_increment stays null, which is what that view returns for them.
const nileColumnMetadataSQL = `
SELECT a.attname,
       NOT a.attnotnull,
       t.typname,
       information_schema._pg_char_max_length(a.atttypid, a.atttypmod),
       information_schema._pg_numeric_precision(a.atttypid, a.atttypmod),
       information_schema._pg_numeric_precision_radix(a.atttypid, a.atttypmod),
       information_schema._pg_numeric_scale(a.atttypid, a.atttypmod),
       information_schema._pg_datetime_precision(a.atttypid, a.atttypmod),
       8 * t.typlen,
       pg_get_expr(ad.adbin, ad.adrelid),
       col_description(a.attrelid, a.attnum),
       NULL::text
FROM pg_attribute AS a
JOIN pg_type AS t ON t.oid = a.atttypid
LEFT JOIN pg_attrdef AS ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE a.attrelid = format('%I.%I', %s, %s)::regclass
  AND a.attnum > 0
  AND NOT a.attisdropped`

func buildNileTableQuery(compact string, vars []any, pattern string, uniqueOnly bool, leadingCatalog bool) (string, []any, bool) {
	args := append([]any(nil), vars...)
	if uniqueOnly && len(args) > 0 {
		if kind, ok := args[len(args)-1].(string); ok && strings.EqualFold(kind, "UNIQUE") {
			args = args[:len(args)-1]
		}
	}
	if strings.Contains(strings.ToUpper(compact), "CURRENT_SCHEMA()") {
		index := 0
		if leadingCatalog {
			index = 1
		}
		if len(args) <= index {
			return "", nil, false
		}
		return sprintfNileSQL(pattern, "current_schema()", "$1::text"), []any{args[index]}, true
	}
	schemaIndex, tableIndex := 0, 1
	if leadingCatalog {
		schemaIndex, tableIndex = 1, 2
	}
	if len(args) <= tableIndex {
		return "", nil, false
	}
	return sprintfNileSQL(pattern, "$1::text", "$2::text"), []any{args[schemaIndex], args[tableIndex]}, true
}

func sprintfNileSQL(pattern string, schemaExpr string, tableExpr string) string {
	return strings.TrimSpace(strings.Replace(strings.Replace(pattern, "%s", schemaExpr, 1), "%s", tableExpr, 1))
}
