package acl

import "time"

const (
	ResourceTypeConversation  = "conversation"
	ResourceTypeKnowledgeBase = "knowledge_base"

	RoleViewer = "viewer"
	RoleEditor = "editor"
	RoleOwner  = "owner"
)

// Entry 描述一条资源授权。
type Entry struct {
	ID               uint
	ResourceType     string
	ResourcePublicID string
	GranteeUserID    uint
	GranteeUsername  string
	Role             string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RoleAtLeast 判断 actual 是否满足所需最低角色（owner > editor > viewer）。
func RoleAtLeast(actual string, minimum string) bool {
	return roleRank(actual) >= roleRank(minimum)
}

func roleRank(role string) int {
	switch role {
	case RoleOwner:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}
