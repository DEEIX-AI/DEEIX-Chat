package usersettings

// PatchSettingsRequest 批量更新用户配置请求。
type PatchSettingsRequest struct {
	Settings map[string]string `json:"settings" binding:"required"`
}

// UserSettingsResponse 用户配置响应，键值对形式的全量配置。
type UserSettingsResponse struct {
	Settings map[string]string `json:"settings"`
}

// UserSettingsResponseDoc Swagger 响应文档。
type UserSettingsResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     UserSettingsResponse `json:"data"`
}
