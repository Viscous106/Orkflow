package memory

import (
	"sync"
	"testing"
	"time"
)

func TestNewMessageChannel(t *testing.T) {
	mc := NewMessageChannel(10)
	if mc == nil {
		t.Fatal("NewMessageChannel returned nil")
	}
	if mc.bufferSize != 10 {
		t.Errorf("expected bufferSize 10, got %d", mc.bufferSize)
	}
	if mc.closed {
		t.Error("channel should not be closed initially")
	}
}

func TestNewMessageChannelDefaultBuffer(t *testing.T) {
	mc := NewMessageChannel(0)
	if mc.bufferSize != 100 {
		t.Errorf("expected default bufferSize 100, got %d", mc.bufferSize)
	}
}

func TestSubscribeAndSend(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	inbox := mc.Subscribe("agent1")

	err := mc.Send("agent2", "agent1", "hello")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case msg := <-inbox:
		if msg.From != "agent2" {
			t.Errorf("expected From 'agent2', got '%s'", msg.From)
		}
		if msg.To != "agent1" {
			t.Errorf("expected To 'agent1', got '%s'", msg.To)
		}
		if msg.Content != "hello" {
			t.Errorf("expected Content 'hello', got '%s'", msg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestBroadcast(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	inbox1 := mc.Subscribe("agent1")
	inbox2 := mc.Subscribe("agent2")
	inbox3 := mc.Subscribe("agent3")

	// agent1 broadcasts
	err := mc.Send("agent1", "*", "broadcast message")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// agent1 should NOT receive their own broadcast
	select {
	case <-inbox1:
		t.Error("sender should not receive their own broadcast")
	case <-time.After(100 * time.Millisecond):
		// Good, no message for sender
	}

	// agent2 and agent3 should receive the broadcast
	for _, inbox := range []<-chan ChannelMessage{inbox2, inbox3} {
		select {
		case msg := <-inbox:
			if msg.Content != "broadcast message" {
				t.Errorf("expected 'broadcast message', got '%s'", msg.Content)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for broadcast")
		}
	}
}

func TestGetHistory(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	mc.Subscribe("agent1")
	mc.Subscribe("agent2")

	mc.Send("agent1", "agent2", "msg1")
	mc.Send("agent2", "agent1", "msg2")
	mc.Send("agent1", "*", "msg3")

	history := mc.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 messages in history, got %d", len(history))
	}

	if history[0].Content != "msg1" || history[1].Content != "msg2" || history[2].Content != "msg3" {
		t.Error("history messages in wrong order")
	}
}

func TestGetMessagesFor(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	mc.Subscribe("agent1")
	mc.Subscribe("agent2")

	mc.Send("agent1", "agent2", "direct")
	mc.Send("agent1", "*", "broadcast")
	mc.Send("agent2", "agent1", "other")

	msgs := mc.GetMessagesFor("agent2")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages for agent2, got %d", len(msgs))
	}
}

func TestGetMessagesFrom(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	mc.Subscribe("agent1")
	mc.Subscribe("agent2")

	mc.Send("agent1", "agent2", "msg1")
	mc.Send("agent1", "*", "msg2")
	mc.Send("agent2", "agent1", "other")

	msgs := mc.GetMessagesFrom("agent1")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages from agent1, got %d", len(msgs))
	}
}

func TestClose(t *testing.T) {
	mc := NewMessageChannel(10)
	inbox := mc.Subscribe("agent1")

	mc.Close()

	if !mc.IsClosed() {
		t.Error("channel should be closed")
	}

	// Sending after close should return error
	err := mc.Send("agent2", "agent1", "hello")
	if err != ErrChannelClosed {
		t.Errorf("expected ErrChannelClosed, got %v", err)
	}

	// Inbox should be closed
	select {
	case _, ok := <-inbox:
		if ok {
			t.Error("expected inbox to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("inbox should be closed immediately")
	}
}

func TestUnsubscribe(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	inbox := mc.Subscribe("agent1")
	mc.Unsubscribe("agent1")

	// Inbox should be closed
	select {
	case _, ok := <-inbox:
		if ok {
			t.Error("expected inbox to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("inbox should be closed immediately")
	}

	// Should not receive messages after unsubscribe
	mc.Send("agent2", "agent1", "hello")
	// No subscriber, message just goes to history
	if mc.Count() != 1 {
		t.Error("message should still be in history")
	}
}

func TestConcurrentSendReceive(t *testing.T) {
	mc := NewMessageChannel(100)
	defer mc.Close()

	numAgents := 5
	numMessages := 10

	var wg sync.WaitGroup
	received := make(map[string]int)
	var mu sync.Mutex

	// Subscribe all agents
	inboxes := make(map[string]<-chan ChannelMessage)
	for i := 0; i < numAgents; i++ {
		agentID := string(rune('A' + i))
		inboxes[agentID] = mc.Subscribe(agentID)
	}

	// Start receivers
	for agentID, inbox := range inboxes {
		wg.Add(1)
		go func(id string, ch <-chan ChannelMessage) {
			defer wg.Done()
			count := 0
			// Read messages until we've received expected count or timeout
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
					count++
				case <-time.After(500 * time.Millisecond):
					mu.Lock()
					received[id] = count
					mu.Unlock()
					return
				}
			}
		}(agentID, inbox)
	}

	// Start senders
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		agentID := string(rune('A' + i))
		go func(id string) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				mc.Send(id, "*", "msg")
			}
		}(agentID)
	}

	wg.Wait()

	// Verify message count
	if mc.Count() != numAgents*numMessages {
		t.Errorf("expected %d messages, got %d", numAgents*numMessages, mc.Count())
	}
}

func TestCount(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	if mc.Count() != 0 {
		t.Error("initial count should be 0")
	}

	mc.Send("a", "b", "msg1")
	mc.Send("a", "b", "msg2")

	if mc.Count() != 2 {
		t.Errorf("expected count 2, got %d", mc.Count())
	}
}

func TestSubscriberCount(t *testing.T) {
	mc := NewMessageChannel(10)
	defer mc.Close()

	if mc.SubscriberCount() != 0 {
		t.Error("initial subscriber count should be 0")
	}

	mc.Subscribe("agent1")
	mc.Subscribe("agent2")

	if mc.SubscriberCount() != 2 {
		t.Errorf("expected subscriber count 2, got %d", mc.SubscriberCount())
	}

	mc.Unsubscribe("agent1")

	if mc.SubscriberCount() != 1 {
		t.Errorf("expected subscriber count 1, got %d", mc.SubscriberCount())
	}
}
