package auth

import (
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(password string, logger *slog.Logger) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Faled to hash password",
			"error", err,
		)
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the given bcrypt hash.
func CheckPassword(password, hash string) bool {
	match := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	return match
}
