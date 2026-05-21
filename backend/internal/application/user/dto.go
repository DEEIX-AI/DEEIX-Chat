package user

import domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"

// ProfileData 用户资料响应内部传输结构，不携带序列化标记。
type ProfileData struct {
	User domainuser.User
}
