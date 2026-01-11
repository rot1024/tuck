package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rot1024/tuck/session"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all sessions",
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := session.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions")
			return
		}

		for _, s := range sessions {
			cmdStr := strings.Join(s.Command, " ")
			if cmdStr == "" {
				cmdStr = "(default shell)"
			}
			fmt.Printf("%s\t%s\n", s.Name, cmdStr)
		}
	},
}
