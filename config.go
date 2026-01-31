package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

// appConfig holds user configuration loaded from the config file.
type appConfig struct {
	Mode         string `json:"mode"`
	Provider     string `json:"provider,omitempty"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url,omitempty"`
	DefaultModel string `json:"default_model"`
	RawOutput    bool   `json:"raw_output"`
	Theme        string `json:"theme"`
	Thinking     bool   `json:"thinking"`
	WebSearch    bool   `json:"web_search"`
}

// resolvedProvider returns the configured provider name, defaulting to "anthropic".
func (c appConfig) resolvedProvider() string {
	if c.Provider == "" {
		return "anthropic"
	}
	return c.Provider
}

// configDir returns the ask configuration directory following XDG conventions.
func configDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "ask")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ask")
}

// dataDir returns the ask data directory following XDG conventions.
func dataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ask")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "ask")
}

// configPath returns the full path to the config file.
func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// loadConfig reads the config file and returns the parsed configuration.
// Returns a zero-value config if the file doesn't exist or can't be parsed.
func loadConfig() appConfig {
	var cfg appConfig
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// defaultConfigJSON returns a formatted default config.
func defaultConfigJSON() []byte {
	cfg := appConfig{
		Mode:         "cli",
		Provider:     "",
		APIKey:       "",
		BaseURL:      "",
		DefaultModel: "",
		RawOutput:    false,
		Theme:        "auto",
		Thinking:     true,
		WebSearch:    false,
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return append(data, '\n')
}

// findClaude locates the claude binary in PATH.
func findClaude() (string, error) {
	return exec.LookPath("claude")
}
