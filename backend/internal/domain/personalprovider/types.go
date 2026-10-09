package personalprovider

import (
	"strconv"
	"strings"
	"time"
)

const (
	// StatusActive 表示服务可用。
	StatusActive = "active"
	// StatusDisabled 表示用户自己停用。
	StatusDisabled = "disabled"
	// StatusSuspended 表示管理员停用；用户不能自行恢复。
	StatusSuspended = "suspended"

	// SourceManual 表示用户在设置页手动添加。
	SourceManual = "manual"
	// SourceLink 表示通过一键导入链接添加。
	SourceLink = "link"
)

// Provider 是一个用户自带 Key 的模型服务。API Key 只以密文保存，任何读取路径都不返回明文。
type Provider struct {
	ID          uint
	PublicID    string
	OwnerUserID uint
	Name        string
	// Icon 是用户选择的内置图标 slug；空串表示按服务地址自动匹配。
	Icon string
	// Protocol 是服务的接口协议，用于拉取模型目录，也是对话模型的默认协议。
	Protocol      string
	BaseURL       string
	Host          string
	APIKeyEnc     string
	KeyHint       string
	Models        []Model
	Status        string
	Source        string
	LastError     string
	LastCheckedAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Model 是用户启用的一个模型及其调用协议。Protocols 为单个协议，或同一媒体模型配套的一组协议
// （如图片生成 + 图片编辑），规则与平台模型绑定一致。
type Model struct {
	Name      string
	Protocols []string
}

// FindModel 返回用户启用的同名模型。
func (p Provider) FindModel(name string) (Model, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Model{}, false
	}
	for _, item := range p.Models {
		if item.Name == name {
			return item, true
		}
	}
	return Model{}, false
}

// KeyHint 返回只用于展示的 Key 提示，不足以还原或猜测完整 Key。
func KeyHint(key string) string {
	key = strings.TrimSpace(key)
	runes := []rune(key)
	switch {
	case len(runes) == 0:
		return ""
	case len(runes) < 12:
		return "••••"
	default:
		prefix := string(runes[:3])
		return prefix + "••••" + string(runes[len(runes)-4:])
	}
}

// APIKeyBinding 是 API Key 密文绑定的上下文（所属用户 + 服务公开 ID）。密文只能在原记录上解密：
// 即使数据库被改写，把密文挪到别的服务或别的用户名下也无法使用。格式写入后不可更改。
func APIKeyBinding(ownerUserID uint, publicID string) string {
	return "personal_provider.api_key:" + strconv.FormatUint(uint64(ownerUserID), 10) + ":" + strings.TrimSpace(publicID)
}
