package auth

import (
	"context"
	"strings"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// SecurityVerificationMethod describes the extra verification factor required
// for user-owned sensitive operations.
type SecurityVerificationMethod string

const (
	SecurityVerificationMethodNone      SecurityVerificationMethod = "none"
	SecurityVerificationMethodTwoFactor SecurityVerificationMethod = "two_factor"
	SecurityVerificationMethodEmail     SecurityVerificationMethod = "email"
)

type verifySecurityCodeInput struct {
	User       *domainuser.User
	Method     SecurityVerificationMethod
	Purpose    string
	Target     string
	Code       string
	VerifiedAt time.Time
}

func hasVerifiedEmail(item *domainuser.User) bool {
	return item != nil && strings.TrimSpace(item.Email) != "" && item.EmailVerifiedAt != nil
}

func hasEmailCandidate(item *domainuser.User) bool {
	return item != nil && strings.TrimSpace(item.Email) != "" && item.EmailVerifiedAt == nil
}

func normalizeSecurityVerificationMethod(value string) SecurityVerificationMethod {
	switch SecurityVerificationMethod(strings.TrimSpace(value)) {
	case SecurityVerificationMethodTwoFactor:
		return SecurityVerificationMethodTwoFactor
	case SecurityVerificationMethodEmail:
		return SecurityVerificationMethodEmail
	case SecurityVerificationMethodNone:
		return SecurityVerificationMethodNone
	default:
		return ""
	}
}

func containsSecurityVerificationMethod(methods []SecurityVerificationMethod, method SecurityVerificationMethod) bool {
	for _, item := range methods {
		if item == method {
			return true
		}
	}
	return false
}

func (s *Service) resolveSecurityVerificationMethods(ctx context.Context, item *domainuser.User) ([]SecurityVerificationMethod, error) {
	if item == nil {
		return []SecurityVerificationMethod{SecurityVerificationMethodNone}, nil
	}
	methods := make([]SecurityVerificationMethod, 0, 2)
	useTwoFactor, err := s.shouldRequireTwoFactor(ctx, item)
	if err != nil {
		return nil, err
	}
	if useTwoFactor {
		methods = append(methods, SecurityVerificationMethodTwoFactor)
	}
	if s.cfg.Snapshot().EmailVerificationEnabled && hasVerifiedEmail(item) {
		methods = append(methods, SecurityVerificationMethodEmail)
	}
	if len(methods) == 0 {
		methods = append(methods, SecurityVerificationMethodNone)
	}
	return methods, nil
}

func (s *Service) verifySecurityCodeWithMethod(ctx context.Context, input verifySecurityCodeInput) error {
	methods, err := s.resolveSecurityVerificationMethods(ctx, input.User)
	if err != nil {
		return err
	}
	method := input.Method
	if method == "" {
		if len(methods) == 0 {
			method = SecurityVerificationMethodNone
		} else {
			method = methods[0]
		}
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return ErrSecurityVerificationMethodUnavailable
	}
	switch method {
	case SecurityVerificationMethodTwoFactor:
		if err = s.verifyCurrentTwoFactorCode(ctx, input.User.ID, input.Code); err != nil {
			return ErrSecurityVerificationCodeInvalid
		}
		return nil
	case SecurityVerificationMethodEmail:
		return s.verifyEmailCode(ctx, input.User.ID, input.Purpose, input.Target, strings.TrimSpace(input.Code), input.VerifiedAt)
	default:
		return nil
	}
}
