package auth

// AccessSessionState 是 access token 会话校验后的运行时主体状态。
type AccessSessionState struct {
	Role                    string
	InitialSecurityRequired bool
}
