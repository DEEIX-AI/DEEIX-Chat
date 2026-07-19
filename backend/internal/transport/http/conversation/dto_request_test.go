package conversation

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRequiredConversationBooleanFieldsAcceptExplicitFalse(t *testing.T) {
	falseValue := false

	for name, request := range map[string]any{
		"star":    SetConversationStarRequest{Starred: &falseValue},
		"archive": SetConversationArchiveRequest{Archived: &falseValue},
	} {
		t.Run(name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(request); err != nil {
				t.Fatalf("expected explicit false to pass validation: %v", err)
			}
		})
	}
}

func TestRequiredConversationBooleanFieldsRejectMissingValues(t *testing.T) {
	for name, request := range map[string]any{
		"star":    SetConversationStarRequest{},
		"archive": SetConversationArchiveRequest{},
	} {
		t.Run(name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(request); err == nil {
				t.Fatal("expected missing required boolean to fail validation")
			}
		})
	}
}
