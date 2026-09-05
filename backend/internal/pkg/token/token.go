package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 定义 JWT 载荷。
type Claims struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// GenerateClaimsInput 描述生成带会话上下文令牌所需的声明与有效期。
type GenerateClaimsInput struct {
	Secret    string
	UserID    uint
	Username  string
	Role      string
	SessionID string
	TokenID   string
	TokenType string
	TTL       time.Duration
}

// GenerateWithClaims 生成带会话上下文和 token 类型的令牌。
func GenerateWithClaims(input GenerateClaimsInput) (string, error) {
	claims := Claims{
		UserID:    input.UserID,
		Username:  input.Username,
		Role:      input.Role,
		SessionID: input.SessionID,
		TokenType: input.TokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(input.TTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        input.TokenID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(input.Secret))
}

// Parse 解析访问令牌。
func Parse(secret string, tokenString string) (*Claims, error) {
	tokenObj, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := tokenObj.Claims.(*Claims)
	if !ok || !tokenObj.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
