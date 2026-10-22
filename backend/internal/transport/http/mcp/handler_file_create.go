package mcp

import (
	"errors"
	"net/http"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/mcp"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func (h *Handler) authorizeFileCreate(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", errInvalidMCPServerID)
	if !ok {
		c.Abort()
		return
	}
	authorization := c.GetHeader("Authorization")
	token := strings.TrimPrefix(authorization, "Bearer ")
	if !strings.HasPrefix(authorization, "Bearer ") {
		token = ""
	}
	if h.fileCreate == nil {
		response.ErrorFrom(c, http.StatusUnauthorized, appmcp.ErrFileCreateUnauthorized)
		c.Abort()
		return
	}
	grant, err := h.fileCreate.Authorize(c.Request.Context(), token, serverID)
	if err != nil {
		writeFileCreateError(c, err)
		c.Abort()
		return
	}
	c.Set(middleware.ContextKeyUserID, grant.UserID)
	c.Next()
}

// CreateFile godoc
// @Summary 创建 MCP 工具产物文件
// @Description 仅接受 DEEIX 在 tools/call 签发的一文件 capability，不接受用户 JWT 或 MCP 共享 HMAC 上下文。重复相同文件返回同一 file_id；创建不代表提取或向量化已完成。
// @Tags mcp
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "登记的 MCP 服务 ID"
// @Param Authorization header string true "Bearer <DEEIX-issued file-create token>"
// @Param file formData file true "唯一文件，不接受其他字段"
// @Success 200 {object} FileCreateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 413 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /mcp/servers/{id}/files [post]
func (h *Handler) CreateFile(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", errInvalidMCPServerID)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.fileCreate.MaxUploadBytes()+(1<<20))
	err := c.Request.ParseMultipartForm(8 << 20)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	} //nolint:errcheck
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.ErrorFrom(c, http.StatusRequestEntityTooLarge, appconversation.ErrFileTooLarge)
		} else {
			response.ErrorFrom(c, http.StatusBadRequest, appmcp.ErrFileCreateInvalidInput)
		}
		return
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.Value) != 0 || len(form.File) != 1 || len(form.File["file"]) != 1 || c.Request.URL.RawQuery != "" {
		response.ErrorFrom(c, http.StatusBadRequest, appmcp.ErrFileCreateInvalidInput)
		return
	}
	part := form.File["file"][0]
	reader, err := part.Open()
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, appmcp.ErrFileCreateInvalidInput)
		return
	}
	defer reader.Close() //nolint:errcheck
	result, err := h.fileCreate.Create(c.Request.Context(), strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "), serverID, appupload.TemporaryFileInput{
		FileName: part.Filename, MimeType: part.Header.Get("Content-Type"), DeclaredSize: part.Size, Reader: reader,
	})
	if err != nil {
		writeFileCreateError(c, err)
		return
	}
	file := result.File
	response.Success(c, FileCreateResponse{
		FileID: file.FileID, FileName: file.FileName, SizeBytes: file.SizeBytes, SHA256: file.SHA256,
		Reused: result.Reused, Replayed: result.Replayed, ProcessingStatus: file.ProcessingStatus,
		ExtractStatus: file.ExtractStatus, EmbedStatus: file.EmbedStatus, ProcessingReady: file.ProcessingReady,
	})
}

func writeFileCreateError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, appmcp.ErrFileCreateUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, appmcp.ErrFileCreateConflict), errors.Is(err, appconversation.ErrStorageQuotaExceeded):
		status = http.StatusConflict
	case errors.Is(err, appmcp.ErrFileCreateResultGone):
		status = http.StatusNotFound
	case errors.Is(err, appconversation.ErrFileTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, appconversation.ErrMIMEBlocked), errors.Is(err, appconversation.ErrDangerousMIMEType), errors.Is(err, appconversation.ErrInvalidFileReference), errors.Is(err, appmcp.ErrFileCreateInvalidInput):
		status = http.StatusBadRequest
	}
	response.ErrorFrom(c, status, err)
}
