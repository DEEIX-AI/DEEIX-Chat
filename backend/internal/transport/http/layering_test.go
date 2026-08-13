package httpx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
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
		{
			dir: "infra",
			forbidden: []string{
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport`,
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

// TestApplicationExportedTypesStayProtocolFree 防止应用层公开类型重新绑定 HTTP/JSON 契约。
func TestApplicationExportedTypesStayProtocolFree(t *testing.T) {
	assertExportedStructsHaveNoProtocolTags(t, filepath.Clean("../../application"))
}

// TestHTTPTransportDoesNotOwnExternalIO 防止 handler 重新承担第三方出站或持久文件读写。
func TestHTTPTransportDoesNotOwnExternalIO(t *testing.T) {
	assertNoForbiddenText(t, filepath.Clean("../../transport/http"), []string{
		"http.DefaultClient",
		"http.Get(",
		"http.Post(",
		"http.Head(",
		"http.NewRequest(",
		"http.NewRequestWithContext(",
		"&http.Client{",
		"os.ReadFile(",
		"os.WriteFile(",
		"os.Open(",
		"os.Create(",
		"os.CreateTemp(",
		"os.MkdirAll(",
		"os.Rename(",
		"exec.Command(",
	})
}

// TestAuthApplicationDoesNotOwnHTTPTransport 防止认证用例重新直接创建 HTTP 客户端或请求。
func TestAuthApplicationDoesNotOwnHTTPTransport(t *testing.T) {
	assertNoForbiddenText(t, filepath.Clean("../../application/auth"), []string{
		`"net/http"`,
		"http.NewRequest(",
		"http.NewRequestWithContext(",
		"security.NewOutboundHTTPClient(",
	})
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

func assertExportedStructsHaveNoProtocolTags(t *testing.T, root string) {
	t.Helper()
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, spec := range general.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Name.IsExported() {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					if field.Tag == nil {
						continue
					}
					tag, unquoteErr := strconv.Unquote(field.Tag.Value)
					if unquoteErr != nil {
						return unquoteErr
					}
					if strings.Contains(tag, "json:") || strings.Contains(tag, "form:") || strings.Contains(tag, "header:") || strings.Contains(tag, "query:") {
						t.Fatalf("%s exports protocol tag %q on application type %s", path, tag, typeSpec.Name.Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
