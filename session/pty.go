package session

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// PTY represents a pseudo-terminal
type PTY struct {
	File *os.File
	Cmd  *exec.Cmd
}

// StartPTY starts a command in a new PTY
func StartPTY(command []string) (*PTY, error) {
	var cmd *exec.Cmd
	if len(command) == 0 {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.Command(shell)
	} else {
		cmd = exec.Command(command[0], command[1:]...)
	}

	// Set up environment
	cmd.Env = os.Environ()

	// Start the command with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &PTY{
		File: ptmx,
		Cmd:  cmd,
	}, nil
}

// Resize resizes the PTY
func (p *PTY) Resize(rows, cols uint16) error {
	return pty.Setsize(p.File, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Close closes the PTY
func (p *PTY) Close() error {
	return p.File.Close()
}

// Wait waits for the command to finish
func (p *PTY) Wait() error {
	return p.Cmd.Wait()
}
