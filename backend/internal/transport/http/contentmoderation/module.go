package contentmoderation

import "github.com/gin-gonic/gin"

// Module registers content moderation admin routes.
type Module struct {
	Handler *Handler
}

// NewModule creates the module.
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}

// RegisterRoutes registers routes under the admin group.
func (m *Module) RegisterRoutes(adminGroup *gin.RouterGroup) {
	if m == nil || m.Handler == nil {
		return
	}
	group := adminGroup.Group("/content-moderation")
	group.GET("/config", m.Handler.GetConfig)
	group.PUT("/config", m.Handler.UpdateConfig)
	group.POST("/probe", m.Handler.Probe)
	group.GET("/stats", m.Handler.GetStats)
	group.GET("/events", m.Handler.ListEvents)
	group.GET("/events/:eventID", m.Handler.GetEvent)
	group.GET("/events/:eventID/images/:index", m.Handler.GetEventImage)
}
