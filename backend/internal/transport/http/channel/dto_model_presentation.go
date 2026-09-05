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

// ModelVendorReferenceResponse 表示阻止删除厂商的平台模型引用。
type ModelVendorReferenceResponse struct {
	ID                uint   `json:"id"`
	PlatformModelName string `json:"platformModelName"`
}

// ModelVendorDeleteConflictDetails 表示厂商删除冲突的结构化详情。
type ModelVendorDeleteConflictDetails struct {
	Reason         string                         `json:"reason" enums:"built_in,referenced_models"`
	ReferenceCount int64                          `json:"referenceCount"`
	Models         []ModelVendorReferenceResponse `json:"models"`
}

// ModelVendorDeleteConflictDoc 表示厂商删除冲突响应。
type ModelVendorDeleteConflictDoc struct {
	ErrorMsg  string                           `json:"errorMsg"`
	ErrorCode string                           `json:"errorCode"`
	Details   ModelVendorDeleteConflictDetails `json:"details"`
	RequestID string                           `json:"requestId,omitempty"`
	Data      any                              `json:"data"`
}

// ModelDisplayGroupDataResponse 模型展示分组数据响应。
type ModelDisplayGroupDataResponse struct {
	Group ModelDisplayGroupResponse `json:"group"`
}

// ModelIconAssetResponse 管理员上传模型展示图标的响应。
type ModelIconAssetResponse struct {
	Ref         string `json:"ref"`
	PublicID    string `json:"publicID"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Reused      bool   `json:"reused"`
}

// ModelIconAssetResponseDoc 图标上传响应文档。
type ModelIconAssetResponseDoc struct {
	ErrorMsg string                 `json:"errorMsg"`
	Data     ModelIconAssetResponse `json:"data"`
}

// ModelIconAssetListItemResponse 管理员图标库中的上传资产。
type ModelIconAssetListItemResponse struct {
	Ref         string `json:"ref"`
	PublicID    string `json:"publicID"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	CreatedAt   string `json:"createdAt"`
}

// ModelIconAssetListResponseDoc 图标资产分页响应文档。
type ModelIconAssetListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                            `json:"total"`
		Results []ModelIconAssetListItemResponse `json:"results"`
	} `json:"data"`
}

// ModelIconAssetDeleteConflictDetails 表示图标仍被使用的引用统计。
type ModelIconAssetDeleteConflictDetails struct {
	ReferenceCount   int64 `json:"referenceCount"`
	Models           int64 `json:"models"`
	Vendors          int64 `json:"vendors"`
	DisplayGroups    int64 `json:"displayGroups"`
	ConversationRuns int64 `json:"conversationRuns"`
}

// ModelIconAssetDeleteConflictDoc 表示图标删除冲突响应。
type ModelIconAssetDeleteConflictDoc struct {
	ErrorMsg  string                              `json:"errorMsg"`
	ErrorCode string                              `json:"errorCode"`
	Details   ModelIconAssetDeleteConflictDetails `json:"details"`
	RequestID string                              `json:"requestId,omitempty"`
	Data      any                                 `json:"data"`
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

func toModelIconAssetResponse(item appchannel.ModelIconAssetUpload) ModelIconAssetResponse {
	return ModelIconAssetResponse{
		Ref: item.Ref, PublicID: item.PublicID, ContentType: item.ContentType,
		SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height, Reused: item.Reused,
	}
}

func toModelIconAssetListItemResponse(item appchannel.ModelIconAssetView) ModelIconAssetListItemResponse {
	return ModelIconAssetListItemResponse{
		Ref: item.Ref, PublicID: item.PublicID, ContentType: item.ContentType,
		SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height, CreatedAt: item.CreatedAt,
	}
}
