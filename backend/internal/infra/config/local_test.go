package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyLocalModeSwitchesToLocalInfrastructureAndPassesProdValidation(t *testing.T) {
	dir := t.TempDir()
	cfg := Load()
	if err := cfg.ApplyLocalMode(dir); err != nil {
		t.Fatalf("ApplyLocalMode: %v", err)
	}

	if !cfg.LocalMode || cfg.LocalDataDir != dir {
		t.Fatalf("local mode flags not set: %+v", cfg)
	}
	if cfg.DatabaseDriver != "sqlite" || cfg.SQLitePath != filepath.Join(dir, "deeix.db") {
		t.Fatalf("database must be sqlite under the data dir, got %s %s", cfg.DatabaseDriver, cfg.SQLitePath)
	}
	if cfg.CacheDriver != "memory" || cfg.StorageBackend != "local" || !strings.HasPrefix(cfg.StorageRootDir, dir) {
		t.Fatalf("cache/storage must be local: %s %s %s", cfg.CacheDriver, cfg.StorageBackend, cfg.StorageRootDir)
	}
	if cfg.HTTPListenAddr != LocalListenAddr {
		t.Fatalf("must bind loopback only, got %q", cfg.HTTPListenAddr)
	}
	if cfg.GeoIPProvider != "none" {
		t.Fatalf("no outbound geo lookups in local mode, got %q", cfg.GeoIPProvider)
	}
	if cfg.Env != "prod" {
		t.Fatalf("local mode must keep production validation, got env %q", cfg.Env)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local config must pass production validation: %v", err)
	}
	if strings.Contains(cfg.CORSAllowOrigin, "*") || !strings.Contains(cfg.CORSAllowOrigin, "tauri://localhost") {
		t.Fatalf("CORS must allow the desktop webview only: %q", cfg.CORSAllowOrigin)
	}
}

func TestApplyLocalModeGeneratesPerInstallSecretsOnce(t *testing.T) {
	dir := t.TempDir()
	first := Load()
	if err := first.ApplyLocalMode(dir); err != nil {
		t.Fatal(err)
	}
	if first.JWTSecret == defaultJWTSecret || first.DataEncryptionKey == defaultDataEncryptionKey {
		t.Fatal("local mode must never run with the well-known dev secrets")
	}
	if len(first.JWTSecret) < 32 || len(first.DataEncryptionKey) < 32 {
		t.Fatal("generated secrets are too short")
	}

	info, err := os.Stat(filepath.Join(dir, localSecretsFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("secrets file must be 0600, got %o", perm)
	}

	second := Load()
	if err := second.ApplyLocalMode(dir); err != nil {
		t.Fatal(err)
	}
	if second.JWTSecret != first.JWTSecret || second.DataEncryptionKey != first.DataEncryptionKey {
		t.Fatal("secrets must be stable across restarts, otherwise encrypted data and sessions are lost")
	}
}

func TestApplyLocalModeRejectsBadInput(t *testing.T) {
	cfg := Load()
	if err := cfg.ApplyLocalMode("relative/dir"); err == nil {
		t.Fatal("relative data dir must be rejected")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, localSecretsFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ApplyLocalMode(dir); err == nil {
		t.Fatal("corrupt secrets file must fail loudly instead of silently regenerating (that would orphan encrypted data)")
	}
}
