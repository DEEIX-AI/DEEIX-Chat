package conversation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func TestNormalizeConversationProjectInputInheritClearsMCPDefaults(t *testing.T) {
	input, err := normalizeConversationProjectInput(ConversationProjectInput{
		Name:              " Project ",
		MCPDefaultMode:    domainconversation.ConversationProjectMCPDefaultModeInherit,
		DefaultMCPToolIDs: []uint{3, 3, 2},
		DefaultSkillIDs:   []uint{5, 0, 5, 4},
	})
	if err != nil {
		t.Fatalf("normalizeConversationProjectInput() error = %v", err)
	}
	if input.Name != "Project" || len(input.DefaultMCPToolIDs) != 0 {
		t.Fatalf("normalized project = %#v", input)
	}
	if !reflect.DeepEqual(input.DefaultSkillIDs, []uint{5, 4}) {
		t.Fatalf("default Skill IDs = %v, want [5 4]", input.DefaultSkillIDs)
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
	err := service.validateConversationProjectDefaults(
		context.Background(),
		1,
		current.MCPDefaultMode,
		current.DefaultMCPToolIDs,
		current.DefaultSkillIDs,
		current.DefaultKnowledgeBaseIDs,
		current,
	)
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
	err := service.validateConversationProjectDefaults(
		context.Background(),
		1,
		domainconversation.ConversationProjectMCPDefaultModeInherit,
		nil,
		nil,
		[]string{"missing"},
		nil,
	)
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
	err := service.validateConversationProjectDefaults(
		context.Background(),
		1,
		domainconversation.ConversationProjectMCPDefaultModeInherit,
		nil,
		nil,
		[]string{"empty"},
		nil,
	)
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
	err := service.validateConversationProjectDefaults(
		context.Background(),
		1,
		domainconversation.ConversationProjectMCPDefaultModeCustom,
		[]uint{1, 2},
		nil,
		nil,
		nil,
	)
	if !errors.Is(err, ErrInvalidConversationProject) {
		t.Fatalf("expected multiple image processors to be rejected, got %v", err)
	}
}

type knowledgeBaseResolverStub struct {
	resolveFiles func(context.Context, uint, []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error)
}

func (s knowledgeBaseResolverStub) ResolveFiles(ctx context.Context, userID uint, publicIDs []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error) {
	return s.resolveFiles(ctx, userID, publicIDs)
}
