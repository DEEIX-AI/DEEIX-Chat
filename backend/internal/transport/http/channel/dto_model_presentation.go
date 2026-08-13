package channel

import appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"

// ModelVendorResponse 技术厂商响应 DTO。
type ModelVendorResponse struct {
	ID        uint   `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	BuiltIn   bool   `json:"builtIn"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ModelDisplayGroupResponse 模型展示分组响应 DTO。
type ModelDisplayGroupResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ModelVendorDataResponse 技术厂商数据响应。
type ModelVendorDataResponse struct {
	Vendor ModelVendorResponse `json:"vendor"`
}

// ModelDisplayGroupDataResponse 模型展示分组数据响应。
type ModelDisplayGroupDataResponse struct {
	Group ModelDisplayGroupResponse `json:"group"`
}

// ModelVendorListResponseDoc 技术厂商分页响应文档。
type ModelVendorListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                 `json:"total"`
		Results []ModelVendorResponse `json:"results"`
	} `json:"data"`
}

// ModelDisplayGroupListResponseDoc 模型展示分组分页响应文档。
type ModelDisplayGroupListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                       `json:"total"`
		Results []ModelDisplayGroupResponse `json:"results"`
	} `json:"data"`
}

// ModelVendorDataResponseDoc 技术厂商单项响应文档。
type ModelVendorDataResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     ModelVendorDataResponse `json:"data"`
}

// ModelDisplayGroupDataResponseDoc 模型展示分组单项响应文档。
type ModelDisplayGroupDataResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     ModelDisplayGroupDataResponse `json:"data"`
}

func toModelVendorResponse(item appchannel.ModelVendorView) ModelVendorResponse {
	return ModelVendorResponse{
		ID: item.ID, Key: item.Key, Name: item.Name, Icon: item.Icon,
		BuiltIn: item.BuiltIn, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toModelDisplayGroupResponse(item appchannel.ModelDisplayGroupView) ModelDisplayGroupResponse {
	return ModelDisplayGroupResponse{
		ID: item.ID, Name: item.Name, Icon: item.Icon, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
