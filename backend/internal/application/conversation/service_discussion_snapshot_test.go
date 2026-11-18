package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type discussionSnapshotServiceRepoStub struct {
	repository.ConversationRepository
	snapshot *domaindiscussion.ConversationSnapshot
}

func (r *discussionSnapshotServiceRepoStub) GetConversationByUser(context.Context, uint, uint) (*model.Conversation, error) {
	return &model.Conversation{ID: 1, UserID: 2}, nil
}

func (r *discussionSnapshotServiceRepoStub) GetDiscussionSnapshot(context.Context, uint, string) (*domaindiscussion.ConversationSnapshot, error) {
	return r.snapshot, nil
}

func (r *discussionSnapshotServiceRepoStub) RefreshDiscussionSnapshot(_ context.Context, _ uint, _ string, pending domaindiscussion.Snapshot, expected int64) (*domaindiscussion.ConversationSnapshot, error) {
	if r.snapshot == nil {
		r.snapshot = &domaindiscussion.ConversationSnapshot{}
	}
	if r.snapshot.PendingVersion != expected {
		return nil, domaindiscussion.ErrSnapshotConflict
	}
	r.snapshot.Pending = &pending
	r.snapshot.PendingVersion++
	return r.snapshot, nil
}

func (r *discussionSnapshotServiceRepoStub) UpdateDiscussionSnapshotPending(_ context.Context, _ uint, _ string, pending domaindiscussion.Snapshot, expected int64) (*domaindiscussion.ConversationSnapshot, error) {
	if r.snapshot == nil || r.snapshot.Pending == nil || r.snapshot.PendingVersion != expected {
		return nil, domaindiscussion.ErrSnapshotConflict
	}
	r.snapshot.Pending = &pending
	return r.snapshot, nil
}

func (r *discussionSnapshotServiceRepoStub) RollbackDiscussionSnapshot(context.Context, uint, string, int64) (*domaindiscussion.ConversationSnapshot, error) {
	return r.snapshot, nil
}

func (r *discussionSnapshotServiceRepoStub) CommitDiscussionSnapshot(context.Context, uint, string, domaindiscussion.Snapshot, bool, int64) (*domaindiscussion.ConversationSnapshot, bool, error) {
	return r.snapshot, false, nil
}

type discussionTemplateProviderStub struct {
	template domaindiscussion.Template
	config   domaindiscussion.TemplateConfig
	calls    int
}

func (p *discussionTemplateProviderStub) ResolvePlan(_ context.Context, _ string, overrides map[string]string) (*domaindiscussion.Template, domaindiscussion.TemplateConfig, domaindiscussion.RuntimePlan, error) {
	p.calls++
	plan, err := domaindiscussion.ResolveRuntimePlan(p.config, overrides)
	return &p.template, p.config, plan, err
}

func discussionSnapshotServiceFixture(model string) domaindiscussion.Snapshot {
	config := domaindiscussion.TemplateConfig{
		Placeholders: []domaindiscussion.Placeholder{{Key: "p1", DefaultModel: model}},
		Rounds:       []domaindiscussion.Round{{Index: 1, Steps: []domaindiscussion.Step{{ID: "s1", Participant: "p1"}}}},
		Summary:      domaindiscussion.Summary{Participant: "p1"},
	}
	plan, _ := domaindiscussion.ResolveRuntimePlan(config, nil)
	return domaindiscussion.Snapshot{
		Template: domaindiscussion.Template{PublicID: "tpl", Name: "snapshot"},
		Config:   config,
		Plan:     plan,
	}
}

func TestResolveDiscussionExecutionIgnoresBindingsForActiveSnapshot(t *testing.T) {
	active := discussionSnapshotServiceFixture("model-pinned")
	repo := &discussionSnapshotServiceRepoStub{snapshot: &domaindiscussion.ConversationSnapshot{Active: &active, ActiveVersion: 1}}
	provider := &discussionTemplateProviderStub{}
	service := &Service{repo: repo, discussionTemplates: provider}
	execution, err := service.resolveDiscussionExecution(context.Background(), DiscussionSendInput{
		ConversationID: 1, UserID: 2, TemplatePublicID: "tpl", ModelBindings: map[string]string{"p1": "model-new"},
	})
	if err != nil {
		t.Fatalf("resolve active: %v", err)
	}
	if execution.plan.Summary.Model != "model-pinned" || execution.usePending {
		t.Fatalf("active snapshot was not pinned: %#v", execution.plan)
	}
	if provider.calls != 0 {
		t.Fatalf("live template was resolved %d times", provider.calls)
	}
}

func TestResolveDiscussionExecutionUpdatesPendingAndChecksVersion(t *testing.T) {
	pending := discussionSnapshotServiceFixture("model-old")
	repo := &discussionSnapshotServiceRepoStub{snapshot: &domaindiscussion.ConversationSnapshot{Pending: &pending, PendingVersion: 4}}
	service := &Service{repo: repo, discussionTemplates: &discussionTemplateProviderStub{}}
	_, err := service.resolveDiscussionExecution(context.Background(), DiscussionSendInput{TemplatePublicID: "tpl", ExpectedPendingVersion: 3})
	if !errors.Is(err, ErrDiscussionSnapshotConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	execution, err := service.resolveDiscussionExecution(context.Background(), DiscussionSendInput{
		TemplatePublicID: "tpl", ExpectedPendingVersion: 4, ModelBindings: map[string]string{"p1": "model-new"},
	})
	if err != nil {
		t.Fatalf("resolve pending: %v", err)
	}
	if !execution.usePending || execution.plan.Summary.Model != "model-new" || repo.snapshot.Pending.Plan.Summary.Model != "model-new" {
		t.Fatalf("pending binding was not applied: %#v", execution.plan)
	}
}

func TestRefreshDiscussionSnapshotKeepsExistingAndDefaultsNewBindings(t *testing.T) {
	active := discussionSnapshotServiceFixture("model-pinned")
	repo := &discussionSnapshotServiceRepoStub{snapshot: &domaindiscussion.ConversationSnapshot{Active: &active, ActiveVersion: 1}}
	provider := &discussionTemplateProviderStub{
		template: domaindiscussion.Template{PublicID: "tpl", Name: "latest"},
		config: domaindiscussion.TemplateConfig{
			Placeholders: []domaindiscussion.Placeholder{{Key: "p1", DefaultModel: "model-latest"}, {Key: "p2", DefaultModel: "model-new-default"}},
			Rounds:       []domaindiscussion.Round{{Index: 1, Steps: []domaindiscussion.Step{{ID: "s1", Participant: "p1"}, {ID: "s2", Participant: "p2"}}}},
			Summary:      domaindiscussion.Summary{Participant: "p1"},
		},
	}
	service := &Service{repo: repo, discussionTemplates: provider}
	current, err := service.RefreshDiscussionSnapshot(context.Background(), 2, 1, "tpl", nil, 0)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	bindings := domaindiscussion.PlanBindings(current.Pending.Plan)
	if bindings["p1"] != "model-pinned" || bindings["p2"] != "model-new-default" {
		t.Fatalf("unexpected refreshed bindings: %#v", bindings)
	}
	if current.Active.Plan.Summary.Model != "model-pinned" || current.PendingVersion != 1 {
		t.Fatalf("refresh changed active: %#v", current)
	}
}

func TestDiscussionSnapshotPayloadUsesCamelCase(t *testing.T) {
	active := discussionSnapshotServiceFixture("model-a")
	active.Config.Rounds[0].ParallelPrompt = "parallel prompt"
	encoded, err := json.Marshal(DiscussionSnapshotPayloadOf(&domaindiscussion.ConversationSnapshot{
		TemplatePublicID: "tpl",
		Active:           &active,
		ActiveVersion:    2,
	}))
	if err != nil {
		t.Fatalf("marshal snapshot payload: %v", err)
	}
	text := string(encoded)
	for _, key := range []string{`"templatePublicID"`, `"activeVersion"`, `"template"`, `"config"`, `"plan"`, `"publicID"`, `"parallelPrompt"`, `"summary"`} {
		if !strings.Contains(text, key) {
			t.Fatalf("missing %s in %s", key, text)
		}
	}
	for _, key := range []string{`"TemplatePublicID"`, `"Template"`, `"Config"`, `"Plan"`, `"PublicID"`} {
		if strings.Contains(text, key) {
			t.Fatalf("domain key %s leaked into %s", key, text)
		}
	}
	var decoded struct {
		Active struct {
			Template struct {
				Config struct {
					Placeholders []json.RawMessage `json:"placeholders"`
					Rounds       []json.RawMessage `json:"rounds"`
				} `json:"config"`
			} `json:"template"`
		} `json:"active"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot payload: %v", err)
	}
	if len(decoded.Active.Template.Config.Placeholders) == 0 || len(decoded.Active.Template.Config.Rounds) == 0 {
		t.Fatalf("template.config must remain usable by the chat binding bar: %s", text)
	}
}
