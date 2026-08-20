package billing

import (
	"errors"
	"testing"
)

func TestResolveEPaySubmitURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "host root", raw: "https://pay.example.com", want: "https://pay.example.com/submit.php"},
		{name: "directory root", raw: "https://pay.example.com/epay/", want: "https://pay.example.com/epay/submit.php"},
		{name: "legacy directory root without slash", raw: "https://pay.example.com/epay", want: "https://pay.example.com/epay/submit.php"},
		{name: "exact endpoint", raw: "https://pay.example.com/epay/submit.php", want: "https://pay.example.com/epay/submit.php"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveEPaySubmitURL(tt.raw)
			if err != nil {
				t.Fatalf("ResolveEPaySubmitURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveEPaySubmitURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEPaySubmitURLRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"https://user:secret@pay.example.com/",
		"https://pay.example.com/?token=secret",
		"javascript:alert(1)",
	} {
		if _, err := ResolveEPaySubmitURL(raw); !errors.Is(err, ErrEPayGatewayInvalid) {
			t.Fatalf("ResolveEPaySubmitURL(%q) error = %v, want ErrEPayGatewayInvalid", raw, err)
		}
	}
}

func TestFormatEPayAmount(t *testing.T) {
	for cents, want := range map[int64]string{
		1:                   "0.01",
		1234:                "12.34",
		9007199254740991000: "90071992547409910.00",
	} {
		if got := FormatEPayAmount(cents); got != want {
			t.Fatalf("FormatEPayAmount(%d) = %q, want %q", cents, got, want)
		}
	}
}
