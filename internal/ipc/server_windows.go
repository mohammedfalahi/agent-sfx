//go:build windows

package ipc

type Handler func(req Request) Response

type Server struct{}

func NewServer(socketPath string, handler Handler) (*Server, error) {
	return nil, ErrUnsupportedPlatform
}

func (s *Server) Serve() error {
	return ErrUnsupportedPlatform
}

func (s *Server) Close() error {
	return nil
}

func (s *Server) SocketPath() string {
	return ""
}
