package auth

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(password string) (string, error) {
	log.Printf("[auth.password] HashPassword: hashing password (length=%d)", len(password))

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[auth.password] HashPassword: FAILED error=%v", err)
		return "", err
	}

	log.Printf("[auth.password] HashPassword: OK hash_length=%d", len(hash))
	return string(hash), nil
}

// CheckPassword reports whether password matches the given bcrypt hash.
func CheckPassword(password, hash string) bool {
	log.Printf("[auth.password] CheckPassword: verifying password against hash (hash_length=%d)", len(hash))

	match := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	log.Printf("[auth.password] CheckPassword: result=%v", match)
	return match
}
