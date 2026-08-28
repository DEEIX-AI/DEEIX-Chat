package extract

// DocumentRequest 是文档抽取引擎（Tika/Docling/MinerU）的统一请求。
// 不读取 MimeType 的引擎可忽略该字段。
type DocumentRequest struct {
	AbsolutePath string
	FileName     string
	MimeType     string
}

// DefaultTikaBaseURL 是本地 Tika 服务的默认地址。
const DefaultTikaBaseURL = "http://127.0.0.1:9998"
