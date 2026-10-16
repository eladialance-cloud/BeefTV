package auth

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// minPasswordLength is the shortest accepted password, counted in runes so a
// multi-byte password is not rejected for its byte length.
const minPasswordLength = 8

// ErrWeakPassword is returned when a password fails the minimum length rule.
var ErrWeakPassword = errors.New("密码至少 8 位")

// HashPassword returns a salted bcrypt digest. Two calls with the same input
// produce different digests, so identical passwords never share a hash.
func HashPassword(plain string) (string, error) {
	if len([]rune(strings.TrimSpace(plain))) < minPasswordLength {
		return "", ErrWeakPassword
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(digest), nil
}

// VerifyPassword reports whether plain matches the stored digest. A missing or
// malformed digest is a failed match, never a panic.
func VerifyPassword(hash string, plain string) bool {
	if strings.TrimSpace(hash) == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
