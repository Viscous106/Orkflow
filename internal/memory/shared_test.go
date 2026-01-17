package memory

import (
	"sync"
	"testing"
	"time"
)

func TestSharedMemory_SetAndGet(t *testing.T) {
	sm := NewSharedMemory("test-session")

	// Test basic set and get
	sm.Set("key1", "value1")
	val, ok := sm.Get("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val)
	}

	// Test non-existent key
	_, ok = sm.Get("nonexistent")
	if ok {
		t.Error("Expected nonexistent key to not exist")
	}
}

func TestSharedMemory_GetString(t *testing.T) {
	sm := NewSharedMemory("test-session")

	sm.Set("str", "hello")
	sm.Set("num", 42)

	if str := sm.GetString("str"); str != "hello" {
		t.Errorf("Expected 'hello', got '%s'", str)
	}

	// Non-string should be converted
	if str := sm.GetString("num"); str != "42" {
		t.Errorf("Expected '42', got '%s'", str)
	}

	// Non-existent should return empty string
	if str := sm.GetString("nope"); str != "" {
		t.Errorf("Expected empty string, got '%s'", str)
	}
}

func TestSharedMemory_Concurrent(t *testing.T) {
	sm := NewSharedMemory("test-session")
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sm.Set(string(rune('a'+i%26)), i)
		}(i)
	}

	wg.Wait()

	// Verify keys exist
	keys := sm.Keys()
	if len(keys) == 0 {
		t.Error("Expected some keys to be set")
	}
}

func TestSharedMemory_WaitFor_Success(t *testing.T) {
	sm := NewSharedMemory("test-session")

	// Set value after a delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		sm.Set("delayed", "arrived")
	}()

	// Wait for it
	val, err := sm.WaitFor("delayed", 2*time.Second)
	if err != nil {
		t.Errorf("WaitFor failed: %v", err)
	}
	if val != "arrived" {
		t.Errorf("Expected 'arrived', got '%v'", val)
	}
}

func TestSharedMemory_WaitFor_Timeout(t *testing.T) {
	sm := NewSharedMemory("test-session")

	// Try to wait for a key that never comes
	_, err := sm.WaitFor("never", 100*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestSharedMemory_WaitFor_AlreadyExists(t *testing.T) {
	sm := NewSharedMemory("test-session")

	// Set value first
	sm.Set("exists", "already")

	// WaitFor should return immediately
	start := time.Now()
	val, err := sm.WaitFor("exists", 1*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("WaitFor failed: %v", err)
	}
	if val != "already" {
		t.Errorf("Expected 'already', got '%v'", val)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("WaitFor took too long for existing key: %v", elapsed)
	}
}

func TestSharedMemory_Snapshot(t *testing.T) {
	sm := NewSharedMemory("test-session")

	sm.Set("a", 1)
	sm.Set("b", 2)

	snapshot := sm.Snapshot()

	if len(snapshot) != 2 {
		t.Errorf("Expected 2 items, got %d", len(snapshot))
	}

	// Verify modifying snapshot doesn't affect original
	snapshot["c"] = 3
	if _, ok := sm.Get("c"); ok {
		t.Error("Modifying snapshot should not affect original")
	}
}

func TestSharedMemory_Clear(t *testing.T) {
	sm := NewSharedMemory("test-session")

	sm.Set("a", 1)
	sm.Set("b", 2)
	sm.Clear()

	if len(sm.Keys()) != 0 {
		t.Error("Expected no keys after clear")
	}
}
