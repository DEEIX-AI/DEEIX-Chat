package auth

import (
	"context"
	"fmt"
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

func (s *Service) resolveSecurityVerificationMethod(ctx context.Context, item *domainuser.User) (SecurityVerificationMethod, error) {
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return SecurityVerificationMethodNone, err
	}
	if len(methods) == 0 {
		return SecurityVerificationMethodNone, nil
	}
	return methods[0], nil
}

func (s *Service) verifySecurityCode(
	ctx context.Context,
	item *domainuser.User,
	purpose string,
	target string,
	code string,
	now time.Time,
) error {
	return s.verifySecurityCodeWithMethod(ctx, item, "", purpose, target, code, now)
}

func (s *Service) verifySecurityCodeWithMethod(
	ctx context.Context,
	item *domainuser.User,
	method SecurityVerificationMethod,
	purpose string,
	target string,
	code string,
	now time.Time,
) error {
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return err
	}
	if method == "" {
		if len(methods) == 0 {
			method = SecurityVerificationMethodNone
		} else {
			method = methods[0]
		}
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return fmt.Errorf("verification method is unavailable")
	}
	switch method {
	case SecurityVerificationMethodTwoFactor:
		if err = s.verifyCurrentTwoFactorCode(ctx, item.ID, code); err != nil {
			return fmt.Errorf("verification code is invalid or expired")
		}
		return nil
	case SecurityVerificationMethodEmail:
		return s.verifyEmailCode(ctx, item.ID, purpose, target, strings.TrimSpace(code), now)
	default:
		return nil
	}
}
