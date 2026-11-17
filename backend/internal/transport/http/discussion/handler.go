package discussion

import (
	"net/http"

	appdiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/discussion"
	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *appdiscussion.Service
}

func NewHandler(service *appdiscussion.Service) *Handler {
	return &Handler{service: service}
}

type TemplateResponse struct {
	ID          uint                            `json:"id"`
	PublicID    string                          `json:"publicID"`
	Name        string                          `json:"name"`
	Description string                          `json:"description"`
	Icon        string                          `json:"icon"`
	Status      string                          `json:"status"`
	MaxRounds   int                             `json:"maxRounds"`
	Config      domaindiscussion.TemplateConfig `json:"config"`
	SortOrder   int                             `json:"sortOrder"`
	CreatedAt   string                          `json:"createdAt"`
	UpdatedAt   string                          `json:"updatedAt"`
}

type TemplateListResponse struct {
	Results []TemplateResponse `json:"results"`
}

type TemplateDataResponse struct {
	Data TemplateResponse `json:"data"`
}

type TemplateRequest struct {
	Name        string                          `json:"name" binding:"required,max=128"`
	Description string                          `json:"description,omitempty" binding:"max=500"`
	Icon        string                          `json:"icon,omitempty" binding:"max=128"`
	Status      string                          `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	MaxRounds   int                             `json:"maxRounds,omitempty" binding:"omitempty,min=1,max=10"`
	Config      domaindiscussion.TemplateConfig `json:"config" binding:"required"`
	SortOrder   int                             `json:"sortOrder,omitempty"`
}

func (h *Handler) ListActive(c *gin.Context) {
	items, err := h.service.ListActiveTemplates(c.Request.Context())
	if err != nil {
		response.InternalError(c, err)
		return
	}
	response.Success(c, TemplateListResponse{Results: toTemplateResponses(items)})
}

func (h *Handler) ListAdmin(c *gin.Context) {
	items, err := h.service.ListTemplates(c.Request.Context(), "")
	if err != nil {
		response.InternalError(c, err)
		return
	}
	response.Success(c, TemplateListResponse{Results: toTemplateResponses(items)})
}

func (h *Handler) Create(c *gin.Context) {
	var req TemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateTemplate(c.Request.Context(), toInput(req))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, TemplateDataResponse{Data: toTemplateResponse(*item)})
}

func (h *Handler) Update(c *gin.Context) {
	publicID := c.Param("id")
	var req TemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateTemplate(c.Request.Context(), publicID, toInput(req))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, TemplateDataResponse{Data: toTemplateResponse(*item)})
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.DeleteTemplate(c.Request.Context(), c.Param("id")); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func toInput(req TemplateRequest) appdiscussion.TemplateInput {
	configJSON, _ := marshalConfig(req.Config)
	return appdiscussion.TemplateInput{
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Status:      req.Status,
		MaxRounds:   req.MaxRounds,
		ConfigJSON:  configJSON,
		SortOrder:   req.SortOrder,
	}
}

func toTemplateResponses(items []appdiscussion.TemplateView) []TemplateResponse {
	results := make([]TemplateResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toTemplateResponse(item))
	}
	return results
}

func toTemplateResponse(item appdiscussion.TemplateView) TemplateResponse {
	return TemplateResponse{
		ID:          item.ID,
		PublicID:    item.PublicID,
		Name:        item.Name,
		Description: item.Description,
		Icon:        item.Icon,
		Status:      item.Status,
		MaxRounds:   item.MaxRounds,
		Config:      item.Config,
		SortOrder:   item.SortOrder,
		CreatedAt:   item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
