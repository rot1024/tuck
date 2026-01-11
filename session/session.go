package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Session represents a tuck session
type Session struct {
	Name    string   `json:"name"`
	PID     int      `json:"pid"`
	Command []string `json:"command"`
}

// DataDir returns the directory for storing session data
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "tuck"), nil
}

// EnsureDataDir creates the data directory if it doesn't exist
func EnsureDataDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	return dir, nil
}

// SocketPath returns the socket path for a session
func SocketPath(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".sock"), nil
}

// InfoPath returns the info file path for a session
func InfoPath(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// ErrorPath returns the error file path for a session
func ErrorPath(name string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".err"), nil
}

// Save saves session info to disk
func (s *Session) Save() error {
	path, err := InfoPath(s.Name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session info: %w", err)
	}
	return nil
}

// Load loads session info from disk
func Load(name string) (*Session, error) {
	path, err := InfoPath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session info: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session info: %w", err)
	}
	return &s, nil
}

// Exists checks if a session exists
func Exists(name string) bool {
	path, err := SocketPath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// List returns all sessions
func List() ([]*Session, error) {
	dir, err := DataDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // remove .json
		s, err := Load(name)
		if err != nil {
			continue
		}
		// Check if the process is still running
		if !isProcessRunning(s.PID) {
			// Clean up stale session
			_ = Remove(name)
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Remove removes a session's files
func Remove(name string) error {
	sockPath, _ := SocketPath(name)
	infoPath, _ := InfoPath(name)
	errPath, _ := ErrorPath(name)
	_ = os.Remove(sockPath)
	_ = os.Remove(infoPath)
	_ = os.Remove(errPath)
	return nil
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
