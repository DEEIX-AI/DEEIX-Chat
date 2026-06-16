package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type redemptionRepositoryStub struct {
	*billingRepositoryStub
	plan         *domainbilling.Plan
	created      []domainbilling.RedemptionCode
	getByIDErr   error
	createErr    error
	deleteErr    error
	deletedIDs   []uint
	listFilter   repository.RedemptionCodeListFilter
	listCalled   bool
	redeemInput  *repository.RedemptionApplyInput
	redeemResult *repository.RedemptionApplyResult
	redeemErr    error
}

func newRedemptionRepositoryStub(mode string) *redemptionRepositoryStub {
	return &redemptionRepositoryStub{
		billingRepositoryStub: &billingRepositoryStub{mode: mode},
	}
}

func (r *redemptionRepositoryStub) GetPlanByID(context.Context, uint) (*domainbilling.Plan, error) {
	if r.plan == nil {
		return nil, repository.ErrNotFound
	}
	return r.plan, nil
}

func (r *redemptionRepositoryStub) CreateRedemptionCode(_ context.Context, item *domainbilling.RedemptionCode) (*domainbilling.RedemptionCode, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	if r.createErr != nil {
		return nil, r.createErr
	}
	copied := *item
	copied.ID = uint(len(r.created) + 1)
	r.created = append(r.created, copied)
	return &copied, nil
}

func (r *redemptionRepositoryStub) ListRedemptionCodes(_ context.Context, filter repository.RedemptionCodeListFilter, _ int, _ int) ([]domainbilling.RedemptionCode, int64, error) {
	r.listFilter = filter
	r.listCalled = true
	return r.created, int64(len(r.created)), nil
}

func (r *redemptionRepositoryStub) GetRedemptionCodeByID(_ context.Context, id uint) (*domainbilling.RedemptionCode, error) {
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	for _, item := range r.created {
		if item.ID == id {
			copied := item
			return &copied, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *redemptionRepositoryStub) DeleteRedemptionCode(_ context.Context, id uint) error {
	r.deletedIDs = append(r.deletedIDs, id)
	return r.deleteErr
}

func (r *redemptionRepositoryStub) RedeemCode(_ context.Context, input repository.RedemptionApplyInput) (*repository.RedemptionApplyResult, error) {
	r.redeemInput = &input
	if r.redeemErr != nil {
		return nil, r.redeemErr
	}
	if r.redeemResult != nil {
		return r.redeemResult, nil
	}
	return &repository.RedemptionApplyResult{
		Code: domainbilling.RedemptionCode{ID: 1, Mode: input.CurrentMode},
		Redemption: domainbilling.Redemption{
			ID:     1,
			CodeID: 1,
			UserID: input.UserID,
			Mode:   input.CurrentMode,
		},
	}, nil
}

func TestCreateRedemptionCodesStoresHashAndReturnsPlaintextOnce(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	items, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:         " 2026 ",
		Mode:         domainbilling.RedemptionCodeModeUsage,
		CreditUSD:    12.5,
		PerUserLimit: 1,
	})
	if err != nil {
		t.Fatalf("CreateRedemptionCodes() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("CreateRedemptionCodes() len = %d, want 1", len(items))
	}
	if got := items[0].Code; got != "2026" {
		t.Fatalf("returned code = %q, want %q", got, "2026")
	}
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte("2026")) //nolint:errcheck
	wantHash := hex.EncodeToString(mac.Sum(nil))
	if got := repo.created[0].CodeHash; got != wantHash {
		t.Fatalf("stored hash = %q, want %q", got, wantHash)
	}
	if got := repo.created[0].CodeHint; got != "****" {
		t.Fatalf("stored hint = %q, want ****", got)
	}
	decrypted, err := secretbox.DecryptString("test-secret", repo.created[0].CodeEncrypted)
	if err != nil {
		t.Fatalf("decrypt stored code = %v", err)
	}
	if decrypted != "2026" {
		t.Fatalf("encrypted code decrypts to %q, want 2026", decrypted)
	}
	if repo.created[0].MaxRedemptions != nil {
		t.Fatalf("manual code MaxRedemptions = %v, want nil", *repo.created[0].MaxRedemptions)
	}
}

func TestRedemptionCodeHintUsesFourStarFourFormat(t *testing.T) {
	if got := redemptionCodeHint("C72F6A7A-241A-4CE0-AD4F-A0E511672A46"); got != "C72F***2A46" {
		t.Fatalf("redemptionCodeHint() = %q, want C72F***2A46", got)
	}
}

func TestListRedemptionCodesHidesPlaintextAndRevealDecryptsCode(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	_, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:         "copy-me",
		Mode:         domainbilling.RedemptionCodeModeUsage,
		CreditUSD:    12.5,
		PerUserLimit: 1,
	})
	if err != nil {
		t.Fatalf("CreateRedemptionCodes() error = %v", err)
	}
	items, _, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if len(items) != 1 || items[0].Code != "" {
		t.Fatalf("ListRedemptionCodes() code = %+v, want hidden plaintext", items)
	}
	revealed, err := service.RevealRedemptionCode(context.Background(), items[0].ID)
	if err != nil {
		t.Fatalf("RevealRedemptionCode() error = %v", err)
	}
	if revealed.Code != "COPY-ME" {
		t.Fatalf("RevealRedemptionCode() code = %q, want COPY-ME", revealed.Code)
	}
}

func TestListRedemptionCodesPassesAvailabilityFilter(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)

	_, _, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{
		Mode:         "usage",
		Status:       "active",
		Availability: "exhausted",
		Query:        "invite",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if repo.listFilter.Availability != "exhausted" {
		t.Fatalf("Availability filter = %q, want exhausted", repo.listFilter.Availability)
	}
	if repo.listFilter.Mode != "usage" || repo.listFilter.Status != "active" || repo.listFilter.Query != "invite" {
		t.Fatalf("List filter = %+v", repo.listFilter)
	}
}

func TestListRedemptionCodesAvailableUsesCurrentBillingMode(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)

	_, _, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{
		Availability: "available",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if !repo.listCalled {
		t.Fatalf("ListRedemptionCodes() did not query repository")
	}
	if repo.listFilter.Mode != domainbilling.RedemptionCodeModeUsage || repo.listFilter.Availability != "available" {
		t.Fatalf("List filter = %+v", repo.listFilter)
	}
}

func TestListRedemptionCodesAvailablePeriodIncludesUsageBalanceCodes(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModePeriod)
	service := NewService(repo)

	_, _, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{
		Availability: "available",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if !repo.listCalled {
		t.Fatalf("ListRedemptionCodes() did not query repository")
	}
	wantModes := []string{domainbilling.RedemptionCodeModeUsage, domainbilling.RedemptionCodeModePeriod}
	if repo.listFilter.Mode != "" || !equalStringSlices(repo.listFilter.Modes, wantModes) || repo.listFilter.Availability != "available" {
		t.Fatalf("List filter = %+v, want modes %v", repo.listFilter, wantModes)
	}
}

func TestListRedemptionCodesAvailablePeriodAllowsExplicitUsageMode(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModePeriod)
	service := NewService(repo)

	_, _, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{
		Mode:         domainbilling.RedemptionCodeModeUsage,
		Availability: "available",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if !repo.listCalled || repo.listFilter.Mode != domainbilling.RedemptionCodeModeUsage {
		t.Fatalf("List filter = %+v, want explicit usage mode", repo.listFilter)
	}
}

func TestListRedemptionCodesAvailableSkipsModeMismatch(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)

	items, total, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{
		Mode:         domainbilling.RedemptionCodeModePeriod,
		Availability: "available",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("ListRedemptionCodes() = %d/%d, want empty", len(items), total)
	}
	if repo.listCalled {
		t.Fatalf("ListRedemptionCodes() queried repository for mode mismatch")
	}
}

func TestListRedemptionCodesAvailableSkipsSelfBillingMode(t *testing.T) {
	repo := newRedemptionRepositoryStub("self")
	service := NewService(repo)

	items, total, err := service.ListRedemptionCodes(context.Background(), RedemptionCodeListInput{
		Availability: "available",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("ListRedemptionCodes() error = %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("ListRedemptionCodes() = %d/%d, want empty", len(items), total)
	}
	if repo.listCalled {
		t.Fatalf("ListRedemptionCodes() queried repository in self billing mode")
	}
}

func TestRevealRedemptionCodeRejectsLegacyCodeWithoutEncryptedPlaintext(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	repo.created = []domainbilling.RedemptionCode{{ID: 1, CodeHint: "LEGACY***"}}
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	_, err := service.RevealRedemptionCode(context.Background(), 1)
	if !errors.Is(err, ErrRedemptionCodePlaintextUnavailable) {
		t.Fatalf("RevealRedemptionCode() error = %v, want ErrRedemptionCodePlaintextUnavailable", err)
	}
}

func TestCreateRedemptionCodesRandomDefaultsToSingleUse(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	items, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Quantity:     2,
		Mode:         domainbilling.RedemptionCodeModeUsage,
		CreditUSD:    5,
		PerUserLimit: 1,
	})
	if err != nil {
		t.Fatalf("CreateRedemptionCodes() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("CreateRedemptionCodes() len = %d, want 2", len(items))
	}
	for index, item := range repo.created {
		if item.MaxRedemptions == nil || *item.MaxRedemptions != 1 {
			t.Fatalf("created[%d].MaxRedemptions = %v, want 1", index, item.MaxRedemptions)
		}
		if !validRedemptionCode(items[index].Code) {
			t.Fatalf("returned random code %q is invalid", items[index].Code)
		}
		if !isUppercaseUUID(items[index].Code) {
			t.Fatalf("returned random code %q is not uppercase UUID v4", items[index].Code)
		}
	}
}

func TestCreateRedemptionCodesRejectsPerUserLimitAboveTotal(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")
	maxRedemptions := 1

	_, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:           "LIMIT_1",
		Mode:           domainbilling.RedemptionCodeModeUsage,
		CreditUSD:      5,
		MaxRedemptions: &maxRedemptions,
		PerUserLimit:   2,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() limit error = %v, want ErrInvalidRedemptionCode", err)
	}

	_, err = service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Mode:         domainbilling.RedemptionCodeModeUsage,
		CreditUSD:    5,
		PerUserLimit: 2,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() default total limit error = %v, want ErrInvalidRedemptionCode", err)
	}
}

func TestCreateRedemptionCodesReturnsValidationReason(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")
	expiredAt := time.Now().Add(-time.Minute)

	_, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Mode:         domainbilling.RedemptionCodeModeUsage,
		CreditUSD:    5,
		PerUserLimit: 1,
		ExpiresAt:    &expiredAt,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() error = %v, want ErrInvalidRedemptionCode", err)
	}
	var validationErr RedemptionCodeValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("CreateRedemptionCodes() error = %T, want RedemptionCodeValidationError", err)
	}
	if validationErr.Field != "expiresAt" || validationErr.Reason != "expires_at" {
		t.Fatalf("validation error = %+v, want expiresAt/expires_at", validationErr)
	}
}

func TestCreateRedemptionCodesPeriodValidatesPlanAndDuration(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModePeriod)
	repo.plan = &domainbilling.Plan{ID: 11, Code: "pro", IsActive: true}
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	items, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:         "PRO_30",
		Mode:         domainbilling.RedemptionCodeModePeriod,
		PlanID:       11,
		DurationDays: 30,
		PerUserLimit: 1,
	})
	if err != nil {
		t.Fatalf("CreateRedemptionCodes() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("CreateRedemptionCodes() len = %d, want 1", len(items))
	}
	created := repo.created[0]
	if created.RewardType != domainbilling.RedemptionRewardTypeSubscription || created.PlanID != 11 || created.DurationDays != 30 {
		t.Fatalf("created period reward = (%q,%d,%d), want subscription/11/30", created.RewardType, created.PlanID, created.DurationDays)
	}

	_, err = service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:         "PRO_0",
		Mode:         domainbilling.RedemptionCodeModePeriod,
		PlanID:       11,
		DurationDays: 0,
		PerUserLimit: 1,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() duration error = %v, want ErrInvalidRedemptionCode", err)
	}

	repo.plan = &domainbilling.Plan{ID: 12, Code: "free", IsActive: true}
	_, err = service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:         "FREE_30",
		Mode:         domainbilling.RedemptionCodeModePeriod,
		PlanID:       12,
		DurationDays: 30,
		PerUserLimit: 1,
	})
	if !errors.Is(err, ErrInvalidRedemptionCode) {
		t.Fatalf("CreateRedemptionCodes() free plan error = %v, want ErrInvalidRedemptionCode", err)
	}
}

func TestCreateRedemptionCodesMapsDuplicateHash(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	repo.createErr = repository.ErrDuplicate
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	_, err := service.CreateRedemptionCodes(context.Background(), 7, RedemptionCodeInput{
		Code:         "2026",
		Mode:         domainbilling.RedemptionCodeModeUsage,
		CreditUSD:    5,
		PerUserLimit: 1,
	})
	if !errors.Is(err, ErrRedemptionCodeConflict) {
		t.Fatalf("CreateRedemptionCodes() error = %v, want ErrRedemptionCodeConflict", err)
	}
}

func TestBatchDeleteRedemptionCodesMapsResults(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	service := NewService(repo)

	result := service.BatchDeleteRedemptionCodes(context.Background(), []uint{1, 2})
	if result.SuccessCount != 2 || result.NotFoundCount != 0 || result.FailedCount != 0 {
		t.Fatalf("BatchDeleteRedemptionCodes() = %+v, want two successes", result)
	}
	if len(repo.deletedIDs) != 2 || repo.deletedIDs[0] != 1 || repo.deletedIDs[1] != 2 {
		t.Fatalf("deleted IDs = %v, want [1 2]", repo.deletedIDs)
	}

	repo.deleteErr = repository.ErrNotFound
	result = service.BatchDeleteRedemptionCodes(context.Background(), []uint{3})
	if result.SuccessCount != 0 || result.NotFoundCount != 1 || result.Results[0].Status != BatchDeleteStatusNotFound {
		t.Fatalf("BatchDeleteRedemptionCodes() not found = %+v", result)
	}
}

func TestRedeemCodeRejectsSelfModeBeforeRepositoryApply(t *testing.T) {
	repo := newRedemptionRepositoryStub("self")
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	_, err := service.RedeemCode(context.Background(), 9, "CODE2026")
	if !errors.Is(err, ErrRedemptionCodeUnavailable) {
		t.Fatalf("RedeemCode() error = %v, want ErrRedemptionCodeUnavailable", err)
	}
	if repo.redeemInput != nil {
		t.Fatalf("repository RedeemCode was called in self mode")
	}
}

func TestRedeemCodeMapsRepositoryLimitErrors(t *testing.T) {
	repo := newRedemptionRepositoryStub(domainbilling.RedemptionCodeModeUsage)
	repo.redeemErr = repository.ErrRedemptionUserLimitExceeded
	service := NewService(repo)
	service.SetRedemptionCodeSecret("test-secret")

	_, err := service.RedeemCode(context.Background(), 9, "CODE2026")
	if !errors.Is(err, ErrRedemptionUserLimitExceeded) {
		t.Fatalf("RedeemCode() error = %v, want ErrRedemptionUserLimitExceeded", err)
	}
}

func isUppercaseUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for _, index := range []int{8, 13, 18, 23} {
		if value[index] != '-' {
			return false
		}
	}
	if value[14] != '4' {
		return false
	}
	switch value[19] {
	case '8', '9', 'A', 'B':
	default:
		return false
	}
	for _, item := range value {
		if item >= 'a' && item <= 'z' {
			return false
		}
	}
	return true
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
