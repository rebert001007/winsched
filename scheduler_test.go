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

func TestScheduler_ResidentTaskDoesNotRegisterCron(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	cfg := &Config{
		Tasks: []TaskConfig{
			{
				Name:            "resident-task",
				Command:         "cmd.exe",
				Args:            []string{"/c", "exit 0"},
				Timeout:         0,
				Enabled:         true,
				Resident:        true,
				RestartInterval: Duration(10 * time.Millisecond),
			},
		},
	}

	sched := NewScheduler(cfg, logger)
	defer sched.Stop()

	if got := len(sched.cron.Entries()); got != 0 {
		t.Fatalf("expected resident task not to register cron entries, got %d", got)
	}
	if !sched.HasTask("resident-task") {
		t.Fatal("resident task should be tracked by scheduler")
	}
}

func TestScheduler_RemoveResidentTaskStopsTracking(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	sched := NewScheduler(&Config{}, logger)
	task := TaskConfig{
		Name:            "resident-remove",
		Command:         "cmd.exe",
		Args:            []string{"/c", "timeout /t 5 /nobreak >nul"},
		Timeout:         0,
		Enabled:         true,
		Resident:        true,
		RestartInterval: Duration(10 * time.Millisecond),
	}
	if err := sched.AddTask(task); err != nil {
		t.Fatal(err)
	}
	if !sched.HasTask(task.Name) {
		t.Fatal("resident task should be tracked before removal")
	}
	if err := sched.RemoveTask(task.Name); err != nil {
		t.Fatal(err)
	}
	if sched.HasTask(task.Name) {
		t.Fatal("resident task should not be tracked after removal")
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
