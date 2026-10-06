package channel

import "time"

// 模型目录数据来源：远端同步，或首次启动时由随版本发布的内置快照写入。
const (
	ModelCatalogOriginRemote  = "remote"
	ModelCatalogOriginBuiltin = "builtin"
)

// ModelCatalogSnapshot 是精简后的 models.dev 模型目录，storage 缓存与内置快照共用同一结构。
type ModelCatalogSnapshot struct {
	// Source 为目录数据来源地址。
	Source string
	// Origin 为 remote / builtin。
	Origin string
	// FetchedAt 为从来源拉取目录的时间。
	FetchedAt time.Time
	// Entries 只包含声明了推理能力且带有可识别 reasoning_options 的模型。
	Entries []ModelCatalogEntry
}
