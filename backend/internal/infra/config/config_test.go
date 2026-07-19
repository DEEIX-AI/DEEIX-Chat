package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsUseBootstrapAdmin(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())

	cfg := Load()
	if cfg.Env != "prod" {
		t.Fatalf("expected default env prod, got %q", cfg.Env)
	}
	if cfg.AdminUsername != defaultAdminUsername {
		t.Fatalf("expected default admin username %q, got %q", defaultAdminUsername, cfg.AdminUsername)
	}
	if cfg.AdminDisplayName != defaultAdminDisplayName {
		t.Fatalf("expected default admin display name %q, got %q", defaultAdminDisplayName, cfg.AdminDisplayName)
	}
}

func TestLoadTreatsBlankAPPEnvAsUnset(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())
	t.Setenv("APP_ENV", " ")

	cfg := Load()
	if cfg.Env != "prod" {
		t.Fatalf("expected blank APP_ENV to default to prod, got %q", cfg.Env)
	}
}

func TestLoadNormalizesAPPEnvAliases(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "development", env: "development", want: "dev"},
		{name: "production", env: "production", want: "prod"},
		{name: "trim and case", env: " Production ", want: "prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupConfigEnv(t)
			chdir(t, t.TempDir())
			t.Setenv("APP_ENV", tt.env)

			cfg := Load()
			if cfg.Env != tt.want {
				t.Fatalf("expected APP_ENV %q to normalize to %q, got %q", tt.env, tt.want, cfg.Env)
			}
		})
	}
}

func TestLoadNormalizesLegacyPostgresDSNTimeZone(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())
	t.Setenv("POSTGRES_DSN", "postgres://deeix_chat:secret%2Fvalue@postgres:5432/deeix_chat?sslmode=disable&TimeZone=Asia%2FShanghai")

	cfg := Load()
	if cfg.PostgresDSN != "postgres://deeix_chat:secret%2Fvalue@postgres:5432/deeix_chat?sslmode=disable&TimeZone=Asia/Shanghai" {
		t.Fatalf("expected legacy timezone to be normalized without decoding credentials, got %q", cfg.PostgresDSN)
	}
}

func TestNormalizePostgresDSNTimeZone(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "url",
			dsn:  "postgres://user:pass%2Fword@postgres:5432/db?sslmode=disable&TimeZone=Asia%2FShanghai",
			want: "postgres://user:pass%2Fword@postgres:5432/db?sslmode=disable&TimeZone=Asia/Shanghai",
		},
		{
			name: "key value",
			dsn:  "host=postgres user=deeix password=secret%2Fvalue dbname=deeix sslmode=disable TimeZone=Asia%2FShanghai",
			want: "host=postgres user=deeix password=secret%2Fvalue dbname=deeix sslmode=disable TimeZone=Asia/Shanghai",
		},
		{
			name: "already normalized",
			dsn:  "host=postgres user=deeix dbname=deeix sslmode=disable TimeZone=Asia/Shanghai",
			want: "host=postgres user=deeix dbname=deeix sslmode=disable TimeZone=Asia/Shanghai",
		},
		{
			name: "other percent encoded fields unchanged",
			dsn:  "postgres://user:pass@postgres:5432/db?application_name=DEEIX%2FChat&sslmode=disable",
			want: "postgres://user:pass@postgres:5432/db?application_name=DEEIX%2FChat&sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePostgresDSN(tt.dsn); got != tt.want {
				t.Fatalf("normalizePostgresDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadReadsRepositoryRootConfigFromBackendWorkingDirectory(t *testing.T) {
	cleanupConfigEnv(t)

	root := filepath.Join(t.TempDir(), "repo")
	backendDir := filepath.Join(root, "backend")
	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		t.Fatalf("create backend dir: %v", err)
	}
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
		backendDir = filepath.Join(root, "backend")
	}
	configPath := filepath.Join(root, "config.yaml")
	configBody := []byte(`
server:
  frontend_dist_dir: ./frontend/out
storage:
  local:
    root_dir: ./data/storage
geoip:
  database_path: ./data/geoip.mmdb
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}
	chdir(t, backendDir)

	cfg := Load()
	if cfg.AdminUsername != defaultAdminUsername {
		t.Fatalf("expected built-in admin username, got %q", cfg.AdminUsername)
	}
	if cfg.AdminDisplayName != defaultAdminDisplayName {
		t.Fatalf("expected built-in admin display name, got %q", cfg.AdminDisplayName)
	}
	assertPath(t, "frontend dist", cfg.FrontendDistDir, filepath.Join(root, "frontend", "out"))
	assertPath(t, "storage root", cfg.StorageRootDir, filepath.Join(root, "data", "storage"))
	assertPath(t, "geoip database", cfg.GeoIPDatabasePath, filepath.Join(root, "data", "geoip.mmdb"))
}

func TestLoadReadsTurnstileSiteverifyURL(t *testing.T) {
	cleanupConfigEnv(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := []byte(`
security:
  turnstile_siteverify_url: "https://turnstile.example.test/siteverify"
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := Load()
	if cfg.TurnstileSiteverifyURL != "https://turnstile.example.test/siteverify" {
		t.Fatalf("expected turnstile siteverify url from config, got %q", cfg.TurnstileSiteverifyURL)
	}

	t.Setenv("TURNSTILE_SITEVERIFY_URL", "https://turnstile-env.example.test/siteverify")
	cfg = Load()
	if cfg.TurnstileSiteverifyURL != "https://turnstile-env.example.test/siteverify" {
		t.Fatalf("expected turnstile siteverify url from env, got %q", cfg.TurnstileSiteverifyURL)
	}
}

func TestLoadReadsBrandingFromConfig(t *testing.T) {
	cleanupConfigEnv(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := []byte(`
app:
  name: Example Backend
branding:
  title: Example Chat
  short_name: Example
  description: Example description
  logo_url: https://cdn.example.com/logo.svg
  favicon_url: https://cdn.example.com/favicon.ico
  pwa_icon_192_url: https://cdn.example.com/icon-192.png
  pwa_icon_512_url: https://cdn.example.com/icon-512.png
  pwa_maskable_icon_512_url: https://cdn.example.com/icon-maskable.png
  apple_touch_icon_180_url: https://cdn.example.com/apple-touch-icon.png
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := Load()
	if cfg.AppName != "Example Backend" || cfg.BrandTitle != "Example Chat" || cfg.BrandShortName != "Example" || cfg.BrandDescription != "Example description" {
		t.Fatalf("unexpected branding text: %+v", cfg)
	}
	if cfg.BrandLogoURL != "https://cdn.example.com/logo.svg" ||
		cfg.BrandFaviconURL != "https://cdn.example.com/favicon.ico" ||
		cfg.BrandPWAIcon192URL != "https://cdn.example.com/icon-192.png" ||
		cfg.BrandPWAIcon512URL != "https://cdn.example.com/icon-512.png" ||
		cfg.BrandPWAMaskableIcon512URL != "https://cdn.example.com/icon-maskable.png" ||
		cfg.BrandAppleTouchIcon180URL != "https://cdn.example.com/apple-touch-icon.png" {
		t.Fatalf("unexpected branding assets: %+v", cfg)
	}
}

func TestValidateAllowsOnlyDevAndProdEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{name: "dev", env: "dev"},
		{name: "prod", env: "prod"},
		{name: "development alias", env: "development"},
		{name: "production alias", env: "production"},
		{name: "staging rejected", env: "staging", wantErr: true},
		{name: "empty rejected", env: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfigForEnv(tt.env)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func validConfigForEnv(env string) Config {
	return Config{
		Env:               env,
		StorageBackend:    "local",
		JWTSecret:         "test-jwt-secret-value",
		DataEncryptionKey: "test-data-encryption-key-value-32",
		CORSAllowOrigin:   "https://example.com",
		PublicAPIBaseURL:  "https://api.example.com",
		PublicWebBaseURL:  "https://example.com",
	}
}

func cleanupConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"CONFIG_FILE",
		"APP_ENV",
		"FRONTEND_DIST_DIR",
		"STORAGE_ROOT_DIR",
		"GEOIP_DATABASE_PATH",
		"TURNSTILE_SITEVERIFY_URL",
		"POSTGRES_DSN",
	}
	for _, key := range keys {
		key := key
		original, ok := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, original)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err = os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}

func assertPath(t *testing.T, label string, got string, want string) {
	t.Helper()
	gotPath := canonicalPath(t, got)
	wantPath := canonicalPath(t, want)
	if gotPath != wantPath {
		t.Fatalf("expected %s path %q, got %q", label, wantPath, gotPath)
	}
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	cleaned := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err == nil {
		return resolved
	}
	return cleaned
}
