package skill

import (
	"context"
	"testing"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type seedFakeRepo struct {
	items       []domainskill.Skill
	nextID      uint
	createCalls int
	patchCalls  int
}

func (r *seedFakeRepo) ListSkills(_ context.Context, filter repository.SkillListFilter, offset int, limit int) ([]domainskill.Skill, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	matched := make([]domainskill.Skill, 0)
	for _, item := range r.items {
		if filter.Scope != "" && item.Scope != filter.Scope {
			continue
		}
		if filter.OwnerUserID != nil && item.OwnerUserID != *filter.OwnerUserID {
			continue
		}
		matched = append(matched, item)
	}
	total := int64(len(matched))
	if offset >= len(matched) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end], total, nil
}

func (r *seedFakeRepo) GetSkill(_ context.Context, id uint) (*domainskill.Skill, error) {
	for _, item := range r.items {
		if item.ID == id {
			result := item
			return &result, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *seedFakeRepo) CreateSkill(_ context.Context, item *domainskill.Skill) (*domainskill.Skill, error) {
	for _, existing := range r.items {
		if existing.Scope == item.Scope && existing.OwnerUserID == item.OwnerUserID && existing.Trigger == item.Trigger {
			return nil, repository.ErrDuplicate
		}
	}
	if r.nextID == 0 {
		r.nextID = 1
	}
	item.ID = r.nextID
	r.nextID++
	r.items = append(r.items, *item)
	r.createCalls++
	result := item
	return result, nil
}

func (r *seedFakeRepo) PatchSkill(_ context.Context, id uint, patch repository.SkillPatch) (*domainskill.Skill, error) {
	for index := range r.items {
		if r.items[index].ID != id {
			continue
		}
		if patch.Title != nil {
			r.items[index].Title = *patch.Title
		}
		if patch.Trigger != nil {
			r.items[index].Trigger = *patch.Trigger
		}
		if patch.Description != nil {
			r.items[index].Description = *patch.Description
		}
		if patch.Markdown != nil {
			r.items[index].Markdown = *patch.Markdown
		}
		if patch.Enabled != nil {
			r.items[index].Enabled = *patch.Enabled
		}
		if patch.SortOrder != nil {
			r.items[index].SortOrder = *patch.SortOrder
		}
		if patch.UpdatedByUserIDSet {
			r.items[index].UpdatedByUserID = patch.UpdatedByUserID
		}
		r.patchCalls++
		result := r.items[index]
		return &result, nil
	}
	return nil, repository.ErrNotFound
}

func (r *seedFakeRepo) DeleteSkill(_ context.Context, id uint) error {
	for index, item := range r.items {
		if item.ID == id {
			r.items = append(r.items[:index], r.items[index+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

func TestBuiltinSkillSeedsLoadAndValidate(t *testing.T) {
	seeds, err := loadBuiltinSkillSeeds()
	if err != nil {
		t.Fatalf("expected seeds to load, got %v", err)
	}
	if len(seeds) < 8 {
		t.Fatalf("expected at least 8 builtin seeds, got %d", len(seeds))
	}
	triggers := map[string]struct{}{}
	for _, seed := range seeds {
		if seed.Trigger == "" || seed.Title == "" || seed.Markdown == "" {
			t.Fatalf("seed %q has empty required fields", seed.Trigger)
		}
		if seed.SortOrder <= 0 {
			t.Fatalf("seed %q must declare positive sort", seed.Trigger)
		}
		if _, exists := triggers[seed.Trigger]; exists {
			t.Fatalf("duplicate seed trigger %q", seed.Trigger)
		}
		triggers[seed.Trigger] = struct{}{}
	}
	if _, exists := triggers["diagram-svg"]; !exists {
		t.Fatal("expected diagram-svg seed to exist")
	}
}

func TestEnsureBuiltinSeedsCreatesAndIsIdempotent(t *testing.T) {
	repo := &seedFakeRepo{}
	service := NewService(repo)
	if err := service.EnsureBuiltinSeeds(context.Background()); err != nil {
		t.Fatalf("expected seed run to succeed, got %v", err)
	}
	seeds, err := loadBuiltinSkillSeeds()
	if err != nil {
		t.Fatalf("expected seeds to load, got %v", err)
	}
	if len(repo.items) != len(seeds) {
		t.Fatalf("expected %d seeded skills, got %d", len(seeds), len(repo.items))
	}
	for _, item := range repo.items {
		if item.Scope != domainskill.ScopeBuiltin || item.OwnerUserID != 0 {
			t.Fatalf("seed %q must be builtin scope with owner 0", item.Trigger)
		}
		if !item.Enabled {
			t.Fatalf("seed %q must be enabled", item.Trigger)
		}
		if item.CreatedByUserID != 0 || item.UpdatedByUserID != 0 {
			t.Fatalf("seed %q must look untouched", item.Trigger)
		}
	}
	if err := service.EnsureBuiltinSeeds(context.Background()); err != nil {
		t.Fatalf("expected second seed run to succeed, got %v", err)
	}
	if repo.createCalls != len(seeds) || repo.patchCalls != 0 {
		t.Fatalf("expected no writes on second run, got create=%d patch=%d", repo.createCalls, repo.patchCalls)
	}
}

func TestEnsureBuiltinSeedsSyncsUntouchedAndSkipsEdited(t *testing.T) {
	repo := &seedFakeRepo{}
	service := NewService(repo)
	if err := service.EnsureBuiltinSeeds(context.Background()); err != nil {
		t.Fatalf("expected seed run to succeed, got %v", err)
	}
	var stale domainskill.Skill
	var edited domainskill.Skill
	for _, item := range repo.items {
		if item.Trigger == "diagram-svg" {
			stale = item
		}
		if item.Trigger == "code-review" {
			edited = item
		}
	}
	if stale.ID == 0 || edited.ID == 0 {
		t.Fatal("expected target seeds to exist")
	}
	// 模拟管理员改过内容的技能。
	if _, err := repo.PatchSkill(context.Background(), edited.ID, repository.SkillPatch{
		Markdown:           strPtr("admin rewrite"),
		UpdatedByUserIDSet: true,
		UpdatedByUserID:    9,
	}); err != nil {
		t.Fatalf("patch failed: %v", err)
	}
	// 模拟旧版本种子内容。
	if _, err := repo.PatchSkill(context.Background(), stale.ID, repository.SkillPatch{
		Markdown: strPtr("old seed content"),
	}); err != nil {
		t.Fatalf("patch failed: %v", err)
	}

	if err := service.EnsureBuiltinSeeds(context.Background()); err != nil {
		t.Fatalf("expected reseed to succeed, got %v", err)
	}
	var syncedStale domainskill.Skill
	for _, item := range repo.items {
		if item.Trigger == "diagram-svg" {
			syncedStale = item
		}
		if item.Trigger == "code-review" && item.Markdown != "admin rewrite" {
			t.Fatal("admin-edited skill must not be overwritten")
		}
	}
	if syncedStale.Markdown == "old seed content" {
		t.Fatal("untouched stale seed must be synced to latest content")
	}
	if syncedStale.UpdatedByUserID != 0 {
		t.Fatalf("synced seed must remain untouched, got updated_by=%d", syncedStale.UpdatedByUserID)
	}
}

func strPtr(value string) *string {
	return &value
}
