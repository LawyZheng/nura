package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with defaults failed: %v", err)
	}
	if cfg.Server.Port != 8000 {
		t.Errorf("expected port 8000, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "nura.db" {
		t.Errorf("expected database path nura.db, got %s", cfg.Database.Path)
	}
	if cfg.LLM.Provider != "mock" {
		t.Errorf("expected llm provider mock, got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Temperature != 0.1 {
		t.Errorf("expected temperature 0.1, got %f", cfg.LLM.Temperature)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "console" {
		t.Errorf("expected log format console, got %s", cfg.Logging.Format)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("NURA_SERVER_PORT", "9090")
	t.Setenv("NURA_DATABASE_PATH", "test.db")
	t.Setenv("NURA_LLM_PROVIDER", "openai")
	t.Setenv("NURA_LLM_API_KEY", "sk-test")
	t.Setenv("NURA_LOGGING_LEVEL", "debug")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with env overrides failed: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "test.db" {
		t.Errorf("expected database path test.db, got %s", cfg.Database.Path)
	}
	if cfg.LLM.Provider != "openai" {
		t.Errorf("expected llm provider openai, got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.APIKey != "sk-test" {
		t.Errorf("expected llm api_key sk-test, got %s", cfg.LLM.APIKey)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := []byte(`server:
  port: 3000
  data_dir: /tmp/nura-test
database:
  path: custom.db
llm:
  provider: anthropic
  model: claude-3
  temperature: 0.5
logging:
  level: warn
  format: json
`)
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load from file failed: %v", err)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
	if cfg.Server.DataDir != "/tmp/nura-test" {
		t.Errorf("expected data_dir /tmp/nura-test, got %s", cfg.Server.DataDir)
	}
	if cfg.Database.Path != "custom.db" {
		t.Errorf("expected database path custom.db, got %s", cfg.Database.Path)
	}
	if cfg.LLM.Provider != "anthropic" {
		t.Errorf("expected llm provider anthropic, got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "claude-3" {
		t.Errorf("expected llm model claude-3, got %s", cfg.LLM.Model)
	}
	if cfg.LLM.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.LLM.Temperature)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("expected log level warn, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format json, got %s", cfg.Logging.Format)
	}
}

func TestLoadInvalidFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent config file, got nil")
	}
}
