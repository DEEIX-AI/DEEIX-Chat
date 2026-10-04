package modelsdev

import (
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/catalogdata"
)

// BuiltinSnapshot 解析随二进制发布的模型目录快照（见 catalogdata），离线或远端不可用时作为兜底数据。
func BuiltinSnapshot() (domainchannel.ModelCatalogSnapshot, error) {
	snapshot, err := DecodeSnapshot(catalogdata.ModelsDev())
	snapshot.Origin = domainchannel.ModelCatalogOriginBuiltin
	return snapshot, err
}
