package textutil

import "testing"

func TestFirstNonEmptyNormalizesSelectedValue(t *testing.T) {
	if got := FirstNonEmpty("  ", " value ", "fallback"); got != "value" {
		t.Fatalf("FirstNonEmpty() = %q, want %q", got, "value")
	}
}

func TestFirstNonBlankPreservesSelectedValue(t *testing.T) {
	if got := FirstNonBlank("  ", "\n value \n", "fallback"); got != "\n value \n" {
		t.Fatalf("FirstNonBlank() = %q, want original text", got)
	}
}

func TestIsASCIIAlpha(t *testing.T) {
	if !IsASCIIAlpha("AbCd") {
		t.Fatal("IsASCIIAlpha rejected ASCII letters")
	}
	for _, value := range []string{"zh-CN", "中文", "a1"} {
		if IsASCIIAlpha(value) {
			t.Fatalf("IsASCIIAlpha(%q) = true, want false", value)
		}
	}
}

func TestCompactSnippet(t *testing.T) {
	if got := CompactSnippet("  first\n\nsecond  ", 8); got != "first se..." {
		t.Fatalf("CompactSnippet() = %q, want %q", got, "first se...")
	}
}

func TestTruncateTrimmed(t *testing.T) {
	if got := TruncateTrimmed("  你好世界  ", 3); got != "你好世" {
		t.Fatalf("TruncateTrimmed() = %q, want %q", got, "你好世")
	}
}
