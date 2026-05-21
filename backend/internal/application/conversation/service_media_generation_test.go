package conversation

import (
	"bytes"
	"testing"
)

func TestDetectGeneratedImageMIMERejectsNonImageBytes(t *testing.T) {
	_, _, err := validateGeneratedImageBytes([]byte("<html>not an image</html>"), "image/png")
	if err == nil {
		t.Fatal("expected non-image generated output to be rejected")
	}
}

func TestDetectGeneratedImageMIMEUsesActualImageBytes(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}
	got, mimeType, err := validateGeneratedImageBytes(data, "image/png")
	if err != nil {
		t.Fatalf("expected jpeg bytes to pass validation: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("expected validation to return original bytes")
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("expected actual jpeg MIME, got %q", mimeType)
	}
}

func TestStripBase64DataURLPrefix(t *testing.T) {
	got := stripBase64DataURLPrefix("data:image/png;base64, aGVsbG8= ")
	if got != "aGVsbG8=" {
		t.Fatalf("unexpected stripped data URL: %q", got)
	}
}
