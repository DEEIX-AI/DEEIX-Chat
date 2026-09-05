package knowledgebase

import (
	"time"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
)

// KnowledgeBaseResponse 表示知识库响应。
type KnowledgeBaseResponse struct {
	PublicID            string    `json:"publicID"`
	Scope               string    `json:"scope" enums:"builtin,user"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	Enabled             bool      `json:"enabled"`
	SortOrder           int       `json:"sortOrder"`
	Revision            uint64    `json:"revision"`
	FileCount           int64     `json:"fileCount"`
	ReadyFileCount      int64     `json:"readyFileCount"`
	ProcessingFileCount int64     `json:"processingFileCount"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// KnowledgeBaseFileResponse 表示知识库文件摘要。
type KnowledgeBaseFileResponse struct {
	FileID              string    `json:"fileID"`
	FileName            string    `json:"fileName"`
	MimeType            string    `json:"mimeType"`
	DetectedMIME        string    `json:"detectedMIME"`
	FileCategory        string    `json:"fileCategory"`
	SizeBytes           int64     `json:"sizeBytes"`
	ProcessingStatus    string    `json:"processingStatus"`
	Processing          bool      `json:"processing"`
	ProcessingReady     bool      `json:"processingReady"`
	ExtractStatus       string    `json:"extractStatus"`
	EmbedStatus         string    `json:"embedStatus"`
	EmbedError          string    `json:"embedError"`
	ChunkCount          int       `json:"chunkCount"`
	RAGOptOut           bool      `json:"ragOptOut"`
	CanVectorize        bool      `json:"canVectorize"`
	VectorizationReason string    `json:"vectorizationReason"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// WriteMyKnowledgeBaseRequest 表示创建个人知识库请求。
type WriteMyKnowledgeBaseRequest struct {
	Name        string `json:"name" binding:"required,max=80"`
	Description string `json:"description,omitempty" binding:"max=255"`
}

// PatchMyKnowledgeBaseRequest 表示更新个人知识库请求。
type PatchMyKnowledgeBaseRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,max=80"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=255"`
}

// WriteKnowledgeBaseRequest 表示创建内置知识库请求。
type WriteKnowledgeBaseRequest struct {
	Name        string `json:"name" binding:"required,max=80"`
	Description string `json:"description,omitempty" binding:"max=255"`
	Enabled     *bool  `json:"enabled,omitempty"`
	SortOrder   int    `json:"sortOrder,omitempty"`
}

// PatchKnowledgeBaseRequest 表示更新内置知识库请求。
type PatchKnowledgeBaseRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,max=80"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=255"`
	Enabled     *bool   `json:"enabled,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`
}

// AddKnowledgeBaseFilesRequest 表示批量添加文件请求。
type AddKnowledgeBaseFilesRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// GetKnowledgeBaseFileProcessingStatusesRequest 表示批量文件处理状态请求。
type GetKnowledgeBaseFileProcessingStatusesRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// GetKnowledgeBaseFileProcessingSnapshotRequest 表示知识库处理快照请求。
type GetKnowledgeBaseFileProcessingSnapshotRequest struct {
	FileIDs []string `json:"fileIDs" binding:"max=100,dive,required,max=64"`
}

// SubmitPlatformFileEmbeddingsRequest 表示批量提交平台资料向量化请求。
type SubmitPlatformFileEmbeddingsRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// KnowledgeBaseFileEmbeddingSkipResponse 表示未提交向量化的平台资料及原因。
type KnowledgeBaseFileEmbeddingSkipResponse struct {
	FileID string `json:"fileID"`
	Reason string `json:"reason"`
}

// KnowledgeBaseFileEmbeddingSubmissionResponse 表示平台资料向量化提交结果。
type KnowledgeBaseFileEmbeddingSubmissionResponse struct {
	SubmittedFileIDs []string                                 `json:"submittedFileIDs"`
	Skipped          []KnowledgeBaseFileEmbeddingSkipResponse `json:"skipped"`
}

// KnowledgeBaseFileProcessingStatusResponse 表示知识库文件处理状态。
type KnowledgeBaseFileProcessingStatusResponse struct {
	FileID              string    `json:"fileID"`
	DetectedMIME        string    `json:"detectedMIME"`
	FileCategory        string    `json:"fileCategory"`
	ProcessingStatus    string    `json:"processingStatus"`
	Processing          bool      `json:"processing"`
	ProcessingReady     bool      `json:"processingReady"`
	ExtractStatus       string    `json:"extractStatus"`
	EmbedStatus         string    `json:"embedStatus"`
	EmbedError          string    `json:"embedError"`
	ChunkCount          int       `json:"chunkCount"`
	RAGOptOut           bool      `json:"ragOptOut"`
	CanVectorize        bool      `json:"canVectorize"`
	VectorizationReason string    `json:"vectorizationReason"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// KnowledgeBaseFileProcessingSnapshotResponse 表示知识库及文件处理状态快照。
type KnowledgeBaseFileProcessingSnapshotResponse struct {
	KnowledgeBase KnowledgeBaseResponse                       `json:"knowledgeBase"`
	Statuses      []KnowledgeBaseFileProcessingStatusResponse `json:"statuses"`
}

// KnowledgeBaseDataResponse 包裹单条知识库响应。
type KnowledgeBaseDataResponse struct {
	KnowledgeBase KnowledgeBaseResponse `json:"knowledgeBase"`
}

// KnowledgeBaseDeleteDataResponse 表示知识库删除响应。
type KnowledgeBaseDeleteDataResponse struct {
	Deleted          bool `json:"deleted"`
	DeletedFileCount int  `json:"deletedFileCount,omitempty"`
}

// KnowledgeBaseFileMutationDataResponse 表示知识库文件关联变更响应。
type KnowledgeBaseFileMutationDataResponse struct {
	Updated bool `json:"updated"`
}

// KnowledgeBaseFileDataResponse 包裹单个平台知识库文件。
type KnowledgeBaseFileDataResponse struct {
	File KnowledgeBaseFileResponse `json:"file"`
}

// PlatformFileDeleteDataResponse 表示平台资料删除响应。
type PlatformFileDeleteDataResponse struct {
	Deleted bool `json:"deleted"`
}

// KnowledgeBasePageResponseDoc 用于 Swagger 展示知识库分页响应。
type KnowledgeBasePageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                   `json:"total"`
		Results []KnowledgeBaseResponse `json:"results"`
	} `json:"data"`
}

// KnowledgeBaseFilePageResponseDoc 用于 Swagger 展示知识库文件分页响应。
type KnowledgeBaseFilePageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                       `json:"total"`
		Results []KnowledgeBaseFileResponse `json:"results"`
	} `json:"data"`
}

// KnowledgeBaseResponseDoc 用于 Swagger 展示单条知识库响应。
type KnowledgeBaseResponseDoc struct {
	ErrorMsg string                    `json:"errorMsg"`
	Data     KnowledgeBaseDataResponse `json:"data"`
}

// KnowledgeBaseDeleteResponseDoc 用于 Swagger 展示删除响应。
type KnowledgeBaseDeleteResponseDoc struct {
	ErrorMsg string                          `json:"errorMsg"`
	Data     KnowledgeBaseDeleteDataResponse `json:"data"`
}

// KnowledgeBaseFileMutationResponseDoc 用于 Swagger 展示文件关联变更响应。
type KnowledgeBaseFileMutationResponseDoc struct {
	ErrorMsg string                                `json:"errorMsg"`
	Data     KnowledgeBaseFileMutationDataResponse `json:"data"`
}

// KnowledgeBaseFileResponseDoc 用于 Swagger 展示平台知识库文件上传响应。
type KnowledgeBaseFileResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     KnowledgeBaseFileDataResponse `json:"data"`
}

// PlatformFileDeleteResponseDoc 用于 Swagger 展示平台资料删除响应。
type PlatformFileDeleteResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     PlatformFileDeleteDataResponse `json:"data"`
}

// KnowledgeBaseFileEmbeddingSubmissionResponseDoc 用于 Swagger 展示平台资料向量化提交响应。
type KnowledgeBaseFileEmbeddingSubmissionResponseDoc struct {
	ErrorMsg string                                       `json:"errorMsg"`
	Data     KnowledgeBaseFileEmbeddingSubmissionResponse `json:"data"`
}

// ErrorDoc 表示错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}

func toKnowledgeBaseResponse(item domainknowledgebase.KnowledgeBase) KnowledgeBaseResponse {
	return KnowledgeBaseResponse{
		PublicID: item.PublicID, Scope: item.Scope, Name: item.Name, Description: item.Description,
		Enabled: item.Enabled, SortOrder: item.SortOrder, Revision: item.Revision,
		FileCount: item.FileCount, ReadyFileCount: item.ReadyFileCount, ProcessingFileCount: item.ProcessingFileCount,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toKnowledgeBaseResponses(items []domainknowledgebase.KnowledgeBase) []KnowledgeBaseResponse {
	results := make([]KnowledgeBaseResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toKnowledgeBaseResponse(item))
	}
	return results
}

func toKnowledgeBaseFileResponses(
	items []domainconversation.FileObject,
	capabilities map[string]appembedding.FileVectorizationCapability,
) []KnowledgeBaseFileResponse {
	results := make([]KnowledgeBaseFileResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toKnowledgeBaseFileResponse(item, capabilities[item.FileID]))
	}
	return results
}

func toKnowledgeBaseFileEmbeddingSubmissionResponse(result appembedding.TargetedSubmissionResult) KnowledgeBaseFileEmbeddingSubmissionResponse {
	skipped := make([]KnowledgeBaseFileEmbeddingSkipResponse, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, KnowledgeBaseFileEmbeddingSkipResponse{FileID: item.FileID, Reason: item.Reason})
	}
	return KnowledgeBaseFileEmbeddingSubmissionResponse{
		SubmittedFileIDs: result.SubmittedFileIDs,
		Skipped:          skipped,
	}
}

func toKnowledgeBaseFileProcessingStatusResponses(
	items []domainconversation.FileObject,
	capabilities map[string]appembedding.FileVectorizationCapability,
) []KnowledgeBaseFileProcessingStatusResponse {
	results := make([]KnowledgeBaseFileProcessingStatusResponse, 0, len(items))
	for _, item := range items {
		results = append(results, KnowledgeBaseFileProcessingStatusResponse{
			FileID: item.FileID, DetectedMIME: item.DetectedMIME, FileCategory: item.FileCategory,
			ProcessingStatus: item.ProcessingStatus, Processing: domainconversation.IsFileProcessing(item), ProcessingReady: item.ProcessingReady,
			ExtractStatus: item.ExtractStatus, EmbedStatus: item.EmbedStatus, EmbedError: item.EmbedError,
			ChunkCount: item.ChunkCount, RAGOptOut: item.RAGOptOut,
			CanVectorize:        capabilities[item.FileID].CanVectorize,
			VectorizationReason: capabilities[item.FileID].Reason,
			UpdatedAt:           item.UpdatedAt,
		})
	}
	return results
}

func toKnowledgeBaseFileResponse(
	item domainconversation.FileObject,
	capability appembedding.FileVectorizationCapability,
) KnowledgeBaseFileResponse {
	return KnowledgeBaseFileResponse{
		FileID: item.FileID, FileName: item.FileName, MimeType: item.MimeType, DetectedMIME: item.DetectedMIME,
		FileCategory: item.FileCategory, SizeBytes: item.SizeBytes, ProcessingStatus: item.ProcessingStatus,
		Processing: domainconversation.IsFileProcessing(item), ProcessingReady: item.ProcessingReady,
		ExtractStatus: item.ExtractStatus, EmbedStatus: item.EmbedStatus, EmbedError: item.EmbedError,
		ChunkCount: item.ChunkCount, RAGOptOut: item.RAGOptOut,
		CanVectorize: capability.CanVectorize, VectorizationReason: capability.Reason,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
