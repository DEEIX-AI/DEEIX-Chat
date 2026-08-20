package embeddingutil

import "testing"

func TestModelSignatureNormalizesModelWhitespace(t *testing.T) {
	left := ModelSignature(" model-name ", 4096)
	right := ModelSignature("model-name", 4096)
	if left != right || left == ModelSignature("model-name", 1536) {
		t.Fatalf("unexpected model signatures: left=%q right=%q", left, right)
	}
}

func TestSpaceSignatureIncludesNormalizedEndpoint(t *testing.T) {
	left := SpaceSignature(" model-name ", 1536, " https://embedding.example/v1/ ")
	right := SpaceSignature("model-name", 1536, "https://embedding.example/v1")
	otherEndpoint := SpaceSignature("model-name", 1536, "https://other.example/v1")
	if left != right {
		t.Fatalf("trailing slash changed signature: left=%q right=%q", left, right)
	}
	if left == otherEndpoint {
		t.Fatalf("different endpoints must not share a vector-space signature: %q", left)
	}
}

func TestSpaceSignatureSupportsBidirectionalDimensionChanges(t *testing.T) {
	initial := SpaceSignature("model-name", 4096, "https://embedding.example/v1")
	reduced := SpaceSignature("model-name", 1536, "https://embedding.example/v1")
	restored := SpaceSignature("model-name", 4096, "https://embedding.example/v1")
	if initial == reduced {
		t.Fatal("different dimensions must not share a vector-space signature")
	}
	if initial != restored {
		t.Fatal("restoring the same vector space must reproduce its signature")
	}
}
