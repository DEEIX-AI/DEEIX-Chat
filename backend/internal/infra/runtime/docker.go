package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	runtimeport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/runtime"
)

// DockerRunner 执行本机 Docker 命令。
type DockerRunner struct{}

// NewDockerRunner 创建 Docker 命令执行器。
func NewDockerRunner() *DockerRunner {
	return &DockerRunner{}
}

// Available 判断当前环境是否存在 docker 命令。
func (r *DockerRunner) Available() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// RunWithTimeout 执行 docker 命令并返回合并输出。
func (r *DockerRunner) RunWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, "docker", args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if commandCtx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("docker_timeout")
	}
	if err != nil {
		if isContainerInspect(args) && isContainerNotFoundOutput(text) {
			return text, fmt.Errorf("%w: %s", runtimeport.ErrContainerNotFound, text)
		}
		if text == "" {
			return "", fmt.Errorf("docker_command_failed")
		}
		return text, errors.New(text)
	}
	return text, nil
}

func isContainerInspect(args []string) bool {
	return len(args) >= 2 && args[0] == "container" && args[1] == "inspect"
}

func isContainerNotFoundOutput(output string) bool {
	value := strings.ToLower(strings.TrimSpace(output))
	return strings.Contains(value, "no such object") ||
		strings.Contains(value, "no such container")
}
