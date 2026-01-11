package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

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
			fmt.Printf("%s\t%s\t%s\n", s.Name, formatRelativeTime(s.LastActive), cmdStr)
		}
	},
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
