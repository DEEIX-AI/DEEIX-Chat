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
		{
			dir: "ports",
			forbidden: []string{
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra`,
				`"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository`,
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

// TestApplicationInfraAdapterRatchet 冻结 application 对 infra 业务适配器的存量依赖：
// 新增依赖直接失败；某个包还清依赖后必须同步删除对应白名单条目，确保依赖只减不增。
// 还债方式见分层规范「出站集成端口」：数据契约放 internal/ports，接口由消费方定义，infra 按端口签名实现。
func TestApplicationInfraAdapterRatchet(t *testing.T) {
	const modulePrefix = "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/"
	// 规范允许的技术型组件：无业务决策、无持久化语义。
	allowedTechnicalPrefixes := []string{
		"infra/config",
		"infra/observability",
	}
	// 存量业务适配器依赖白名单已全部清偿；此表保留为空，新增依赖会直接失败。
	allowlist := map[string][]string{}

	applicationRoot := filepath.Clean("../../application")
	actual := map[string]map[string]bool{}
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(applicationRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		relative, relErr := filepath.Rel(applicationRoot, path)
		if relErr != nil {
			return relErr
		}
		packageName := strings.Split(filepath.ToSlash(relative), "/")[0]
		file, parseErr := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, importSpec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(importSpec.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if !strings.HasPrefix(importPath, modulePrefix+"infra/") {
				continue
			}
			infraPath := strings.TrimPrefix(importPath, modulePrefix)
			if actual[packageName] == nil {
				actual[packageName] = map[string]bool{}
			}
			actual[packageName][infraPath] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	isAllowedTechnical := func(infraPath string) bool {
		for _, prefix := range allowedTechnicalPrefixes {
			if infraPath == prefix || strings.HasPrefix(infraPath, prefix+"/") {
				return true
			}
		}
		return false
	}
	inAllowlist := func(packageName, infraPath string) bool {
		for _, item := range allowlist[packageName] {
			if item == infraPath {
				return true
			}
		}
		return false
	}

	for packageName, imports := range actual {
		for infraPath := range imports {
			if isAllowedTechnical(infraPath) || inAllowlist(packageName, infraPath) {
				continue
			}
			t.Errorf("application/%s 新增了 infra 业务适配器依赖 %q：请改为通过 internal/ports 定义端口契约，而不是扩大白名单", packageName, infraPath)
		}
	}
	for packageName, entries := range allowlist {
		for _, infraPath := range entries {
			if !actual[packageName][infraPath] {
				t.Errorf("application/%s 已不再依赖 %q：请从白名单删除该条目，锁定还债进度", packageName, infraPath)
			}
		}
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
