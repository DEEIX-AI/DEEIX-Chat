package models_test

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/schema"
)

// allowedTableDomains 是表名允许使用的业务域前缀白名单，取自当前全部表名。
// 表名须以 "<域>_" 开头，或恰为 "<域>s"（如 skills、knowledge_bases、permission_groups）。
// 新增业务域时须在此显式登记，避免表名前缀随意扩散。
var allowedTableDomains = []string{
	"announcement",
	"audit",
	"billing",
	"chat",
	"content_moderation",
	"file",
	"identity",
	"knowledge_base",
	"llm",
	"mcp",
	"permission_group",
	"prompt",
	"skill",
	"system",
	"ui",
	"user",
}

func TestSchemaModelsUseWhitelistedTablePrefixes(t *testing.T) {
	seen := make(map[string]struct{})
	for _, item := range schema.Models() {
		namer, ok := item.(interface{ TableName() string })
		if !ok {
			t.Errorf("model %T must declare TableName() explicitly", item)
			continue
		}
		tableName := namer.TableName()
		if _, duplicated := seen[tableName]; duplicated {
			t.Errorf("table name %q is declared by more than one model", tableName)
		}
		seen[tableName] = struct{}{}
		if !hasAllowedTableDomain(tableName) {
			t.Errorf("model %T table name %q does not use a whitelisted domain prefix %v", item, tableName, allowedTableDomains)
		}
	}
}

func hasAllowedTableDomain(tableName string) bool {
	for _, domain := range allowedTableDomains {
		if strings.HasPrefix(tableName, domain+"_") || tableName == domain+"s" {
			return true
		}
	}
	return false
}
