package conversation

import (
	"errors"
	"net/http"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/filecontent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const multipartUploadOverheadBytes = 1 << 20

// UploadFile godoc
// @Summary 上传文件
// @Description 上传对话附件文件，统一存储并扣减用户配额（默认100MB）
// @Tags chat
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param purpose formData string false "文件用途"
// @Param file formData file true "文件"
// @Success 200 {object} UploadFileResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 413 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /files [post]
// UploadFile 上传文件。
func (h *Handler) UploadFile(c *gin.Context) {
	userID := middleware.MustUserID(c)
	// 先在 HTTP 层限制 multipart 总体积，避免解析表单时绕过 service 的文件流式大小校验。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadRequestBytes())
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errFileRequired)
		return
	}

	fileReader, err := fileHeader.Open()
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileStream)
		return
	}
	defer fileReader.Close() //nolint:errcheck

	result, err := h.uploads.UploadFile(c.Request.Context(), appupload.UploadFileInput{
		UserID:       userID,
		Purpose:      c.PostForm("purpose"),
		FileName:     fileHeader.Filename,
		MimeType:     fileHeader.Header.Get("Content-Type"),
		DeclaredSize: fileHeader.Size,
		Reader:       fileReader,
	})
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrStorageQuotaExceeded):
			response.ErrorFrom(c, http.StatusConflict, errStorageQuotaExceeded)
			return
		case errors.Is(err, appconversation.ErrDangerousMIMEType):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, appconversation.ErrMIMEBlocked):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
			response.ErrorFrom(c, http.StatusBadRequest, errFileEmbeddingUnavailable)
			return
		case errors.Is(err, appconversation.ErrFileTooLarge):
			response.ErrorFrom(c, http.StatusRequestEntityTooLarge, err)
			return
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFile)
			return
		default:
			response.InternalError(c)
			return
		}
	}

	h.recordAudit(c, "upload_file",
		"file",
		result.File.FileID,
		map[string]any{
			"file_name":  result.File.FileName,
			"size_bytes": result.File.SizeBytes,
		},
	)

	capability := h.processing.ResolveFileVectorizationCapabilities(
		c.Request.Context(),
		[]model.FileObject{result.File},
	)[result.File.FileID]
	response.Success(c, FileUploadResponse{
		File:   toFileObjectResponse(&result.File, capability),
		Quota:  toStorageQuotaResponse(result.Quota),
		Reused: result.Reused,
	})
}

func (h *Handler) maxUploadRequestBytes() int64 {
	maxUploadBytes := int64(20 * 1024 * 1024)
	if h != nil && h.cfg != nil {
		if configured := h.cfg.Snapshot().MaxUploadFileBytes; configured > 0 {
			maxUploadBytes = configured
		}
	}
	// multipart 边界和字段头会占用额外字节，预留固定开销后再交给 service 校验真实文件大小。
	return maxUploadBytes + multipartUploadOverheadBytes
}

// ListFiles godoc
// @Summary 文件分页列表
// @Description 查询当前用户上传的文件
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索关键词"
// @Param kind query string false "筛选，支持单值或逗号分隔多值: image,document,spreadsheet,presentation,code,pdf,audio,video"
// @Param sort query string false "排序: created|name|size|last_used"
// @Success 200 {object} FileListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /files [get]
// ListFiles 查询文件列表。
func (h *Handler) ListFiles(c *gin.Context) {
	userID := middleware.MustUserID(c)
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	searchQuery := strings.TrimSpace(c.Query("q"))
	filterKind := normalizeFileKinds(c.Query("kind"))
	sortBy := normalizeFileSort(c.Query("sort"))

	result, err := h.uploads.ListFiles(c.Request.Context(), appupload.ListFilesInput{
		UserID:      userID,
		Page:        page,
		PageSize:    pageSize,
		SearchQuery: searchQuery,
		FilterKind:  filterKind,
		SortBy:      sortBy,
	})
	if err != nil {
		response.InternalError(c)
		return
	}
	results := make([]FileObjectResponse, 0, len(result.Items))
	capabilities := h.processing.ResolveFileVectorizationCapabilities(c.Request.Context(), result.Items)
	for i := range result.Items {
		results = append(results, toFileObjectResponse(&result.Items[i], capabilities[result.Items[i].FileID]))
	}
	response.Success(c, FileListResponse{
		Total:   result.Total,
		Results: results,
		Quota:   toStorageQuotaResponse(result.Quota),
	})
}

// GetFileProcessingStatus 查询文件处理状态。
func (h *Handler) GetFileProcessingStatus(c *gin.Context) {
	userID := middleware.MustUserID(c)
	fileID := c.Param("file_id")
	if strings.TrimSpace(fileID) == "" {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
		return
	}
	result, err := h.processing.GetFileProcessingStatus(c.Request.Context(), userID, fileID)
	if err != nil {
		if errors.Is(err, appprocessing.ErrFileNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, toFileProcessingStatusResponse(result))
}

// GetFileProcessingStatuses godoc
// @Summary 批量查询文件处理状态
// @Description 一次查询当前用户多个文件的处理状态
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param request body GetFileProcessingStatusesRequest true "文件ID，最多100个"
// @Success 200 {array} FileProcessingStatusResponse
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /files/processing/statuses [post]
func (h *Handler) GetFileProcessingStatuses(c *gin.Context) {
	var req GetFileProcessingStatusesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	result, err := h.processing.GetFileProcessingStatuses(
		c.Request.Context(),
		middleware.MustUserID(c),
		req.FileIDs,
	)
	if err != nil {
		response.InternalError(c)
		return
	}
	statuses := make([]FileProcessingStatusResponse, 0, len(result))
	for i := range result {
		statuses = append(statuses, toFileProcessingStatusResponse(&result[i]))
	}
	response.Success(c, statuses)
}

// SubmitFileEmbeddings godoc
// @Summary 批量提交指定文件向量化
// @Description 为当前用户已完成文本提取的文件提交向量化任务，最多100个；重复提交会幂等跳过
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Accept json
// @Param request body SubmitFileEmbeddingsRequest true "文件ID，最多100个"
// @Success 200 {object} FileEmbeddingSubmissionResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Failure 503 {object} ErrorDoc
// @Router /files/embeddings [post]
func (h *Handler) SubmitFileEmbeddings(c *gin.Context) {
	var req SubmitFileEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.processing.SubmitFileEmbeddings(c.Request.Context(), middleware.MustUserID(c), req.FileIDs)
	if err != nil {
		switch {
		case errors.Is(err, appembedding.ErrTooManyTargetedFiles):
			response.ErrorWithCode(c, http.StatusBadRequest, "embedding.too_many_files")
		case errors.Is(err, appembedding.ErrEmbeddingServiceNotConfigured):
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_not_configured")
		case errors.Is(err, appembedding.ErrEmbeddingServiceUnavailable):
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_unavailable")
		default:
			response.ErrorWithCode(c, http.StatusInternalServerError, "embedding.submit_failed")
		}
		return
	}
	h.recordAudit(c, "submit_file_embeddings", "file", "", map[string]any{
		"requested_file_ids": req.FileIDs,
		"submitted_file_ids": result.SubmittedFileIDs,
		"skipped_count":      len(result.Skipped),
	})
	response.Success(c, toFileEmbeddingSubmissionResponse(result))
}

// GetFileExtract 获取文件提取文本。
func (h *Handler) GetFileExtract(c *gin.Context) {
	userID := middleware.MustUserID(c)
	fileID := c.Param("file_id")
	if strings.TrimSpace(fileID) == "" {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
		return
	}
	result, err := h.service.GetFileExtract(c.Request.Context(), userID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
			return
		case errors.Is(err, appconversation.ErrFileProcessingNotReady):
			response.ErrorFrom(c, http.StatusConflict, errFileExtractNotReady)
			return
		default:
			response.InternalError(c)
			return
		}
	}
	response.Success(c, toFileExtractResponse(result))
}

// GetChatFilePolicy 返回聊天文件策略。
func (h *Handler) GetChatFilePolicy(c *gin.Context) {
	userID := middleware.MustUserID(c)
	result, err := h.service.GetChatFilePolicy(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Success(c, toChatFilePolicyResponse(result))
}

// UpdateFile godoc
// @Summary 更新文件属性
// @Description 修改文件名或 RAG 检索开关，file_name 和 rag_opt_out 至少填一个
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file_id path string true "文件ID"
// @Param body body UpdateFileRequest true "更新内容"
// @Success 200 {object} FileUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /files/{file_id} [patch]
func (h *Handler) UpdateFile(c *gin.Context) {
	userID := middleware.MustUserID(c)
	fileID := c.Param("file_id")
	if strings.TrimSpace(fileID) == "" {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
		return
	}

	var req UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if req.FileName == nil && req.RAGOptOut == nil {
		response.ErrorFrom(c, http.StatusBadRequest, errAtLeastOneOfFileNameOrRAGOptOutRequired)
		return
	}

	var (
		item *model.FileObject
		err  error
	)

	if req.FileName != nil {
		item, err = h.uploads.RenameFile(c.Request.Context(), userID, fileID, *req.FileName)
		if err != nil {
			switch {
			case errors.Is(err, appconversation.ErrInvalidFileReference):
				response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
			case errors.Is(err, appconversation.ErrInvalidFileName):
				response.ErrorFrom(c, http.StatusBadRequest, err)
			case errors.Is(err, appconversation.ErrFileNotFound):
				response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
			default:
				response.InternalError(c)
			}
			return
		}
	}

	if req.RAGOptOut != nil {
		item, err = h.uploads.UpdateFileRAGOptOut(c.Request.Context(), userID, fileID, *req.RAGOptOut)
		if err != nil {
			switch {
			case errors.Is(err, appconversation.ErrInvalidFileReference):
				response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
			case errors.Is(err, appconversation.ErrFileNotFound):
				response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
			default:
				response.InternalError(c)
			}
			return
		}
	}

	auditDetail := map[string]any{}
	if req.FileName != nil {
		auditDetail["file_name"] = item.FileName
	}
	if req.RAGOptOut != nil {
		auditDetail["rag_opt_out"] = item.RAGOptOut
	}
	h.recordAudit(c, "update_file",
		"file",
		item.FileID,
		auditDetail,
	)

	capability := h.processing.ResolveFileVectorizationCapabilities(
		c.Request.Context(),
		[]model.FileObject{*item},
	)[item.FileID]
	response.Success(c, toFileObjectResponse(item, capability))
}

// DeleteFile godoc
// @Summary 删除文件
// @Description 删除指定文件并回收用户配额
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file_id path string true "文件ID"
// @Success 200 {object} DeleteFileResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /files/{file_id} [delete]
// DeleteFile 删除文件。
func (h *Handler) DeleteFile(c *gin.Context) {
	userID := middleware.MustUserID(c)
	fileID := c.Param("file_id")
	if fileID == "" {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
		return
	}

	result, err := h.uploads.DeleteFile(c.Request.Context(), userID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
			return
		case errors.Is(err, appconversation.ErrFileInUse):
			response.ErrorFrom(c, http.StatusConflict, err)
			return
		default:
			response.InternalError(c)
			return
		}
	}

	h.recordAudit(c, "delete_file",
		"file",
		result.FileID,
		map[string]any{
			"deleted": true,
		},
	)

	response.Success(c, toDeleteFileResponse(result))
}

// GetFileContent godoc
// @Summary 获取文件内容
// @Description 按当前登录用户权限读取文件内容，用于在线预览或下载
// @Tags chat
// @Produce application/octet-stream
// @Security BearerAuth
// @Param file_id path string true "文件ID"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /files/{file_id}/content [get]
func (h *Handler) GetFileContent(c *gin.Context) {
	userID := middleware.MustUserID(c)
	fileID := c.Param("file_id")
	if strings.TrimSpace(fileID) == "" {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
		return
	}

	result, err := h.uploads.OpenFileContent(c.Request.Context(), userID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
			return
		default:
			response.InternalError(c)
			return
		}
	}

	_ = filecontent.Write(c, result, false)
}
