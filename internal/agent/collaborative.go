package agent

import (
	"fmt"
	"time"

	"Orkflow/internal/memory"
	"Orkflow/pkg/types"
)

const (
	DefaultMaxTurns      = 100 // High limit - agents should stop via <DONE/>, not turn limit
	MessagePollTimeout   = 100 * time.Millisecond
	MessageCollectWindow = 500 * time.Millisecond
)

// RunCollaborativeAgent runs an agent in collaborative mode with real-time messaging.
// The agent:
//  1. Subscribes to the message channel
//  2. Runs in a loop for MaxTurns:
//     - Collects new messages from inbox
//     - Builds prompt with message context
//     - Generates response via LLM
//     - Parses and sends outgoing messages
//     - Checks for DONE signal
//  3. Returns the final output
func (r *Runner) RunCollaborativeAgent(agentDef *types.Agent, channel *memory.MessageChannel) (string, error) {
	client, ok := r.Clients[agentDef.Model]
	if !ok {
		return "", fmt.Errorf("model not found: %s", agentDef.Model)
	}

	maxTurns := agentDef.MaxTurns
	if maxTurns <= 0 {
		maxTurns = DefaultMaxTurns
	}

	// Subscribe to the message channel
	inbox := channel.Subscribe(agentDef.ID)

	// Ensure we unsubscribe when done
	defer func() {
		channel.Unsubscribe(agentDef.ID)
	}()

	var conversation []string
	var allReceivedMessages []memory.ChannelMessage

	fmt.Printf("[%s] 🤝 Starting collaborative agent (max %d turns)\n", agentDef.ID, maxTurns)
	if r.Logger != nil {
		r.Logger.LogAgent(agentDef.ID, "COLLABORATIVE_START", fmt.Sprintf("MaxTurns: %d", maxTurns))
	}

	for turn := 0; turn < maxTurns; turn++ {
		// 1. Collect new messages (non-blocking with timeout)
		newMessages := r.collectMessages(inbox, agentDef.ListensTo)
		allReceivedMessages = append(allReceivedMessages, newMessages...)

		// Log received messages
		for _, msg := range newMessages {
			fmt.Printf("[%s] 📨 Received from %s: %s\n", agentDef.ID, msg.From, truncate(msg.Content, 50))
			if r.Logger != nil {
				r.Logger.LogAgent(agentDef.ID, "MESSAGE_RECEIVED", fmt.Sprintf("From: %s", msg.From))
			}
		}

		// 2. Build prompt with message context
		prompt := r.buildCollaborativePrompt(agentDef, allReceivedMessages, conversation, turn)

		// 3. Generate response
		fmt.Printf("[%s] 💭 Turn %d/%d - Generating response...\n", agentDef.ID, turn+1, maxTurns)
		startTime := time.Now()
		response, err := client.Generate(prompt)
		elapsed := time.Since(startTime)

		if err != nil {
			return "", fmt.Errorf("[%s] turn %d failed: %w", agentDef.ID, turn+1, err)
		}

		fmt.Printf("[%s] ✓ Response generated in %.1fs (%d chars)\n", agentDef.ID, elapsed.Seconds(), len(response))
		conversation = append(conversation, response)

		// Log to file if logger available
		if r.Logger != nil {
			r.Logger.LogAgentOutput(agentDef.ID, fmt.Sprintf("Turn %d", turn+1), response)
		}

		// 4. Parse and send outgoing messages
		outgoing := ParseOutgoingMessages(response)
		for _, msg := range outgoing {
			// Respect canBroadcast setting
			if msg.To == "*" && !agentDef.CanBroadcast {
				fmt.Printf("[%s] ⚠️ Broadcast skipped (can_broadcast=false)\n", agentDef.ID)
				continue
			}

			err := channel.Send(agentDef.ID, msg.To, msg.Content)
			if err != nil {
				// Channel closed, agent should stop
				break
			}
			fmt.Printf("[%s] 📤 Sent to %s: %s\n", agentDef.ID, msg.To, truncate(msg.Content, 50))
			if r.Logger != nil {
				r.Logger.LogAgent(agentDef.ID, "MESSAGE_SENT", fmt.Sprintf("To: %s", msg.To))
			}
		}

		// 5. Check for DONE signal
		if ContainsDoneSignal(response) {
			fmt.Printf("[%s] ✅ Agent signaled DONE\n", agentDef.ID)
			if r.Logger != nil {
				r.Logger.LogAgent(agentDef.ID, "COLLABORATIVE_DONE", fmt.Sprintf("Turn: %d", turn+1))
			}
			break
		}

		// Small delay to allow other agents to process
		time.Sleep(50 * time.Millisecond)
	}

	// Extract and return final output
	finalOutput := ExtractFinalOutput(conversation)

	// Publish to shared memory if outputs defined
	if r.SharedMemory != nil && len(agentDef.Outputs) > 0 {
		for _, key := range agentDef.Outputs {
			r.SharedMemory.Set(key, finalOutput)
			fmt.Printf("[%s] 📤 Published '%s' to shared memory\n", agentDef.ID, key)
			if r.Logger != nil {
				r.Logger.LogAgent(agentDef.ID, "SHARED_MEMORY_PUBLISH", key)
			}
		}
	}

	return finalOutput, nil
}

// collectMessages gathers messages from the inbox channel with a timeout.
// It filters messages to only include those from agents in listenTo list (if specified).
func (r *Runner) collectMessages(inbox <-chan memory.ChannelMessage, listenTo []string) []memory.ChannelMessage {
	var messages []memory.ChannelMessage
	deadline := time.After(MessageCollectWindow)

	for {
		select {
		case msg, ok := <-inbox:
			if !ok {
				// Channel closed
				return messages
			}
			// Filter by listenTo if specified
			if len(listenTo) == 0 || containsString(listenTo, msg.From) {
				messages = append(messages, msg)
			}
		case <-deadline:
			return messages
		default:
			// No more messages available immediately
			if len(messages) > 0 {
				return messages
			}
			// Wait a bit before checking timeout
			time.Sleep(MessagePollTimeout)
		}
	}
}

// buildCollaborativePrompt constructs the prompt for a collaborative agent turn.
func (r *Runner) buildCollaborativePrompt(
	agentDef *types.Agent,
	receivedMessages []memory.ChannelMessage,
	conversation []string,
	turn int,
) string {
	var prompt string

	// Base prompt from agent definition
	prompt = agentDef.GetPrompt()

	// Add collaborative instructions
	prompt += fmt.Sprintf(`

## Collaborative Mode Instructions

You are in a collaborative workflow with other agents. You can communicate using these XML tags:

1. Send a message to a specific agent:
   <message to="agent_id">Your message here</message>

2. Broadcast to all agents:
   <broadcast>Your message here</broadcast>

3. Signal that you're done:
   <DONE/>

This is turn %d. Communicate with other agents as needed, then provide your analysis.
`, turn+1)

	// Add received messages context
	if len(receivedMessages) > 0 {
		prompt += "\n## Messages from Other Agents:\n"
		for _, msg := range receivedMessages {
			prompt += fmt.Sprintf("\n[From %s]:\n%s\n", msg.From, msg.Content)
		}
	}

	// Add conversation history (previous turns)
	if len(conversation) > 0 {
		prompt += "\n## Your Previous Responses:\n"
		for i, resp := range conversation {
			// Only include stripped content to avoid confusion
			stripped := StripMessageTags(resp)
			if stripped != "" {
				prompt += fmt.Sprintf("\n[Turn %d]:\n%s\n", i+1, truncate(stripped, 500))
			}
		}
	}

	// Add any required context from shared memory
	if r.SharedMemory != nil && len(agentDef.Requires) > 0 {
		for _, key := range agentDef.Requires {
			if val, ok := r.SharedMemory.Get(key); ok {
				prompt += fmt.Sprintf("\n## Context - %s:\n%v\n", key, val)
			}
		}
	}

	return prompt
}

// containsString checks if a slice contains a string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// truncate shortens a string to maxLen characters, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
