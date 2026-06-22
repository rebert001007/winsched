package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// exeDir is the directory containing the winsched executable.
// All config, log, and data paths default to locations under this directory.
var exeDir string

func init() {
	exe, err := os.Executable()
	if err != nil {
		exeDir = `C:\ProgramData\winsched`
	} else {
		exeDir = filepath.Dir(exe)
	}
}

// Config is the top-level configuration.
type Config struct {
	Logging  LoggingConfig  `yaml:"logging"`
	API      APIConfig      `yaml:"api"`
	Proxy    ProxyConfig    `yaml:"proxy"`
	Telegram TelegramConfig `yaml:"telegram"`
	Tasks    []TaskConfig   `yaml:"tasks"`
}

// LoggingConfig controls log output.
type LoggingConfig struct {
	File  string `yaml:"file"`
	Level string `yaml:"level"`
}

// APIConfig controls the HTTP API server.
type APIConfig struct {
	Port    int  `yaml:"port"`
	Enabled bool `yaml:"enabled"`
}

// ProxyConfig defines proxy settings for tasks that require it.
type ProxyConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// TelegramConfig controls Telegram bot notifications.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
	Enabled  bool   `yaml:"enabled"`
}

// TaskConfig defines a single scheduled task.
type TaskConfig struct {
	Name            string   `yaml:"name" json:"name"`
	Description     string   `yaml:"description" json:"description"`
	Cron            string   `yaml:"cron" json:"cron"`
	Command         string   `yaml:"command" json:"command"`
	Args            []string `yaml:"args" json:"args"`
	Timeout         Duration `yaml:"timeout" json:"timeout"`
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	UseProxy        bool     `yaml:"use_proxy" json:"use_proxy"`
	Resident        bool     `yaml:"resident" json:"resident"`
	RestartInterval Duration `yaml:"restart_interval" json:"restart_interval"`
}

// DefaultConfig returns a Config with safe defaults and no tasks.
func DefaultConfig() *Config {
	return &Config{
		Logging: LoggingConfig{
			File:  filepath.Join(exeDir, "service.log"),
			Level: "info",
		},
		API: APIConfig{
			Port:    15732,
			Enabled: true,
		},
		Proxy: ProxyConfig{
			Host: "127.0.0.1",
			Port: 10808,
		},
		Telegram: TelegramConfig{
			Enabled: false,
		},
	}
}

// LoadConfig reads and parses a YAML config file. Returns a default config
// if the file is missing or unparseable (never returns nil).
func LoadConfig(path string, logger *Logger) *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Warn("Cannot read config file %s: %v — using defaults", path, err)
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		logger.Error("Cannot parse config file %s: %v — using defaults", path, err)
		return DefaultConfig()
	}

	for i := range cfg.Tasks {
		if cfg.Tasks[i].Name == "" {
			logger.Warn("Task at index %d has no name, setting to 'unnamed-%d'", i, i)
			cfg.Tasks[i].Name = fmt.Sprintf("unnamed-%d", i)
		}
		if cfg.Tasks[i].Timeout.ToGo() == 0 {
			if cfg.Tasks[i].Resident {
				cfg.Tasks[i].Timeout = 0
			} else {
				cfg.Tasks[i].Timeout = Duration(30 * 60 * 1e9) // 30 minutes
			}
		}
		if cfg.Tasks[i].Resident && cfg.Tasks[i].RestartInterval.ToGo() == 0 {
			cfg.Tasks[i].RestartInterval = Duration(10 * 1e9) // 10 seconds
		}
	}

	if cfg.API.Port == 0 {
		cfg.API.Port = 15732
	}

	return cfg
}

// SaveConfig serializes the config as YAML and writes it to disk.
func SaveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
