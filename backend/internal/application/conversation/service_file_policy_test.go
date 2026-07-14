package conversation

import "testing"

func TestConversationFilePolicyRecognizesPresentations(t *testing.T) {
	tests := []struct {
		name     string
		detected string
		fileName string
		wantMIME string
	}{
		{
			name:     "legacy ppt",
			detected: "application/octet-stream",
			fileName: "slides.ppt",
			wantMIME: "application/vnd.ms-powerpoint",
		},
		{
			name:     "openxml pptx",
			detected: "application/zip",
			fileName: "slides.pptx",
			wantMIME: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMIME := normalizeDetectedMIME(tt.detected, tt.fileName)
			if gotMIME != tt.wantMIME {
				t.Fatalf("normalizeDetectedMIME() = %q, want %q", gotMIME, tt.wantMIME)
			}
			if got := inferFileCategory(gotMIME, tt.fileName); got != fileCategoryPresentation {
				t.Fatalf("inferFileCategory() = %q, want %q", got, fileCategoryPresentation)
			}
			if !supportsExtraction(fileCategoryPresentation) {
				t.Fatal("presentation should support extraction")
			}
			if !supportsRAG(fileCategoryPresentation) {
				t.Fatal("presentation should support RAG")
			}
		})
	}
}
