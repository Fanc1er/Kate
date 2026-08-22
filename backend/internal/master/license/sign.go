package license

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
)

// signPayload 使用 RSA-SHA256 对载荷密文签名。
func signPayload(data []byte, priv *rsa.PrivateKey) ([]byte, error) {
	digest := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
}

// verifyPayload 使用 RSA-SHA256 验签。
func verifyPayload(data, sig []byte, pub *rsa.PublicKey) error {
	digest := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}
