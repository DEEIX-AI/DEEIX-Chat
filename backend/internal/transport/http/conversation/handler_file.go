package conversation

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
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
		response.Error(c, http.StatusBadRequest, "file is required")
		return
	}

	fileReader, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid file stream")
		return
	}
	defer fileReader.Close() //nolint:errcheck

	result, err := h.service.UploadFile(c.Request.Context(), appupload.UploadFileInput{
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
			response.Error(c, http.StatusConflict, "storage quota exceeded")
			return
		case errors.Is(err, appconversation.ErrDangerousMIMEType):
			response.Error(c, http.StatusBadRequest, "dangerous file type not allowed")
			return
		case errors.Is(err, appconversation.ErrMIMEBlocked):
			response.Error(c, http.StatusBadRequest, "mime blocked")
			return
		case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
			response.Error(c, http.StatusBadRequest, "embedding unavailable for this file size")
			return
		case errors.Is(err, appconversation.ErrFileTooLarge):
			response.Error(c, http.StatusRequestEntityTooLarge, "file too large")
			return
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.Error(c, http.StatusBadRequest, "invalid file")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "upload file failed")
			return
		}
	}

	h.recordAudit(c, "upload_file",
		"file",
		result.File.FileID,
		map[string]interface{}{
			"file_name":  result.File.FileName,
			"size_bytes": result.File.SizeBytes,
		},
	)

	response.Success(c, FileUploadResponse{
		File:   toFileObjectResponse(&result.File),
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
	page, pageSize := pageParams(c)
	searchQuery := strings.TrimSpace(c.Query("q"))
	filterKind := normalizeFileKinds(c.Query("kind"))
	sortBy := normalizeFileSort(c.Query("sort"))

	result, err := h.service.ListFiles(c.Request.Context(), userID, page, pageSize, searchQuery, filterKind, sortBy)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list files failed")
		return
	}
	results := make([]FileObjectResponse, 0, len(result.Items))
	for i := range result.Items {
		results = append(results, toFileObjectResponse(&result.Items[i]))
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
		response.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}
	result, err := h.service.GetFileProcessingStatus(c.Request.Context(), userID, fileID)
	if err != nil {
		if errors.Is(err, appconversation.ErrFileNotFound) {
			response.Error(c, http.StatusNotFound, "file not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "get file processing status failed")
		return
	}
	response.Success(c, toFileProcessingStatusResponse(result))
}

// GetFileExtract 获取文件提取文本。
func (h *Handler) GetFileExtract(c *gin.Context) {
	userID := middleware.MustUserID(c)
	fileID := c.Param("file_id")
	if strings.TrimSpace(fileID) == "" {
		response.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}
	result, err := h.service.GetFileExtract(c.Request.Context(), userID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.Error(c, http.StatusBadRequest, "invalid file id")
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.Error(c, http.StatusNotFound, "file not found")
			return
		case errors.Is(err, appconversation.ErrFileProcessingNotReady):
			response.Error(c, http.StatusConflict, "file extract not ready")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "get file extract failed")
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
		response.Error(c, http.StatusInternalServerError, "get chat file policy failed")
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
		response.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	var req UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if req.FileName == nil && req.RagOptOut == nil {
		response.Error(c, http.StatusBadRequest, "at least one of file_name or rag_opt_out is required")
		return
	}

	var (
		item *model.FileObject
		err  error
	)

	if req.FileName != nil {
		item, err = h.service.RenameFile(c.Request.Context(), userID, fileID, *req.FileName)
		if err != nil {
			switch {
			case errors.Is(err, appconversation.ErrInvalidFileReference):
				response.Error(c, http.StatusBadRequest, "invalid file id")
			case errors.Is(err, appconversation.ErrInvalidFileName):
				response.Error(c, http.StatusBadRequest, "invalid file name")
			case errors.Is(err, appconversation.ErrFileNotFound):
				response.Error(c, http.StatusNotFound, "file not found")
			default:
				response.Error(c, http.StatusInternalServerError, "update file failed")
			}
			return
		}
	}

	if req.RagOptOut != nil {
		item, err = h.service.UpdateFileRagOptOut(c.Request.Context(), userID, fileID, *req.RagOptOut)
		if err != nil {
			switch {
			case errors.Is(err, appconversation.ErrInvalidFileReference):
				response.Error(c, http.StatusBadRequest, "invalid file id")
			case errors.Is(err, appconversation.ErrFileNotFound):
				response.Error(c, http.StatusNotFound, "file not found")
			default:
				response.Error(c, http.StatusInternalServerError, "update file failed")
			}
			return
		}
	}

	auditDetail := map[string]interface{}{}
	if req.FileName != nil {
		auditDetail["file_name"] = item.FileName
	}
	if req.RagOptOut != nil {
		auditDetail["rag_opt_out"] = item.RagOptOut
	}
	h.recordAudit(c, "update_file",
		"file",
		item.FileID,
		auditDetail,
	)

	response.Success(c, toFileObjectResponse(item))
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
		response.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	result, err := h.service.DeleteFile(c.Request.Context(), userID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.Error(c, http.StatusBadRequest, "invalid file id")
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.Error(c, http.StatusNotFound, "file not found")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "delete file failed")
			return
		}
	}

	h.recordAudit(c, "delete_file",
		"file",
		result.FileID,
		map[string]interface{}{
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
		response.Error(c, http.StatusBadRequest, "invalid file id")
		return
	}

	result, err := h.service.OpenFileContent(c.Request.Context(), userID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.Error(c, http.StatusBadRequest, "invalid file id")
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.Error(c, http.StatusNotFound, "file not found")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "open file failed")
			return
		}
	}

	defer result.Reader.Close() //nolint:errcheck

	contentType := safeFileContentType(result.ContentType)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", buildContentDisposition(result.File.FileName, isPassiveInlineContentType(contentType)))
	c.Header("Cache-Control", "private, max-age=60")
	applyFileSecurityHeaders(c, false)
	if result.SizeBytes > 0 {
		c.Header("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	}
	if !result.ModTime.IsZero() {
		c.Header("Last-Modified", result.ModTime.UTC().Format(http.TimeFormat))
	}
	if _, err = io.Copy(c.Writer, result.Reader); err != nil {
		c.Abort()
		return
	}
}
