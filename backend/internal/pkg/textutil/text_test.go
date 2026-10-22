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

func TestSanitizeForTextColumn(t *testing.T) {
	clean := "普通文本 123"
	if got := SanitizeForTextColumn(clean); got != clean {
		t.Fatalf("clean text changed: %q", got)
	}
	// 抓取到二进制内容时解码出的 NUL 与非法 UTF-8 都不能进入 text 列；连续的非法字节合并为一个替换符。
	if got := SanitizeForTextColumn("GIF89a\x00\xff\xfe tail"); got != "GIF89a\uFFFD tail" {
		t.Fatalf("unexpected sanitized value: %q", got)
	}
}
