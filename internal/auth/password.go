// Package auth provides password hashing and stateless bearer tokens used to
// authenticate and authorize API requests.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of the plaintext password, suitable for
// persisting. Never store the plaintext.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether plaintext matches a previously stored bcrypt
// hash. It runs in constant time relative to the hash, courtesy of bcrypt.
func CheckPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}
