package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/gin-gonic/gin"
)

func TestDescribeUsesTypedErrorContract(t *testing.T) {
	typed := apperr.NewMasked("billing.insufficient_funds", "insufficient balance", "usage balance is insufficient")
	wrapped := fmt.Errorf("settle run 7: %w", typed)

	for _, err := range []error{typed, wrapped} {
		got := Describe(http.StatusPaymentRequired, err)
		want := Description{Status: http.StatusPaymentRequired, Code: "billing.insufficient_funds", Message: "insufficient balance"}
		if got != want {
			t.Fatalf("Describe(%v) = %#v, want %#v", err, got, want)
		}
	}
}

func TestDescribePlainErrorUsesSafeStatusContract(t *testing.T) {
	cases := []struct {
		name   string
		status int
		err    error
		want   Description
	}{
		{
			name:   "does not infer a resource from text",
			status: http.StatusNotFound,
			err:    errors.New("conversation not found"),
			want:   Description{Status: http.StatusNotFound, Code: CodeResourceNotFound, Message: "resource not found"},
		},
		{
			name:   "does not expose internal text",
			status: http.StatusInternalServerError,
			err:    errors.New("pq: permission denied for table users"),
			want:   Description{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "internal server error"},
		},
		{
			name:   "nil error is still safe",
			status: http.StatusBadRequest,
			err:    nil,
			want:   Description{Status: http.StatusBadRequest, Code: CodeRequestInvalid, Message: "invalid request"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Describe(tc.status, tc.err); got != tc.want {
				t.Fatalf("Describe() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDescribeCodeRequiresRegisteredCode(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		want   Description
	}{
		{
			name:   "registered code",
			status: http.StatusTooManyRequests,
			code:   CodeUpstreamRateLimited,
			want:   Description{Status: http.StatusTooManyRequests, Code: CodeUpstreamRateLimited, Message: "upstream rate limited"},
		},
		{
			name:   "registered media code",
			status: http.StatusBadGateway,
			code:   "media.artifact_unavailable",
			want:   Description{Status: http.StatusBadGateway, Code: "media.artifact_unavailable", Message: "generated media artifact is temporarily unavailable"},
		},
		{
			name:   "registered empty response code",
			status: http.StatusBadGateway,
			code:   "llm.empty_response",
			want:   Description{Status: http.StatusBadGateway, Code: "llm.empty_response", Message: "model returned empty response"},
		},
		{
			name:   "unknown code",
			status: http.StatusConflict,
			code:   "custom.unknown_code",
			want:   Description{Status: http.StatusConflict, Code: CodeResourceConflict, Message: "resource conflict"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DescribeCode(tc.status, tc.code); got != tc.want {
				t.Fatalf("DescribeCode() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestErrorFromWritesTypedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("ctx_request_id", "req-1")

	ErrorFrom(context, http.StatusConflict, apperr.New("knowledge_base.not_ready", "selected knowledge base has no ready files"))

	var payload Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusConflict || payload.ErrorCode != "knowledge_base.not_ready" ||
		payload.ErrorMsg != "selected knowledge base has no ready files" || payload.RequestID != "req-1" {
		t.Fatalf("envelope = %#v (status %d)", payload, recorder.Code)
	}
}

func TestErrorWithDetailsUsesCanonicalMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	details := map[string]string{"field": "code"}

	ErrorWithDetails(context, http.StatusBadRequest, "billing.invalid_redemption_code", details)

	var payload Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ErrorCode != "billing.invalid_redemption_code" || payload.ErrorMsg != "invalid redemption code" {
		t.Fatalf("envelope = %#v", payload)
	}
	decodedDetails, ok := payload.Details.(map[string]any)
	if !ok || decodedDetails["field"] != "code" {
		t.Fatalf("details = %#v", payload.Details)
	}
}

func TestCanonicalResponseCodesAreValid(t *testing.T) {
	codePattern := regexp.MustCompile(`^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)+$`)
	for code, message := range codeMessages {
		if !codePattern.MatchString(code) {
			t.Errorf("invalid response code %q", code)
		}
		if strings.TrimSpace(message) == "" {
			t.Errorf("response code %q has an empty message", code)
		}
	}
}

func TestResponseBoundaryUsesTypedAPIsAndRegisteredLiteralCodes(t *testing.T) {
	const responseImportPath = "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	forbidden := map[string]struct{}{
		"Error":              {},
		"InferErrorCode":     {},
		"PublicErrorMessage": {},
	}
	root := filepath.Clean("../../..")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		responseAliases := map[string]struct{}{}
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil || importPath != responseImportPath {
				continue
			}
			alias := "response"
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			responseAliases[alias] = struct{}{}
		}

		if file.Name.Name == "response" && filepath.Base(filepath.Dir(path)) == "response" {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if _, disallowed := forbidden[function.Name.Name]; disallowed {
					t.Fatalf("%s declares removed text-inference API %s", path, function.Name.Name)
				}
			}
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := responseAliases[identifier.Name]; !ok {
				return true
			}
			if _, disallowed := forbidden[selector.Sel.Name]; disallowed {
				t.Fatalf("%s calls removed text-inference API %s", path, selector.Sel.Name)
			}

			codeArgument := -1
			switch selector.Sel.Name {
			case "ErrorWithCode", "ErrorWithDetails":
				codeArgument = 2
			case "DescribeCode":
				codeArgument = 1
			}
			if codeArgument < 0 || len(call.Args) <= codeArgument {
				return true
			}
			literal, ok := call.Args[codeArgument].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			code, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr != nil {
				t.Fatalf("%s has invalid error code literal %s", path, literal.Value)
			}
			if _, registered := canonicalMessage(code); !registered {
				t.Fatalf("%s uses unregistered response code %q", path, code)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransportDoesNotBypassErrorEnvelope(t *testing.T) {
	root := filepath.Clean("../../transport/http")
	abortPattern := regexp.MustCompile(`AbortWithStatus\(\s*http\.Status([A-Za-z]+)\s*\)`)
	rawErrorPattern := regexp.MustCompile(`gin\.H\s*\{\s*"errorMsg"\s*:`)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return walkErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(content)
		for _, match := range abortPattern.FindAllStringSubmatch(text, -1) {
			if match[1] != "NoContent" {
				t.Fatalf("%s uses AbortWithStatus(%s); errors must use the response envelope", path, match[1])
			}
		}
		if rawErrorPattern.MatchString(text) {
			t.Fatalf("%s writes raw errorMsg JSON; use the response package", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceErrorsAreEnglishForI18nFallbacks(t *testing.T) {
	pattern := regexp.MustCompile(`(fmt\.Errorf|errors\.New|apperr\.New|apperr\.NewMasked)\("([^"]*[\p{Han}][^"]*)"`)
	for _, root := range []string{
		filepath.Clean("../../application"),
		filepath.Clean("../../transport/http"),
		filepath.Clean("../../shared/response"),
	} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
				return walkErr
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if match := pattern.FindStringSubmatch(string(content)); len(match) > 0 {
				t.Fatalf("%s has non-English API error constructor: %q", path, match[2])
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
