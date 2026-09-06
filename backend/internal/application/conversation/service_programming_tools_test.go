package conversation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func testProgrammingRuntimeConfig(enable bool, shell bool) *config.Runtime {
	return config.NewRuntime(config.Config{
		ProgrammingEnable:      enable,
		ProgrammingShellEnable: shell,
	})
}

func TestProgrammingWorkspacePathSandbox(t *testing.T) {
	root := t.TempDir()
	workspace := programmingWorkspace{
		Root:           root,
		MaxFileBytes:   1024,
		MaxOutputChars: 1000,
	}

	full, err := workspace.resolvePath("src/main.go")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	want := filepath.Join(root, "src", "main.go")
	if full != want {
		t.Fatalf("got %q want %q", full, want)
	}

	if _, err := workspace.resolvePath("../outside.txt"); err == nil {
		t.Fatal("expected path escape to fail")
	}
	if _, err := workspace.resolvePath("/etc/passwd"); err == nil {
		t.Fatal("expected absolute path to fail")
	}
}

func TestProgrammingWriteReadEdit(t *testing.T) {
	root := t.TempDir()
	workspace := programmingWorkspace{
		Root:           root,
		MaxFileBytes:   4096,
		MaxOutputChars: 4000,
	}

	writeOut, err := workspace.writeFile(t.Context(), programmingToolArgs{
		Path:    "hello.txt",
		Content: "hello world",
	})
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	if !strings.Contains(writeOut, `"status":"written"`) {
		t.Fatalf("unexpected write result: %s", writeOut)
	}

	readOut, err := workspace.readFile(t.Context(), programmingToolArgs{Path: "hello.txt"})
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	var readPayload map[string]any
	if err := json.Unmarshal([]byte(readOut), &readPayload); err != nil {
		t.Fatalf("unmarshal read: %v", err)
	}
	if readPayload["content"] != "hello world" {
		t.Fatalf("unexpected content: %#v", readPayload["content"])
	}

	editOut, err := workspace.editFile(t.Context(), programmingToolArgs{
		Path:      "hello.txt",
		OldString: "world",
		NewString: "programming",
	})
	if err != nil {
		t.Fatalf("editFile: %v", err)
	}
	if !strings.Contains(editOut, `"status":"edited"`) {
		t.Fatalf("unexpected edit result: %s", editOut)
	}

	data, err := os.ReadFile(filepath.Join(root, "hello.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello programming" {
		t.Fatalf("unexpected file content %q", string(data))
	}
}

func TestMergeProgrammingToolRuntime(t *testing.T) {
	svc := &Service{cfg: testProgrammingRuntimeConfig(true, false)}
	runtime := svc.mergeProgrammingToolRuntime(selectedToolRuntime{}, true)
	if len(runtime.definitions) == 0 {
		t.Fatal("expected programming tool definitions")
	}
	if !runtime.hasBuiltinTool("read_file") || !runtime.hasBuiltinTool("write_file") || !runtime.hasBuiltinTool("edit_file") {
		t.Fatalf("missing core tools: %#v", runtime.builtinBindings)
	}
	if runtime.hasBuiltinTool("run_command") {
		t.Fatal("run_command should be absent when shell disabled")
	}

	withShell := (&Service{cfg: testProgrammingRuntimeConfig(true, true)}).mergeProgrammingToolRuntime(selectedToolRuntime{}, true)
	if !withShell.hasBuiltinTool("run_command") {
		t.Fatal("expected run_command when shell enabled")
	}
}
