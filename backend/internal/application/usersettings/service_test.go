package usersettings

import (
	"errors"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

func TestSettingValidationErrorKeepsStablePublicContract(t *testing.T) {
	t.Parallel()

	err := settingValidationError(ErrInvalidSettingValue, "invalid value for chat.content_width")
	if !errors.Is(err, ErrInvalidSettingValue) {
		t.Fatal("validation error must preserve sentinel identity")
	}
	if code := apperr.Code(err); code != ErrInvalidSettingValue.Code() {
		t.Fatalf("code = %q, want %q", code, ErrInvalidSettingValue.Code())
	}
	if message := apperr.MessageOr(err, ""); message != ErrInvalidSettingValue.Message() {
		t.Fatalf("message = %q, want %q", message, ErrInvalidSettingValue.Message())
	}
}

func TestValidateDefaultMCPToolIDs(t *testing.T) {
	t.Parallel()

	validValues := []string{
		"[]",
		"[1]",
		"[1,2,3]",
		" [42] ",
	}
	for _, value := range validValues {
		if err := validateDefaultMCPToolIDs(value, "chat.default_mcp_tool_ids"); err != nil {
			t.Fatalf("expected %s to be valid, got %v", value, err)
		}
	}

	invalidValues := []string{
		"",
		"{}",
		"[0]",
		"[-1]",
		"[1.5]",
		`["1"]`,
	}
	for _, value := range invalidValues {
		if err := validateDefaultMCPToolIDs(value, "chat.default_mcp_tool_ids"); err == nil {
			t.Fatalf("expected %s to be invalid", value)
		}
	}
}

func TestDefaultMCPToolIDsSettingIsAllowed(t *testing.T) {
	t.Parallel()

	if got := allowedKeys["chat.default_mcp_tool_ids"]; got != "[]" {
		t.Fatalf("expected chat.default_mcp_tool_ids default to be [], got %q", got)
	}
	if err := validateValue("chat.default_mcp_tool_ids", "[1,2,3]"); err != nil {
		t.Fatalf("expected chat.default_mcp_tool_ids to be accepted, got %v", err)
	}
}

func TestContentWidthSettingIsAllowed(t *testing.T) {
	t.Parallel()

	if got := allowedKeys["chat.content_width"]; got != "compact" {
		t.Fatalf("expected chat.content_width default to be compact, got %q", got)
	}
	for _, value := range []string{"compact", "standard", "wide"} {
		if err := validateValue("chat.content_width", value); err != nil {
			t.Fatalf("expected chat.content_width=%s to be accepted, got %v", value, err)
		}
	}
	if err := validateValue("chat.content_width", "loose"); err == nil {
		t.Fatal("expected invalid chat.content_width to be rejected")
	}
}

func TestReuseModelOptionsSettingIsAllowed(t *testing.T) {
	t.Parallel()

	if got := allowedKeys["chat.reuse_model_options"]; got != "true" {
		t.Fatalf("expected chat.reuse_model_options default to be true, got %q", got)
	}
	for _, value := range []string{"true", "false"} {
		if err := validateValue("chat.reuse_model_options", value); err != nil {
			t.Fatalf("expected chat.reuse_model_options=%s to be accepted, got %v", value, err)
		}
	}
	if err := validateValue("chat.reuse_model_options", "yes"); err == nil {
		t.Fatal("expected invalid chat.reuse_model_options to be rejected")
	}
}

func TestReasoningContentPassbackSettingIsAllowed(t *testing.T) {
	t.Parallel()

	if got := allowedKeys["chat.reasoning_content_passback"]; got != "true" {
		t.Fatalf("expected chat.reasoning_content_passback default to be true, got %q", got)
	}
	for _, value := range []string{"true", "false"} {
		if err := validateValue("chat.reasoning_content_passback", value); err != nil {
			t.Fatalf("expected chat.reasoning_content_passback=%s to be accepted, got %v", value, err)
		}
	}
	if err := validateValue("chat.reasoning_content_passback", "yes"); err == nil {
		t.Fatal("expected invalid chat.reasoning_content_passback to be rejected")
	}
}

func TestAutoGenerateLabelsSettingIsAllowed(t *testing.T) {
	t.Parallel()

	if got := allowedKeys["chat.auto_generate_labels"]; got != "true" {
		t.Fatalf("expected chat.auto_generate_labels default to be true, got %q", got)
	}
	for _, value := range []string{"true", "false"} {
		if err := validateValue("chat.auto_generate_labels", value); err != nil {
			t.Fatalf("expected chat.auto_generate_labels=%s to be accepted, got %v", value, err)
		}
	}
	if err := validateValue("chat.auto_generate_labels", "yes"); err == nil {
		t.Fatal("expected invalid chat.auto_generate_labels to be rejected")
	}
}

func TestTraceAutoExpandSettingsAreAllowed(t *testing.T) {
	t.Parallel()

	keys := []string{
		"chat.auto_expand_thinking",
		"chat.auto_expand_tool_calls",
	}
	for _, key := range keys {
		if got := allowedKeys[key]; got != "true" {
			t.Fatalf("expected %s default to be true, got %q", key, got)
		}
		for _, value := range []string{"true", "false"} {
			if err := validateValue(key, value); err != nil {
				t.Fatalf("expected %s=%s to be accepted, got %v", key, value, err)
			}
		}
		if err := validateValue(key, "yes"); err == nil {
			t.Fatalf("expected invalid %s to be rejected", key)
		}
	}
}
