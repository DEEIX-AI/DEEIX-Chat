package usersettings

import "testing"

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
