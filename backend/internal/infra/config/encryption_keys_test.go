package config

import (
	"strings"
	"testing"
)

func TestLoadReadsPreviousDataEncryptionKeys(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())
	t.Setenv("DATA_ENCRYPTION_KEY", "new-data-encryption-key-0123456789abcdef")
	t.Setenv("DATA_ENCRYPTION_KEYS_PREVIOUS", " old-key-one-0123456789abcdef0123 , ,old-key-two-0123456789abcdef0123 ")

	cfg := Load()
	if got := strings.Join(cfg.DataEncryptionKeysPrevious, "|"); got != "old-key-one-0123456789abcdef0123|old-key-two-0123456789abcdef0123" {
		t.Fatalf("previous keys = %q", got)
	}
	keyring, err := cfg.Keyring()
	if err != nil {
		t.Fatalf("Keyring: %v", err)
	}
	again, err := cfg.Keyring()
	if err != nil || again != keyring {
		t.Fatalf("expected the keyring to be reused for the same keys, got %p and %p (%v)", keyring, again, err)
	}
}

func TestKeyringRequiresAKey(t *testing.T) {
	if _, err := (Config{}).Keyring(); err == nil {
		t.Fatal("expected an error without DATA_ENCRYPTION_KEY")
	}
}

// 示例配置里的 change-me-… 长度合规，生产校验拦不住；必须至少在日志里提醒。
func TestSecurityWarningsFlagPlaceholderKeys(t *testing.T) {
	cases := map[string]struct {
		cfg  Config
		want int
	}{
		"example placeholders": {Config{DataEncryptionKey: "change-me-dev-data-encryption-key-32-bytes-min", JWTSecret: "change-me-dev-jwt-secret"}, 2},
		"built-in defaults":    {Config{DataEncryptionKey: defaultDataEncryptionKey, JWTSecret: defaultJWTSecret}, 2},
		"placeholder previous": {Config{DataEncryptionKey: "real-data-encryption-key-0123456789abcdef", JWTSecret: "real-jwt-secret-0123456789", DataEncryptionKeysPrevious: []string{"change-me-old"}}, 1},
		"real keys":            {Config{DataEncryptionKey: "real-data-encryption-key-0123456789abcdef", JWTSecret: "real-jwt-secret-0123456789"}, 0},
	}
	for name, tc := range cases {
		if got := tc.cfg.SecurityWarnings(); len(got) != tc.want {
			t.Errorf("%s: got %d warnings %q, want %d", name, len(got), got, tc.want)
		}
	}
}
