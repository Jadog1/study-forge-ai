package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

// ChatMessage persists one chat turn in the latest chat session.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatSession stores the latest chat conversation for restore-on-refresh UX.
type ChatSession struct {
	Class     string        `json:"class,omitempty"`
	Mode      string        `json:"mode,omitempty"`
	Messages  []ChatMessage `json:"messages"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func latestChatPath() (string, error) {
	return config.Path("chat", "latest.json")
}

// LoadLatestChatSession reads the latest chat session from disk.
// Returns nil when no persisted session exists yet.
func LoadLatestChatSession() (*ChatSession, error) {
	path, err := latestChatPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read latest chat session: %w", err)
	}
	var session ChatSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse latest chat session: %w", err)
	}
	return &session, nil
}

// SaveLatestChatSession writes the latest chat session to disk.
func SaveLatestChatSession(session *ChatSession) error {
	if session == nil {
		return fmt.Errorf("chat session is nil")
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = time.Now().UTC()
	}
	path, err := latestChatPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create chat dir: %w", err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal latest chat session: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ClearLatestChatSession removes the persisted latest chat session.
func ClearLatestChatSession() error {
	path, err := latestChatPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove latest chat session: %w", err)
	}
	return nil
}
