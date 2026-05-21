package middleware

import "github.com/gin-gonic/gin"

// MustUserID 获取登录用户ID，不存在时返回0。
func MustUserID(c *gin.Context) uint {
	value, ok := c.Get(ContextKeyUserID)
	if !ok {
		return 0
	}
	userID, ok := value.(uint)
	if !ok {
		return 0
	}
	return userID
}

// MustUsername 获取登录用户名。
func MustUsername(c *gin.Context) string {
	value, ok := c.Get(ContextKeyUsername)
	if !ok {
		return ""
	}
	username, ok := value.(string)
	if !ok {
		return ""
	}
	return username
}

// MustRequestID 获取请求ID。
func MustRequestID(c *gin.Context) string {
	value, ok := c.Get(ContextKeyRequestID)
	if !ok {
		return ""
	}
	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}

// MustSessionID 获取当前登录会话ID。
func MustSessionID(c *gin.Context) string {
	value, ok := c.Get(ContextKeySessionID)
	if !ok {
		return ""
	}
	sessionID, ok := value.(string)
	if !ok {
		return ""
	}
	return sessionID
}

// MustTraceID 获取链路追踪 ID。
func MustTraceID(c *gin.Context) string {
	value, ok := c.Get(ContextKeyTraceID)
	if !ok {
		return ""
	}
	traceID, ok := value.(string)
	if !ok {
		return ""
	}
	return traceID
}
