package conversation

import (
	"encoding/json"
	"strings"
	"time"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

// ---------- Conversation ----------

// ConversationResponse 对外会话响应 DTO。
type ConversationResponse struct {
	PublicID            string     `json:"publicID"`
	UserID              uint       `json:"userID"`
	ProjectID           string     `json:"projectID"`
	ProjectName         string     `json:"projectName"`
	Title               string     `json:"title"`
	LabelsJSON          string     `json:"labelsJSON"`
	Model               string     `json:"model"`
	Provider            string     `json:"provider"`
	SessionKey          string     `json:"sessionKey"`
	IsStarred           bool       `json:"isStarred"`
	StarredAt           *time.Time `json:"starredAt"`
	MessageCount        int        `json:"messageCount"`
	Status              string     `json:"status"`
	ContextPolicy       string     `json:"contextPolicyJSON"`
	LastCompactedAt     *time.Time `json:"lastCompactedAt"`
	LastResponseID      string     `json:"lastResponseID"`
	ShareStatus         string     `json:"shareStatus"`
	ShareID             string     `json:"shareID"`
	SharedAt            *time.Time `json:"sharedAt"`
	LastShareAccessedAt *time.Time `json:"lastShareAccessedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type ConversationExportResponse struct {
	Version                 int                                     `json:"version"`
	ExportScope             string                                  `json:"exportScope"`
	ExportedAt              time.Time                               `json:"exportedAt"`
	Conversation            ConversationResponse                    `json:"conversation"`
	Messages                []MessageResponse                       `json:"messages"`
	Runs                    []RunResponse                           `json:"runs"`
	TotalMessages           int64                                   `json:"totalMessages"`
	TotalRuns               int64                                   `json:"totalRuns"`
	DefaultMessagePublicIDs []string                                `json:"defaultMessagePublicIDs"`
	Compatibility           ConversationExportCompatibilityResponse `json:"compatibility"`
}

type ConversationExportCompatibilityResponse struct {
	Format string `json:"format"`
	Notes  string `json:"notes"`
}

func toConversationResponse(item *model.Conversation) ConversationResponse {
	labelsJSON := strings.TrimSpace(item.LabelsJSON)
	if labelsJSON == "" || labelsJSON == "null" {
		labelsJSON = "[]"
	}
	shareStatus := strings.TrimSpace(item.ShareStatus)
	if shareStatus == "" {
		shareStatus = "none"
	}
	return ConversationResponse{
		PublicID:            item.PublicID,
		UserID:              item.UserID,
		ProjectID:           item.ProjectPublicID,
		ProjectName:         item.ProjectName,
		Title:               item.Title,
		LabelsJSON:          labelsJSON,
		Model:               item.Model,
		Provider:            item.Provider,
		SessionKey:          item.SessionKey,
		IsStarred:           item.IsStarred,
		StarredAt:           item.StarredAt,
		MessageCount:        item.MessageCount,
		Status:              item.Status,
		ContextPolicy:       item.ContextPolicy,
		LastCompactedAt:     item.LastCompactedAt,
		LastResponseID:      item.LastResponseID,
		ShareStatus:         shareStatus,
		ShareID:             item.ShareID,
		SharedAt:            item.SharedAt,
		LastShareAccessedAt: item.LastShareAccessedAt,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func toConversationExportResponse(item *appconversation.ConversationExportResult) ConversationExportResponse {
	if item == nil {
		return ConversationExportResponse{}
	}
	runModels := make(map[string]model.Run, len(item.Runs))
	runs := make([]RunResponse, 0, len(item.Runs))
	for _, run := range item.Runs {
		if runID := strings.TrimSpace(run.RunID); runID != "" {
			runModels[runID] = run
		}
		runs = append(runs, toRunResponse(run))
	}

	fallbackModel := ""
	if item.Conversation != nil {
		fallbackModel = item.Conversation.Model
	}
	messages := make([]MessageResponse, 0, len(item.Messages))
	for _, message := range item.Messages {
		messages = append(messages, toMessageResponseWithRunAndFallback(message, runModels[strings.TrimSpace(message.RunID)], fallbackModel))
	}

	return ConversationExportResponse{
		Version:                 item.Version,
		ExportScope:             item.ExportScope,
		ExportedAt:              item.ExportedAt,
		Conversation:            toConversationResponse(item.Conversation),
		Messages:                messages,
		Runs:                    runs,
		TotalMessages:           item.TotalMessages,
		TotalRuns:               item.TotalRuns,
		DefaultMessagePublicIDs: item.DefaultMessagePublicIDs,
		Compatibility: ConversationExportCompatibilityResponse{
			Format: "deeix.conversation.export",
			Notes:  "Full backup export. Import compatibility is not guaranteed.",
		},
	}
}

// ConversationProjectResponse 对外会话项目响应 DTO。
type ConversationProjectResponse struct {
	PublicID     string    `json:"publicID"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"systemPrompt"`
	Color        string    `json:"color"`
	Icon         string    `json:"icon"`
	SortOrder    int       `json:"sortOrder"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func toConversationProjectResponse(item *model.ConversationProject) ConversationProjectResponse {
	if item == nil {
		return ConversationProjectResponse{}
	}
	return ConversationProjectResponse{
		PublicID:     item.PublicID,
		Name:         item.Name,
		Description:  item.Description,
		SystemPrompt: item.SystemPrompt,
		Color:        item.Color,
		Icon:         item.Icon,
		SortOrder:    item.SortOrder,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

// BatchSetConversationProjectResponse 批量设置会话项目归属响应 DTO。
type BatchSetConversationProjectResponse struct {
	Updated int64 `json:"updated"`
}

// ConversationShareResponse 会话分享响应 DTO。
type ConversationShareResponse struct {
	ShareID        string     `json:"shareID"`
	Status         string     `json:"status"`
	TitleSnapshot  string     `json:"titleSnapshot"`
	ModelSnapshot  string     `json:"modelSnapshot"`
	MessageCount   int        `json:"messageCount"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	RevokedAt      *time.Time `json:"revokedAt"`
	LastAccessedAt *time.Time `json:"lastAccessedAt"`
}

func toConversationShareResponse(item *appconversation.ConversationShareResult) ConversationShareResponse {
	if item == nil {
		return ConversationShareResponse{Status: "none"}
	}
	return ConversationShareResponse{
		ShareID:        item.ShareID,
		Status:         item.Status,
		TitleSnapshot:  item.TitleSnapshot,
		ModelSnapshot:  item.ModelSnapshot,
		MessageCount:   item.MessageCount,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		RevokedAt:      item.RevokedAt,
		LastAccessedAt: item.LastAccessedAt,
	}
}

// RevokeConversationSharesResponse 批量关闭会话分享响应 DTO。
type RevokeConversationSharesResponse struct {
	Revoked bool `json:"revoked"`
}

// PublicSharedMessageResponse 公开分享消息响应 DTO。
type PublicSharedMessageResponse struct {
	PublicID          string                       `json:"publicID"`
	ParentPublicID    string                       `json:"parentPublicID"`
	SourcePublicID    string                       `json:"sourcePublicID"`
	RunID             string                       `json:"runID"`
	Role              string                       `json:"role"`
	ContentType       string                       `json:"contentType"`
	Content           string                       `json:"content"`
	BranchReason      string                       `json:"branchReason"`
	TokenUsage        int64                        `json:"tokenUsage"`
	InputTokens       int64                        `json:"inputTokens"`
	OutputTokens      int64                        `json:"outputTokens"`
	CacheReadTokens   int64                        `json:"cacheReadTokens"`
	CacheWriteTokens  int64                        `json:"cacheWriteTokens"`
	ReasoningTokens   int64                        `json:"reasoningTokens"`
	LatencyMS         int64                        `json:"latencyMS"`
	Status            string                       `json:"status"`
	ErrorCode         string                       `json:"errorCode"`
	ErrorMessage      string                       `json:"errorMessage"`
	Attachments       string                       `json:"attachments"`
	PlatformModelName string                       `json:"platformModelName"`
	UpstreamModelName string                       `json:"upstreamModelName"`
	ModelVendor       string                       `json:"modelVendor"`
	ModelIcon         string                       `json:"modelIcon"`
	ProcessTrace      *MessageProcessTraceResponse `json:"processTrace,omitempty"`
	EditedAt          *time.Time                   `json:"editedAt"`
	CreatedAt         time.Time                    `json:"createdAt"`
	UpdatedAt         time.Time                    `json:"updatedAt"`
}

func toPublicSharedMessageResponse(
	item model.Message,
	runModel appconversation.PublicSharedRunModel,
	fallbackModel string,
) PublicSharedMessageResponse {
	platformModelName := strings.TrimSpace(runModel.PlatformModelName)
	if platformModelName == "" {
		platformModelName = strings.TrimSpace(fallbackModel)
	}
	return PublicSharedMessageResponse{
		PublicID:          item.PublicID,
		ParentPublicID:    item.ParentPublicID,
		SourcePublicID:    item.SourcePublicID,
		RunID:             item.RunID,
		Role:              item.Role,
		ContentType:       item.ContentType,
		Content:           item.Content,
		BranchReason:      item.BranchReason,
		TokenUsage:        item.TokenUsage,
		InputTokens:       item.InputTokens,
		OutputTokens:      item.OutputTokens,
		CacheReadTokens:   item.CacheReadTokens,
		CacheWriteTokens:  item.CacheWriteTokens,
		ReasoningTokens:   item.ReasoningTokens,
		LatencyMS:         item.LatencyMS,
		Status:            item.Status,
		ErrorCode:         item.ErrorCode,
		ErrorMessage:      item.ErrorMessage,
		Attachments:       item.Attachments,
		PlatformModelName: platformModelName,
		UpstreamModelName: runModel.UpstreamModelName,
		ModelVendor:       runModel.ModelVendor,
		ModelIcon:         runModel.ModelIcon,
		ProcessTrace:      toPublicMessageProcessTraceResponse(item.ProcessTrace),
		EditedAt:          item.EditedAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

// PublicSharedConversationResponse 公开分享会话响应 DTO。
type PublicSharedConversationResponse struct {
	ShareID                 string                        `json:"shareID"`
	Title                   string                        `json:"title"`
	Model                   string                        `json:"model"`
	CreatedAt               time.Time                     `json:"createdAt"`
	LastAccessedAt          *time.Time                    `json:"lastAccessedAt"`
	DefaultMessagePublicIDs []string                      `json:"defaultMessagePublicIDs"`
	Messages                []PublicSharedMessageResponse `json:"messages"`
}

func toPublicSharedConversationResponse(item *appconversation.PublicSharedConversationResult) PublicSharedConversationResponse {
	messages := make([]PublicSharedMessageResponse, 0, len(item.Messages))
	for _, message := range item.Messages {
		messages = append(messages, toPublicSharedMessageResponse(message, item.RunModels[message.RunID], item.Model))
	}
	return PublicSharedConversationResponse{
		ShareID:                 item.ShareID,
		Title:                   item.Title,
		Model:                   item.Model,
		CreatedAt:               item.CreatedAt,
		LastAccessedAt:          item.LastAccessedAt,
		DefaultMessagePublicIDs: item.DefaultMessageIDs,
		Messages:                messages,
	}
}

// ConversationDeleteResponse 删除会话响应 DTO。
type ConversationDeleteResponse struct {
	Deleted          bool                  `json:"deleted"`
	DeletedFileCount int                   `json:"deletedFileCount,omitempty"`
	Quota            *StorageQuotaResponse `json:"quota,omitempty"`
}

func toConversationDeleteResponse(result *appconversation.DeleteConversationResult) ConversationDeleteResponse {
	if result == nil {
		return ConversationDeleteResponse{Deleted: true}
	}
	response := ConversationDeleteResponse{
		Deleted:          result.Deleted,
		DeletedFileCount: result.DeletedFileCount,
	}
	if result.Quota != nil {
		quota := toStorageQuotaResponse(*result.Quota)
		response.Quota = &quota
	}
	return response
}

// ---------- File Object ----------

// FileObjectResponse 文件对象响应 DTO。
type FileObjectResponse struct {
	FileID                 string     `json:"fileID"`
	Purpose                string     `json:"purpose"`
	FileName               string     `json:"fileName"`
	MimeType               string     `json:"mimeType"`
	DetectedMIME           string     `json:"detectedMIME"`
	FileCategory           string     `json:"fileCategory"`
	SizeBytes              int64      `json:"sizeBytes"`
	SHA256                 string     `json:"sha256"`
	Status                 string     `json:"status"`
	ProcessingStatus       string     `json:"processingStatus"`
	ProcessingReady        bool       `json:"processingReady"`
	ProcessingErrorCode    string     `json:"processingErrorCode"`
	ProcessingErrorMessage string     `json:"processingErrorMessage"`
	ExtractStatus          string     `json:"extractStatus"`
	EmbedStatus            string     `json:"embedStatus"`
	EmbedError             string     `json:"embedError"`
	ChunkCount             int        `json:"chunkCount"`
	RagOptOut              bool       `json:"ragOptOut"`
	LastAccessedAt         *time.Time `json:"lastAccessedAt"`
	ExpiresAt              *time.Time `json:"expiresAt"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

func toFileObjectResponse(item *model.FileObject) FileObjectResponse {
	return FileObjectResponse{
		FileID:                 item.FileID,
		Purpose:                item.Purpose,
		FileName:               item.FileName,
		MimeType:               item.MimeType,
		DetectedMIME:           item.DetectedMIME,
		FileCategory:           item.FileCategory,
		SizeBytes:              item.SizeBytes,
		SHA256:                 item.SHA256,
		Status:                 item.Status,
		ProcessingStatus:       item.ProcessingStatus,
		ProcessingReady:        item.ProcessingReady,
		ProcessingErrorCode:    item.ProcessingErrorCode,
		ProcessingErrorMessage: appprocessing.HumanizeFileProcessingError(item.FileCategory, item.ProcessingErrorCode, item.ProcessingErrorMessage),
		ExtractStatus:          item.ExtractStatus,
		EmbedStatus:            item.EmbedStatus,
		EmbedError:             item.EmbedError,
		ChunkCount:             item.ChunkCount,
		RagOptOut:              item.RagOptOut,
		LastAccessedAt:         item.LastAccessedAt,
		ExpiresAt:              item.ExpiresAt,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
	}
}

// ---------- Storage Quota ----------

// StorageQuotaResponse 存储配额响应 DTO。
type StorageQuotaResponse struct {
	ID            uint      `json:"id"`
	UserID        uint      `json:"userID"`
	QuotaBytes    int64     `json:"quotaBytes"`
	UsedBytes     int64     `json:"usedBytes"`
	ReservedBytes int64     `json:"reservedBytes"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func toStorageQuotaResponse(q model.StorageQuota) StorageQuotaResponse {
	return StorageQuotaResponse{
		ID:            q.ID,
		UserID:        q.UserID,
		QuotaBytes:    q.QuotaBytes,
		UsedBytes:     q.UsedBytes,
		ReservedBytes: q.ReservedBytes,
		CreatedAt:     q.CreatedAt,
		UpdatedAt:     q.UpdatedAt,
	}
}

// ---------- File Upload / Delete ----------

// FileUploadResponse 上传文件响应 DTO。
type FileUploadResponse struct {
	File   FileObjectResponse   `json:"file"`
	Quota  StorageQuotaResponse `json:"quota"`
	Reused bool                 `json:"reused"`
}

// DeleteFileResponse 删除文件响应 DTO。
type DeleteFileResponse struct {
	Deleted bool                 `json:"deleted"`
	FileID  string               `json:"fileID"`
	Quota   StorageQuotaResponse `json:"quota"`
}

// FileListResponse 文件列表响应 DTO。
type FileListResponse struct {
	Total   int64                `json:"total"`
	Results []FileObjectResponse `json:"results"`
	Quota   StorageQuotaResponse `json:"quota"`
}

func toDeleteFileResponse(r *appupload.DeleteFileResult) DeleteFileResponse {
	return DeleteFileResponse{
		Deleted: r.Deleted,
		FileID:  r.FileID,
		Quota:   toStorageQuotaResponse(r.Quota),
	}
}

// ---------- Message ----------

// MessageTraceBlockResponse 消息轨迹块响应 DTO。
type MessageTraceBlockResponse struct {
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	ContentMarkdown string    `json:"contentMarkdown"`
	Status          string    `json:"status"`
	Stage           string    `json:"stage,omitempty"`
	RoundID         string    `json:"roundID,omitempty"`
	ParentEventID   string    `json:"parentEventID,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
	PayloadJSON     string    `json:"payloadJSON,omitempty"`
}

// MessageTraceEventResponse 消息轨迹事件响应 DTO。
type MessageTraceEventResponse struct {
	EventID         string     `json:"eventID"`
	EventType       string     `json:"eventType"`
	Phase           string     `json:"phase"`
	Stage           string     `json:"stage,omitempty"`
	RoundID         string     `json:"roundID,omitempty"`
	ParentEventID   string     `json:"parentEventID,omitempty"`
	Title           string     `json:"title"`
	Summary         string     `json:"summary"`
	ContentMarkdown string     `json:"contentMarkdown"`
	Status          string     `json:"status"`
	Seq             int        `json:"seq"`
	StartedAt       time.Time  `json:"startedAt"`
	EndedAt         *time.Time `json:"endedAt,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	PayloadJSON     string     `json:"payloadJSON,omitempty"`
}

// MessagePromptTraceBlockResponse 上下文规划块响应 DTO。
type MessagePromptTraceBlockResponse struct {
	Kind          string                             `json:"kind"`
	Title         string                             `json:"title"`
	TokenEstimate int64                              `json:"tokenEstimate"`
	Cacheable     bool                               `json:"cacheable"`
	SourceCount   int                                `json:"sourceCount"`
	SourceRefs    []MessagePromptTraceSourceResponse `json:"sourceRefs,omitempty"`
}

// MessagePromptTraceSourceResponse 上下文来源引用响应 DTO。
type MessagePromptTraceSourceResponse struct {
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceID"`
	Title      string `json:"title"`
	ArtifactID uint   `json:"artifactID,omitempty"`
}

// MessagePromptTraceResponse 上下文规划响应 DTO。
type MessagePromptTraceResponse struct {
	Mode                   string                            `json:"mode"`
	PromptFingerprint      string                            `json:"promptFingerprint"`
	StatefulUsed           bool                              `json:"statefulUsed"`
	StatefulDisabledReason string                            `json:"statefulDisabledReason"`
	TotalTokenEstimate     int64                             `json:"totalTokenEstimate"`
	SentTokenEstimate      int64                             `json:"sentTokenEstimate"`
	FullMessageCount       int                               `json:"fullMessageCount"`
	SentMessageCount       int                               `json:"sentMessageCount"`
	StatefulSavedMessages  int                               `json:"statefulSavedMessages"`
	StatefulSavedTokens    int64                             `json:"statefulSavedTokens"`
	Blocks                 []MessagePromptTraceBlockResponse `json:"blocks"`
}

// MessageProcessTraceResponse 消息处理轨迹响应 DTO。
type MessageProcessTraceResponse struct {
	Enabled       bool                        `json:"enabled"`
	Status        string                      `json:"status"`
	Process       *MessageTraceBlockResponse  `json:"process,omitempty"`
	Tools         *MessageTraceBlockResponse  `json:"tools,omitempty"`
	UpstreamThink *MessageTraceBlockResponse  `json:"upstreamThink,omitempty"`
	PromptTrace   *MessagePromptTraceResponse `json:"promptTrace,omitempty"`
	Events        []MessageTraceEventResponse `json:"events,omitempty"`
}

type MessageBillingCostResponse struct {
	BillingMode         string  `json:"billingMode"`
	BilledCurrency      string  `json:"billedCurrency"`
	BilledNanousd       int64   `json:"billedNanousd"`
	BilledUSD           float64 `json:"billedUSD"`
	PricingSnapshotJSON string  `json:"pricingSnapshotJSON"`
}

func toMessageProcessTraceResponse(trace *model.MessageProcessTrace) *MessageProcessTraceResponse {
	if trace == nil {
		return nil
	}
	return &MessageProcessTraceResponse{
		Enabled:       trace.Enabled,
		Status:        trace.Status,
		Process:       toTraceBlockResponse(trace.Process),
		Tools:         toTraceBlockResponse(trace.Tools),
		UpstreamThink: toTraceBlockResponse(trace.UpstreamThink),
		PromptTrace:   toPromptTraceResponse(trace.PromptTrace),
		Events:        toTraceEventResponses(trace.Events),
	}
}

func toPublicMessageProcessTraceResponse(trace *model.MessageProcessTrace) *MessageProcessTraceResponse {
	if trace == nil {
		return nil
	}
	return &MessageProcessTraceResponse{
		Enabled:       trace.Enabled,
		Status:        trace.Status,
		Process:       toPublicTraceBlockResponse(trace.Process),
		Tools:         toPublicTraceBlockResponse(trace.Tools),
		UpstreamThink: toPublicTraceBlockResponse(trace.UpstreamThink),
		PromptTrace:   toPromptTraceResponse(trace.PromptTrace),
		Events:        toPublicTraceEventResponses(trace.Events),
	}
}

func toPromptTraceResponse(trace *model.MessagePromptTrace) *MessagePromptTraceResponse {
	if trace == nil {
		return nil
	}
	blocks := make([]MessagePromptTraceBlockResponse, 0, len(trace.Blocks))
	for _, block := range trace.Blocks {
		sourceRefs := make([]MessagePromptTraceSourceResponse, 0, len(block.SourceRefs))
		for _, ref := range block.SourceRefs {
			sourceRefs = append(sourceRefs, MessagePromptTraceSourceResponse{
				SourceType: ref.SourceType,
				SourceID:   ref.SourceID,
				Title:      ref.Title,
				ArtifactID: ref.ArtifactID,
			})
		}
		blocks = append(blocks, MessagePromptTraceBlockResponse{
			Kind:          block.Kind,
			Title:         block.Title,
			TokenEstimate: block.TokenEstimate,
			Cacheable:     block.Cacheable,
			SourceCount:   block.SourceCount,
			SourceRefs:    sourceRefs,
		})
	}
	return &MessagePromptTraceResponse{
		Mode:                   trace.Mode,
		PromptFingerprint:      trace.PromptFingerprint,
		StatefulUsed:           trace.StatefulUsed,
		StatefulDisabledReason: trace.StatefulDisabledReason,
		TotalTokenEstimate:     trace.TotalTokenEstimate,
		SentTokenEstimate:      trace.SentTokenEstimate,
		FullMessageCount:       trace.FullMessageCount,
		SentMessageCount:       trace.SentMessageCount,
		StatefulSavedMessages:  trace.StatefulSavedMessages,
		StatefulSavedTokens:    trace.StatefulSavedTokens,
		Blocks:                 blocks,
	}
}

// ContextArtifactResponse 上下文证据详情响应 DTO。
type ContextArtifactResponse struct {
	ID            uint       `json:"id"`
	MessageID     uint       `json:"messageID"`
	RunID         string     `json:"runID"`
	Kind          string     `json:"kind"`
	SourceType    string     `json:"sourceType"`
	SourceID      string     `json:"sourceID"`
	SourceTitle   string     `json:"sourceTitle"`
	Content       string     `json:"content"`
	TokenEstimate int64      `json:"tokenEstimate"`
	Score         float64    `json:"score"`
	MetadataJSON  string     `json:"metadataJSON"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

func toContextArtifactResponse(item *model.ContextArtifact) ContextArtifactResponse {
	return ContextArtifactResponse{
		ID:            item.ID,
		MessageID:     item.MessageID,
		RunID:         item.RunID,
		Kind:          string(item.Kind),
		SourceType:    item.SourceType,
		SourceID:      item.SourceID,
		SourceTitle:   item.SourceTitle,
		Content:       item.Content,
		TokenEstimate: item.TokenEstimate,
		Score:         item.Score,
		MetadataJSON:  item.MetadataJSON,
		ExpiresAt:     item.ExpiresAt,
		CreatedAt:     item.CreatedAt,
	}
}

func toTraceEventResponses(events []model.MessageTraceEvent) []MessageTraceEventResponse {
	if len(events) == 0 {
		return nil
	}
	result := make([]MessageTraceEventResponse, 0, len(events))
	for _, event := range events {
		result = append(result, MessageTraceEventResponse{
			EventID:         event.EventID,
			EventType:       event.EventType,
			Phase:           event.Phase,
			Stage:           event.Stage,
			RoundID:         event.RoundID,
			ParentEventID:   event.ParentEventID,
			Title:           event.Title,
			Summary:         event.Summary,
			ContentMarkdown: event.ContentMarkdown,
			Status:          event.Status,
			Seq:             event.Seq,
			StartedAt:       event.StartedAt,
			EndedAt:         event.EndedAt,
			UpdatedAt:       event.UpdatedAt,
			PayloadJSON:     sanitizeTracePayloadJSON(event.PayloadJSON),
		})
	}
	return result
}

func toPublicTraceEventResponses(events []model.MessageTraceEvent) []MessageTraceEventResponse {
	if len(events) == 0 {
		return nil
	}
	result := make([]MessageTraceEventResponse, 0, len(events))
	for _, event := range events {
		result = append(result, MessageTraceEventResponse{
			EventID:         event.EventID,
			EventType:       event.EventType,
			Phase:           event.Phase,
			Stage:           event.Stage,
			RoundID:         event.RoundID,
			ParentEventID:   event.ParentEventID,
			Title:           event.Title,
			Summary:         event.Summary,
			ContentMarkdown: event.ContentMarkdown,
			Status:          event.Status,
			Seq:             event.Seq,
			StartedAt:       event.StartedAt,
			EndedAt:         event.EndedAt,
			UpdatedAt:       event.UpdatedAt,
			PayloadJSON:     sanitizePublicTracePayloadJSON(event.PayloadJSON),
		})
	}
	return result
}

// MessageResponse 消息响应 DTO。
type MessageResponse struct {
	ID                uint                         `json:"id"`
	ConversationID    uint                         `json:"conversationID"`
	UserID            uint                         `json:"userID"`
	PublicID          string                       `json:"publicID"`
	ParentMessageID   *uint                        `json:"parentMessageID"`
	RunID             string                       `json:"runID"`
	Role              string                       `json:"role"`
	ContentType       string                       `json:"contentType"`
	Content           string                       `json:"content"`
	BranchReason      string                       `json:"branchReason"`
	SourceMessageID   *uint                        `json:"sourceMessageID"`
	TokenUsage        int64                        `json:"tokenUsage"`
	InputTokens       int64                        `json:"inputTokens"`
	OutputTokens      int64                        `json:"outputTokens"`
	CacheReadTokens   int64                        `json:"cacheReadTokens"`
	CacheWriteTokens  int64                        `json:"cacheWriteTokens"`
	ReasoningTokens   int64                        `json:"reasoningTokens"`
	LatencyMS         int64                        `json:"latencyMS"`
	Status            string                       `json:"status"`
	ErrorCode         string                       `json:"errorCode"`
	ErrorMessage      string                       `json:"errorMessage"`
	Attachments       string                       `json:"attachments"`
	PlatformModelName string                       `json:"platformModelName"`
	UpstreamModelName string                       `json:"upstreamModelName"`
	ModelVendor       string                       `json:"modelVendor"`
	ModelIcon         string                       `json:"modelIcon"`
	ParentPublicID    string                       `json:"parentPublicID"`
	SourcePublicID    string                       `json:"sourcePublicID"`
	MyFeedback        string                       `json:"myFeedback"`
	ThumbsUpCount     int64                        `json:"thumbsUpCount"`
	ThumbsDownCount   int64                        `json:"thumbsDownCount"`
	BillingCost       *MessageBillingCostResponse  `json:"billingCost,omitempty"`
	ProcessTrace      *MessageProcessTraceResponse `json:"processTrace,omitempty"`
	EditedAt          *time.Time                   `json:"editedAt"`
	CreatedAt         time.Time                    `json:"createdAt"`
	UpdatedAt         time.Time                    `json:"updatedAt"`
}

func toTraceBlockResponse(b *model.MessageTraceBlock) *MessageTraceBlockResponse {
	if b == nil {
		return nil
	}
	return &MessageTraceBlockResponse{
		Title:           b.Title,
		Summary:         b.Summary,
		ContentMarkdown: b.ContentMarkdown,
		Status:          b.Status,
		Stage:           b.Stage,
		RoundID:         b.RoundID,
		ParentEventID:   b.ParentEventID,
		UpdatedAt:       b.UpdatedAt,
		PayloadJSON:     sanitizeTracePayloadJSON(b.PayloadJSON),
	}
}

func toPublicTraceBlockResponse(b *model.MessageTraceBlock) *MessageTraceBlockResponse {
	if b == nil {
		return nil
	}
	return &MessageTraceBlockResponse{
		Title:           b.Title,
		Summary:         b.Summary,
		ContentMarkdown: b.ContentMarkdown,
		Status:          b.Status,
		Stage:           b.Stage,
		RoundID:         b.RoundID,
		ParentEventID:   b.ParentEventID,
		UpdatedAt:       b.UpdatedAt,
		PayloadJSON:     sanitizePublicTracePayloadJSON(b.PayloadJSON),
	}
}

func sanitizeTracePayloadJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return value
	}
	deleteUpstreamNameFields(payload, "")
	if len(payload) == 0 {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func sanitizePublicTracePayloadJSON(raw string) string {
	value := sanitizeTracePayloadJSON(raw)
	if value == "" {
		return ""
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return ""
	}
	deletePublicSensitiveTraceFields(payload)
	if len(payload) == 0 {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func deleteUpstreamNameFields(payload map[string]interface{}, parentKey string) {
	for key, value := range payload {
		if isUpstreamNameField(key, parentKey) {
			delete(payload, key)
			continue
		}
		switch child := value.(type) {
		case map[string]interface{}:
			deleteUpstreamNameFields(child, key)
		case []interface{}:
			for _, item := range child {
				if itemMap, ok := item.(map[string]interface{}); ok {
					deleteUpstreamNameFields(itemMap, key)
				}
			}
		}
	}
}

func deletePublicSensitiveTraceFields(payload map[string]interface{}) {
	for key, value := range payload {
		if isPublicSensitiveTraceField(key) {
			delete(payload, key)
			continue
		}
		switch child := value.(type) {
		case map[string]interface{}:
			deletePublicSensitiveTraceFields(child)
		case []interface{}:
			for _, item := range child {
				if itemMap, ok := item.(map[string]interface{}); ok {
					deletePublicSensitiveTraceFields(itemMap)
				}
			}
		}
	}
}

func isPublicSensitiveTraceField(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
	switch normalized {
	case "upstreamdebug", "authorization", "proxyauthorization", "cookie", "setcookie":
		return true
	default:
		return strings.Contains(normalized, "apikey") ||
			strings.Contains(normalized, "secretkey") ||
			strings.Contains(normalized, "accesskey")
	}
}

func isUpstreamNameField(key string, parentKey string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	if normalized == "upstreamname" {
		return true
	}
	if strings.ToLower(strings.TrimSpace(parentKey)) == "upstream" && (normalized == "name" || normalized == "displayname") {
		return true
	}
	return false
}

func messageBillingMode(snapshotJSON string) string {
	snapshot := map[string]interface{}{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(snapshotJSON)), &snapshot); err != nil {
		return ""
	}
	mode, _ := snapshot["billing_mode"].(string)
	return strings.TrimSpace(mode)
}

func messageNanousdToUSD(value int64) float64 {
	return float64(value) / 1_000_000_000
}

func toMessageBillingCostResponse(m model.Message) *MessageBillingCostResponse {
	snapshotJSON := strings.TrimSpace(m.PricingSnapshot)
	if snapshotJSON == "" {
		return nil
	}
	billingMode := messageBillingMode(snapshotJSON)
	if billingMode == "self" {
		return nil
	}
	currency := strings.TrimSpace(m.BilledCurrency)
	if currency == "" {
		currency = "USD"
	}
	return &MessageBillingCostResponse{
		BillingMode:         billingMode,
		BilledCurrency:      currency,
		BilledNanousd:       m.BilledNanousd,
		BilledUSD:           messageNanousdToUSD(m.BilledNanousd),
		PricingSnapshotJSON: snapshotJSON,
	}
}

func toMessageResponse(m model.Message) MessageResponse {
	return toMessageResponseWithRun(m, model.Run{})
}

// toMessageResponseWithRun 将消息和同 run 的模型快照合并成前端展示 DTO。
func toMessageResponseWithRun(m model.Message, run model.Run) MessageResponse {
	return toMessageResponseWithRunAndFallback(m, run, "")
}

func toMessageResponseWithRunAndFallback(m model.Message, run model.Run, fallbackModel string) MessageResponse {
	platformModelName := strings.TrimSpace(run.PlatformModelName)
	if platformModelName == "" {
		platformModelName = strings.TrimSpace(run.RequestedModelName)
	}
	if platformModelName == "" {
		platformModelName = strings.TrimSpace(fallbackModel)
	}
	return MessageResponse{
		ID:                m.ID,
		ConversationID:    m.ConversationID,
		UserID:            m.UserID,
		PublicID:          m.PublicID,
		ParentMessageID:   m.ParentMessageID,
		RunID:             m.RunID,
		Role:              m.Role,
		ContentType:       m.ContentType,
		Content:           m.Content,
		BranchReason:      m.BranchReason,
		SourceMessageID:   m.SourceMessageID,
		TokenUsage:        m.TokenUsage,
		InputTokens:       m.InputTokens,
		OutputTokens:      m.OutputTokens,
		CacheReadTokens:   m.CacheReadTokens,
		CacheWriteTokens:  m.CacheWriteTokens,
		ReasoningTokens:   m.ReasoningTokens,
		LatencyMS:         m.LatencyMS,
		Status:            m.Status,
		ErrorCode:         m.ErrorCode,
		ErrorMessage:      m.ErrorMessage,
		Attachments:       m.Attachments,
		PlatformModelName: platformModelName,
		UpstreamModelName: strings.TrimSpace(run.UpstreamModelName),
		ModelVendor:       strings.TrimSpace(run.ModelVendor),
		ModelIcon:         strings.TrimSpace(run.ModelIcon),
		ParentPublicID:    m.ParentPublicID,
		SourcePublicID:    m.SourcePublicID,
		MyFeedback:        m.MyFeedback,
		ThumbsUpCount:     m.ThumbsUpCount,
		ThumbsDownCount:   m.ThumbsDownCount,
		BillingCost:       toMessageBillingCostResponse(m),
		ProcessTrace:      toMessageProcessTraceResponse(m.ProcessTrace),
		EditedAt:          m.EditedAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

// ---------- Send Message ----------

// SendMessageResponse 发送消息响应 DTO。
type SendMessageResponse struct {
	UserMessage      MessageResponse `json:"userMessage"`
	AssistantMessage MessageResponse `json:"assistantMessage"`
}

type CancelMessageGenerationResponse struct {
	Canceled bool `json:"canceled"`
}

func toSendMessageResponse(r *appconversation.SendMessageResult) SendMessageResponse {
	run := model.Run{
		PlatformModelName: r.PlatformModelName,
		UpstreamModelName: r.UpstreamModelName,
	}
	return SendMessageResponse{
		UserMessage:      toMessageResponseWithRun(r.UserMessage, run),
		AssistantMessage: toMessageResponseWithRun(r.AssistantMessage, run),
	}
}

// ---------- Message Feedback ----------

// MessageFeedbackResponse 消息反馈响应 DTO。
type MessageFeedbackResponse struct {
	MessageID       uint   `json:"messageID"`
	MessagePublicID string `json:"messagePublicID"`
	MyFeedback      string `json:"myFeedback"`
	ThumbsUpCount   int64  `json:"thumbsUpCount"`
	ThumbsDownCount int64  `json:"thumbsDownCount"`
}

func toMessageFeedbackResponse(r *appconversation.MessageFeedbackResult) MessageFeedbackResponse {
	return MessageFeedbackResponse{
		MessageID:       r.MessageID,
		MessagePublicID: r.MessagePublicID,
		MyFeedback:      r.MyFeedback,
		ThumbsUpCount:   r.ThumbsUpCount,
		ThumbsDownCount: r.ThumbsDownCount,
	}
}

// ---------- Conversation Run ----------

// RunResponse 对话运行日志响应 DTO。
type RunResponse struct {
	ID                  uint       `json:"id"`
	RunID               string     `json:"runID"`
	RequestID           string     `json:"requestID"`
	UserID              uint       `json:"userID"`
	ConversationID      uint       `json:"conversationID"`
	TaskType            string     `json:"taskType"`
	Endpoint            string     `json:"endpoint"`
	Provider            string     `json:"provider"`
	ProviderProtocol    string     `json:"providerProtocol"`
	UpstreamID          uint       `json:"upstreamID"`
	UpstreamModelID     uint       `json:"upstreamModelID"`
	RequestedModelName  string     `json:"requestedModelName"`
	PlatformModelName   string     `json:"platformModelName"`
	RoutedBindingCode   string     `json:"routedBindingCode"`
	ModelVendor         string     `json:"modelVendor"`
	ModelIcon           string     `json:"modelIcon"`
	UpstreamModelName   string     `json:"upstreamModelName"`
	InputTokens         int64      `json:"inputTokens"`
	OutputTokens        int64      `json:"outputTokens"`
	CacheReadTokens     int64      `json:"cacheReadTokens"`
	CacheWriteTokens    int64      `json:"cacheWriteTokens"`
	ReasoningTokens     int64      `json:"reasoningTokens"`
	ToolCallsCount      int        `json:"toolCallsCount"`
	FirstTokenLatencyMS int64      `json:"firstTokenLatencyMS"`
	TotalLatencyMS      int64      `json:"totalLatencyMS"`
	Status              string     `json:"status"`
	ErrorCode           string     `json:"errorCode"`
	ErrorMessage        string     `json:"errorMessage"`
	StartedAt           time.Time  `json:"startedAt"`
	EndedAt             *time.Time `json:"endedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

func toRunResponse(r model.Run) RunResponse {
	return RunResponse{
		ID:                  r.ID,
		RunID:               r.RunID,
		RequestID:           r.RequestID,
		UserID:              r.UserID,
		ConversationID:      r.ConversationID,
		TaskType:            r.TaskType,
		Endpoint:            r.Endpoint,
		Provider:            r.Provider,
		ProviderProtocol:    r.ProviderProtocol,
		UpstreamID:          r.UpstreamID,
		UpstreamModelID:     r.UpstreamModelID,
		RequestedModelName:  r.RequestedModelName,
		PlatformModelName:   r.PlatformModelName,
		RoutedBindingCode:   r.RoutedBindingCode,
		ModelVendor:         r.ModelVendor,
		ModelIcon:           r.ModelIcon,
		UpstreamModelName:   r.UpstreamModelName,
		InputTokens:         r.InputTokens,
		OutputTokens:        r.OutputTokens,
		CacheReadTokens:     r.CacheReadTokens,
		CacheWriteTokens:    r.CacheWriteTokens,
		ReasoningTokens:     r.ReasoningTokens,
		ToolCallsCount:      r.ToolCallsCount,
		FirstTokenLatencyMS: r.FirstTokenLatencyMS,
		TotalLatencyMS:      r.TotalLatencyMS,
		Status:              r.Status,
		ErrorCode:           r.ErrorCode,
		ErrorMessage:        r.ErrorMessage,
		StartedAt:           r.StartedAt,
		EndedAt:             r.EndedAt,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}
}

// ---------- File Processing Status ----------

// FileProcessingStatusResponse 文件处理状态响应 DTO。
type FileProcessingStatusResponse struct {
	FileID           string     `json:"fileID"`
	DetectedMIME     string     `json:"detectedMIME"`
	FileCategory     string     `json:"fileCategory"`
	ProcessingStatus string     `json:"processingStatus"`
	ProcessingReady  bool       `json:"processingReady"`
	ExtractStatus    string     `json:"extractStatus"`
	EmbedStatus      string     `json:"embedStatus"`
	PreviewText      string     `json:"previewText"`
	OCRUsed          bool       `json:"ocrUsed"`
	RAGReady         bool       `json:"ragReady"`
	RAGReason        string     `json:"ragReason"`
	ErrorCode        string     `json:"errorCode"`
	ErrorMessage     string     `json:"errorMessage"`
	ExtractChars     int        `json:"extractChars"`
	ExtractPages     int        `json:"extractPages"`
	StartedAt        *time.Time `json:"startedAt"`
	CompletedAt      *time.Time `json:"completedAt"`
}

func toFileProcessingStatusResponse(d *appprocessing.FileProcessingStatusDTO) FileProcessingStatusResponse {
	return FileProcessingStatusResponse{
		FileID:           d.FileID,
		DetectedMIME:     d.DetectedMIME,
		FileCategory:     d.FileCategory,
		ProcessingStatus: d.ProcessingStatus,
		ProcessingReady:  d.ProcessingReady,
		ExtractStatus:    d.ExtractStatus,
		EmbedStatus:      d.EmbedStatus,
		PreviewText:      d.PreviewText,
		OCRUsed:          d.OCRUsed,
		RAGReady:         d.RAGReady,
		RAGReason:        d.RAGReason,
		ErrorCode:        d.ErrorCode,
		ErrorMessage:     appprocessing.HumanizeFileProcessingError(d.FileCategory, d.ErrorCode, d.ErrorMessage),
		ExtractChars:     d.ExtractChars,
		ExtractPages:     d.ExtractPages,
		StartedAt:        d.StartedAt,
		CompletedAt:      d.CompletedAt,
	}
}

// FileExtractResponse 文件提取文本响应 DTO。
type FileExtractResponse struct {
	FileID       string `json:"fileID"`
	ExtractText  string `json:"extractText"`
	PreviewText  string `json:"previewText"`
	ExtractChars int    `json:"extractChars"`
	ExtractPages int    `json:"extractPages"`
	OCRUsed      bool   `json:"ocrUsed"`
}

func toFileExtractResponse(d *appconversation.FileExtractResult) FileExtractResponse {
	return FileExtractResponse{
		FileID:       d.FileID,
		ExtractText:  d.ExtractText,
		PreviewText:  d.PreviewText,
		ExtractChars: d.ExtractChars,
		ExtractPages: d.ExtractPages,
		OCRUsed:      d.OCRUsed,
	}
}

// ---------- Chat File Policy ----------

// ChatFilePolicyResponse 聊天文件策略响应 DTO。
type ChatFilePolicyResponse struct {
	MaxMessageFiles        int      `json:"maxMessageFiles"`
	MaxUploadFileBytes     int64    `json:"maxUploadFileBytes"`
	AllowedMIMETypes       []string `json:"allowedMIMETypes"`
	ImageMaxBytes          int64    `json:"imageMaxBytes"`
	DocMaxBytes            int64    `json:"docMaxBytes"`
	EffectiveImageMaxBytes int64    `json:"effectiveImageMaxBytes"`
	EffectiveDocMaxBytes   int64    `json:"effectiveDocMaxBytes"`
	FullContextMaxBytes    int64    `json:"fullContextMaxBytes"`
	FullContextMaxTokens   int      `json:"fullContextMaxTokens"`
	FullContextPDFMaxPages int      `json:"fullContextPDFMaxPages"`
	RAGAvailable           bool     `json:"ragAvailable"`
	RAGAvailabilityReason  string   `json:"ragAvailabilityReason"`
	CapabilityMode         string   `json:"capabilityMode"`
	FileMode               string   `json:"fileMode"`
}

func toChatFilePolicyResponse(d *appconversation.ChatFilePolicyDTO) ChatFilePolicyResponse {
	return ChatFilePolicyResponse{
		MaxMessageFiles:        d.MaxMessageFiles,
		MaxUploadFileBytes:     d.MaxUploadFileBytes,
		AllowedMIMETypes:       d.AllowedMIMETypes,
		ImageMaxBytes:          d.ImageMaxBytes,
		DocMaxBytes:            d.DocMaxBytes,
		EffectiveImageMaxBytes: d.EffectiveImageMaxBytes,
		EffectiveDocMaxBytes:   d.EffectiveDocMaxBytes,
		FullContextMaxBytes:    d.FullContextMaxBytes,
		FullContextMaxTokens:   d.FullContextMaxTokens,
		FullContextPDFMaxPages: d.FullContextPDFMaxPages,
		RAGAvailable:           d.RAGAvailable,
		RAGAvailabilityReason:  d.RAGAvailabilityReason,
		CapabilityMode:         d.CapabilityMode,
		FileMode:               d.FileMode,
	}
}

// ---------- Swagger 文档类型 ----------

// UploadFileResponseDoc 上传文件响应文档。
type UploadFileResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     FileUploadResponse `json:"data"`
}

// FileListResponseDoc 文件分页响应文档。
type FileListResponseDoc struct {
	ErrorMsg string           `json:"errorMsg"`
	Data     FileListResponse `json:"data"`
}

// DeleteFileResponseDoc 删除文件响应文档。
type DeleteFileResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     DeleteFileResponse `json:"data"`
}

// FileUpdateResponseDoc 文件更新响应文档。
type FileUpdateResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     FileObjectResponse `json:"data"`
}

// ConversationCreateResponseDoc 创建会话响应文档。
type ConversationCreateResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     ConversationResponse `json:"data"`
}

// ConversationExportResponseDoc 会话导出响应文档。
type ConversationExportResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     ConversationExportResponse `json:"data"`
}

// ConversationListResponseDoc 会话分页响应文档。
type ConversationListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                  `json:"total"`
		Results []ConversationResponse `json:"results"`
	} `json:"data"`
}

// ConversationProjectResponseDoc 会话项目响应文档。
type ConversationProjectResponseDoc struct {
	ErrorMsg string                      `json:"errorMsg"`
	Data     ConversationProjectResponse `json:"data"`
}

// ConversationProjectListResponseDoc 会话项目列表响应文档。
type ConversationProjectListResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     []ConversationProjectResponse `json:"data"`
}

// BatchSetConversationProjectResponseDoc 批量设置会话项目响应文档。
type BatchSetConversationProjectResponseDoc struct {
	ErrorMsg string                              `json:"errorMsg"`
	Data     BatchSetConversationProjectResponse `json:"data"`
}

// MessageListResponseDoc 消息分页响应文档。
type MessageListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64             `json:"total"`
		Results []MessageResponse `json:"results"`
	} `json:"data"`
}

// SendMessageResponseDoc 发送消息响应文档。
type SendMessageResponseDoc struct {
	ErrorMsg string              `json:"errorMsg"`
	Data     SendMessageResponse `json:"data"`
}

// MessageResponseDoc 消息响应文档。
type MessageResponseDoc struct {
	ErrorMsg string          `json:"errorMsg"`
	Data     MessageResponse `json:"data"`
}

// MessageFeedbackResponseDoc 设置消息反馈响应文档。
type MessageFeedbackResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     MessageFeedbackResponse `json:"data"`
}

// ConversationRunListResponseDoc 运行日志分页响应文档。
type ConversationRunListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64         `json:"total"`
		Results []RunResponse `json:"results"`
	} `json:"data"`
}

// ContextArtifactResponseDoc 上下文证据详情响应文档。
type ContextArtifactResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     ContextArtifactResponse `json:"data"`
}

// ConversationUpdateResponseDoc 会话更新响应文档。
type ConversationUpdateResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     ConversationResponse `json:"data"`
}

// ConversationDeleteResponseDoc 删除会话响应文档。
type ConversationDeleteResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     ConversationDeleteResponse `json:"data"`
}

// ConversationShareResponseDoc 会话分享响应文档。
type ConversationShareResponseDoc struct {
	ErrorMsg string                    `json:"errorMsg"`
	Data     ConversationShareResponse `json:"data"`
}

// RevokeConversationSharesResponseDoc 批量关闭会话分享响应文档。
type RevokeConversationSharesResponseDoc struct {
	ErrorMsg string                           `json:"errorMsg"`
	Data     RevokeConversationSharesResponse `json:"data"`
}

// PublicSharedConversationResponseDoc 公开分享会话响应文档。
type PublicSharedConversationResponseDoc struct {
	ErrorMsg string                           `json:"errorMsg"`
	Data     PublicSharedConversationResponse `json:"data"`
}

// ErrorDoc 错误响应文档。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}
