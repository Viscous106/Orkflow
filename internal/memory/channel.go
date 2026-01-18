package memory

import (
	"sync"
	"time"
)

// ChannelMessage represents a message between agents in a collaborative workflow.
// Named differently from Message (used for session persistence) to avoid conflicts.
type ChannelMessage struct {
	From      string    // Agent ID of sender
	To        string    // Target agent ID, or "*" for broadcast
	Content   string    // Message content
	Timestamp time.Time // When the message was sent
}

// MessageChannel is a pub/sub message channel for real-time inter-agent communication.
// It allows agents running in parallel to send and receive messages during execution.
type MessageChannel struct {
	mu          sync.RWMutex
	messages    []ChannelMessage                 // All messages (append-only log)
	subscribers map[string]chan ChannelMessage   // Agent ID -> their inbox channel
	bufferSize  int                              // Size of each subscriber's channel buffer
	closed      bool                             // Whether the channel has been closed
}

// NewMessageChannel creates a new message channel for collaborative workflows.
// bufferSize determines how many messages can be queued per subscriber before blocking.
func NewMessageChannel(bufferSize int) *MessageChannel {
	if bufferSize <= 0 {
		bufferSize = 100 // Default buffer size
	}
	return &MessageChannel{
		messages:    make([]ChannelMessage, 0),
		subscribers: make(map[string]chan ChannelMessage),
		bufferSize:  bufferSize,
		closed:      false,
	}
}

// Send sends a message from one agent to another (or to all if to == "*").
// Returns an error if the channel is closed.
func (mc *MessageChannel) Send(from, to, content string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return ErrChannelClosed
	}

	msg := ChannelMessage{
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Append to history
	mc.messages = append(mc.messages, msg)

	// Deliver to subscribers
	if to == "*" {
		// Broadcast to all except sender
		for agentID, inbox := range mc.subscribers {
			if agentID != from {
				select {
				case inbox <- msg:
				default:
					// Channel full, skip (non-blocking)
				}
			}
		}
	} else {
		// Direct message to specific agent
		if inbox, ok := mc.subscribers[to]; ok {
			select {
			case inbox <- msg:
			default:
				// Channel full, skip (non-blocking)
			}
		}
	}

	return nil
}

// Subscribe creates an inbox channel for an agent to receive messages.
// The agent should read from this channel in a loop.
func (mc *MessageChannel) Subscribe(agentID string) <-chan ChannelMessage {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// If already subscribed, return existing channel
	if existing, ok := mc.subscribers[agentID]; ok {
		return existing
	}

	inbox := make(chan ChannelMessage, mc.bufferSize)
	mc.subscribers[agentID] = inbox
	return inbox
}

// Unsubscribe removes an agent's subscription and closes their inbox.
func (mc *MessageChannel) Unsubscribe(agentID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if inbox, ok := mc.subscribers[agentID]; ok {
		close(inbox)
		delete(mc.subscribers, agentID)
	}
}

// GetHistory returns all messages sent through the channel.
// This is useful for building context or debugging.
func (mc *MessageChannel) GetHistory() []ChannelMessage {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Return a copy to avoid race conditions
	history := make([]ChannelMessage, len(mc.messages))
	copy(history, mc.messages)
	return history
}

// GetMessagesFor returns all messages addressed to a specific agent (including broadcasts).
func (mc *MessageChannel) GetMessagesFor(agentID string) []ChannelMessage {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result []ChannelMessage
	for _, msg := range mc.messages {
		if msg.To == agentID || msg.To == "*" {
			result = append(result, msg)
		}
	}
	return result
}

// GetMessagesFrom returns all messages sent by a specific agent.
func (mc *MessageChannel) GetMessagesFrom(agentID string) []ChannelMessage {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result []ChannelMessage
	for _, msg := range mc.messages {
		if msg.From == agentID {
			result = append(result, msg)
		}
	}
	return result
}

// Close signals all agents to stop and closes all subscriber channels.
func (mc *MessageChannel) Close() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return
	}

	mc.closed = true

	// Close all subscriber inboxes
	for _, inbox := range mc.subscribers {
		close(inbox)
	}
	mc.subscribers = make(map[string]chan ChannelMessage)
}

// IsClosed returns whether the channel has been closed.
func (mc *MessageChannel) IsClosed() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.closed
}

// Count returns the total number of messages sent.
func (mc *MessageChannel) Count() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.messages)
}

// SubscriberCount returns the number of active subscribers.
func (mc *MessageChannel) SubscriberCount() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.subscribers)
}
