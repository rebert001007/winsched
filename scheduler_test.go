package main

import (
	"testing"
	"time"
)

func TestScheduler_SkipDisabled(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	cfg := &Config{
		Tasks: []TaskConfig{
			{
				Name:    "disabled-task",
				Cron:    "@every 1s",
				Command: "cmd.exe",
				Args:    []string{"/c", "echo hello"},
				Timeout: Duration(5 * time.Second),
				Enabled: false,
			},
		},
	}

	sched := NewScheduler(cfg, logger)
	entries := sched.cron.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for disabled tasks, got %d", len(entries))
	}
	sched.Stop()
}

func TestScheduler_InvalidCron(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	cfg := &Config{
		Tasks: []TaskConfig{
			{
				Name:    "bad-cron-task",
				Cron:    "not-a-cron-expr",
				Command: "cmd.exe",
				Args:    []string{"/c", "echo hello"},
				Timeout: Duration(5 * time.Second),
				Enabled: true,
			},
		},
	}

	sched := NewScheduler(cfg, logger)
	entries := sched.cron.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for invalid cron, got %d", len(entries))
	}
	sched.Stop()
}

func TestScheduler_EnabledTask(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	cfg := &Config{
		Tasks: []TaskConfig{
			{
				Name:    "valid-task",
				Cron:    "@every 1h",
				Command: "cmd.exe",
				Args:    []string{"/c", "echo hello"},
				Timeout: Duration(5 * time.Second),
				Enabled: true,
			},
		},
	}

	sched := NewScheduler(cfg, logger)
	entries := sched.cron.Entries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for enabled task, got %d", len(entries))
	}
	sched.Stop()
}

func TestScheduler_TelegramNotifierNil(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	cfg := &Config{
		Telegram: TelegramConfig{Enabled: false},
	}
	sched := NewScheduler(cfg, logger)
	defer sched.Stop()

	if sched.notifier != nil {
		t.Error("notifier should be nil when Telegram is not enabled")
	}
}

func TestScheduler_TelegramNotifierCreated(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	cfg := &Config{
		Telegram: TelegramConfig{Enabled: true, BotToken: "tok", ChatID: "123"},
	}
	sched := NewScheduler(cfg, logger)
	defer sched.Stop()

	if sched.notifier == nil {
		t.Error("notifier should be non-nil when Telegram is enabled with valid config")
	}
}
