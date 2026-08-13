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

func TestEncryptBytesRoundTrip(t *testing.T) {
	plaintext := []byte{0x00, 0xff, 0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	encrypted, err := Encrypt("test-data-encryption-key", plaintext)
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	if encrypted == string(plaintext) {
		t.Fatal("Encrypt returned plaintext")
	}
	decrypted, err := Decrypt("test-data-encryption-key", encrypted)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if len(decrypted) != len(plaintext) {
		t.Fatalf("length mismatch: got %d want %d", len(decrypted), len(plaintext))
	}
	for i := range plaintext {
		if decrypted[i] != plaintext[i] {
			t.Fatalf("byte %d: got %x want %x", i, decrypted[i], plaintext[i])
		}
	}
}
