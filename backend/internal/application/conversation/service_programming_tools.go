package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const (
	programmingServerName = "programming"
)

type programmingWorkspace struct {
	Root           string
	ShellEnabled   bool
	Timeout        time.Duration
	MaxFileBytes   int64
	MaxOutputChars int
}

type programmingToolArgs struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	Pattern   string `json:"pattern"`
	Glob      string `json:"glob"`
	Command   string `json:"command"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
	Recursive *bool  `json:"recursive"`
}

func programmingToolDefinitions(shellEnabled bool) []llm.ToolDefinition {
	defs := []llm.ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a UTF-8 text file from the programming workspace. Paths are relative to the workspace root.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"},"offset":{"type":"integer","description":"1-based start line (optional)"},"limit":{"type":"integer","description":"Max lines to return (optional)"}},"required":["path"]}`),
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a UTF-8 text file in the programming workspace. Parent directories are created automatically.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"},"content":{"type":"string","description":"Full file contents"}},"required":["path","content"]}`),
		},
		{
			Name:        "edit_file",
			Description: "Replace an exact occurrence of old_string with new_string in a workspace file. old_string must uniquely match once.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"}},"required":["path","old_string","new_string"]}`),
		},
		{
			Name:        "list_dir",
			Description: "List files and directories in a workspace path.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative directory path; empty or . for root"},"recursive":{"type":"boolean","description":"List recursively (default false)"}},"required":[]}`),
		},
		{
			Name:        "glob",
			Description: "Find files under the workspace matching a glob pattern (e.g. **/*.go).",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern relative to workspace"},"path":{"type":"string","description":"Optional subdirectory to search under"}},"required":["pattern"]}`),
		},
		{
			Name:        "grep",
			Description: "Search file contents in the workspace with a regular expression.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Regular expression"},"path":{"type":"string","description":"Optional file or directory"},"glob":{"type":"string","description":"Optional file name glob filter"}},"required":["pattern"]}`),
		},
		{
			Name:        "delete_file",
			Description: "Delete a file from the programming workspace.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
		},
	}
	if shellEnabled {
		defs = append(defs, llm.ToolDefinition{
			Name:        "run_command",
			Description: "Run a shell command with the programming workspace as the working directory. Prefer non-interactive commands.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Shell command to execute"}},"required":["command"]}`),
		})
	}
	return defs
}

func (s *Service) mergeProgrammingToolRuntime(runtime selectedToolRuntime, enabled bool) selectedToolRuntime {
	if !enabled {
		return runtime
	}
	cfg := s.cfg.Snapshot()
	if !cfg.ProgrammingEnable {
		return runtime
	}
	defs := programmingToolDefinitions(cfg.ProgrammingShellEnable)
	if len(defs) == 0 {
		return runtime
	}
	if runtime.nameMap == nil {
		runtime.nameMap = map[string]string{}
	}
	if runtime.mcpBindings == nil {
		runtime.mcpBindings = map[string]mcpToolCallBinding{}
	}
	if runtime.schemas == nil {
		runtime.schemas = map[string]json.RawMessage{}
	}
	if runtime.builtinBindings == nil {
		runtime.builtinBindings = map[string]builtinToolBinding{}
	}
	usedNames := map[string]int{}
	for name := range runtime.nameMap {
		usedNames[name] = 1
	}
	for _, def := range defs {
		modelName := uniqueModelToolName(llm.NormalizeToolName(def.Name), usedNames)
		if modelName == "" {
			continue
		}
		schema := def.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		runtime.definitions = append(runtime.definitions, llm.ToolDefinition{
			Name:        modelName,
			Description: def.Description,
			InputSchema: schema,
		})
		runtime.nameMap[modelName] = def.Name
		runtime.schemas[modelName] = schema
		runtime.builtinBindings[modelName] = builtinToolBinding{
			ToolName: def.Name,
			Kind:     "programming",
		}
	}
	return runtime
}

func injectProgrammingToolGuidance(messages []llm.Message, runtime selectedToolRuntime, customPrompt string) []llm.Message {
	if !runtime.hasProgrammingTools() {
		return messages
	}
	content := strings.TrimSpace(customPrompt)
	if content == "" {
		content = defaultProgrammingGuidancePrompt(runtime)
	}
	insertAt := 0
	for insertAt < len(messages) && messages[insertAt].Role == "system" {
		insertAt++
	}
	next := make([]llm.Message, 0, len(messages)+1)
	next = append(next, messages[:insertAt]...)
	next = append(next, llm.Message{Role: "system", Content: content})
	next = append(next, messages[insertAt:]...)
	return next
}

func defaultProgrammingGuidancePrompt(runtime selectedToolRuntime) string {
	var builder strings.Builder
	builder.WriteString("# programming_mode\n")
	builder.WriteString("- You are in programming mode with a private sandboxed workspace.\n")
	builder.WriteString("- Use tools to inspect and change files: read_file, write_file, edit_file, list_dir, glob, grep, delete_file")
	if runtime.hasBuiltinTool("run_command") {
		builder.WriteString(", run_command")
	}
	builder.WriteString(".\n")
	builder.WriteString("- Prefer edit_file for surgical changes; use write_file for new files or full rewrites.\n")
	builder.WriteString("- Paths are relative to the workspace root. Never attempt to escape the workspace.\n")
	builder.WriteString("- After making changes, briefly summarize what you changed and why.\n")
	builder.WriteString("- Do not expose raw tool JSON unless the user asks.\n")
	return strings.TrimSpace(builder.String())
}

func (r selectedToolRuntime) hasProgrammingTools() bool {
	for _, binding := range r.builtinBindings {
		if binding.Kind == "programming" {
			return true
		}
	}
	return false
}

func (r selectedToolRuntime) hasBuiltinTool(toolName string) bool {
	want := strings.TrimSpace(toolName)
	for _, binding := range r.builtinBindings {
		if binding.ToolName == want {
			return true
		}
	}
	return false
}

func (s *Service) resolveProgrammingWorkspace(userID uint, conversationID uint, sessionID string) (programmingWorkspace, error) {
	cfg := s.cfg.Snapshot()
	root, err := ensureProgrammingWorkspaceRoot(cfg, userID, conversationID, sessionID)
	if err != nil {
		return programmingWorkspace{}, err
	}
	timeoutSec := cfg.ProgrammingToolTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	maxBytes := cfg.ProgrammingMaxFileBytes
	if maxBytes <= 0 {
		maxBytes = 1_048_576
	}
	maxChars := cfg.ProgrammingMaxOutputChars
	if maxChars <= 0 {
		maxChars = 100_000
	}
	return programmingWorkspace{
		Root:           root,
		ShellEnabled:   cfg.ProgrammingShellEnable,
		Timeout:        time.Duration(timeoutSec) * time.Second,
		MaxFileBytes:   maxBytes,
		MaxOutputChars: maxChars,
	}, nil
}

func ensureProgrammingWorkspaceRoot(cfg config.Config, userID uint, conversationID uint, sessionID string) (string, error) {
	storageRoot := strings.TrimSpace(cfg.StorageRootDir)
	if storageRoot == "" {
		storageRoot = "./storage"
	}
	workspaceKey := ""
	if conversationID > 0 {
		workspaceKey = fmt.Sprintf("c%d", conversationID)
	} else {
		session := sanitizeWorkspaceSegment(sessionID)
		if session == "" {
			session = "ephemeral"
		}
		workspaceKey = "t_" + session
	}
	root := filepath.Join(storageRoot, "programming", fmt.Sprintf("u%d", userID), workspaceKey)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", fmt.Errorf("create programming workspace: %w", err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve programming workspace: %w", err)
	}
	return abs, nil
}

func sanitizeWorkspaceSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
		if builder.Len() >= 64 {
			break
		}
	}
	return builder.String()
}

func (s *Service) executeProgrammingTool(ctx context.Context, toolName string, argumentsJSON string, workspace programmingWorkspace) (string, error) {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "", fmt.Errorf("programming tool name is required")
	}
	args, err := parseProgrammingToolArgs(argumentsJSON)
	if err != nil {
		return "", err
	}
	timeout := workspace.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch name {
	case "read_file":
		return workspace.readFile(toolCtx, args)
	case "write_file":
		return workspace.writeFile(toolCtx, args)
	case "edit_file":
		return workspace.editFile(toolCtx, args)
	case "list_dir":
		return workspace.listDir(toolCtx, args)
	case "glob":
		return workspace.glob(toolCtx, args)
	case "grep":
		return workspace.grep(toolCtx, args)
	case "delete_file":
		return workspace.deleteFile(toolCtx, args)
	case "run_command":
		if !workspace.ShellEnabled {
			return "", fmt.Errorf("run_command is disabled by admin")
		}
		return workspace.runCommand(toolCtx, args)
	default:
		return "", fmt.Errorf("unknown programming tool %q", name)
	}
}

func parseProgrammingToolArgs(raw string) (programmingToolArgs, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return programmingToolArgs{}, nil
	}
	var args programmingToolArgs
	if err := json.Unmarshal([]byte(value), &args); err != nil {
		return programmingToolArgs{}, fmt.Errorf("invalid tool arguments: %w", err)
	}
	return args, nil
}

func (w programmingWorkspace) resolvePath(rel string) (string, error) {
	cleanedRel := strings.TrimSpace(rel)
	if cleanedRel == "" || cleanedRel == "." {
		cleanedRel = "."
	}
	cleanedRel = filepath.Clean(cleanedRel)
	if filepath.IsAbs(cleanedRel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	full := filepath.Clean(filepath.Join(w.Root, cleanedRel))
	relToRoot, err := filepath.Rel(w.Root, full)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return full, nil
}

func (w programmingWorkspace) readFile(_ context.Context, args programmingToolArgs) (string, error) {
	full, err := w.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}
	if info.Size() > w.MaxFileBytes {
		return "", fmt.Errorf("file exceeds max size of %d bytes", w.MaxFileBytes)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text")
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	offset := args.Offset
	limit := args.Limit
	if offset > 0 || limit > 0 {
		start := 0
		if offset > 1 {
			start = offset - 1
		}
		if start > len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if limit > 0 && start+limit < end {
			end = start + limit
		}
		var numbered strings.Builder
		for i := start; i < end; i++ {
			fmt.Fprintf(&numbered, "%d|%s\n", i+1, lines[i])
		}
		content = strings.TrimRight(numbered.String(), "\n")
	}
	return marshalProgrammingResult(map[string]any{
		"path":    filepath.ToSlash(args.Path),
		"content": truncateProgrammingOutput(content, w.MaxOutputChars),
		"bytes":   len(data),
	})
}

func (w programmingWorkspace) writeFile(_ context.Context, args programmingToolArgs) (string, error) {
	full, err := w.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	content := args.Content
	if int64(len(content)) > w.MaxFileBytes {
		return "", fmt.Errorf("content exceeds max size of %d bytes", w.MaxFileBytes)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0o640); err != nil {
		return "", err
	}
	return marshalProgrammingResult(map[string]any{
		"path":   filepath.ToSlash(args.Path),
		"bytes":  len(content),
		"status": "written",
	})
}

func (w programmingWorkspace) editFile(_ context.Context, args programmingToolArgs) (string, error) {
	if args.OldString == "" {
		return "", fmt.Errorf("old_string is required")
	}
	full, err := w.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text")
	}
	content := string(data)
	count := strings.Count(content, args.OldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found")
	}
	if count > 1 {
		return "", fmt.Errorf("old_string matched %d times; provide a unique occurrence", count)
	}
	updated := strings.Replace(content, args.OldString, args.NewString, 1)
	if int64(len(updated)) > w.MaxFileBytes {
		return "", fmt.Errorf("updated content exceeds max size of %d bytes", w.MaxFileBytes)
	}
	if err := os.WriteFile(full, []byte(updated), 0o640); err != nil {
		return "", err
	}
	return marshalProgrammingResult(map[string]any{
		"path":   filepath.ToSlash(args.Path),
		"status": "edited",
		"bytes":  len(updated),
	})
}

func (w programmingWorkspace) listDir(_ context.Context, args programmingToolArgs) (string, error) {
	full, err := w.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	recursive := args.Recursive != nil && *args.Recursive
	entries := make([]map[string]any, 0, 64)
	walkFn := func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == full {
			return nil
		}
		rel, relErr := filepath.Rel(w.Root, path)
		if relErr != nil {
			return relErr
		}
		item := map[string]any{
			"path":  filepath.ToSlash(rel),
			"isDir": entry.IsDir(),
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			item["size"] = info.Size()
		}
		entries = append(entries, item)
		if !recursive && entry.IsDir() && path != full {
			return filepath.SkipDir
		}
		if len(entries) >= 500 {
			return fmt.Errorf("too many entries")
		}
		return nil
	}
	if recursive {
		if err := filepath.WalkDir(full, walkFn); err != nil && !strings.Contains(err.Error(), "too many entries") {
			return "", err
		}
	} else {
		dirEntries, err := os.ReadDir(full)
		if err != nil {
			return "", err
		}
		for _, entry := range dirEntries {
			rel := entry.Name()
			if args.Path != "" && args.Path != "." {
				rel = filepath.ToSlash(filepath.Join(args.Path, entry.Name()))
			}
			item := map[string]any{
				"path":  filepath.ToSlash(rel),
				"isDir": entry.IsDir(),
			}
			if info, infoErr := entry.Info(); infoErr == nil {
				item["size"] = info.Size()
			}
			entries = append(entries, item)
			if len(entries) >= 500 {
				break
			}
		}
	}
	return marshalProgrammingResult(map[string]any{
		"path":    filepath.ToSlash(strings.TrimSpace(args.Path)),
		"entries": entries,
		"count":   len(entries),
	})
}

func (w programmingWorkspace) glob(_ context.Context, args programmingToolArgs) (string, error) {
	pattern := strings.TrimSpace(args.Pattern)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	base := w.Root
	if strings.TrimSpace(args.Path) != "" {
		resolved, err := w.resolvePath(args.Path)
		if err != nil {
			return "", err
		}
		base = resolved
	}
	matches := make([]string, 0, 64)
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(w.Root, path)
		if relErr != nil {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		ok, matchErr := filepath.Match(filepath.ToSlash(pattern), slashRel)
		if matchErr != nil {
			return matchErr
		}
		if !ok {
			// Also try matching against basename for simple patterns like "*.go".
			ok, _ = filepath.Match(pattern, entry.Name())
		}
		if ok {
			matches = append(matches, slashRel)
		}
		if len(matches) >= 200 {
			return fmt.Errorf("too many matches")
		}
		return nil
	})
	if err != nil && !strings.Contains(err.Error(), "too many matches") {
		return "", err
	}
	return marshalProgrammingResult(map[string]any{
		"pattern": pattern,
		"matches": matches,
		"count":   len(matches),
	})
}

func (w programmingWorkspace) grep(_ context.Context, args programmingToolArgs) (string, error) {
	pattern := strings.TrimSpace(args.Pattern)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regexp: %w", err)
	}
	startPath := w.Root
	if strings.TrimSpace(args.Path) != "" {
		resolved, resolveErr := w.resolvePath(args.Path)
		if resolveErr != nil {
			return "", resolveErr
		}
		startPath = resolved
	}
	globFilter := strings.TrimSpace(args.Glob)
	type hit struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Content string `json:"content"`
	}
	hits := make([]hit, 0, 64)
	walkErr := filepath.WalkDir(startPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if globFilter != "" {
			ok, matchErr := filepath.Match(globFilter, entry.Name())
			if matchErr != nil || !ok {
				return nil
			}
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.Size() > w.MaxFileBytes || info.Size() == 0 {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || !utf8.Valid(data) {
			return nil
		}
		rel, relErr := filepath.Rel(w.Root, path)
		if relErr != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				hits = append(hits, hit{
					Path:    filepath.ToSlash(rel),
					Line:    i + 1,
					Content: truncateProgrammingOutput(line, 400),
				})
				if len(hits) >= 100 {
					return fmt.Errorf("too many matches")
				}
			}
		}
		return nil
	})
	if walkErr != nil && !strings.Contains(walkErr.Error(), "too many matches") {
		return "", walkErr
	}
	return marshalProgrammingResult(map[string]any{
		"pattern": pattern,
		"hits":    hits,
		"count":   len(hits),
	})
}

func (w programmingWorkspace) deleteFile(_ context.Context, args programmingToolArgs) (string, error) {
	full, err := w.resolvePath(args.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("refusing to delete directories; delete files only")
	}
	if err := os.Remove(full); err != nil {
		return "", err
	}
	return marshalProgrammingResult(map[string]any{
		"path":   filepath.ToSlash(args.Path),
		"status": "deleted",
	})
}

func (w programmingWorkspace) runCommand(ctx context.Context, args programmingToolArgs) (string, error) {
	command := strings.TrimSpace(args.Command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	if err := validateProgrammingCommand(command); err != nil {
		return "", err
	}
	output, exitCode, err := runProgrammingShell(ctx, w.Root, command)
	result := map[string]any{
		"command":  command,
		"cwd":      w.Root,
		"exitCode": exitCode,
		"output":   truncateProgrammingOutput(output, w.MaxOutputChars),
		"os":       runtime.GOOS,
	}
	if err != nil && exitCode == -1 {
		return "", err
	}
	if err != nil {
		result["error"] = err.Error()
	}
	return marshalProgrammingResult(result)
}

func validateProgrammingCommand(command string) error {
	lower := strings.ToLower(command)
	blocked := []string{
		"rm -rf /",
		"rm -rf /*",
		"mkfs",
		"format c:",
		":(){:|:&};:",
		"shutdown",
		"reboot",
		"diskpart",
	}
	for _, item := range blocked {
		if strings.Contains(lower, item) {
			return fmt.Errorf("command blocked by safety policy")
		}
	}
	return nil
}

func marshalProgrammingResult(payload map[string]any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func truncateProgrammingOutput(value string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(value) <= maxChars {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxChars]) + "\n...[truncated]"
}
