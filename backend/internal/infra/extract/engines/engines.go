// Package engines 将运行时配置映射为各文本抽取引擎客户端，
// 由组合根注册到 application/extraction 的引擎工厂端口。
package engines

import (
	"context"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/builtin"
	docling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/docling"
	mineru "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/mineru"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/ocr"
	tika "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/tika"
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
)

// NewTika 创建 Tika 客户端；未配置地址时返回 nil。
func NewTika(cfg config.Config) *tika.Client {
	return tika.New(cfg)
}

// NewDocling 创建 Docling 客户端；未配置地址时返回 nil。
func NewDocling(cfg config.Config) *docling.Client {
	return docling.New(docling.ClientConfig{
		BaseURL:        strings.TrimSpace(cfg.ExtractDoclingBaseURL),
		AuthToken:      cfg.ExtractDoclingAuthToken,
		TimeoutSeconds: cfg.ExtractDoclingTimeoutSeconds,
	})
}

// NewMinerU 创建 MinerU 客户端；配置不完整时返回 nil。
func NewMinerU(cfg config.Config) *mineru.Client {
	return mineru.New(mineru.ClientConfig{
		Source:         strings.TrimSpace(cfg.ExtractMinerUSource),
		BaseURL:        strings.TrimSpace(cfg.ExtractMinerUBaseURL),
		AuthToken:      cfg.ExtractMinerUAuthToken,
		TimeoutSeconds: cfg.ExtractMinerUTimeoutSeconds,
		OutboundPolicy: cfg.StrictOutboundPolicy(),
	})
}

// NewOCR 按提供方创建 OCR 客户端；未知提供方返回 nil。
func NewOCR(provider string, cfg config.Config) *ocr.Client {
	switch provider {
	case extractport.OCREngineTesseract:
		return ocr.NewTesseract(ocr.ClientConfig{
			BaseURL:        strings.TrimSpace(cfg.ExtractTesseractOCRBaseURL),
			AuthToken:      cfg.ExtractTesseractOCRAuthToken,
			TimeoutSeconds: cfg.ExtractTesseractOCRTimeoutSeconds,
		})
	case extractport.OCREngineRapidOCR:
		return ocr.NewRapidOCR(ocr.ClientConfig{
			BaseURL:        ocr.ResolveRapidOCRBaseURL(cfg),
			AuthToken:      cfg.ExtractRapidOCRAuthToken,
			TimeoutSeconds: cfg.ExtractRapidOCRTimeoutSeconds,
		})
	case extractport.OCREnginePaddle:
		return ocr.NewPaddle(ocr.ClientConfig{
			BaseURL:        strings.TrimSpace(cfg.ExtractPaddleOCRBaseURL),
			AuthToken:      cfg.ExtractPaddleOCRAuthToken,
			TimeoutSeconds: cfg.ExtractPaddleOCRTimeoutSeconds,
		})
	case extractport.OCREngineMistral:
		return ocr.NewMistral(ocr.ClientConfig{
			BaseURL:        cfg.ExtractMistralOCRBaseURL,
			AuthToken:      cfg.ExtractMistralOCRAuthToken,
			Model:          cfg.ExtractMistralOCRModel,
			TimeoutSeconds: cfg.ExtractMistralOCRTimeoutSeconds,
			OutboundPolicy: cfg.TrustedOutboundPolicy(),
		})
	case extractport.OCREngineLLM:
		return ocr.NewLLM(ocr.ClientConfig{
			BaseURL:        cfg.ExtractLLMOCRBaseURL,
			AuthToken:      cfg.ExtractLLMOCRAuthToken,
			Model:          cfg.ExtractLLMOCRModel,
			TimeoutSeconds: cfg.ExtractLLMOCRTimeoutSeconds,
			Prompt:         cfg.ExtractLLMOCRPrompt,
			OutboundPolicy: cfg.TrustedOutboundPolicy(),
		})
	default:
		return nil
	}
}

// Builtin 是内置本地解析器（文本/Word/Excel/PDF）的端口实现。
type Builtin struct{}

// ExtractText 提取纯文本内容。
func (Builtin) ExtractText(data []byte) string {
	return builtin.ExtractText(data)
}

// ExtractWordText 提取 Word 文档文本。
func (Builtin) ExtractWordText(ctx context.Context, absolutePath string, data []byte, mimeType string, fileName string) extractport.WordTextResult {
	return builtin.ExtractWordText(ctx, absolutePath, data, mimeType, fileName)
}

// ExtractExcelText 提取 Excel 表格文本。
func (Builtin) ExtractExcelText(data []byte, mimeType string, fileName string) string {
	return builtin.ExtractExcelText(data, mimeType, fileName)
}

// ExtractPDFText 提取 PDF 纯文本。
func (Builtin) ExtractPDFText(absolutePath string, maxPages int) (string, error) {
	return builtin.ExtractPDFText(absolutePath, maxPages)
}

// ExtractPDFPages 按页提取 PDF 原生文本。
func (Builtin) ExtractPDFPages(absolutePath string, maxPages int) (extractport.PDFTextResult, error) {
	return builtin.ExtractPDFPages(absolutePath, maxPages)
}

// DetectPDFPageCount 探测 PDF 页数。
func (Builtin) DetectPDFPageCount(absolutePath string) int {
	return builtin.DetectPDFPageCount(absolutePath)
}
