// Package keys 内置授权验签公钥，支持环境变量覆盖。
package keys

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

// PublicKeyPEM 内置 RSA 公钥（开发用），生产可用 CINSIGHT_LICENSE_PUBLIC_KEY 覆盖。
const PublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnXunTNOP66rWPPTv1JFH
b30O8FD261XJqDJBkl7arsB1XyM8fc6i061MIlMMwc7VybLGQ4lzWUR7r5rBbJ5S
yMnG2rRlSjX0HegjrtjRlMo8qmS4ySebrPu/JEYf80aiAkByFSlD8jSyBXhfko1e
WiRz5qBEWmkBl0i9NuhdHjWakl21Ve3SJQDV8ou2pblH4TJhunuRIFGKvwfr9y/m
e91dAvTEiBXrVyA3RG2q1q0u2W02p9GJ0eO5TdVPnsUUV3JaU5vNi674lpdd6FCM
S4Ja8jL1zsKh0YAS96aQquLMZ+oNdWYhVVkJI7DYk9JggLsz4E0OYCMdLw42cVn7
GwIDAQAB
-----END PUBLIC KEY-----`

// LoadPublicKey 加载验签公钥：优先环境变量，否则内置常量。
func LoadPublicKey() (*rsa.PublicKey, error) {
	pemBytes := []byte(PublicKeyPEM)
	if v := os.Getenv("CINSIGHT_LICENSE_PUBLIC_KEY"); v != "" {
		pemBytes = []byte(v)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("license: invalid public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("license: public key is not RSA")
	}
	return rsaPub, nil
}
