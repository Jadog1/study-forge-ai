// Package sfq wraps sfq quiz commands so the app can run them from one place.
package sfq

import (
	"fmt"
	"os/exec"
	"strings"
)

// Generate runs `sfq generate <quizPath>` to produce and open an interactive
// HTML quiz in the default browser. The sfq binary is always invoked directly.
func Generate(quizPath string) error {
	cmd := exec.Command("sfq", "generate", quizPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sfq generate failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Track starts `sfq track <quizPath>` to run a tracked local quiz session.
// Tracked mode persists answers and score so history/results can be queried.
func Track(quizPath string) error {
	cmd := exec.Command("sfq", "track", quizPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sfq track failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Schema returns the quiz YAML schema for use as AI formatting guidance.
// It tries the configured sfq binary's "schema" sub-command first; if the
// command is empty or the binary is unavailable, the built-in schema is
// returned instead.
func Schema(command string) string {
	if parts := strings.Fields(strings.TrimSpace(command)); len(parts) > 0 {
		args := append(parts[1:], "schema")
		cmd := exec.Command(parts[0], args...)
		out, err := cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return string(out)
		}
	}
	return builtInSchema
}

// builtInSchema is the canonical quiz YAML format the AI must follow.
const builtInSchema = `Quiz YAML schema — respond with ONLY valid YAML, no markdown code fences, no extra text:

title: "<descriptive quiz title>"
class: "<class name>"
tags:
  - "<topic tag>"
sections:
  - type: "question"
    id: "q-001"
    question: "<question text>"
    hint: "<helpful nudge without giving away the answer>"
    answer: "<clear, complete answer>"
    reasoning: "<explanation of why this is correct>"
    section_id: "<source section id if known, e.g. sec-abc12345 — omit if unknown>"
    component_id: "<source component id if known, e.g. cmp-abc12345 — omit if unknown>"
    tags:
      - "<tag>"
			- "src_section:<section-id>"
			- "src_component:<component-id>"
  - type: "question"
    id: "q-002"
    question: "<next question>"
    hint: "<hint>"
    answer: "<answer>"
    reasoning: "<reasoning>"
    section_id: "<source section id or omit>"
    component_id: "<source component id or omit>"
		tags:
			- "<tag>"
			- "src_section:<section-id when known>"
			- "src_component:<component-id when known>"

Rules:
- All non-optional string values must be non-empty.
- section id values must be unique and sequential: q-001, q-002, q-003, etc.
- sections must contain at least one entry.
- section_id and component_id are optional; include them only when the study material identifies the source knowledge section or component.
- When section_id/component_id are present, include matching tags with prefixes src_section: and src_component:.
- Do not add any fields beyond those listed above.
- Respond with ONLY the YAML document, nothing else.`
