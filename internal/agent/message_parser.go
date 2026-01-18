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

// StripMessageTags removes all message/broadcast/DONE tags from a response,
// returning the "clean" content for final output
func StripMessageTags(response string) string {
	result := messagePattern.ReplaceAllString(response, "")
	result = broadcastPattern.ReplaceAllString(result, "")
	result = donePattern.ReplaceAllString(result, "")
	// Clean up extra whitespace
	result = strings.TrimSpace(result)
	return result
}

// ExtractFinalOutput extracts the final output from a collaborative conversation.
// It looks for content after the last message tag, or the stripped response if no tags.
func ExtractFinalOutput(conversation []string) string {
	if len(conversation) == 0 {
		return ""
	}

	// Use the last response as the final output
	lastResponse := conversation[len(conversation)-1]
	return StripMessageTags(lastResponse)
}
