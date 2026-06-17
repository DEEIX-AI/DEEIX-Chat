package skill

import (
	"context"
	"errors"
	"testing"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type fakeSkillRepo struct {
	items map[uint]domainskill.Skill
	next  uint
}

func (r *fakeSkillRepo) ListSkills(context.Context, repository.SkillListFilter, int, int) ([]domainskill.Skill, int64, error) {
	return nil, 0, nil
}

func (r *fakeSkillRepo) GetSkill(_ context.Context, id uint) (*domainskill.Skill, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &item, nil
}

func (r *fakeSkillRepo) CreateSkill(_ context.Context, item *domainskill.Skill) (*domainskill.Skill, error) {
	if r.next == 0 {
		r.next = 1
	}
	item.ID = r.next
	r.next++
	r.items[item.ID] = *item
	result := r.items[item.ID]
	return &result, nil
}

func (r *fakeSkillRepo) PatchSkill(context.Context, uint, repository.SkillPatch) (*domainskill.Skill, error) {
	return nil, nil
}

func (r *fakeSkillRepo) DeleteSkill(context.Context, uint) error {
	return nil
}

func TestCreateUserRequiresMarkdown(t *testing.T) {
	service := NewService(&fakeSkillRepo{items: map[uint]domainskill.Skill{}})
	_, err := service.CreateUser(context.Background(), 7, WriteInput{
		Title:   "Review",
		Trigger: "review",
		Enabled: true,
	})
	if !errors.Is(err, ErrInvalidSkill) {
		t.Fatalf("expected ErrInvalidSkill, got %v", err)
	}
}

func TestCreateUserStoresMarkdown(t *testing.T) {
	service := NewService(&fakeSkillRepo{items: map[uint]domainskill.Skill{}})
	item, err := service.CreateUser(context.Background(), 7, WriteInput{
		Title:       "Review",
		Trigger:     "/review",
		Description: "Review code",
		Markdown:    "Review the submitted code and return prioritized findings.",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("expected skill to be created, got %v", err)
	}
	if item.Trigger != "review" {
		t.Fatalf("expected normalized trigger, got %q", item.Trigger)
	}
	if item.Markdown != "Review the submitted code and return prioritized findings." {
		t.Fatalf("unexpected markdown: %q", item.Markdown)
	}
}

func TestResolveAvailableEnforcesVisibility(t *testing.T) {
	service := NewService(&fakeSkillRepo{items: map[uint]domainskill.Skill{
		1: {ID: 1, Scope: domainskill.ScopeUser, OwnerUserID: 8, Enabled: true, Title: "Private"},
		2: {ID: 2, Scope: domainskill.ScopeBuiltin, Enabled: true, Title: "Builtin"},
	}})

	if _, err := service.ResolveAvailable(context.Background(), 7, 1); !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("expected ErrSkillNotFound, got %v", err)
	}
	item, err := service.ResolveAvailable(context.Background(), 7, 2)
	if err != nil {
		t.Fatalf("expected builtin visible, got %v", err)
	}
	if item.ID != 2 {
		t.Fatalf("expected id 2, got %d", item.ID)
	}
}
