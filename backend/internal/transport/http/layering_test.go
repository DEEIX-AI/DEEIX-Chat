package httpx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBackendLayeringImports 防止 HTTP 边界层和内层包重新出现分层倒灌。
func TestBackendLayeringImports(t *testing.T) {
	root := filepath.Clean("../../")
	checks := []struct {
		dir       string
		forbidden []string
	}{
		{
			dir: "transport/http",
			forbidden: []string{
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"`,
				`"gorm.io/gorm"`,
				`"github.com/redis/go-redis`,
			},
		},
		{
			dir: "application",
			forbidden: []string{
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence`,
				`"github.com/gin-gonic/gin"`,
				`"gorm.io/gorm"`,
				`"github.com/redis/go-redis`,
			},
		},
		{
			dir: "repository",
			forbidden: []string{
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence`,
				`"github.com/gin-gonic/gin"`,
				`"gorm.io/gorm"`,
				`"github.com/redis/go-redis`,
			},
		},
		{
			dir: "domain",
			forbidden: []string{
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra`,
				`"github.com/gin-gonic/gin"`,
				`"gorm.io/gorm"`,
				`"github.com/redis/go-redis`,
			},
		},
	}

	for _, check := range checks {
		check := check
		t.Run(check.dir, func(t *testing.T) {
			assertNoForbiddenImports(t, filepath.Join(root, check.dir), check.forbidden)
		})
	}
}

// TestDomainTypesStayProtocolFree 防止领域对象携带 HTTP、JSON 或 ORM 契约。
func TestDomainTypesStayProtocolFree(t *testing.T) {
	assertNoForbiddenText(t, filepath.Clean("../../domain"), []string{"`json:", "`gorm:", "`form:"})
}

func assertNoForbiddenImports(t *testing.T, root string, forbidden []string) {
	t.Helper()
	assertNoForbiddenText(t, root, forbidden)
}

func assertNoForbiddenText(t *testing.T, root string, forbidden []string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(content)
		for _, item := range forbidden {
			if strings.Contains(text, item) {
				t.Fatalf("%s contains forbidden dependency or contract %q", path, item)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
