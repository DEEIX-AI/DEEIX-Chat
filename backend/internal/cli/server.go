package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/app"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/buildinfo"
)

// 启动形态：
//
//	deeix-chat                        普通服务器，读 config.yaml / 环境变量
//	deeix-chat --local --data-dir D   桌面 sidecar：SQLite + 本地存储，全部落在 D；
//	                                  监听 127.0.0.1 随机端口，绑定成功后向 stdout 写一行
//	                                  握手 JSON（见 Handoff），父进程据此得知端口与一次性登录 grant。
//
// 握手只写一行、只写一次，之后 stdout 不再有任何输出（日志走 stderr），
// 父进程可以按行读取后放心地丢弃 stdout。

// Handoff 是本地模式向父进程输出的握手信息。
type Handoff struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Origin  string `json:"origin"`
	Grant   string `json:"grant"`
	PID     int    `json:"pid"`
}

// Run 解析命令行并启动服务。
func Run() error {
	return run(os.Args[1:], os.Stdout)
}

func run(args []string, handoffOut io.Writer) error {
	fs := flag.NewFlagSet("deeix-chat", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	local := fs.Bool("local", false, "run as a desktop sidecar: SQLite + local storage, loopback only")
	dataDir := fs.String("data-dir", "", "data directory for --local (required with --local)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*local {
		if *dataDir != "" {
			return errors.New("--data-dir requires --local")
		}
		instance, err := app.NewApp()
		if err != nil {
			return err
		}
		defer instance.Close()
		return instance.Run()
	}

	absDataDir, err := filepath.Abs(*dataDir)
	if *dataDir == "" || err != nil {
		return errors.New("--local requires --data-dir <absolute path>")
	}

	instance, err := app.NewAppWithOptions(app.Options{LocalDataDir: absDataDir})
	if err != nil {
		return err
	}
	defer instance.Close()

	listener, err := instance.Listen()
	if err != nil {
		return err
	}
	grant, err := instance.IssueLocalGrant()
	if err != nil {
		return err
	}
	handoff := Handoff{
		Type:    "ready",
		Version: buildinfo.ResolveVersion(),
		Origin:  "http://" + listener.Addr().String(),
		Grant:   grant,
		PID:     os.Getpid(),
	}
	line, err := json.Marshal(handoff)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(handoffOut, "%s\n", line); err != nil {
		return fmt.Errorf("write handoff: %w", err)
	}

	// Shut down when the parent dies: the shell holds our stdin pipe, so EOF
	// means it is gone — including SIGKILL, where it cannot signal us.
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		instance.RequestShutdown()
	}()

	return instance.Serve(listener)
}
