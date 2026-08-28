package extract

// PDFTextPage 表示 PDF 原生提取的单页文本。
type PDFTextPage struct {
	PageNumber    int
	Text          string
	ExtractFailed bool
}

// PDFTextResult 表示 PDF 原生提取结果。
type PDFTextResult struct {
	PageCount int
	Pages     []PDFTextPage
}

// WordTextResult 表示 Word 文档提取结果。
type WordTextResult struct {
	Text   string
	Engine string
}
