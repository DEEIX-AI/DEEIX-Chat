package channel

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册无需登录即可由 img 元素读取的安全图标内容。
func (m *Module) RegisterPublicRoutes(public *gin.RouterGroup) {
	public.GET("/llm/icon-assets/:public_id", m.Handler.GetModelIconAsset)
}

// RegisterRoutes 注册用户侧模型目录路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/models", m.Handler.ListPublicModels)
}

// RegisterAdminRoutes 注册管理员侧上游配置路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	// 上游管理
	adminGroup.GET("/llm/upstreams", m.Handler.ListUpstreams)
	adminGroup.POST("/llm/upstreams", m.Handler.CreateUpstream)
	adminGroup.POST("/llm/upstreams/batch-delete", m.Handler.BatchDeleteUpstreams)
	adminGroup.PATCH("/llm/upstreams/:id", m.Handler.UpdateUpstream)
	adminGroup.DELETE("/llm/upstreams/:id", m.Handler.DeleteUpstream)
	adminGroup.POST("/llm/upstreams/:id/circuit/open", m.Handler.OpenUpstreamCircuit)
	adminGroup.POST("/llm/upstreams/:id/circuit/reset", m.Handler.ResetUpstreamCircuit)

	// 上游模型路由绑定
	adminGroup.GET("/llm/upstreams/:id/models", m.Handler.ListUpstreamModels)
	adminGroup.POST("/llm/upstreams/:id/models", m.Handler.UpsertUpstreamModel)
	adminGroup.POST("/llm/upstreams/:id/models/batch-delete", m.Handler.BatchDeleteUpstreamModels)
	adminGroup.DELETE("/llm/upstreams/:id/models/:route_id", m.Handler.DeleteUpstreamModel)
	adminGroup.PATCH("/llm/upstreams/:id/models/:route_id/disable", m.Handler.DisableUpstreamModel)
	adminGroup.PATCH("/llm/upstreams/:id/models/:route_id/enable", m.Handler.EnableUpstreamModel)
	adminGroup.POST("/llm/upstreams/:id/models/:route_id/test", m.Handler.TestUpstreamModelRoute)
	adminGroup.POST("/llm/upstreams/:id/models/:route_id/circuit/open", m.Handler.OpenUpstreamModelCircuit)
	adminGroup.POST("/llm/upstreams/:id/models/:route_id/circuit/reset", m.Handler.ResetUpstreamModelCircuit)

	// 上游远程模型
	adminGroup.GET("/llm/upstreams/:id/models/remote", m.Handler.ListRemoteModels)
	adminGroup.POST("/llm/upstreams/:id/models/sync", m.Handler.SyncUpstreamModels)
	adminGroup.POST("/llm/upstreams/:id/models/import", m.Handler.ImportUpstreamModels)

	// 模型管理
	adminGroup.GET("/llm/models", m.Handler.ListModels)
	adminGroup.POST("/llm/models", m.Handler.CreateModel)
	adminGroup.POST("/llm/models/order", m.Handler.ReorderModels)
	adminGroup.POST("/llm/models/batch-delete", m.Handler.BatchDeleteModels)
	adminGroup.PATCH("/llm/models/display-group", m.Handler.SetModelsDisplayGroup)
	adminGroup.PATCH("/llm/models/:id", m.Handler.UpdateModel)
	adminGroup.PATCH("/llm/models/:id/protocols", m.Handler.SetModelProtocols)
	adminGroup.DELETE("/llm/models/:id", m.Handler.DeleteModel)
	adminGroup.POST("/llm/models/:id/test", m.Handler.TestModel)
	adminGroup.POST("/llm/models/:id/test-all", m.Handler.TestModelAll)
	adminGroup.GET("/llm/models/:id/sources", m.Handler.ListModelUpstreamSources)
	adminGroup.POST("/llm/models/:id/sources", m.Handler.BindModelUpstreamSource)
	adminGroup.PATCH("/llm/models/:id/sources/:route_id", m.Handler.UpdateModelUpstreamSource)
	adminGroup.POST("/llm/icon-assets", m.Handler.UploadModelIconAsset)
	adminGroup.GET("/llm/icon-assets", m.Handler.ListModelIconAssets)
	adminGroup.DELETE("/llm/icon-assets/:public_id", m.Handler.DeleteModelIconAsset)

	// 技术厂商与自定义展示分组
	adminGroup.GET("/llm/model-vendors", m.Handler.ListModelVendors)
	adminGroup.POST("/llm/model-vendors", m.Handler.CreateModelVendor)
	adminGroup.PATCH("/llm/model-vendors/:key", m.Handler.UpdateModelVendor)
	adminGroup.DELETE("/llm/model-vendors/:key", m.Handler.DeleteModelVendor)
	adminGroup.GET("/llm/model-display-groups", m.Handler.ListModelDisplayGroups)
	adminGroup.POST("/llm/model-display-groups", m.Handler.CreateModelDisplayGroup)
	adminGroup.PATCH("/llm/model-display-groups/:id", m.Handler.UpdateModelDisplayGroup)
	adminGroup.DELETE("/llm/model-display-groups/:id", m.Handler.DeleteModelDisplayGroup)

	// 全局设置
	adminGroup.GET("/llm/settings", m.Handler.ListLLMSettings)
	adminGroup.PATCH("/llm/settings/:key", m.Handler.UpdateLLMSetting)
}
