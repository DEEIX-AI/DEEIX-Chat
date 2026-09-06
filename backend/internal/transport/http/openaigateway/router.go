package openaigateway

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册 OpenAI 兼容路由（挂在 API Key 鉴权组下）。
func (m *Module) RegisterRoutes(apiKeyGroup *gin.RouterGroup) {
	apiKeyGroup.POST("/chat/completions", m.Handler.ChatCompletions)
	apiKeyGroup.GET("/models", m.Handler.ListModels)
}
