package system

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"

// CapabilitiesResponse 是 GET /capabilities 的响应体；契约见 docs/ARCHITECTURE.md §4。
type CapabilitiesResponse struct {
	Features config.Capabilities `json:"features"`
}

// CapabilitiesResponseDoc 仅供 Swagger 描述带信封的响应。
type CapabilitiesResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     CapabilitiesResponse `json:"data"`
}
