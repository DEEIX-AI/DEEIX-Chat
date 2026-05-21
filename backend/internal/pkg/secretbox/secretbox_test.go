package secretbox

import "testing"

func TestEncryptStringRoundTrip(t *testing.T) {
	plaintext := "sk-test-secret"
	encrypted, err := EncryptString("test-data-encryption-key", plaintext)
	if err != nil {
		t.Fatalf("EncryptString returned error: %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("EncryptString returned plaintext")
	}
	decrypted, err := DecryptString("test-data-encryption-key", encrypted)
	if err != nil {
		t.Fatalf("DecryptString returned error: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecryptStringRejectsPlaintext(t *testing.T) {
	if _, err := DecryptString("test-data-encryption-key", `{"keys":[]}`); err == nil {
		t.Fatal("DecryptString accepted plaintext")
	}
}
