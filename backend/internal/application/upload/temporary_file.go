package upload

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/google/uuid"
)

// TemporaryFileInput 描述只在当前请求生命周期内使用的文件。
type TemporaryFileInput struct {
	FileName     string
	MimeType     string
	DeclaredSize int64
	Reader       io.Reader
}

// PreparedTemporaryFile 是通过统一上传策略校验后的请求级临时文件。
// 调用方必须调用 Cleanup；该文件不会写入对象存储、文件表或用户配额。
type PreparedTemporaryFile struct {
	FileID       string
	FileName     string
	MimeType     string
	DetectedMIME string
	FileCategory string
	SizeBytes    int64
	SHA256       string
	AbsolutePath string
	Cleanup      func()
}

type stagedUpload struct {
	absolutePath string
	detectedMIME string
	sha256       string
	sizeBytes    int64
}

// PrepareTemporaryFile 复用普通上传的文件名、MIME、大小和危险类型策略，
// 但只生成请求级临时文件，不产生任何持久化记录。
func (s *Service) PrepareTemporaryFile(_ context.Context, input TemporaryFileInput) (*PreparedTemporaryFile, error) {
	if input.Reader == nil {
		return nil, s.errInvalidFileReference()
	}
	fileName := sanitizeFileName(input.FileName)
	if fileName == "" {
		return nil, s.errInvalidFileReference()
	}
	mimeType := strings.TrimSpace(input.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	cfg := s.snapshot()
	maxUploadBytes := cfg.MaxUploadFileBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = 20 * 1024 * 1024
	}
	if input.DeclaredSize > maxUploadBytes {
		return nil, s.errFileTooLarge()
	}

	fileID := "temporary_" + conv.NormalizePublicID(uuid.NewString())
	staged, err := stageUploadedFile(input.Reader, fileID, fileName, maxUploadBytes, mimeType)
	if err != nil {
		if errors.Is(err, errLocalFileTooLarge) {
			return nil, s.errFileTooLarge()
		}
		return nil, err
	}
	cleanup := func() { _ = os.Remove(staged.absolutePath) }
	category := inferFileCategory(staged.detectedMIME, fileName)
	if isDangerousMIME(staged.detectedMIME) {
		cleanup()
		return nil, s.errDangerousMIMEType()
	}
	if !isAllowedMIME(staged.detectedMIME, cfg) {
		cleanup()
		return nil, s.errMIMEBlocked()
	}
	if typeLimit := maxBytesForCategory(category, cfg); typeLimit > 0 && staged.sizeBytes > typeLimit {
		cleanup()
		return nil, s.errFileTooLarge()
	}

	return &PreparedTemporaryFile{
		FileID:       fileID,
		FileName:     fileName,
		MimeType:     mimeType,
		DetectedMIME: staged.detectedMIME,
		FileCategory: category,
		SizeBytes:    staged.sizeBytes,
		SHA256:       staged.sha256,
		AbsolutePath: staged.absolutePath,
		Cleanup:      cleanup,
	}, nil
}

func stageUploadedFile(
	reader io.Reader,
	fileID string,
	fileName string,
	maxUploadBytes int64,
	declaredMIME string,
) (stagedUpload, error) {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 20 * 1024 * 1024
	}
	tmpFile, err := os.CreateTemp("", fileID+"_*.upload")
	if err != nil {
		return stagedUpload{}, err
	}
	tmpName := tmpFile.Name()
	remove := true
	defer func() {
		_ = tmpFile.Close()
		if remove {
			_ = os.Remove(tmpName)
		}
	}()

	bufferedReader := bufio.NewReader(reader)
	header, _ := bufferedReader.Peek(512)
	detectedMIME := detectContentMIME(header, declaredMIME, fileName)
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(bufferedReader, maxUploadBytes+1))
	if err != nil {
		return stagedUpload{}, err
	}
	if written > maxUploadBytes {
		return stagedUpload{}, errLocalFileTooLarge
	}
	if err = tmpFile.Close(); err != nil {
		return stagedUpload{}, err
	}
	remove = false
	return stagedUpload{
		absolutePath: tmpName,
		detectedMIME: detectedMIME,
		sha256:       hex.EncodeToString(hasher.Sum(nil)),
		sizeBytes:    written,
	}, nil
}
