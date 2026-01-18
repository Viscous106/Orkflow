package agent

import (
	"regexp"
	"strings"
)

// OutgoingMessage represents a message to be sent to another agent
type OutgoingMessage struct {
	To      string // Target agent ID or "*" for broadcast
	Content string // Message content
}

var (
	// Regex patterns for parsing message tags from LLM responses
	messagePattern   = regexp.MustCompile(`(?s)<message\s+to="([^"]+)">(.*?)</message>`)
	broadcastPattern = regexp.MustCompile(`(?s)<broadcast>(.*?)</broadcast>`)
	donePattern      = regexp.MustCompile(`<DONE\s*/>`)
)

// ParseOutgoingMessages extracts messages from an LLM response.
// Supports:
//   - <message to="agent_id">content</message> - Direct message
//   - <broadcast>content</broadcast> - Broadcast to all agents
//   - <DONE/> - Signal that agent is finished
func ParseOutgoingMessages(response string) []OutgoingMessage {
	var messages []OutgoingMessage

	// Find direct messages
	matches := messagePattern.FindAllStringSubmatch(response, -1)
	for _, match := range matches {
		if len(match) == 3 {
			messages = append(messages, OutgoingMessage{
				To:      strings.TrimSpace(match[1]),
				Content: strings.TrimSpace(match[2]),
			})
		}
	}

	// Find broadcasts
	broadcastMatches := broadcastPattern.FindAllStringSubmatch(response, -1)
	for _, match := range broadcastMatches {
		if len(match) == 2 {
			messages = append(messages, OutgoingMessage{
				To:      "*",
				Content: strings.TrimSpace(match[1]),
			})
		}
	}

	return messages
}

// ContainsDoneSignal checks if the response contains a <DONE/> signal
func ContainsDoneSignal(response string) bool {
	return donePattern.MatchString(response)
}

// StripMessageTags replaces message tags with a readable text format
// e.g. <message to="bob">hi</message> -> [To bob]: hi
func StripMessageTags(response string) string {
	// Replace direct messages
	result := messagePattern.ReplaceAllString(response, "[To $1]: $2")
	// Replace broadcasts
	result = broadcastPattern.ReplaceAllString(result, "[Broadcast]: $1")
	// Remove DONE signal
	result = donePattern.ReplaceAllString(result, "")
	// Clean up extra whitespace
	result = strings.TrimSpace(result)
	return result
}

// ExtractFinalOutput collects the entire conversation history as the output.
// It converts message tags to readable text so downstream agents can understand the context.
func ExtractFinalOutput(conversation []string) string {
	if len(conversation) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, resp := range conversation {
		cleaned := StripMessageTags(resp)
		if cleaned != "" {
			sb.WriteString(cleaned + "\n\n")
		}
	}
	return strings.TrimSpace(sb.String())
}
