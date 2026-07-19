package mcp

import "time"

type ServerResponse struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	BaseURL         string     `json:"baseURL"`
	HeadersJSON     string     `json:"headersJSON"`
	Status          string     `json:"status"`
	SortOrder       int        `json:"sortOrder"`
	ToolCount       int        `json:"toolCount"`
	ActiveToolCount int        `json:"activeToolCount"`
	LastSyncedAt    *time.Time `json:"lastSyncedAt" extensions:"x-nullable,!x-omitempty"`
	LastError       string     `json:"lastError"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type ToolResponse struct {
	ID              uint      `json:"id"`
	ServerID        uint      `json:"serverID"`
	ServerName      string    `json:"serverName"`
	Name            string    `json:"name"`
	DisplayName     string    `json:"displayName"`
	Description     string    `json:"description"`
	InputSchemaJSON string    `json:"inputSchemaJSON"`
	Status          string    `json:"status"`
	SortOrder       int       `json:"sortOrder"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type CreateServerRequest struct {
	Name        string `json:"name"`
	BaseURL     string `json:"baseURL"`
	AuthToken   string `json:"authToken,omitempty"`
	HeadersJSON string `json:"headersJSON,omitempty"`
	Status      string `json:"status,omitempty"`
}

type UpdateToolRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
}

type UpdateServerToolsStatusRequest struct {
	ToolIDs []uint `json:"toolIDs"`
	Status  string `json:"status"`
}

type ReorderServerOrderItem struct {
	ServerID uint   `json:"serverID"`
	ToolIDs  []uint `json:"toolIDs"`
}

type ReorderServersRequest struct {
	Servers []ReorderServerOrderItem `json:"servers"`
}

type ServerDataResponse struct {
	Server ServerResponse `json:"server"`
}

type DeleteServerResponse struct {
	Deleted bool `json:"deleted"`
}

type ServerListResponse struct {
	Results []ServerResponse `json:"results"`
}

type ToolListResponse struct {
	Results []ToolResponse `json:"results"`
}

type ServerToolOrderResponse struct {
	Server ServerResponse `json:"server"`
	Tools  []ToolResponse `json:"tools"`
}

type ServerToolOrderListResponse struct {
	Results []ServerToolOrderResponse `json:"results"`
}

// ErrorDoc 表示 MCP 管理接口的错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}

// ServerListResponseDoc 包裹 MCP 服务列表响应。
type ServerListResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     ServerListResponse `json:"data"`
}

// ServerDataResponseDoc 包裹 MCP 服务详情响应。
type ServerDataResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     ServerDataResponse `json:"data"`
}

// ToolListResponseDoc 包裹 MCP 工具列表响应。
type ToolListResponseDoc struct {
	ErrorMsg string           `json:"errorMsg"`
	Data     ToolListResponse `json:"data"`
}

// ToolResponseDoc 包裹 MCP 工具详情响应。
type ToolResponseDoc struct {
	ErrorMsg string       `json:"errorMsg"`
	Data     ToolResponse `json:"data"`
}

// ServerToolOrderListResponseDoc 包裹 MCP 服务及工具排序响应。
type ServerToolOrderListResponseDoc struct {
	ErrorMsg string                      `json:"errorMsg"`
	Data     ServerToolOrderListResponse `json:"data"`
}

// DeleteServerResponseDoc 包裹 MCP 服务删除响应。
type DeleteServerResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     DeleteServerResponse `json:"data"`
}
