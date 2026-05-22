package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatStartMessage(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 0, 5, 0, time.Local)
	msg := FormatStartMessage("daily-report", "0 9 * * *", now)

	if !strings.Contains(msg, "daily-report") {
		t.Error("message should contain task name")
	}
	if !strings.Contains(msg, "0 9 * * *") {
		t.Error("message should contain cron expression")
	}
	if !strings.Contains(msg, "2026-05-21 09:00:05") {
		t.Error("message should contain start timestamp")
	}
	if !strings.Contains(msg, "▶️") {
		t.Error("message should contain start icon")
	}
}

func TestFormatStartMessage_EscapesHTML(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	msg := FormatStartMessage("<b>&bad</b>", "*/5 * * * *", now)

	if strings.Contains(msg, "<b>") && !strings.Contains(msg, "&lt;b&gt;") {
		t.Error("HTML special chars in task name should be escaped")
	}
}

func TestFormatSuccessMessage(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 5, 30, 0, time.Local)
	msg := FormatSuccessMessage("daily-report", "5m25s", "hello world", now)

	if !strings.Contains(msg, "daily-report") {
		t.Error("message should contain task name")
	}
	if !strings.Contains(msg, "5m25s") {
		t.Error("message should contain duration")
	}
	if !strings.Contains(msg, "hello world") {
		t.Error("message should contain output")
	}
	if !strings.Contains(msg, "✅") {
		t.Error("message should contain success icon")
	}
}

func TestFormatSuccessMessage_NoOutput(t *testing.T) {
	msg := FormatSuccessMessage("test", "1s", "", time.Now())

	if strings.Contains(msg, "Output") {
		t.Error("message should not contain Output section when output is empty")
	}
}

func TestFormatSuccessMessage_EscapesHTML(t *testing.T) {
	msg := FormatSuccessMessage("<script>", "1s", "<tag>", time.Now())

	if strings.Contains(msg, "<script>") && !strings.Contains(msg, "&lt;script&gt;") {
		t.Error("HTML in task name should be escaped")
	}
	if strings.Contains(msg, "<tag>") && !strings.Contains(msg, "&lt;tag&gt;") {
		t.Error("HTML in output should be escaped")
	}
}

func TestFormatFailureMessage(t *testing.T) {
	now := time.Date(2026, 5, 21, 9, 5, 0, 0, time.Local)
	msg := FormatFailureMessage("daily-report", "5m0s", "failed", `task "daily-report" failed: exit status 1`, now)

	if !strings.Contains(msg, "daily-report") {
		t.Error("message should contain task name")
	}
	if !strings.Contains(msg, "failed") {
		t.Error("message should contain status")
	}
	if !strings.Contains(msg, "exit status 1") {
		t.Error("message should contain error text")
	}
	if !strings.Contains(msg, "❌") {
		t.Error("message should contain failure icon for failed status")
	}
}

func TestFormatFailureMessage_Timeout(t *testing.T) {
	now := time.Now()
	msg := FormatFailureMessage("task1", "5m0s", "timeout", "timed out", now)

	if !strings.Contains(msg, "⏰") {
		t.Error("timeout message should contain alarm clock icon")
	}
	if !strings.Contains(msg, "timeout") {
		t.Error("timeout message should contain timeout label")
	}
}

func TestFormatFailureMessage_TruncatesLongError(t *testing.T) {
	longErr := strings.Repeat("x", 2000)
	msg := FormatFailureMessage("task", "1s", "failed", longErr, time.Now())

	if len(msg) > 2048 {
		t.Errorf("message should not be excessively long, got %d chars", len(msg))
	}
}

func TestFormatFailureMessage_NoError(t *testing.T) {
	msg := FormatFailureMessage("task", "1s", "failed", "", time.Now())

	if strings.Contains(msg, "Error:") {
		t.Error("message should not contain Error section when errMsg is empty")
	}
}

func TestNewTelegramNotifier_Disabled(t *testing.T) {
	logger, _ := NewLogger(InfoLevel, "", false)
	defer logger.Close()

	n := NewTelegramNotifier(TelegramConfig{Enabled: false}, ProxyConfig{}, logger)
	if n != nil {
		t.Error("notifier should be nil when disabled")
	}
}

func TestNewTelegramNotifier_EmptyToken(t *testing.T) {
	logger, _ := NewLogger(InfoLevel, "", false)
	defer logger.Close()

	n := NewTelegramNotifier(TelegramConfig{Enabled: true, BotToken: "", ChatID: "123"}, ProxyConfig{}, logger)
	if n != nil {
		t.Error("notifier should be nil when bot_token is empty")
	}
}

func TestNewTelegramNotifier_EmptyChatID(t *testing.T) {
	logger, _ := NewLogger(InfoLevel, "", false)
	defer logger.Close()

	n := NewTelegramNotifier(TelegramConfig{Enabled: true, BotToken: "token", ChatID: ""}, ProxyConfig{}, logger)
	if n != nil {
		t.Error("notifier should be nil when chat_id is empty")
	}
}

func TestNewTelegramNotifier_Valid(t *testing.T) {
	logger, _ := NewLogger(InfoLevel, "", false)
	defer logger.Close()

	n := NewTelegramNotifier(TelegramConfig{Enabled: true, BotToken: "token", ChatID: "123"}, ProxyConfig{}, logger)
	if n == nil {
		t.Error("notifier should be non-nil with valid config")
	}
}
