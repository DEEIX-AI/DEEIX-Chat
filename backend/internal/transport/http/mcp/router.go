package mcp

import "github.com/gin-gonic/gin"

func (m *Module) RegisterRoutes(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/mcp")
	group.GET("/tools", m.Handler.ListAvailableTools)
}

func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	group := adminGroup.Group("/mcp")
	group.GET("/servers", m.Handler.ListServers)
	group.POST("/servers", m.Handler.CreateServer)
	group.PATCH("/servers/order", m.Handler.ReorderServers)
	group.PATCH("/servers/:id", m.Handler.UpdateServer)
	group.DELETE("/servers/:id", m.Handler.DeleteServer)
	group.GET("/servers/:id/tools", m.Handler.ListServerTools)
	group.PATCH("/servers/:id/tools/status", m.Handler.UpdateServerToolsStatus)
	group.POST("/servers/:id/sync", m.Handler.SyncServerTools)
	group.PATCH("/tools/:id", m.Handler.UpdateTool)
}
