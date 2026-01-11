package session

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// AppName is the application name used in messages
const AppName = "tuck"

// DefaultDetachKey is Ctrl+\ (ASCII 28)
const DefaultDetachKey = 28

// ParseDetachKey parses a detach key string like "ctrl-a", "ctrl-\\", etc.
func ParseDetachKey(s string) (byte, error) {
	if s == "" {
		return DefaultDetachKey, nil
	}

	// Handle ctrl-X format
	if len(s) >= 6 && (s[:5] == "ctrl-" || s[:5] == "Ctrl-") {
		char := s[5:]
		if char == "\\" || char == "backslash" {
			return 28, nil // Ctrl+\
		}
		if len(char) == 1 {
			c := char[0]
			if c >= 'a' && c <= 'z' {
				return c - 'a' + 1, nil // ctrl-a = 1, ctrl-b = 2, etc.
			}
			if c >= 'A' && c <= 'Z' {
				return c - 'A' + 1, nil
			}
		}
	}

	// Handle ^X format
	if len(s) == 2 && s[0] == '^' {
		c := s[1]
		if c == '\\' {
			return 28, nil
		}
		if c >= 'a' && c <= 'z' {
			return c - 'a' + 1, nil
		}
		if c >= 'A' && c <= 'Z' {
			return c - 'A' + 1, nil
		}
	}

	return 0, fmt.Errorf("invalid detach key: %q (use ctrl-a, ctrl-b, etc.)", s)
}

// FormatDetachKey formats a detach key byte to a human-readable string
func FormatDetachKey(key byte) string {
	if key == 28 {
		return "Ctrl+\\"
	}
	if key >= 1 && key <= 26 {
		return fmt.Sprintf("Ctrl+%c", 'A'+key-1)
	}
	return fmt.Sprintf("0x%02x", key)
}

// Client connects to a session
type Client struct {
	conn      net.Conn
	oldState  *term.State
	done      chan struct{}
	name      string
	quiet     bool
	detachKey byte
}

// AttachOptions contains options for attaching to a session
type AttachOptions struct {
	Quiet            bool
	SuppressAttached bool // Don't show "attached" message (for new session)
	DetachKey        byte // Key to detach (0 = use default)
}

// Attach connects to an existing session
func Attach(name string, opts AttachOptions) error {
	if !Exists(name) {
		return fmt.Errorf("session %q does not exist", name)
	}

	sockPath, err := SocketPath(name)
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return fmt.Errorf("failed to connect to session: %w", err)
	}

	detachKey := opts.DetachKey
	if detachKey == 0 {
		detachKey = DefaultDetachKey
	}

	c := &Client{
		conn:      conn,
		done:      make(chan struct{}),
		name:      name,
		quiet:     opts.Quiet,
		detachKey: detachKey,
	}

	return c.run(!opts.SuppressAttached)
}

func (c *Client) run(showAttached bool) error {
	defer c.conn.Close()

	// Show attach message before entering raw mode
	if showAttached && !c.quiet {
		fmt.Fprintf(os.Stderr, "[%s: 🔗 attached %q (%s to detach)]\n", AppName, c.name, FormatDetachKey(c.detachKey))
	}

	// Set terminal to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	c.oldState = oldState
	defer c.restore()

	// Send initial window size
	c.sendWindowSize()

	// Handle window resize
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-sigwinch:
				c.sendWindowSize()
			case <-c.done:
				return
			}
		}
	}()
	defer signal.Stop(sigwinch)

	// Handle output from server
	go c.handleOutput()

	// Handle input from terminal
	return c.handleInput()
}

func (c *Client) restore() {
	if c.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), c.oldState)
	}
}

func (c *Client) sendWindowSize() {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], uint16(height))
	binary.BigEndian.PutUint16(data[2:4], uint16(width))
	writeMessage(c.conn, MsgResize, data)
}

func (c *Client) handleOutput() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		msgType, data, err := readMessage(c.conn)
		if err != nil {
			if err != io.EOF {
				// Connection closed
			}
			c.close()
			return
		}

		switch msgType {
		case MsgOutput:
			os.Stdout.Write(data)
		case MsgExit:
			// Restore terminal and show message
			c.restore()
			if !c.quiet {
				fmt.Fprintf(os.Stderr, "\n[%s: 🏁 ended %q]\n", AppName, c.name)
			}
			os.Exit(0)
		}
	}
}

func (c *Client) handleInput() error {
	buf := make([]byte, 1024)
	for {
		select {
		case <-c.done:
			return nil
		default:
		}

		n, err := os.Stdin.Read(buf)
		if err != nil {
			return err
		}

		// Check for detach key
		for i := 0; i < n; i++ {
			if buf[i] == c.detachKey {
				c.close()
				c.restore()
				if !c.quiet {
					fmt.Fprintf(os.Stderr, "\n[%s: 👋 detached %q]\n", AppName, c.name)
				}
				return nil
			}
		}

		if n > 0 {
			writeMessage(c.conn, MsgInput, buf[:n])
		}
	}
}

func (c *Client) close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}
