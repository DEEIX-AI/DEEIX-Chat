package model

import "time"

// Conversation 记录用户的对话会话元数据。
type Conversation struct {
	BaseModel
	UserID                uint       `gorm:"not null;index:idx_chat_conversations_user_id;comment:用户ID"`
	ProjectID             *uint      `gorm:"index:idx_chat_conversations_project_id;comment:项目分组ID"`
	PublicID              string     `gorm:"size:32;not null;default:'';index:idx_chat_conversations_public_id;comment:公开会话ID"`
	Title                 string     `gorm:"size:255;not null;default:'';comment:会话标题"`
	LabelsJSON            string     `gorm:"type:text;not null;default:'[]';comment:会话标签JSON"`
	Model                 string     `gorm:"size:128;not null;default:'';comment:模型名称"`
	Provider              string     `gorm:"size:32;not null;default:'';index:idx_chat_conversations_provider;comment:模型提供商"`
	SessionKey            string     `gorm:"size:128;not null;default:'';uniqueIndex:idx_chat_conversations_session_key;comment:会话上下文键"`
	IsStarred             bool       `gorm:"not null;default:false;index:idx_chat_conversations_is_starred;comment:是否星标"`
	StarredAt             *time.Time `gorm:"index:idx_chat_conversations_starred_at;comment:最近星标时间"`
	MessageCount          int        `gorm:"not null;default:0;comment:消息计数"`
	Status                string     `gorm:"size:32;not null;default:'';index:idx_chat_conversations_status;comment:会话状态"`
	ContextPolicy         string     `gorm:"type:text;not null;default:'';comment:上下文策略快照JSON"`
	LastCompactedAt       *time.Time `gorm:"comment:最近上下文压缩时间"`
	LastResponseID        string     `gorm:"size:128;not null;default:'';index:idx_chat_conversations_last_response_id;comment:最新响应ID"`
	LastPromptFingerprint string     `gorm:"size:64;not null;default:'';index:idx_chat_conversations_last_prompt_fingerprint;comment:最新上游状态指纹"`
}

// TableName 指定表名。
func (Conversation) TableName() string {
	return "chat_conversations"
}

// ConversationProject 存储用户会话项目分组。
type ConversationProject struct {
	BaseModel
	UserID       uint   `gorm:"not null;index:idx_chat_conversation_projects_user_id;comment:用户ID"`
	PublicID     string `gorm:"size:32;not null;default:'';uniqueIndex:idx_chat_conversation_projects_public_id;comment:公开项目ID"`
	Name         string `gorm:"size:80;not null;default:'';comment:项目名称"`
	Description  string `gorm:"size:255;not null;default:'';comment:项目描述"`
	SystemPrompt string `gorm:"type:text;not null;default:'';comment:项目级系统提示词"`
	Color        string `gorm:"size:32;not null;default:'';comment:项目颜色"`
	Icon         string `gorm:"size:32;not null;default:'';comment:项目图标"`
	SortOrder    int    `gorm:"not null;default:0;index:idx_chat_conversation_projects_sort_order;comment:展示顺序"`
	Status       string `gorm:"size:32;not null;default:'active';index:idx_chat_conversation_projects_status;comment:项目状态(active/archived)"`
}

// TableName 指定表名。
func (ConversationProject) TableName() string {
	return "chat_conversation_projects"
}

// ConversationShare 存储会话公开分享快照。
type ConversationShare struct {
	BaseModel
	ShareID               string     `gorm:"size:32;not null;default:'';uniqueIndex:idx_chat_conversation_shares_share_id;comment:公开分享ID"`
	ConversationID        uint       `gorm:"not null;index:idx_chat_conversation_shares_conversation_id;comment:原会话ID"`
	UserID                uint       `gorm:"not null;index:idx_chat_conversation_shares_user_id;comment:原会话所有者ID"`
	Status                string     `gorm:"size:32;not null;default:'active';index:idx_chat_conversation_shares_status;comment:分享状态(active/revoked/expired)"`
	TitleSnapshot         string     `gorm:"size:255;not null;default:'';comment:分享时标题快照"`
	ModelSnapshot         string     `gorm:"size:128;not null;default:'';comment:分享时平台模型快照"`
	MessageIDsJSON        string     `gorm:"type:text;not null;default:'[]';comment:分享时全部分支消息public_id列表JSON"`
	DefaultMessageIDsJSON string     `gorm:"column:default_message_ids_json;type:text;not null;default:'[]';comment:公开页默认分支消息public_id列表JSON"`
	RevokedAt             *time.Time `gorm:"index:idx_chat_conversation_shares_revoked_at;comment:撤销时间"`
	RegeneratedAt         *time.Time `gorm:"comment:重新生成时间"`
	LastAccessedAt        *time.Time `gorm:"index:idx_chat_conversation_shares_last_accessed_at;comment:最近公开访问时间"`
}

// TableName 指定表名。
func (ConversationShare) TableName() string {
	return "chat_conversation_shares"
}

// Message 存储会话内消息。
type Message struct {
	BaseModel
	ConversationID   uint   `gorm:"not null;index:idx_chat_messages_conversation_id;comment:会话ID"`
	UserID           uint   `gorm:"not null;index:idx_chat_messages_user_id;comment:用户ID"`
	PublicID         string `gorm:"size:32;not null;default:'';uniqueIndex:idx_chat_messages_public_id;comment:公开消息ID"`
	ParentMessageID  *uint  `gorm:"index:idx_chat_messages_parent_message_id;comment:父消息ID"`
	RunID            string `gorm:"size:64;not null;default:'';index:idx_chat_messages_run_id;comment:会话运行ID"`
	Role             string `gorm:"size:32;not null;default:'';index:idx_chat_messages_role;comment:消息角色(user/assistant/system/tool)"`
	ContentType      string `gorm:"size:32;not null;default:'';comment:消息内容类型"`
	Content          string `gorm:"type:text;not null;default:'';comment:消息内容"`
	BranchReason     string `gorm:"size:32;not null;default:'default';index:idx_chat_messages_branch_reason;comment:分支来源(default/retry/edit)"`
	SourceMessageID  *uint  `gorm:"index:idx_chat_messages_source_message_id;comment:来源消息ID(重试/编辑源)"`
	TokenUsage       int64  `gorm:"not null;default:0;comment:token总消耗"`
	InputTokens      int64  `gorm:"not null;default:0;comment:输入Token"`
	OutputTokens     int64  `gorm:"not null;default:0;comment:输出Token"`
	CacheReadTokens  int64  `gorm:"not null;default:0;comment:缓存读取Token"`
	CacheWriteTokens int64  `gorm:"not null;default:0;comment:缓存写入Token"`
	ReasoningTokens  int64  `gorm:"not null;default:0;comment:推理Token"`
	LatencyMS        int64  `gorm:"not null;default:0;comment:消息处理时长毫秒"`
	BilledCurrency   string `gorm:"size:16;not null;default:'USD';comment:消息计费币种"`
	BilledNanousd    int64  `gorm:"not null;default:0;comment:消息计费金额(纳美元)"`
	PricingSnapshot  string `gorm:"type:text;not null;default:'';comment:消息计费快照JSON"`
	Status           string `gorm:"size:32;not null;default:'';index:idx_chat_messages_status;comment:消息处理状态"`
	ErrorCode        string `gorm:"size:64;not null;default:'';comment:错误码"`
	ErrorMessage     string `gorm:"size:255;not null;default:'';comment:错误信息"`
	IsCompacted      bool   `gorm:"not null;default:false;index:idx_chat_messages_is_compacted;comment:是否已被压缩(压缩后不纳入祖先链)"`
	ParentPublicID   string `gorm:"-"`
	SourcePublicID   string `gorm:"-"`
	Attachments      string `gorm:"-"`
	MyFeedback       string `gorm:"-"`
	ThumbsUpCount    int64  `gorm:"-"`
	ThumbsDownCount  int64  `gorm:"-"`
}

// TableName 指定表名。
func (Message) TableName() string {
	return "chat_messages"
}

// ConversationMessageFeedback 存储用户对消息的点赞/点踩反馈。
type ConversationMessageFeedback struct {
	BaseModel
	UserID         uint   `gorm:"not null;default:0;uniqueIndex:idx_chat_feedback_user_message,priority:1;index:idx_chat_feedback_user_id;comment:反馈用户ID"`
	ConversationID uint   `gorm:"not null;default:0;index:idx_chat_feedback_conversation_id;comment:会话ID"`
	MessageID      uint   `gorm:"not null;default:0;uniqueIndex:idx_chat_feedback_user_message,priority:2;index:idx_chat_feedback_message_id;comment:消息ID"`
	Feedback       string `gorm:"size:16;not null;default:'';index:idx_chat_feedback_feedback;comment:反馈类型(up/down)"`
}

// TableName 指定表名。
func (ConversationMessageFeedback) TableName() string {
	return "chat_feedback"
}

// Attachment 存储多模态附件元信息。
type Attachment struct {
	BaseModel
	ConversationID uint      `gorm:"not null;index:idx_chat_attachments_conversation_id;comment:会话ID"`
	MessageID      uint      `gorm:"not null;index:idx_chat_attachments_message_id;comment:消息ID"`
	UserID         uint      `gorm:"not null;index:idx_chat_attachments_user_id;comment:用户ID"`
	FileID         string    `gorm:"size:64;not null;default:'';index:idx_chat_attachments_file_id;comment:文件对象ID"`
	Kind           string    `gorm:"size:32;not null;default:'';index:idx_chat_attachments_kind;comment:附件类型(file/image)"`
	FileName       string    `gorm:"size:255;not null;default:'';comment:文件名"`
	MimeType       string    `gorm:"size:128;not null;default:'';comment:媒体类型"`
	FileSize       int64     `gorm:"not null;default:0;comment:文件大小(Byte)"`
	SHA256         string    `gorm:"size:64;not null;default:'';index:idx_chat_attachments_sha256;comment:文件SHA256"`
	StoragePath    string    `gorm:"size:512;not null;default:'';comment:存储路径"`
	Status         string    `gorm:"size:32;not null;default:'active';index:idx_chat_attachments_status;comment:附件状态"`
	MetaJSON       string    `gorm:"type:text;not null;default:'';comment:额外元数据JSON"`
	UploadedAt     time.Time `gorm:"comment:上传时间"`
}

// TableName 指定表名。
func (Attachment) TableName() string {
	return "chat_attachments"
}

// FileObject 存储文件对象元信息。
type FileObject struct {
	BaseModel
	FileID                 string     `gorm:"size:64;not null;default:'';uniqueIndex:idx_file_objects_file_id;comment:文件对象ID"`
	UserID                 uint       `gorm:"not null;default:0;index:idx_file_objects_user_id;comment:用户ID"`
	Purpose                string     `gorm:"size:32;not null;default:'';comment:文件用途"`
	FileName               string     `gorm:"size:255;not null;default:'';comment:文件名"`
	MimeType               string     `gorm:"size:128;not null;default:'';comment:客户端声明媒体类型"`
	DetectedMIME           string     `gorm:"size:128;not null;default:'';index:idx_file_objects_detected_mime;comment:后端探测媒体类型"`
	FileCategory           string     `gorm:"size:32;not null;default:'unknown';index:idx_file_objects_file_category;comment:文件分类(image/pdf/word/excel/text/unknown)"`
	SizeBytes              int64      `gorm:"not null;default:0;comment:文件大小(Byte)"`
	SHA256                 string     `gorm:"size:64;not null;default:'';index:idx_file_objects_sha256;comment:文件SHA256"`
	StoragePath            string     `gorm:"size:512;not null;default:'';comment:存储路径"`
	Status                 string     `gorm:"size:32;not null;default:'active';index:idx_file_objects_status;comment:文件状态"`
	LastAccessedAt         *time.Time `gorm:"index:idx_file_objects_last_accessed_at;comment:最近使用时间"`
	ExpiresAt              *time.Time `gorm:"index:idx_file_objects_expires_at;comment:过期时间"`
	ProcessingStatus       string     `gorm:"size:32;not null;default:'uploaded';index:idx_file_objects_processing_status;comment:文件处理状态(uploaded/queued/extracting/extracted/embedding/ready/failed)"`
	ProcessingReady        bool       `gorm:"not null;default:false;index:idx_file_objects_processing_ready;comment:是否可用于对话"`
	ProcessingErrorCode    string     `gorm:"size:64;not null;default:'';comment:文件处理错误码"`
	ProcessingErrorMessage string     `gorm:"size:255;not null;default:'';comment:文件处理错误信息"`
	ExtractStatus          string     `gorm:"size:16;not null;default:'none';index:idx_file_objects_extract_status;comment:文本提取状态(none/processing/ready/failed)"`
	ExtractEngine          string     `gorm:"size:64;not null;default:'';comment:提取引擎"`
	ExtractStoragePath     string     `gorm:"size:512;not null;default:'';comment:提取文本存储路径"`
	ExtractChars           int        `gorm:"not null;default:0;comment:提取字符数"`
	ExtractPages           int        `gorm:"not null;default:0;comment:提取页数"`
	PreviewText            string     `gorm:"type:text;not null;default:'';comment:提取预览文本"`
	OCRUsed                bool       `gorm:"not null;default:false;comment:是否使用OCR"`
	RAGReady               bool       `gorm:"not null;default:false;comment:RAG是否就绪"`
	RAGReason              string     `gorm:"size:255;not null;default:'';comment:RAG处理说明"`
	EmbedStatus            string     `gorm:"size:16;not null;default:'none';index:idx_file_objects_embed_status;comment:向量嵌入状态(none/processing/ready/failed)"`
	EmbedError             string     `gorm:"type:text;not null;default:'';comment:嵌入失败原因"`
	PageCount              int        `gorm:"not null;default:0;comment:PDF页数"`
	ChunkCount             int        `gorm:"not null;default:0;comment:分片数量"`
	ExtractorVersion       string     `gorm:"size:32;not null;default:'';comment:提取器版本"`
	ExtractedAt            *time.Time `gorm:"comment:文本提取完成时间"`
	ProcessingPayloadJSON  string     `gorm:"type:text;not null;default:'';comment:文件处理扩展负载JSON"`
	ProcessingStartedAt    *time.Time `gorm:"comment:处理开始时间"`
	ProcessingCompletedAt  *time.Time `gorm:"comment:处理完成时间"`
	RagOptOut              bool       `gorm:"not null;default:false;comment:用户是否关闭此文件的RAG检索"`
}

// TableName 指定表名。
func (FileObject) TableName() string {
	return "file_objects"
}

// FileChunk 存储 RAG 分片及向量嵌入。
// embedding 列当前统一写入 vector(1536)。
// 若底层模型维度较小（如 all-MiniLM-L6-v2 的 384），写库前会零填充到统一维度。
// 通过 applyVectorBaseline 在 schema baseline 阶段用 raw SQL 创建。
type FileChunk struct {
	ID         uint      `gorm:"primaryKey;comment:主键ID"`
	FileObjID  uint      `gorm:"not null;index:idx_file_chunks_file_obj_id;comment:文件对象ID(外键)"`
	UserID     uint      `gorm:"not null;index:idx_file_chunks_user_id;comment:用户ID"`
	ChunkIndex int       `gorm:"not null;default:0;comment:分片序号"`
	PageNum    int       `gorm:"not null;default:0;comment:所在页码"`
	CharOffset int       `gorm:"not null;default:0;comment:字符偏移量"`
	Content    string    `gorm:"type:text;not null;comment:分片文本内容"`
	TokenCount int       `gorm:"not null;default:0;comment:估算token数"`
	CreatedAt  time.Time `gorm:"comment:创建时间"`
}

// TableName 指定表名。
func (FileChunk) TableName() string {
	return "file_chunks"
}

// UserStorageQuota 存储用户文件配额。
type UserStorageQuota struct {
	BaseModel
	UserID        uint  `gorm:"not null;default:0;uniqueIndex:idx_file_storage_quotas_user_id;comment:用户ID"`
	QuotaBytes    int64 `gorm:"not null;default:0;comment:总配额(Byte)，0表示不限制"`
	UsedBytes     int64 `gorm:"not null;default:0;comment:已用空间(Byte)"`
	ReservedBytes int64 `gorm:"not null;default:0;comment:预留空间(Byte)"`
}

// TableName 指定表名。
func (UserStorageQuota) TableName() string {
	return "file_storage_quotas"
}

// ConversationRun 存储每轮对话运行日志。
type ConversationRun struct {
	BaseModel
	RunID               string     `gorm:"size:64;not null;default:'';uniqueIndex:idx_chat_runs_run_id;comment:运行ID"`
	RequestID           string     `gorm:"size:64;not null;default:'';index:idx_chat_runs_request_id;comment:请求ID"`
	UserID              uint       `gorm:"not null;default:0;index:idx_chat_runs_user_id;comment:用户ID"`
	ConversationID      uint       `gorm:"not null;default:0;index:idx_chat_runs_conversation_id;comment:会话ID"`
	TaskType            string     `gorm:"size:32;not null;default:'chat';index:idx_chat_runs_task_type;comment:任务类型"`
	Endpoint            string     `gorm:"size:32;not null;default:'';index:idx_chat_runs_endpoint;comment:调用端点"`
	Provider            string     `gorm:"size:32;not null;default:'';index:idx_chat_runs_provider;comment:模型提供商"`
	ProviderProtocol    string     `gorm:"size:64;not null;default:'';index:idx_chat_runs_provider_protocol;comment:协议适配器快照"`
	UpstreamID          uint       `gorm:"not null;default:0;index:idx_chat_runs_upstream_id;comment:上游ID"`
	UpstreamModelID     uint       `gorm:"not null;default:0;index:idx_chat_runs_upstream_model_id;comment:上游真实模型ID"`
	UpstreamName        string     `gorm:"size:128;not null;default:'';comment:上游名称快照"`
	RequestedModelName  string     `gorm:"size:128;not null;default:'';index:idx_chat_runs_requested_model_name;comment:请求平台模型名"`
	PlatformModelName   string     `gorm:"size:128;not null;default:'';index:idx_chat_runs_platform_model_name;comment:路由命中平台模型名"`
	RoutedBindingCode   string     `gorm:"size:64;not null;default:'';index:idx_chat_runs_routed_binding_code;comment:实际路由上游模型绑定编码"`
	ModelVendor         string     `gorm:"size:64;not null;default:'';comment:平台模型厂商快照"`
	ModelIcon           string     `gorm:"size:64;not null;default:'';comment:平台模型图标快照"`
	UpstreamModelName   string     `gorm:"size:256;not null;default:'';comment:上游真实模型名称"`
	InputTokens         int64      `gorm:"not null;default:0;comment:输入Token"`
	OutputTokens        int64      `gorm:"not null;default:0;comment:输出Token"`
	CacheReadTokens     int64      `gorm:"not null;default:0;comment:缓存读取Token"`
	CacheWriteTokens    int64      `gorm:"not null;default:0;comment:缓存写入Token"`
	ReasoningTokens     int64      `gorm:"not null;default:0;comment:推理Token"`
	ToolCallsCount      int        `gorm:"not null;default:0;comment:工具调用次数"`
	FirstTokenLatencyMS int64      `gorm:"not null;default:0;comment:首Token时延毫秒"`
	TotalLatencyMS      int64      `gorm:"not null;default:0;comment:总时长毫秒"`
	Status              string     `gorm:"size:32;not null;default:'';index:idx_chat_runs_status;comment:运行状态"`
	ErrorCode           string     `gorm:"size:64;not null;default:'';comment:错误码"`
	ErrorMessage        string     `gorm:"size:255;not null;default:'';comment:错误信息"`
	StartedAt           time.Time  `gorm:"not null;comment:开始时间"`
	EndedAt             *time.Time `gorm:"comment:结束时间"`
}

// TableName 指定表名。
func (ConversationRun) TableName() string {
	return "chat_runs"
}

// ChatRunEvent 存储运行轨迹、事件流和工具调用明细。
type ChatRunEvent struct {
	BaseModel
	MessageID       uint       `gorm:"not null;default:0;index:idx_chat_run_events_message_id;comment:消息ID"`
	ConversationID  uint       `gorm:"not null;default:0;index:idx_chat_run_events_conversation_id;comment:会话ID"`
	UserID          uint       `gorm:"not null;default:0;index:idx_chat_run_events_user_id;comment:用户ID"`
	RunID           string     `gorm:"size:64;not null;default:'';index:idx_chat_run_events_run_id;uniqueIndex:uk_chat_run_events_run_scope_event,priority:1;comment:运行ID"`
	EventScope      string     `gorm:"size:32;not null;default:'';index:idx_chat_run_events_scope;uniqueIndex:uk_chat_run_events_run_scope_event,priority:2;comment:事件范围(trace_block/trace_event/tool_call)"`
	EventID         string     `gorm:"size:255;not null;default:'';uniqueIndex:uk_chat_run_events_run_scope_event,priority:3;comment:事件ID"`
	EventType       string     `gorm:"size:32;not null;default:'';index:idx_chat_run_events_type;comment:事件类型"`
	Phase           string     `gorm:"size:32;not null;default:'';index:idx_chat_run_events_phase;comment:阶段(process/tools/upstream_think)"`
	Stage           string     `gorm:"size:32;not null;default:'';index:idx_chat_run_events_stage;comment:链路阶段(process/think/tool/answer)"`
	RoundID         string     `gorm:"size:64;not null;default:'';index:idx_chat_run_events_round_id;comment:链路轮次ID"`
	ParentEventID   string     `gorm:"size:255;not null;default:'';index:idx_chat_run_events_parent_event_id;comment:父事件ID"`
	Status          string     `gorm:"size:32;not null;default:'';index:idx_chat_run_events_status;comment:事件状态(streaming/completed/error)"`
	Title           string     `gorm:"size:255;not null;default:'';comment:轨迹标题"`
	Summary         string     `gorm:"size:255;not null;default:'';comment:轨迹摘要"`
	ContentMarkdown string     `gorm:"type:text;not null;default:'';comment:轨迹Markdown内容"`
	PayloadJSON     string     `gorm:"type:text;not null;default:'';comment:轨迹负载JSON"`
	Seq             int        `gorm:"not null;default:0;index:idx_chat_run_events_seq;comment:事件顺序"`
	ToolCallID      string     `gorm:"size:255;not null;default:'';index:idx_chat_run_events_tool_call_id;comment:工具调用ID"`
	ToolName        string     `gorm:"size:128;not null;default:'';index:idx_chat_run_events_tool_name;comment:工具名称"`
	LatencyMS       int64      `gorm:"not null;default:0;comment:调用时长毫秒"`
	InputJSON       string     `gorm:"type:text;not null;default:'';comment:输入JSON"`
	OutputJSON      string     `gorm:"type:text;not null;default:'';comment:输出JSON"`
	ErrorJSON       string     `gorm:"type:text;not null;default:'';comment:错误JSON"`
	StartedAt       time.Time  `gorm:"not null;comment:开始时间"`
	EndedAt         *time.Time `gorm:"comment:结束时间"`
}

func (ChatRunEvent) TableName() string {
	return "chat_run_events"
}

// ChatContextRecord 存储上下文快照和本轮上下文证据。
type ChatContextRecord struct {
	BaseModel
	RecordType     string     `gorm:"size:32;not null;default:'';index:idx_chat_context_records_type;comment:记录类型(snapshot/artifact)"`
	ConversationID uint       `gorm:"not null;default:0;index:idx_chat_context_records_conversation_id;index:idx_chat_context_records_conversation_message,priority:1;index:idx_chat_context_records_conversation_kind,priority:1;comment:会话ID"`
	MessageID      uint       `gorm:"not null;default:0;index:idx_chat_context_records_message_id;index:idx_chat_context_records_conversation_message,priority:2;comment:触发消息ID"`
	UserID         uint       `gorm:"not null;default:0;index:idx_chat_context_records_user_id;comment:用户ID"`
	RunID          string     `gorm:"size:64;not null;default:'';index:idx_chat_context_records_run_id;comment:运行ID"`
	FromTurn       int        `gorm:"not null;default:0;comment:压缩快照起始轮次"`
	ToTurn         int        `gorm:"not null;default:0;comment:压缩快照结束轮次"`
	SourceTokens   int64      `gorm:"not null;default:0;comment:原始Token数量"`
	SummaryTokens  int64      `gorm:"not null;default:0;comment:摘要Token数量"`
	SummaryText    string     `gorm:"type:text;not null;default:'';comment:摘要文本"`
	Strategy       string     `gorm:"size:32;not null;default:'';comment:压缩策略"`
	Kind           string     `gorm:"size:32;not null;default:'';index:idx_chat_context_records_kind;index:idx_chat_context_records_conversation_kind,priority:2;comment:证据类型"`
	SourceType     string     `gorm:"size:32;not null;default:'';index:idx_chat_context_records_source_type;comment:来源类型"`
	SourceID       string     `gorm:"size:128;not null;default:'';index:idx_chat_context_records_source_id;comment:来源ID"`
	SourceTitle    string     `gorm:"size:255;not null;default:'';comment:来源标题"`
	Content        string     `gorm:"type:text;not null;default:'';comment:证据文本或摘要"`
	ContentHash    string     `gorm:"size:64;not null;default:'';index:idx_chat_context_records_content_hash;comment:证据内容Hash"`
	TokenEstimate  int64      `gorm:"not null;default:0;comment:估算Token数"`
	Score          float64    `gorm:"not null;default:0;comment:检索相关性分数"`
	MetadataJSON   string     `gorm:"type:text;not null;default:'';comment:证据元数据JSON"`
	ExpiresAt      *time.Time `gorm:"index:idx_chat_context_records_expires_at;comment:过期时间"`
}

func (ChatContextRecord) TableName() string {
	return "chat_context_records"
}

// MessageChunk 存储会话消息的 RAG 向量分片，用于历史对话语义检索。
// embedding 列（vector(1536)）通过 applyVectorBaseline 用 raw SQL 创建。
type MessageChunk struct {
	ID             uint      `gorm:"primaryKey;comment:主键ID"`
	ConversationID uint      `gorm:"not null;index:idx_chat_message_chunks_conversation_id;comment:会话ID"`
	MessageID      uint      `gorm:"not null;index:idx_chat_message_chunks_message_id;comment:消息ID"`
	UserID         uint      `gorm:"not null;index:idx_chat_message_chunks_user_id;comment:用户ID"`
	Role           string    `gorm:"size:32;not null;default:'';index:idx_chat_message_chunks_role;comment:消息角色(user/assistant)"`
	ChunkIndex     int       `gorm:"not null;default:0;comment:分片序号"`
	Content        string    `gorm:"type:text;not null;comment:分片文本"`
	TokenCount     int       `gorm:"not null;default:0;comment:估算Token数"`
	CreatedAt      time.Time `gorm:"comment:创建时间"`
}

// TableName 指定表名。
func (MessageChunk) TableName() string {
	return "chat_message_chunks"
}
