package user

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	UsernameMinLength    = 3
	UsernameMaxLength    = 16
	DisplayNameMinLength = 3
	DisplayNameMaxLength = 16
	PasswordMinLength    = 8
	PasswordMaxLength    = 128
)

var reservedUsernames = map[string]struct{}{
	"admin": {}, "api": {}, "auth": {}, "billing": {}, "chat": {}, "deeix_chat": {}, "deeix-chat": {},
	"files": {}, "help": {}, "me": {}, "root": {}, "settings": {}, "support": {},
	"system": {}, "user": {}, "users": {},
}

func NormalizeUsername(raw string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(raw))
	if len(username) < UsernameMinLength || len(username) > UsernameMaxLength || strings.Contains(username, "@") {
		return "", ErrInvalidUsername
	}
	for _, ch := range username {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return "", ErrInvalidUsername
	}
	if username[0] == '-' || username[0] == '_' || username[len(username)-1] == '-' || username[len(username)-1] == '_' {
		return "", ErrInvalidUsername
	}
	if _, reserved := reservedUsernames[username]; reserved {
		return "", ErrInvalidUsername
	}
	return username, nil
}

func NormalizeDisplayName(raw string) (string, error) {
	displayName := strings.TrimSpace(raw)
	count := utf8.RuneCountInString(displayName)
	if count < DisplayNameMinLength || count > DisplayNameMaxLength {
		return "", ErrInvalidDisplayName
	}
	return displayName, nil
}

func NormalizeGeneratedDisplayName(raw string) string {
	displayName := strings.TrimSpace(raw)
	if displayName == "" {
		displayName = "user"
	}
	runes := []rune(displayName)
	if len(runes) > DisplayNameMaxLength {
		runes = runes[:DisplayNameMaxLength]
	}
	for len(runes) < DisplayNameMinLength {
		runes = append(runes, '_')
	}
	return string(runes)
}

func NormalizePassword(raw string) (string, error) {
	password := strings.TrimSpace(raw)
	count := utf8.RuneCountInString(password)
	if count < PasswordMinLength || count > PasswordMaxLength || isDigitsOnly(password) {
		return "", ErrInvalidPassword
	}
	return password, nil
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if !unicode.IsDigit(ch) {
			return false
		}
	}
	return true
}
