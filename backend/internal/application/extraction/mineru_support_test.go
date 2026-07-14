package extraction

import (
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	mineruextract "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/mineru"
)

func TestSupportsMinerUFileUsesSourceSpecificOfficeFormats(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		selected string
		file     domainconversation.FileObject
		want     bool
	}{
		{
			name:     "cloud supports legacy doc",
			source:   mineruextract.SourceCloud,
			selected: "word",
			file:     domainconversation.FileObject{FileCategory: "word", FileName: "legacy.doc"},
			want:     true,
		},
		{
			name:     "self hosted rejects legacy doc",
			source:   mineruextract.SourceSelfHosted,
			selected: "word",
			file:     domainconversation.FileObject{FileCategory: "word", FileName: "legacy.doc"},
			want:     false,
		},
		{
			name:     "self hosted supports docx",
			source:   mineruextract.SourceSelfHosted,
			selected: "word",
			file:     domainconversation.FileObject{FileCategory: "word", FileName: "report.docx"},
			want:     true,
		},
		{
			name:     "cloud supports legacy ppt",
			source:   mineruextract.SourceCloud,
			selected: "presentation",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.ppt"},
			want:     true,
		},
		{
			name:     "self hosted rejects legacy ppt",
			source:   mineruextract.SourceSelfHosted,
			selected: "presentation",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.ppt"},
			want:     false,
		},
		{
			name:     "self hosted supports pptx",
			source:   mineruextract.SourceSelfHosted,
			selected: "presentation",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.pptx"},
			want:     true,
		},
		{
			name:     "self hosted supports pptx detected mime without extension",
			source:   mineruextract.SourceSelfHosted,
			selected: "presentation",
			file: domainconversation.FileObject{
				FileCategory: "presentation",
				FileName:     "deck",
				DetectedMIME: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			},
			want: true,
		},
		{
			name:     "excel is disabled unless selected",
			source:   mineruextract.SourceCloud,
			selected: defaultMinerUFileTypes,
			file:     domainconversation.FileObject{FileCategory: "excel", FileName: "data.xlsx"},
			want:     false,
		},
		{
			name:     "cloud supports legacy xls when excel selected",
			source:   mineruextract.SourceCloud,
			selected: "excel",
			file:     domainconversation.FileObject{FileCategory: "excel", FileName: "data.xls"},
			want:     true,
		},
		{
			name:     "self hosted rejects legacy xls",
			source:   mineruextract.SourceSelfHosted,
			selected: "excel",
			file:     domainconversation.FileObject{FileCategory: "excel", FileName: "data.xls"},
			want:     false,
		},
		{
			name:     "empty selection uses defaults",
			source:   mineruextract.SourceCloud,
			selected: "",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.pptx"},
			want:     true,
		},
		{
			name:     "unselected presentation is not supported",
			source:   mineruextract.SourceCloud,
			selected: "pdf,word",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.pptx"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsMinerUFile(tt.file, tt.source, tt.selected); got != tt.want {
				t.Fatalf("supportsMinerUFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
