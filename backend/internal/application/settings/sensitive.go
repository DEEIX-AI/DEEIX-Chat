package settings

import (
	"context"
	"strings"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
)

var sensitiveSettingKeys = map[string]struct{}{
	"auth:smtp_password":                   {},
	"billing:stripe_secret_key":            {},
	"billing:stripe_webhook_secret":        {},
	"billing:epay_key":                     {},
	"extract:tika_auth_token":              {},
	"extract:docling_auth_token":           {},
	"extract:tesseract_ocr_auth_token":     {},
	"extract:rapidocr_auth_token":          {},
	"extract:paddle_ocr_auth_token":        {},
	"extract:tencent_ocr_secret_key":       {},
	"extract:aliyun_ocr_access_key_secret": {},
	"extract:mineru_auth_token":            {},
	"extract:llm_ocr_auth_token":           {},
	"file:embedding_key":                   {},
}

func isSensitiveSetting(namespace string, key string) bool {
	_, ok := sensitiveSettingKeys[settingKey(namespace, key)]
	return ok
}

func settingKey(namespace string, key string) string {
	return strings.TrimSpace(namespace) + ":" + strings.TrimSpace(key)
}

func configuredSettingValue(value string) bool {
	return strings.TrimSpace(value) != ""
}

func (s *Service) settingResponse(item domainsettings.SystemSetting) SettingItem {
	sensitive := isSensitiveSetting(item.Namespace, item.Key)
	value := item.Value
	if sensitive {
		value = ""
	}
	return SettingItem{
		Key:         item.Key,
		Value:       value,
		ValueType:   item.ValueType,
		Description: item.Description,
		Sensitive:   sensitive,
		Configured:  configuredSettingValue(item.Value),
	}
}

func (s *Service) encryptSettingForStorage(item domainsettings.SystemSetting) (domainsettings.SystemSetting, error) {
	if !isSensitiveSetting(item.Namespace, item.Key) || strings.TrimSpace(item.Value) == "" {
		return item, nil
	}
	encrypted, err := secretbox.EncryptString(s.dataEncryptionKey, item.Value)
	if err != nil {
		return item, err
	}
	item.Value = encrypted
	return item, nil
}

func (s *Service) decryptSettingValue(item domainsettings.SystemSetting) (string, error) {
	if !isSensitiveSetting(item.Namespace, item.Key) || strings.TrimSpace(item.Value) == "" {
		return item.Value, nil
	}
	return secretbox.DecryptString(s.dataEncryptionKey, item.Value)
}

func (s *Service) encryptSettingsForStorage(items []domainsettings.SystemSetting) ([]domainsettings.SystemSetting, error) {
	results := make([]domainsettings.SystemSetting, 0, len(items))
	for _, item := range items {
		next, err := s.encryptSettingForStorage(item)
		if err != nil {
			return nil, err
		}
		results = append(results, next)
	}
	return results, nil
}

func (s *Service) preparePatchItemsForStorage(patches []PatchItem) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0, len(patches))
	for _, p := range patches {
		if isSensitiveSetting(p.Namespace, p.Key) && strings.TrimSpace(p.Value) == "" && !p.Clear {
			continue
		}
		item := domainsettings.SystemSetting{
			Namespace: p.Namespace,
			Key:       p.Key,
			Value:     p.Value,
		}
		if p.Clear {
			item.Value = ""
		}
		next, err := s.encryptSettingForStorage(item)
		if err != nil {
			return nil, err
		}
		items = append(items, next)
	}
	return items, nil
}

func (s *Service) loadEffectiveSettings(ctx context.Context, namespaces ...string) (map[string]string, error) {
	result := make(map[string]string)
	for _, namespace := range namespaces {
		items, err := s.repo.ListByNamespace(ctx, namespace)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			result[settingKey(item.Namespace, item.Key)] = strings.TrimSpace(item.Value)
		}
	}
	return result, nil
}

func applyPatchesToEffectiveSettings(next map[string]string, patches []PatchItem, namespaces ...string) {
	allowed := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		allowed[namespace] = struct{}{}
	}
	for _, item := range patches {
		if _, ok := allowed[item.Namespace]; !ok {
			continue
		}
		if isSensitiveSetting(item.Namespace, item.Key) && strings.TrimSpace(item.Value) == "" && !item.Clear {
			continue
		}
		if item.Clear {
			next[settingKey(item.Namespace, item.Key)] = ""
			continue
		}
		next[settingKey(item.Namespace, item.Key)] = strings.TrimSpace(item.Value)
	}
}
