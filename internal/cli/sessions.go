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
var showWorkflowOnly bool

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
	Long: `Show details of a session with visual workflow diagram.

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

		// Header
		fmt.Println()
		fmt.Println("╔═══════════════════════════════════════════════════════════╗")
		fmt.Printf("║  📋 Session: %-44s ║\n", session.ID)
		fmt.Printf("║  📁 Workflow: %-43s ║\n", truncateStr(session.Workflow, 43))
		fmt.Printf("║  🕐 Created: %-44s ║\n", session.CreatedAt.Format("Jan 02 15:04"))
		fmt.Printf("║  📨 Messages: %-43d ║\n", len(session.Messages))
		fmt.Println("╠═══════════════════════════════════════════════════════════╣")

		// Group messages by parallel execution (same timestamp range = parallel)
		// For now, detect parallel by checking if agents ran close together
		if len(session.Messages) >= 2 {
			// Check for parallel pattern: multiple agents with similar timestamps before a final one
			parallelAgents := []memory.Message{}
			finalAgent := memory.Message{}

			// Simple heuristic: if first N-1 messages are close in time, they're parallel
			for i, msg := range session.Messages {
				if i < len(session.Messages)-1 {
					parallelAgents = append(parallelAgents, msg)
				} else {
					finalAgent = msg
				}
			}

			// Display parallel branches
			if len(parallelAgents) > 1 {
				fmt.Println("║                    PARALLEL EXECUTION                     ║")
				fmt.Println("║  ┌─────────────────────┐   ┌─────────────────────┐       ║")

				// Show first two parallel agents
				agent1 := truncateStr(parallelAgents[0].AgentID, 19)
				agent2 := ""
				if len(parallelAgents) > 1 {
					agent2 = truncateStr(parallelAgents[1].AgentID, 19)
				}

				fmt.Printf("║  │ %-19s │   │ %-19s │       ║\n", agent1, agent2)
				fmt.Printf("║  │ %-19s │   │ %-19s │       ║\n",
					truncateStr(parallelAgents[0].Role, 19),
					truncateStr(safeGetRole(parallelAgents, 1), 19))
				fmt.Println("║  └─────────────────────┘   └─────────────────────┘       ║")
				fmt.Println("║            ╲                     ╱                       ║")
				fmt.Println("║             ╲                   ╱                        ║")
				fmt.Println("║              ▼                 ▼                         ║")
				fmt.Println("║         ┌─────────────────────────┐                      ║")
				fmt.Printf("║         │ %-23s │                      ║\n", truncateStr(finalAgent.AgentID, 23))
				fmt.Printf("║         │ %-23s │                      ║\n", truncateStr(finalAgent.Role, 23))
				fmt.Println("║         └─────────────────────────┘                      ║")
			}
		}

		// Only show message details if --workflow flag is not set
		if !showWorkflowOnly {
			fmt.Println("╠═══════════════════════════════════════════════════════════╣")
			fmt.Println("║                      MESSAGE DETAILS                      ║")
			fmt.Println("╚═══════════════════════════════════════════════════════════╝")
			fmt.Println()

			// Message details
			for i, msg := range session.Messages {
				icon := "💬"
				if msg.Role == "Backend Engineer" {
					icon = "⚙️"
				} else if msg.Role == "Frontend Engineer" {
					icon = "🎨"
				} else if msg.Role == "Tech Lead" || msg.Role == "reviewer" {
					icon = "👀"
				}

				fmt.Printf("┌──────────────────────────────────────────────────────────────┐\n")
				fmt.Printf("│ %s Message %d: [%s] - %s\n", icon, i+1, msg.AgentID, msg.Role)
				fmt.Printf("│ 🕐 %s\n", msg.Timestamp.Format("15:04:05"))
				fmt.Printf("├──────────────────────────────────────────────────────────────┤\n")

				content := msg.Content
				if !showFull && len(content) > 300 {
					content = content[:300] + "\n... [truncated - use --full to see complete content]"
				}

				// Indent content
				lines := splitLines(content)
				for _, line := range lines {
					if len(line) > 60 {
						// Word wrap long lines
						wrapped := wordWrap(line, 60)
						for _, w := range wrapped {
							fmt.Printf("│ %s\n", w)
						}
					} else {
						fmt.Printf("│ %s\n", line)
					}
				}
				fmt.Printf("└──────────────────────────────────────────────────────────────┘\n\n")
			}
		} // end if !showWorkflowOnly
	},
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func safeGetRole(msgs []memory.Message, idx int) string {
	if idx < len(msgs) {
		return msgs[idx].Role
	}
	return ""
}

func splitLines(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == '\n' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func wordWrap(s string, width int) []string {
	if len(s) <= width {
		return []string{s}
	}

	result := []string{}
	for len(s) > width {
		// Find last space before width
		idx := width
		for idx > 0 && s[idx] != ' ' {
			idx--
		}
		if idx == 0 {
			idx = width // No space found, just cut
		}
		result = append(result, s[:idx])
		s = s[idx:]
		if len(s) > 0 && s[0] == ' ' {
			s = s[1:] // Skip leading space
		}
	}
	if len(s) > 0 {
		result = append(result, s)
	}
	return result
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
	sessionsShowCmd.Flags().BoolVarP(&showWorkflowOnly, "workflow", "w", false, "Show only the workflow diagram")
}
