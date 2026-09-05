package knowledgebase

import (
	"errors"
	"net/http"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/knowledgebase"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/filecontent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/queryparam"
	"github.com/gin-gonic/gin"
)

// Handler 封装知识库 HTTP 处理。
type Handler struct {
	service *appknowledgebase.Service
	cfg     *config.Runtime
}

// NewHandler 创建知识库处理器。
func NewHandler(service *appknowledgebase.Service, cfg *config.Runtime) *Handler {
	return &Handler{service: service, cfg: cfg}
}

const multipartUploadOverheadBytes = 1 << 20

// requireKnowledgeBaseEnabled 在知识库功能被后台关闭时拒绝用户侧请求。
func (h *Handler) requireKnowledgeBaseEnabled(c *gin.Context) {
	if h.cfg.Snapshot().KnowledgeBaseEnabled {
		c.Next()
		return
	}
	response.ErrorWithCode(c, http.StatusForbidden, "knowledge_base.disabled")
	c.Abort()
}

// ListVisible godoc
// @Summary 查询当前用户可用知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param sort query string false "排序方式(default/name/created/updated/files)"
// @Param id query []string false "知识库ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBasePageResponseDoc
// @Router /knowledge-bases [get]
func (h *Handler) ListVisible(c *gin.Context) {
	items, total, err := h.service.ListVisible(c.Request.Context(), middleware.MustUserID(c), listInput(c))
	writeList(c, items, total, err)
}

// ListMine godoc
// @Summary 查询我的知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param sort query string false "排序方式(default/name/created/updated/files)"
// @Param id query []string false "知识库ID"
// @Param enabled query bool false "可用状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBasePageResponseDoc
// @Router /knowledge-bases/mine [get]
func (h *Handler) ListMine(c *gin.Context) {
	input := listInput(c)
	input.Enabled = queryparam.OptionalBool(strings.TrimSpace(c.Query("enabled")))
	items, total, err := h.service.ListMine(c.Request.Context(), middleware.MustUserID(c), input)
	writeList(c, items, total, err)
}

// GetVisible godoc
// @Summary 查询知识库详情
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Success 200 {object} KnowledgeBaseResponseDoc
// @Router /knowledge-bases/{id} [get]
func (h *Handler) GetVisible(c *gin.Context) {
	item, err := h.service.GetVisible(c.Request.Context(), middleware.MustUserID(c), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, KnowledgeBaseDataResponse{KnowledgeBase: toKnowledgeBaseResponse(*item)})
}

// CreateMine godoc
// @Summary 创建个人知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param body body WriteMyKnowledgeBaseRequest true "知识库配置"
// @Success 200 {object} KnowledgeBaseResponseDoc
// @Router /knowledge-bases/mine [post]
func (h *Handler) CreateMine(c *gin.Context) {
	var req WriteMyKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateUser(c.Request.Context(), middleware.MustUserID(c), appknowledgebase.WriteInput{
		Name: req.Name, Description: req.Description, Enabled: true,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, KnowledgeBaseDataResponse{KnowledgeBase: toKnowledgeBaseResponse(*item)})
}

// PatchMine godoc
// @Summary 更新个人知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param body body PatchMyKnowledgeBaseRequest true "更新字段"
// @Success 200 {object} KnowledgeBaseResponseDoc
// @Router /knowledge-bases/mine/{id} [patch]
func (h *Handler) PatchMine(c *gin.Context) {
	var req PatchMyKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	h.patch(c, false, appknowledgebase.PatchInput{Name: req.Name, Description: req.Description})
}

// DeleteMine godoc
// @Summary 删除个人知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param delete_files query bool false "是否同步删除不再被其他资源引用的知识库文件"
// @Success 200 {object} KnowledgeBaseDeleteResponseDoc
// @Router /knowledge-bases/mine/{id} [delete]
func (h *Handler) DeleteMine(c *gin.Context) {
	h.delete(c, false)
}

// ListVisibleFiles godoc
// @Summary 查询知识库文件
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBaseFilePageResponseDoc
// @Router /knowledge-bases/{id}/files [get]
func (h *Handler) ListVisibleFiles(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListVisibleFiles(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), page, pageSize)
	h.writeFileList(c, items, total, err)
}

// GetVisibleFileProcessingStatuses godoc
// @Summary 批量查询知识库文件处理状态
// @Tags knowledge-bases
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body GetKnowledgeBaseFileProcessingStatusesRequest true "文件ID，最多100个"
// @Success 200 {array} KnowledgeBaseFileProcessingStatusResponse
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /knowledge-bases/{id}/files/processing/statuses [post]
func (h *Handler) GetVisibleFileProcessingStatuses(c *gin.Context) {
	h.getFileProcessingStatuses(c, false)
}

// GetVisibleFileProcessingSnapshot godoc
// @Summary 查询当前用户可见知识库处理快照
// @Tags knowledge-bases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "知识库公开ID"
// @Param request body GetKnowledgeBaseFileProcessingSnapshotRequest true "当前页处理中或待确认的文件ID，最多100个，可为空"
// @Success 200 {object} KnowledgeBaseFileProcessingSnapshotResponse
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Router /knowledge-bases/{id}/files/processing/snapshot [post]
func (h *Handler) GetVisibleFileProcessingSnapshot(c *gin.Context) {
	h.getFileProcessingSnapshot(c, false)
}

// ListAvailableMineFiles godoc
// @Summary 查询可加入个人知识库的文件
// @Description 分页返回当前用户尚未关联到指定个人知识库的有效文件
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param q query string false "文件名搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBaseFilePageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /knowledge-bases/mine/{id}/available-files [get]
func (h *Handler) ListAvailableMineFiles(c *gin.Context) {
	items, total, err := h.service.ListAvailableUserFiles(
		c.Request.Context(),
		middleware.MustUserID(c),
		c.Param("id"),
		listInput(c),
	)
	h.writeFileList(c, items, total, err)
}

// GetVisibleFileContent godoc
// @Summary 获取知识库文件内容
// @Description 仅允许读取当前用户可见且仍与知识库关联的文件
// @Tags knowledge-bases
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param file_id path string true "文件ID"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /knowledge-bases/{id}/files/{file_id}/content [get]
func (h *Handler) GetVisibleFileContent(c *gin.Context) {
	result, err := h.service.OpenVisibleFileContent(
		c.Request.Context(),
		middleware.MustUserID(c),
		c.Param("id"),
		c.Param("file_id"),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	_ = filecontent.Write(c, result, false)
}

// AddMineFiles godoc
// @Summary 将已有文件加入个人知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param body body AddKnowledgeBaseFilesRequest true "文件ID列表"
// @Success 200 {object} KnowledgeBaseFileMutationResponseDoc
// @Router /knowledge-bases/mine/{id}/files [post]
func (h *Handler) AddMineFiles(c *gin.Context) {
	h.addFiles(c, false)
}

// RemoveMineFile godoc
// @Summary 将文件移出个人知识库
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param file_id path string true "文件ID"
// @Success 200 {object} KnowledgeBaseFileMutationResponseDoc
// @Router /knowledge-bases/mine/{id}/files/{file_id} [delete]
func (h *Handler) RemoveMineFile(c *gin.Context) {
	h.removeFile(c, false)
}

// ListAdmin godoc
// @Summary 查询内置知识库
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param sort query string false "排序方式(default/name/created/updated/files)"
// @Param id query []string false "知识库ID"
// @Param enabled query bool false "可用状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBasePageResponseDoc
// @Router /admin/knowledge-bases [get]
func (h *Handler) ListAdmin(c *gin.Context) {
	input := listInput(c)
	input.Enabled = queryparam.OptionalBool(strings.TrimSpace(c.Query("enabled")))
	items, total, err := h.service.ListAdminBuiltin(c.Request.Context(), input)
	writeList(c, items, total, err)
}

// ListPlatformFiles godoc
// @Summary 查询平台资料
// @Description 分页返回供内置知识库复用的全部平台资料
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param q query string false "文件名搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBaseFilePageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/files [get]
func (h *Handler) ListPlatformFiles(c *gin.Context) {
	items, total, err := h.service.ListPlatformFiles(
		c.Request.Context(),
		middleware.MustUserID(c),
		listInput(c),
	)
	h.writeFileList(c, items, total, err)
}

// UploadAdminFile godoc
// @Summary 上传内置知识库资料
// @Description 上传平台级资料，不占用管理员个人存储额度
// @Tags admin-knowledge-bases
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "文件"
// @Success 200 {object} KnowledgeBaseFileResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 413 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/files [post]
func (h *Handler) UploadAdminFile(c *gin.Context) {
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

	result, err := h.service.UploadBuiltinFile(c.Request.Context(), middleware.MustUserID(c), appupload.UploadFileInput{
		FileName: fileHeader.Filename, MimeType: fileHeader.Header.Get("Content-Type"),
		DeclaredSize: fileHeader.Size, Reader: fileReader,
	})
	if err != nil {
		h.writeUploadError(c, err)
		return
	}
	h.audit(c, "knowledge_base.upload_builtin_file", result.File.FileID, map[string]any{
		"file_name": result.File.FileName, "size_bytes": result.File.SizeBytes,
	})
	capability := h.service.ResolveFileVectorizationCapabilities(
		c.Request.Context(),
		[]domainconversation.FileObject{result.File},
	)[result.File.FileID]
	response.Success(c, KnowledgeBaseFileDataResponse{File: toKnowledgeBaseFileResponse(result.File, capability)})
}

// SubmitAdminFileEmbeddings godoc
// @Summary 批量提交平台资料向量化
// @Description 为管理员选中的平台资料提交向量化任务，最多100个；重复提交会幂等跳过
// @Tags admin-knowledge-bases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body SubmitPlatformFileEmbeddingsRequest true "平台资料ID，最多100个"
// @Success 200 {object} KnowledgeBaseFileEmbeddingSubmissionResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Failure 503 {object} ErrorDoc
// @Router /admin/knowledge-bases/files/embeddings [post]
func (h *Handler) SubmitAdminFileEmbeddings(c *gin.Context) {
	var req SubmitPlatformFileEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.SubmitPlatformFileEmbeddings(c.Request.Context(), middleware.MustUserID(c), req.FileIDs)
	if err != nil {
		switch {
		case errors.Is(err, appembedding.ErrTooManyTargetedFiles):
			response.ErrorWithCode(c, http.StatusBadRequest, "embedding.too_many_files")
		case errors.Is(err, appembedding.ErrEmbeddingServiceNotConfigured):
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_not_configured")
		case errors.Is(err, appembedding.ErrEmbeddingServiceUnavailable):
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_unavailable")
		default:
			writeError(c, err)
		}
		return
	}
	h.audit(c, "knowledge_base.submit_platform_file_embeddings", "", map[string]any{
		"requested_file_ids": req.FileIDs,
		"submitted_file_ids": result.SubmittedFileIDs,
		"skipped_count":      len(result.Skipped),
	})
	response.Success(c, toKnowledgeBaseFileEmbeddingSubmissionResponse(result))
}

// DeleteAdminFile godoc
// @Summary 删除平台资料
// @Description 仅允许删除未被任何知识库、会话或账户资料引用的平台资料
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param file_id path string true "文件ID"
// @Success 200 {object} PlatformFileDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/files/{file_id} [delete]
func (h *Handler) DeleteAdminFile(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		writeError(c, appknowledgebase.ErrInvalidKnowledgeBase)
		return
	}
	err := h.service.DeletePlatformFile(c.Request.Context(), middleware.MustUserID(c), fileID)
	if err != nil {
		writeError(c, err)
		return
	}
	h.audit(c, "knowledge_base.delete_platform_file", fileID, map[string]any{"deleted": true})
	response.Success(c, PlatformFileDeleteDataResponse{Deleted: true})
}

// GetPlatformFileContent godoc
// @Summary 获取平台资料内容
// @Description 仅允许管理员读取平台资料池中的文件
// @Tags admin-knowledge-bases
// @Produce application/octet-stream
// @Security BearerAuth
// @Param file_id path string true "文件ID"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/files/{file_id}/content [get]
func (h *Handler) GetPlatformFileContent(c *gin.Context) {
	result, err := h.service.OpenPlatformFileContent(
		c.Request.Context(),
		middleware.MustUserID(c),
		c.Param("file_id"),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	_ = filecontent.Write(c, result, false)
}

func (h *Handler) maxUploadRequestBytes() int64 {
	maxUploadBytes := int64(20 * 1024 * 1024)
	if h != nil && h.cfg != nil {
		if configured := h.cfg.Snapshot().MaxUploadFileBytes; configured > 0 {
			maxUploadBytes = configured
		}
	}
	return maxUploadBytes + multipartUploadOverheadBytes
}

func (h *Handler) writeUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconversation.ErrStorageQuotaExceeded):
		response.ErrorFrom(c, http.StatusConflict, errStorageQuotaExceeded)
	case errors.Is(err, appconversation.ErrDangerousMIMEType), errors.Is(err, appconversation.ErrMIMEBlocked), errors.Is(err, appconversation.ErrInvalidFileReference):
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFile)
	case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
		response.ErrorFrom(c, http.StatusBadRequest, errFileEmbeddingUnavailable)
	case errors.Is(err, appconversation.ErrFileTooLarge):
		response.ErrorFrom(c, http.StatusRequestEntityTooLarge, err)
	default:
		response.InternalError(c)
	}
}

// CreateAdmin godoc
// @Summary 创建内置知识库
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param body body WriteKnowledgeBaseRequest true "知识库配置"
// @Success 200 {object} KnowledgeBaseResponseDoc
// @Router /admin/knowledge-bases [post]
func (h *Handler) CreateAdmin(c *gin.Context) {
	var req WriteKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateBuiltin(c.Request.Context(), middleware.MustUserID(c), writeInput(req))
	if err != nil {
		writeError(c, err)
		return
	}
	h.audit(c, "knowledge_base.create_builtin", item.PublicID, map[string]any{"name": item.Name})
	response.Success(c, KnowledgeBaseDataResponse{KnowledgeBase: toKnowledgeBaseResponse(*item)})
}

// GetAdmin godoc
// @Summary 查询内置知识库详情
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Success 200 {object} KnowledgeBaseResponseDoc
// @Router /admin/knowledge-bases/{id} [get]
func (h *Handler) GetAdmin(c *gin.Context) {
	item, err := h.service.GetAdmin(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, KnowledgeBaseDataResponse{KnowledgeBase: toKnowledgeBaseResponse(*item)})
}

// PatchAdmin godoc
// @Summary 更新内置知识库
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param body body PatchKnowledgeBaseRequest true "更新字段"
// @Success 200 {object} KnowledgeBaseResponseDoc
// @Router /admin/knowledge-bases/{id} [patch]
func (h *Handler) PatchAdmin(c *gin.Context) {
	var req PatchKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	h.patch(c, true, appknowledgebase.PatchInput{
		Name: req.Name, Description: req.Description, Enabled: req.Enabled, SortOrder: req.SortOrder,
	})
}

// DeleteAdmin godoc
// @Summary 删除内置知识库
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param delete_files query bool false "是否同步删除不再被其他资源引用的知识库文件"
// @Success 200 {object} KnowledgeBaseDeleteResponseDoc
// @Router /admin/knowledge-bases/{id} [delete]
func (h *Handler) DeleteAdmin(c *gin.Context) { h.delete(c, true) }

// ListAdminFiles godoc
// @Summary 查询内置知识库文件
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBaseFilePageResponseDoc
// @Router /admin/knowledge-bases/{id}/files [get]
func (h *Handler) ListAdminFiles(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListAdminFiles(c.Request.Context(), c.Param("id"), page, pageSize)
	h.writeFileList(c, items, total, err)
}

// GetAdminFileProcessingStatuses godoc
// @Summary 批量查询内置知识库文件处理状态
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "知识库ID"
// @Param request body GetKnowledgeBaseFileProcessingStatusesRequest true "文件ID，最多100个"
// @Success 200 {array} KnowledgeBaseFileProcessingStatusResponse
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/{id}/files/processing/statuses [post]
func (h *Handler) GetAdminFileProcessingStatuses(c *gin.Context) {
	h.getFileProcessingStatuses(c, true)
}

// GetAdminFileProcessingSnapshot godoc
// @Summary 查询内置知识库处理快照
// @Tags admin-knowledge-bases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "知识库公开ID"
// @Param request body GetKnowledgeBaseFileProcessingSnapshotRequest true "当前页处理中或待确认的文件ID，最多100个，可为空"
// @Success 200 {object} KnowledgeBaseFileProcessingSnapshotResponse
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Router /admin/knowledge-bases/{id}/files/processing/snapshot [post]
func (h *Handler) GetAdminFileProcessingSnapshot(c *gin.Context) {
	h.getFileProcessingSnapshot(c, true)
}

// ListAvailableAdminFiles godoc
// @Summary 查询可加入内置知识库的文件
// @Description 分页返回尚未关联到指定内置知识库的平台资料
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param q query string false "文件名搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} KnowledgeBaseFilePageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/{id}/available-files [get]
func (h *Handler) ListAvailableAdminFiles(c *gin.Context) {
	items, total, err := h.service.ListAvailableAdminFiles(
		c.Request.Context(),
		middleware.MustUserID(c),
		c.Param("id"),
		listInput(c),
	)
	h.writeFileList(c, items, total, err)
}

// GetAdminFileContent godoc
// @Summary 获取内置知识库文件内容
// @Description 仅允许读取仍与指定内置知识库关联的文件
// @Tags admin-knowledge-bases
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param file_id path string true "文件ID"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/knowledge-bases/{id}/files/{file_id}/content [get]
func (h *Handler) GetAdminFileContent(c *gin.Context) {
	result, err := h.service.OpenAdminFileContent(c.Request.Context(), c.Param("id"), c.Param("file_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	_ = filecontent.Write(c, result, false)
}

// AddAdminFiles godoc
// @Summary 将平台资料加入内置知识库
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param body body AddKnowledgeBaseFilesRequest true "文件ID列表"
// @Success 200 {object} KnowledgeBaseFileMutationResponseDoc
// @Router /admin/knowledge-bases/{id}/files [post]
func (h *Handler) AddAdminFiles(c *gin.Context) { h.addFiles(c, true) }

// RemoveAdminFile godoc
// @Summary 将文件移出内置知识库
// @Tags admin-knowledge-bases
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param file_id path string true "文件ID"
// @Success 200 {object} KnowledgeBaseFileMutationResponseDoc
// @Router /admin/knowledge-bases/{id}/files/{file_id} [delete]
func (h *Handler) RemoveAdminFile(c *gin.Context) { h.removeFile(c, true) }

func (h *Handler) patch(c *gin.Context, admin bool, input appknowledgebase.PatchInput) {
	var result *domainknowledgebase.KnowledgeBase
	var err error
	if admin {
		result, err = h.service.UpdateBuiltin(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), input)
	} else {
		result, err = h.service.UpdateUser(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), input)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	if admin {
		h.audit(c, "knowledge_base.update_builtin", result.PublicID, nil)
	}
	response.Success(c, KnowledgeBaseDataResponse{KnowledgeBase: toKnowledgeBaseResponse(*result)})
}

func (h *Handler) delete(c *gin.Context, admin bool) {
	publicID := c.Param("id")
	deleteFiles := c.Query("delete_files") == "true"
	var result appknowledgebase.DeleteResult
	var err error
	if admin {
		result, err = h.service.DeleteBuiltin(c.Request.Context(), publicID, deleteFiles)
	} else {
		result, err = h.service.DeleteUser(c.Request.Context(), middleware.MustUserID(c), publicID, deleteFiles)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	if admin {
		h.audit(c, "knowledge_base.delete_builtin", publicID, map[string]any{
			"delete_files":       deleteFiles,
			"deleted_file_count": result.DeletedFileCount,
		})
	}
	response.Success(c, KnowledgeBaseDeleteDataResponse{Deleted: true, DeletedFileCount: result.DeletedFileCount})
}

func (h *Handler) addFiles(c *gin.Context, admin bool) {
	var req AddKnowledgeBaseFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	var err error
	if admin {
		err = h.service.AddBuiltinFiles(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), req.FileIDs)
	} else {
		err = h.service.AddUserFiles(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), req.FileIDs)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	if admin {
		h.audit(c, "knowledge_base.add_files_builtin", c.Param("id"), map[string]any{"count": len(req.FileIDs)})
	}
	response.Success(c, KnowledgeBaseFileMutationDataResponse{Updated: true})
}

func (h *Handler) removeFile(c *gin.Context, admin bool) {
	var err error
	if admin {
		err = h.service.RemoveBuiltinFile(c.Request.Context(), c.Param("id"), c.Param("file_id"))
	} else {
		err = h.service.RemoveUserFile(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), c.Param("file_id"))
	}
	if err != nil {
		writeError(c, err)
		return
	}
	if admin {
		h.audit(c, "knowledge_base.remove_file_builtin", c.Param("id"), map[string]any{"file_id": c.Param("file_id")})
	}
	response.Success(c, KnowledgeBaseFileMutationDataResponse{Updated: true})
}

func (h *Handler) getFileProcessingStatuses(c *gin.Context, admin bool) {
	var req GetKnowledgeBaseFileProcessingStatusesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	var files []domainconversation.FileObject
	var err error
	if admin {
		files, err = h.service.GetAdminFileProcessingStatuses(c.Request.Context(), c.Param("id"), req.FileIDs)
	} else {
		files, err = h.service.GetVisibleFileProcessingStatuses(
			c.Request.Context(),
			middleware.MustUserID(c),
			c.Param("id"),
			req.FileIDs,
		)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	capabilities := h.service.ResolveFileVectorizationCapabilities(c.Request.Context(), files)
	response.Success(c, toKnowledgeBaseFileProcessingStatusResponses(files, capabilities))
}

func (h *Handler) getFileProcessingSnapshot(c *gin.Context, admin bool) {
	var req GetKnowledgeBaseFileProcessingSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	var item *domainknowledgebase.KnowledgeBase
	var files []domainconversation.FileObject
	var err error
	if admin {
		item, files, err = h.service.GetAdminFileProcessingSnapshot(c.Request.Context(), c.Param("id"), req.FileIDs)
	} else {
		item, files, err = h.service.GetVisibleFileProcessingSnapshot(
			c.Request.Context(),
			middleware.MustUserID(c),
			c.Param("id"),
			req.FileIDs,
		)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, KnowledgeBaseFileProcessingSnapshotResponse{
		KnowledgeBase: toKnowledgeBaseResponse(*item),
		Statuses: toKnowledgeBaseFileProcessingStatusResponses(
			files,
			h.service.ResolveFileVectorizationCapabilities(c.Request.Context(), files),
		),
	})
}

func writeList(c *gin.Context, items []domainknowledgebase.KnowledgeBase, total int64, err error) {
	if err != nil {
		writeError(c, err)
		return
	}
	response.SuccessPage(c, total, toKnowledgeBaseResponses(items))
}

func (h *Handler) writeFileList(c *gin.Context, items []domainconversation.FileObject, total int64, err error) {
	if err != nil {
		writeError(c, err)
		return
	}
	capabilities := h.service.ResolveFileVectorizationCapabilities(c.Request.Context(), items)
	response.SuccessPage(c, total, toKnowledgeBaseFileResponses(items, capabilities))
}

func writeInput(req WriteKnowledgeBaseRequest) appknowledgebase.WriteInput {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return appknowledgebase.WriteInput{Name: req.Name, Description: req.Description, Enabled: enabled, SortOrder: req.SortOrder}
}

func listInput(c *gin.Context) appknowledgebase.ListInput {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	return appknowledgebase.ListInput{Query: c.Query("q"), Sort: c.Query("sort"), IDs: c.QueryArray("id"), Page: page, PageSize: pageSize}
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appknowledgebase.ErrInvalidKnowledgeBase):
		response.ErrorWithCode(c, http.StatusBadRequest, "knowledge_base.invalid")
	case errors.Is(err, appknowledgebase.ErrKnowledgeBaseNotFound), errors.Is(err, appknowledgebase.ErrKnowledgeBaseFileNotFound):
		response.ErrorWithCode(c, http.StatusNotFound, "knowledge_base.not_found")
	case errors.Is(err, appconversation.ErrFileNotFound):
		response.ErrorWithCode(c, http.StatusNotFound, "file.not_found")
	case errors.Is(err, appknowledgebase.ErrKnowledgeBaseConflict):
		response.ErrorWithCode(c, http.StatusConflict, "knowledge_base.conflict")
	case errors.Is(err, appknowledgebase.ErrPlatformFileInUse):
		response.ErrorWithCode(c, http.StatusConflict, "knowledge_base.platform_file_in_use")
	case errors.Is(err, appknowledgebase.ErrKnowledgeBaseFileCleanupUnavailable):
		response.ErrorWithCode(c, http.StatusServiceUnavailable, "knowledge_base.file_cleanup_unavailable")
	default:
		response.ErrorWithCode(c, http.StatusInternalServerError, "knowledge_base.internal")
	}
}

func (h *Handler) audit(c *gin.Context, action string, resourceID string, detail any) {
	h.service.RecordAudit(c.Request.Context(), appknowledgebase.AuditInput{
		UserID: middleware.MustUserID(c), RequestID: middleware.MustRequestID(c), Action: action, ResourceID: resourceID,
		ClientIP: c.ClientIP(), UserAgent: c.Request.UserAgent(), Detail: detail,
	})
}
