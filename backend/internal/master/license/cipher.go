package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

const aesKeySeed = "cinsight-license-v1::aes-gcm"

// deriveAESKey 派生 AES-256 密钥，签发与验证两端保持一致。
func deriveAESKey() []byte {
	h := sha256.Sum256([]byte(aesKeySeed))
	return h[:]
}

// encryptPayload 使用 AES-256-GCM 加密载荷。
func encryptPayload(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// decryptPayload 使用 AES-256-GCM 解密载荷。
func decryptPayload(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("license: invalid nonce size")
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
