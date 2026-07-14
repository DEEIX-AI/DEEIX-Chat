package conversation

import (
	"context"
	"reflect"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
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
		MCPDefaultMode:    domainconversation.ConversationProjectMCPDefaultModeCustom,
		DefaultMCPToolIDs: []uint{3, 2},
		DefaultSkillIDs:   []uint{5, 4},
	}
	err := service.validateConversationProjectDefaults(
		context.Background(),
		1,
		current.MCPDefaultMode,
		current.DefaultMCPToolIDs,
		current.DefaultSkillIDs,
		current,
	)
	if err != nil {
		t.Fatalf("validateConversationProjectDefaults() error = %v", err)
	}
}
