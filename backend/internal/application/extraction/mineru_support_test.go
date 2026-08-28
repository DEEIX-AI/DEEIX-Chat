package extraction

import (
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	mineruextract "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
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
			source:   mineruextract.MinerUSourceCloud,
			selected: "word",
			file:     domainconversation.FileObject{FileCategory: "word", FileName: "legacy.doc"},
			want:     true,
		},
		{
			name:     "self hosted rejects legacy doc",
			source:   mineruextract.MinerUSourceSelfHosted,
			selected: "word",
			file:     domainconversation.FileObject{FileCategory: "word", FileName: "legacy.doc"},
			want:     false,
		},
		{
			name:     "self hosted supports docx",
			source:   mineruextract.MinerUSourceSelfHosted,
			selected: "word",
			file:     domainconversation.FileObject{FileCategory: "word", FileName: "report.docx"},
			want:     true,
		},
		{
			name:     "cloud supports legacy ppt",
			source:   mineruextract.MinerUSourceCloud,
			selected: "presentation",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.ppt"},
			want:     true,
		},
		{
			name:     "self hosted rejects legacy ppt",
			source:   mineruextract.MinerUSourceSelfHosted,
			selected: "presentation",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.ppt"},
			want:     false,
		},
		{
			name:     "self hosted supports pptx",
			source:   mineruextract.MinerUSourceSelfHosted,
			selected: "presentation",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.pptx"},
			want:     true,
		},
		{
			name:     "self hosted supports pptx detected mime without extension",
			source:   mineruextract.MinerUSourceSelfHosted,
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
			source:   mineruextract.MinerUSourceCloud,
			selected: defaultMinerUFileTypes,
			file:     domainconversation.FileObject{FileCategory: "excel", FileName: "data.xlsx"},
			want:     false,
		},
		{
			name:     "cloud supports legacy xls when excel selected",
			source:   mineruextract.MinerUSourceCloud,
			selected: "excel",
			file:     domainconversation.FileObject{FileCategory: "excel", FileName: "data.xls"},
			want:     true,
		},
		{
			name:     "self hosted rejects legacy xls",
			source:   mineruextract.MinerUSourceSelfHosted,
			selected: "excel",
			file:     domainconversation.FileObject{FileCategory: "excel", FileName: "data.xls"},
			want:     false,
		},
		{
			name:     "empty selection uses defaults",
			source:   mineruextract.MinerUSourceCloud,
			selected: "",
			file:     domainconversation.FileObject{FileCategory: "presentation", FileName: "deck.pptx"},
			want:     true,
		},
		{
			name:     "unselected presentation is not supported",
			source:   mineruextract.MinerUSourceCloud,
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
