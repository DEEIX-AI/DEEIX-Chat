// Package secretbox 加密需要可还原的敏感数据（上游 API Key、TOTP 密钥、MCP 令牌等）。
//
// 服务器要用明文调用外部服务，所以这里是可解密的加密而不是哈希：能同时拿到数据库与主密钥的人
// 可以解密。载荷格式为 v1:<base64(nonce || AES-256-GCM 密文)>，密钥为主密钥的 SHA-256。
//
// Keyring 同时持有当前主密钥与轮换前的旧主密钥：加密只用当前密钥，解密依次尝试，
// 因此更换主密钥后存量数据仍可读取。
//
// *Bound 系列把密文绑定到调用方给出的上下文（作为 AES-GCM 附加数据，不写入载荷）：
// 只有用同一上下文才能解密，数据库里的密文被挪到别的记录上会直接解密失败。
package secretbox

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const prefix = "v1:"

var (
	// ErrNoKey 表示未配置主密钥。
	ErrNoKey = errors.New("secretbox: encryption key is not configured")
	// ErrInvalidPayload 表示载荷不是本包产生的格式。
	ErrInvalidPayload = errors.New("secretbox: invalid encrypted payload")
	// ErrDecrypt 表示所有已配置的主密钥都无法解密该载荷。
	ErrDecrypt = errors.New("secretbox: decryption failed")
)

// Keyring 持有当前主密钥与仍可用于解密的旧主密钥。创建后只读，并发安全。
type Keyring struct {
	keys []masterKey // keys[0] 为当前密钥，只有它用于加密
}

type masterKey struct {
	secret []byte
	aead   cipher.AEAD
}

// NewKeyring 以 active 为当前主密钥构建密钥环；previous 为轮换前的旧主密钥，只用于解密。
func NewKeyring(active string, previous ...string) (*Keyring, error) {
	if strings.TrimSpace(active) == "" {
		return nil, ErrNoKey
	}
	ring := &Keyring{}
	seen := map[string]bool{}
	for _, secret := range append([]string{active}, previous...) {
		secret = strings.TrimSpace(secret)
		if secret == "" || seen[secret] {
			continue
		}
		seen[secret] = true
		sum := sha256.Sum256([]byte(secret))
		block, err := aes.NewCipher(sum[:])
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		ring.keys = append(ring.keys, masterKey{secret: []byte(secret), aead: aead})
	}
	return ring, nil
}

// EncryptString 去除首尾空白后加密；空串返回空串。
func (k *Keyring) EncryptString(plaintext string) (string, error) {
	return k.Encrypt([]byte(strings.TrimSpace(plaintext)))
}

// DecryptString 解密 EncryptString 产生的载荷；空载荷返回空串。
func (k *Keyring) DecryptString(payload string) (string, error) {
	plaintext, err := k.Decrypt(payload)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptStringBound 与 EncryptString 相同，但密文只能用同一 binding 解密。
func (k *Keyring) EncryptStringBound(plaintext string, binding string) (string, error) {
	return k.seal([]byte(strings.TrimSpace(plaintext)), []byte(binding))
}

// DecryptStringBound 解密 EncryptStringBound 产生的载荷；binding 不一致时返回 ErrDecrypt。
func (k *Keyring) DecryptStringBound(payload string, binding string) (string, error) {
	plaintext, _, err := k.open(payload, []byte(binding))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Encrypt 用当前主密钥加密任意字节；空输入返回空串。
func (k *Keyring) Encrypt(plaintext []byte) (string, error) {
	return k.seal(plaintext, nil)
}

func (k *Keyring) seal(plaintext []byte, binding []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", nil
	}
	if k == nil || len(k.keys) == 0 {
		return "", ErrNoKey
	}
	aead := k.keys[0].aead
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return prefix + base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, plaintext, binding)), nil
}

// Decrypt 依次用当前与旧主密钥尝试解密；空载荷返回 nil。
func (k *Keyring) Decrypt(payload string) ([]byte, error) {
	plaintext, _, err := k.Open(payload)
	return plaintext, err
}

// Open 解密载荷，并报告它是否由当前主密钥加密；空载荷返回 (nil, true, nil)。
func (k *Keyring) Open(payload string) (plaintext []byte, current bool, err error) {
	return k.open(payload, nil)
}

func (k *Keyring) open(payload string, binding []byte) (plaintext []byte, current bool, err error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, true, nil
	}
	if k == nil || len(k.keys) == 0 {
		return nil, false, ErrNoKey
	}
	if !strings.HasPrefix(payload, prefix) {
		return nil, false, ErrInvalidPayload
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(payload, prefix))
	if err != nil {
		return nil, false, ErrInvalidPayload
	}
	for index, key := range k.keys {
		size := key.aead.NonceSize()
		if len(raw) < size+key.aead.Overhead() {
			return nil, false, ErrInvalidPayload
		}
		if plaintext, err := key.aead.Open(nil, raw[:size], raw[size:], binding); err == nil {
			return plaintext, index == 0, nil
		}
	}
	return nil, false, ErrDecrypt
}

// Reencrypt 把由旧主密钥加密的载荷改用当前主密钥加密，并在返回前确认新载荷可还原出相同明文。
// 已由当前主密钥加密（或为空）时返回 ("", false, nil)；所有主密钥都无法解密时返回 ErrDecrypt。
func (k *Keyring) Reencrypt(payload string) (string, bool, error) {
	return k.reencrypt(payload, nil)
}

// ReencryptBound 是 Reencrypt 对 EncryptStringBound 载荷的版本，新载荷沿用同一 binding。
func (k *Keyring) ReencryptBound(payload string, binding string) (string, bool, error) {
	return k.reencrypt(payload, []byte(binding))
}

func (k *Keyring) reencrypt(payload string, binding []byte) (string, bool, error) {
	plaintext, current, err := k.open(payload, binding)
	if err != nil || current {
		return "", false, err
	}
	if len(plaintext) == 0 {
		return "", false, errors.New("secretbox: payload holds an empty value")
	}
	next, err := k.seal(plaintext, binding)
	if err != nil {
		return "", false, err
	}
	verified, current, err := k.open(next, binding)
	if err != nil || !current || !bytes.Equal(verified, plaintext) {
		return "", false, errors.New("secretbox: re-encrypted payload failed verification")
	}
	return next, true, nil
}

// PreviousKeyCount 返回仅用于解密的旧主密钥数量（已去除空值与重复项）。
func (k *Keyring) PreviousKeyCount() int {
	if k == nil || len(k.keys) == 0 {
		return 0
	}
	return len(k.keys) - 1
}

// LookupHashes 返回 value 在每个主密钥下的 HMAC-SHA256（十六进制），当前密钥在前。
// 用于需要按值查找的确定性索引（如兑换码）：新数据用第一个写入，查找时依次尝试，
// 更换主密钥后用旧密钥写入的索引仍能找到。
func (k *Keyring) LookupHashes(value string) []string {
	if k == nil {
		return nil
	}
	hashes := make([]string, 0, len(k.keys))
	for _, key := range k.keys {
		mac := hmac.New(sha256.New, key.secret)
		mac.Write([]byte(value))
		hashes = append(hashes, hex.EncodeToString(mac.Sum(nil)))
	}
	return hashes
}
