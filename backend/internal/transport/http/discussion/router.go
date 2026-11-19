package discussion

import "github.com/gin-gonic/gin"

func (m *Module) RegisterRoutes(authGroup *gin.RouterGroup) {
	group := authGroup.Group("/discussion-templates")
	group.GET("", m.Handler.ListActive)
}

func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	group := adminGroup.Group("/discussion-templates")
	group.GET("", m.Handler.ListAdmin)
	group.POST("", m.Handler.Create)
	group.PATCH("/:id", m.Handler.Update)
	group.DELETE("/:id", m.Handler.Delete)
}
