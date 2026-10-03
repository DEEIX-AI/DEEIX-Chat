package modelsdev

import (
	_ "embed"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// builtinReasoningSnapshot 是发版时从 models.dev 生成的精简推理目录（gzip JSON，格式见 EncodeReasoningSnapshot）。
// 更新方式：在 backend 目录执行 `make models-dev-snapshot`。
//
//go:embed reasoning_catalog.json.gz
var builtinReasoningSnapshot []byte

// BuiltinReasoningSnapshot 解析随二进制发布的推理目录快照，离线或远端不可用时作为兜底数据。
func BuiltinReasoningSnapshot() (domainchannel.ReasoningCatalogSnapshot, error) {
	snapshot, err := DecodeReasoningSnapshot(builtinReasoningSnapshot)
	snapshot.Origin = domainchannel.ReasoningCatalogOriginBuiltin
	return snapshot, err
}
