package cmd

import (
	"fmt"
	"os"

	"github.com/rot1024/tuck/session"
	"github.com/spf13/cobra"
)

// getDetachKeys returns the detach keys from flags or environment variables
func getDetachKeys() ([]session.DetachKey, error) {
	var keyStrs []string

	// Collect from flags
	keyStrs = append(keyStrs, detachKeyFlags...)

	// Collect from environment variables (TUCK_DETACH_KEY, TUCK_DETACH_KEY_1, TUCK_DETACH_KEY_2, ...)
	if envKey := os.Getenv("TUCK_DETACH_KEY"); envKey != "" {
		keyStrs = append(keyStrs, envKey)
	}
	for i := 1; ; i++ {
		envKey := os.Getenv(fmt.Sprintf("TUCK_DETACH_KEY_%d", i))
		if envKey == "" {
			break
		}
		keyStrs = append(keyStrs, envKey)
	}

	// Use defaults if none specified
	if len(keyStrs) == 0 {
		return session.DefaultDetachKeys, nil
	}

	// Parse all keys
	var keys []session.DetachKey
	for _, s := range keyStrs {
		key, err := session.ParseDetachKey(s)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// mustGetDetachKeys returns the detach keys or exits on error
func mustGetDetachKeys() []session.DetachKey {
	keys, err := getDetachKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return keys
}

var (
	quietFlag      bool
	detachKeyFlags []string
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
	rootCmd.PersistentFlags().StringArrayVarP(&detachKeyFlags, "detach-key", "d", nil, "Detach key (e.g., `., ~., ctrl-a). Can be specified multiple times")

	// Allow command arguments with dashes (e.g., "claude --continue")
	newCmd.Flags().SetInterspersed(false)
	createCmd.Flags().SetInterspersed(false)
	rootCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(clearCmd)
}
