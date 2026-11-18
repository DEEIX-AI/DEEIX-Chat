package conversation

import (
	"context"
	"errors"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type resolvedDiscussionExecution struct {
	template     *domaindiscussion.Template
	config       domaindiscussion.TemplateConfig
	plan         domaindiscussion.RuntimePlan
	snapshot     *domaindiscussion.ConversationSnapshot
	snapshotRepo repository.DiscussionSnapshotRepository
	usePending   bool
}

func (s *Service) discussionSnapshotRepository() (repository.DiscussionSnapshotRepository, bool) {
	repo, ok := s.repo.(repository.DiscussionSnapshotRepository)
	return repo, ok && repo != nil
}

func (s *Service) GetDiscussionSnapshot(ctx context.Context, userID uint, conversationID uint, templatePublicID string) (*domaindiscussion.ConversationSnapshot, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, ErrConversationNotFound
	}
	repo, ok := s.discussionSnapshotRepository()
	if !ok {
		return nil, nil
	}
	return repo.GetDiscussionSnapshot(ctx, conversationID, templatePublicID)
}

func (s *Service) RefreshDiscussionSnapshot(ctx context.Context, userID uint, conversationID uint, templatePublicID string, models map[string]string, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, ErrConversationNotFound
	}
	repo, ok := s.discussionSnapshotRepository()
	if !ok {
		return nil, errors.New("discussion snapshot repository is not configured")
	}
	current, err := repo.GetDiscussionSnapshot(ctx, conversationID, templatePublicID)
	if err != nil {
		return nil, err
	}
	var basePlan domaindiscussion.RuntimePlan
	if current != nil {
		base := current.Active
		if current.Pending != nil {
			base = current.Pending
		}
		if base != nil {
			basePlan = base.Plan
		}
	}
	overrides := domaindiscussion.MergeBindings(basePlan, models)
	template, config, plan, err := s.discussionTemplates.ResolvePlan(ctx, templatePublicID, overrides)
	if err != nil {
		return nil, err
	}
	pending := domaindiscussion.Snapshot{Template: *template, Config: config, Plan: plan}
	return repo.RefreshDiscussionSnapshot(ctx, conversationID, templatePublicID, pending, expectedPendingVersion)
}

func (s *Service) RollbackDiscussionSnapshot(ctx context.Context, userID uint, conversationID uint, templatePublicID string, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, ErrConversationNotFound
	}
	repo, ok := s.discussionSnapshotRepository()
	if !ok {
		return nil, errors.New("discussion snapshot repository is not configured")
	}
	return repo.RollbackDiscussionSnapshot(ctx, conversationID, templatePublicID, expectedPendingVersion)
}

func (s *Service) AdoptDiscussionSnapshot(ctx context.Context, userID uint, conversationID uint, templatePublicID string, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error) {
	if _, err := s.repo.GetConversationByUser(ctx, conversationID, userID); err != nil {
		return nil, ErrConversationNotFound
	}
	repo, ok := s.discussionSnapshotRepository()
	if !ok {
		return nil, errors.New("discussion snapshot repository is not configured")
	}
	result, _, err := repo.CommitDiscussionSnapshot(ctx, conversationID, templatePublicID, domaindiscussion.Snapshot{}, true, expectedPendingVersion)
	return result, err
}

func (s *Service) resolveDiscussionExecution(ctx context.Context, input DiscussionSendInput) (*resolvedDiscussionExecution, error) {
	repo, supported := s.discussionSnapshotRepository()
	if !supported {
		template, config, plan, err := s.discussionTemplates.ResolvePlan(ctx, input.TemplatePublicID, input.ModelBindings)
		if err != nil {
			return nil, err
		}
		return &resolvedDiscussionExecution{template: template, config: config, plan: plan}, nil
	}
	snapshot, err := repo.GetDiscussionSnapshot(ctx, input.ConversationID, input.TemplatePublicID)
	if err != nil {
		return nil, err
	}
	if snapshot != nil && snapshot.Pending != nil {
		if input.ExpectedPendingVersion != snapshot.PendingVersion {
			return nil, ErrDiscussionSnapshotConflict
		}
		pending := *snapshot.Pending
		plan, err := domaindiscussion.ResolveRuntimePlan(pending.Config, domaindiscussion.MergeBindings(pending.Plan, input.ModelBindings))
		if err != nil {
			return nil, err
		}
		pending.Plan = plan
		snapshot, err = repo.UpdateDiscussionSnapshotPending(ctx, input.ConversationID, input.TemplatePublicID, pending, input.ExpectedPendingVersion)
		if err != nil {
			return nil, mapDiscussionSnapshotError(err)
		}
		template := pending.Template
		return &resolvedDiscussionExecution{
			template: &template, config: pending.Config, plan: plan, snapshot: snapshot,
			snapshotRepo: repo, usePending: true,
		}, nil
	}
	if input.ExpectedPendingVersion != 0 {
		return nil, ErrDiscussionSnapshotConflict
	}
	if snapshot != nil && snapshot.Active != nil {
		active := *snapshot.Active
		template := active.Template
		return &resolvedDiscussionExecution{
			template: &template, config: active.Config, plan: active.Plan, snapshot: snapshot, snapshotRepo: repo,
		}, nil
	}
	template, config, plan, err := s.discussionTemplates.ResolvePlan(ctx, input.TemplatePublicID, input.ModelBindings)
	if err != nil {
		return nil, err
	}
	return &resolvedDiscussionExecution{template: template, config: config, plan: plan, snapshot: snapshot, snapshotRepo: repo}, nil
}

func mapDiscussionSnapshotError(err error) error {
	switch {
	case errors.Is(err, domaindiscussion.ErrSnapshotConflict):
		return ErrDiscussionSnapshotConflict
	case errors.Is(err, domaindiscussion.ErrPendingNotFound):
		return ErrDiscussionPendingNotFound
	default:
		return err
	}
}
