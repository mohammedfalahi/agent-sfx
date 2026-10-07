//go:build windows

package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/user"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

// Handler processes an incoming IPC Request and returns a Response.
type Handler func(req Request) Response

// Server listens on a Windows named pipe and serves IPC requests.
type Server struct {
	socketPath string
	listener   net.Listener
	handler    Handler
	quit       chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	closed     bool
}

// buildUserSecurityDescriptor constructs an SDDL string granting Generic All (GA)
// access strictly to the current user's SID, protected against inheritance.
func buildUserSecurityDescriptor() string {
	u, err := user.Current()
	if err == nil && u.Uid != "" {
		return fmt.Sprintf("D:P(A;;GA;;;%s)", u.Uid)
	}
	// Fallback to Owner if SID resolution fails
	return "D:P(A;;GA;;;OW)"
}

// NewServer initializes and starts listening on the given Windows named pipe.
// It applies current-user DACL restriction via SDDL.
func NewServer(socketPath string, handler Handler) (*Server, error) {
	sddl := buildUserSecurityDescriptor()
	cfg := &winio.PipeConfig{
		SecurityDescriptor: sddl,
		InputBufferSize:    MaxMessageSizeBytes * 2,
		OutputBufferSize:   MaxMessageSizeBytes * 2,
	}

	l, err := winio.ListenPipe(socketPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on pipe %s: %w", socketPath, err)
	}

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

// Close stops the server, closes the listener, and waits for active connections to finish.
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
	return err
}

// SocketPath returns the pipe path the server is listening on.
func (s *Server) SocketPath() string {
	return s.socketPath
}
