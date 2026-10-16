package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
)

// sessionTokenBytes is the amount of entropy behind each clear session token.
const sessionTokenBytes = 32

// NewSessionToken returns a fresh URL-safe clear token together with the
// SHA-256 hash that must be persisted. The clear token is handed to the caller
// once and never stored.
func NewSessionToken() (token string, tokenHash string, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashSessionToken(token), nil
}

// HashSessionToken maps a clear token to the value stored on the session row.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
