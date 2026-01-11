package session

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// Message types
const (
	MsgInput  byte = 1
	MsgOutput byte = 2
	MsgResize byte = 3
	MsgExit   byte = 4
)

// clientInfo holds per-client state
type clientInfo struct {
	rows uint16
	cols uint16
}

// Server manages a session
type Server struct {
	session     *Session
	pty         *PTY
	listener    net.Listener
	clients     map[net.Conn]*clientInfo
	mu          sync.RWMutex
	done        chan struct{}
	ptyExited   bool
	outputBuf   []byte
	outputBufMu sync.Mutex
	hadClient   bool
}

// NewServer creates a new server for a session
func NewServer(name string, command []string) (*Server, error) {
	// Ensure data directory exists
	if _, err := EnsureDataDir(); err != nil {
		return nil, err
	}

	// Check if session already exists
	if Exists(name) {
		return nil, os.ErrExist
	}

	// Start PTY
	p, err := StartPTY(name, command)
	if err != nil {
		return nil, err
	}

	// Create Unix socket
	sockPath, err := SocketPath(name)
	if err != nil {
		_ = p.Close()
		return nil, err
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		_ = p.Close()
		return nil, err
	}

	// Save session info
	sess := &Session{
		Name:       name,
		PID:        os.Getpid(),
		Command:    command,
		LastActive: time.Now(),
	}
	if err := sess.Save(); err != nil {
		_ = listener.Close()
		_ = p.Close()
		return nil, err
	}

	return &Server{
		session:  sess,
		pty:      p,
		listener: listener,
		clients:  make(map[net.Conn]*clientInfo),
		done:     make(chan struct{}),
	}, nil
}

// Run starts the server
func (s *Server) Run() error {
	// Handle PTY output in background
	go s.handlePTYOutput()

	// Wait for PTY process to exit
	go func() {
		_ = s.pty.Wait()
		s.mu.Lock()
		s.ptyExited = true
		s.mu.Unlock()

		// Notify all clients that PTY exited
		s.broadcast(MsgExit, nil)

		// Wait briefly for client to connect if none yet
		for range 50 { // 5 seconds max
			s.mu.RLock()
			hadClient := s.hadClient
			s.mu.RUnlock()
			if hadClient {
				break
			}
			sleepMs(100)
		}

		// Small delay to ensure exit message is sent
		sleepMs(100)
		s.Shutdown()
	}()

	// Accept connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				continue
			}
		}
		go s.handleClient(conn)
	}
}

// Shutdown stops the server
func (s *Server) Shutdown() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}

	_ = s.listener.Close()
	_ = s.pty.Close()

	s.mu.Lock()
	for conn := range s.clients {
		_ = conn.Close()
	}
	s.mu.Unlock()

	// Clean up session files
	_ = Remove(s.session.Name)
}

// handlePTYOutput reads from PTY and broadcasts to all clients
func (s *Server) handlePTYOutput() {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-s.done:
			return
		default:
		}

		n, err := s.pty.File.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			// Buffer output for late-connecting clients
			s.outputBufMu.Lock()
			s.outputBuf = append(s.outputBuf, buf[:n]...)
			// Limit buffer size to 1MB
			if len(s.outputBuf) > 1024*1024 {
				s.outputBuf = s.outputBuf[len(s.outputBuf)-1024*1024:]
			}
			s.outputBufMu.Unlock()

			s.broadcast(MsgOutput, buf[:n])
		}
	}
}

// handleClient handles a single client connection
func (s *Server) handleClient(conn net.Conn) {
	s.mu.Lock()
	s.clients[conn] = &clientInfo{}
	s.hadClient = true
	// Update last active time
	s.session.LastActive = time.Now()
	_ = s.session.Save()
	s.mu.Unlock()

	// Send buffered output to new client
	s.outputBufMu.Lock()
	if len(s.outputBuf) > 0 {
		_ = writeMessage(conn, MsgOutput, s.outputBuf)
	}
	s.outputBufMu.Unlock()

	// If PTY already exited, send exit message and close
	s.mu.RLock()
	ptyExited := s.ptyExited
	s.mu.RUnlock()
	if ptyExited {
		_ = writeMessage(conn, MsgExit, nil)
		_ = conn.Close()
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		return
	}

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		// Resize PTY to a remaining client's size if any
		for _, info := range s.clients {
			if info != nil && info.rows > 0 && info.cols > 0 {
				_ = s.pty.Resize(info.rows, info.cols)
				break
			}
		}
		s.mu.Unlock()
		_ = conn.Close()
	}()

	// Read messages from client
	for {
		select {
		case <-s.done:
			return
		default:
		}

		msgType, data, err := readMessage(conn)
		if err != nil {
			return
		}

		switch msgType {
		case MsgInput:
			// Resize PTY to active client's size on input
			s.mu.RLock()
			if info := s.clients[conn]; info != nil && info.rows > 0 && info.cols > 0 {
				_ = s.pty.Resize(info.rows, info.cols)
			}
			s.mu.RUnlock()
			_, _ = s.pty.File.Write(data)
		case MsgResize:
			if len(data) >= 4 {
				rows := binary.BigEndian.Uint16(data[0:2])
				cols := binary.BigEndian.Uint16(data[2:4])
				// Store client's window size
				s.mu.Lock()
				if info := s.clients[conn]; info != nil {
					info.rows = rows
					info.cols = cols
				}
				s.mu.Unlock()
				_ = s.pty.Resize(rows, cols)
			}
		}
	}
}

// broadcast sends a message to all connected clients
func (s *Server) broadcast(msgType byte, data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.clients {
		_ = writeMessage(conn, msgType, data)
	}
}

// Protocol helpers

// Message format: [type:1byte][length:4bytes][data:N bytes]

func writeMessage(w io.Writer, msgType byte, data []byte) error {
	header := make([]byte, 5)
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func readMessage(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])
	if length > 1024*1024 { // 1MB max
		return 0, nil, io.ErrShortBuffer
	}
	data := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return 0, nil, err
		}
	}
	return msgType, data, nil
}

func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
