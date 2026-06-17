package skill

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册技能用户侧路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/skills", m.Handler.ListVisibleSkills)
	authRequired.GET("/skills/mine", m.Handler.ListMySkills)
	authRequired.POST("/skills/mine", m.Handler.CreateMySkill)
	authRequired.PATCH("/skills/mine/:id", m.Handler.PatchMySkill)
	authRequired.DELETE("/skills/mine/:id", m.Handler.DeleteMySkill)
	authRequired.GET("/skills/:id", m.Handler.GetVisibleSkill)
}

// RegisterAdminRoutes 注册技能管理路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/skills", m.Handler.ListAdminSkills)
	adminGroup.POST("/skills", m.Handler.CreateAdminSkill)
	adminGroup.PATCH("/skills/:id", m.Handler.PatchAdminSkill)
	adminGroup.DELETE("/skills/:id", m.Handler.DeleteAdminSkill)
}
