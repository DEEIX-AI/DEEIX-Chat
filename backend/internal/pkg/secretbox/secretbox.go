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
	return Encrypt(secret, []byte(value))
}

// DecryptString 解密 AES-GCM 字符串。
func DecryptString(secret string, encrypted string) (string, error) {
	raw, err := Decrypt(secret, encrypted)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Encrypt 使用 AES-GCM 加密任意字节，返回带 v1: 前缀的 base64 载荷。
func Encrypt(secret string, plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
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
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, ciphertext...)
	return prefix + base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt 解密 Encrypt 产生的载荷。
func Decrypt(secret string, encrypted string) ([]byte, error) {
	value := strings.TrimSpace(encrypted)
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(value, prefix) {
		return nil, errors.New("invalid encrypted payload")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key(secret))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("invalid encrypted payload")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func key(secret string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return sum[:]
}
