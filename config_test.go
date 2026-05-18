package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default level 'info', got %q", cfg.Logging.Level)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := `
logging:
  level: debug
tasks:
  - name: test-task
    cron: "@every 5m"
    command: "echo"
    args: ["hello"]
    enabled: true
    timeout: "2m"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	logger, _ := NewLogger(InfoLevel, "", false)
	cfg := LoadConfig(path, logger)
	if cfg == nil {
		t.Fatal("LoadConfig returned nil")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected level 'debug', got %q", cfg.Logging.Level)
	}
	if len(cfg.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(cfg.Tasks))
	}
	if cfg.Tasks[0].Name != "test-task" {
		t.Errorf("expected task name 'test-task', got %q", cfg.Tasks[0].Name)
	}
	if cfg.Tasks[0].Timeout.ToGo().Minutes() != 2 {
		t.Errorf("expected timeout 2m, got %v", cfg.Tasks[0].Timeout.ToGo())
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	logger, _ := NewLogger(InfoLevel, "", false)
	cfg := LoadConfig(`C:\nonexistent\path\config.yaml`, logger)
	if cfg == nil {
		t.Fatal("LoadConfig should return default config for missing file")
	}
	if len(cfg.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(cfg.Tasks))
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{{{bad yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	logger, _ := NewLogger(InfoLevel, "", false)
	cfg := LoadConfig(path, logger)
	if cfg == nil {
		t.Fatal("LoadConfig should return default config for invalid YAML")
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	data := `timeout: "5m"`
	var cfg struct {
		Timeout Duration `yaml:"timeout"`
	}
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatal(err)
	}
	expected := Duration(5 * 60 * 1e9)
	if cfg.Timeout != expected {
		t.Errorf("expected %v, got %v", expected, cfg.Timeout)
	}
}
