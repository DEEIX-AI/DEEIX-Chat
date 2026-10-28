package billing

import (
	"context"
	"errors"
	"math"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// ExpiringCreditGrantInput 定义管理员发放限时额度的入参。
type ExpiringCreditGrantInput struct {
	UserID          uint
	CreditUSD       float64
	DurationDays    float64
	CreatedByUserID uint
	RefNo           string
	Description     string
}

// StartBackgroundWorkers 启动限时额度到期回收任务。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if s == nil || s.expiringCreditRepo == nil {
		return
	}
	s.workerOnce.Do(func() {
		go func() {
			const sweepInterval = time.Minute
			s.sweepExpiringCredits(ctx, time.Now())
			ticker := time.NewTicker(sweepInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					s.sweepExpiringCredits(ctx, now)
				}
			}
		}()
	})
}

func (s *Service) SetLogger(logger *zap.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

func (s *Service) sweepExpiringCredits(ctx context.Context, now time.Time) {
	sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := s.expiringCreditRepo.ExpireDueExpiringCredits(sweepCtx, now, 1000); err != nil && ctx.Err() == nil {
		s.logger.Warn("expiring_credit_sweep_failed", zap.Error(err))
	}
}

// GrantExpiringCredit 管理员发放一笔到期自动回收剩余值的限时额度。
func (s *Service) GrantExpiringCredit(ctx context.Context, input ExpiringCreditGrantInput) (*domainbilling.ExpiringCredit, *domainbilling.BillingAccount, error) {
	durationSeconds, validDuration := fractionalDaysToSeconds(input.DurationDays, false)
	if s.expiringCreditRepo == nil || input.UserID == 0 || input.CreatedByUserID == 0 || !validDuration ||
		input.CreditUSD <= 0 || input.CreditUSD >= float64(math.MaxInt64)/1e9 || math.IsNaN(input.CreditUSD) || math.IsInf(input.CreditUSD, 0) {
		return nil, nil, ErrInvalidExpiringCredit
	}
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, nil, err
	}
	if mode != "usage" && mode != "period" {
		return nil, nil, ErrPaymentRequired
	}
	credit, account, err := s.expiringCreditRepo.GrantExpiringCredit(ctx, repository.ExpiringCreditGrantInput{
		UserID:          input.UserID,
		AmountNanousd:   usdToNanousd(input.CreditUSD),
		DurationSeconds: durationSeconds,
		CreatedByUserID: input.CreatedByUserID,
		RefNo:           input.RefNo,
		Description:     input.Description,
	})
	switch {
	case errors.Is(err, repository.ErrInvalidInput):
		return nil, nil, ErrInvalidExpiringCredit
	case errors.Is(err, repository.ErrConflict):
		return nil, nil, ErrExpiringCreditConflict
	default:
		return credit, account, err
	}
}

func fractionalDaysToSeconds(days float64, allowZero bool) (int64, bool) {
	if math.IsNaN(days) || math.IsInf(days, 0) || days < 0 || days > 3650 {
		return 0, false
	}
	if days == 0 {
		return 0, allowZero
	}
	if days < 0.1 {
		return 0, false
	}
	seconds := int64(math.Round(days * 24 * 60 * 60))
	return seconds, seconds > 0
}
