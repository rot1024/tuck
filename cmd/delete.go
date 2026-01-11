package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/rot1024/tuck/session"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a session",
	Long:    `Delete a session by name. This will terminate the running process.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		sess, err := session.Load(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: session %q does not exist\n", name)
			os.Exit(1)
		}

		// Kill the server process
		if sess.PID > 0 {
			proc, err := os.FindProcess(sess.PID)
			if err == nil {
				_ = proc.Signal(syscall.SIGTERM)
			}
		}

		// Remove session files
		_ = session.Remove(name)
		fmt.Printf("Session %q deleted\n", name)
	},
}
