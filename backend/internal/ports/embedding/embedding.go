// Package embedding 定义面向应用层的 embedding 契约。
//
// 输入为多模态：一个批次可混合文本与图片项。部署实际可 embed
// 哪些模态取决于所配置的协议；调用方在构建图片输入前
// 应先查询 Capabilities。
package embedding

import "errors"

// Protocol 标识 embedding 端点使用的线上格式。
type Protocol string

const (
	// ProtocolOpenAI 是 OpenAI 兼容的 POST /embeddings 请求，input 为字符串数组。
	ProtocolOpenAI Protocol = "openai"
	// ProtocolGemini 是 Google Generative Language 的 batchEmbedContents 请求。
	ProtocolGemini Protocol = "gemini"
	// ProtocolVoyage 是 Voyage AI 的 POST /multimodalembeddings 请求。
	ProtocolVoyage Protocol = "voyage"
	// ProtocolJina 是 Jina AI 的 POST /embeddings 请求，input 为带类型的对象。
	ProtocolJina Protocol = "jina"
)

// Purpose 告知区分索引与查询的提供方当前批次属于哪一侧；
// 不做此区分的提供方会忽略该字段。
type Purpose string

const (
	PurposeDocument Purpose = "document"
	PurposeQuery    Purpose = "query"
)

// InputKind 表示单个 Input 的模态。
type InputKind string

const (
	InputText  InputKind = "text"
	InputImage InputKind = "image"
)

// Input 是 embedding 批次中的一项。
type Input struct {
	Kind InputKind
	// Kind 为 InputText 时设置 Text。
	Text string
	// Kind 为 InputImage 时设置 MimeType 与 Data。Data 保存原始
	// 图片字节，由协议适配器决定如何编码。
	MimeType string
	Data     []byte
}

// TextInputs 将普通字符串包装为文本输入。
func TextInputs(texts []string) []Input {
	inputs := make([]Input, 0, len(texts))
	for _, text := range texts {
		inputs = append(inputs, Input{Kind: InputText, Text: text})
	}
	return inputs
}

// Request 描述发送给 embedding 提供方的单个批次。
type Request struct {
	Protocol   Protocol
	APIBase    string
	APIKey     string
	Model      string
	Inputs     []Input
	Purpose    Purpose
	Dimensions int
	// OmitDimensions 仅控制请求序列化。Dimensions 仍表示
	// 预期的响应宽度，并始终用于校验。
	OmitDimensions bool
	TimeoutSeconds int
}

// Capabilities 报告协议可 embed 的模态。
type Capabilities struct {
	Image bool
}

// ErrModalityUnsupported 在批次包含所选协议无法 embed 的
// 输入类型时返回。
var ErrModalityUnsupported = errors.New("embedding: input modality is not supported by the configured protocol")

// ProtocolCapabilities 返回协议的模态支持情况。未知
// 协议按仅支持文本处理。
func ProtocolCapabilities(protocol Protocol) Capabilities {
	switch protocol {
	case ProtocolGemini, ProtocolVoyage, ProtocolJina:
		return Capabilities{Image: true}
	default:
		return Capabilities{}
	}
}
