package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatMessage struct {
	Role    string // "user", "assistant", "system", "action"
	Content string
	// Fields only populated for Role == "action":
	ActionLabel   string
	ActionRunning bool
	ActionFailed  bool
}

// ChatTab holds all state for the Chat tab.
type ChatTab struct {
	messages     []ChatMessage
	input        textinput.Model
	scrollOffset int // messages from the bottom to scroll up by
	autoSFQ      bool
	sfqResult    string // most recent automatic SFQ lookup result
}

func newChatTab() ChatTab {
	input := textinput.New()
	input.Placeholder = "Ask about your notes or classes…"
	input.Focus()
	input.CharLimit = 10000
	input.Width = 80
	return ChatTab{input: input}
}

func (c ChatTab) resize(width int) ChatTab {
	c.input.Width = clamp(width-4, 18, width)
	return c
}

// updateInput handles key input from the user.
// When the user presses Enter with a non-empty message, the message is returned
// as the second value so the parent can dispatch it to the AI provider.
// busy should be true while an AI response is in flight; Enter is suppressed.
func (c ChatTab) updateInput(msg tea.Msg, busy bool) (ChatTab, string, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "enter":
			if !busy {
				prompt := strings.TrimSpace(c.input.Value())
				if prompt != "" {
					c.messages = append(c.messages,
						ChatMessage{Role: "user", Content: prompt},
						ChatMessage{Role: "assistant", Content: ""},
					)
					c.input.SetValue("")
					c.sfqResult = ""
					c.scrollOffset = 0
					return c, prompt, nil
				}
			}
		case "up", "pgup":
			step := 1
			if k.String() == "pgup" {
				step = 4
			}
			c.scrollOffset = min(c.scrollOffset+step, max(0, len(c.messages)-1))
			return c, "", nil
		case "down", "pgdn":
			step := 1
			if k.String() == "pgdn" {
				step = 4
			}
			c.scrollOffset = max(0, c.scrollOffset-step)
			return c, "", nil
		}
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return c, "", cmd
}

// appendAIChunk appends a streaming chunk to the last Agent log entry.
func (c ChatTab) appendAIChunk(part string) ChatTab {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == "assistant" {
			c.messages[i].Content += part
			return c
		}
	}
	c.messages = append(c.messages, ChatMessage{Role: "assistant", Content: part})
	return c
}

// addError appends a styled error message to the log.
func (c ChatTab) addError(errMsg string) ChatTab {
	c.messages = append(c.messages, ChatMessage{Role: "system", Content: "Error: " + errMsg})
	return c
}

// startAction records the start of an agent tool call inline in the conversation.
func (c ChatTab) startAction(label, detail string) ChatTab {
	c.messages = append(c.messages, ChatMessage{
		Role:          "action",
		Content:       detail,
		ActionLabel:   label,
		ActionRunning: true,
	})
	c.scrollOffset = 0 // keep latest visible
	return c
}

// finishAction updates the most recent matching running action with its result.
func (c ChatTab) finishAction(label, detail string, err error) ChatTab {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == "action" &&
			c.messages[i].ActionLabel == label &&
			c.messages[i].ActionRunning {
			c.messages[i].ActionRunning = false
			c.messages[i].Content = detail
			c.messages[i].ActionFailed = err != nil
			return c
		}
	}
	// Not found — append a completed action entry.
	c.messages = append(c.messages, ChatMessage{
		Role:         "action",
		Content:      detail,
		ActionLabel:  label,
		ActionFailed: err != nil,
	})
	return c
}

// setAutoSFQResult stores the result from a background SFQ lookup.
func (c ChatTab) setAutoSFQResult(text string) ChatTab {
	c.sfqResult = text
	return c
}

// lastUserPrompt returns the most recent "You: …" message text, stripped of the prefix.
func (c ChatTab) lastUserPrompt() string {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == "user" {
			return c.messages[i].Content
		}
	}
	return ""
}

// toggleAutoSFQ flips the autoSFQ flag.
func (c ChatTab) toggleAutoSFQ() ChatTab {
	c.autoSFQ = !c.autoSFQ
	return c
}

func (c ChatTab) view(width, height int, providerName string, providerDisabled bool, selectedClass string, busy bool) string {
	indicator := successStyle.Render("●")
	pName := providerName
	if providerDisabled {
		indicator = warnStyle.Render("⚠")
		pName += " (not configured)"
	}
	agentState := dimStyle.Render("idle")
	if busy {
		agentState = warnStyle.Render("busy…")
	}
	classDisplay := selectedClass
	if classDisplay == "" {
		classDisplay = dimStyle.Render("none")
	}
	autoLabel := dimStyle.Render("auto-sfq: off")
	if c.autoSFQ {
		autoLabel = successStyle.Render("auto-sfq: on")
	}

	metaBody := lipgloss.NewStyle().Width(width - 6).Render(fmt.Sprintf(
		"%s %s  %s\nClass: %s",
		indicator,
		pName,
		agentState,
		classDisplay,
	))
	metaBody += "\n" + autoLabel

	sfqHeight := 0
	if c.sfqResult != "" {
		sfqHeight = 8
	}
	conversationHeight := clamp(height-sfqHeight-20, 6, height-14)
	conversation := c.renderConversation(width, conversationHeight, busy)

	sections := []string{
		renderSection("Session", metaBody, width),
		renderSection("Conversation", conversation, width),
	}

	if c.sfqResult != "" {
		relatedBody := lipgloss.NewStyle().Width(width - 6).Render(truncate(strings.TrimSpace(c.sfqResult), 400))
		sections = append(sections, renderSection("Related notes (SFQ)", clipLines(relatedBody, 6), width))
	}

	// Render the input directly inside the section — no inner border (avoids double borders).
	inputContent := c.input.View() + "\n" +
		dimStyle.Render("Enter send  •  Ctrl+P actions  •  ↑/↓ scroll  •  Esc cancel")
	sections = append(sections, renderSection("Composer", inputContent, width))
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (c ChatTab) renderConversation(width, height int, busy bool) string {
	if len(c.messages) == 0 {
		empty := dimStyle.Render("No messages yet. Type below or press Ctrl+P for actions.")
		if busy {
			empty += "\n\n  " + warnStyle.Render("⟳") + " " + dimStyle.Render("Thinking…")
		}
		return empty
	}

	messages := c.messages
	// When busy and the last assistant slot is empty, temporarily fill it with a thinking indicator.
	if busy && c.lastAssistantEmpty() {
		temp := make([]ChatMessage, len(messages))
		copy(temp, messages)
		for i := len(temp) - 1; i >= 0; i-- {
			if temp[i].Role == "assistant" && strings.TrimSpace(temp[i].Content) == "" {
				temp[i].Content = dimStyle.Render("Thinking…")
				break
			}
		}
		messages = temp
	}

	total := len(messages)
	maxVisible := clamp(height/3+2, 3, 10)
	endIdx := total - c.scrollOffset
	if endIdx <= 0 {
		endIdx = 1
	}
	if endIdx > total {
		endIdx = total
	}
	startIdx := endIdx - maxVisible
	if startIdx < 0 {
		startIdx = 0
	}

	parts := make([]string, 0, maxVisible+2)
	if startIdx > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↑ %d more above  (PgUp)", startIdx)))
	}
	for _, msg := range messages[startIdx:endIdx] {
		parts = append(parts, renderChatMessage(msg, width))
	}
	if endIdx < total {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("  ↓ %d more below  (PgDn)", total-endIdx)))
	}
	return tailLines(strings.Join(parts, "\n"), height)
}

func (c ChatTab) lastAssistantEmpty() bool {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == "assistant" {
			return strings.TrimSpace(c.messages[i].Content) == ""
		}
	}
	return false
}

func renderChatMessage(message ChatMessage, width int) string {
	bubbleWidth := clamp((width*3)/5, 24, width-6)
	switch message.Role {
	case "user":
		label := dimStyle.Render("you")
		content := userBubbleStyle.Width(bubbleWidth).Render(message.Content)
		block := lipgloss.JoinVertical(lipgloss.Right, label, content)
		return lipgloss.PlaceHorizontal(width-2, lipgloss.Right, block)
	case "action":
		var icon, name string
		if message.ActionRunning {
			icon = warnStyle.Render("⟳")
			name = dimStyle.Render(titleCaseLabel(message.ActionLabel))
		} else if message.ActionFailed {
			icon = errorStyle.Render("✗")
			name = errorStyle.Render(titleCaseLabel(message.ActionLabel))
		} else {
			icon = successStyle.Render("✓")
			name = dimStyle.Render(titleCaseLabel(message.ActionLabel))
		}
		detail := ""
		if message.Content != "" {
			detail = "  " + dimStyle.Render(truncate(message.Content, 80))
		}
		return "  " + icon + " " + name + detail
	case "system":
		content := systemBubbleStyle.Width(bubbleWidth).Render(message.Content)
		return lipgloss.PlaceHorizontal(width-2, lipgloss.Center, content)
	default: // assistant
		label := dimStyle.Render("ai")
		content := assistantBubbleStyle.Width(bubbleWidth).Render(message.Content)
		return lipgloss.JoinVertical(lipgloss.Left, label, content)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func titleCaseLabel(label string) string {
	parts := strings.Fields(strings.ReplaceAll(label, "_", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
