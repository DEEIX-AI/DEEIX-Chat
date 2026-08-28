package knowledgebase

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册知识库用户侧路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/knowledge-bases", m.Handler.ListVisible)
	authRequired.GET("/knowledge-bases/mine", m.Handler.ListMine)
	authRequired.POST("/knowledge-bases/mine", m.Handler.CreateMine)
	authRequired.PATCH("/knowledge-bases/mine/:id", m.Handler.PatchMine)
	authRequired.DELETE("/knowledge-bases/mine/:id", m.Handler.DeleteMine)
	authRequired.GET("/knowledge-bases/:id", m.Handler.GetVisible)
	authRequired.GET("/knowledge-bases/:id/files", m.Handler.ListVisibleFiles)
	authRequired.POST("/knowledge-bases/:id/files/processing/statuses", m.Handler.GetVisibleFileProcessingStatuses)
	authRequired.POST("/knowledge-bases/:id/files/processing/snapshot", m.Handler.GetVisibleFileProcessingSnapshot)
	authRequired.GET("/knowledge-bases/:id/files/:file_id/content", m.Handler.GetVisibleFileContent)
	authRequired.GET("/knowledge-bases/mine/:id/available-files", m.Handler.ListAvailableMineFiles)
	authRequired.POST("/knowledge-bases/mine/:id/files", m.Handler.AddMineFiles)
	authRequired.DELETE("/knowledge-bases/mine/:id/files/:file_id", m.Handler.RemoveMineFile)
}

// RegisterAdminRoutes 注册知识库管理员路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/knowledge-bases", m.Handler.ListAdmin)
	adminGroup.POST("/knowledge-bases", m.Handler.CreateAdmin)
	adminGroup.GET("/knowledge-bases/:id", m.Handler.GetAdmin)
	adminGroup.GET("/knowledge-bases/files", m.Handler.ListPlatformFiles)
	adminGroup.POST("/knowledge-bases/files", m.Handler.UploadAdminFile)
	adminGroup.GET("/knowledge-bases/files/:file_id/content", m.Handler.GetPlatformFileContent)
	adminGroup.DELETE("/knowledge-bases/files/:file_id", m.Handler.DeleteAdminFile)
	adminGroup.PATCH("/knowledge-bases/:id", m.Handler.PatchAdmin)
	adminGroup.DELETE("/knowledge-bases/:id", m.Handler.DeleteAdmin)
	adminGroup.GET("/knowledge-bases/:id/files", m.Handler.ListAdminFiles)
	adminGroup.POST("/knowledge-bases/:id/files/processing/statuses", m.Handler.GetAdminFileProcessingStatuses)
	adminGroup.POST("/knowledge-bases/:id/files/processing/snapshot", m.Handler.GetAdminFileProcessingSnapshot)
	adminGroup.GET("/knowledge-bases/:id/files/:file_id/content", m.Handler.GetAdminFileContent)
	adminGroup.GET("/knowledge-bases/:id/available-files", m.Handler.ListAvailableAdminFiles)
	adminGroup.POST("/knowledge-bases/:id/files", m.Handler.AddAdminFiles)
	adminGroup.DELETE("/knowledge-bases/:id/files/:file_id", m.Handler.RemoveAdminFile)
}
