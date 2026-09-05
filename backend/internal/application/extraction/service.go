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
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstore"
)

// ErrInvalidStoredFilePath 表示存储路径非法。
var ErrInvalidStoredFilePath = errors.New("invalid stored file path")

const (
	EngineBuiltin      = "builtin"
	EngineTika         = "tika"
	EngineDocling      = "docling"
	EngineMinerU       = "mineru"
	defaultEngine      = EngineBuiltin
	TikaSourceExternal = "external"
	TikaSourceManaged  = "managed"
	DefaultTikaBaseURL = extractport.DefaultTikaBaseURL
	OCREngineRapidOCR  = extractport.OCREngineRapidOCR
	OCREngineTesseract = extractport.OCREngineTesseract
	OCREnginePaddle    = extractport.OCREnginePaddle
	OCREngineTencent   = extractport.OCREngineTencent
	OCREngineAliyun    = extractport.OCREngineAliyun
	OCREngineMistral   = extractport.OCREngineMistral
	OCREngineLLM       = extractport.OCREngineLLM
	defaultOCREngine   = OCREngineRapidOCR
)

const defaultMinerUFileTypes = "pdf,word,presentation"

// Service 封装文件提取与文本产物读写能力。
type Service struct {
	cfg           *config.Runtime
	storeProvider appstorage.Provider
	factories     EngineFactories
}

type engine interface {
	Name() string
	Supports(file domainconversation.FileObject) bool
	Extract(ctx context.Context, input ExtractInput) (Result, error)
}

// DocumentExtractor 定义文档抽取引擎（Tika/Docling/MinerU）的调用端口。
type DocumentExtractor interface {
	ExtractText(ctx context.Context, req extractport.DocumentRequest) (string, error)
}

// OCRExtractor 定义 OCR 引擎的调用端口。
type OCRExtractor interface {
	ExtractText(ctx context.Context, req extractport.OCRRequest) (extractport.OCRResponse, error)
}

// BuiltinParser 定义内置本地解析（文本/Word/Excel/PDF）的调用端口。
type BuiltinParser interface {
	ExtractText(data []byte) string
	ExtractWordText(ctx context.Context, absolutePath string, data []byte, mimeType string, fileName string) extractport.WordTextResult
	ExtractExcelText(data []byte, mimeType string, fileName string) string
	ExtractPDFText(absolutePath string, maxPages int) (string, error)
	ExtractPDFPages(absolutePath string, maxPages int) (extractport.PDFTextResult, error)
	DetectPDFPageCount(absolutePath string) int
}

// EngineFactories 由组合根注入，按配置创建各抽取引擎客户端。
// 工厂在引擎不可用（未配置、提供方未知）时必须返回 nil 接口。
type EngineFactories struct {
	NewTika    func(cfg config.Config) DocumentExtractor
	NewDocling func(cfg config.Config) DocumentExtractor
	NewMinerU  func(cfg config.Config) DocumentExtractor
	NewOCR     func(provider string, cfg config.Config) OCRExtractor
	Builtin    BuiltinParser
}

// ExtractInput 表示单个已存储文件的提取输入。
type ExtractInput struct {
	File                  domainconversation.FileObject
	PDFMaxPages           int
	OCREngine             string
	ImageOCREnabled       bool
	PDFOCRFallbackEnabled bool
	PDFOCRPageRanges      []extractport.PageRange
}

// Result 表示提取结果。
type Result struct {
	Text      string
	PageCount int
	Engine    string
	OCRUsed   bool
	OCRPages  []extractport.PageText
}

// NewServiceWithRuntime 创建使用运行时配置容器和显式引擎工厂的提取服务。
func NewServiceWithRuntime(cfg *config.Runtime, factories EngineFactories) *Service {
	return &Service{
		cfg:       cfg,
		factories: factories,
	}
}

// SetObjectStoreProvider 注入对象存储 provider。
func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	if provider != nil {
		s.storeProvider = provider
	}
}

func (s *Service) openObjectStore(ctx context.Context) (objectstore.Store, error) {
	if s == nil || s.storeProvider == nil {
		return nil, appstorage.ErrProviderNotConfigured
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
	return s.extractLocalFile(ctx, input, absPath)
}

// ExtractTemporaryFile 从系统临时目录内、由调用方管理生命周期的普通文件中提取文本。
// 该入口不读取或写入对象存储，也不接受任意本地文件路径。
func (s *Service) ExtractTemporaryFile(ctx context.Context, input ExtractInput) (Result, error) {
	absPath := filepath.Clean(strings.TrimSpace(input.File.StoragePath))
	if absPath == "" || !filepath.IsAbs(absPath) {
		return Result{}, ErrInvalidStoredFilePath
	}
	temporaryRoot := filepath.Clean(os.TempDir())
	relativePath, err := filepath.Rel(temporaryRoot, absPath)
	if err != nil || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return Result{}, ErrInvalidStoredFilePath
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, ErrInvalidStoredFilePath
	}
	return s.extractLocalFile(ctx, input, absPath)
}

func (s *Service) extractLocalFile(ctx context.Context, input ExtractInput, absPath string) (result Result, err error) {
	defer func() {
		err = withErrorCode(err)
	}()
	file := input.File
	file.StoragePath = absPath
	input.File = file
	input.OCREngine = normalizeOCREngine(input.OCREngine)

	pageCount := 0
	if input.File.FileCategory == "pdf" {
		pageCount = s.detectPDFPageCount(absPath)
	}
	if input.File.FileCategory == "image" {
		if !input.ImageOCREnabled {
			return Result{Engine: "image_direct"}, NewError("image_ocr_disabled", errors.New("image OCR is disabled"))
		}
		result, err := s.extractImageWithOCR(ctx, input)
		return sanitizeExtractResult(result), err
	}

	primary := s.resolvePrimaryEngine()
	if primary != nil && !primary.Supports(input.File) {
		if _, ok := primary.(documentParserEngine); ok {
			primary = builtinEngine{parser: s.factories.Builtin}
		}
	}
	if input.File.FileCategory == "pdf" {
		if _, ok := primary.(builtinEngine); ok {
			result, extractErr := s.extractBuiltinPDF(ctx, input, pageCount)
			return sanitizeExtractResult(result), extractErr
		}
	}
	var pdfPageProbe extractport.PDFTextResult
	var pdfPageProbeErr error
	if input.File.FileCategory == "pdf" && input.PDFOCRFallbackEnabled {
		pdfPageProbe, pdfPageProbeErr = s.extractPDFPagesNative(absPath, input.PDFMaxPages)
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
			return Result{}, NewError("extract_failed", errors.New("extraction failed"))
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
		return Result{PageCount: pageCount, Engine: "pdf_ocr_fallback", OCRUsed: true}, NewError("ocr_failed", errors.New("OCR failed"))
	}

	if input.File.FileCategory == "pdf" {
		if primary != nil {
			return Result{PageCount: pageCount, Engine: primaryEngineName(primary)}, NewError("pdf_no_extractable_text", errors.New("PDF has no extractable text"))
		}
		return Result{PageCount: pageCount, Engine: primaryEngineName(primary)}, NewError("extract_failed", errors.New("extraction failed"))
	}
	return Result{}, NewError("extract_failed", errors.New("extraction failed"))
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

func (s *Service) resolvePrimaryEngine() engine {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	switch normalizeEngine(snapshot.ExtractEngine) {
	case EngineTika:
		if s.factories.NewTika == nil {
			return nil
		}
		client := s.factories.NewTika(snapshot)
		if client != nil {
			return tikaEngine{client: client}
		}
		return nil
	case EngineDocling:
		return documentParserEngine{
			name:     EngineDocling,
			supports: supportsPDFDocumentParser,
			extract: func(ctx context.Context, input ExtractInput) (string, error) {
				client := resolveDocumentExtractor(s.factories.NewDocling, snapshot)
				if client == nil {
					return "", fmt.Errorf("docling_unavailable")
				}
				return client.ExtractText(ctx, extractport.DocumentRequest{
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
				client := resolveDocumentExtractor(s.factories.NewMinerU, snapshot)
				if client == nil {
					return "", fmt.Errorf("mineru_unavailable")
				}
				return client.ExtractText(ctx, extractport.DocumentRequest{
					AbsolutePath: input.File.StoragePath,
					FileName:     input.File.FileName,
				})
			},
		}
	default:
		return builtinEngine{parser: s.factories.Builtin}
	}
}

func resolveDocumentExtractor(factory func(cfg config.Config) DocumentExtractor, snapshot config.Config) DocumentExtractor {
	if factory == nil {
		return nil
	}
	return factory(snapshot)
}

func (s *Service) detectPDFPageCount(absPath string) int {
	if s.factories.Builtin == nil {
		return 0
	}
	return s.factories.Builtin.DetectPDFPageCount(absPath)
}

func (s *Service) extractPDFPagesNative(absPath string, maxPages int) (extractport.PDFTextResult, error) {
	if s.factories.Builtin == nil {
		return extractport.PDFTextResult{}, errors.New("builtin_unavailable")
	}
	return s.factories.Builtin.ExtractPDFPages(absPath, maxPages)
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
		return format == "docx" || (format == "doc" && normalizeMinerUSource(source) == extractport.MinerUSourceCloud)
	case "presentation":
		if !selected["presentation"] {
			return false
		}
		format := documentOfficeFormat(file)
		return format == "pptx" || (format == "ppt" && normalizeMinerUSource(source) == extractport.MinerUSourceCloud)
	case "excel":
		if !selected["excel"] {
			return false
		}
		format := documentOfficeFormat(file)
		return format == "xlsx" || (format == "xls" && normalizeMinerUSource(source) == extractport.MinerUSourceCloud)
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
	if strings.EqualFold(strings.TrimSpace(raw), extractport.MinerUSourceSelfHosted) {
		return extractport.MinerUSourceSelfHosted
	}
	return extractport.MinerUSourceCloud
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
	case OCREngineMistral:
		return OCREngineMistral
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
		pages := make([]extractport.PageText, 0, len(result.OCRPages))
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
	native, err := s.extractPDFPagesNative(input.File.StoragePath, input.PDFMaxPages)
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
	item := s.resolveOCREngine(snapshot, input.OCREngine)
	if !item.Supports(input.File) {
		return Result{Engine: ocrEngineName(item.provider), OCRUsed: true}, NewOCRError(item.provider, "ocr_unavailable", nil)
	}
	result, err := item.Extract(ctx, input)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(result.Text) == "" {
		return result, NewOCRError(item.provider, "ocr_empty_content", nil)
	}
	return result, nil
}

func (s *Service) extractWithOCRPageRanges(ctx context.Context, input ExtractInput, pageCount int, ranges []extractport.PageRange) (Result, error) {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}
	item := s.resolveOCREngine(snapshot, input.OCREngine)
	if !item.Supports(input.File) {
		return Result{PageCount: pageCount, Engine: ocrEngineName(item.provider), OCRUsed: true}, NewOCRError(item.provider, "ocr_unavailable", nil)
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

type builtinEngine struct {
	parser BuiltinParser
}

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

func (e builtinEngine) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	parser := e.parser
	if parser == nil {
		return Result{}, errors.New("builtin_unavailable")
	}
	switch input.File.FileCategory {
	case "text":
		data, err := os.ReadFile(input.File.StoragePath)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Text:   parser.ExtractText(data),
			Engine: "builtin_text",
		}, nil
	case "word":
		data, err := os.ReadFile(input.File.StoragePath)
		if err != nil {
			return Result{}, err
		}
		wordResult := parser.ExtractWordText(ctx, input.File.StoragePath, data, input.File.DetectedMIME, input.File.FileName)
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
			Text:   parser.ExtractExcelText(data, input.File.DetectedMIME, input.File.FileName),
			Engine: "builtin_excel",
		}, nil
	case "pdf":
		text, pdfErr := parser.ExtractPDFText(input.File.StoragePath, input.PDFMaxPages)
		return Result{
			Text:      text,
			PageCount: parser.DetectPDFPageCount(input.File.StoragePath),
			Engine:    "builtin_pdf",
		}, pdfErr
	default:
		return Result{}, NewError("extract_failed", errors.New("extraction failed"))
	}
}

type tikaEngine struct {
	client DocumentExtractor
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
		return Result{}, NewError("tika_disabled", errors.New("tika is disabled"))
	}
	text, err := e.client.ExtractText(ctx, extractport.DocumentRequest{
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
	client   OCRExtractor
}

func (e ocrEngine) Supports(file domainconversation.FileObject) bool {
	return e.client != nil && (file.FileCategory == "pdf" || file.FileCategory == "image")
}

func (e ocrEngine) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	provider := normalizeOCREngine(input.OCREngine)
	engineName := ocrEngineName(provider)
	if e.client == nil {
		return Result{Engine: engineName}, NewOCRError(provider, "ocr_unavailable", nil)
	}
	response, err := e.client.ExtractText(ctx, extractport.OCRRequest{
		AbsolutePath: input.File.StoragePath,
		FileName:     input.File.FileName,
		MimeType:     input.File.DetectedMIME,
		PageRanges:   input.PDFOCRPageRanges,
	})
	if err != nil {
		return Result{
			Engine:  engineName,
			OCRUsed: true,
		}, NewOCRError(provider, "ocr_failed", err)
	}
	return Result{
		Text:     response.Text,
		Engine:   engineName,
		OCRUsed:  true,
		OCRPages: response.Pages,
	}, nil
}

func (s *Service) resolveOCREngine(snapshot config.Config, mode string) ocrEngine {
	mode = normalizeOCREngine(mode)
	if s.factories.NewOCR == nil {
		return ocrEngine{provider: mode}
	}
	return ocrEngine{provider: mode, client: s.factories.NewOCR(mode, snapshot)}
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
	case OCREngineMistral:
		return "ocr_mistral"
	case OCREngineLLM:
		return "ocr_llm"
	case OCREngineRapidOCR:
		return "ocr_rapidocr"
	default:
		return "ocr"
	}
}

func (s *Service) extractBuiltinPDF(ctx context.Context, input ExtractInput, pageCount int) (Result, error) {
	native, err := s.extractPDFPagesNative(input.File.StoragePath, input.PDFMaxPages)
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
	native extractport.PDFTextResult,
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
		return Result{PageCount: pageCount, Engine: nativeEngineName}, NewError("pdf_no_extractable_text", errors.New("PDF has no extractable text"))
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
		return Result{PageCount: pageCount, Engine: nativeEngineName}, NewError("pdf_no_extractable_text", errors.New("PDF has no extractable text"))
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
		}, NewOCRError(input.OCREngine, "ocr_invalid_response", nil)
	}

	merged := joinBuiltinPDFPages(native.Pages, ocrPages)
	if strings.TrimSpace(merged) == "" {
		return Result{
			PageCount: pageCount,
			Engine:    ocrResult.Engine,
			OCRUsed:   true,
		}, NewError("extract_failed", errors.New("extraction failed"))
	}
	return Result{
		Text:      merged,
		PageCount: pageCount,
		Engine:    ocrResult.Engine,
		OCRUsed:   true,
		OCRPages:  ocrResult.OCRPages,
	}, nil
}

func collectPDFOCRCandidatePages(fileName string, pages []extractport.PDFTextPage) []int {
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

func compactPageNumbersToRanges(pageNumbers []int) []extractport.PageRange {
	if len(pageNumbers) == 0 {
		return nil
	}
	ranges := make([]extractport.PageRange, 0)
	start := pageNumbers[0]
	end := start
	for _, pageNumber := range pageNumbers[1:] {
		if pageNumber == end+1 {
			end = pageNumber
			continue
		}
		ranges = append(ranges, extractport.PageRange{Start: start, End: end})
		start = pageNumber
		end = pageNumber
	}
	ranges = append(ranges, extractport.PageRange{Start: start, End: end})
	return ranges
}

func buildFullPDFPageRanges(pageCount int) []extractport.PageRange {
	if pageCount <= 0 {
		return nil
	}
	return []extractport.PageRange{{Start: 1, End: pageCount}}
}

func indexOCRPages(pages []extractport.PageText) map[int]string {
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

func joinBuiltinPDFPages(nativePages []extractport.PDFTextPage, ocrPages map[int]string) string {
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
