// Stub package for golang.org/x/net/http2
package http2

import "net/http"

type Server struct {
	MaxHandlers int
}

func ConfigureServer(s *http.Server, conf *Server) error { return nil }
