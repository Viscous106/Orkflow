package memory

import (
	"fmt"
	"sync"
	"time"
)

// SharedMemory is a thread-safe key-value store for inter-agent communication
// within a workflow session. Agents can publish data under keys and subscribe
// to data from other agents.
type SharedMemory struct {
	mu        sync.RWMutex
	data      map[string]interface{}
	sessionID string
	cond      *sync.Cond
}

// NewSharedMemory creates a new SharedMemory instance for a session
func NewSharedMemory(sessionID string) *SharedMemory {
	sm := &SharedMemory{
		data:      make(map[string]interface{}),
		sessionID: sessionID,
	}
	sm.cond = sync.NewCond(&sm.mu)
	return sm
}

// Set stores a value under a key. This is thread-safe and will notify
// any goroutines waiting on this key.
func (sm *SharedMemory) Set(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.data[key] = value
	sm.cond.Broadcast() // Wake up all waiters
}

// Get retrieves a value by key. Returns the value and true if found,
// or nil and false if not found.
func (sm *SharedMemory) Get(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.data[key]
	return val, ok
}

// GetString retrieves a string value by key. Returns empty string if not found
// or if the value is not a string.
func (sm *SharedMemory) GetString(key string) string {
	val, ok := sm.Get(key)
	if !ok {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", val)
}

// WaitFor blocks until a key is available or timeout is reached.
// Returns the value and nil error if found, or nil and error if timeout.
func (sm *SharedMemory) WaitFor(key string, timeout time.Duration) (interface{}, error) {
	deadline := time.Now().Add(timeout)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for {
		// Check if key exists
		if val, ok := sm.data[key]; ok {
			return val, nil
		}

		// Check timeout
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timeout waiting for key '%s' after %v", key, timeout)
		}

		// Wait with timeout using a goroutine
		done := make(chan struct{})
		go func() {
			time.Sleep(remaining)
			sm.cond.Broadcast()
			close(done)
		}()

		// Wait for signal
		sm.cond.Wait()

		// Clean up timer goroutine by checking if done was already closed
		select {
		case <-done:
			// Timer fired, will check again and likely timeout
		default:
			// Signal came from Set(), continue to check
		}
	}
}

// Keys returns all keys currently in shared memory
func (sm *SharedMemory) Keys() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	keys := make([]string, 0, len(sm.data))
	for k := range sm.data {
		keys = append(keys, k)
	}
	return keys
}

// Clear removes all data from shared memory
func (sm *SharedMemory) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.data = make(map[string]interface{})
}

// GetSessionID returns the session ID this shared memory belongs to
func (sm *SharedMemory) GetSessionID() string {
	return sm.sessionID
}

// Snapshot returns a copy of all data for debugging or persistence
func (sm *SharedMemory) Snapshot() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snapshot := make(map[string]interface{}, len(sm.data))
	for k, v := range sm.data {
		snapshot[k] = v
	}
	return snapshot
}
