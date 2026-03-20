// Package studyforge provides a thin wrapper around the studyforge CLI binary.
// The constraint is that HTML is NEVER generated directly by study-agent;
// it always delegates rendering to studyforge.
package studyforge

import (
	"fmt"
	"os/exec"
)

// Build invokes `studyforge build <quizPath>` to render the quiz YAML into
// an interactive HTML study guide. It streams combined stdout/stderr output.
func Build(quizPath string) error {
	cmd := exec.Command("studyforge", "build", quizPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("studyforge build failed: %w\nOutput:\n%s", err, string(out))
	}
	if len(out) > 0 {
		fmt.Printf("%s\n", out)
	}
	return nil
}
