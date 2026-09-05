package contentmoderation

import (
	"encoding/json"
	"reflect"
	"testing"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

func TestStorageJSONPreservesExistingFieldNames(t *testing.T) {
	location := domaincm.ContentLocation{
		Field:      "assistant_images",
		FileID:     "file-1",
		Attachment: 2,
		ChunkIndex: 3,
		ChunkCount: 4,
	}
	if got, want := marshalContentLocation(location), `{"field":"assistant_images","fileID":"file-1","attachment":2,"chunkIndex":3,"chunkCount":4}`; got != want {
		t.Fatalf("location JSON = %s, want %s", got, want)
	}

	images := []domaincm.IsolatedImageMeta{{
		Index:        1,
		SHA256:       "abc",
		MimeType:     "image/png",
		SizeBytes:    42,
		StoragePath:  "moderation-isolated/event/1.bin",
		SourceFileID: "file-1",
	}}
	raw := marshalIsolatedImageMetadata(images)
	if got := unmarshalIsolatedImageMetadata(raw); !reflect.DeepEqual(got, images) {
		t.Fatalf("image metadata round trip = %#v, want %#v", got, images)
	}
	if raw == "" || raw[0] != '[' {
		t.Fatalf("unexpected image metadata JSON: %q", raw)
	}
}

func TestPolicyJSONPreservesExistingFieldNames(t *testing.T) {
	policy := Policy{
		InputTextCategories:   []string{"hate"},
		OutputTextCategories:  []string{},
		InputImageCategories:  []string{"violence"},
		OutputImageCategories: []string{},
		Version:               7,
	}
	raw, err := json.Marshal(newPolicyJSON(policy))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"inputTextCategories", "outputTextCategories", "inputImageCategories", "outputImageCategories", "version"} {
		if _, ok := document[key]; !ok {
			t.Fatalf("policy JSON missing %q: %s", key, raw)
		}
	}
}
