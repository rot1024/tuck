package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/rot1024/tuck/session"
	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all sessions",
	Long:  `Delete all sessions. This will terminate all running processes.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := session.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions to clear")
			return
		}

		for _, sess := range sessions {
			// Kill the server process
			if sess.PID > 0 {
				proc, err := os.FindProcess(sess.PID)
				if err == nil {
					_ = proc.Signal(syscall.SIGTERM)
				}
			}

			// Remove session files
			_ = session.Remove(sess.Name)
			fmt.Printf("Session %q deleted\n", sess.Name)
		}

		fmt.Printf("Cleared %d session(s)\n", len(sessions))
	},
}
