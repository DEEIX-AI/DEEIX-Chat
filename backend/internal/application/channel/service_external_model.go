package channel

import (
	"encoding/json"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const (
	externalRouteConnectTimeoutMS    = 10_000
	externalRouteReadTimeoutMS       = 120_000
	externalRouteStreamIdleTimeoutMS = 60_000
)

// ExternalModel 描述一个不在平台路由表中的模型（用户自带 Key 的模型）。
type ExternalModel struct {
	// Ref 是对外使用的模型引用（personal:<id>/<model>），会话与计费都只认它。
	Ref string
	// Model 是上游真实模型名，用于识别厂商、图标与内置目录能力。
	Model        string
	ProviderName string
	// Protocols 是模型的调用协议：单个协议，或同一媒体模型配套的一组协议。
	Protocols []string
}

// ExternalRouteInput 描述外部模型的调用配置。APIKey 为明文，只在内存中传递。
type ExternalRouteInput struct {
	ExternalModel
	// TaskType 选出执行任务的协议（对话、图片生成、图片编辑、视频生成、视频扩展）。
	TaskType string
	BaseURL  string
	APIKey   string
}

// BuildExternalRoute 为外部模型构造调用路由：补齐厂商、图标、内置目录的推理能力与输入模态，
// 并标记 UntrustedEndpoint，使请求只走强制 SSRF 防护、不跟随重定向的客户端。
// 路由不关联任何平台上游（UpstreamID 为 0），因此不参与平台熔断、限流退避与故障切换。
// 模型的协议都不能执行该任务时返回 ErrRouteNotFound。
func (s *Service) BuildExternalRoute(input ExternalRouteInput) (*ResolvedRoute, error) {
	model := strings.TrimSpace(input.Model)
	providerName := strings.TrimSpace(input.ProviderName)
	vendor := normalizeModelVendor("", model, providerName)
	protocol := externalRouteProtocol(model, input.Protocols, NormalizeTaskType(input.TaskType))
	if protocol == "" {
		return nil, ErrRouteNotFound
	}
	route := &ResolvedRoute{
		PlatformModelName:               strings.TrimSpace(input.Ref),
		UpstreamName:                    providerName,
		Protocol:                        protocol,
		BaseURL:                         strings.TrimSpace(input.BaseURL),
		APIKey:                          input.APIKey,
		ConnectTimeoutMS:                externalRouteConnectTimeoutMS,
		ReadTimeoutMS:                   externalRouteReadTimeoutMS,
		StreamIdleTimeoutMS:             externalRouteStreamIdleTimeoutMS,
		ModelVendor:                     vendor,
		ModelIcon:                       normalizeModelIcon("", vendor, model),
		UpstreamModel:                   model,
		ReasoningContentPassback:        reasoningContentPassbackRequired(protocol, vendor, model, providerName),
		ReasoningPassbackRequestOptions: reasoningPassbackRequestOptions(protocol, vendor, model, providerName),
		UntrustedEndpoint:               true,
	}
	protocolKeys := []string{llm.OptionPolicyProtocolKey(protocol)}
	if catalog := s.activeModelCatalog(); catalog != nil {
		entry := catalog.Match(model, vendor, protocolKeys)
		route.CatalogReasoning, _ = domainchannel.CatalogReasoningCapability(entry, protocolKeys[0], vendor)
	}
	if catalog := s.activeModalityCatalog(); catalog != nil {
		if entry := catalog.MatchExternal(model, vendor, protocolKeys); entry != nil {
			route.CatalogInputModalities = append([]string(nil), entry.InputModalities...)
		}
	}
	return route, nil
}

// ExternalModelView 构造外部模型在用户模型目录中的展示数据。能力按上游真实模型名解析，
// 返回的 PlatformModelName 仍是上游模型名，由调用方替换为对外引用。
func (s *Service) ExternalModelView(model ExternalModel) ModelView {
	name := strings.TrimSpace(model.Model)
	providerName := strings.TrimSpace(model.ProviderName)
	vendor := normalizeModelVendor("", name, providerName)
	protocols := make([]string, 0, len(model.Protocols))
	for _, protocol := range model.Protocols {
		protocols = append(protocols, llm.NormalizeAdapter(protocol))
	}
	protocolsJSON, _ := json.Marshal(protocols)
	return ModelView{
		PlatformModelName: name,
		Vendor:            vendor,
		VendorName:        vendor,
		VendorIcon:        resolveVendorIcon(vendor),
		KindsJSON:         marshalKinds(externalModelKinds(name, protocols)),
		Icon:              normalizeModelIcon("", vendor, name),
		CapabilitiesJSON:  "{}",
		AccessScope:       ModelAccessScopePublic,
		Status:            "active",
		ProtocolsJSON:     string(protocolsJSON),
	}
}
