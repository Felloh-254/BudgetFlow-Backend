// Stub package for golang.org/x/crypto/acme/autocert
package autocert

import (
	"crypto/tls"
	"context"
)

var AcceptTOS = func(tosURL string) bool { return true }

type Manager struct {
	Prompt     func(tosURL string) bool
	HostPolicy HostPolicy
	Cache      Cache
	Email      string
}

func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, nil
}
func (m *Manager) TLSConfig() *tls.Config { return &tls.Config{} }
func (m *Manager) HTTPHandler(fallback interface{}) interface{} { return nil }

type HostPolicy func(ctx context.Context, host string) error
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
}
