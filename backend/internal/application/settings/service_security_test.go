package settings

import "testing"

func TestEmbeddingHostAllowsAdministratorConfiguredPrivateOrigin(t *testing.T) {
	if err := validatePatchItem(PatchItem{Namespace: "file", Key: "embedding_host", Value: "http://embedding:8080/v1"}); err != nil {
		t.Fatalf("private Embedding endpoint rejected: %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "file", Key: "embedding_host", Value: "http://169.254.169.254/latest/meta-data"}); err == nil {
		t.Fatal("metadata endpoint must remain blocked")
	}
}
