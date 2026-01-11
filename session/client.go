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

// DefaultDetachKeys is the default detach method (tilde + period, like SSH)
var DefaultDetachKeys = []DetachKey{{EscapeChar: '~'}}

// DetachKey represents a method to detach from a session
type DetachKey struct {
	CtrlKey    byte // Single control key (e.g., 28 for Ctrl+\)
	EscapeChar byte // Escape character for sequence (e.g., '~' for ~.)
}

// IsEscapeSequence returns true if this is an escape sequence (char + .)
func (d DetachKey) IsEscapeSequence() bool {
	return d.EscapeChar != 0
}

// String returns a human-readable representation
func (d DetachKey) String() string {
	if d.IsEscapeSequence() {
		return fmt.Sprintf("%c.", d.EscapeChar)
	}
	return formatCtrlKey(d.CtrlKey)
}

// formatCtrlKey formats a control key byte to a human-readable string
func formatCtrlKey(key byte) string {
	switch key {
	case 27:
		return "Ctrl+["
	case 28:
		return "Ctrl+\\"
	case 29:
		return "Ctrl+]"
	case 30:
		return "Ctrl+^"
	case 31:
		return "Ctrl+_"
	}
	if key >= 1 && key <= 26 {
		return fmt.Sprintf("Ctrl+%c", 'A'+key-1)
	}
	return fmt.Sprintf("0x%02x", key)
}

// ParseDetachKey parses a detach key string
// Formats:
//   - "ctrl-a", "^a" → control key
//   - "~.", "`." → escape sequence (char followed by .)
func ParseDetachKey(s string) (DetachKey, error) {
	if s == "" {
		return DetachKey{}, fmt.Errorf("empty detach key")
	}

	// Handle escape sequence format: X. (any char followed by .)
	if len(s) == 2 && s[1] == '.' {
		return DetachKey{EscapeChar: s[0]}, nil
	}

	// Handle ctrl-X format
	if len(s) >= 6 && (s[:5] == "ctrl-" || s[:5] == "Ctrl-") {
		char := s[5:]
		if key, ok := parseCtrlChar(char); ok {
			return DetachKey{CtrlKey: key}, nil
		}
	}

	// Handle ^X format
	if len(s) >= 2 && s[0] == '^' {
		char := s[1:]
		if key, ok := parseCtrlChar(char); ok {
			return DetachKey{CtrlKey: key}, nil
		}
	}

	return DetachKey{}, fmt.Errorf("invalid detach key: %q (use ctrl-a, ^a, ~., `., etc.)", s)
}

// parseCtrlChar parses a character for ctrl combination
func parseCtrlChar(char string) (byte, bool) {
	// Special characters
	switch char {
	case "[":
		return 27, true // Ctrl+[ (ESC)
	case "\\", "backslash":
		return 28, true // Ctrl+\
	case "]":
		return 29, true // Ctrl+]
	case "^", "caret":
		return 30, true // Ctrl+^
	case "_", "underscore":
		return 31, true // Ctrl+_
	}

	if len(char) == 1 {
		c := char[0]
		if c >= 'a' && c <= 'z' {
			return c - 'a' + 1, true // ctrl-a = 1, ctrl-b = 2, etc.
		}
		if c >= 'A' && c <= 'Z' {
			return c - 'A' + 1, true
		}
	}
	return 0, false
}

// FormatDetachKeys formats multiple detach keys for display
func FormatDetachKeys(keys []DetachKey) string {
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 {
		return keys[0].String()
	}
	result := keys[0].String()
	for _, k := range keys[1:] {
		result += " or " + k.String()
	}
	return result
}

// Client connects to a session
type Client struct {
	conn       net.Conn
	oldState   *term.State
	done       chan struct{}
	name       string
	quiet      bool
	detachKeys []DetachKey
	// Escape sequence state (tracks state for each escape char)
	afterNewline  bool
	sawEscapeChar byte // The escape char we saw (0 if none)
	// Terminal ESC sequence tracking (to ignore focus events etc.)
	inEscSeq bool
}

// AttachOptions contains options for attaching to a session
type AttachOptions struct {
	Quiet            bool
	SuppressAttached bool        // Don't show "attached" message (for new session)
	DetachKeys       []DetachKey // Keys/sequences to detach (nil = use default)
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

	detachKeys := opts.DetachKeys
	if len(detachKeys) == 0 {
		detachKeys = DefaultDetachKeys
	}

	c := &Client{
		conn:         conn,
		done:         make(chan struct{}),
		name:         name,
		quiet:        opts.Quiet,
		detachKeys:   detachKeys,
		afterNewline: true, // Start as if we just saw a newline
	}

	return c.run(!opts.SuppressAttached)
}

func (c *Client) run(showAttached bool) error {
	defer c.conn.Close()

	// Show attach message before entering raw mode
	if showAttached && !c.quiet {
		fmt.Fprintf(os.Stderr, "[%s: 🔗 attached %q (%s to detach)]\n", AppName, c.name, FormatDetachKeys(c.detachKeys))
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
			// Track newlines in output for escape sequence detection (like SSH)
			for _, b := range data {
				if b == '\n' || b == '\r' {
					c.afterNewline = true
					break
				}
			}
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

		// Process input byte by byte for escape sequence detection
		var toSend []byte
		for i := 0; i < n; i++ {
			b := buf[i]

			// Check for single-key detach (control keys)
			for _, dk := range c.detachKeys {
				if !dk.IsEscapeSequence() && b == dk.CtrlKey {
					c.doDetach()
					return nil
				}
			}

			// Escape sequence state machine
			if c.sawEscapeChar != 0 {
				// We previously saw an escape char after a newline
				escChar := c.sawEscapeChar
				c.sawEscapeChar = 0
				switch b {
				case '.':
					// X. = detach
					c.doDetach()
					return nil
				case escChar:
					// XX = send single X
					toSend = append(toSend, escChar)
				default:
					// Not a recognized sequence, send buffered escape char and current char
					toSend = append(toSend, escChar, b)
				}
				// Update newline state based on current char
				c.afterNewline = (b == '\n' || b == '\r')
			} else if c.afterNewline && c.isEscapeChar(b) {
				// Escape char after newline - start escape sequence
				c.sawEscapeChar = b
				c.afterNewline = false
			} else {
				// Normal character
				toSend = append(toSend, b)

				// Track terminal ESC sequences (like focus events) to ignore them
				if b == 27 { // ESC
					c.inEscSeq = true
				} else if c.inEscSeq {
					// Check if ESC sequence ends (letter terminates CSI sequences)
					if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
						c.inEscSeq = false
					}
					// Don't update afterNewline while in ESC sequence
				} else {
					// Not in ESC sequence - update afterNewline normally
					if b == '\n' || b == '\r' {
						c.afterNewline = true
					} else if b >= 32 && b < 127 {
						// Printable ASCII - user is typing, reset afterNewline
						c.afterNewline = false
					}
				}
			}
		}

		if len(toSend) > 0 {
			writeMessage(c.conn, MsgInput, toSend)
		}
	}
}

// isEscapeChar checks if byte is a configured escape character
func (c *Client) isEscapeChar(b byte) bool {
	for _, dk := range c.detachKeys {
		if dk.IsEscapeSequence() && dk.EscapeChar == b {
			return true
		}
	}
	return false
}

func (c *Client) doDetach() {
	c.close()
	c.restore()
	if !c.quiet {
		fmt.Fprintf(os.Stderr, "\n[%s: 👋 detached %q]\n", AppName, c.name)
	}
}

func (c *Client) close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}
