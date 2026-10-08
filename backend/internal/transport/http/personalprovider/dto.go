package personalprovider

import (
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
)

// PersonalProviderAccessResponse 描述当前用户能否使用自带 Key 的模型服务。
type PersonalProviderAccessResponse struct {
	// Enabled 为 false 表示管理员未开启，或当前用户不在允许的权限组内。
	Enabled bool `json:"enabled"`
	// MaxPerUser 是每个用户最多可添加的服务数。
	MaxPerUser int `json:"maxPerUser"`
	// Protocols 是可选的调用协议。
	Protocols []string `json:"protocols"`
}

// PersonalProviderResponse 是用户自己的模型服务。永远不包含 API Key，只有打码提示。
type PersonalProviderResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Icon 是内置图标 slug；空串表示按服务地址自动匹配。
	Icon     string   `json:"icon"`
	Protocol string   `json:"protocol"`
	BaseURL  string   `json:"baseURL"`
	Host     string   `json:"host"`
	KeyHint  string   `json:"keyHint"`
	Models   []string `json:"models"`
	// Status: active 可用；disabled 用户已停用；suspended 被管理员停用，用户不能自行启用。
	Status string `json:"status" enums:"active,disabled,suspended"`
	Source string `json:"source" enums:"manual,link"`
	// LastError 是最近一次检测失败的错误码；为空表示最近一次检测成功。
	LastError     string     `json:"lastError"`
	LastCheckedAt *time.Time `json:"lastCheckedAt" extensions:"x-nullable,!x-omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// PersonalProviderDataResponse 包裹单条服务。
type PersonalProviderDataResponse struct {
	Provider PersonalProviderResponse `json:"provider"`
}

// PersonalProviderListResponse 是服务列表。
type PersonalProviderListResponse struct {
	Providers []PersonalProviderResponse `json:"providers"`
}

// PersonalProviderModelsResponse 是上游可用模型名列表。
type PersonalProviderModelsResponse struct {
	Models []string `json:"models"`
}

// PersonalProviderDeleteResponse 表示删除结果。
type PersonalProviderDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

// PersonalProviderProbeRequest 用候选配置检测并拉取模型列表，不保存任何内容。
type PersonalProviderProbeRequest struct {
	Protocol string `json:"protocol" binding:"required,max=64"`
	BaseURL  string `json:"baseURL" binding:"required,max=512"`
	APIKey   string `json:"apiKey" binding:"required,max=512"`
}

// CreatePersonalProviderRequest 新建服务。服务端会重新检测，只保存上游目录中存在的模型。
type CreatePersonalProviderRequest struct {
	Name string `json:"name,omitempty" binding:"max=64"`
	// Icon 是内置图标 slug；省略表示按服务地址自动匹配。
	Icon     string   `json:"icon,omitempty" binding:"max=64"`
	Protocol string   `json:"protocol" binding:"required,max=64"`
	BaseURL  string   `json:"baseURL" binding:"required,max=512"`
	APIKey   string   `json:"apiKey" binding:"required,max=512"`
	Models   []string `json:"models" binding:"max=200"`
	// Source 为 link 表示来自一键导入链接，需要管理员开启链接导入。
	Source string `json:"source,omitempty" binding:"omitempty,oneof=manual link" enums:"manual,link"`
}

// UpdatePersonalProviderRequest 更新服务；省略的字段不修改。地址与协议不可修改。
type UpdatePersonalProviderRequest struct {
	Name *string `json:"name,omitempty" binding:"omitempty,max=64"`
	// Icon 为空串表示恢复按服务地址自动匹配。
	Icon    *string   `json:"icon,omitempty" binding:"omitempty,max=64"`
	APIKey  *string   `json:"apiKey,omitempty" binding:"omitempty,max=512"`
	Models  *[]string `json:"models,omitempty" binding:"omitempty,max=200"`
	Enabled *bool     `json:"enabled,omitempty"`
}

// AdminPersonalProviderResponse 是管理员视角的服务信息：只有元数据与打码提示，没有 Key。
type AdminPersonalProviderResponse struct {
	ID          string `json:"id"`
	OwnerUserID uint   `json:"ownerUserID"`
	// OwnerLabel 是所属用户的展示名（显示名，其次用户名）；无法解析时为空串。
	OwnerLabel       string `json:"ownerLabel"`
	OwnerPublicID    string `json:"ownerPublicID"`
	OwnerUsername    string `json:"ownerUsername"`
	OwnerDisplayName string `json:"ownerDisplayName"`
	OwnerEmail       string `json:"ownerEmail"`
	// OwnerAvatarURL 是所属用户头像的原始地址（可能是 file: 引用），由前端按头像规则解析。
	OwnerAvatarURL string     `json:"ownerAvatarURL"`
	Name           string     `json:"name"`
	Icon           string     `json:"icon"`
	Protocol       string     `json:"protocol"`
	BaseURL        string     `json:"baseURL"`
	Host           string     `json:"host"`
	KeyHint        string     `json:"keyHint"`
	ModelCount     int        `json:"modelCount"`
	Status         string     `json:"status" enums:"active,disabled,suspended"`
	Source         string     `json:"source" enums:"manual,link"`
	LastError      string     `json:"lastError"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt" extensions:"x-nullable,!x-omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// AdminSuspendPersonalProvidersRequest 停用或恢复指定服务。
type AdminSuspendPersonalProvidersRequest struct {
	IDs       []string `json:"ids" binding:"required,min=1,max=500"`
	Suspended bool     `json:"suspended"`
}

// AdminSuspendPersonalProviderHostRequest 停用某个域名下的全部服务。
type AdminSuspendPersonalProviderHostRequest struct {
	Host string `json:"host" binding:"required,max=255"`
}

// AdminDeletePersonalProvidersRequest 删除指定服务。
type AdminDeletePersonalProvidersRequest struct {
	IDs []string `json:"ids" binding:"required,min=1,max=500"`
}

// PersonalProviderAffectedResponse 表示批量操作影响的条数。
type PersonalProviderAffectedResponse struct {
	Affected int64 `json:"affected"`
}

// PersonalProviderAccessResponseDoc 用于 Swagger。
type PersonalProviderAccessResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     PersonalProviderAccessResponse `json:"data"`
}

// PersonalProviderListResponseDoc 用于 Swagger。
type PersonalProviderListResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     PersonalProviderListResponse `json:"data"`
}

// PersonalProviderResponseDoc 用于 Swagger。
type PersonalProviderResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     PersonalProviderDataResponse `json:"data"`
}

// PersonalProviderModelsResponseDoc 用于 Swagger。
type PersonalProviderModelsResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     PersonalProviderModelsResponse `json:"data"`
}

// PersonalProviderDeleteResponseDoc 用于 Swagger。
type PersonalProviderDeleteResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     PersonalProviderDeleteResponse `json:"data"`
}

// AdminPersonalProviderPageResponseDoc 用于 Swagger。
type AdminPersonalProviderPageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                           `json:"total"`
		Results []AdminPersonalProviderResponse `json:"results"`
	} `json:"data"`
}

// PersonalProviderAffectedResponseDoc 用于 Swagger。
type PersonalProviderAffectedResponseDoc struct {
	ErrorMsg string                           `json:"errorMsg"`
	Data     PersonalProviderAffectedResponse `json:"data"`
}

// ErrorDoc 表示错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}

func toProviderResponses(items []domainpersonalprovider.Provider) []PersonalProviderResponse {
	results := make([]PersonalProviderResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toProviderResponse(item))
	}
	return results
}

func toProviderResponse(item domainpersonalprovider.Provider) PersonalProviderResponse {
	models := item.Models
	if models == nil {
		models = []string{}
	}
	return PersonalProviderResponse{
		ID:            item.PublicID,
		Name:          item.Name,
		Icon:          item.Icon,
		Protocol:      item.Protocol,
		BaseURL:       item.BaseURL,
		Host:          item.Host,
		KeyHint:       item.KeyHint,
		Models:        models,
		Status:        item.Status,
		Source:        item.Source,
		LastError:     item.LastError,
		LastCheckedAt: item.LastCheckedAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toAdminProviderResponses(items []domainpersonalprovider.Provider, labels map[uint]appadmin.UserLabel) []AdminPersonalProviderResponse {
	results := make([]AdminPersonalProviderResponse, 0, len(items))
	for _, item := range items {
		owner := labels[item.OwnerUserID]
		results = append(results, AdminPersonalProviderResponse{
			ID:               item.PublicID,
			OwnerUserID:      item.OwnerUserID,
			OwnerLabel:       owner.Label,
			OwnerPublicID:    owner.PublicID,
			OwnerUsername:    owner.Username,
			OwnerDisplayName: owner.DisplayName,
			OwnerEmail:       owner.Email,
			OwnerAvatarURL:   owner.AvatarURL,
			Name:             item.Name,
			Icon:             item.Icon,
			Protocol:         item.Protocol,
			BaseURL:          item.BaseURL,
			Host:             item.Host,
			KeyHint:          item.KeyHint,
			ModelCount:       len(item.Models),
			Status:           item.Status,
			Source:           item.Source,
			LastError:        item.LastError,
			LastCheckedAt:    item.LastCheckedAt,
			CreatedAt:        item.CreatedAt,
		})
	}
	return results
}
