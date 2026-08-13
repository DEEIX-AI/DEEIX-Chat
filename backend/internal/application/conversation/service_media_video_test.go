package conversation

import (
	"testing"
)

func TestValidateGeneratedVideoBytesRejectsUndetectedContent(t *testing.T) {
	if _, _, err := validateGeneratedVideoBytes([]byte("not a video"), "video/mp4"); err == nil {
		t.Fatal("expected declared video MIME to be insufficient without a supported video header")
	}
}
