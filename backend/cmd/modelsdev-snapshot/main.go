// Command modelsdev-snapshot 从 models.dev 拉取目录并生成内置推理目录快照。
//
// 用法（在 backend 目录）：
//
//	go run ./cmd/modelsdev-snapshot [-url https://models.dev/api.json] [-input api.json] [-output path]
//
// 指定 -input 时从本地文件读取，不访问网络。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelcatalog/modelsdev"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func main() {
	url := flag.String("url", modelsdev.DefaultURL, "models.dev api.json 地址")
	input := flag.String("input", "", "本地 api.json 路径；为空时从 -url 拉取")
	output := flag.String("output", "internal/infra/modelcatalog/modelsdev/reasoning_catalog.json.gz", "快照输出路径")
	flag.Parse()

	if err := run(*url, *input, *output); err != nil {
		fmt.Fprintln(os.Stderr, "modelsdev-snapshot:", err)
		os.Exit(1)
	}
}

func run(url string, input string, output string) error {
	var (
		raw []byte
		err error
	)
	if input != "" {
		raw, err = os.ReadFile(input)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		raw, err = modelsdev.New(security.NewStrictOutboundPolicy(false)).Fetch(ctx, url)
	}
	if err != nil {
		return err
	}
	entries, err := modelsdev.ParseReasoningCatalog(raw)
	if err != nil {
		return err
	}
	data, err := modelsdev.EncodeReasoningSnapshot(domainchannel.ReasoningCatalogSnapshot{
		Source:    url,
		FetchedAt: time.Now().UTC().Truncate(time.Second),
		Entries:   entries,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d reasoning models, %d bytes\n", output, len(entries), len(data))
	return nil
}
