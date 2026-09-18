// Package mcpauth 提供 MCP 工具调用携带的签名用户上下文。
//
// DEEIX 在 tools/call 时可以把当前用户身份以 HMAC 签名的形式发给 MCP 服务端，
// 供外部网关按用户隔离单租户 MCP 工具。签名密钥使用 MCP 专用配置，
// 服务端可以用同一个 MCP 专用密钥独立校验，不需要额外的密钥分发。
package mcpauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	// HeaderName 是签名用户上下文的请求头名。
	HeaderName = "X-Deeix-User-Context"
	// TemplateSignedUserContext 是管理员在 MCP 服务端请求头中填写的占位符。
	// 值等于该占位符的请求头会在每次 tools/call 时被替换为新鲜签名的 token。
	TemplateSignedUserContext = "${DEEIX_SIGNED_USER_CONTEXT}"
	// DefaultTTL 是签名上下文的默认有效期。token 每次调用都会重新签发，
	// 有效期只需覆盖传输与网关校验耗时。
	DefaultTTL = 5 * time.Minute
)

// Payload 描述签名上下文中携带的用户信息。
type Payload struct {
	UserID         uint   `json:"user_id"`
	ConversationID uint   `json:"conversation_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	ExpiresAt      int64  `json:"exp"`
}

// Sign 用 HMAC-SHA256 对 payload 签名并返回 token。
// token 格式为 v1.<base64url(payload)>.<base64url(hmac-sha256)>，
// 服务端用同一密钥对前半部分重算 HMAC 即可校验。
func Sign(secret string, payload Payload) (string, error) {
	key := strings.TrimSpace(secret)
	if key == "" {
		return "", errors.New("signing secret is empty")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return "v1." + encoded + "." + signature, nil
}
