package contentmoderation

import (
	"context"
	"errors"
	"testing"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestGetEventDetailOnlyMapsRepositoryNotFound(t *testing.T) {
	storageErr := errors.New("database unavailable")
	tests := []struct {
		name      string
		repo      *coordinatorTestRepo
		wantError error
	}{
		{
			name:      "not found",
			repo:      &coordinatorTestRepo{getEventErr: repository.ErrNotFound},
			wantError: ErrEventNotFound,
		},
		{
			name:      "storage failure",
			repo:      &coordinatorTestRepo{getEventErr: storageErr},
			wantError: storageErr,
		},
		{
			name: "event",
			repo: &coordinatorTestRepo{getEvent: &domaincm.Event{
				PublicID:           "cme_1",
				CategoriesJSON:     "[]",
				CategoryScoresJSON: "{}",
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(nil, test.repo, "test-key", nil)
			detail, err := service.GetEventDetail(context.Background(), "superadmin", "cme_1")
			if test.wantError != nil {
				if !errors.Is(err, test.wantError) {
					t.Fatalf("error = %v, want %v", err, test.wantError)
				}
				return
			}
			if err != nil || detail == nil {
				t.Fatalf("detail = %#v, error = %v", detail, err)
			}
		})
	}
}

func TestOpenEventImagePreservesRepositoryFailure(t *testing.T) {
	storageErr := errors.New("database unavailable")
	service := NewService(nil, &coordinatorTestRepo{getEventErr: storageErr}, "test-key", nil)
	_, _, err := service.OpenEventImage(context.Background(), "superadmin", "cme_1", 0)
	if !errors.Is(err, storageErr) {
		t.Fatalf("error = %v, want storage error", err)
	}
}

func TestListEventsRejectsInvalidFilters(t *testing.T) {
	service := NewService(nil, &coordinatorTestRepo{}, "test-key", nil)
	tests := []EventListInput{
		{Direction: "sideways"},
		{Modality: "video"},
		{Result: "blocked"},
		{Category: "%"},
	}
	for _, input := range tests {
		if _, _, err := service.ListEvents(context.Background(), "superadmin", input); !errors.Is(err, ErrInvalidEventFilter) {
			t.Fatalf("input %#v returned %v, want invalid filter", input, err)
		}
	}
}
