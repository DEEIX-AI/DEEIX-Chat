package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
)

const (
	defaultMistralOCRModel        = "mistral-ocr-latest"
	mistralOCRDocumentPlaceholder = "__deeix_mistral_ocr_document_data__"
)

type mistralOCRRequest struct {
	Model              string             `json:"model"`
	Document           mistralOCRDocument `json:"document"`
	Pages              []int              `json:"pages,omitempty"`
	IncludeImageBase64 bool               `json:"include_image_base64"`
	IncludeBlocks      bool               `json:"include_blocks"`
}

type mistralOCRDocument struct {
	Type        string `json:"type"`
	DocumentURL string `json:"document_url,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

type mistralOCRResponse struct {
	Pages []mistralOCRPage `json:"pages"`
}

type mistralOCRPage struct {
	Index    int    `json:"index"`
	Markdown string `json:"markdown"`
}

func (c *Client) extractTextWithMistral(ctx context.Context, req Request) (Response, error) {
	if c == nil || !c.mistral || c.httpClient == nil || strings.TrimSpace(c.baseURL) == "" {
		return Response{}, fmt.Errorf("ocr_unavailable")
	}
	model := strings.TrimSpace(c.model)
	if model == "" {
		model = defaultMistralOCRModel
	}

	payload := mistralOCRRequest{
		Model:              model,
		IncludeImageBase64: false,
		IncludeBlocks:      false,
	}
	mimeType := "application/pdf"
	if isImageRequest(req) {
		mimeType = strings.TrimSpace(req.MimeType)
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		payload.Document = mistralOCRDocument{
			Type: "image_url",
		}
	} else {
		payload.Document = mistralOCRDocument{
			Type: "document_url",
		}
		pageNumbers := resolveOCRPageNumbers(req.PageRanges)
		if len(pageNumbers) > 0 {
			payload.Pages = make([]int, 0, len(pageNumbers))
			for _, pageNumber := range pageNumbers {
				payload.Pages = append(payload.Pages, pageNumber-1)
			}
		}
	}

	filePath := strings.TrimSpace(req.AbsolutePath)
	body, contentLength, err := newMistralOCRRequestBody(filePath, mimeType, payload)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, body)
	if err != nil {
		_ = body.Close()
		return Response{}, err
	}
	httpReq.ContentLength = contentLength
	httpReq.GetBody = func() (io.ReadCloser, error) {
		replayBody, replayLength, replayErr := newMistralOCRRequestBody(filePath, mimeType, payload)
		if replayErr != nil {
			return nil, replayErr
		}
		if replayLength != contentLength {
			_ = replayBody.Close()
			return nil, fmt.Errorf("ocr_file_changed_during_request")
		}
		return replayBody, nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(c.authToken); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Response{}, mistralHTTPError(resp)
	}
	return parseMistralOCRResponse(io.LimitReader(resp.Body, 50*1024*1024))
}

func newMistralOCRRequestBody(filePath string, mimeType string, payload mistralOCRRequest) (io.ReadCloser, int64, error) {
	prefix, suffix, err := buildMistralOCRRequestEnvelope(mimeType, payload)
	if err != nil {
		return nil, 0, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, err
	}
	if !fileInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, 0, fmt.Errorf("ocr_invalid_file_path")
	}

	fileSize := fileInfo.Size()
	if fileSize < 0 || fileSize > (math.MaxInt64-2)/4*3 {
		_ = file.Close()
		return nil, 0, fmt.Errorf("ocr_unprocessable: file is too large")
	}
	encodedSize := ((fileSize + 2) / 3) * 4
	envelopeSize := int64(len(prefix) + len(suffix))
	if encodedSize > math.MaxInt64-envelopeSize {
		_ = file.Close()
		return nil, 0, fmt.Errorf("ocr_unprocessable: file is too large")
	}

	reader, writer := io.Pipe()
	go func() {
		defer file.Close()
		if _, writeErr := writer.Write(prefix); writeErr != nil {
			_ = writer.CloseWithError(writeErr)
			return
		}
		encoder := base64.NewEncoder(base64.StdEncoding, writer)
		_, copyErr := io.Copy(encoder, file)
		closeErr := encoder.Close()
		if copyErr != nil {
			_ = writer.CloseWithError(copyErr)
			return
		}
		if closeErr != nil {
			_ = writer.CloseWithError(closeErr)
			return
		}
		_, writeErr := writer.Write(suffix)
		_ = writer.CloseWithError(writeErr)
	}()

	return reader, envelopeSize + encodedSize, nil
}

func buildMistralOCRRequestEnvelope(mimeType string, payload mistralOCRRequest) ([]byte, []byte, error) {
	fieldName := "document_url"
	switch payload.Document.Type {
	case "document_url":
		payload.Document.DocumentURL = mistralOCRDocumentPlaceholder
	case "image_url":
		fieldName = "image_url"
		payload.Document.ImageURL = mistralOCRDocumentPlaceholder
	default:
		return nil, nil, fmt.Errorf("ocr_unprocessable: unsupported Mistral document type")
	}

	envelope, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	placeholder, err := json.Marshal(mistralOCRDocumentPlaceholder)
	if err != nil {
		return nil, nil, err
	}
	marker := append([]byte(`"`+fieldName+`":`), placeholder...)
	markerIndex := bytes.Index(envelope, marker)
	if markerIndex < 0 {
		return nil, nil, fmt.Errorf("ocr_unprocessable: invalid Mistral request envelope")
	}
	valueIndex := markerIndex + len(marker) - len(placeholder)
	dataURIPrefix, err := json.Marshal("data:" + strings.TrimSpace(mimeType) + ";base64,")
	if err != nil {
		return nil, nil, err
	}

	prefix := bytes.Clone(envelope[:valueIndex])
	prefix = append(prefix, dataURIPrefix[:len(dataURIPrefix)-1]...)
	suffix := []byte{'"'}
	suffix = append(suffix, envelope[valueIndex+len(placeholder):]...)
	return prefix, suffix, nil
}

func mistralHTTPError(resp *http.Response) error {
	detailBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	detail := strings.TrimSpace(string(detailBytes))
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("ocr_unauthorized")
	case http.StatusForbidden:
		return fmt.Errorf("ocr_forbidden")
	case http.StatusUnprocessableEntity:
		if detail == "" {
			return fmt.Errorf("ocr_unprocessable")
		}
		return fmt.Errorf("ocr_unprocessable: %s", detail)
	default:
		if detail == "" {
			return fmt.Errorf("ocr_http_%d", resp.StatusCode)
		}
		return fmt.Errorf("ocr_http_%d: %s", resp.StatusCode, detail)
	}
}

func parseMistralOCRResponse(body io.Reader) (Response, error) {
	var payload mistralOCRResponse
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return Response{}, err
	}
	pages := make([]PageText, 0, len(payload.Pages))
	for _, page := range payload.Pages {
		text := strings.ReplaceAll(page.Markdown, "\x00", "")
		if strings.TrimSpace(text) == "" {
			continue
		}
		pages = append(pages, PageText{
			PageNumber: page.Index + 1,
			Text:       text,
		})
	}
	sort.SliceStable(pages, func(i, j int) bool {
		return pages[i].PageNumber < pages[j].PageNumber
	})
	if len(pages) == 0 {
		return Response{}, errors.New(errOCREmptyContent)
	}
	parts := make([]string, 0, len(pages))
	for _, page := range pages {
		parts = append(parts, page.Text)
	}
	return Response{
		Text:          strings.Join(parts, "\n\n"),
		RenderedPages: len(pages),
		Pages:         pages,
	}, nil
}
