// Command catalog-snapshot 拉取外部目录并刷新随二进制发布的内置快照（internal/infra/catalogdata/data）。
//
// 用法（在 backend 目录）：
//
//	go run ./cmd/catalog-snapshot [-only models-dev,openrouter-pricing] [-keep-on-error]
//	                              [-models-dev-input api.json] [-openrouter-input models.json]
//
// 数据源：
//   - models-dev：models.dev 模型目录（推理能力与输入模态）；
//   - openrouter-pricing：OpenRouter 官方模型定价。
//
// 镜像与桌面端构建会以 -keep-on-error 调用：拉取或校验失败时保留仓库中已提交的快照并以 0 退出，
// 构建不因外部服务不可达而失败。快照写入前会校验条目数，避免把截断或异常的数据打进版本。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/catalogdata"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelcatalog/modelsdev"
	openrouterpricing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelpricing/openrouter"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	sourceModelsDev         = "models-dev"
	sourceOpenRouterPricing = "openrouter-pricing"

	// 最少条目数：低于该值视为拉取异常（截断、限流页面等），不覆盖已提交的快照。
	minModelsDevEntries         = 1000
	minOpenRouterPricingEntries = 100

	fetchTimeout = 2 * time.Minute
)

type options struct {
	only            map[string]bool
	keepOnError     bool
	outputDir       string
	modelsDevInput  string
	openRouterInput string
}

func main() {
	only := flag.String("only", sourceModelsDev+","+sourceOpenRouterPricing, "要刷新的数据源，逗号分隔")
	keepOnError := flag.Bool("keep-on-error", false, "拉取或校验失败时保留已有快照并以 0 退出（构建时使用）")
	outputDir := flag.String("output-dir", catalogdata.Dir, "快照输出目录（相对 backend 模块根目录）")
	modelsDevInput := flag.String("models-dev-input", "", "本地 models.dev api.json 路径；为空时从官方地址拉取")
	openRouterInput := flag.String("openrouter-input", "", "本地 OpenRouter /models 响应 JSON 路径；为空时从官方地址拉取")
	flag.Parse()

	opts := options{
		only:            parseSources(*only),
		keepOnError:     *keepOnError,
		outputDir:       *outputDir,
		modelsDevInput:  *modelsDevInput,
		openRouterInput: *openRouterInput,
	}
	failed := false
	for _, source := range []string{sourceModelsDev, sourceOpenRouterPricing} {
		if !opts.only[source] {
			continue
		}
		if err := refresh(source, opts); err != nil {
			failed = true
			fmt.Fprintf(os.Stderr, "catalog-snapshot: %s: %v\n", source, err)
		}
	}
	if failed && !opts.keepOnError {
		os.Exit(1)
	}
	if failed {
		fmt.Fprintln(os.Stderr, "catalog-snapshot: kept the committed snapshot for failed sources")
	}
}

func parseSources(raw string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result[item] = true
		}
	}
	return result
}

func refresh(source string, opts options) error {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	fetchedAt := time.Now().UTC().Truncate(time.Second)
	switch source {
	case sourceModelsDev:
		return refreshModelsDev(ctx, opts, fetchedAt)
	case sourceOpenRouterPricing:
		return refreshOpenRouterPricing(ctx, opts, fetchedAt)
	default:
		return fmt.Errorf("unknown source")
	}
}

func refreshModelsDev(ctx context.Context, opts options, fetchedAt time.Time) error {
	var (
		raw []byte
		err error
	)
	if opts.modelsDevInput != "" {
		raw, err = os.ReadFile(opts.modelsDevInput)
	} else {
		raw, err = modelsdev.New(security.NewStrictOutboundPolicy(false)).Fetch(ctx, modelsdev.DefaultURL)
	}
	if err != nil {
		return err
	}
	entries, err := modelsdev.ParseCatalog(raw)
	if err != nil {
		return err
	}
	if len(entries) < minModelsDevEntries {
		return fmt.Errorf("only %d models, expected at least %d", len(entries), minModelsDevEntries)
	}
	data, err := modelsdev.EncodeSnapshot(domainchannel.ModelCatalogSnapshot{
		Source:    modelsdev.DefaultURL,
		FetchedAt: fetchedAt,
		Entries:   entries,
	})
	if err != nil {
		return err
	}
	reasoning, modalities := domainchannel.SplitCatalogEntries(entries)
	return write(opts, catalogdata.ModelsDevFile, data,
		fmt.Sprintf("%d models (%d with reasoning, %d with input modalities)", len(entries), len(reasoning), len(modalities)))
}

func refreshOpenRouterPricing(ctx context.Context, opts options, fetchedAt time.Time) error {
	var provider appbilling.OpenRouterPricingProvider = openrouterpricing.New(security.NewStrictOutboundPolicy(false))
	if opts.openRouterInput != "" {
		provider = fileProvider(opts.openRouterInput)
	}
	data, count, err := appbilling.EncodeOpenRouterPricingSnapshot(ctx, provider, fetchedAt)
	if err != nil {
		return err
	}
	if count < minOpenRouterPricingEntries {
		return fmt.Errorf("only %d priced models, expected at least %d", count, minOpenRouterPricingEntries)
	}
	return write(opts, catalogdata.OpenRouterPricingFile, data, fmt.Sprintf("%d priced models", count))
}

// fileProvider 从本地文件读取 OpenRouter /models 响应，用于离线重新生成快照。
type fileProvider string

func (p fileProvider) FetchModels(context.Context) ([]byte, error) {
	return os.ReadFile(string(p))
}

func write(opts options, name string, data []byte, summary string) error {
	path := filepath.Join(opts.outputDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// 先写临时文件再原子替换，构建中途中断也不会留下半个快照。
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	fmt.Printf("wrote %s: %s, %d bytes\n", path, summary, len(data))
	return nil
}
