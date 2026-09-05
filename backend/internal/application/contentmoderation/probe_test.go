package contentmoderation

import (
	"errors"
	"testing"
)

func TestProbeErrorMessageUsesStablePublicText(t *testing.T) {
	wrapped := func(err error) error {
		return errors.Join(errors.New("provider endpoint details"), err)
	}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "timeout", err: wrapped(ErrModerationTimeout), want: ErrModerationTimeout.Error()},
		{name: "rate limited", err: wrapped(ErrModerationRateLimited), want: ErrModerationRateLimited.Error()},
		{name: "invalid response", err: wrapped(ErrModerationInvalidResp), want: ErrModerationInvalidResp.Error()},
		{name: "network", err: wrapped(ErrModerationNetwork), want: ErrModerationNetwork.Error()},
		{name: "service", err: wrapped(ErrModerationService), want: ErrModerationService.Error()},
		{name: "unknown", err: errors.New("provider URL and response details"), want: ErrProbeFailed.Error()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := probeErrorMessage(test.err); got != test.want {
				t.Fatalf("probeErrorMessage() = %q, want %q", got, test.want)
			}
		})
	}
}
