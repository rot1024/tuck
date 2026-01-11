package cmd

import (
	"fmt"
	"os"

	"github.com/rot1024/tuck/session"
	"github.com/spf13/cobra"
)

// getDetachKey returns the detach key from flag or environment variable
func getDetachKey() (byte, error) {
	keyStr := detachKeyFlag
	if keyStr == "" {
		keyStr = os.Getenv("TUCK_DETACH_KEY")
	}
	if keyStr == "" {
		return session.DefaultDetachKey, nil
	}
	key, err := session.ParseDetachKey(keyStr)
	if err != nil {
		return 0, err
	}
	return key, nil
}

// mustGetDetachKey returns the detach key or exits on error
func mustGetDetachKey() byte {
	key, err := getDetachKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return key
}

var (
	quietFlag     bool
	detachKeyFlag string
)

var rootCmd = &cobra.Command{
	Use:   "tuck",
	Short: "A simple terminal session manager",
	Long: `tuck is a lightweight terminal session manager that allows you to
detach and reattach terminal sessions without screen splitting.

Unlike tmux or screen, tuck does not use the alternate screen buffer,
so your terminal's scrollback buffer remains functional.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default to "tuck new" behavior
		newCmd.Run(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Suppress status messages")
	rootCmd.PersistentFlags().StringVarP(&detachKeyFlag, "detach-key", "d", "", "Detach key (e.g., ctrl-a, ctrl-b, default: ctrl-\\)")

	// Allow command arguments with dashes (e.g., "claude --continue")
	newCmd.Flags().SetInterspersed(false)
	createCmd.Flags().SetInterspersed(false)
	rootCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
}
