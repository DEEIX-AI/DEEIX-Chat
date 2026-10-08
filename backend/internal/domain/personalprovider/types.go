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
	Icon          string
	Protocol      string
	BaseURL       string
	Host          string
	APIKeyEnc     string
	KeyHint       string
	Models        []string
	Status        string
	Source        string
	LastError     string
	LastCheckedAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// HasModel 报告 model 是否在用户启用的模型列表中。
func (p Provider) HasModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	for _, item := range p.Models {
		if item == model {
			return true
		}
	}
	return false
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
