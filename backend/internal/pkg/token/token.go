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

// Generate 生成访问令牌。
func Generate(secret string, userID uint, username string, role string, ttl time.Duration) (string, error) {
	return GenerateWithClaims(secret, userID, username, role, "", "", "access", ttl)
}

// GenerateWithClaims 生成带会话上下文和 token 类型的令牌。
func GenerateWithClaims(
	secret string,
	userID uint,
	username string,
	role string,
	sessionID string,
	tokenID string,
	tokenType string,
	ttl time.Duration,
) (string, error) {
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		SessionID: sessionID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        tokenID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse 解析访问令牌。
func Parse(secret string, tokenString string) (*Claims, error) {
	tokenObj, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
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
