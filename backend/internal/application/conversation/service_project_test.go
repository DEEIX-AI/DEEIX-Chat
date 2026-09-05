package conversation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestNormalizeConversationProjectInputInheritClearsMCPDefaults(t *testing.T) {
	input, err := normalizeConversationProjectInput(ConversationProjectInput{
		Name:              " Project ",
		DefaultModel:      " model-a ",
		MCPDefaultMode:    domainconversation.ConversationProjectMCPDefaultModeInherit,
		DefaultMCPToolIDs: []uint{3, 3, 2},
		DefaultSkillIDs:   []uint{5, 0, 5, 4},
	})
	if err != nil {
		t.Fatalf("normalizeConversationProjectInput() error = %v", err)
	}
	if input.Name != "Project" || input.DefaultModel != "model-a" || len(input.DefaultMCPToolIDs) != 0 {
		t.Fatalf("normalized project = %#v", input)
	}
	if !reflect.DeepEqual(input.DefaultSkillIDs, []uint{5, 4}) {
		t.Fatalf("default Skill IDs = %v, want [5 4]", input.DefaultSkillIDs)
	}
}

func TestNormalizeConversationProjectDefaultModel(t *testing.T) {
	empty := "  "
	patch, err := normalizeConversationProjectPatch(ConversationProjectPatchInput{DefaultModel: &empty})
	if err != nil {
		t.Fatalf("normalizeConversationProjectPatch() error = %v", err)
	}
	if patch.DefaultModel == nil || *patch.DefaultModel != "" {
		t.Fatalf("normalized default model = %#v, want explicit empty value", patch.DefaultModel)
	}

	tooLong := strings.Repeat("m", conversationProjectModelMaxChars+1)
	if _, err = normalizeConversationProjectInput(ConversationProjectInput{Name: "Project", DefaultModel: tooLong}); !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected oversized default model to be rejected, got %v", err)
	}
	if _, err = normalizeConversationProjectPatch(ConversationProjectPatchInput{DefaultModel: &tooLong}); !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected oversized default model patch to be rejected, got %v", err)
	}
}

func TestNewProjectDefaultIDs(t *testing.T) {
	got := newProjectDefaultIDs([]uint{4, 2, 3}, []uint{2, 4})
	if !reflect.DeepEqual(got, []uint{3}) {
		t.Fatalf("newProjectDefaultIDs() = %v, want [3]", got)
	}
}

func TestValidateConversationProjectDefaultsPreservesUnavailableExistingSelections(t *testing.T) {
	service := &Service{cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 1})}
	current := &domainconversation.ConversationProject{
		MCPDefaultMode:          domainconversation.ConversationProjectMCPDefaultModeCustom,
		DefaultMCPToolIDs:       []uint{3, 2},
		DefaultSkillIDs:         []uint{5, 4},
		DefaultKnowledgeBaseIDs: []string{"legacy-unavailable"},
	}
	err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:           1,
		DefaultModel:     current.DefaultModel,
		MCPDefaultMode:   current.MCPDefaultMode,
		MCPToolIDs:       current.DefaultMCPToolIDs,
		SkillIDs:         current.DefaultSkillIDs,
		KnowledgeBaseIDs: current.DefaultKnowledgeBaseIDs,
		Current:          current,
	})
	if err != nil {
		t.Fatalf("validateConversationProjectDefaults() error = %v", err)
	}
}

func TestValidateConversationProjectDefaultsRejectsUnavailableKnowledgeBase(t *testing.T) {
	service := &Service{
		cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 1}),
		knowledgeBaseResolver: knowledgeBaseResolverStub{resolveFiles: func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
			return nil, nil, domainknowledgebase.ErrReferenceUnavailable
		}},
	}
	err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:           1,
		MCPDefaultMode:   domainconversation.ConversationProjectMCPDefaultModeInherit,
		KnowledgeBaseIDs: []string{"missing"},
	})
	if !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected unavailable knowledge base to be rejected, got %v", err)
	}
}

func TestValidateConversationProjectDefaultsRejectsKnowledgeBaseWithoutReadyFiles(t *testing.T) {
	service := &Service{
		cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 1}),
		knowledgeBaseResolver: knowledgeBaseResolverStub{resolveFiles: func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
			return []domainknowledgebase.KnowledgeBase{{PublicID: "empty", ReadyFileCount: 0}}, nil, nil
		}},
	}
	err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:           1,
		MCPDefaultMode:   domainconversation.ConversationProjectMCPDefaultModeInherit,
		KnowledgeBaseIDs: []string{"empty"},
	})
	if !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected knowledge base without ready files to be rejected, got %v", err)
	}
}

func TestValidateConversationProjectDefaultsRejectsMultipleImageProcessors(t *testing.T) {
	service := &Service{
		cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 4}),
		mcpRepo: selectedToolRuntimeMCPRepositoryStub{
			listToolsByIDs: func(context.Context, []uint) ([]domainmcp.Tool, error) {
				return []domainmcp.Tool{
					{ID: 1, AttachmentInputMode: domainmcp.AttachmentInputModeImage},
					{ID: 2, AttachmentInputMode: domainmcp.AttachmentInputModeImage},
				}, nil
			},
		},
	}
	err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:         1,
		MCPDefaultMode: domainconversation.ConversationProjectMCPDefaultModeCustom,
		MCPToolIDs:     []uint{1, 2},
	})
	if !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected multiple image processors to be rejected, got %v", err)
	}
}

func TestValidateConversationProjectDefaultModelAvailability(t *testing.T) {
	resolver := &projectDefaultModelResolverStub{models: []channel.ModelView{
		{PlatformModelName: "chat-model", KindsJSON: `["chat"]`},
		{PlatformModelName: "image-model", KindsJSON: `["image_gen","image_edit"]`},
	}}
	service := &Service{
		cfg:           config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 1}),
		routeResolver: resolver,
	}

	if err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:         1,
		DefaultModel:   "chat-model",
		MCPDefaultMode: domainconversation.ConversationProjectMCPDefaultModeInherit,
	}); err != nil {
		t.Fatalf("expected visible chat model to be accepted, got %v", err)
	}
	if err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:         1,
		DefaultModel:   "image-model",
		MCPDefaultMode: domainconversation.ConversationProjectMCPDefaultModeInherit,
	}); !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected non-chat model to be rejected, got %v", err)
	}

	current := &domainconversation.ConversationProject{DefaultModel: "retired-model"}
	if err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:         1,
		DefaultModel:   current.DefaultModel,
		MCPDefaultMode: domainconversation.ConversationProjectMCPDefaultModeInherit,
		Current:        current,
	}); err != nil {
		t.Fatalf("expected unchanged retired model to be preserved, got %v", err)
	}
	if err := service.validateConversationProjectDefaults(context.Background(), conversationProjectDefaultsValidationInput{
		UserID:         1,
		DefaultModel:   "other-retired-model",
		MCPDefaultMode: domainconversation.ConversationProjectMCPDefaultModeInherit,
		Current:        current,
	}); !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected newly selected unavailable model to be rejected, got %v", err)
	}
}

func TestCreateConversationUsesOnlyAvailableProjectDefaultModel(t *testing.T) {
	tests := []struct {
		name           string
		explicitModel  string
		projectDefault string
		wantModel      string
	}{
		{name: "project default", projectDefault: "project-model", wantModel: "project-model"},
		{name: "explicit selection wins", explicitModel: "manual-model", projectDefault: "project-model", wantModel: "manual-model"},
		{name: "retired project default falls back", projectDefault: "retired-model", wantModel: ""},
		{name: "non-chat project default falls back", projectDefault: "image-model", wantModel: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &projectDefaultConversationRepoStub{project: domainconversation.ConversationProject{
				ID:           9,
				UserID:       1,
				PublicID:     "project-one",
				Name:         "Project one",
				DefaultModel: test.projectDefault,
			}}
			service := &Service{
				cfg:  config.NewRuntime(config.Config{}),
				repo: repo,
				routeResolver: &projectDefaultModelResolverStub{models: []channel.ModelView{
					{PlatformModelName: "project-model", KindsJSON: `["chat"]`},
					{PlatformModelName: "image-model", KindsJSON: `["image_gen"]`},
				}},
			}

			created, err := service.CreateConversation(context.Background(), 1, "New chat", test.explicitModel, repo.project.PublicID)
			if err != nil {
				t.Fatalf("CreateConversation() error = %v", err)
			}
			if created.Model != test.wantModel || repo.created == nil || repo.created.Model != test.wantModel {
				t.Fatalf("created model = %q, persisted model = %v, want %q", created.Model, repo.created, test.wantModel)
			}
		})
	}
}

type projectDefaultModelResolverStub struct {
	models []channel.ModelView
}

func (s *projectDefaultModelResolverStub) ResolveRoute(context.Context, channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	return nil, errors.New("route resolution is not expected")
}

func (s *projectDefaultModelResolverStub) MarkRouteFailure(context.Context, *channel.ResolvedRoute, error) {
}

func (s *projectDefaultModelResolverStub) MarkRouteSuccess(context.Context, *channel.ResolvedRoute) {}

func (s *projectDefaultModelResolverStub) ListActiveModels(context.Context, uint) ([]channel.ModelView, error) {
	return append([]channel.ModelView(nil), s.models...), nil
}

type projectDefaultConversationRepoStub struct {
	repository.ConversationRepository
	project domainconversation.ConversationProject
	created *domainconversation.Conversation
}

func (s *projectDefaultConversationRepoStub) GetConversationProjectByPublicID(_ context.Context, userID uint, publicID string) (*domainconversation.ConversationProject, error) {
	if s.project.UserID != userID || s.project.PublicID != publicID {
		return nil, repository.ErrNotFound
	}
	project := s.project
	return &project, nil
}

func (s *projectDefaultConversationRepoStub) CreateConversation(_ context.Context, item *domainconversation.Conversation) error {
	created := *item
	s.created = &created
	return nil
}

type knowledgeBaseResolverStub struct {
	resolveFiles func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error)
}

func (s knowledgeBaseResolverStub) ResolveFiles(ctx context.Context, userID uint, publicIDs []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	return s.resolveFiles(ctx, userID, publicIDs)
}
