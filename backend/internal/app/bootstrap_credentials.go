package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func writeBootstrapSuperAdminPasswordFile(cfg config.Config, username string, password string) (string, error) {
	dir := bootstrapCredentialDir(cfg)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create bootstrap credential dir: %w", err)
	}
	path := filepath.Join(dir, "bootstrap-superadmin.password")
	content := fmt.Sprintf("username=%s\npassword=%s\n", strings.TrimSpace(username), password)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write bootstrap password file: %w", err)
	}
	return path, nil
}

func bootstrapCredentialDir(cfg config.Config) string {
	if root := strings.TrimSpace(cfg.StorageRootDir); root != "" {
		return filepath.Join(root, "secrets")
	}
	if sqlitePath := strings.TrimSpace(cfg.SQLitePath); sqlitePath != "" {
		return filepath.Join(filepath.Dir(sqlitePath), "secrets")
	}
	return filepath.Join(".", "data", "secrets")
}
