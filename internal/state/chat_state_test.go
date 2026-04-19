package state

import (
	"os"
	"testing"
	"time"

	"github.com/studyforge/study-agent/internal/config"
)

func TestLatestChatSession_RoundtripAndClear(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	})
	_ = os.Setenv("HOME", tmp)
	_ = os.Setenv("USERPROFILE", tmp)

	if _, err := config.EnsureInitialized(); err != nil {
		t.Fatalf("ensure initialized: %v", err)
	}

	in := &ChatSession{
		Class: "math101",
		Mode:  "socratic",
		Messages: []ChatMessage{
			{Role: "user", Content: "Explain chain rule"},
			{Role: "assistant", Content: "Use derivative of outer times inner."},
		},
		UpdatedAt: time.Now().UTC().Round(time.Second),
	}

	if err := SaveLatestChatSession(in); err != nil {
		t.Fatalf("save latest chat: %v", err)
	}

	out, err := LoadLatestChatSession()
	if err != nil {
		t.Fatalf("load latest chat: %v", err)
	}
	if out == nil {
		t.Fatal("expected persisted chat session, got nil")
	}
	if out.Class != in.Class || out.Mode != in.Mode {
		t.Fatalf("expected class/mode %q/%q, got %q/%q", in.Class, in.Mode, out.Class, out.Mode)
	}
	if len(out.Messages) != 2 || out.Messages[0].Role != "user" || out.Messages[1].Role != "assistant" {
		t.Fatalf("unexpected messages: %#v", out.Messages)
	}

	if err := ClearLatestChatSession(); err != nil {
		t.Fatalf("clear latest chat: %v", err)
	}
	out, err = LoadLatestChatSession()
	if err != nil {
		t.Fatalf("load latest chat after clear: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil after clear, got %#v", out)
	}
}
