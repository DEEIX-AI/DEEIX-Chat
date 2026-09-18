package auth

import "testing"

func TestPublicAuthErrorContracts(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		message     string
		wantCode    string
		wantMessage string
	}{
		{
			name:        "username change required",
			code:        ErrUsernameChangeRequired.Code(),
			message:     ErrUsernameChangeRequired.Message(),
			wantCode:    "auth.username_change_required",
			wantMessage: "username change required",
		},
		{
			name:        "account locked",
			code:        ErrAccountLocked.Code(),
			message:     ErrAccountLocked.Message(),
			wantCode:    "auth.account_locked",
			wantMessage: "account is temporarily locked, try again later",
		},
		{
			name:        "authorization code required",
			code:        ErrAuthorizationCodeRequired.Code(),
			message:     ErrAuthorizationCodeRequired.Message(),
			wantCode:    "auth.authorization_code_required",
			wantMessage: "authorization code is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.code != test.wantCode || test.message != test.wantMessage {
				t.Fatalf("contract = (%q, %q), want (%q, %q)", test.code, test.message, test.wantCode, test.wantMessage)
			}
		})
	}
}
