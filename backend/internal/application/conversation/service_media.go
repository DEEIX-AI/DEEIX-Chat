package conversation

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"strings"

	_ "image/gif" // 注册 GIF 解码器。
)

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
