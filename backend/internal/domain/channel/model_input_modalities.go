package channel

import (
	"strings"
)

// 模型输入模态，取值与 models.dev modalities.input 一致。
const (
	InputModalityText  = "text"
	InputModalityImage = "image"
	InputModalityPDF   = "pdf"
	InputModalityAudio = "audio"
	InputModalityVideo = "video"
)

// 输入模态来源。未声明且目录未匹配时来源为空，表示能力未知。
const (
	InputModalitiesSourceExplicit = "explicit"
	InputModalitiesSourceCatalog  = "catalog"
)

// inputModalitiesCapabilityKey 是能力 JSON 中显式声明输入模态的字段，例如 {"inputModalities": ["text","image","pdf"]}。
const inputModalitiesCapabilityKey = "inputModalities"

// inputModalityOrder 是输入模态的规范顺序，归一化结果按此排序，保证比较与展示稳定。
var inputModalityOrder = []string{InputModalityText, InputModalityImage, InputModalityPDF, InputModalityAudio, InputModalityVideo}

// InputModalities 是模型生效的输入模态及其来源。
//
// 输入模态描述模型本身能接收哪些内容；能否以原生内容块发送，还取决于协议是否支持该内容块、
// 上游地址是否可信以及大小限制，由会话层综合判断。
type InputModalities struct {
	Values []string
	// Source 为 explicit / catalog；为空表示未知，调用方应保持既有行为。
	Source string
}

// Known 表示输入模态来自显式声明或目录，而不是未知。
func (m InputModalities) Known() bool {
	return m.Source != ""
}

// Supports 判断是否支持指定模态；能力未知时返回 false，由调用方决定未知时的兼容行为。
func (m InputModalities) Supports(modality string) bool {
	modality = strings.ToLower(strings.TrimSpace(modality))
	for _, value := range m.Values {
		if value == modality {
			return true
		}
	}
	return false
}

// NormalizeInputModalities 只保留已知模态，去重并按规范顺序排列；结果非空时总包含 text。
func NormalizeInputModalities(values []string) []string {
	result := normalizeModalities(values)
	if len(result) > 0 && result[0] != InputModalityText {
		result = append([]string{InputModalityText}, result...)
	}
	return result
}

// NormalizeOutputModalities 只保留已知模态，去重并按规范顺序排列；不补 text（图像生成模型可能只输出图片）。
func NormalizeOutputModalities(values []string) []string {
	return normalizeModalities(values)
}

func normalizeModalities(values []string) []string {
	present := make(map[string]struct{}, len(values))
	for _, value := range values {
		present[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	var result []string
	for _, modality := range inputModalityOrder {
		if _, ok := present[modality]; ok {
			result = append(result, modality)
		}
	}
	return result
}

// ResolveInputModalities 按「显式声明 > 目录」解析输入模态；都没有时返回未知。
// 显式声明格式无效时视为未声明并回退到目录，保存时由 ValidateModelCapsOverrides 拦截。
func ResolveInputModalities(capabilitiesJSON string, catalog []string) InputModalities {
	payload, _ := parseCapabilities(capabilitiesJSON)
	if values, declared, err := explicitInputModalities(payload); declared && err == nil {
		return InputModalities{Values: values, Source: InputModalitiesSourceExplicit}
	}
	if values := NormalizeInputModalities(catalog); len(values) > 0 {
		return InputModalities{Values: values, Source: InputModalitiesSourceCatalog}
	}
	return InputModalities{}
}

// explicitInputModalities 读取能力 JSON 中的 inputModalities：必须是字符串数组，且每项都是已知模态。
func explicitInputModalities(payload map[string]any) ([]string, bool, error) {
	raw, exists := payload[inputModalitiesCapabilityKey]
	if !exists || raw == nil {
		return nil, false, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, true, ErrInvalidModelCapsOverride
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok || !isKnownInputModality(text) {
			return nil, true, ErrInvalidModelCapsOverride
		}
		values = append(values, text)
	}
	return NormalizeInputModalities(append(values, InputModalityText)), true, nil
}

func validateInputModalitiesOverride(payload map[string]any) error {
	_, _, err := explicitInputModalities(payload)
	return err
}

func isKnownInputModality(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, modality := range inputModalityOrder {
		if value == modality {
			return true
		}
	}
	return false
}
