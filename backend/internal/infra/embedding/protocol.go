package embedding

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
)

// protocol 把 ports 层的批量请求编码为某家厂商的 HTTP 请求，并把响应解码回按输入顺序排列的向量。
// 调用方负责维度校验与顺序完整性检查，适配器只关心线上格式。
type protocol interface {
	// encode 返回完整请求 URL、请求体和额外请求头。
	encode(input portembedding.Request) (string, []byte, http.Header, error)
	// decode 返回与 input.Inputs 等长的向量切片；缺失项为 nil。
	decode(body []byte, count int) ([][]float32, error)
}

// resolveProtocol 按配置名选择适配器；空值等同 openai，保持升级前部署的行为。
func resolveProtocol(name portembedding.Protocol) (protocol, error) {
	switch name {
	case "", portembedding.ProtocolOpenAI:
		return openAIProtocol{}, nil
	case portembedding.ProtocolGemini:
		return geminiProtocol{}, nil
	case portembedding.ProtocolVoyage:
		return voyageProtocol{}, nil
	case portembedding.ProtocolJina:
		return jinaProtocol{}, nil
	default:
		return nil, fmt.Errorf("embedding: unknown protocol %q", name)
	}
}

// ── OpenAI 兼容 ──────────────────────────────────────────────────────────────

type openAIProtocol struct{}

type openAIRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions *int     `json:"dimensions,omitempty"`
}

type openAIResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// encode 只接受文本输入；OpenAI /embeddings 没有图片字段，遇到图片直接返回模态不支持。
func (openAIProtocol) encode(input portembedding.Request) (string, []byte, http.Header, error) {
	texts := make([]string, 0, len(input.Inputs))
	for _, item := range input.Inputs {
		if item.Kind != portembedding.InputText {
			return "", nil, nil, portembedding.ErrModalityUnsupported
		}
		texts = append(texts, item.Text)
	}
	payload := openAIRequest{Model: input.Model, Input: texts}
	if !input.OmitDimensions {
		payload.Dimensions = &input.Dimensions
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, fmt.Errorf("embedding: marshal request: %w", err)
	}
	headers := http.Header{}
	if key := strings.TrimSpace(input.APIKey); key != "" {
		headers.Set("Authorization", "Bearer "+key)
	}
	return strings.TrimRight(input.APIBase, "/") + "/embeddings", body, headers, nil
}

// decode 按响应中的 index 归位向量，重复或越界的 index 视为响应损坏。
func (openAIProtocol) decode(body []byte, count int) ([][]float32, error) {
	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("embedding: decode response: %w", err)
	}
	result := make([][]float32, count)
	for _, item := range response.Data {
		if item.Index < 0 || item.Index >= count {
			return nil, fmt.Errorf("embedding: response index %d out of range", item.Index)
		}
		if result[item.Index] != nil {
			return nil, fmt.Errorf("embedding: duplicate response index %d", item.Index)
		}
		result[item.Index] = item.Embedding
	}
	return result, nil
}

// ── Gemini batchEmbedContents ────────────────────────────────────────────────

type geminiProtocol struct{}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiEmbedRequest struct {
	Model   string `json:"model"`
	Content struct {
		Parts []geminiPart `json:"parts"`
	} `json:"content"`
	TaskType             string `json:"taskType,omitempty"`
	OutputDimensionality *int   `json:"outputDimensionality,omitempty"`
}

type geminiBatchRequest struct {
	Requests []geminiEmbedRequest `json:"requests"`
}

type geminiBatchResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

// encode 生成 batchEmbedContents 请求：每个输入一条 request，文本走 parts.text，图片走 parts.inlineData。
func (geminiProtocol) encode(input portembedding.Request) (string, []byte, http.Header, error) {
	model := strings.TrimSpace(input.Model)
	if !strings.HasPrefix(model, "models/") {
		model = "models/" + model
	}
	taskType := ""
	switch input.Purpose {
	case portembedding.PurposeDocument:
		taskType = "RETRIEVAL_DOCUMENT"
	case portembedding.PurposeQuery:
		taskType = "RETRIEVAL_QUERY"
	}
	requests := make([]geminiEmbedRequest, 0, len(input.Inputs))
	for _, item := range input.Inputs {
		request := geminiEmbedRequest{Model: model, TaskType: taskType}
		switch item.Kind {
		case portembedding.InputText:
			request.Content.Parts = []geminiPart{{Text: item.Text}}
		case portembedding.InputImage:
			request.Content.Parts = []geminiPart{{InlineData: &geminiInlineData{
				MimeType: item.MimeType,
				Data:     base64.StdEncoding.EncodeToString(item.Data),
			}}}
		default:
			return "", nil, nil, portembedding.ErrModalityUnsupported
		}
		if !input.OmitDimensions {
			dimensions := input.Dimensions
			request.OutputDimensionality = &dimensions
		}
		requests = append(requests, request)
	}
	body, err := json.Marshal(geminiBatchRequest{Requests: requests})
	if err != nil {
		return "", nil, nil, fmt.Errorf("embedding: marshal request: %w", err)
	}
	headers := http.Header{}
	if key := strings.TrimSpace(input.APIKey); key != "" {
		headers.Set("x-goog-api-key", key)
	}
	endpoint := strings.TrimRight(input.APIBase, "/") + "/models/" + url.PathEscape(strings.TrimPrefix(model, "models/")) + ":batchEmbedContents"
	return endpoint, body, headers, nil
}

// decode 要求返回条数与输入条数一致；Gemini 按请求顺序返回，没有 index 字段。
func (geminiProtocol) decode(body []byte, count int) ([][]float32, error) {
	var response geminiBatchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("embedding: decode response: %w", err)
	}
	if len(response.Embeddings) != count {
		return nil, fmt.Errorf("embedding: response has %d vectors, expected %d", len(response.Embeddings), count)
	}
	result := make([][]float32, count)
	for index, item := range response.Embeddings {
		result[index] = item.Values
	}
	return result, nil
}

// ── Voyage multimodalembeddings ──────────────────────────────────────────────

type voyageProtocol struct{}

type voyageContentItem struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	ImageBase64 string `json:"image_base64,omitempty"`
}

type voyageInput struct {
	Content []voyageContentItem `json:"content"`
}

type voyageRequest struct {
	Model           string        `json:"model"`
	Inputs          []voyageInput `json:"inputs"`
	InputType       string        `json:"input_type,omitempty"`
	OutputDimension *int          `json:"output_dimension,omitempty"`
}

// encode 生成 multimodalembeddings 请求，图片以 data URI 形式放在 content 项里。
func (voyageProtocol) encode(input portembedding.Request) (string, []byte, http.Header, error) {
	inputs := make([]voyageInput, 0, len(input.Inputs))
	for _, item := range input.Inputs {
		switch item.Kind {
		case portembedding.InputText:
			inputs = append(inputs, voyageInput{Content: []voyageContentItem{{Type: "text", Text: item.Text}}})
		case portembedding.InputImage:
			inputs = append(inputs, voyageInput{Content: []voyageContentItem{{
				Type:        "image_base64",
				ImageBase64: dataURI(item.MimeType, item.Data),
			}}})
		default:
			return "", nil, nil, portembedding.ErrModalityUnsupported
		}
	}
	payload := voyageRequest{Model: input.Model, Inputs: inputs, InputType: string(input.Purpose)}
	if !input.OmitDimensions {
		payload.OutputDimension = &input.Dimensions
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, fmt.Errorf("embedding: marshal request: %w", err)
	}
	headers := http.Header{}
	if key := strings.TrimSpace(input.APIKey); key != "" {
		headers.Set("Authorization", "Bearer "+key)
	}
	return strings.TrimRight(input.APIBase, "/") + "/multimodalembeddings", body, headers, nil
}

// decode 复用 OpenAI 的 data[].embedding/index 响应结构，Voyage 的返回与之一致。
func (voyageProtocol) decode(body []byte, count int) ([][]float32, error) {
	return openAIProtocol{}.decode(body, count)
}

// ── Jina embeddings ──────────────────────────────────────────────────────────

type jinaProtocol struct{}

type jinaInput struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type jinaRequest struct {
	Model      string      `json:"model"`
	Input      []jinaInput `json:"input"`
	Dimensions *int        `json:"dimensions,omitempty"`
}

// encode 生成 Jina /embeddings 请求，输入为对象数组，图片以 data URI 形式放在 image 字段。
func (jinaProtocol) encode(input portembedding.Request) (string, []byte, http.Header, error) {
	inputs := make([]jinaInput, 0, len(input.Inputs))
	for _, item := range input.Inputs {
		switch item.Kind {
		case portembedding.InputText:
			inputs = append(inputs, jinaInput{Text: item.Text})
		case portembedding.InputImage:
			inputs = append(inputs, jinaInput{Image: dataURI(item.MimeType, item.Data)})
		default:
			return "", nil, nil, portembedding.ErrModalityUnsupported
		}
	}
	payload := jinaRequest{Model: input.Model, Input: inputs}
	if !input.OmitDimensions {
		payload.Dimensions = &input.Dimensions
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, fmt.Errorf("embedding: marshal request: %w", err)
	}
	headers := http.Header{}
	if key := strings.TrimSpace(input.APIKey); key != "" {
		headers.Set("Authorization", "Bearer "+key)
	}
	return strings.TrimRight(input.APIBase, "/") + "/embeddings", body, headers, nil
}

// decode 复用 OpenAI 的 data[].embedding/index 响应结构。
func (jinaProtocol) decode(body []byte, count int) ([][]float32, error) {
	return openAIProtocol{}.decode(body, count)
}

// dataURI 把图片字节编码为 data URI，供以内联字符串接收图片的协议使用。
func dataURI(mimeType string, data []byte) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}
