package extraction

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/builtin"
	doclingextract "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/docling"
	mineruextract "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/mineru"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/ocr"
	tikaextract "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/tika"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
)

// ErrInvalidStoredFilePath 表示存储路径非法。
var ErrInvalidStoredFilePath = errors.New("invalid stored file path")

const defaultStorageRootDir = "./storage"

const (
	EngineBuiltin      = "builtin"
	EngineTika         = "tika"
	EngineDocling      = "docling"
	EngineMinerU       = "mineru"
	defaultEngine      = EngineBuiltin
	TikaSourceExternal = "external"
	TikaSourceManaged  = "managed"
	DefaultTikaBaseURL = tikaextract.DefaultTikaBaseURL
	OCREngineRapidOCR  = "rapidocr"
	OCREngineTesseract = "tesseract"
	OCREnginePaddle    = "paddle"
	OCREngineTencent   = "tencent"
	OCREngineAliyun    = "aliyun"
	OCREngineLLM       = "llm"
	defaultOCREngine   = OCREngineRapidOCR
)

const defaultMinerUFileTypes = "pdf,word,presentation"

// Service 封装文件提取与文本产物读写能力。
type Service struct {
	cfg           *config.Runtime
	storeProvider appstorage.Provider
}

type engine interface {
	Name() string
	Supports(file domainconversation.FileObject) bool
	Extract(ctx context.Context, input ExtractInput) (Result, error)
}

// ExtractInput 表示单个已存储文件的提取输入。
type ExtractInput struct {
	File                  domainconversation.FileObject
	PDFMaxPages           int
	OCREngine             string
	ImageOCREnabled       bool
	PDFOCRFallbackEnabled bool
	PDFOCRPageRanges      []ocr.PageRange
}

// Result 表示提取结果。
type Result struct {
	Text      string
	PageCount int
	Engine    string
	OCRUsed   bool
	OCRPages  []ocr.PageText
}

// NewService 创建提取服务。
func NewService(cfg config.Config) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg))
}

// NewServiceWithRuntime 创建使用运行时配置容器的提取服务。
func NewServiceWithRuntime(cfg *config.Runtime) *Service {
	return &Service{cfg: cfg, storeProvider: appstorage.NewRuntimeProvider(cfg, nil)}
}

// SetObjectStoreProvider 注入对象存储 provider。
func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	if provider != nil {
		s.storeProvider = provider
	}
}

func (s *Service) openObjectStore(ctx context.Context) (objectstore.Store, error) {
	if s.storeProvider == nil {
		s.storeProvider = appstorage.NewRuntimeProvider(s.cfg, nil)
	}
	return s.storeProvider.Open(ctx)
}

// ExtractStoredFile 从已落盘文件中提取文本。
func (s *Service) ExtractStoredFile(ctx context.Context, input ExtractInput) (Result, error) {
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return Result{}, err
	}
	absPath, cleanup, err := store.Materialize(ctx, input.File.StoragePath)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()
	file := input.File
	file.StoragePath = absPath
	input.File = file
	input.OCREngine = normalizeOCREngine(input.OCREngine)

	pageCount := 0
	if input.File.FileCategory == "pdf" {
		pageCount = builtin.DetectPDFPageCount(absPath)
	}
	if input.File.FileCategory == "image" {
		if !input.ImageOCREnabled {
			return Result{Engine: "image_direct"}, fmt.Errorf("image_ocr_disabled")
		}
		result, err := s.extractImageWithOCR(ctx, input)
		return sanitizeExtractResult(result), err
	}

	primary := s.resolvePrimaryEngine()
	if primary != nil && !primary.Supports(input.File) {
		if _, ok := primary.(documentParserEngine); ok {
			primary = builtinEngine{}
		}
	}
	if input.File.FileCategory == "pdf" {
		if _, ok := primary.(builtinEngine); ok {
			result, extractErr := s.extractBuiltinPDF(ctx, input, pageCount)
			return sanitizeExtractResult(result), extractErr
		}
	}
	var pdfPageProbe builtin.PDFTextResult
	var pdfPageProbeErr error
	if input.File.FileCategory == "pdf" && input.PDFOCRFallbackEnabled {
		pdfPageProbe, pdfPageProbeErr = builtin.ExtractPDFPages(absPath, input.PDFMaxPages)
	}
	if primary != nil && primary.Supports(input.File) {
		result, extractErr := primary.Extract(ctx, input)
		result = sanitizeExtractResult(result)
		if result.PageCount == 0 {
			result.PageCount = pageCount
		}
		if input.File.FileCategory == "pdf" && input.PDFOCRFallbackEnabled && pdfPageProbeErr == nil {
			candidatePages := collectPDFOCRCandidatePages(input.File.FileName, pdfPageProbe.Pages)
			if len(candidatePages) > 0 || strings.TrimSpace(result.Text) == "" || extractErr != nil {
				selectiveResult, selectiveErr := s.extractPDFWithSelectiveOCR(ctx, input, pageCount, pdfPageProbe, primaryEngineName(primary))
				selectiveResult = sanitizeExtractResult(selectiveResult)
				if selectiveErr == nil && strings.TrimSpace(selectiveResult.Text) != "" {
					return selectiveResult, nil
				}
				if strings.TrimSpace(result.Text) != "" && extractErr == nil {
					return result, nil
				}
				if selectiveErr != nil {
					return selectiveResult, selectiveErr
				}
			}
		}
		if strings.TrimSpace(result.Text) != "" {
			return result, nil
		}
		if extractErr != nil && input.File.FileCategory != "pdf" {
			return Result{}, extractErr
		}
		if input.File.FileCategory != "pdf" {
			return Result{}, fmt.Errorf("extract_failed")
		}
		if extractErr != nil && !input.PDFOCRFallbackEnabled {
			return Result{PageCount: pageCount, Engine: primaryEngineName(primary)}, extractErr
		}
	}

	if input.File.FileCategory == "pdf" && input.PDFOCRFallbackEnabled {
		result, err := s.extractWithOCRFallback(ctx, input, pageCount)
		result = sanitizeExtractResult(result)
		if err == nil && strings.TrimSpace(result.Text) != "" {
			return result, nil
		}
		if err != nil {
			return result, err
		}
		return Result{PageCount: pageCount, Engine: "pdf_ocr_fallback", OCRUsed: true}, fmt.Errorf("ocr_failed")
	}

	if input.File.FileCategory == "pdf" {
		if primary != nil {
			return Result{PageCount: pageCount, Engine: primaryEngineName(primary)}, fmt.Errorf("pdf_no_extractable_text")
		}
		return Result{PageCount: pageCount, Engine: primaryEngineName(primary)}, fmt.Errorf("extract_failed")
	}
	return Result{}, fmt.Errorf("extract_failed")
}

// WriteExtractedText 将提取结果写入标准文本产物路径。
func (s *Service) WriteExtractedText(ctx context.Context, userID uint, fileID string, text string) (string, error) {
	text = sanitizeExtractedText(text)

	now := time.Now()
	relativePath := filepath.ToSlash(filepath.Join(
		".extracts",
		fmt.Sprintf("uid_%d", userID),
		now.Format("2006"),
		now.Format("01"),
		fileID+".txt",
	))
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return "", err
	}
	if _, err = store.Put(ctx, relativePath, bytes.NewReader([]byte(text)), objectstore.PutOptions{
		SizeBytes:   int64(len([]byte(text))),
		ContentType: "text/plain; charset=utf-8",
	}); err != nil {
		return "", err
	}
	return relativePath, nil
}

// ReadExtractedText 读取标准文本产物。
func (s *Service) ReadExtractedText(ctx context.Context, relativePath string) (string, error) {
	store, err := s.openObjectStore(ctx)
	if err != nil {
		return "", err
	}
	reader, _, err := store.Open(ctx, relativePath)
	if err != nil {
		return "", err
	}
	defer reader.Close() //nolint:errcheck
	data, err := io.ReadAll(io.LimitReader(reader, 50*1024*1024))
	if err != nil {
		return "", err
	}
	return sanitizeExtractedText(string(data)), nil
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{StorageRootDir: defaultStorageRootDir}
	}
	return s.cfg.Snapshot()
}

func (s *Service) resolvePrimaryEngine() engine {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	switch normalizeEngine(snapshot.ExtractEngine) {
	case EngineTika:
		client := tikaextract.New(snapshot)
		if client != nil {
			return tikaEngine{client: client}
		}
		return nil
	case EngineDocling:
		return documentParserEngine{
			name:     EngineDocling,
			supports: supportsPDFDocumentParser,
			extract: func(ctx context.Context, input ExtractInput) (string, error) {
				client := doclingextract.New(doclingextract.ClientConfig{
					BaseURL:        strings.TrimSpace(snapshot.ExtractDoclingBaseURL),
					AuthToken:      snapshot.ExtractDoclingAuthToken,
					TimeoutSeconds: snapshot.ExtractDoclingTimeoutSeconds,
				})
				if client == nil {
					return "", fmt.Errorf("docling_unavailable")
				}
				return client.ExtractText(ctx, doclingextract.Request{
					AbsolutePath: input.File.StoragePath,
					FileName:     input.File.FileName,
					MimeType:     input.File.DetectedMIME,
				})
			},
		}
	case EngineMinerU:
		return documentParserEngine{
			name: EngineMinerU,
			supports: func(file domainconversation.FileObject) bool {
				return supportsMinerUFile(file, snapshot.ExtractMinerUSource, snapshot.ExtractMinerUFileTypes)
			},
			extract: func(ctx context.Context, input ExtractInput) (string, error) {
				client := mineruextract.New(mineruextract.ClientConfig{
					Source:                strings.TrimSpace(snapshot.ExtractMinerUSource),
					BaseURL:               strings.TrimSpace(snapshot.ExtractMinerUBaseURL),
					AuthToken:             snapshot.ExtractMinerUAuthToken,
					TimeoutSeconds:        snapshot.ExtractMinerUTimeoutSeconds,
					Env:                   snapshot.Env,
					SSRFProtectionEnabled: snapshot.SSRFProtectionEnabled,
				})
				if client == nil {
					return "", fmt.Errorf("mineru_unavailable")
				}
				return client.ExtractText(ctx, mineruextract.Request{
					AbsolutePath: input.File.StoragePath,
					FileName:     input.File.FileName,
				})
			},
		}
	default:
		return builtinEngine{}
	}
}

func normalizeEngine(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case EngineDocling:
		return EngineDocling
	case EngineMinerU:
		return EngineMinerU
	case EngineTika:
		return EngineTika
	case EngineBuiltin:
		return EngineBuiltin
	default:
		return defaultEngine
	}
}

func normalizeTikaSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case TikaSourceManaged:
		return TikaSourceManaged
	case TikaSourceExternal:
		return TikaSourceExternal
	default:
		return TikaSourceManaged
	}
}

// NormalizeTikaSourceForRuntime 供其他模块复用 Tika 服务来源的标准化逻辑。
func NormalizeTikaSourceForRuntime(raw string) string {
	return normalizeTikaSource(raw)
}

func supportsPDFDocumentParser(file domainconversation.FileObject) bool {
	return file.FileCategory == "pdf"
}

func supportsMinerUFile(file domainconversation.FileObject, source string, selectedTypes string) bool {
	selected := parseMinerUFileTypes(selectedTypes)
	switch file.FileCategory {
	case "pdf":
		return selected["pdf"]
	case "word":
		if !selected["word"] {
			return false
		}
		format := documentOfficeFormat(file)
		return format == "docx" || (format == "doc" && normalizeMinerUSource(source) == mineruextract.SourceCloud)
	case "presentation":
		if !selected["presentation"] {
			return false
		}
		format := documentOfficeFormat(file)
		return format == "pptx" || (format == "ppt" && normalizeMinerUSource(source) == mineruextract.SourceCloud)
	case "excel":
		if !selected["excel"] {
			return false
		}
		format := documentOfficeFormat(file)
		return format == "xlsx" || (format == "xls" && normalizeMinerUSource(source) == mineruextract.SourceCloud)
	default:
		return false
	}
}

func parseMinerUFileTypes(raw string) map[string]bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = defaultMinerUFileTypes
	}
	result := make(map[string]bool, 4)
	for _, item := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "pdf":
			result["pdf"] = true
		case "word":
			result["word"] = true
		case "presentation":
			result["presentation"] = true
		case "excel":
			result["excel"] = true
		}
	}
	return result
}

func normalizeMinerUSource(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), mineruextract.SourceSelfHosted) {
		return mineruextract.SourceSelfHosted
	}
	return mineruextract.SourceCloud
}

func documentExtension(file domainconversation.FileObject) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(file.FileName)), "."))
}

func documentOfficeFormat(file domainconversation.FileObject) string {
	if ext := documentExtension(file); ext != "" {
		return ext
	}
	mime := strings.ToLower(strings.TrimSpace(file.DetectedMIME))
	switch {
	case strings.Contains(mime, "wordprocessingml"):
		return "docx"
	case strings.Contains(mime, "msword"):
		return "doc"
	case strings.Contains(mime, "presentationml"):
		return "pptx"
	case strings.Contains(mime, "ms-powerpoint"):
		return "ppt"
	case strings.Contains(mime, "spreadsheetml"):
		return "xlsx"
	case strings.Contains(mime, "ms-excel"):
		return "xls"
	default:
		return ""
	}
}

func normalizeOCREngine(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case OCREngineTesseract:
		return OCREngineTesseract
	case OCREnginePaddle:
		return OCREnginePaddle
	case OCREngineTencent:
		return OCREngineTencent
	case OCREngineAliyun:
		return OCREngineAliyun
	case OCREngineLLM:
		return OCREngineLLM
	case OCREngineRapidOCR:
		return OCREngineRapidOCR
	default:
		return defaultOCREngine
	}
}

func sanitizeExtractResult(result Result) Result {
	result.Text = sanitizeExtractedText(result.Text)
	if len(result.OCRPages) > 0 {
		pages := make([]ocr.PageText, 0, len(result.OCRPages))
		for _, page := range result.OCRPages {
			page.Text = sanitizeExtractedText(page.Text)
			pages = append(pages, page)
		}
		result.OCRPages = pages
	}
	return result
}

func sanitizeExtractedText(text string) string {
	if text == "" || !strings.ContainsRune(text, '\x00') {
		return text
	}
	return strings.ReplaceAll(text, "\x00", "")
}

func (s *Service) extractWithOCRFallback(ctx context.Context, input ExtractInput, pageCount int) (Result, error) {
	native, err := builtin.ExtractPDFPages(input.File.StoragePath, input.PDFMaxPages)
	if err != nil {
		return s.extractWithOCRPageRanges(ctx, input, pageCount, nil)
	}
	return s.extractPDFWithSelectiveOCR(ctx, input, pageCount, native, "builtin_pdf")
}

func (s *Service) extractImageWithOCR(ctx context.Context, input ExtractInput) (Result, error) {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}
	item := resolveOCREngine(snapshot, input.OCREngine)
	if !item.Supports(input.File) {
		return Result{Engine: ocrEngineName(item.provider), OCRUsed: true}, errors.New(prefixOCRError(item.provider, "ocr_unavailable"))
	}
	result, err := item.Extract(ctx, input)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(result.Text) == "" {
		return result, errors.New(prefixOCRError(item.provider, "ocr_empty_content"))
	}
	return result, nil
}

func (s *Service) extractWithOCRPageRanges(ctx context.Context, input ExtractInput, pageCount int, ranges []ocr.PageRange) (Result, error) {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}
	item := resolveOCREngine(snapshot, input.OCREngine)
	if !item.Supports(input.File) {
		return Result{PageCount: pageCount, Engine: ocrEngineName(item.provider), OCRUsed: true}, errors.New(prefixOCRError(item.provider, "ocr_unavailable"))
	}
	if len(ranges) == 0 {
		ranges = buildFullPDFPageRanges(pageCount)
	}
	input.PDFOCRPageRanges = ranges
	result, err := item.Extract(ctx, input)
	if result.PageCount == 0 {
		result.PageCount = pageCount
	}
	return result, err
}

func primaryEngineName(item engine) string {
	switch item.(type) {
	case tikaEngine:
		return EngineTika
	case documentParserEngine:
		return item.Name()
	case builtinEngine:
		return EngineBuiltin
	default:
		return EngineBuiltin
	}
}

type builtinEngine struct{}

func (builtinEngine) Name() string {
	return "builtin"
}

func (builtinEngine) Supports(file domainconversation.FileObject) bool {
	switch file.FileCategory {
	case "text", "word", "excel", "pdf":
		return true
	default:
		return false
	}
}

func (builtinEngine) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	switch input.File.FileCategory {
	case "text":
		data, err := os.ReadFile(input.File.StoragePath)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Text:   builtin.ExtractText(data),
			Engine: "builtin_text",
		}, nil
	case "word":
		data, err := os.ReadFile(input.File.StoragePath)
		if err != nil {
			return Result{}, err
		}
		wordResult := builtin.ExtractWordText(ctx, input.File.StoragePath, data, input.File.DetectedMIME, input.File.FileName)
		return Result{
			Text:   wordResult.Text,
			Engine: wordResult.Engine,
		}, nil
	case "excel":
		data, err := os.ReadFile(input.File.StoragePath)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Text:   builtin.ExtractExcelText(data, input.File.DetectedMIME, input.File.FileName),
			Engine: "builtin_excel",
		}, nil
	case "pdf":
		text, pdfErr := builtin.ExtractPDFText(input.File.StoragePath, input.PDFMaxPages)
		return Result{
			Text:      text,
			PageCount: builtin.DetectPDFPageCount(input.File.StoragePath),
			Engine:    "builtin_pdf",
		}, pdfErr
	default:
		return Result{}, fmt.Errorf("extract_failed")
	}
}

type tikaEngine struct {
	client *tikaextract.Client
}

func (e tikaEngine) Name() string {
	return "tika"
}

func (e tikaEngine) Supports(file domainconversation.FileObject) bool {
	if e.client == nil {
		return false
	}
	switch file.FileCategory {
	case "text", "word", "presentation", "excel", "pdf":
		return true
	default:
		return false
	}
}

func (e tikaEngine) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	if e.client == nil {
		return Result{}, fmt.Errorf("tika_disabled")
	}
	text, err := e.client.ExtractText(ctx, tikaextract.Request{
		AbsolutePath: input.File.StoragePath,
		FileName:     input.File.FileName,
		MimeType:     input.File.DetectedMIME,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{
		Text:      text,
		PageCount: 0,
		Engine:    "tika",
	}, nil
}

type documentParserEngine struct {
	name     string
	supports func(file domainconversation.FileObject) bool
	extract  func(ctx context.Context, input ExtractInput) (string, error)
}

func (e documentParserEngine) Name() string {
	return e.name
}

func (e documentParserEngine) Supports(file domainconversation.FileObject) bool {
	if e.extract == nil {
		return false
	}
	if e.supports == nil {
		return supportsPDFDocumentParser(file)
	}
	return e.supports(file)
}

func (e documentParserEngine) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	if e.extract == nil {
		return Result{Engine: e.name}, fmt.Errorf("%s_unavailable", e.name)
	}
	text, err := e.extract(ctx, input)
	if err != nil {
		return Result{Engine: e.name}, err
	}
	return Result{
		Text:   text,
		Engine: e.name,
	}, nil
}

type ocrEngine struct {
	provider string
	client   *ocr.Client
}

func (e ocrEngine) Name() string {
	return ocrEngineName(e.provider)
}

func (e ocrEngine) Supports(file domainconversation.FileObject) bool {
	return e.client != nil && (file.FileCategory == "pdf" || file.FileCategory == "image")
}

func (e ocrEngine) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	provider := normalizeOCREngine(input.OCREngine)
	engineName := ocrEngineName(provider)
	if e.client == nil {
		return Result{Engine: engineName}, errors.New(prefixOCRError(provider, "ocr_unavailable"))
	}
	response, err := e.client.ExtractText(ctx, ocr.Request{
		AbsolutePath: input.File.StoragePath,
		FileName:     input.File.FileName,
		MimeType:     input.File.DetectedMIME,
		PageRanges:   input.PDFOCRPageRanges,
	})
	if err != nil {
		return Result{
			Engine:  engineName,
			OCRUsed: true,
		}, errors.New(prefixOCRError(provider, err.Error()))
	}
	return Result{
		Text:     response.Text,
		Engine:   engineName,
		OCRUsed:  true,
		OCRPages: response.Pages,
	}, nil
}

func resolveOCREngine(snapshot config.Config, mode string) ocrEngine {
	mode = normalizeOCREngine(mode)
	switch mode {
	case OCREngineTesseract:
		return ocrEngine{
			provider: mode,
			client: ocr.NewTesseract(ocr.ClientConfig{
				BaseURL:               strings.TrimSpace(snapshot.ExtractTesseractOCRBaseURL),
				AuthToken:             snapshot.ExtractTesseractOCRAuthToken,
				TimeoutSeconds:        snapshot.ExtractTesseractOCRTimeoutSeconds,
				Env:                   snapshot.Env,
				SSRFProtectionEnabled: snapshot.SSRFProtectionEnabled,
			}),
		}
	case OCREngineRapidOCR:
		return ocrEngine{
			provider: mode,
			client: ocr.NewRapidOCR(ocr.ClientConfig{
				BaseURL:        ocr.ResolveRapidOCRBaseURL(snapshot),
				AuthToken:      snapshot.ExtractRapidOCRAuthToken,
				TimeoutSeconds: snapshot.ExtractRapidOCRTimeoutSeconds,
			}),
		}
	case OCREnginePaddle:
		return ocrEngine{
			provider: mode,
			client: ocr.NewPaddle(ocr.ClientConfig{
				BaseURL:               strings.TrimSpace(snapshot.ExtractPaddleOCRBaseURL),
				AuthToken:             snapshot.ExtractPaddleOCRAuthToken,
				TimeoutSeconds:        snapshot.ExtractPaddleOCRTimeoutSeconds,
				Env:                   snapshot.Env,
				SSRFProtectionEnabled: snapshot.SSRFProtectionEnabled,
			}),
		}
	case OCREngineLLM:
		return ocrEngine{
			provider: mode,
			client: ocr.NewLLM(ocr.ClientConfig{
				BaseURL:               snapshot.ExtractLLMOCRBaseURL,
				AuthToken:             snapshot.ExtractLLMOCRAuthToken,
				Model:                 snapshot.ExtractLLMOCRModel,
				TimeoutSeconds:        snapshot.ExtractLLMOCRTimeoutSeconds,
				Prompt:                snapshot.ExtractLLMOCRPrompt,
				Env:                   snapshot.Env,
				SSRFProtectionEnabled: snapshot.SSRFProtectionEnabled,
			}),
		}
	default:
		return ocrEngine{provider: mode}
	}
}

func ocrEngineName(engine string) string {
	switch normalizeOCREngine(engine) {
	case OCREngineTesseract:
		return "ocr_tesseract"
	case OCREnginePaddle:
		return "ocr_paddle"
	case OCREngineTencent:
		return "ocr_tencent"
	case OCREngineAliyun:
		return "ocr_aliyun"
	case OCREngineLLM:
		return "ocr_llm"
	case OCREngineRapidOCR:
		return "ocr_rapidocr"
	default:
		return "ocr"
	}
}

func prefixOCRError(mode string, raw string) string {
	provider := normalizeOCREngine(mode)
	value := strings.TrimSpace(raw)
	if value == "" {
		return provider + "_ocr_failed"
	}
	if strings.HasPrefix(value, "ocr_") {
		return strings.Replace(value, "ocr_", provider+"_ocr_", 1)
	}
	return provider + "_ocr_failed: " + value
}

func (s *Service) extractBuiltinPDF(ctx context.Context, input ExtractInput, pageCount int) (Result, error) {
	native, err := builtin.ExtractPDFPages(input.File.StoragePath, input.PDFMaxPages)
	if err != nil {
		if input.PDFOCRFallbackEnabled {
			return s.extractWithOCRPageRanges(ctx, input, pageCount, nil)
		}
		return Result{PageCount: pageCount, Engine: "builtin_pdf"}, err
	}
	return s.extractPDFWithSelectiveOCR(ctx, input, pageCount, native, "builtin_pdf")
}

func (s *Service) extractPDFWithSelectiveOCR(
	ctx context.Context,
	input ExtractInput,
	pageCount int,
	native builtin.PDFTextResult,
	nativeEngineName string,
) (Result, error) {
	if native.PageCount > 0 {
		pageCount = native.PageCount
	}

	nativeText := joinBuiltinPDFPages(native.Pages, nil)
	if !input.PDFOCRFallbackEnabled {
		if strings.TrimSpace(nativeText) != "" {
			return Result{
				Text:      nativeText,
				PageCount: pageCount,
				Engine:    nativeEngineName,
			}, nil
		}
		return Result{PageCount: pageCount, Engine: nativeEngineName}, fmt.Errorf("pdf_no_extractable_text")
	}

	candidatePages := collectPDFOCRCandidatePages(input.File.FileName, native.Pages)
	if len(candidatePages) == 0 {
		if strings.TrimSpace(nativeText) != "" {
			return Result{
				Text:      nativeText,
				PageCount: pageCount,
				Engine:    nativeEngineName,
			}, nil
		}
		return Result{PageCount: pageCount, Engine: nativeEngineName}, fmt.Errorf("pdf_no_extractable_text")
	}

	ocrResult, err := s.extractWithOCRPageRanges(ctx, input, pageCount, compactPageNumbersToRanges(candidatePages))
	if err != nil {
		return ocrResult, err
	}

	ocrPages := indexOCRPages(ocrResult.OCRPages)
	if len(ocrPages) == 0 {
		if len(candidatePages) == len(native.Pages) && strings.TrimSpace(ocrResult.Text) != "" {
			return Result{
				Text:      strings.TrimSpace(ocrResult.Text),
				PageCount: pageCount,
				Engine:    ocrResult.Engine,
				OCRUsed:   true,
			}, nil
		}
		return Result{
			PageCount: pageCount,
			Engine:    ocrResult.Engine,
			OCRUsed:   true,
		}, errors.New(prefixOCRError(input.OCREngine, "ocr_invalid_response"))
	}

	merged := joinBuiltinPDFPages(native.Pages, ocrPages)
	if strings.TrimSpace(merged) == "" {
		return Result{
			PageCount: pageCount,
			Engine:    ocrResult.Engine,
			OCRUsed:   true,
		}, fmt.Errorf("extract_failed")
	}
	return Result{
		Text:      merged,
		PageCount: pageCount,
		Engine:    ocrResult.Engine,
		OCRUsed:   true,
		OCRPages:  ocrResult.OCRPages,
	}, nil
}

func collectPDFOCRCandidatePages(fileName string, pages []builtin.PDFTextPage) []int {
	candidates := make([]int, 0)
	for _, page := range pages {
		if page.ExtractFailed || shouldOCRPDFPage(fileName, page.Text) {
			candidates = append(candidates, page.PageNumber)
		}
	}
	return candidates
}

func shouldOCRPDFPage(fileName string, text string) bool {
	value := strings.TrimSpace(text)
	if value == "" {
		return true
	}
	meaningfulChars := countMeaningfulPDFChars(value)
	if meaningfulChars < 24 {
		return true
	}
	if looksLikeGarbledChinesePDFText(fileName, value, meaningfulChars) {
		return true
	}
	return looksLikeMojibakePDFText(value, meaningfulChars)
}

func countMeaningfulPDFChars(text string) int {
	count := 0
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			count++
		}
	}
	return count
}

func looksLikeGarbledChinesePDFText(fileName string, text string, meaningfulChars int) bool {
	if !containsHan(fileName) || meaningfulChars <= 0 {
		return false
	}

	var hanCount int
	var latinDigitCount int
	var mojibakeCount int
	var nonASCIILetterCount int
	var replacementCount int
	var privateUseCount int
	var symbolCount int
	var whitespaceCount int

	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			hanCount++
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			latinDigitCount++
			if r > unicode.MaxASCII && unicode.IsLetter(r) {
				nonASCIILetterCount++
			}
			if isLikelyMojibakeRune(r) {
				mojibakeCount++
			}
		case r == unicode.ReplacementChar:
			replacementCount++
		case isPrivateUseRune(r):
			privateUseCount++
		case unicode.IsSpace(r):
			whitespaceCount++
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbolCount++
			if isLikelyMojibakeRune(r) {
				mojibakeCount++
			}
		}
	}

	if replacementCount > 0 || privateUseCount > 0 {
		return true
	}
	if hanCount*10 >= meaningfulChars*2 {
		return false
	}

	// 中文命名文档若解析出高密度 ASCII/符号文本，通常是缺少字体到 Unicode 的映射，而不是有效正文。
	latinDense := latinDigitCount*10 >= meaningfulChars*8
	tooFewSpaces := whitespaceCount*20 <= meaningfulChars
	symbolHeavy := symbolCount*10 >= meaningfulChars*3
	mojibakeHeavy := mojibakeCount*10 >= meaningfulChars
	nonASCIIHeavy := nonASCIILetterCount*10 >= meaningfulChars*3
	return (latinDense || mojibakeHeavy || nonASCIIHeavy) && (tooFewSpaces || symbolHeavy || mojibakeHeavy)
}

func looksLikeMojibakePDFText(text string, meaningfulChars int) bool {
	if meaningfulChars <= 0 {
		return false
	}

	var hanCount int
	var mojibakeCount int
	var nonASCIILetterCount int
	var symbolCount int
	var whitespaceCount int
	var replacementCount int
	var privateUseCount int

	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			hanCount++
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r > unicode.MaxASCII && unicode.IsLetter(r) {
				nonASCIILetterCount++
			}
			if isLikelyMojibakeRune(r) {
				mojibakeCount++
			}
		case r == unicode.ReplacementChar:
			replacementCount++
		case isPrivateUseRune(r):
			privateUseCount++
		case unicode.IsSpace(r):
			whitespaceCount++
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbolCount++
			if isLikelyMojibakeRune(r) {
				mojibakeCount++
			}
		}
	}

	if replacementCount > 0 || privateUseCount > 0 {
		return true
	}
	if hanCount*10 >= meaningfulChars*3 {
		return false
	}

	tooFewSpaces := whitespaceCount*20 <= meaningfulChars
	symbolHeavy := symbolCount*10 >= meaningfulChars*3
	mojibakeHeavy := mojibakeCount*10 >= meaningfulChars
	nonASCIIHeavy := nonASCIILetterCount*10 >= meaningfulChars*4
	return (mojibakeHeavy && (tooFewSpaces || symbolHeavy)) || (nonASCIIHeavy && tooFewSpaces && symbolHeavy)
}

func isLikelyMojibakeRune(r rune) bool {
	switch r {
	case 'Ã', 'Â', 'Ä', 'Å', 'Æ', 'Ç', 'Ð', 'Ñ', 'Ø', 'Ù', 'Þ', 'ß',
		'à', 'á', 'â', 'ã', 'ä', 'å', 'æ', 'ç', 'è', 'é', 'ê', 'ë',
		'ì', 'í', 'î', 'ï', 'ð', 'ñ', 'ò', 'ó', 'ô', 'õ', 'ö', 'ø',
		'ù', 'ú', 'û', 'ü', 'ý', 'þ', 'ÿ', 'Œ', 'œ', 'Š', 'š', 'Ž',
		'ž', '€', '™':
		return true
	default:
		return false
	}
}

func containsHan(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func isPrivateUseRune(r rune) bool {
	switch {
	case r >= 0xE000 && r <= 0xF8FF:
		return true
	case r >= 0xF0000 && r <= 0xFFFFD:
		return true
	case r >= 0x100000 && r <= 0x10FFFD:
		return true
	default:
		return false
	}
}

func compactPageNumbersToRanges(pageNumbers []int) []ocr.PageRange {
	if len(pageNumbers) == 0 {
		return nil
	}
	ranges := make([]ocr.PageRange, 0)
	start := pageNumbers[0]
	end := start
	for _, pageNumber := range pageNumbers[1:] {
		if pageNumber == end+1 {
			end = pageNumber
			continue
		}
		ranges = append(ranges, ocr.PageRange{Start: start, End: end})
		start = pageNumber
		end = pageNumber
	}
	ranges = append(ranges, ocr.PageRange{Start: start, End: end})
	return ranges
}

func buildFullPDFPageRanges(pageCount int) []ocr.PageRange {
	if pageCount <= 0 {
		return nil
	}
	return []ocr.PageRange{{Start: 1, End: pageCount}}
}

func indexOCRPages(pages []ocr.PageText) map[int]string {
	result := make(map[int]string, len(pages))
	for _, page := range pages {
		if page.PageNumber <= 0 {
			continue
		}
		if value := strings.TrimSpace(page.Text); value != "" {
			result[page.PageNumber] = value
		}
	}
	return result
}

func joinBuiltinPDFPages(nativePages []builtin.PDFTextPage, ocrPages map[int]string) string {
	parts := make([]string, 0, len(nativePages))
	for _, page := range nativePages {
		value := strings.TrimSpace(page.Text)
		if ocrPages != nil {
			if ocrText, ok := ocrPages[page.PageNumber]; ok && strings.TrimSpace(ocrText) != "" {
				value = strings.TrimSpace(ocrText)
			}
		}
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n")
}
