package repository

import (
	"context"
	"time"
)

// ProviderAuthTransaction 是由公共客户端发起的 OAuth provider 授权
// 在服务端保存的短期状态。
type ProviderAuthTransaction struct {
	ProviderSlug         string `json:"providerSlug"`
	ClientID             string `json:"clientID"`
	ClientRedirectURI    string `json:"clientRedirectURI"`
	ClientState          string `json:"clientState"`
	ClientCodeChallenge  string `json:"clientCodeChallenge"`
	ProviderCodeVerifier string `json:"providerCodeVerifier"`
	Intent               string `json:"intent"`
	// UserID 仅在绑定身份（intent=bind）时设置，记录发起授权的已登录用户。
	UserID    uint      `json:"userID,omitempty"`
	Next      string    `json:"next"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ProviderAuthGrant 是服务端完成 provider 回调后交给公共客户端的一次性交接凭证。
// 敏感的 provider code 与 token 永远不会离开服务端。
type ProviderAuthGrant struct {
	ProviderSlug string `json:"providerSlug"`
	ClientID     string `json:"clientID"`
	Intent       string `json:"intent"`
	UserID       uint   `json:"userID"`
	Subject      string `json:"subject"`
	// Profile 仅在绑定身份时携带：绑定在客户端兑换时才落库，回调阶段只保存已取回的资料。
	Profile      *ProviderAuthGrantProfile `json:"profile,omitempty"`
	ErrorCode    string                    `json:"errorCode,omitempty"`
	ErrorMessage string                    `json:"errorMessage,omitempty"`
	ErrorDetails string                    `json:"errorDetails,omitempty"`
	ExpiresAt    time.Time                 `json:"expiresAt"`
}

// ProviderAuthGrantProfile 是回调阶段从身份源取回、等待绑定的用户资料。
type ProviderAuthGrantProfile struct {
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	ProfileJSON   string `json:"profileJSON"`
}

// ProviderAuthBridgeRepository 存储并原子地消费 provider auth bridge
// 使用的短期事务与授权记录。
type ProviderAuthBridgeRepository interface {
	PutProviderAuthTransaction(ctx context.Context, id string, item ProviderAuthTransaction, ttl time.Duration) error
	ConsumeProviderAuthTransaction(ctx context.Context, id string) (*ProviderAuthTransaction, error)
	PutProviderAuthGrant(ctx context.Context, key string, item ProviderAuthGrant, ttl time.Duration) error
	ConsumeProviderAuthGrant(ctx context.Context, key string) (*ProviderAuthGrant, error)
}
