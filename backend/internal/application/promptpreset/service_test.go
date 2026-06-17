package promptpreset

import (
	"context"
	"errors"
	"strings"
	"testing"

	domainpromptpreset "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/promptpreset"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type fakePromptPresetRepo struct {
	items map[uint]domainpromptpreset.PromptPreset
}

func (r *fakePromptPresetRepo) ListPromptPresets(context.Context, repository.PromptPresetListFilter, int, int) ([]domainpromptpreset.PromptPreset, int64, error) {
	return nil, 0, nil
}

func (r *fakePromptPresetRepo) GetPromptPreset(_ context.Context, id uint) (*domainpromptpreset.PromptPreset, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &item, nil
}

func (r *fakePromptPresetRepo) CreatePromptPreset(_ context.Context, item *domainpromptpreset.PromptPreset) (*domainpromptpreset.PromptPreset, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	copy := *item
	copy.ID = uint(len(r.items) + 1)
	r.items[copy.ID] = copy
	return &copy, nil
}

func (r *fakePromptPresetRepo) PatchPromptPreset(context.Context, uint, repository.PromptPresetPatch) (*domainpromptpreset.PromptPreset, error) {
	return nil, nil
}

func (r *fakePromptPresetRepo) DeletePromptPreset(context.Context, uint) error {
	return nil
}

func TestCreateUserAllowsChineseTrigger(t *testing.T) {
	t.Parallel()

	service := NewService(&fakePromptPresetRepo{items: map[uint]domainpromptpreset.PromptPreset{}})
	trigger := strings.Repeat("中", maxPromptPresetNameLength)
	item, err := service.CreateUser(context.Background(), 1, WriteInput{
		Title:   trigger,
		Trigger: "/" + trigger,
		Content: "帮助优化中文表达。",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("expected Chinese trigger to pass, got %v", err)
	}
	if item.Trigger != trigger {
		t.Fatalf("expected slash prefix to be normalized, got %q", item.Trigger)
	}

	_, err = service.CreateUser(context.Background(), 1, WriteInput{
		Title:   trigger + "文",
		Trigger: trigger + "文",
		Content: "帮助优化中文表达。",
		Enabled: true,
	})
	if !errors.Is(err, ErrInvalidPromptPreset) {
		t.Fatalf("expected overlong Chinese trigger to fail, got %v", err)
	}
}

func TestUserCannotUpdateBuiltinPromptPreset(t *testing.T) {
	t.Parallel()

	service := NewService(&fakePromptPresetRepo{items: map[uint]domainpromptpreset.PromptPreset{
		10: {
			ID:      10,
			Scope:   domainpromptpreset.ScopeBuiltin,
			Trigger: "musk",
			Enabled: true,
		},
	}})

	_, err := service.UpdateUser(context.Background(), 1, 10, PatchInput{Title: stringPtr("Mine")})
	if !errors.Is(err, ErrPromptPresetNotFound) {
		t.Fatalf("expected builtin prompt to be hidden from user updates, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
