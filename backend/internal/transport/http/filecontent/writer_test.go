package filecontent

import "testing"

func TestSafeContentTypeDowngradesActiveContent(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{contentType: "text/html; charset=utf-8", want: "text/plain; charset=utf-8"},
		{contentType: "application/javascript", want: "text/plain; charset=utf-8"},
		{contentType: "image/svg+xml", want: "text/plain; charset=utf-8"},
		{contentType: "application/pdf", want: "application/pdf"},
	}
	for _, tt := range tests {
		if got := safeContentType(tt.contentType); got != tt.want {
			t.Fatalf("safeContentType(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}

func TestBuildContentDispositionDefaultsToAttachment(t *testing.T) {
	got := buildContentDisposition("report.html", false)
	want := `attachment; filename="report.html"; filename*=UTF-8''report.html`
	if got != want {
		t.Fatalf("unexpected disposition: got %q want %q", got, want)
	}
}
