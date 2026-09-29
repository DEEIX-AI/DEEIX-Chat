package settings

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

// ErrFeatureDisabled 表示补丁命中了当前能力位下锁定的设置项。
var ErrFeatureDisabled = errors.New("feature disabled")

// FeatureDisabledError 报告哪个能力位锁定了哪些键；契约见 docs/ARCHITECTURE.md §4。
type FeatureDisabledError struct {
	Feature string
	Keys    []string
}

func (e *FeatureDisabledError) Error() string { return ErrFeatureDisabled.Error() + ": " + e.Feature }

func (e *FeatureDisabledError) Unwrap() error { return ErrFeatureDisabled }

// lockedSettings 列出能力位关闭时不可修改的设置项。这些键决定门禁是否存在，
// 而不是门禁的参数；本地模式固定为自用计费，改了也没有对象。
var lockedSettings = []struct {
	feature string
	keys    []string
}{
	{feature: "billingGating", keys: []string{"billing.mode", "billing.payment_providers"}},
}

// SetRuntime 注入运行时配置，锁定键按其能力位判断；未注入时不锁定任何键。
func (s *Service) SetRuntime(runtime *config.Runtime) {
	s.runtime = runtime
}

// rejectLockedSettings 在写入前拒绝会改变锁定键的整个补丁，不做部分应用。
// 写回当前值不算修改：计费配置接口总是把 mode 一起提交。
func (s *Service) rejectLockedSettings(ctx context.Context, patches []PatchItem) error {
	if s.runtime == nil {
		return nil
	}
	caps := s.runtime.Snapshot().Capabilities()
	for _, locked := range lockedSettings {
		if caps.Enabled(locked.feature) {
			continue
		}
		var hit []string
		for _, patch := range patches {
			for _, key := range locked.keys {
				if patch.Namespace+"."+patch.Key != key {
					continue
				}
				current, err := s.currentValue(ctx, patch.Namespace, patch.Key)
				if err != nil {
					return err
				}
				if patch.Clear || strings.TrimSpace(patch.Value) != current {
					hit = append(hit, key)
				}
			}
		}
		if len(hit) > 0 {
			sort.Strings(hit)
			return &FeatureDisabledError{Feature: locked.feature, Keys: hit}
		}
	}
	return nil
}

func (s *Service) currentValue(ctx context.Context, namespace, key string) (string, error) {
	items, err := s.repo.ListByNamespace(ctx, namespace)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.Key == key {
			return strings.TrimSpace(item.Value), nil
		}
	}
	return "", nil
}
