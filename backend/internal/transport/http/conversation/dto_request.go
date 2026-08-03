package conversation

// CreateConversationRequest 创建会话请求。
type CreateConversationRequest struct {
	Title     string `json:"title,omitempty" binding:"max=255"`
	Model     string `json:"model,omitempty" binding:"max=128"`
	ProjectID string `json:"projectID,omitempty" binding:"omitempty,max=32"`
}

// CreateConversationProjectRequest 创建会话项目请求。
type CreateConversationProjectRequest struct {
	Name              string `json:"name" binding:"required,max=80"`
	Description       string `json:"description,omitempty" binding:"max=255"`
	SystemPrompt      string `json:"systemPrompt,omitempty" binding:"max=12000"`
	MCPDefaultMode    string `json:"mcpDefaultMode,omitempty" binding:"omitempty,oneof=inherit custom"`
	DefaultMCPToolIDs []uint `json:"defaultMCPToolIDs,omitempty" binding:"max=128"`
	DefaultSkillIDs   []uint `json:"defaultSkillIDs,omitempty" binding:"max=128"`
	Color             string `json:"color,omitempty" binding:"max=32"`
	Icon              string `json:"icon,omitempty" binding:"max=32"`
}

// UpdateConversationProjectRequest 更新会话项目请求。
type UpdateConversationProjectRequest struct {
	Name              *string `json:"name,omitempty" binding:"omitempty,max=80"`
	Description       *string `json:"description,omitempty" binding:"omitempty,max=255"`
	SystemPrompt      *string `json:"systemPrompt,omitempty" binding:"omitempty,max=12000"`
	MCPDefaultMode    *string `json:"mcpDefaultMode,omitempty" binding:"omitempty,oneof=inherit custom"`
	DefaultMCPToolIDs *[]uint `json:"defaultMCPToolIDs,omitempty" binding:"omitempty,max=128"`
	DefaultSkillIDs   *[]uint `json:"defaultSkillIDs,omitempty" binding:"omitempty,max=128"`
	Color             *string `json:"color,omitempty" binding:"omitempty,max=32"`
	Icon              *string `json:"icon,omitempty" binding:"omitempty,max=32"`
	Status            *string `json:"status,omitempty" binding:"omitempty,oneof=active archived"`
}

// ReorderConversationProjectsRequest 更新项目排序请求。
type ReorderConversationProjectsRequest struct {
	ProjectIDs []string `json:"projectIDs" binding:"required,max=200"`
}

// SetConversationProjectRequest 设置会话项目归属请求。
type SetConversationProjectRequest struct {
	ProjectID string `json:"projectID,omitempty" binding:"omitempty,max=32"`
}

// BatchSetConversationProjectRequest 批量设置会话项目归属请求。
type BatchSetConversationProjectRequest struct {
	ConversationPublicIDs []string `json:"conversationPublicIDs" binding:"required,max=1000"`
	ProjectID             string   `json:"projectID,omitempty" binding:"omitempty,max=32"`
}

// RenameConversationRequest 重命名会话请求。
type RenameConversationRequest struct {
	Title string `json:"title" binding:"required,max=255"`
}

// UpdateConversationLabelsRequest 更新会话标签请求。
type UpdateConversationLabelsRequest struct {
	Labels *[]string `json:"labels" binding:"required,max=6,dive,max=24" maxLength:"24"`
}

// SetConversationStarRequest 设置星标请求。
type SetConversationStarRequest struct {
	Starred *bool `json:"starred" binding:"required"`
}

// SetConversationArchiveRequest 设置归档状态请求。
type SetConversationArchiveRequest struct {
	Archived *bool `json:"archived" binding:"required"`
}

// CreateConversationShareRequest 创建会话公开分享请求。
type CreateConversationShareRequest struct {
	DefaultMessagePublicIDs []string `json:"defaultMessagePublicIDs,omitempty" binding:"max=1000"`
}

// RevokeConversationSharesRequest 批量关闭会话公开分享请求。
type RevokeConversationSharesRequest struct {
	ConversationPublicIDs []string `json:"conversationPublicIDs,omitempty" binding:"max=1000"`
}

// RenameFileRequest 文件重命名请求。
type RenameFileRequest struct {
	FileName string `json:"fileName" binding:"required,max=255"`
}

// UpdateFileRequest 文件更新请求，file_name 和 rag_opt_out 至少填一个。
type UpdateFileRequest struct {
	FileName  *string `json:"fileName,omitempty"`
	RagOptOut *bool   `json:"ragOptOut,omitempty"`
}

// SendMessageRequest 发送消息请求。
type SendMessageRequest struct {
	ContentType             string                 `json:"contentType" binding:"required,oneof=text markdown image file mixed"`
	Content                 string                 `json:"content" binding:"required"`
	Model                   string                 `json:"model,omitempty" binding:"omitempty,max=128"`
	Options                 map[string]interface{} `json:"options,omitempty"`
	ClientRunID             string                 `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	FileIDs                 []string               `json:"fileIDs,omitempty" binding:"max=20"`
	SelectedToolIDs         []uint                 `json:"selectedToolIDs,omitempty" binding:"max=128"`
	SkillIDs                []uint                 `json:"skillIDs,omitempty" binding:"max=128"`
	HTMLVisualPromptEnabled bool                   `json:"htmlVisualPrompt,omitempty"`
	ParentMessagePublicID   string                 `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID   string                 `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason            string                 `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// MediaImageRequest 图片生成/编辑请求。
type MediaImageRequest struct {
	Prompt                string                 `json:"prompt" binding:"required"`
	Model                 string                 `json:"model,omitempty" binding:"omitempty,max=128"`
	Options               map[string]interface{} `json:"options,omitempty"`
	ClientRunID           string                 `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	FileIDs               []string               `json:"fileIDs,omitempty" binding:"max=20"`
	MaskFileID            string                 `json:"maskFileID,omitempty" binding:"omitempty,max=128"`
	ParentMessagePublicID string                 `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID string                 `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason          string                 `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// MediaVideoRequest 视频生成请求。
type MediaVideoRequest struct {
	Prompt                string                 `json:"prompt" binding:"required"`
	Model                 string                 `json:"model,omitempty" binding:"omitempty,max=128"`
	Options               map[string]interface{} `json:"options,omitempty"`
	ClientRunID           string                 `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	FileIDs               []string               `json:"fileIDs,omitempty" binding:"max=1"`
	ParentMessagePublicID string                 `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID string                 `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason          string                 `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// SetMessageFeedbackRequest 设置消息反馈请求。
type SetMessageFeedbackRequest struct {
	Feedback string `json:"feedback,omitempty" binding:"omitempty,oneof=up down"`
}

// UpdateMessageRequest 更新消息内容请求。
type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}
