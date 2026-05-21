package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const prefix = "v1:"

// EncryptString 使用 AES-GCM 加密字符串。
func EncryptString(secret string, plaintext string) (string, error) {
	value := strings.TrimSpace(plaintext)
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)
	return prefix + base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptString 解密 AES-GCM 字符串。
func DecryptString(secret string, encrypted string) (string, error) {
	value := strings.TrimSpace(encrypted)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefix) {
		return "", errors.New("invalid encrypted payload")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted payload")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func key(secret string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return sum[:]
}
