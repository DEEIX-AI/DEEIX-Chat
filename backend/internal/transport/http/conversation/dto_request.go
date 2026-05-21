package conversation

// CreateConversationRequest 创建会话请求。
type CreateConversationRequest struct {
	Title string `json:"title" binding:"max=255"`
	Model string `json:"model" binding:"max=128"`
}

// RenameConversationRequest 重命名会话请求。
type RenameConversationRequest struct {
	Title string `json:"title" binding:"required,max=255"`
}

// SetConversationStarRequest 设置星标请求。
type SetConversationStarRequest struct {
	Starred bool `json:"starred"`
}

// SetConversationArchiveRequest 设置归档状态请求。
type SetConversationArchiveRequest struct {
	Archived bool `json:"archived"`
}

// CreateConversationShareRequest 创建会话公开分享请求。
type CreateConversationShareRequest struct {
	DefaultMessagePublicIDs []string `json:"defaultMessagePublicIDs" binding:"max=1000"`
}

// RevokeConversationSharesRequest 批量关闭会话公开分享请求。
type RevokeConversationSharesRequest struct {
	ConversationPublicIDs []string `json:"conversationPublicIDs" binding:"max=1000"`
}

// RenameFileRequest 文件重命名请求。
type RenameFileRequest struct {
	FileName string `json:"fileName" binding:"required,max=255"`
}

// UpdateFileRequest 文件更新请求，file_name 和 rag_opt_out 至少填一个。
type UpdateFileRequest struct {
	FileName  *string `json:"fileName"`
	RagOptOut *bool   `json:"ragOptOut"`
}

// SendMessageRequest 发送消息请求。
type SendMessageRequest struct {
	ContentType           string                 `json:"contentType" binding:"required,oneof=text markdown image file mixed"`
	Content               string                 `json:"content" binding:"required,max=20000"`
	Model                 string                 `json:"model" binding:"omitempty,max=128"`
	Options               map[string]interface{} `json:"options"`
	ClientRunID           string                 `json:"clientRunID" binding:"omitempty,max=64"`
	FileIDs               []string               `json:"fileIDs" binding:"max=20"`
	SelectedToolIDs       []uint                 `json:"selectedToolIDs" binding:"max=32"`
	ParentMessagePublicID string                 `json:"parentMessagePublicID" binding:"omitempty,max=32"`
	SourceMessagePublicID string                 `json:"sourceMessagePublicID" binding:"omitempty,max=32"`
	BranchReason          string                 `json:"branchReason" binding:"omitempty,oneof=default retry edit"`
}

// MediaImageRequest 图片生成/编辑请求。
type MediaImageRequest struct {
	Prompt                string                 `json:"prompt" binding:"required,max=20000"`
	Model                 string                 `json:"model" binding:"omitempty,max=128"`
	Options               map[string]interface{} `json:"options"`
	ClientRunID           string                 `json:"clientRunID" binding:"omitempty,max=64"`
	FileIDs               []string               `json:"fileIDs" binding:"max=20"`
	ParentMessagePublicID string                 `json:"parentMessagePublicID" binding:"omitempty,max=32"`
	SourceMessagePublicID string                 `json:"sourceMessagePublicID" binding:"omitempty,max=32"`
	BranchReason          string                 `json:"branchReason" binding:"omitempty,oneof=default retry edit"`
}

// SetMessageFeedbackRequest 设置消息反馈请求。
type SetMessageFeedbackRequest struct {
	Feedback string `json:"feedback" binding:"omitempty,oneof=up down"`
}
