package contentmoderation

import (
	"encoding/json"
	"strings"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

// These private documents preserve the stored JSON schema without coupling domain types
// to a transport or persistence protocol.
type contentLocationJSON struct {
	Field      string `json:"field,omitempty"`
	FileID     string `json:"fileID,omitempty"`
	Attachment int    `json:"attachment,omitempty"`
	ChunkIndex int    `json:"chunkIndex,omitempty"`
	ChunkCount int    `json:"chunkCount,omitempty"`
}

type isolatedImageMetaJSON struct {
	Index        int    `json:"index"`
	SHA256       string `json:"sha256"`
	MimeType     string `json:"mimeType"`
	SizeBytes    int64  `json:"sizeBytes"`
	StoragePath  string `json:"storagePath"`
	SourceFileID string `json:"sourceFileID,omitempty"`
}

func marshalContentLocation(location domaincm.ContentLocation) string {
	return mustJSON(contentLocationJSON{
		Field:      location.Field,
		FileID:     location.FileID,
		Attachment: location.Attachment,
		ChunkIndex: location.ChunkIndex,
		ChunkCount: location.ChunkCount,
	})
}

func marshalIsolatedImageMetadata(images []domaincm.IsolatedImageMeta) string {
	documents := make([]isolatedImageMetaJSON, 0, len(images))
	for _, image := range images {
		documents = append(documents, isolatedImageMetaJSON{
			Index:        image.Index,
			SHA256:       image.SHA256,
			MimeType:     image.MimeType,
			SizeBytes:    image.SizeBytes,
			StoragePath:  image.StoragePath,
			SourceFileID: image.SourceFileID,
		})
	}
	return mustJSON(documents)
}

func unmarshalIsolatedImageMetadata(raw string) []domaincm.IsolatedImageMeta {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var documents []isolatedImageMetaJSON
	if err := json.Unmarshal([]byte(raw), &documents); err != nil {
		return nil
	}
	images := make([]domaincm.IsolatedImageMeta, 0, len(documents))
	for _, document := range documents {
		images = append(images, domaincm.IsolatedImageMeta{
			Index:        document.Index,
			SHA256:       document.SHA256,
			MimeType:     document.MimeType,
			SizeBytes:    document.SizeBytes,
			StoragePath:  document.StoragePath,
			SourceFileID: document.SourceFileID,
		})
	}
	return images
}
