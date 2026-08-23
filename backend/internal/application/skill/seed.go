package skill

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gopkg.in/yaml.v3"
)

//go:embed seeddata/*.md
var builtinSeedFS embed.FS

const builtinSeedDir = "seeddata"

// builtinSkillSeed 描述一份内置技能种子（SKILL.md + 元数据）。
type builtinSkillSeed struct {
	Trigger     string
	Title       string
	Description string
	Markdown    string
	SortOrder   int
}

// builtinSkillFrontmatter 对应种子文件头部的 YAML 元数据。
type builtinSkillFrontmatter struct {
	Name        string `yaml:"name"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Sort        int    `yaml:"sort"`
}

// EnsureBuiltinSeeds 幂等补齐内置技能种子：缺失则创建；未被管理员改动过的
// 种子同步为最新内容；管理员创建或编辑过的技能一律跳过。
func (s *Service) EnsureBuiltinSeeds(ctx context.Context) error {
	seeds, err := loadBuiltinSkillSeeds()
	if err != nil {
		return err
	}
	existing, err := s.listBuiltinSkillsForSeed(ctx)
	if err != nil {
		return err
	}
	byTrigger := make(map[string]domainskill.Skill, len(existing))
	for _, item := range existing {
		byTrigger[normalizeTrigger(item.Trigger)] = item
	}
	for _, seed := range seeds {
		current, exists := byTrigger[seed.Trigger]
		if !exists {
			if err := s.createBuiltinSeed(ctx, seed); err != nil {
				return err
			}
			continue
		}
		if builtinSkillTouched(current) {
			continue
		}
		if current.Title == seed.Title &&
			current.Description == seed.Description &&
			current.Markdown == seed.Markdown &&
			current.SortOrder == seed.SortOrder {
			continue
		}
		if err := s.syncBuiltinSeed(ctx, current.ID, seed); err != nil {
			return err
		}
	}
	return nil
}

// builtinSkillTouched 判断内置技能是否由管理员创建或编辑过：
// 种子写入时 created/updated 均为 0，管理员一旦改动 updated 会变为真实用户 ID。
func builtinSkillTouched(item domainskill.Skill) bool {
	return item.CreatedByUserID != 0 || item.UpdatedByUserID != 0
}

func (s *Service) createBuiltinSeed(ctx context.Context, seed builtinSkillSeed) error {
	item, err := normalizeWriteInput(WriteInput{
		Title:       seed.Title,
		Trigger:     seed.Trigger,
		Description: seed.Description,
		Markdown:    seed.Markdown,
		Enabled:     true,
		SortOrder:   seed.SortOrder,
	}, domainskill.ScopeBuiltin, 0, 0)
	if err != nil {
		return fmt.Errorf("seed skill %q: %w", seed.Trigger, err)
	}
	if _, err := s.repo.CreateSkill(ctx, item); err != nil && !errors.Is(err, repository.ErrDuplicate) {
		return fmt.Errorf("create seed skill %q: %w", seed.Trigger, err)
	}
	return nil
}

func (s *Service) syncBuiltinSeed(ctx context.Context, id uint, seed builtinSkillSeed) error {
	title := seed.Title
	description := seed.Description
	markdown := seed.Markdown
	sortOrder := seed.SortOrder
	// 不携带 UpdatedByUserID，同步后仍视为“未改动”，后续升级可继续覆盖。
	_, err := s.repo.PatchSkill(ctx, id, repository.SkillPatch{
		Title:       &title,
		Description: &description,
		Markdown:    &markdown,
		SortOrder:   &sortOrder,
	})
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("sync seed skill %q: %w", seed.Trigger, err)
	}
	return nil
}

func (s *Service) listBuiltinSkillsForSeed(ctx context.Context) ([]domainskill.Skill, error) {
	const pageSize = 100
	var results []domainskill.Skill
	for page := 1; ; page++ {
		items, total, err := s.repo.ListSkills(ctx, repository.SkillListFilter{
			Scope: domainskill.ScopeBuiltin,
		}, (page-1)*pageSize, pageSize)
		if err != nil {
			return nil, err
		}
		results = append(results, items...)
		if int64(len(results)) >= total || len(items) == 0 {
			return results, nil
		}
	}
}

// loadBuiltinSkillSeeds 解析并校验全部内置技能种子文件。
func loadBuiltinSkillSeeds() ([]builtinSkillSeed, error) {
	entries, err := fs.ReadDir(builtinSeedFS, builtinSeedDir)
	if err != nil {
		return nil, err
	}
	seeds := make([]builtinSkillSeed, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := builtinSeedFS.ReadFile(builtinSeedDir + "/" + entry.Name())
		if err != nil {
			return nil, err
		}
		seed, err := parseBuiltinSkillSeed(entry.Name(), raw)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", builtinSeedDir, entry.Name(), err)
		}
		if _, exists := seen[seed.Trigger]; exists {
			return nil, fmt.Errorf("%s/%s: duplicate seed trigger %q", builtinSeedDir, entry.Name(), seed.Trigger)
		}
		seen[seed.Trigger] = struct{}{}
		seeds = append(seeds, seed)
	}
	if len(seeds) == 0 {
		return nil, errors.New("no builtin skill seeds found")
	}
	sort.Slice(seeds, func(i, j int) bool {
		if seeds[i].SortOrder != seeds[j].SortOrder {
			return seeds[i].SortOrder < seeds[j].SortOrder
		}
		return seeds[i].Trigger < seeds[j].Trigger
	})
	return seeds, nil
}

func parseBuiltinSkillSeed(name string, raw []byte) (builtinSkillSeed, error) {
	content := strings.TrimSpace(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	const open = "---\n"
	if !strings.HasPrefix(content, open) {
		return builtinSkillSeed{}, errors.New("missing frontmatter")
	}
	rest := strings.TrimPrefix(content, open)
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return builtinSkillSeed{}, errors.New("unterminated frontmatter")
	}
	var frontmatter builtinSkillFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &frontmatter); err != nil {
		return builtinSkillSeed{}, fmt.Errorf("invalid frontmatter: %w", err)
	}
	body := strings.TrimSpace(rest[end+len("\n---"):])

	trigger := normalizeTrigger(frontmatter.Name)
	title := strings.TrimSpace(frontmatter.Title)
	description := strings.TrimSpace(frontmatter.Description)
	if trigger == "" || runeCount(trigger) > maxSkillTriggerLength {
		return builtinSkillSeed{}, fmt.Errorf("invalid trigger %q", frontmatter.Name)
	}
	if title == "" || runeCount(title) > maxSkillTitleLength {
		return builtinSkillSeed{}, fmt.Errorf("invalid title %q", title)
	}
	if runeCount(description) > maxSkillDescriptionLength {
		return builtinSkillSeed{}, errors.New("description exceeds limit")
	}
	if body == "" || runeCount(body) > maxSkillMarkdownLength {
		return builtinSkillSeed{}, errors.New("markdown empty or exceeds limit")
	}
	if frontmatter.Sort <= 0 {
		return builtinSkillSeed{}, errors.New("sort must be positive")
	}
	return builtinSkillSeed{
		Trigger:     trigger,
		Title:       title,
		Description: description,
		Markdown:    body,
		SortOrder:   frontmatter.Sort,
	}, nil
}
