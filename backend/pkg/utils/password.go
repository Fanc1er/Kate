package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword bcrypt 哈希密码。
func HashPassword(pwd string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验明文密码与 bcrypt 哈希。
func CheckPassword(hash, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil
}
