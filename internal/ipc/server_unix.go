//go:build !windows

package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// Handler processes an incoming IPC Request and returns a Response.
type Handler func(req Request) Response

// Server listens on a Unix domain socket and serves IPC requests.
type Server struct {
	socketPath string
	listener   net.Listener
	handler    Handler
	quit       chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	closed     bool
}

// NewServer initializes and starts listening on the given Unix socket path.
// It removes any existing stale socket file before listening.
func NewServer(socketPath string, handler Handler) (*Server, error) {
	if err := EnsureSecureDir(SocketDir(socketPath)); err != nil {
		return nil, fmt.Errorf("insecure socket directory: %w", err)
	}

	// Remove stale socket if present
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on socket %s: %w", socketPath, err)
	}

	// Restrict socket file permissions to owner-only
	_ = os.Chmod(socketPath, 0600)

	s := &Server{
		socketPath: socketPath,
		listener:   l,
		handler:    handler,
		quit:       make(chan struct{}),
	}

	return s, nil
}

// Serve begins accepting connections until Stop is called or an error occurs.
func (s *Server) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return nil
			default:
				return err
			}
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer c.Close()
			s.handleConn(c)
		}(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	// Set read and write deadline
	_ = conn.SetDeadline(time.Now().Add(100 * time.Millisecond))

	reader := bufio.NewReader(io.LimitReader(conn, MaxMessageSizeBytes+1))
	line, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		s.writeResponse(conn, Response{
			Version: ProtocolVersion,
			OK:      false,
			Error:   fmt.Sprintf("read error: %v", err),
		})
		return
	}

	if len(line) > MaxMessageSizeBytes {
		s.writeResponse(conn, Response{
			Version: ProtocolVersion,
			OK:      false,
			Error:   ErrOversizedMessage.Error(),
		})
		return
	}

	if len(line) == 0 {
		return
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeResponse(conn, Response{
			Version: ProtocolVersion,
			OK:      false,
			Error:   fmt.Sprintf("malformed json: %v", err),
		})
		return
	}

	if err := req.Validate(); err != nil {
		s.writeResponse(conn, Response{
			Version: ProtocolVersion,
			OK:      false,
			Error:   err.Error(),
		})
		return
	}

	resp := s.handler(req)
	resp.Version = ProtocolVersion
	s.writeResponse(conn, resp)
}

func (s *Server) writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = conn.Write(append(data, '\n'))
}

// Close stops the server, closes the listener, waits for active connections to finish,
// and deletes the socket file.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.quit)
	err := s.listener.Close()
	s.mu.Unlock()

	s.wg.Wait()
	_ = os.Remove(s.socketPath)
	return err
}

// SocketPath returns the socket path the server is listening on.
func (s *Server) SocketPath() string {
	return s.socketPath
}
