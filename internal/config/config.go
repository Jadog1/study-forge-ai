// Package config loads and persists the application's configuration and
// storage paths.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AppDirName = ".study-forge-ai"
	ConfigFile = "config.yaml"
)

// InitResult describes the result of ensuring the app data directory exists.
type InitResult struct {
	RootDir    string
	ConfigPath string
	Created    bool
}

// Config is the top-level configuration structure.
type Config struct {
	// Provider selects the active AI backend: "openai", "claude", or "local".
	Provider   string           `yaml:"provider"`
	Embeddings EmbeddingsConfig `yaml:"embeddings"`
	OpenAI     OpenAIConfig     `yaml:"openai"`
	Claude     ClaudeConfig     `yaml:"claude"`
	Voyage     VoyageConfig     `yaml:"voyage"`
	Local      LocalConfig      `yaml:"local"`
	SFQ        SFQConfig        `yaml:"sfq"`
	// CustomPromptContext is appended verbatim to every AI prompt.
	// Use it to steer output style (e.g. "prefer real-world analogies").
	CustomPromptContext string `yaml:"custom_prompt_context,omitempty"`
	// ModelPrices stores per-million-token pricing overrides for AI models.
	// Keys are model name strings (e.g. "gpt-4o-mini").
	// Built-in prices for well-known models are provided automatically.
	ModelPrices map[string]ModelPrice `yaml:"model_prices,omitempty"`
}

// EmbeddingsConfig selects which provider/model to use for embeddings.
type EmbeddingsConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// OpenAIConfig holds credentials and model selection for OpenAI.
type OpenAIConfig struct {
	APIKey string `yaml:"-"`
	Model  string `yaml:"model"`
}

// ClaudeConfig holds credentials and model selection for Anthropic Claude.
type ClaudeConfig struct {
	APIKey string `yaml:"-"`
	Model  string `yaml:"model"`
}

// VoyageConfig holds credentials and model selection for VoyageAI.
type VoyageConfig struct {
	APIKey string `yaml:"-"`
	Model  string `yaml:"model"`
}

// LocalConfig points to a locally-running Ollama (or compatible) instance.
type LocalConfig struct {
	Endpoint           string `yaml:"endpoint"`
	EmbeddingsEndpoint string `yaml:"embeddings_endpoint,omitempty"`
	Model              string `yaml:"model"`
}

// SFQConfig controls how study-agent invokes the sfq plugin search command.
type SFQConfig struct {
	Command string `yaml:"command"`
}

// RootDir returns the hard-coded per-user application data directory.
func RootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, AppDirName), nil
}

// DisplayRootDir returns the user-facing version of the app directory path.
func DisplayRootDir() string {
	return filepath.ToSlash(filepath.Join("~", AppDirName))
}

// DisplayPath converts an absolute app data path into a friendly ~/ path.
func DisplayPath(path string) string {
	root, err := RootDir()
	if err != nil {
		return filepath.ToSlash(path)
	}

	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanRoot {
		return DisplayRootDir()
	}
	if strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
		suffix := strings.TrimPrefix(cleanPath, cleanRoot)
		suffix = strings.TrimLeft(suffix, string(filepath.Separator)+"/")
		return filepath.ToSlash(filepath.Join("~", AppDirName, suffix))
	}

	return filepath.ToSlash(path)
}

// Path resolves a path inside the per-user application directory.
func Path(parts ...string) (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	elems := append([]string{root}, parts...)
	return filepath.Join(elems...), nil
}

// ConfigPath returns the absolute path to config.yaml.
func ConfigPath() (string, error) {
	return Path(ConfigFile)
}

// EnsureInitialized creates the app data directories and default config file.
func EnsureInitialized() (*InitResult, error) {
	root, err := RootDir()
	if err != nil {
		return nil, err
	}

	dirs := []string{
		filepath.Join(root, "notes", "raw"),
		filepath.Join(root, "notes", "processed"),
		filepath.Join(root, "classes"),
		filepath.Join(root, "quizzes"),
		filepath.Join(root, "plans"),
		filepath.Join(root, "cache"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	configPath := filepath.Join(root, ConfigFile)
	created := false
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		data, marshalErr := yaml.Marshal(DefaultConfig())
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal default config: %w", marshalErr)
		}
		if writeErr := os.WriteFile(configPath, data, 0600); writeErr != nil {
			return nil, fmt.Errorf("write config: %w", writeErr)
		}
		created = true
	} else if err != nil {
		return nil, fmt.Errorf("stat config: %w", err)
	}

	return &InitResult{RootDir: root, ConfigPath: configPath, Created: created}, nil
}

// EnvOpenAIKey and EnvClaudeKey are the environment variable names used to
// supply API keys at runtime. Keys are never read from or written to disk.
const (
	EnvOpenAIKey = "OPENAI_API_KEY_SFA"
	EnvClaudeKey = "ANTHROPIC_API_KEY_SFA"
	EnvVoyageKey = "VOYAGE_API_KEY_SFA"
)

// Load reads config.yaml from the per-user application directory.
// API keys are NEVER read from the config file; they must be supplied via
// environment variables OPENAI_API_KEY_SFA and ANTHROPIC_API_KEY_SFA.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// Always discard any key value that may have existed in the file, then
	// populate from environment variables only.
	cfg.OpenAI.APIKey = os.Getenv(EnvOpenAIKey)
	cfg.Claude.APIKey = os.Getenv(EnvClaudeKey)
	cfg.Voyage.APIKey = os.Getenv(EnvVoyageKey)
	if cfg.Embeddings.Provider == "" {
		cfg.Embeddings.Provider = "openai"
	}
	if cfg.Embeddings.Model == "" {
		switch cfg.Embeddings.Provider {
		case "voyage":
			cfg.Embeddings.Model = cfg.Voyage.Model
		case "local":
			cfg.Embeddings.Model = cfg.Local.Model
		default:
			cfg.Embeddings.Model = "text-embedding-3-small"
		}
	}
	return &cfg, nil
}

// Save writes cfg to config.yaml inside the per-user application directory.
// API keys are stripped before writing so they are never persisted to disk.
func Save(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	// Write a sanitized copy — API keys must never be persisted to disk.
	sanitized := *cfg
	sanitized.OpenAI.APIKey = ""
	sanitized.Claude.APIKey = ""
	sanitized.Voyage.APIKey = ""
	data, err := yaml.Marshal(&sanitized)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// DefaultConfig returns a sane, ready-to-edit default configuration.
// API keys are intentionally empty — supply them via OPENAI_API_KEY_SFA
// and ANTHROPIC_API_KEY_SFA environment variables.
func DefaultConfig() *Config {
	return &Config{
		Provider: "openai",
		Embeddings: EmbeddingsConfig{
			Provider: "openai",
			Model:    "text-embedding-3-small",
		},
		OpenAI: OpenAIConfig{
			APIKey: "",
			Model:  "gpt-5-mini",
		},
		Claude: ClaudeConfig{
			APIKey: "",
			Model:  "claude-4-5-haiku",
		},
		Voyage: VoyageConfig{
			APIKey: "",
			Model:  "voyage-3-large",
		},
		Local: LocalConfig{
			Endpoint:           "http://localhost:8000",
			EmbeddingsEndpoint: "http://localhost:8000/v1/embeddings",
			Model:              "llama3",
		},
		SFQ: SFQConfig{
			Command: "sfq",
		},
		CustomPromptContext: "",
	}
}
