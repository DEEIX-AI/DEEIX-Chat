package conversation

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册会话域路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.POST("/conversations", m.Handler.CreateConversation)
	authRequired.GET("/conversations", m.Handler.ListConversations)
	authRequired.POST("/conversations/shares/revoke", m.Handler.RevokeConversationShares)
	authRequired.GET("/conversation-projects", m.Handler.ListConversationProjects)
	authRequired.POST("/conversation-projects", m.Handler.CreateConversationProject)
	authRequired.POST("/conversation-projects/reorder", m.Handler.ReorderConversationProjects)
	authRequired.PATCH("/conversation-projects/:id", m.Handler.UpdateConversationProject)
	authRequired.DELETE("/conversation-projects/:id", m.Handler.DeleteConversationProject)
	authRequired.POST("/conversations/project", m.Handler.BatchSetConversationProject)
	authRequired.GET("/conversations/:id", m.Handler.GetConversation)
	authRequired.GET("/conversations/:id/export", m.Handler.ExportConversation)
	authRequired.PATCH("/conversations/:id/title", m.Handler.RenameConversation)
	authRequired.PATCH("/conversations/:id/star", m.Handler.SetConversationStar)
	authRequired.PATCH("/conversations/:id/archive", m.Handler.SetConversationArchive)
	authRequired.PATCH("/conversations/:id/project", m.Handler.SetConversationProject)
	authRequired.DELETE("/conversations/:id", m.Handler.DeleteConversation)
	authRequired.GET("/conversations/:id/share", m.Handler.GetConversationShare)
	authRequired.POST("/conversations/:id/share", m.Handler.CreateConversationShare)
	authRequired.DELETE("/conversations/:id/share", m.Handler.RevokeConversationShare)
	authRequired.POST("/conversations/:id/share/regenerate", m.Handler.RegenerateConversationShare)
	authRequired.POST("/shared-conversations/:share_id/clone", m.Handler.CloneSharedConversation)
	authRequired.GET("/conversations/:id/messages", m.Handler.ListMessages)
	authRequired.GET("/conversations/:id/runs", m.Handler.ListConversationRuns)
	authRequired.POST("/conversations/:id/messages", m.Handler.SendMessage)
	authRequired.POST("/conversations/:id/messages/stream", m.Handler.StreamMessage)
	authRequired.POST("/conversations/:id/media/images/generations/stream", m.Handler.StreamImageGeneration)
	authRequired.POST("/conversations/:id/media/images/edits/stream", m.Handler.StreamImageEdit)
	authRequired.GET("/context-artifacts/:id", m.Handler.GetContextArtifact)
	authRequired.GET("/conversation-runs/:run_id/stream", m.Handler.ResumeMessageGenerationStream)
	authRequired.POST("/conversation-runs/:run_id/cancel", m.Handler.CancelMessageGeneration)
	authRequired.PATCH("/messages/:id", m.Handler.UpdateMessage)
	authRequired.PUT("/messages/:id/feedback", m.Handler.SetMessageFeedback)
	authRequired.POST("/files", m.Handler.UploadFile)
	authRequired.GET("/files", m.Handler.ListFiles)
	authRequired.GET("/files/:file_id/processing", m.Handler.GetFileProcessingStatus)
	authRequired.GET("/files/:file_id/extract", m.Handler.GetFileExtract)
	authRequired.PATCH("/files/:file_id", m.Handler.UpdateFile)
	authRequired.GET("/files/:file_id/content", m.Handler.GetFileContent)
	authRequired.DELETE("/files/:file_id", m.Handler.DeleteFile)
	authRequired.GET("/runtime/chat-file-policy", m.Handler.GetChatFilePolicy)
}

// RegisterPublicRoutes 注册不需要登录的会话公开路由。
func (m *Module) RegisterPublicRoutes(public *gin.RouterGroup) {
	public.GET("/shared-conversations/:share_id", m.Handler.GetPublicSharedConversation)
	public.GET("/shared-conversations/:share_id/files/:file_id/content", m.Handler.GetPublicSharedFileContent)
}
