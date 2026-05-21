package user

import (
	"net/mail"
	"strings"
)

// NormalizeEmail 归一化并校验邮箱，空值表示未设置。
func NormalizeEmail(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", nil
	}

	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", ErrInvalidEmail
	}

	return normalized, nil
}

// NormalizePhone 归一化并校验手机号，空值表示未设置。
func NormalizePhone(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	var builder strings.Builder
	for index, char := range trimmed {
		switch {
		case char == '+' && index == 0:
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == ' ' || char == '-' || char == '(' || char == ')':
			continue
		default:
			return "", ErrInvalidPhone
		}
	}

	normalized := builder.String()
	digits := strings.TrimPrefix(normalized, "+")
	if len(digits) < 6 || len(digits) > 20 {
		return "", ErrInvalidPhone
	}

	return normalized, nil
}
