package conversation

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func runProgrammingShell(ctx context.Context, workDir string, command string) (string, int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = workDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	errText := strings.TrimSpace(stderr.String())
	if errText != "" {
		if output != "" {
			output += "\n"
		}
		output += errText
	}
	exitCode := 0
	if err != nil {
		if ctx.Err() != nil {
			return output, -1, fmt.Errorf("command timed out or cancelled: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			return output, exitCode, nil
		}
		return output, -1, err
	}
	return output, exitCode, nil
}
