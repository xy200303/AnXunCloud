// Package password 提供 bcrypt 哈希与密码/账号格式校验。
package password

import (
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{4,32}$`)
	phoneRe    = regexp.MustCompile(`^1\d{10}$`)
	letterRe   = regexp.MustCompile(`[a-zA-Z]`)
	digitRe    = regexp.MustCompile(`\d`)
)

// Hash 生成 bcrypt 哈希。
func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// Compare 校验明文与哈希是否匹配。
func Compare(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

// ValidPassword 校验密码：8–32 位且须含字母与数字。
func ValidPassword(pwd string) bool {
	return len(pwd) >= 8 && len(pwd) <= 32 && letterRe.MatchString(pwd) && digitRe.MatchString(pwd)
}

// ValidUsername 校验登录名：4–32 位字母数字下划线。
func ValidUsername(name string) bool { return usernameRe.MatchString(name) }

// ValidPhone 校验手机号：1 开头 11 位数字。
func ValidPhone(phone string) bool { return phoneRe.MatchString(phone) }
