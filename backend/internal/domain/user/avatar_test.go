package user

import "testing"

func TestFileAvatarURL(t *testing.T) {
	fileID := "file_test_123"
	ref := BuildFileAvatarURL(fileID)
	if ref != "file:file_test_123" {
		t.Fatalf("BuildFileAvatarURL() = %q", ref)
	}

	parsed, ok := ParseFileAvatarURL(ref)
	if !ok || parsed != fileID {
		t.Fatalf("ParseFileAvatarURL() = %q, %v", parsed, ok)
	}
}

func TestParseFileAvatarURLRejectsInvalidReference(t *testing.T) {
	tests := []string{
		"file:",
		"file:avatar",
		"file:file_../x",
		"file: file_test",
		"http://example.com/avatar.png",
	}
	for _, item := range tests {
		if fileID, ok := ParseFileAvatarURL(item); ok {
			t.Fatalf("ParseFileAvatarURL(%q) = %q, true; want false", item, fileID)
		}
	}
}

func TestIsValidAvatarURL(t *testing.T) {
	for _, value := range []string{"", "/avatars/default.png", "generated:github:user", "file:file_test_123", "https://example.com/avatar.png"} {
		if !IsValidAvatarURL(value) {
			t.Fatalf("IsValidAvatarURL(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"file:invalid", "javascript:alert(1)", "https:///avatar.png"} {
		if IsValidAvatarURL(value) {
			t.Fatalf("IsValidAvatarURL(%q) = true, want false", value)
		}
	}
}
