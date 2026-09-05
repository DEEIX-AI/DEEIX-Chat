package conversation

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// TemporaryChatMaxAttachments 限制单次临时请求携带的历史附件总数。
const TemporaryChatMaxAttachments = 20

// TemporaryChatAttachment 是随临时会话请求传入、且不进入持久化文件链路的附件。
type TemporaryChatAttachment struct {
	MessageIndex int
	FileName     string
	MimeType     string
	DeclaredSize int64
	Reader       io.Reader
}

type temporaryAttachmentContext struct {
	messages          []llm.Message
	stableAttachments []AttachmentInput
	moderationImages  []appcm.OutputImageSource
}

func (s *Service) prepareTemporaryAttachmentContext(
	ctx context.Context,
	input TemporaryChatInput,
	messages []llm.Message,
) (temporaryAttachmentContext, error) {
	result := temporaryAttachmentContext{
		messages: append([]llm.Message(nil), messages...),
	}
	if len(input.Attachments) == 0 {
		return result, nil
	}
	if s.uploadSvc == nil {
		return result, ErrInvalidFileReference
	}

	cfg := s.cfg.Snapshot()
	imageCount := 0
	imageBytes := 0
	for _, item := range input.Attachments {
		file, err := s.uploadSvc.PrepareTemporaryFile(ctx, appupload.TemporaryFileInput{
			FileName:     item.FileName,
			MimeType:     item.MimeType,
			DeclaredSize: item.DeclaredSize,
			Reader:       item.Reader,
		})
		if err != nil {
			return result, err
		}
		processErr := func() error {
			defer file.Cleanup()
			switch file.FileCategory {
			case "image":
				imageCount++
				if imageCount > maxConversationImageContextCount {
					return ErrTooManyMessageFiles
				}
				data, err := os.ReadFile(file.AbsolutePath)
				if err != nil {
					return fmt.Errorf("%w: read temporary image", ErrFileNotFound)
				}
				if len(data) > maxConversationImageSourceBytes {
					return ErrFileTooLarge
				}
				resized, mimeType := resizeImageIfNeeded(data, resolveImageMimeType(file.DetectedMIME), cfg.ImageMaxDimension)
				imageBytes += len(resized)
				if imageBytes > maxConversationImageContextBytes {
					return ErrFileTooLarge
				}
				message := result.messages[item.MessageIndex]
				parts := append([]llm.ContentPart(nil), message.Parts...)
				if len(parts) == 0 && strings.TrimSpace(message.Content) != "" {
					parts = append(parts, llm.ContentPart{Kind: llm.ContentPartText, Text: message.Content})
				}
				parts = append(parts, llm.ContentPart{
					Kind:     llm.ContentPartImage,
					MimeType: mimeType,
					Data:     resized,
					FileName: file.FileName,
				})
				message.Content = ""
				message.Parts = parts
				result.messages[item.MessageIndex] = message
				result.moderationImages = append(result.moderationImages, appcm.OutputImageSource{
					FileID:   temporaryAttachmentContextID(file.SHA256),
					Data:     resized,
					MimeType: mimeType,
					SHA256:   file.SHA256,
				})
			case "pdf", "word", "presentation", "excel", "text":
				if s.extractSvc == nil {
					return ErrInvalidFileReference
				}
				extracted, err := s.extractSvc.ExtractTemporaryFile(ctx, extraction.ExtractInput{
					File: domainconversation.FileObject{
						FileID:       file.FileID,
						FileName:     file.FileName,
						MimeType:     file.MimeType,
						DetectedMIME: file.DetectedMIME,
						FileCategory: file.FileCategory,
						SizeBytes:    file.SizeBytes,
						SHA256:       file.SHA256,
						StoragePath:  file.AbsolutePath,
					},
					PDFMaxPages:           cfg.FileFullContextPDFMaxPages,
					OCREngine:             cfg.ExtractOCREngine,
					ImageOCREnabled:       false,
					PDFOCRFallbackEnabled: cfg.ExtractPDFOCRFallbackEnabled,
				})
				if err != nil || strings.TrimSpace(extracted.Text) == "" {
					return fmt.Errorf("%w: temporary attachment extraction failed", ErrFileProcessingNotReady)
				}
				attachment := AttachmentInput{
					FileID:          temporaryAttachmentContextID(file.SHA256),
					Kind:            "file",
					FileName:        file.FileName,
					MimeType:        file.MimeType,
					DetectedMIME:    file.DetectedMIME,
					FileCategory:    file.FileCategory,
					FileSize:        file.SizeBytes,
					SHA256:          file.SHA256,
					PageCount:       extracted.PageCount,
					ExtractedText:   extracted.Text,
					Current:         item.MessageIndex == len(input.Messages)-1,
					MessageRole:     "user",
					ContextMode:     fileContextModeFull,
					ProcessingReady: true,
					ExtractStatus:   "success",
				}
				if !canUseAttachmentFullContext(attachment, cfg) {
					return ErrFileTooLargeForFullContext
				}
				result.stableAttachments = append(result.stableAttachments, attachment)
			default:
				return ErrInvalidFileReference
			}
			return nil
		}()
		if processErr != nil {
			return result, processErr
		}
	}
	return result, nil
}

func temporaryAttachmentContextID(sha string) string {
	normalized := strings.ToLower(strings.TrimSpace(sha))
	if len(normalized) > 32 {
		normalized = normalized[:32]
	}
	return "temporary_" + normalized
}
