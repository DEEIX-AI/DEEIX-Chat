package extract

// OCR 引擎提供方标识。
const (
	OCREngineRapidOCR  = "rapidocr"
	OCREngineTesseract = "tesseract"
	OCREnginePaddle    = "paddle"
	OCREngineTencent   = "tencent"
	OCREngineAliyun    = "aliyun"
	OCREngineMistral   = "mistral"
	OCREngineLLM       = "llm"
)

// PageRange 表示 OCR 需要处理的连续页区间。
type PageRange struct {
	Start int
	End   int
}

// PageText 表示单页 OCR 结果。
type PageText struct {
	PageNumber int
	Text       string
}

// OCRRequest 表示一次 OCR 请求。
type OCRRequest struct {
	AbsolutePath string
	FileName     string
	MimeType     string
	PageRanges   []PageRange
}

// OCRResponse 表示 OCR 返回结果。
type OCRResponse struct {
	Text          string
	RenderedPages int
	Pages         []PageText
}
