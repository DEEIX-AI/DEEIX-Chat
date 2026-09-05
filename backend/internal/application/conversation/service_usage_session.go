package conversation

import (
	"context"
	"sync"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"go.uber.org/zap"
)

const (
	usageAuthorizationRenewalInterval = 30 * time.Minute
	usageSessionSettleTimeout         = 10 * time.Second
	usageSessionReleaseTimeout        = 5 * time.Second
)

// UsageSession 持有一次模型运行的计费生命周期：预留预算 → 后台续租 → 按结果结算或释放。
// 传输层在写入响应前 Begin，在运行结束后 Finish，并以 Close 兜底停止续租；三者都可安全重复调用。
type UsageSession struct {
	service       *Service
	input         SendMessageBillingInput
	authorization *domainbilling.UsageAuthorization
	stopRenewal   func()

	mu       sync.Mutex
	finished bool
}

// BeginUsageSession 在模型调用前固定计费策略、预留预算并启动租约续期。
// input 只需携带请求级上下文（用户、会话、模型、run），运行结果由 Finish 补入。
func (s *Service) BeginUsageSession(ctx context.Context, input SendMessageBillingInput) (*UsageSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	authorization, err := s.AuthorizeSendMessageUsage(ctx, input)
	if err != nil {
		return nil, err
	}
	return &UsageSession{
		service:       s,
		input:         input,
		authorization: authorization,
		stopRenewal:   s.startUsageAuthorizationRenewal(ctx, authorization),
	}, nil
}

// Authorization 返回本次运行的计费授权，供生成流程做预算校验与账单标注。
func (u *UsageSession) Authorization() *domainbilling.UsageAuthorization {
	if u == nil {
		return nil
	}
	return u.authorization
}

// Finish 停止续租并按运行结果收口：产生可计费用量时结算账本并把账单快照回填到 assistant 消息，
// 否则释放预留预算。结算与释放使用脱离请求取消的短超时上下文，客户端断开不影响入账。
// 失败已在这里记录日志（有预留的结算失败同时被标记对账），返回给调用方决定是否再向客户端提示。
func (u *UsageSession) Finish(ctx context.Context, result *SendMessageResult) error {
	if u == nil || !u.markFinished() {
		return nil
	}
	if u.stopRenewal != nil {
		u.stopRenewal()
	}
	if result != nil && result.Billable {
		return u.settle(ctx, result)
	}
	return u.release(ctx)
}

// Close 兜底停止续租。未 Finish 的会话不结算也不释放预留，交由租约过期与对账处理。
func (u *UsageSession) Close() {
	if u == nil {
		return
	}
	if u.stopRenewal != nil {
		u.stopRenewal()
	}
}

func (u *UsageSession) markFinished() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.finished {
		return false
	}
	u.finished = true
	return true
}

func (u *UsageSession) settle(ctx context.Context, result *SendMessageResult) error {
	if u.service == nil || result == nil {
		return nil
	}
	settleCtx, cancel := background.WithTimeout(ctx, usageSessionSettleTimeout)
	defer cancel()
	input := u.input
	input.Result = result
	usageLedger, err := u.service.RecordSendMessageBilling(settleCtx, input, u.authorization)
	if err != nil {
		u.logFailure("usage_session_settle_failed", err)
		return err
	}
	ApplyUsageBilling(&result.AssistantMessage, usageLedger)
	return nil
}

func (u *UsageSession) release(ctx context.Context) error {
	if u.service == nil {
		return nil
	}
	releaseCtx, cancel := background.WithTimeout(ctx, usageSessionReleaseTimeout)
	defer cancel()
	if err := u.service.ReleaseSendMessageUsageAuthorization(releaseCtx, u.authorization); err != nil {
		u.logFailure("usage_session_release_failed", err)
		return err
	}
	return nil
}

func (u *UsageSession) logFailure(event string, err error) {
	if u.service.logger == nil {
		return
	}
	fields := []zap.Field{
		zap.Uint("user_id", u.input.UserID),
		zap.Uint("conversation_id", u.input.ConversationID),
		zap.String("client_run_id", u.input.ClientRunID),
		zap.Error(err),
	}
	if u.authorization != nil && u.authorization.Reservation != nil {
		fields = append(fields, zap.String("reservation_ref_no", u.authorization.Reservation.RefNo))
	}
	u.service.logger.Error(event, fields...)
}

// startUsageAuthorizationRenewal 为长时间运行的付费调用持续刷新预算租约。
// 返回的停止函数幂等，且会等待正在进行的续租结束，保证结算与续租不会并发改写同一预留。
func (s *Service) startUsageAuthorizationRenewal(parent context.Context, authorization *domainbilling.UsageAuthorization) func() {
	if authorization == nil || authorization.Reservation == nil {
		return func() {}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(usageAuthorizationRenewalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := background.WithTimeout(parent, usageSessionReleaseTimeout)
				err := s.RenewSendMessageUsageAuthorization(ctx, authorization)
				cancel()
				if err != nil && s.logger != nil {
					s.logger.Warn("usage_authorization_renew_failed",
						zap.Uint("user_id", authorization.Reservation.UserID),
						zap.String("reservation_ref_no", authorization.Reservation.RefNo),
						zap.Error(err),
					)
				}
			case <-stop:
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(stop) })
		<-done
	}
}
