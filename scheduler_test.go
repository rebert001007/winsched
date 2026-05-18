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
