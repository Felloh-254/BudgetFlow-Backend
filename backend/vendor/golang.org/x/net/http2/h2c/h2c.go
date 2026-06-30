// Stub package for golang.org/x/net/http2/h2c
package h2c

import (
	"net/http"
	"golang.org/x/net/http2"
)

func NewHandler(h http.Handler, s *http2.Server) http.Handler { return h }
