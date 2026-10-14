package conversation

import "encoding/json"

// CreateConversationRequest 创建会话请求。
type CreateConversationRequest struct {
	Title     string `json:"title,omitempty" binding:"max=255"`
	Model     string `json:"model,omitempty" binding:"max=128"`
	ProjectID string `json:"projectID,omitempty" binding:"omitempty,max=32"`
}

// CreateConversationProjectRequest 创建会话项目请求。
type CreateConversationProjectRequest struct {
	Name                    string   `json:"name" binding:"required,max=80"`
	Description             string   `json:"description,omitempty" binding:"max=255"`
	SystemPrompt            string   `json:"systemPrompt,omitempty" binding:"max=12000"`
	DefaultModel            string   `json:"defaultModel,omitempty" binding:"max=128"`
	MCPDefaultMode          string   `json:"mcpDefaultMode,omitempty" binding:"omitempty,oneof=inherit custom"`
	DefaultMCPToolIDs       []uint   `json:"defaultMCPToolIDs,omitempty" binding:"max=128"`
	DefaultSkillIDs         []uint   `json:"defaultSkillIDs,omitempty" binding:"max=128"`
	DefaultKnowledgeBaseIDs []string `json:"defaultKnowledgeBaseIDs,omitempty" binding:"max=8,dive,required,max=32"`
	Color                   string   `json:"color,omitempty" binding:"max=32"`
	Icon                    string   `json:"icon,omitempty" binding:"max=32"`
}

// UpdateConversationProjectRequest 更新会话项目请求。
type UpdateConversationProjectRequest struct {
	Name                    *string   `json:"name,omitempty" binding:"omitempty,max=80"`
	Description             *string   `json:"description,omitempty" binding:"omitempty,max=255"`
	SystemPrompt            *string   `json:"systemPrompt,omitempty" binding:"omitempty,max=12000"`
	DefaultModel            *string   `json:"defaultModel,omitempty" binding:"omitempty,max=128"`
	MCPDefaultMode          *string   `json:"mcpDefaultMode,omitempty" binding:"omitempty,oneof=inherit custom"`
	DefaultMCPToolIDs       *[]uint   `json:"defaultMCPToolIDs,omitempty" binding:"omitempty,max=128"`
	DefaultSkillIDs         *[]uint   `json:"defaultSkillIDs,omitempty" binding:"omitempty,max=128"`
	DefaultKnowledgeBaseIDs *[]string `json:"defaultKnowledgeBaseIDs,omitempty" binding:"omitempty,max=8,dive,required,max=32"`
	Color                   *string   `json:"color,omitempty" binding:"omitempty,max=32"`
	Icon                    *string   `json:"icon,omitempty" binding:"omitempty,max=32"`
	Status                  *string   `json:"status,omitempty" binding:"omitempty,oneof=active archived"`
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
	RAGOptOut *bool   `json:"ragOptOut,omitempty"`
}

// GetFileProcessingStatusesRequest 批量文件处理状态查询请求。
type GetFileProcessingStatusesRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// SubmitFileEmbeddingsRequest 批量提交文件向量化请求。
type SubmitFileEmbeddingsRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// GetConversationRunStatusesRequest 批量运行状态查询请求。
type GetConversationRunStatusesRequest struct {
	RunIDs []string `json:"runIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// SendMessageRequest 发送消息请求。
type SendMessageRequest struct {
	ContentType string `json:"contentType" binding:"required,oneof=text markdown image file mixed"`
	Content     string `json:"content" binding:"required"`
	Model       string `json:"model,omitempty" binding:"omitempty,max=128"`
	// Options 为高级参数 JSON：仅管理员生效；普通用户只保留原生工具选择（tools）。
	Options map[string]any `json:"options,omitempty"`
	// Controls 为模型控件选择 {控件 id: 取值}，取值为字符串、布尔或数值；未知控件与无效取值被忽略。
	Controls                map[string]any `json:"controls,omitempty"`
	ClientRunID             string         `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	FileIDs                 []string       `json:"fileIDs,omitempty" binding:"max=20"`
	SelectedToolIDs         []uint         `json:"selectedToolIDs,omitempty" binding:"max=128"`
	SkillIDs                []uint         `json:"skillIDs,omitempty" binding:"max=128"`
	KnowledgeBaseIDs        []string       `json:"knowledgeBaseIDs,omitempty" binding:"max=8,dive,required,max=32"`
	HTMLVisualPromptEnabled bool           `json:"htmlVisualPrompt,omitempty"`
	ParentMessagePublicID   string         `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID   string         `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason            string         `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`

	// UIComponentIDs 是本次会话勾选的交互式组件；后端据此注入组件目录提示词，不可见的 ID 被忽略。
	UIComponentIDs []uint `json:"uiComponentIDs,omitempty" binding:"max=32"`

	// UIBlockVersion 是客户端能渲染的 deeix-ui 块格式版本：缺省或 1 只用静态组件，2 允许可交互块（state、computed、原语）。
	UIBlockVersion int `json:"uiBlockVersion,omitempty" binding:"omitempty,min=1,max=2"`

	// UIStates 是用户在当前分支的交互式组件里设定的状态（只含与初始值不同的键），作为本轮上下文发给模型，不落库。
	// 条数与字段长度超过这里声明的上限时按请求体不合法拒绝；除此之外，不在当前分支上的消息、
	// 状态内容不合法或超出注入预算的条目只是被忽略，不会让发送失败。
	UIStates []MessageUIStateInputRequest `json:"uiStates,omitempty" binding:"max=32,dive"`
}

// MessageUIStateInputRequest 是随发送请求提交的单个组件块当前状态。
type MessageUIStateInputRequest struct {
	// MessagePublicID 是组件所在 assistant 消息的 public_id。
	MessagePublicID string `json:"messagePublicID" binding:"max=64"`
	BlockID         string `json:"blockID" binding:"max=128"`
	// Title 是组件标题，帮助模型对应到具体组件；超过 40 个字符的部分被截断。
	Title string          `json:"title,omitempty" binding:"max=200"`
	State json.RawMessage `json:"state,omitempty" swaggertype:"object"`
}

// TemporaryChatMessageRequest 是仅在当前页面内维护的临时对话请求。
// 历史正文和请求级附件由浏览器逐轮提交，服务端不创建会话、消息或文件记录。
type TemporaryChatMessageRequest struct {
	SessionID        string                        `json:"sessionID" binding:"required,max=64"`
	ClientRunID      string                        `json:"clientRunID" binding:"required,max=64"`
	Model            string                        `json:"model" binding:"required,max=128"`
	Options          map[string]any                `json:"options,omitempty"`
	Controls         map[string]any                `json:"controls,omitempty"`
	SelectedToolIDs  []uint                        `json:"selectedToolIDs,omitempty" binding:"max=128"`
	SkillIDs         []uint                        `json:"skillIDs,omitempty" binding:"max=128"`
	KnowledgeBaseIDs []string                      `json:"knowledgeBaseIDs,omitempty" binding:"omitempty,max=8,dive,max=32"`
	HTMLVisualPrompt bool                          `json:"htmlVisualPrompt,omitempty"`
	Messages         []TemporaryChatHistoryMessage `json:"messages" binding:"required,min=1,max=100,dive"`

	// UIComponentIDs 是本次会话勾选的交互式组件。
	UIComponentIDs []uint `json:"uiComponentIDs,omitempty" binding:"max=32"`

	// UIBlockVersion 语义同 SendMessageRequest.uiBlockVersion。
	UIBlockVersion int `json:"uiBlockVersion,omitempty" binding:"omitempty,min=1,max=2"`

	// UIStates 是用户在本次临时对话的交互式组件里设定的状态；语义同 SendMessageRequest.uiStates。
	UIStates []TemporaryChatUIStateRequest `json:"uiStates,omitempty" binding:"max=32,dive"`
}

// TemporaryChatUIStateRequest 是临时对话里单个组件块的当前状态。
// 临时对话没有消息记录，用 messages 数组的下标定位组件所在的 assistant 消息。
type TemporaryChatUIStateRequest struct {
	MessageIndex int             `json:"messageIndex" binding:"min=0,max=99"`
	BlockID      string          `json:"blockID" binding:"max=128"`
	Title        string          `json:"title,omitempty" binding:"max=200"`
	State        json.RawMessage `json:"state,omitempty" swaggertype:"object"`
}

// TemporaryChatHistoryMessage 是临时对话可提交的消息。
type TemporaryChatHistoryMessage struct {
	Role    string `json:"role" binding:"required,oneof=user assistant"`
	Content string `json:"content" binding:"max=200000"`
}

// MediaImageRequest 图片生成/编辑请求。
type MediaImageRequest struct {
	Prompt                string         `json:"prompt" binding:"required"`
	Model                 string         `json:"model,omitempty" binding:"omitempty,max=128"`
	Options               map[string]any `json:"options,omitempty"`
	ClientRunID           string         `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	FileIDs               []string       `json:"fileIDs,omitempty" binding:"max=20"`
	MaskFileID            string         `json:"maskFileID,omitempty" binding:"omitempty,max=128"`
	ParentMessagePublicID string         `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID string         `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason          string         `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// MediaVideoRequest 视频生成请求。
type MediaVideoRequest struct {
	Prompt                string         `json:"prompt" binding:"required"`
	Model                 string         `json:"model,omitempty" binding:"omitempty,max=128"`
	Options               map[string]any `json:"options,omitempty"`
	ClientRunID           string         `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	FileIDs               []string       `json:"fileIDs,omitempty" binding:"max=1"`
	ParentMessagePublicID string         `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID string         `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason          string         `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// MediaVideoExtensionRequest 视频扩展请求。
type MediaVideoExtensionRequest struct {
	Prompt                string         `json:"prompt" binding:"required"`
	Model                 string         `json:"model,omitempty" binding:"omitempty,max=128"`
	Options               map[string]any `json:"options,omitempty"`
	ClientRunID           string         `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	SourceVideoFileID     string         `json:"sourceVideoFileID" binding:"required,max=128"`
	ParentMessagePublicID string         `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID string         `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason          string         `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// SetMessageFeedbackRequest 设置消息反馈请求。
type SetMessageFeedbackRequest struct {
	Feedback string `json:"feedback,omitempty" binding:"omitempty,oneof=up down"`
}

// SetMessageUIStateRequest 保存助手消息内交互式组件的状态快照。
type SetMessageUIStateRequest struct {
	// Blocks 以组件块 id 为键、该块的状态对象为值（只含与初始值不同的键）。
	// 出现的块整块替换；值为 null 或空对象表示清除该块；没出现的块保持不变。
	Blocks map[string]json.RawMessage `json:"blocks" binding:"required,min=1,max=32" swaggertype:"object"`
}

// UpdateMessageRequest 更新消息内容请求。
type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}
