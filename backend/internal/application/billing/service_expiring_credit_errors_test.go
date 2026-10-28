package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type expiringCreditErrorRepository struct {
	repository.BillingRepository
	failure error
}

func (*expiringCreditErrorRepository) GetBillingMode(context.Context) (string, error) {
	return "usage", nil
}

func (r *expiringCreditErrorRepository) GrantExpiringCredit(context.Context, repository.ExpiringCreditGrantInput) (*domainbilling.ExpiringCredit, *domainbilling.BillingAccount, error) {
	return nil, nil, r.failure
}

func (*expiringCreditErrorRepository) ExpireDueExpiringCredits(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func TestExpiringGrantMapsRepositoryErrorsAtApplicationBoundary(t *testing.T) {
	internal := errors.New("database unavailable")
	for _, tc := range []struct{ input, want error }{
		{repository.ErrInvalidInput, ErrInvalidExpiringCredit},
		{repository.ErrConflict, ErrExpiringCreditConflict},
		{internal, internal},
	} {
		svc := NewService(&expiringCreditErrorRepository{failure: tc.input})
		_, _, err := svc.GrantExpiringCredit(t.Context(), ExpiringCreditGrantInput{
			UserID: 1, CreatedByUserID: 9, CreditUSD: 20, DurationDays: 0.1,
		})
		if !errors.Is(err, tc.want) {
			t.Fatalf("repository error %v mapped to %v, want %v", tc.input, err, tc.want)
		}
	}
}
