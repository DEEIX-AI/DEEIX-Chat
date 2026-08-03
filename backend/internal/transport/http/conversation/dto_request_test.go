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

func TestConversationLabelsRequestAllowsExplicitEmptyList(t *testing.T) {
	labels := []string{}
	request := UpdateConversationLabelsRequest{Labels: &labels}
	if err := binding.Validator.ValidateStruct(request); err != nil {
		t.Fatalf("expected explicit empty labels to pass validation: %v", err)
	}
}

func TestConversationLabelsRequestRejectsMissingOrOversizedValues(t *testing.T) {
	tooMany := []string{"1", "2", "3", "4", "5", "6", "7"}
	tooLong := []string{"1234567890123456789012345"}
	for name, request := range map[string]UpdateConversationLabelsRequest{
		"missing":  {},
		"too_many": {Labels: &tooMany},
		"too_long": {Labels: &tooLong},
	} {
		t.Run(name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(request); err == nil {
				t.Fatal("expected invalid labels request to fail validation")
			}
		})
	}
}
