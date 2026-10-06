// Package catalogdata 存放随二进制发布的外部目录快照，是远端不可达时的兜底数据。
//
// 每份快照对应一个外部数据源，运行时以 storage/catalogs 中最近一次同步的结果为准；
// 缺失、损坏或比内置快照旧时，由内置快照写入 storage 作为种子，之后照常按需同步远端。
//
// 快照由 `make catalog-snapshot`（cmd/catalog-snapshot）生成。镜像与桌面端构建时会先刷新一次，
// 拉取失败则保留仓库中已提交的版本，构建不受影响。
package catalogdata

import (
	"embed"
)

// 快照文件名，位于 data 目录；生成器与读取方共用。
const (
	// ModelsDevFile 是 models.dev 模型目录（推理能力与输入模态），格式见 modelsdev.EncodeSnapshot。
	ModelsDevFile = "models-dev.json.gz"
	// OpenRouterPricingFile 是 OpenRouter 官方模型定价，格式见 billing.EncodeOpenRouterPricingSnapshot。
	OpenRouterPricingFile = "openrouter-pricing.json.gz"
)

// Dir 是快照目录相对 backend 模块根目录的路径，生成器写入时使用。
const Dir = "internal/infra/catalogdata/data"

//go:embed data/*.json.gz
var files embed.FS

// ModelsDev 返回内置的 models.dev 模型目录快照（gzip JSON）。
func ModelsDev() []byte {
	return read(ModelsDevFile)
}

// OpenRouterPricing 返回内置的 OpenRouter 官方定价快照（gzip JSON）。
func OpenRouterPricing() []byte {
	return read(OpenRouterPricingFile)
}

func read(name string) []byte {
	data, err := files.ReadFile("data/" + name)
	if err != nil {
		return nil
	}
	return data
}
