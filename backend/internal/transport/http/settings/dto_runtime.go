package settings

import (
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/runtime"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
)

type ServiceRuntimeResponse struct {
	Source        string `json:"source"`
	BaseURL       string `json:"baseURL"`
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	Network       string `json:"network"`
	Status        string `json:"status"`
	Reachable     bool   `json:"reachable"`
	Message       string `json:"message"`
}

// EmbeddingIndexStatusResponse 表示全平台向量索引的状态分布。
type EmbeddingIndexStatusResponse struct {
	ModelSignature string `json:"modelSignature"`
	ReadyCount     int64  `json:"readyCount"`
	StaleCount     int64  `json:"staleCount"`
	// PendingCount 是尚未索引的可向量化文件与排队、执行中的任务之和。
	PendingCount int64 `json:"pendingCount"`
	// StalledCount 是 PendingCount 中排队或执行超过停滞阈值、可以重新提交的任务数。
	StalledCount int64 `json:"stalledCount"`
	// ActiveCount 是 PendingCount 中仍在正常排队或执行的任务数。
	ActiveCount int64 `json:"activeCount"`
	FailedCount int64 `json:"failedCount"`
	// EmptyCount 是提取完成但无文本的文件数；这些文件不参与自动重建。
	EmptyCount int64 `json:"emptyCount"`
	// UnsupportedCount 是当前配置下类型无法向量化的文件数，不计入待处理。
	UnsupportedCount int64 `json:"unsupportedCount"`
	NeedsReindex     bool  `json:"needsReindex"`
	// ReindexRunning 表示补建任务仍在后台执行；多实例部署时只反映处理本次请求的实例。
	ReindexRunning bool `json:"reindexRunning"`
}

type EmbeddingIndexStatusResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     EmbeddingIndexStatusResponse `json:"data"`
}

type EmbeddingReindexResponse struct {
	Submitted int    `json:"submitted"`
	Message   string `json:"message"`
}

type EmbeddingReindexResponseDoc struct {
	ErrorMsg string                   `json:"errorMsg"`
	Data     EmbeddingReindexResponse `json:"data"`
}

// EmbeddingTaskResponse 表示管理员任务列表中的一个文件。
type EmbeddingTaskResponse struct {
	FileID    string `json:"fileID"`
	FileName  string `json:"fileName"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
	UserID    uint   `json:"userID"`
	UserLabel string `json:"userLabel"`
	// EmbedStatus 取值 none、queued、processing、failed、stale、empty。
	EmbedStatus string `json:"embedStatus"`
	// EmbedError 是失败原因：处理流水线失败时取处理错误，否则取向量化错误。
	EmbedError string `json:"embedError"`
	// Stalled 表示排队或执行超过停滞阈值，重试会重新投递任务。
	Stalled bool `json:"stalled"`
	// Retryable 为 false 时 RetryBlockedReason 说明原因，取值与向量化提交的 skipped.reason 一致。
	Retryable          bool      `json:"retryable"`
	RetryBlockedReason string    `json:"retryBlockedReason"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// EmbeddingTaskListResponseDoc 向量化任务分页响应文档。
type EmbeddingTaskListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                   `json:"total"`
		Results []EmbeddingTaskResponse `json:"results"`
	} `json:"data"`
}

// EmbeddingTaskRetryRequest 是管理员重试向量化任务的请求体。
type EmbeddingTaskRetryRequest struct {
	FileIDs []string `json:"fileIDs" binding:"required,min=1,max=100,dive,required,max=64"`
}

// EmbeddingTaskSkipResponse 表示未提交重试的文件及原因。
type EmbeddingTaskSkipResponse struct {
	FileID string `json:"fileID"`
	Reason string `json:"reason"`
}

// EmbeddingTaskRetryResponse 表示重试提交结果。
type EmbeddingTaskRetryResponse struct {
	SubmittedFileIDs []string                    `json:"submittedFileIDs"`
	Skipped          []EmbeddingTaskSkipResponse `json:"skipped"`
}

// EmbeddingTaskRetryResponseDoc 重试提交响应文档。
type EmbeddingTaskRetryResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     EmbeddingTaskRetryResponse `json:"data"`
}

func toServiceRuntimeResponse(view appruntime.ServiceRuntimeView) ServiceRuntimeResponse {
	return ServiceRuntimeResponse{
		Source:        view.Source,
		BaseURL:       view.BaseURL,
		ContainerName: view.ContainerName,
		Image:         view.Image,
		Network:       view.Network,
		Status:        view.Status,
		Reachable:     view.Reachable,
		Message:       view.Message,
	}
}

func toEmbeddingIndexStatusResponse(status appembedding.EmbeddingIndexStatus) EmbeddingIndexStatusResponse {
	return EmbeddingIndexStatusResponse{
		ModelSignature:   status.ModelSignature,
		ReadyCount:       status.ReadyCount,
		StaleCount:       status.StaleCount,
		PendingCount:     status.PendingCount,
		StalledCount:     status.StalledCount,
		ActiveCount:      status.ActiveCount,
		FailedCount:      status.FailedCount,
		EmptyCount:       status.EmptyCount,
		UnsupportedCount: status.UnsupportedCount,
		NeedsReindex:     status.NeedsReindex,
		ReindexRunning:   status.ReindexRunning,
	}
}

func toEmbeddingTaskResponse(task appembedding.FileTask, owner appadmin.UserLabel) EmbeddingTaskResponse {
	file := task.File
	return EmbeddingTaskResponse{
		FileID:             file.FileID,
		FileName:           file.FileName,
		MimeType:           textutil.FirstNonEmpty(file.DetectedMIME, file.MimeType),
		SizeBytes:          file.SizeBytes,
		UserID:             file.UserID,
		UserLabel:          owner.Label,
		EmbedStatus:        file.EmbedStatus,
		EmbedError:         embeddingTaskError(file),
		Stalled:            task.Stalled,
		Retryable:          task.RetryBlockedReason == "",
		RetryBlockedReason: task.RetryBlockedReason,
		UpdatedAt:          file.UpdatedAt,
	}
}

// embeddingTaskError 返回管理员能据此判断下一步的失败原因：处理失败的文件没有文本可向量化，
// 原因在处理错误上；其余取向量化错误。
func embeddingTaskError(file domainconversation.FileObject) string {
	if file.ProcessingStatus == "failed" {
		return textutil.FirstNonEmpty(file.ProcessingErrorMessage, file.EmbedError)
	}
	return file.EmbedError
}

func toEmbeddingTaskRetryResponse(result appembedding.TargetedSubmissionResult) EmbeddingTaskRetryResponse {
	skipped := make([]EmbeddingTaskSkipResponse, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, EmbeddingTaskSkipResponse{FileID: item.FileID, Reason: item.Reason})
	}
	submitted := result.SubmittedFileIDs
	if submitted == nil {
		submitted = []string{}
	}
	return EmbeddingTaskRetryResponse{SubmittedFileIDs: submitted, Skipped: skipped}
}

func toTikaRuntimeResponse(view appruntime.ServiceRuntimeView) ServiceRuntimeResponse {
	return toServiceRuntimeResponse(view)
}

func toDoclingRuntimeResponse(view appruntime.ServiceRuntimeView) ServiceRuntimeResponse {
	return toServiceRuntimeResponse(view)
}

func toTesseractRuntimeResponse(view appruntime.ServiceRuntimeView) ServiceRuntimeResponse {
	return toServiceRuntimeResponse(view)
}

func toRapidOCRRuntimeResponse(view appruntime.ServiceRuntimeView) ServiceRuntimeResponse {
	return toServiceRuntimeResponse(view)
}

func toMinerURuntimeResponse(view appruntime.ServiceRuntimeView) ServiceRuntimeResponse {
	return toServiceRuntimeResponse(view)
}
