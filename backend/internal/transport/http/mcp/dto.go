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
	LastSyncedAt    *time.Time `json:"lastSyncedAt"`
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
	AuthToken   string `json:"authToken"`
	HeadersJSON string `json:"headersJSON"`
	Status      string `json:"status"`
}

type UpdateToolRequest struct {
	DisplayName *string `json:"displayName"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
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
