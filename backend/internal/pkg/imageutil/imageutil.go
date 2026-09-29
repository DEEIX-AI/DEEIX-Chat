// Package imageutil 提供与业务无关的图片缩放与 MIME 归一化。
package imageutil

import (
	"bytes"
	"image"
	_ "image/gif" // 注册 GIF 解码器。
	"image/jpeg"
	"image/png"
	"math"
	"strings"

	_ "golang.org/x/image/webp" // 注册 WebP 解码器。
)

// ResizeIfNeeded 在图片尺寸超过 maxDim 时进行缩放并重新编码。
// 返回的 MIME 始终与实际字节编码一致；失败时保留原始数据和 MIME。
// 使用最近邻插值以降低 CPU 开销，缩略图语义信息仍足够供 LLM 识别。
func ResizeIfNeeded(data []byte, mimeType string, maxDim int) ([]byte, string) {
	mime := ResolveMimeType(mimeType)
	if maxDim <= 0 || len(data) == 0 {
		return data, mime
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mime // 无法解码时返回原始数据，由上游模型按原图处理。
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxDim && h <= maxDim {
		return data, mime
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
	switch {
	case strings.Contains(mime, "png"):
		if encErr := png.Encode(&buf, dst); encErr != nil {
			return data, mime
		}
		return buf.Bytes(), "image/png"
	default: // jpeg 及其他格式统一使用 JPEG 输出
		if encErr := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); encErr != nil {
			return data, mime
		}
		return buf.Bytes(), "image/jpeg"
	}
}

// IsSupportedMimeType 判断 MIME 是否为本包能解码并且主流多模态接口接受的图片格式。
func IsSupportedMimeType(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// ResolveMimeType 规范化图片 MIME 类型，未知时默认为 image/jpeg。
func ResolveMimeType(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	switch normalized {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return normalized
	default:
		return "image/jpeg"
	}
}
