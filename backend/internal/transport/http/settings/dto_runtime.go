package settings

import (
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/runtime"
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

type EmbeddingIndexStatusResponse struct {
	ModelSignature string `json:"modelSignature"`
	ReadyCount     int64  `json:"readyCount"`
	StaleCount     int64  `json:"staleCount"`
	PendingCount   int64  `json:"pendingCount"`
	FailedCount    int64  `json:"failedCount"`
	NeedsReindex   bool   `json:"needsReindex"`
}

type EmbeddingReindexResponse struct {
	Submitted int    `json:"submitted"`
	Message   string `json:"message"`
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
		ModelSignature: status.ModelSignature,
		ReadyCount:     status.ReadyCount,
		StaleCount:     status.StaleCount,
		PendingCount:   status.PendingCount,
		FailedCount:    status.FailedCount,
		NeedsReindex:   status.NeedsReindex,
	}
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
