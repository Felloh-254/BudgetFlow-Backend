// Stub for github.com/golang-jwt/jwt (v3 compat) - used by echo's JWT middleware which we don't use
package jwt

import "errors"

type MapClaims map[string]interface{}
func (m MapClaims) Valid() error { return nil }

type Claims interface{ Valid() error }

type SigningMethod interface {
	Alg() string
	Sign(signingString string, key interface{}) (string, error)
	Verify(signingString, signature string, key interface{}) error
}

type Token struct {
	Claims Claims
	Valid   bool
	Method  SigningMethod
	Header  map[string]interface{}
}

type Keyfunc func(*Token) (interface{}, error)

var ErrSignatureInvalid = errors.New("signature invalid")

func Parse(tokenString string, keyFunc Keyfunc) (*Token, error) {
	return nil, errors.New("not implemented")
}

func ParseWithClaims(tokenString string, claims Claims, keyFunc Keyfunc) (*Token, error) {
	return nil, errors.New("not implemented")
}
