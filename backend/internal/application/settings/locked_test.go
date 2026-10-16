package settings

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func lockedTestService(cfg config.Config) (*Service, *testSettingsRepo) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"billing": {
			{Namespace: "billing", Key: "mode", Value: "self"},
			{Namespace: "billing", Key: "payment_providers", Value: "disabled"},
			{Namespace: "billing", Key: "display_currency", Value: "USD"},
		},
		"desktop": {
			{Namespace: "desktop", Key: "download_enabled", Value: "true"},
			{Namespace: "desktop", Key: "download_url", Value: "https://deeix.com/download"},
		},
	}}
	service := NewService(repo, testKeyring())
	service.SetRuntime(config.NewRuntime(cfg))
	return service, repo
}

// 契约（docs/ARCHITECTURE.md §4）：billingGating 关闭时 billing.mode 与
// billing.payment_providers 不可修改，且整个补丁都不落库。
func TestBatchUpdateRejectsLockedKeysWhenGatingDisabled(t *testing.T) {
	service, repo := lockedTestService(config.Config{LocalMode: true})
	_, err := service.BatchUpdate(context.Background(), []PatchItem{
		{Namespace: "billing", Key: "display_currency", Value: "CNY"},
		{Namespace: "billing", Key: "payment_providers", Value: "stripe"},
		{Namespace: "billing", Key: "mode", Value: "usage"},
	})
	var disabled *FeatureDisabledError
	if !errors.As(err, &disabled) {
		t.Fatalf("expected FeatureDisabledError, got %v", err)
	}
	if !errors.Is(err, ErrFeatureDisabled) {
		t.Fatal("must unwrap to ErrFeatureDisabled")
	}
	if disabled.Feature != "billingGating" {
		t.Fatalf("feature = %q", disabled.Feature)
	}
	if want := []string{"billing.mode", "billing.payment_providers"}; !reflect.DeepEqual(disabled.Keys, want) {
		t.Fatalf("keys = %v, want %v", disabled.Keys, want)
	}
	for _, item := range repo.byNamespace["billing"] {
		if item.Key == "display_currency" && item.Value != "USD" {
			t.Fatal("a rejected batch must not be partially applied")
		}
	}
}

func TestBatchUpdateAllowsUnlockedBillingKeysWhenGatingDisabled(t *testing.T) {
	service, _ := lockedTestService(config.Config{LocalMode: true})
	// 计费配置接口总是连同当前 mode 一起提交；写回相同的值不是修改。
	if _, err := service.BatchUpdate(context.Background(), []PatchItem{
		{Namespace: "billing", Key: "mode", Value: "self"},
		{Namespace: "billing", Key: "display_currency", Value: "CNY"},
	}); err != nil {
		t.Fatalf("metering keys must stay writable: %v", err)
	}
}

func TestBatchUpdateLeavesLockedKeysAloneInServerMode(t *testing.T) {
	service, _ := lockedTestService(config.Config{})
	_, err := service.BatchUpdate(context.Background(), []PatchItem{
		{Namespace: "billing", Key: "mode", Value: "usage"},
	})
	if errors.Is(err, ErrFeatureDisabled) {
		t.Fatal("server mode must not lock billing.mode")
	}
}

// 本地模式只有桌面端自己连接，网页端下载入口没有对象：desktop.* 不可修改。
func TestBatchUpdateRejectsDesktopDownloadKeysInLocalMode(t *testing.T) {
	service, _ := lockedTestService(config.Config{LocalMode: true})
	_, err := service.BatchUpdate(context.Background(), []PatchItem{
		{Namespace: "desktop", Key: "download_enabled", Value: "false"},
	})
	var disabled *FeatureDisabledError
	if !errors.As(err, &disabled) || disabled.Feature != "desktopDownload" || !reflect.DeepEqual(disabled.Keys, []string{"desktop.download_enabled"}) {
		t.Fatalf("expected desktopDownload lock, got %v", err)
	}

	server, _ := lockedTestService(config.Config{})
	if _, err := server.BatchUpdate(context.Background(), []PatchItem{
		{Namespace: "desktop", Key: "download_url", Value: "https://example.com/download"},
	}); errors.Is(err, ErrFeatureDisabled) {
		t.Fatal("server mode must not lock the desktop download settings")
	}
}
