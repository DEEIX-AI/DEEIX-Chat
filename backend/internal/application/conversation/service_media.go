package conversation

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"path/filepath"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/filetype"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/imageutil"
)

const maxMediaImageEditInputPixels = 64 * 1024 * 1024

func resizeImageIfNeeded(data []byte, mimeType string, maxDim int) ([]byte, string) {
	return imageutil.ResizeIfNeeded(data, mimeType, maxDim)
}

func resolveImageMimeType(mimeType string) string {
	return imageutil.ResolveMimeType(mimeType)
}

// normalizeMediaImageEditInput 将用户上传的编辑输入图规整为静态 PNG。
// 手机拍摄图片常带有上游不稳定支持的编码、色彩模式或容器元数据；图片编辑协议统一接收这里输出的 8-bit RGBA PNG。
func normalizeMediaImageEditInput(data []byte, declaredMIME string) ([]byte, string, error) {
	detected := detectGeneratedImageMIME(data)
	if detected == "" {
		return nil, strings.TrimSpace(declaredMIME), fmt.Errorf("image edit input is not a supported image")
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, detected, err
	}
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, detected, fmt.Errorf("image edit input has invalid dimensions")
	}
	if int64(width)*int64(height) > maxMediaImageEditInputPixels {
		return nil, detected, fmt.Errorf("image edit input dimensions exceed limit")
	}

	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, detected, err
	}
	return buf.Bytes(), "image/png", nil
}

func mediaImageEditInputFileName(fileName string, mimeType string) string {
	normalizedName := strings.TrimSpace(fileName)
	ext := filepath.Ext(normalizedName)
	base := strings.TrimSuffix(normalizedName, ext)
	if strings.TrimSpace(base) == "" {
		base = "image-edit-input"
	}
	return base + filetype.ImageExtension(mimeType)
}
