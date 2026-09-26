package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

// 本地模式登录。
//
// 桌面 sidecar 只服务本机上的一个人，没有"注册"和"输入密码"的场景。启动时服务器
// 生成一枚一次性 grant 交给拉起它的父进程（桌面壳），桌面壳用它换取一个普通会话，
// 之后走与服务器部署完全相同的 access/refresh 轮换。因此本地模式没有任何长期存在
// 的免鉴权入口：grant 只能从进程握手拿到、只能用一次、两分钟内过期。
//
// 本地用户不设密码（PasswordEnabled=false），密码登录对它天然不可用。

const (
	localGrantTTL   = 2 * time.Minute
	localGrantBytes = 32
)

// ErrLocalGrantInvalid 表示 grant 不存在、已使用或已过期。
var ErrLocalGrantInvalid = errors.New("invalid local grant")

// LocalOwnerDisplayName 本地用户的显示名。
const LocalOwnerDisplayName = "Me"

type localGrantState struct {
	mu        sync.Mutex
	grant     string
	expiresAt time.Time
}

var localGrant localGrantState

// EnsureLocalOwner 确保本地模式的唯一用户存在，幂等。用户名来自配置（AdminUsername，
// 本地模式下为 "owner"），角色 superadmin，不设密码、无需初始化引导。
func (s *Service) EnsureLocalOwner(ctx context.Context) (*domainuser.User, error) {
	username := strings.TrimSpace(s.cfg.Snapshot().AdminUsername)
	existing, err := s.repo.GetByUsername(ctx, username)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	now := time.Now()
	item := &domainuser.User{
		PublicID:              conv.NormalizePublicID(uuid.NewString()),
		Username:              username,
		DisplayName:           LocalOwnerDisplayName,
		Role:                  domainuser.RoleSuperAdmin,
		Status:                domainuser.StatusActive,
		Timezone:              "Etc/UTC",
		Locale:                "en-US",
		OnboardingCompletedAt: &now,
		UsernameChangedAt:     &now,
	}
	if err := s.repo.CreateWithCredential(ctx, repository.CreateWithCredentialInput{
		User: item,
		// 无密码凭据：PasswordEnabled=false 使密码登录与"必须改密码"的判断都不成立。
		Credential: domainuser.Credential{PasswordEnabled: false},
	}); err != nil {
		return nil, err
	}
	return item, nil
}

// IssueLocalGrant 生成新的一次性 grant，替换任何未使用的旧 grant。
func (s *Service) IssueLocalGrant() (string, error) {
	buf := make([]byte, localGrantBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	grant := base64.RawURLEncoding.EncodeToString(buf)
	localGrant.mu.Lock()
	localGrant.grant = grant
	localGrant.expiresAt = time.Now().Add(localGrantTTL)
	localGrant.mu.Unlock()
	return grant, nil
}

// ExchangeLocalGrant 用一次性 grant 换取本地用户的会话。grant 用后即焚。
func (s *Service) ExchangeLocalGrant(
	ctx context.Context,
	presented string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	presented = strings.TrimSpace(presented)

	localGrant.mu.Lock()
	current := localGrant.grant
	expired := time.Now().After(localGrant.expiresAt)
	matched := current != "" && !expired && subtle.ConstantTimeCompare([]byte(current), []byte(presented)) == 1
	if matched {
		localGrant.grant = ""
	}
	localGrant.mu.Unlock()

	if !matched {
		s.RecordAuthEvent(ctx, repository.AuthEventInput{RequestID: requestID, EventType: "local_grant_exchange", Result: "failure", Reason: "grant_invalid", ClientIP: normalizedAuditCtx.ClientIP, UserAgent: normalizedAuditCtx.UserAgent})
		return nil, ErrLocalGrantInvalid
	}

	owner, err := s.EnsureLocalOwner(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	result, err := s.issueLoginResult(ctx, owner, normalizedAuditCtx, now)
	if err != nil {
		return nil, err
	}
	s.RecordAuthEvent(ctx, repository.AuthEventInput{UserID: owner.ID, RequestID: requestID, EventType: "local_grant_exchange", Result: "success", ClientIP: normalizedAuditCtx.ClientIP, UserAgent: normalizedAuditCtx.UserAgent})
	return result, nil
}
