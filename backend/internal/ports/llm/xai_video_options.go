package llm

import (
	"math"
	"strconv"
	"strings"
)

// SanitizeXAIVideoExtensionOptions 仅保留 xAI 视频扩展支持的时长参数。
func SanitizeXAIVideoExtensionOptions(options map[string]any) {
	if len(options) == 0 {
		return
	}
	for key := range options {
		if key != "duration" {
			delete(options, key)
		}
	}
	duration, ok := IntegerOption(options, "duration")
	if !ok || duration < 2 || duration > 10 {
		delete(options, "duration")
		return
	}
	options["duration"] = duration
}

// SanitizeXAIVideoOptions 将 xAI 视频协议参数收敛为实际会上送的规范值。
// Application 层复用该函数，保证有效参数、计费和 adapter 请求一致。
func SanitizeXAIVideoOptions(options map[string]any) {
	if len(options) == 0 {
		return
	}
	aspectRatio := strings.ToLower(stringOption(options, "aspect_ratio"))
	if isXAIVideoAspectRatio(aspectRatio) {
		options["aspect_ratio"] = aspectRatio
	} else {
		delete(options, "aspect_ratio")
	}
	duration, durationOK := IntegerOption(options, "duration")
	if durationOK && duration >= 1 && duration <= 15 {
		options["duration"] = duration
	} else {
		delete(options, "duration")
	}
	resolution := strings.ToLower(stringOption(options, "resolution"))
	if isXAIVideoResolution(resolution) {
		options["resolution"] = resolution
	} else {
		delete(options, "resolution")
	}
}

func isXAIVideoAspectRatio(value string) bool {
	switch value {
	case "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3":
		return true
	default:
		return false
	}
}

func isXAIVideoResolution(value string) bool {
	switch value {
	case "480p", "720p", "1080p":
		return true
	default:
		return false
	}
}

func stringOption(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	value, ok := options[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

// IntegerOption 读取整数类型的 LLM 选项，并拒绝有损或越界转换。
func IntegerOption(options map[string]any, key string) (int, bool) {
	if options == nil {
		return 0, false
	}
	value, ok := options[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		maxInt := int64(^uint(0) >> 1)
		minInt := -maxInt - 1
		if typed < minInt || typed > maxInt {
			return 0, false
		}
		return int(typed), true
	case float64:
		if math.Trunc(typed) != typed {
			return 0, false
		}
		if strconv.IntSize == 32 && (typed < float64(-1<<31) || typed >= float64(1<<31)) {
			return 0, false
		}
		if strconv.IntSize == 64 && (typed < float64(-1<<63) || typed >= float64(1<<63)) {
			return 0, false
		}
		return int(typed), true
	}
	return 0, false
}
