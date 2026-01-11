package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rot1024/tuck/session"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:     "new [command...]",
	Aliases: []string{"n"},
	Short:   "Create a new session with auto-generated name",
	Long: `Create a new session with an auto-generated name based on current directory.
If no command is specified, the default shell is used.

After creating the session, you will be automatically attached to it.
Use Ctrl+\ to detach from the session.`,
	Run: func(cmd *cobra.Command, args []string) {
		name := generateSessionName()
		createAndAttachSession(name, args)
	},
}

var createCmd = &cobra.Command{
	Use:     "create <name> [command...]",
	Aliases: []string{"c"},
	Short:   "Create a new session with specified name",
	Long: `Create a new session with the specified name and command.
If no command is specified, the default shell is used.

After creating the session, you will be automatically attached to it.
Use Ctrl+\ to detach from the session.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		command := args[1:]
		createAndAttachSession(name, command)
	},
}

func createAndAttachSession(name string, command []string) {
	if session.Exists(name) {
		fmt.Fprintf(os.Stderr, "Error: session %q already exists\n", name)
		os.Exit(1)
	}

	// Fork to create server process
	if os.Getenv("TUCK_SERVER") == "1" {
		// We are the server process
		runServer(name, command)
		return
	}

	// Start server process in background
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	serverArgs := append([]string{"create", name}, command...)
	serverCmd := exec.Command(exe, serverArgs...)
	serverCmd.Env = append(os.Environ(), "TUCK_SERVER=1")
	serverCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Capture server stderr for error reporting
	errPath, _ := session.ErrorPath(name)
	if errPath != "" {
		os.Remove(errPath) // Clean up any previous error
	}

	if err := serverCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}

	// Wait a bit for server to start and then attach
	for i := range 50 {
		if session.Exists(name) {
			break
		}
		// Check if server wrote an error
		if errPath != "" {
			if errData, err := os.ReadFile(errPath); err == nil && len(errData) > 0 {
				os.Remove(errPath)
				fmt.Fprintf(os.Stderr, "Error: %s\n", string(errData))
				os.Exit(1)
			}
		}
		if i == 49 {
			break
		}
		sleepMs(100)
	}

	if !session.Exists(name) {
		// Check for error file one more time
		if errPath != "" {
			if errData, err := os.ReadFile(errPath); err == nil && len(errData) > 0 {
				os.Remove(errPath)
				fmt.Fprintf(os.Stderr, "Error: %s\n", string(errData))
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "Error: failed to create session (server did not start)\n")
		os.Exit(1)
	}

	// Show created message
	if !quietFlag {
		fmt.Fprintf(os.Stderr, "[%s: ✨ created %q]\n", session.AppName, name)
	}

	// Attach to the session
	if err := session.Attach(name, session.AttachOptions{
		Quiet:            quietFlag,
		SuppressAttached: true,
		DetachKey:        mustGetDetachKey(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(name string, command []string) {
	server, err := session.NewServer(name, command)
	if err != nil {
		// Write error to file for client to read
		if errPath, pathErr := session.ErrorPath(name); pathErr == nil {
			os.WriteFile(errPath, []byte(err.Error()), 0600)
		}
		os.Exit(1)
	}
	server.Run()
}

func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// generateSessionName creates a session name from current directory
func generateSessionName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "session"
	}
	base := filepath.Base(cwd)
	if base == "" || base == "/" || base == "." {
		base = "session"
	}

	// If base name is available, use it
	if !session.Exists(base) {
		return base
	}

	// Otherwise, append a number
	for i := 1; i < 1000; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !session.Exists(name) {
			return name
		}
	}
	return base
}
