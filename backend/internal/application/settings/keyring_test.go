package settings

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"

const testDataEncryptionKey = "test-data-encryption-key"

func testKeyring() *secretbox.Keyring {
	keyring, err := secretbox.NewKeyring(testDataEncryptionKey)
	if err != nil {
		panic(err)
	}
	return keyring
}
