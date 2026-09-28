package auth

import (
	"errors"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager issues and validates JWTs. It's a struct (not package-level
// functions) so the secret is injected via config rather than hardcoded,
// and so it can be swapped for a mock in tests.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	log.Printf("[auth.jwt] NewTokenManager created: ttl=%s secret_length=%d", ttl, len(secret))
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

func (m *TokenManager) Generate(userID int) (string, error) {
	log.Printf("[auth.jwt] Generate: generating token for user_id=%d ttl=%s", userID, m.ttl)

	claims := jwt.MapClaims{
		"user_id": userID,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(m.ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		log.Printf("[auth.jwt] Generate: FAILED user_id=%d error=%v", userID, err)
		return "", err
	}

	log.Printf("[auth.jwt] Generate: OK user_id=%d token_length=%d", userID, len(signed))
	return signed, nil
}

// Parse validates the token and returns the embedded user ID.
func (m *TokenManager) Parse(tokenStr string) (int, error) {
	log.Printf("[auth.jwt] Parse: parsing token (length=%d)", len(tokenStr))

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("[auth.jwt] Parse: unexpected signing method: %v", t.Header["alg"])
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		log.Printf("[auth.jwt] Parse: invalid token error=%v valid=%v", err, token != nil && token.Valid)
		return 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("[auth.jwt] Parse: invalid claims type")
		return 0, errors.New("invalid token claims")
	}

	uidFloat, ok := claims["user_id"].(float64)
	if !ok {
		log.Printf("[auth.jwt] Parse: invalid user_id claim: %v", claims["user_id"])
		return 0, errors.New("invalid user_id claim")
	}

	userID := int(uidFloat)
	log.Printf("[auth.jwt] Parse: OK user_id=%d", userID)
	return userID, nil
}
