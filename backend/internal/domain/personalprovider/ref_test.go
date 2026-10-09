package personalprovider

import (
	"strings"
	"testing"
)

func TestModelRefRoundTrip(t *testing.T) {
	ref := FormatModelRef("ab12cd34ef56", "anthropic/claude-sonnet-4.5:beta")
	if ref != "personal:ab12cd34ef56/anthropic/claude-sonnet-4.5:beta" {
		t.Fatalf("ref = %q", ref)
	}
	if !IsModelRef(ref) {
		t.Fatal("expected a personal ref")
	}
	id, model, ok := ParseModelRef(ref)
	if !ok || id != "ab12cd34ef56" || model != "anthropic/claude-sonnet-4.5:beta" {
		t.Fatalf("parse = %q %q %v", id, model, ok)
	}
}

func TestParseModelRefRejectsMalformedRefs(t *testing.T) {
	for _, ref := range []string{
		"gpt-4o",
		"personal:",
		"personal:ab12cd34ef56",
		"personal:ab12cd34ef56/",
		"personal:AB12CD34EF56/gpt-4o",
		"personal:short/gpt-4o",
		"personal:ab12cd34ef56/gpt 4o",
		"personal:ab12cd34ef56/gpt\t4o",
		"personal:../../x/gpt-4o",
	} {
		if _, _, ok := ParseModelRef(ref); ok {
			t.Errorf("expected %q to be rejected", ref)
		}
	}
}

func TestKeyHintNeverRevealsShortKeys(t *testing.T) {
	if got := KeyHint("sk-short"); got != "••••" {
		t.Fatalf("short key hint = %q", got)
	}
	if got := KeyHint("sk-abcdefghijklmnop1234"); got != "sk-••••1234" {
		t.Fatalf("hint = %q", got)
	}
	if got := KeyHint("  "); got != "" {
		t.Fatalf("empty hint = %q", got)
	}
}

func TestIsValidIconAcceptsOnlyBundledSlugs(t *testing.T) {
	for _, icon := range []string{"", "openai", "google-color", "zhipu2"} {
		if !IsValidIcon(icon) {
			t.Fatalf("IsValidIcon(%q) = false", icon)
		}
	}
	for _, icon := range []string{"https://evil.example/pixel.png", "file:abc", "OpenAI", "-x", "a/b", "a b", strings.Repeat("a", MaxIconLength+1)} {
		if IsValidIcon(icon) {
			t.Fatalf("IsValidIcon(%q) = true", icon)
		}
	}
}
