// Package sfq wraps sfq plugin search so the TUI can run it from one place.
package sfq

import (
	"fmt"
	"os/exec"
	"strings"
)

// Search runs the configured sfq command with search arguments.
// Example command value: "studyforge sfq".
func Search(command, query string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return "", fmt.Errorf("sfq command is empty")
	}

	args := append(parts[1:], "search", query)
	cmd := exec.Command(parts[0], args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sfq search failed: %w\n%s", err, string(out))
	}
	return string(out), nil
}

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
    tags:
      - "<tag>"
  - type: "question"
    id: "q-002"
    question: "<next question>"
    hint: "<hint>"
    answer: "<answer>"
    reasoning: "<reasoning>"
    tags:
      - "<tag>"

Rules:
- All string values must be non-empty.
- section id values must be unique and sequential: q-001, q-002, q-003, etc.
- sections must contain at least one entry.
- Do not add any fields beyond those listed above.
- Respond with ONLY the YAML document, nothing else.`
