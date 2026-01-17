/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"Orkflow/internal/memory"

	"github.com/spf13/cobra"
)

var showFull bool

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage workflow sessions",
	Long:  `List, view, and manage saved workflow sessions.`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := memory.ListSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No saved sessions found.")
			fmt.Println("Run a workflow to create a session.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tWORKFLOW\tMESSAGES\tLAST UPDATED")
		fmt.Fprintln(w, "--\t--------\t--------\t------------")

		for _, s := range sessions {
			ago := time.Since(s.UpdatedAt).Round(time.Minute)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s ago\n", s.ID, s.Workflow, len(s.Messages), ago)
		}
		w.Flush()
	},
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show details of a session",
	Long: `Show details of a session.

Use --full to display complete message content instead of truncated.

Examples:
  orka sessions show abc123
  orka sessions show abc123 --full`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		session, err := memory.LoadSession(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Session: %s\n", session.ID)
		fmt.Printf("Workflow: %s\n", session.Workflow)
		fmt.Printf("Created: %s\n", session.CreatedAt.Format(time.RFC1123))
		fmt.Printf("Updated: %s\n", session.UpdatedAt.Format(time.RFC1123))
		fmt.Printf("Messages: %d\n\n", len(session.Messages))

		for i, msg := range session.Messages {
			fmt.Printf("╔══ Message %d [%s - %s] ══╗\n", i+1, msg.AgentID, msg.Role)
			if showFull || len(msg.Content) <= 200 {
				fmt.Printf("%s\n", msg.Content)
			} else {
				fmt.Printf("%s...\n[truncated - use --full to see complete content]\n", msg.Content[:200])
			}
			fmt.Println()
		}
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := memory.DeleteSession(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting session: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Session %s deleted.\n", args[0])
	},
}

var sessionsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove expired and excess sessions",
	Run: func(cmd *cobra.Command, args []string) {
		if err := memory.CleanupOldSessions(); err != nil {
			fmt.Fprintf(os.Stderr, "Error cleaning sessions: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Sessions cleaned up.")
	},
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsCleanCmd)

	sessionsShowCmd.Flags().BoolVarP(&showFull, "full", "f", false, "Show complete message content")
}
