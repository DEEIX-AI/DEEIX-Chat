package llm

import (
	"encoding/json"
	"net/url"
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// 百炼与火山方舟的内置联网搜索沿用 OpenAI / Anthropic 的工具名 web_search，按协议和工具名无法区分厂商，
// 因此按上游地址识别厂商，把用量改写为厂商专属的计费名，避免按 OpenAI / Claude 的价格计费。
const (
	usageKeyBailianWebSearch     = "bailian_web_search"
	usageKeyBailianWebSearchIntl = "bailian_web_search_intl"
	usageKeyArkWebSearch         = "ark_web_search"
	usageKeyArkContentSource     = "ark_web_search_content_source"
	usageKeyArkDoubaoSearch      = "ark_doubao_search"
)

type serverToolVendor int

const (
	serverToolVendorUnknown serverToolVendor = iota
	serverToolVendorBailian
	serverToolVendorBailianIntl
	serverToolVendorArk
)

func serverToolVendorForBaseURL(baseURL string) serverToolVendor {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return serverToolVendorUnknown
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "dashscope-intl.aliyuncs.com" || strings.HasSuffix(host, ".ap-southeast-1.maas.aliyuncs.com"):
		return serverToolVendorBailianIntl
	case host == "dashscope.aliyuncs.com" || host == "dashscope-us.aliyuncs.com" || strings.HasSuffix(host, ".maas.aliyuncs.com"):
		return serverToolVendorBailian
	case strings.HasPrefix(host, "ark.") && strings.HasSuffix(host, ".volces.com"):
		return serverToolVendorArk
	}
	return serverToolVendorUnknown
}

// attributeVendorServerToolUsage 在适配器返回后按上游厂商改写服务端工具用量。
func attributeVendorServerToolUsage(route portllm.RouteConfig, output *portllm.GenerateOutput) {
	if output == nil {
		return
	}
	vendor := serverToolVendorForBaseURL(route.BaseURL)
	if vendor == serverToolVendorUnknown {
		return
	}
	adapter := portllm.NormalizeAdapter(route.Protocol)
	rawUsage := parseRawUsage(output.Usage.RawUsageJSON)
	var usage map[string]int64
	switch vendor {
	case serverToolVendorBailian, serverToolVendorBailianIntl:
		usage = bailianServerToolUsage(adapter, vendor, output.ServerSideToolUsage, rawUsage)
	case serverToolVendorArk:
		usage = arkServerToolUsage(adapter, output.ServerSideToolUsage, rawUsage)
	}
	if len(usage) > 0 {
		output.ServerSideToolUsage = usage
	}
}

func parseRawUsage(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return parsed
}

// bailianServerToolUsage 处理百炼：Responses 读 tool_usage / x_tools，Anthropic 读 server_tool_use，
// Chat Completions 的 enable_search 仅在 include_tool_usage 开启时通过 usage.x_tools 返回次数。
func bailianServerToolUsage(adapter string, vendor serverToolVendor, current map[string]int64, rawUsage map[string]any) map[string]int64 {
	usage := cloneInt64Map(current)
	if adapter == portllm.AdapterOpenAIChatCompletions {
		for key, value := range asMap(rawUsage["x_tools"]) {
			if count := serverToolUsageCount(value); count > 0 {
				if usage == nil {
					usage = map[string]int64{}
				}
				usage[key] += count
			}
		}
	}
	searchKey := usageKeyBailianWebSearch
	if vendor == serverToolVendorBailianIntl {
		searchKey = usageKeyBailianWebSearchIntl
	}
	return renameServerToolUsage(usage, map[string]string{"web_search": searchKey, "search": searchKey})
}

// arkServerToolUsage 处理火山方舟：Responses 按 tool_usage_details 区分联网内容插件的各内容源与豆包搜索，
// Chat API 与 Messages API 只支持豆包搜索 Custom 版。
func arkServerToolUsage(adapter string, current map[string]int64, rawUsage map[string]any) map[string]int64 {
	usage := cloneInt64Map(current)
	if adapter == portllm.AdapterOpenAIChatCompletions {
		if count := toInt64(asMap(rawUsage["tool_usage"])["web_search"]); count > 0 {
			if usage == nil {
				usage = map[string]int64{}
			}
			usage["web_search"] += count
		}
	}
	if usage["web_search"] <= 0 {
		return usage
	}
	if adapter != portllm.AdapterOpenAIResponses {
		return renameServerToolUsage(usage, map[string]string{"web_search": usageKeyArkDoubaoSearch})
	}
	details := asMap(asMap(rawUsage["tool_usage_details"])["web_search"])
	if len(details) == 0 {
		return renameServerToolUsage(usage, map[string]string{"web_search": usageKeyArkWebSearch})
	}
	delete(usage, "web_search")
	for source, value := range details {
		count := toInt64(value)
		if count <= 0 {
			continue
		}
		switch strings.TrimSpace(source) {
		case "doubao":
			usage[usageKeyArkDoubaoSearch] += count
		case "search_engine":
			usage[usageKeyArkWebSearch] += count
		default:
			usage[usageKeyArkContentSource] += count
		}
	}
	return usage
}

func renameServerToolUsage(usage map[string]int64, names map[string]string) map[string]int64 {
	if len(usage) == 0 {
		return usage
	}
	result := make(map[string]int64, len(usage))
	for key, count := range usage {
		if mapped, ok := names[key]; ok {
			key = mapped
		}
		result[key] += count
	}
	return result
}

// serverToolUsageCount 兼容整数计数与 {num_requests} / {count} 对象两种写法。
func serverToolUsageCount(value any) int64 {
	if object := asMap(value); len(object) > 0 {
		return firstNonZero(toInt64(object["num_requests"]), toInt64(object["count"]))
	}
	return toInt64(value)
}
