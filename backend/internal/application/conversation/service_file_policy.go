package conversation

import (
	"context"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

const (
	fileCategoryImage        = "image"
	fileCategoryVideo        = "video"
	fileCategoryPDF          = "pdf"
	fileCategoryWord         = "word"
	fileCategoryPresentation = "presentation"
	fileCategoryExcel        = "excel"
	fileCategoryText         = "text"
	fileCategoryUnknown      = "unknown"
)

func normalizeDetectedMIME(detected string, fileName string) string {
	value := normalizeMIMEValue(detected)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(fileName)), "."))
	if isActiveFileExtension(ext) || isActiveUploadMIME(value) {
		return "text/plain"
	}
	switch ext {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "doc":
		return "application/msword"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "xls":
		return "application/vnd.ms-excel"
	case "csv":
		return "text/csv"
	case "md", "markdown":
		return "text/markdown"
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "text/yaml"
	case "toml":
		return "application/toml"
	}
	if ext != "" && isTextMIMEForEmbed("", "sample."+ext) {
		return "text/plain"
	}
	if value == "application/zip" {
		switch ext {
		case "docx":
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		case "pptx":
			return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		case "xlsx":
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
	}
	return value
}

func normalizeMIMEValue(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.Index(value, ";"); idx > 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func isActiveFileExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "html", "htm", "css", "js", "jsx", "mjs", "ts", "tsx", "xml", "xhtml", "svg":
		return true
	default:
		return false
	}
}

func isActiveUploadMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "text/html",
		"text/css",
		"text/javascript",
		"text/xml",
		"application/javascript",
		"application/ecmascript",
		"application/x-javascript",
		"application/typescript",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml":
		return true
	default:
		return false
	}
}

func detectContentMIME(header []byte, declared string, fileName string) string {
	if len(header) == 0 {
		return normalizeDetectedMIME(declared, fileName)
	}
	return normalizeDetectedMIME(http.DetectContentType(header), fileName)
}

func inferFileCategory(mimeType string, fileName string) string {
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(fileName)), "."))
	switch {
	case strings.HasPrefix(mime, "image/"):
		return fileCategoryImage
	case strings.HasPrefix(mime, "video/"):
		return fileCategoryVideo
	case mime == "application/pdf" || ext == "pdf":
		return fileCategoryPDF
	case strings.Contains(mime, "wordprocessingml") || strings.Contains(mime, "msword") || ext == "docx" || ext == "doc":
		return fileCategoryWord
	case strings.Contains(mime, "presentationml") || strings.Contains(mime, "ms-powerpoint") || ext == "pptx" || ext == "ppt":
		return fileCategoryPresentation
	case strings.Contains(mime, "spreadsheetml") || strings.Contains(mime, "ms-excel") || mime == "text/csv" || ext == "xlsx" || ext == "xls" || ext == "csv":
		return fileCategoryExcel
	case isTextMIMEForEmbed(mime, fileName):
		return fileCategoryText
	default:
		return fileCategoryUnknown
	}
}

func parseAllowedMIMETypes(raw string) map[string]struct{} {
	items := strings.Split(raw, ",")
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		result[value] = struct{}{}
	}
	return result
}

func isAllowedMIME(mimeType string, cfg config.Config) bool {
	allowed := parseAllowedMIMETypes(cfg.FileAllowedMIMETypes)
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[strings.ToLower(strings.TrimSpace(mimeType))]
	return ok
}

func maxBytesForCategory(category string, cfg config.Config) int64 {
	if category == fileCategoryImage {
		return cfg.FileImageMaxBytes
	}
	if category == fileCategoryVideo {
		return 0
	}
	return cfg.FileDocMaxBytes
}

func supportsInlineExtraction(category string) bool {
	return category == fileCategoryText
}

func supportsExtraction(category string) bool {
	switch category {
	case fileCategoryPDF, fileCategoryWord, fileCategoryPresentation, fileCategoryExcel, fileCategoryText:
		return true
	default:
		return false
	}
}

func supportsRAG(category string) bool {
	switch category {
	case fileCategoryPDF, fileCategoryWord, fileCategoryPresentation, fileCategoryExcel, fileCategoryText, fileCategoryImage:
		return true
	default:
		return false
	}
}

type chatFileCapability struct {
	RAGAvailable           bool
	RAGAvailabilityReason  string
	CapabilityMode         string
	EffectiveImageMaxBytes int64
	EffectiveDocMaxBytes   int64
}

func minPositiveInt64(values ...int64) int64 {
	var result int64
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if result == 0 || value < result {
			result = value
		}
	}
	return result
}

func sortedAllowedMIMETypes(raw string) []string {
	result := make([]string, 0)
	for value := range parseAllowedMIMETypes(raw) {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *Service) resolveChatFileCapability(ctx context.Context) chatFileCapability {
	cfg := s.cfg.Snapshot()
	capability := chatFileCapability{
		EffectiveImageMaxBytes: minPositiveInt64(cfg.MaxUploadFileBytes, cfg.FileImageMaxBytes),
		EffectiveDocMaxBytes:   minPositiveInt64(cfg.MaxUploadFileBytes, cfg.FileDocMaxBytes),
	}

	ragAvailable, reason := s.embeddingSvc.Available(ctx)
	capability.RAGAvailable = ragAvailable
	capability.RAGAvailabilityReason = reason
	capability.CapabilityMode = "full_context_and_rag"
	if !ragAvailable {
		capability.CapabilityMode = "full_context_only"
	}
	return capability
}
