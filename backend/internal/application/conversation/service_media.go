package conversation

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"path/filepath"
	"strings"

	_ "image/gif" // 注册 GIF 解码器。
	_ "golang.org/x/image/webp"
)

const maxMediaImageEditInputPixels = 64 * 1024 * 1024

// resizeImageIfNeeded 在图片尺寸超过 maxDim 时进行缩放并重新编码。
// 若解码/编码失败则返回原始字节，不报错，保证降级可用。
// 使用最近邻插值以降低 CPU 开销，缩略图语义信息仍足够供 LLM 识别。
func resizeImageIfNeeded(data []byte, mimeType string, maxDim int) []byte {
	if maxDim <= 0 || len(data) == 0 {
		return data
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data // 无法解码时返回原始数据，由上游模型按原图处理。
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxDim && h <= maxDim {
		return data
	}

	var scale float64
	if w >= h {
		scale = float64(maxDim) / float64(w)
	} else {
		scale = float64(maxDim) / float64(h)
	}
	newW := int(math.Round(float64(w) * scale))
	newH := int(math.Round(float64(h) * scale))
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	// 最近邻缩放
	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	for dy := 0; dy < newH; dy++ {
		for dx := 0; dx < newW; dx++ {
			sx := int(float64(dx)/scale) + bounds.Min.X
			sy := int(float64(dy)/scale) + bounds.Min.Y
			if sx >= bounds.Max.X {
				sx = bounds.Max.X - 1
			}
			if sy >= bounds.Max.Y {
				sy = bounds.Max.Y - 1
			}
			dst.Set(dx, dy, src.At(sx, sy))
		}
	}

	var buf bytes.Buffer
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.Contains(mime, "png"):
		if encErr := png.Encode(&buf, dst); encErr != nil {
			return data
		}
	default: // jpeg 及其他格式统一使用 JPEG 输出
		if encErr := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); encErr != nil {
			return data
		}
	}
	return buf.Bytes()
}

// resolveImageMimeType 规范化图片 MIME 类型，未知时默认为 image/jpeg。
func resolveImageMimeType(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	switch normalized {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return normalized
	default:
		return "image/jpeg"
	}
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
	return base + imageFileExtension(mimeType)
}
