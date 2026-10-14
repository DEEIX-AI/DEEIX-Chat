package settings

import (
	"context"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// duplicateSettingGroup 描述同一 namespace+key 下的多行配置：Kept 为生效行，Removed 为应清理的其余行。
type duplicateSettingGroup struct {
	Kept    domainsettings.SystemSetting
	Removed []domainsettings.SystemSetting
}

// listAllSettings 读取全部配置并为每个 namespace+key 只保留生效行；所有读取路径都必须经由这里或 listNamespaceSettings。
func listAllSettings(ctx context.Context, repo repository.SettingsRepository) ([]domainsettings.SystemSetting, error) {
	items, err := repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	kept, _ := resolveDuplicateSettings(items)
	return kept, nil
}

// listNamespaceSettings 读取指定 namespace 的配置并消解重复行。
func listNamespaceSettings(ctx context.Context, repo repository.SettingsRepository, namespace string) ([]domainsettings.SystemSetting, error) {
	items, err := repo.ListByNamespace(ctx, namespace)
	if err != nil {
		return nil, err
	}
	kept, _ := resolveDuplicateSettings(items)
	return kept, nil
}

// resolveDuplicateSettings 为每个 namespace+key 选出一行生效配置，保持各 key 首次出现的顺序。
// 唯一索引正常时不会有重复；数据卷跨 libc 或排序规则迁移后文本索引的顺序会失效，
// 冲突检测查不到已有行，种子便再插入一行默认值。逐行覆盖会让哪一行生效取决于物理存储顺序，
// 因此这里按固定规则选行，保证管理员真正配置过的值不会被默认值盖掉。
func resolveDuplicateSettings(items []domainsettings.SystemSetting) ([]domainsettings.SystemSetting, []duplicateSettingGroup) {
	positions := make(map[string]int, len(items))
	kept := make([]domainsettings.SystemSetting, 0, len(items))
	removed := make(map[string][]domainsettings.SystemSetting)
	var duplicateKeys []string
	for _, item := range items {
		key := settingKey(item.Namespace, item.Key)
		position, seen := positions[key]
		if !seen {
			positions[key] = len(kept)
			kept = append(kept, item)
			continue
		}
		if _, recorded := removed[key]; !recorded {
			duplicateKeys = append(duplicateKeys, key)
		}
		if preferSetting(item, kept[position]) {
			removed[key] = append(removed[key], kept[position])
			kept[position] = item
		} else {
			removed[key] = append(removed[key], item)
		}
	}
	groups := make([]duplicateSettingGroup, 0, len(duplicateKeys))
	for _, key := range duplicateKeys {
		groups = append(groups, duplicateSettingGroup{Kept: kept[positions[key]], Removed: removed[key]})
	}
	return kept, groups
}

// preferSetting 判断 candidate 是否应取代 current 成为生效行：改动过的行优先于原样的种子行，
// 同类中更新时间较新的优先，最后取 id 较小的一行，保证每次读取结果一致。
func preferSetting(candidate domainsettings.SystemSetting, current domainsettings.SystemSetting) bool {
	if candidateTouched, currentTouched := settingTouched(candidate), settingTouched(current); candidateTouched != currentTouched {
		return candidateTouched
	}
	if !candidate.UpdatedAt.Equal(current.UpdatedAt) {
		return candidate.UpdatedAt.After(current.UpdatedAt)
	}
	return candidate.ID < current.ID
}

// settingTouched 判断该行是否已偏离种子写入时的原样：写入后被更新过，或值不同于注册表默认值。
// 种子写入时创建与更新时间相同；未注册的键只能依据时间判断。
func settingTouched(item domainsettings.SystemSetting) bool {
	if !item.UpdatedAt.Equal(item.CreatedAt) {
		return true
	}
	spec, ok := lookupSettingSpec(item.Namespace, item.Key)
	return ok && item.Value != spec.Default
}

// missingSettings 返回 existing 中尚不存在的默认配置。
// 种子不能只依赖 ON CONFLICT：唯一索引失效时冲突检测会漏掉已有行，按全表结果过滤才能避免再插入重复行。
func missingSettings(defaults []domainsettings.SystemSetting, existing []domainsettings.SystemSetting) []domainsettings.SystemSetting {
	present := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		present[settingKey(item.Namespace, item.Key)] = struct{}{}
	}
	missing := make([]domainsettings.SystemSetting, 0)
	for _, item := range defaults {
		if _, ok := present[settingKey(item.Namespace, item.Key)]; !ok {
			missing = append(missing, item)
		}
	}
	return missing
}

// repairDuplicateSettings 删除重复行并重建唯一索引。重复行意味着索引已失效，
// 同一原因通常也会波及其它表的文本唯一索引，因此日志提示运维对整个数据库执行 REINDEX。
// 修复失败不阻断启动：读取路径已能稳定选中生效行，下次启动会再次尝试。
func (s *Service) repairDuplicateSettings(ctx context.Context, groups []duplicateSettingGroup) {
	if len(groups) == 0 {
		return
	}
	ids := make([]uint, 0, len(groups))
	for _, group := range groups {
		removedIDs := make([]uint, 0, len(group.Removed))
		for _, item := range group.Removed {
			removedIDs = append(removedIDs, item.ID)
		}
		ids = append(ids, removedIDs...)
		s.warn("system setting has duplicate rows",
			zap.String("setting", settingKey(group.Kept.Namespace, group.Kept.Key)),
			zap.Uint("kept_id", group.Kept.ID),
			zap.Uints("removed_ids", removedIDs),
		)
	}
	if err := s.repo.RemoveDuplicates(ctx, ids); err != nil {
		s.warn("remove duplicate system settings failed", zap.Uints("ids", ids), zap.Error(err))
		return
	}
	s.warn("duplicate system settings removed and system_settings reindexed; "+
		"duplicates mean the database's text indexes were built under a different collation, "+
		"run REINDEX DATABASE to repair the remaining tables",
		zap.Int("removed", len(ids)),
	)
}

func (s *Service) warn(message string, fields ...zap.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, fields...)
}
