package memory

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	MaxSessions    = 50
	ExpiryDays     = 30
	SessionsFolder = ".orka/sessions"
)

type Message struct {
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	ID        string    `json:"id"`
	Workflow  string    `json:"workflow"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

// GetSessionsDir returns the path to sessions directory
func GetSessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, SessionsFolder)
}

// GenerateID creates a short unique session ID
func GenerateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewSession creates a new session
func NewSession(workflow string) *Session {
	return &Session{
		ID:        GenerateID(),
		Workflow:  workflow,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
	}
}

// AddMessage appends a message to the session
func (s *Session) AddMessage(agentID, role, content string) {
	s.Messages = append(s.Messages, Message{
		AgentID:   agentID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// GetHistory returns formatted history for context
func (s *Session) GetHistory() string {
	if len(s.Messages) == 0 {
		return ""
	}

	var result string
	result = "=== Previous Session Context ===\n\n"
	for _, msg := range s.Messages {
		result += fmt.Sprintf("[%s] %s:\n%s\n\n", msg.AgentID, msg.Role, msg.Content)
	}
	return result
}

// Save persists the session to disk
func (s *Session) Save() error {
	dir := GetSessionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, s.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// LoadSession loads a session by ID
func LoadSession(id string) (*Session, error) {
	path := filepath.Join(GetSessionsDir(), id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListSessions returns all session IDs sorted by update time
func ListSessions() ([]Session, error) {
	dir := GetSessionsDir()
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}

		id := f.Name()[:len(f.Name())-5]
		session, err := LoadSession(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, *session)
	}

	// Sort by updated_at descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// GetLatestSession returns the most recently updated session
func GetLatestSession() (*Session, error) {
	sessions, err := ListSessions()
	if err != nil || len(sessions) == 0 {
		return nil, err
	}
	return &sessions[0], nil
}

// CleanupOldSessions removes expired and excess sessions
func CleanupOldSessions() error {
	sessions, err := ListSessions()
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -ExpiryDays)
	dir := GetSessionsDir()

	for i, s := range sessions {
		shouldDelete := false

		// Delete if expired
		if s.UpdatedAt.Before(cutoff) {
			shouldDelete = true
		}

		// Delete if over max limit (keep newest)
		if i >= MaxSessions {
			shouldDelete = true
		}

		if shouldDelete {
			path := filepath.Join(dir, s.ID+".json")
			os.Remove(path)
		}
	}

	return nil
}

// DeleteSession removes a session by ID
func DeleteSession(id string) error {
	path := filepath.Join(GetSessionsDir(), id+".json")
	return os.Remove(path)
}
