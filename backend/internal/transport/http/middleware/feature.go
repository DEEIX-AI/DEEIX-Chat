package middleware

import (
	"net/http"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// FeatureDisabledDetails 是 feature.disabled 响应的 details；契约见 docs/ARCHITECTURE.md §4。
// Keys 仅在设置项被锁定时出现。
type FeatureDisabledDetails struct {
	Feature string   `json:"feature"`
	Keys    []string `json:"keys,omitempty"`
}

// WriteFeatureDisabled 写出 feature.disabled 响应；供中间件与处理器共用，保证形状一致。
func WriteFeatureDisabled(c *gin.Context, feature string, keys ...string) {
	response.ErrorWithDetails(c, http.StatusNotFound, response.CodeFeatureDisabled, FeatureDisabledDetails{Feature: feature, Keys: keys})
}

// RequireFeature 在能力位关闭时以 404 feature.disabled 拒绝整组路由。
// 挂在路由组上而不是逐条路由，避免漏挂；能力位每次请求都重新读取，管理员改配置即时生效。
func RequireFeature(runtime *config.Runtime, feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if runtime.Snapshot().Capabilities().Enabled(feature) {
			c.Next()
			return
		}
		WriteFeatureDisabled(c, feature)
		c.Abort()
	}
}

// FeatureGate 供各模块注册路由时挂门禁或查询能力位，不直接持有 config.Runtime。
type FeatureGate struct {
	runtime *config.Runtime
}

// NewFeatureGate 绑定运行时配置。
func NewFeatureGate(runtime *config.Runtime) FeatureGate {
	return FeatureGate{runtime: runtime}
}

// Require 返回能力位关闭时以 404 拒绝的中间件。键名在注册时校验：
// 拼错的键名会让整组路由在服务器模式下也返回 404，必须在启动时暴露而不是等请求。
func (g FeatureGate) Require(feature string) gin.HandlerFunc {
	if !config.IsFeature(feature) {
		panic("unknown feature gate: " + feature)
	}
	return RequireFeature(g.runtime, feature)
}

// Enabled 查询当前能力位；只给契约里明确写出的例外用，其余一律 Require。
func (g FeatureGate) Enabled(feature string) bool {
	return g.runtime.Snapshot().Capabilities().Enabled(feature)
}
