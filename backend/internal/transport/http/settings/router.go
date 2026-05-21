package settings

import "github.com/gin-gonic/gin"

func (m *Module) RegisterPublicRoutes(api *gin.RouterGroup) {
	api.GET("/settings/login-page", m.Handler.GetLoginPageSettings)
}

func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/settings/model-option-policy", m.Handler.GetModelOptionPolicy)
}

// RegisterAdminRoutes 注册 settings 管理路由（由 superadmin 中间件保护）。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	g := adminGroup.Group("/settings")
	g.GET("", m.Handler.ListAll)
	g.GET("/tika/runtime", m.Handler.GetTikaRuntime)
	g.POST("/tika/runtime/start", m.Handler.StartTikaRuntime)
	g.POST("/tika/runtime/stop", m.Handler.StopTikaRuntime)
	g.POST("/tika/runtime/restart", m.Handler.RestartTikaRuntime)
	g.GET("/docling/runtime", m.Handler.GetDoclingRuntime)
	g.GET("/tesseract/runtime", m.Handler.GetTesseractRuntime)
	g.GET("/rapidocr/runtime", m.Handler.GetRapidOCRRuntime)
	g.GET("/mineru/runtime", m.Handler.GetMinerURuntime)
	g.POST("/rapidocr/runtime/start", m.Handler.StartRapidOCRRuntime)
	g.POST("/rapidocr/runtime/stop", m.Handler.StopRapidOCRRuntime)
	g.POST("/rapidocr/runtime/restart", m.Handler.RestartRapidOCRRuntime)
	g.GET("/embedding/runtime", m.Handler.GetEmbeddingRuntime)
	g.GET("/embedding/status", m.Handler.GetEmbeddingStatus)
	g.POST("/embedding/reindex", m.Handler.TriggerReindex)
	g.GET("/:namespace", m.Handler.ListByNamespace)
	g.PATCH("", m.Handler.Patch)
}
