// Package class manages class directories, syllabi, and rule files under
// ~/.study-forge-ai/classes/<name>/.
package class

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/studyforge/study-agent/internal/config"
	"gopkg.in/yaml.v3"
)

// Syllabus lists the weekly/daily topics for a class.
type Syllabus struct {
	Class  string  `yaml:"class"`
	Topics []Topic `yaml:"topics"`
}

// Topic is a single entry in the syllabus.
type Topic struct {
	Week        int      `yaml:"week,omitempty"`
	Day         string   `yaml:"day,omitempty"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// Rules captures exam expectations and question style preferences.
type Rules struct {
	Class          string   `yaml:"class"`
	ExamExpect     string   `yaml:"exam_expectations,omitempty"`
	QuestionStyles []string `yaml:"question_styles,omitempty"`
	Notes          string   `yaml:"notes,omitempty"`
}

// Context tracks external file paths used as class-level AI context.
type Context struct {
	Class        string   `yaml:"class"`
	ContextFiles []string `yaml:"context_files"`
}

func classDir(name string) (string, error) {
	return config.Path("classes", name)
}

func classFilePath(name, fileName string) (string, error) {
	dir, err := classDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Create scaffolds a new class directory with default syllabus and rules files.
func Create(name string) error {
	dir, err := classDir(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create class dir: %w", err)
	}

	syllabus := Syllabus{
		Class: name,
		Topics: []Topic{
			{Week: 1, Title: "Introduction", Description: "Edit this topic."},
		},
	}
	if err := writeYAML(filepath.Join(dir, "syllabus.yaml"), syllabus); err != nil {
		return fmt.Errorf("write syllabus: %w", err)
	}

	rules := Rules{
		Class:          name,
		ExamExpect:     "Conceptual understanding with worked examples",
		QuestionStyles: []string{"open-ended", "multiple-choice"},
	}
	if err := writeYAML(filepath.Join(dir, "rules.yaml"), rules); err != nil {
		return fmt.Errorf("write rules: %w", err)
	}

	ctx := Context{Class: name, ContextFiles: []string{}}
	if err := writeYAML(filepath.Join(dir, "context.yaml"), ctx); err != nil {
		return fmt.Errorf("write context: %w", err)
	}

	return nil
}

// LoadSyllabus reads the syllabus for the named class.
func LoadSyllabus(name string) (*Syllabus, error) {
	var s Syllabus
	path, err := classFilePath(name, "syllabus.yaml")
	if err != nil {
		return nil, err
	}
	if err := readYAML(path, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// LoadRules reads the rules for the named class.
func LoadRules(name string) (*Rules, error) {
	var r Rules
	path, err := classFilePath(name, "rules.yaml")
	if err != nil {
		return nil, err
	}
	if err := readYAML(path, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// List returns the names of all classes that have been created.
func List() ([]string, error) {
	dir, err := config.Path("classes")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list classes: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// LoadContext reads class context.yaml. If missing, returns an empty context.
func LoadContext(name string) (*Context, error) {
	path, err := classFilePath(name, "context.yaml")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Context{Class: name, ContextFiles: []string{}}, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var c Context
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if c.Class == "" {
		c.Class = name
	}
	return &c, nil
}

// SaveContext writes class context.yaml.
func SaveContext(name string, c *Context) error {
	if c == nil {
		return fmt.Errorf("context is nil")
	}
	if c.Class == "" {
		c.Class = name
	}
	path, err := classFilePath(name, "context.yaml")
	if err != nil {
		return err
	}
	return writeYAML(path, c)
}

// AddContextFile appends a file path to class context if not present.
func AddContextFile(name, filePath string) error {
	c, err := LoadContext(name)
	if err != nil {
		return err
	}
	for _, p := range c.ContextFiles {
		if p == filePath {
			return nil
		}
	}
	c.ContextFiles = append(c.ContextFiles, filePath)
	return SaveContext(name, c)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeYAML(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readYAML(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	return yaml.Unmarshal(data, v)
}
