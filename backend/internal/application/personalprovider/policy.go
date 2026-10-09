package personalprovider

import (
	"net/url"
	"strconv"
	"strings"

	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	maxNameRunes   = 64
	maxBaseURLLen  = 512
	maxAPIKeyLen   = 512
	maxModelsCount = 200
)

// allowedProtocols 是服务的接口协议：用于拉取模型目录，也是对话模型的默认协议，与平台对话协议一致。
// 图片、视频协议在单个模型上指定（见 modelProtocols）。
var allowedProtocols = []string{
	llm.AdapterOpenAIChatCompletions,
	llm.AdapterOpenAIResponses,
	llm.AdapterAnthropicMessages,
	llm.AdapterGoogleGenerateContent,
	llm.AdapterGeminiInteractions,
	llm.AdapterXAIResponses,
	llm.AdapterOpenRouterChat,
	llm.AdapterOpenRouterResponses,
}

// modelProtocols 是单个模型可选的调用协议：对话协议之外，还有服务厂商的图片与视频协议，顺序同后台。
// 一个模型可以是其中一个协议，或同一媒体模型的配套协议（规则见 channel.NormalizeExternalModelProtocols）。
var modelProtocols = []string{
	llm.AdapterOpenAIChatCompletions,
	llm.AdapterOpenAIResponses,
	llm.AdapterOpenAIImageGenerations,
	llm.AdapterOpenAIImageEdits,
	llm.AdapterAnthropicMessages,
	llm.AdapterGoogleGenerateContent,
	llm.AdapterGoogleImageGeneration,
	llm.AdapterGeminiInteractions,
	llm.AdapterXAIResponses,
	llm.AdapterXAIImage,
	llm.AdapterXAIImageEdits,
	llm.AdapterXAIVideo,
	llm.AdapterXAIVideoExtensions,
	llm.AdapterOpenRouterChat,
	llm.AdapterOpenRouterResponses,
	llm.AdapterOpenRouterImages,
}

// ModelProtocols 返回单个模型可选的协议列表（副本）。
func ModelProtocols() []string {
	return append([]string(nil), modelProtocols...)
}

// AllowedProtocols 返回可选协议列表（副本）。
func AllowedProtocols() []string {
	return append([]string(nil), allowedProtocols...)
}

func isAllowedProtocol(protocol string) bool {
	for _, item := range allowedProtocols {
		if item == protocol {
			return true
		}
	}
	return false
}

// Policy 是从运行时配置读出的管理员策略快照。
type Policy struct {
	Enabled      bool
	MaxPerUser   int
	BlockedHosts []blockedHost
}

// blockedHost 是一条黑名单：默认只匹配域名本身；"*.example.com" 匹配其任意层级的子域名，不含 example.com 本身。
type blockedHost struct {
	Host     string
	Wildcard bool
}

// policyFromConfig 解析管理员配置；格式错误的条目在保存设置时已被拒绝，这里只做防御性跳过。
func policyFromConfig(cfg config.Config) Policy {
	policy := Policy{
		Enabled:    cfg.PersonalProvidersEnabled,
		MaxPerUser: cfg.PersonalProvidersMaxPerUser,
	}
	if policy.MaxPerUser <= 0 {
		policy.MaxPerUser = config.DefaultPersonalProvidersMaxPerUser
	}
	for _, item := range splitList(cfg.PersonalProvidersBlockedHosts) {
		host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(item)), ".")
		trimmed := strings.TrimPrefix(host, "*.")
		if trimmed != "" {
			policy.BlockedHosts = append(policy.BlockedHosts, blockedHost{Host: trimmed, Wildcard: trimmed != host})
		}
	}
	return policy
}

// isBlockedHost 报告 host 是否命中管理员黑名单：普通条目精确匹配，通配条目匹配子域名。
func (p Policy) isBlockedHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	for _, blocked := range p.BlockedHosts {
		if blocked.Wildcard {
			if strings.HasSuffix(host, "."+blocked.Host) {
				return true
			}
		} else if host == blocked.Host {
			return true
		}
	}
	return false
}

// normalizedEndpoint 是校验通过的服务地址。
type normalizedEndpoint struct {
	BaseURL string
	Host    string
}

// normalizeBaseURL 校验并规范化用户提供的地址。
// 只接受 HTTPS：Key 会随每个请求发送，明文 HTTP 会在传输中泄露。
// 内网、回环、链路本地与云元数据目标一律拒绝；真正发起连接时 dialer 还会对解析出的 IP 再校验一次（防 DNS 重绑定）。
func normalizeBaseURL(raw string, policy Policy) (normalizedEndpoint, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > maxBaseURLLen {
		return normalizedEndpoint{}, ErrInvalidBaseURL
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Host == "" || parsed.Opaque != "" {
		return normalizedEndpoint{}, ErrInvalidBaseURL
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return normalizedEndpoint{}, ErrInvalidBaseURL
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return normalizedEndpoint{}, ErrInvalidBaseURL
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" {
		return normalizedEndpoint{}, ErrInvalidBaseURL
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return normalizedEndpoint{}, ErrInvalidBaseURL
		}
	}
	if err := security.ValidateOutboundHTTPURL(value, security.NewPublicOnlyOutboundPolicy()); err != nil {
		return normalizedEndpoint{}, ErrBlockedHost
	}
	if policy.isBlockedHost(host) {
		return normalizedEndpoint{}, ErrBlockedHost
	}

	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return normalizedEndpoint{BaseURL: parsed.String(), Host: host}, nil
}

// normalizeAPIKey 校验 Key：非空、长度受限、只含可见 ASCII，避免把换行等字符注入请求头。
func normalizeAPIKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" || len(key) > maxAPIKeyLen {
		return "", ErrInvalidAPIKey
	}
	for _, char := range key {
		if char < 0x21 || char > 0x7e {
			return "", ErrInvalidAPIKey
		}
	}
	return key, nil
}

// normalizeIcon 统一图标 slug 的大小写与空白；只接受内置图标 slug。
func normalizeIcon(raw string) (string, error) {
	icon := strings.ToLower(strings.TrimSpace(raw))
	if !domainpersonalprovider.IsValidIcon(icon) {
		return "", ErrInvalidIcon
	}
	return icon, nil
}

func normalizeName(raw string, fallback string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = fallback
	}
	if name == "" || len([]rune(name)) > maxNameRunes {
		return "", ErrInvalidName
	}
	for _, char := range name {
		if char < 0x20 || char == 0x7f {
			return "", ErrInvalidName
		}
	}
	return name, nil
}

func splitList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
}
