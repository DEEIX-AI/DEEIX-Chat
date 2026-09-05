package settings

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errDoclingRuntimeServiceUnavailable   = apperr.New("runtime.docling_unavailable", "docling runtime service unavailable")
	errEmbeddingServiceNotAvailable       = apperr.New("embedding.service_unavailable", "embedding service is not available")
	errInvalidNamespace                   = apperr.New("settings.invalid_namespace", "invalid namespace")
	errMineruRuntimeServiceUnavailable    = apperr.New("runtime.mineru_unavailable", "mineru runtime service unavailable")
	errRapidocrRuntimeServiceUnavailable  = apperr.New("runtime.rapidocr_unavailable", "rapidocr runtime service unavailable")
	errTesseractRuntimeServiceUnavailable = apperr.New("runtime.tesseract_unavailable", "tesseract runtime service unavailable")
	errTikaRuntimeServiceUnavailable      = apperr.New("runtime.tika_unavailable", "tika runtime service unavailable")
)
