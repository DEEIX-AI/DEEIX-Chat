package config

import "encoding/json"

// Capabilities 是服务器向客户端声明的功能能力位。客户端据此决定显示什么，
// 路由层据此拒绝不可用的功能；两者都不判断部署形态本身。
// 契约见 docs/ARCHITECTURE.md §4：只增不删，键名即契约。
type Capabilities struct {
	// MultiUser 存在多个账号：账号管理、权限组。
	MultiUser bool `json:"multiUser"`
	// Registration 可自行注册账号。
	Registration bool `json:"registration"`
	// IdentityProviders 第三方登录与身份绑定。
	IdentityProviders bool `json:"identityProviders"`
	// AccountSecurity 密码、邮箱、两步验证、会话、注销账号。
	AccountSecurity bool `json:"accountSecurity"`
	// Announcements 系统公告。
	Announcements bool `json:"announcements"`
	// BillingGating 计费规则可以拒绝请求：套餐、余额、兑换码、支付。
	BillingGating bool `json:"billingGating"`
	// UsageMetering 记录用量与费用。
	UsageMetering bool `json:"usageMetering"`
	// ContentModeration 内容审核。
	ContentModeration bool `json:"contentModeration"`
	// Sharing 对话公开分享链接；只监听回环的服务器没有可分享的对象。
	Sharing bool `json:"sharing"`
}

// Capabilities 从配置推导能力位；这是唯一的推导点。
// 本地模式只服务一个自动登录的用户，多用户、身份、门禁类功能没有对象；
// 用量计量保留，本地用户要看自己的花费。服务器模式全部开启，
// 更细的"当前是否配置了 X"由各功能自己的端点回答。
func (c Config) Capabilities() Capabilities {
	if c.LocalMode {
		return Capabilities{UsageMetering: true}
	}
	return Capabilities{
		MultiUser:         true,
		Registration:      true,
		IdentityProviders: true,
		AccountSecurity:   true,
		Announcements:     true,
		BillingGating:     true,
		UsageMetering:     true,
		ContentModeration: true,
		Sharing:           true,
	}
}

// Enabled 按契约键名查询能力位；未知键名视为不可用，拼错的键名不能放行请求。
// 键名必须与 JSON 标签一致，capabilities_test.go 校验这一点。
func (c Capabilities) Enabled(name string) bool {
	switch name {
	case "multiUser":
		return c.MultiUser
	case "registration":
		return c.Registration
	case "identityProviders":
		return c.IdentityProviders
	case "accountSecurity":
		return c.AccountSecurity
	case "announcements":
		return c.Announcements
	case "billingGating":
		return c.BillingGating
	case "usageMetering":
		return c.UsageMetering
	case "contentModeration":
		return c.ContentModeration
	case "sharing":
		return c.Sharing
	default:
		return false
	}
}

// IsFeature 报告 name 是否是契约里的能力键；路由注册时用它拒绝拼错的键名。
func IsFeature(name string) bool {
	_, ok := Capabilities{}.Flags()[name]
	return ok
}

// Flags 经 JSON 往返得到键名到值的映射，即对外契约的形状。
func (c Capabilities) Flags() map[string]bool {
	raw, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	flags := map[string]bool{}
	if err := json.Unmarshal(raw, &flags); err != nil {
		return nil
	}
	return flags
}
