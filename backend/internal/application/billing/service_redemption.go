package billing

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	defaultRedemptionCodeBytes = 16
	maxRedemptionCodeQuantity  = 100

	BatchDeleteStatusDeleted  = "deleted"
	BatchDeleteStatusNotFound = "not_found"
	BatchDeleteStatusFailed   = "failed"
)

// RedemptionCodeInput 定义管理员创建兑换码的入参。
type RedemptionCodeInput struct {
	Code           string
	Quantity       int
	Mode           string
	CreditUSD      float64
	PlanID         uint
	DurationDays   int
	MaxRedemptions *int
	PerUserLimit   int
	ExpiresAt      *time.Time
	Description    string
}

// RedemptionCodeListInput 定义管理员查询兑换码列表的入参。
type RedemptionCodeListInput struct {
	Mode         string
	Status       string
	Availability string
	Query        string
	Page         int
	PageSize     int
}

// RedemptionCodeUpdateInput 定义管理员更新兑换码管理字段的入参。
type RedemptionCodeUpdateInput struct {
	Status            *string
	MaxRedemptionsSet bool
	MaxRedemptions    *int
	PerUserLimit      *int
	ExpiresAtSet      bool
	ExpiresAt         *time.Time
	Description       *string
}

// RedemptionCodeValidationError 表示兑换码配置字段校验失败。
type RedemptionCodeValidationError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (e RedemptionCodeValidationError) Error() string {
	return ErrInvalidRedemptionCode.Error()
}

func (e RedemptionCodeValidationError) Is(target error) bool {
	return target == ErrInvalidRedemptionCode
}

// RedemptionCodeView 表示后台兑换码列表视图。
type RedemptionCodeView struct {
	domainbilling.RedemptionCode
	Code string
}

// RedemptionApplyView 表示用户兑换结果。
type RedemptionApplyView struct {
	Redemption   domainbilling.Redemption
	Code         domainbilling.RedemptionCode
	Account      *domainbilling.BillingAccount
	Subscription *domainbilling.Subscription
}

// BatchDeleteResultView 表示单个兑换码批量删除结果。
type BatchDeleteResultView struct {
	ID     uint
	Status string
	Error  string
}

// BatchDeleteData 表示兑换码批量删除汇总。
type BatchDeleteData struct {
	Total         int
	SuccessCount  int
	NotFoundCount int
	FailedCount   int
	Results       []BatchDeleteResultView
}

// ListRedemptionCodes 查询管理员兑换码列表。
func (s *Service) ListRedemptionCodes(ctx context.Context, input RedemptionCodeListInput) ([]RedemptionCodeView, int64, error) {
	page, pageSize := normalizePage(input.Page, input.PageSize)
	mode := strings.TrimSpace(input.Mode)
	status := strings.TrimSpace(input.Status)
	availability := strings.TrimSpace(input.Availability)
	query := strings.TrimSpace(input.Query)
	currentMode := ""
	if availability == "available" {
		modeValue, err := s.repo.GetBillingMode(ctx)
		if err != nil {
			return nil, 0, err
		}
		currentMode = strings.TrimSpace(modeValue)
		if currentMode != domainbilling.RedemptionCodeModeUsage && currentMode != domainbilling.RedemptionCodeModePeriod {
			return []RedemptionCodeView{}, 0, nil
		}
		if mode != "" && !redemptionCodeModeAvailableInBillingMode(mode, currentMode) {
			return []RedemptionCodeView{}, 0, nil
		}
		if mode == "" {
			if currentMode == domainbilling.RedemptionCodeModePeriod {
				mode = ""
			} else {
				mode = currentMode
			}
		}
	}
	modes := []string(nil)
	if availability == "available" && mode == "" {
		modes = redemptionCodeModesAvailableInBillingMode(currentMode)
	}
	items, total, err := s.repo.ListRedemptionCodes(ctx, repository.RedemptionCodeListFilter{
		Mode:         mode,
		Modes:        modes,
		Status:       status,
		Availability: availability,
		Query:        query,
	}, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	results := make([]RedemptionCodeView, 0, len(items))
	for _, item := range items {
		results = append(results, RedemptionCodeView{RedemptionCode: item})
	}
	return results, total, nil
}

// RevealRedemptionCode 按需解密管理员指定的兑换码明文。
func (s *Service) RevealRedemptionCode(ctx context.Context, id uint) (*RedemptionCodeView, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetRedemptionCodeByID(ctx, id)
	if err != nil {
		return nil, mapRedemptionRepositoryError(err)
	}
	code, err := s.redemptionCodePlaintext(item.CodeEncrypted)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, ErrRedemptionCodePlaintextUnavailable
	}
	return &RedemptionCodeView{RedemptionCode: *item, Code: code}, nil
}

// CreateRedemptionCodes 创建一个或多个兑换码。
func (s *Service) CreateRedemptionCodes(ctx context.Context, actorUserID uint, input RedemptionCodeInput) ([]RedemptionCodeView, error) {
	if actorUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	normalized, err := s.normalizeRedemptionCodeInput(ctx, input)
	if err != nil {
		return nil, err
	}

	quantity := normalized.Quantity
	results := make([]RedemptionCodeView, 0, quantity)
	for i := 0; i < quantity; i++ {
		code := normalized.Code
		if code == "" {
			code, err = generateRedemptionCode()
			if err != nil {
				return nil, err
			}
		}
		codeHash, hashErr := s.redemptionCodeHash(code)
		if hashErr != nil {
			return nil, hashErr
		}
		codeEncrypted, encryptErr := s.redemptionCodeEncrypted(code)
		if encryptErr != nil {
			return nil, encryptErr
		}
		item := &domainbilling.RedemptionCode{
			CodeHash:        codeHash,
			CodeEncrypted:   codeEncrypted,
			CodeHint:        redemptionCodeHint(code),
			Mode:            normalized.Mode,
			RewardType:      normalized.RewardType,
			CreditNanousd:   normalized.CreditNanousd,
			PlanID:          normalized.PlanID,
			DurationDays:    normalized.DurationDays,
			MaxRedemptions:  copyIntPointer(normalized.MaxRedemptions),
			PerUserLimit:    normalized.PerUserLimit,
			Status:          domainbilling.RedemptionCodeStatusActive,
			ExpiresAt:       normalized.ExpiresAt,
			Description:     normalized.Description,
			CreatedByUserID: actorUserID,
		}
		created, createErr := s.repo.CreateRedemptionCode(ctx, item)
		if createErr != nil {
			if errors.Is(createErr, repository.ErrDuplicate) {
				return nil, ErrRedemptionCodeConflict
			}
			return nil, createErr
		}
		results = append(results, RedemptionCodeView{
			RedemptionCode: *created,
			Code:           code,
		})
	}
	return results, nil
}

// UpdateRedemptionCode 更新兑换码管理字段，不允许修改奖励本身。
func (s *Service) UpdateRedemptionCode(ctx context.Context, id uint, input RedemptionCodeUpdateInput) (*RedemptionCodeView, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	patch := repository.RedemptionCodePatch{
		MaxRedemptionsSet: input.MaxRedemptionsSet,
		MaxRedemptions:    copyIntPointer(input.MaxRedemptions),
		ExpiresAtSet:      input.ExpiresAtSet,
		ExpiresAt:         input.ExpiresAt,
		Description:       input.Description,
	}
	if input.Status != nil {
		status := normalizeRedemptionStatus(*input.Status)
		if status == "" {
			return nil, redemptionCodeValidationError("status", "status")
		}
		patch.Status = &status
	}
	if input.PerUserLimit != nil {
		if *input.PerUserLimit <= 0 {
			return nil, redemptionCodeValidationError("perUserLimit", "per_user_limit")
		}
		patch.PerUserLimit = input.PerUserLimit
	}
	if input.MaxRedemptionsSet && input.MaxRedemptions != nil && *input.MaxRedemptions <= 0 {
		return nil, redemptionCodeValidationError("maxRedemptions", "max_redemptions")
	}
	if input.MaxRedemptionsSet && input.MaxRedemptions != nil && input.PerUserLimit != nil && *input.PerUserLimit > *input.MaxRedemptions {
		return nil, redemptionCodeValidationError("perUserLimit", "limit_relationship")
	}
	if input.ExpiresAtSet && input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		return nil, redemptionCodeValidationError("expiresAt", "expires_at")
	}
	updated, err := s.repo.PatchRedemptionCode(ctx, id, patch)
	if err != nil {
		return nil, mapRedemptionRepositoryError(err)
	}
	return &RedemptionCodeView{RedemptionCode: *updated}, nil
}

// DeleteRedemptionCode 软删除兑换码，保留历史兑换记录。
func (s *Service) DeleteRedemptionCode(ctx context.Context, id uint) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	return mapRedemptionRepositoryError(s.repo.DeleteRedemptionCode(ctx, id))
}

// BatchDeleteRedemptionCodes 批量软删除兑换码，逐项返回结果。
func (s *Service) BatchDeleteRedemptionCodes(ctx context.Context, ids []uint) *BatchDeleteData {
	result := &BatchDeleteData{
		Total:   len(ids),
		Results: make([]BatchDeleteResultView, 0, len(ids)),
	}
	for _, id := range ids {
		err := s.DeleteRedemptionCode(ctx, id)
		switch {
		case err == nil:
			result.SuccessCount++
			result.Results = append(result.Results, BatchDeleteResultView{ID: id, Status: BatchDeleteStatusDeleted})
		case errors.Is(err, ErrRedemptionCodeUnavailable):
			result.NotFoundCount++
			result.Results = append(result.Results, BatchDeleteResultView{ID: id, Status: BatchDeleteStatusNotFound})
		default:
			result.FailedCount++
			result.Results = append(result.Results, BatchDeleteResultView{ID: id, Status: BatchDeleteStatusFailed, Error: err.Error()})
		}
	}
	return result
}

// RedeemCode 兑换当前用户提交的兑换码。
func (s *Service) RedeemCode(ctx context.Context, userID uint, code string) (*RedemptionApplyView, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	codeHash, err := s.redemptionCodeHash(code)
	if err != nil {
		return nil, err
	}
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode != domainbilling.RedemptionCodeModeUsage && mode != domainbilling.RedemptionCodeModePeriod {
		return nil, ErrRedemptionCodeUnavailable
	}
	now := time.Now()
	result, err := s.repo.RedeemCode(ctx, repository.RedemptionApplyInput{
		CodeHash:       codeHash,
		UserID:         userID,
		CurrentMode:    mode,
		RefNo:          redemptionRefNo(now, userID),
		SubscriptionAt: now,
	})
	if err != nil {
		return nil, mapRedemptionRepositoryError(err)
	}
	return &RedemptionApplyView{
		Redemption:   result.Redemption,
		Code:         result.Code,
		Account:      result.Account,
		Subscription: result.Subscription,
	}, nil
}

type normalizedRedemptionCodeInput struct {
	Code           string
	Quantity       int
	Mode           string
	RewardType     string
	CreditNanousd  int64
	PlanID         uint
	DurationDays   int
	MaxRedemptions *int
	PerUserLimit   int
	ExpiresAt      *time.Time
	Description    string
}

func (s *Service) normalizeRedemptionCodeInput(ctx context.Context, input RedemptionCodeInput) (normalizedRedemptionCodeInput, error) {
	code := normalizeRedemptionCode(input.Code)
	if code != "" && !validRedemptionCode(code) {
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("code", "code_format")
	}
	quantity := input.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > maxRedemptionCodeQuantity {
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("quantity", "quantity")
	}
	if code != "" && quantity != 1 {
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("quantity", "manual_quantity")
	}
	perUserLimit := input.PerUserLimit
	if perUserLimit <= 0 {
		perUserLimit = 1
	}
	if input.MaxRedemptions != nil && *input.MaxRedemptions <= 0 {
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("maxRedemptions", "max_redemptions")
	}
	maxRedemptions := copyIntPointer(input.MaxRedemptions)
	if code == "" && maxRedemptions == nil {
		value := 1
		maxRedemptions = &value
	}
	if maxRedemptions != nil && perUserLimit > *maxRedemptions {
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("perUserLimit", "limit_relationship")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("expiresAt", "expires_at")
	}
	mode := normalizeRedemptionMode(input.Mode)
	switch mode {
	case domainbilling.RedemptionCodeModeUsage:
		if input.CreditUSD <= 0 || math.IsNaN(input.CreditUSD) || math.IsInf(input.CreditUSD, 0) {
			return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("creditUSD", "credit")
		}
		return normalizedRedemptionCodeInput{
			Code:           code,
			Quantity:       quantity,
			Mode:           mode,
			RewardType:     domainbilling.RedemptionRewardTypeBalance,
			CreditNanousd:  usdToNanousd(input.CreditUSD),
			MaxRedemptions: maxRedemptions,
			PerUserLimit:   perUserLimit,
			ExpiresAt:      input.ExpiresAt,
			Description:    strings.TrimSpace(input.Description),
		}, nil
	case domainbilling.RedemptionCodeModePeriod:
		if input.PlanID == 0 {
			return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("planID", "plan")
		}
		plan, err := s.repo.GetPlanByID(ctx, input.PlanID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("planID", "plan")
			}
			return normalizedRedemptionCodeInput{}, err
		}
		if !plan.IsActive || strings.TrimSpace(plan.Code) == "free" {
			return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("planID", "plan")
		}
		durationDays := input.DurationDays
		if durationDays <= 0 {
			return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("durationDays", "duration")
		}
		return normalizedRedemptionCodeInput{
			Code:           code,
			Quantity:       quantity,
			Mode:           mode,
			RewardType:     domainbilling.RedemptionRewardTypeSubscription,
			PlanID:         input.PlanID,
			DurationDays:   durationDays,
			MaxRedemptions: maxRedemptions,
			PerUserLimit:   perUserLimit,
			ExpiresAt:      input.ExpiresAt,
			Description:    strings.TrimSpace(input.Description),
		}, nil
	default:
		return normalizedRedemptionCodeInput{}, redemptionCodeValidationError("mode", "mode")
	}
}

func redemptionCodeValidationError(field string, reason string) RedemptionCodeValidationError {
	return RedemptionCodeValidationError{
		Field:  strings.TrimSpace(field),
		Reason: strings.TrimSpace(reason),
	}
}

func (s *Service) redemptionCodeHash(code string) (string, error) {
	normalized := normalizeRedemptionCode(code)
	if normalized == "" || !validRedemptionCode(normalized) {
		return "", ErrInvalidRedemptionCode
	}
	if s == nil {
		return "", ErrRedemptionCodeHashUnavailable
	}
	secret := strings.TrimSpace(s.redemptionCodeSecret)
	if secret == "" {
		return "", ErrRedemptionCodeHashUnavailable
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(normalized)) //nolint:errcheck
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) redemptionCodeEncrypted(code string) (string, error) {
	normalized := normalizeRedemptionCode(code)
	if normalized == "" || !validRedemptionCode(normalized) {
		return "", ErrInvalidRedemptionCode
	}
	if s == nil {
		return "", ErrRedemptionCodeHashUnavailable
	}
	secret := strings.TrimSpace(s.redemptionCodeSecret)
	if secret == "" {
		return "", ErrRedemptionCodeHashUnavailable
	}
	return secretbox.EncryptString(secret, normalized)
}

func (s *Service) redemptionCodePlaintext(encrypted string) (string, error) {
	encrypted = strings.TrimSpace(encrypted)
	if encrypted == "" {
		return "", nil
	}
	if s == nil {
		return "", ErrRedemptionCodeHashUnavailable
	}
	secret := strings.TrimSpace(s.redemptionCodeSecret)
	if secret == "" {
		return "", ErrRedemptionCodeHashUnavailable
	}
	code, err := secretbox.DecryptString(secret, encrypted)
	if err != nil {
		return "", ErrRedemptionCodeHashUnavailable
	}
	return normalizeRedemptionCode(code), nil
}

func normalizeRedemptionCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func validRedemptionCode(value string) bool {
	if len(value) < 3 || len(value) > 64 {
		return false
	}
	for _, item := range value {
		if (item >= 'A' && item <= 'Z') || (item >= '0' && item <= '9') || item == '-' || item == '_' {
			continue
		}
		return false
	}
	return true
}

func normalizeRedemptionMode(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.RedemptionCodeModeUsage:
		return domainbilling.RedemptionCodeModeUsage
	case domainbilling.RedemptionCodeModePeriod:
		return domainbilling.RedemptionCodeModePeriod
	default:
		return ""
	}
}

func redemptionCodeModeAvailableInBillingMode(codeMode string, billingMode string) bool {
	switch strings.TrimSpace(billingMode) {
	case domainbilling.RedemptionCodeModeUsage:
		return strings.TrimSpace(codeMode) == domainbilling.RedemptionCodeModeUsage
	case domainbilling.RedemptionCodeModePeriod:
		switch strings.TrimSpace(codeMode) {
		case domainbilling.RedemptionCodeModeUsage, domainbilling.RedemptionCodeModePeriod:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func redemptionCodeModesAvailableInBillingMode(billingMode string) []string {
	switch strings.TrimSpace(billingMode) {
	case domainbilling.RedemptionCodeModeUsage:
		return []string{domainbilling.RedemptionCodeModeUsage}
	case domainbilling.RedemptionCodeModePeriod:
		return []string{domainbilling.RedemptionCodeModeUsage, domainbilling.RedemptionCodeModePeriod}
	default:
		return nil
	}
}

func normalizeRedemptionStatus(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.RedemptionCodeStatusActive:
		return domainbilling.RedemptionCodeStatusActive
	case domainbilling.RedemptionCodeStatusInactive:
		return domainbilling.RedemptionCodeStatusInactive
	default:
		return ""
	}
}

func generateRedemptionCode() (string, error) {
	raw := make([]byte, defaultRedemptionCodeBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	value := strings.ToUpper(hex.EncodeToString(raw))
	return value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:], nil
}

func redemptionCodeHint(code string) string {
	normalized := normalizeRedemptionCode(code)
	if len(normalized) <= 8 {
		return "****"
	}
	return normalized[:4] + "***" + normalized[len(normalized)-4:]
}

func redemptionRefNo(now time.Time, userID uint) string {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("redemption_%d_%d", userID, now.UnixNano())
	}
	return fmt.Sprintf("redemption_%d_%d_%s", userID, now.UnixNano(), hex.EncodeToString(raw))
}

func copyIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func mapRedemptionRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidInput):
		return ErrInvalidRedemptionCode
	case errors.Is(err, repository.ErrRedemptionUnavailable), errors.Is(err, repository.ErrNotFound):
		return ErrRedemptionCodeUnavailable
	case errors.Is(err, repository.ErrRedemptionExhausted):
		return ErrRedemptionCodeExhausted
	case errors.Is(err, repository.ErrRedemptionUserLimitExceeded):
		return ErrRedemptionUserLimitExceeded
	default:
		return err
	}
}

func marshalRedemptionSnapshot(value map[string]interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
