package conversation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const maxAssistantImageContentItems = 8

var markdownDataImageRE = regexp.MustCompile(`(?is)^!\[[^\]]*\]\((data:image/[^)]+)\)$`)

type assistantImageContentNormalization struct {
	Content             string
	AttachmentRows      []model.Attachment
	AttachmentSnapshots []AttachmentInput
}

type assistantImageCandidate struct {
	Value    string
	MIMEType string
}

type assistantImagePayload struct {
	Bytes    []byte
	MIMEType string
}

type assistantImageContentInput struct {
	UserID             uint
	ConversationID     uint
	AssistantMessageID uint
	ModelName          string
	Content            string
}

type assistantGeneratedImagesInput struct {
	UserID                  uint
	ConversationID          uint
	AssistantMessageID      uint
	ModelName               string
	TrustedProviderEndpoint string
	GeneratedImages         []llm.GeneratedImage
}

type assistantImageSaveInput struct {
	UserID             uint
	ConversationID     uint
	AssistantMessageID uint
	ModelName          string
	Images             []assistantImagePayload
}

func (s *Service) normalizeAssistantImageContent(ctx context.Context, input assistantImageContentInput) (*assistantImageContentNormalization, error) {
	candidates := extractAssistantImageCandidates(input.Content)
	if len(candidates) == 0 {
		return nil, nil
	}
	images := make([]assistantImagePayload, 0, len(candidates))
	for _, candidate := range candidates {
		data, mimeType, ok := decodeAssistantImageCandidate(candidate)
		if !ok {
			continue
		}
		images = append(images, assistantImagePayload{Bytes: data, MIMEType: mimeType})
		if len(images) >= maxAssistantImageContentItems {
			break
		}
	}
	return s.saveAssistantImages(ctx, assistantImageSaveInput{
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		AssistantMessageID: input.AssistantMessageID,
		ModelName:          input.ModelName,
		Images:             images,
	})
}

// normalizeAssistantGeneratedImages 将协议层结构化图片结果转换为消息附件。
func (s *Service) normalizeAssistantGeneratedImages(ctx context.Context, input assistantGeneratedImagesInput) (*assistantImageContentNormalization, error) {
	images := make([]assistantImagePayload, 0, len(input.GeneratedImages))
	for _, image := range input.GeneratedImages {
		data, mimeType, err := s.readGeneratedImage(ctx, image, input.TrustedProviderEndpoint)
		if err != nil {
			return nil, err
		}
		images = append(images, assistantImagePayload{Bytes: data, MIMEType: mimeType})
		if len(images) >= maxAssistantImageContentItems {
			break
		}
	}
	return s.saveAssistantImages(ctx, assistantImageSaveInput{
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		AssistantMessageID: input.AssistantMessageID,
		ModelName:          input.ModelName,
		Images:             images,
	})
}

// saveAssistantImages 统一保存助手生成的图片，并构造消息附件快照。
func (s *Service) saveAssistantImages(ctx context.Context, input assistantImageSaveInput) (*assistantImageContentNormalization, error) {
	if len(input.Images) == 0 {
		return nil, nil
	}

	uploaded := make([]model.FileObject, 0, len(input.Images))
	attachmentRows := make([]model.Attachment, 0, len(input.Images))
	now := time.Now()
	for _, image := range input.Images {
		fileName := generatedImageFileName(input.ModelName, now, len(uploaded), len(input.Images), image.MIMEType)
		uploadResult, uploadErr := s.uploadSvc.UploadFile(ctx, appupload.UploadFileInput{
			UserID:       input.UserID,
			Purpose:      "generated_image",
			FileName:     fileName,
			MimeType:     image.MIMEType,
			DeclaredSize: int64(len(image.Bytes)),
			Reader:       bytes.NewReader(image.Bytes),
		})
		if uploadErr != nil {
			return nil, uploadErr
		}
		file := uploadResult.File
		uploaded = append(uploaded, file)
		attachmentRows = append(attachmentRows, model.Attachment{
			ConversationID: input.ConversationID,
			MessageID:      input.AssistantMessageID,
			UserID:         input.UserID,
			FileID:         file.FileID,
			Kind:           "image",
			FileName:       file.FileName,
			MimeType:       file.DetectedMIME,
			FileSize:       file.SizeBytes,
			SHA256:         file.SHA256,
			StoragePath:    file.StoragePath,
			Status:         "active",
			UploadedAt:     now,
		})
	}
	if len(uploaded) == 0 {
		return nil, nil
	}
	return &assistantImageContentNormalization{
		Content:             generatedImageMarkdown(uploaded),
		AttachmentRows:      attachmentRows,
		AttachmentSnapshots: attachmentsFromFiles(uploaded),
	}, nil
}

func extractAssistantImageCandidates(content string) []assistantImageCandidate {
	text := unwrapAssistantImageContent(content)
	if text == "" {
		return nil
	}
	if candidate, ok := markdownDataImageCandidate(text); ok {
		return []assistantImageCandidate{candidate}
	}
	if candidate, ok := directAssistantImageCandidate(text); ok {
		return []assistantImageCandidate{candidate}
	}
	if !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	candidates := make([]assistantImageCandidate, 0, 1)
	collectAssistantImageCandidates(payload, "", &candidates)
	return candidates
}

func unwrapAssistantImageContent(content string) string {
	text := strings.TrimSpace(content)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return text
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		return text
	}
	end := len(lines)
	for end > 1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if end <= 1 || !strings.HasPrefix(strings.TrimSpace(lines[end-1]), "```") {
		return text
	}
	return strings.TrimSpace(strings.Join(lines[1:end-1], "\n"))
}

func markdownDataImageCandidate(text string) (assistantImageCandidate, bool) {
	matches := markdownDataImageRE.FindStringSubmatch(strings.TrimSpace(text))
	if len(matches) != 2 {
		return assistantImageCandidate{}, false
	}
	return assistantImageCandidateFromString(matches[1], true)
}

func directAssistantImageCandidate(text string) (assistantImageCandidate, bool) {
	if candidate, ok := assistantImageCandidateFromString(text, true); ok {
		return candidate, true
	}
	if !isLikelyBase64Payload(text) {
		return assistantImageCandidate{}, false
	}
	return assistantImageCandidate{Value: text}, true
}

func collectAssistantImageCandidates(value any, key string, result *[]assistantImageCandidate) {
	if len(*result) >= maxAssistantImageContentItems {
		return
	}
	switch typed := value.(type) {
	case string:
		if candidate, ok := assistantImageCandidateFromString(typed, assistantImagePayloadKey(key)); ok {
			*result = append(*result, candidate)
		}
	case []any:
		for _, item := range typed {
			collectAssistantImageCandidates(item, key, result)
			if len(*result) >= maxAssistantImageContentItems {
				return
			}
		}
	case map[string]any:
		for childKey, childValue := range typed {
			collectAssistantImageCandidates(childValue, childKey, result)
			if len(*result) >= maxAssistantImageContentItems {
				return
			}
		}
	}
}

func assistantImageCandidateFromString(value string, allowRawBase64 bool) (assistantImageCandidate, bool) {
	text := strings.TrimSpace(value)
	if text == "" {
		return assistantImageCandidate{}, false
	}
	mimeType := dataURLImageMIMEType(text)
	if mimeType != "" {
		return assistantImageCandidate{Value: text, MIMEType: mimeType}, true
	}
	if allowRawBase64 && isLikelyBase64Payload(text) {
		return assistantImageCandidate{Value: text}, true
	}
	return assistantImageCandidate{}, false
}

func assistantImagePayloadKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "b64_json", "base64", "image", "image_base64", "image_b64", "data", "result":
		return true
	default:
		return false
	}
}

func dataURLImageMIMEType(value string) string {
	text := strings.TrimSpace(value)
	lower := strings.ToLower(text)
	if !strings.HasPrefix(lower, "data:image/") {
		return ""
	}
	semicolon := strings.Index(text, ";")
	comma := strings.Index(text, ",")
	if comma < 0 || (semicolon >= 0 && semicolon > comma) {
		return ""
	}
	return strings.TrimSpace(text[len("data:"):semicolon])
}

func isLikelyBase64Payload(value string) bool {
	normalized := normalizeBase64ImagePayload(value)
	if len(normalized) < 64 {
		return false
	}
	for _, item := range normalized {
		if (item >= 'A' && item <= 'Z') || (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') || item == '+' || item == '/' || item == '=' {
			continue
		}
		return false
	}
	return true
}

func decodeAssistantImageCandidate(candidate assistantImageCandidate) ([]byte, string, bool) {
	encoded := normalizeBase64ImagePayload(stripBase64DataURLPrefix(candidate.Value))
	if encoded == "" {
		return nil, "", false
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, "", false
	}
	data, mimeType, err := validateGeneratedImageBytes(data, candidate.MIMEType)
	if err != nil {
		return nil, "", false
	}
	return data, mimeType, true
}

func normalizeBase64ImagePayload(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, item := range value {
		switch item {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			builder.WriteRune(item)
		}
	}
	return builder.String()
}
