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
func rewriteNileConstraintQuery(sql string, vars []interface{}) (string, []interface{}, bool) {
	compact := strings.Join(strings.Fields(sql), " ")
	switch {
	case isGORMUniqueConstraintNameQuery(compact):
		return buildNileConstraintQuery(compact, vars, nileUniqueConstraintNamesSQL, true)
	case isGORMConstraintColumnQuery(compact):
		return buildNileConstraintQuery(compact, vars, nileConstraintColumnsSQL, false)
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

func buildNileConstraintQuery(compact string, vars []interface{}, pattern string, uniqueOnly bool) (string, []interface{}, bool) {
	args := append([]interface{}(nil), vars...)
	if uniqueOnly && len(args) > 0 {
		if kind, ok := args[len(args)-1].(string); ok && strings.EqualFold(kind, "UNIQUE") {
			args = args[:len(args)-1]
		}
	}
	if strings.Contains(strings.ToUpper(compact), "CURRENT_SCHEMA()") {
		if len(args) < 2 {
			return "", nil, false
		}
		return sprintfNileSQL(pattern, "current_schema()", "$1::text"), []interface{}{args[1]}, true
	}
	if len(args) < 3 {
		return "", nil, false
	}
	return sprintfNileSQL(pattern, "$1::text", "$2::text"), []interface{}{args[1], args[2]}, true
}

func sprintfNileSQL(pattern string, schemaExpr string, tableExpr string) string {
	return strings.TrimSpace(strings.Replace(strings.Replace(pattern, "%s", schemaExpr, 1), "%s", tableExpr, 1))
}
