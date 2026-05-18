package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoggerLevelFiltering(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.log")

	logger, err := NewLogger(InfoLevel, filePath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Debug("should not appear")
	logger.Info("should appear")
	logger.Warn("warning")
	logger.Error("error")

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if contains(content, "should not appear") {
		t.Error("debug message should have been filtered out")
	}
	if !contains(content, "should appear") {
		t.Error("info message should be present")
	}
	if !contains(content, "warning") {
		t.Error("warn message should be present")
	}
	if !contains(content, "error") {
		t.Error("error message should be present")
	}
}

func TestLoggerDebugLevel(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.log")

	logger, err := NewLogger(DebugLevel, filePath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Debug("debug msg")
	data, _ := os.ReadFile(filePath)
	if !contains(string(data), "debug msg") {
		t.Error("debug message should appear at debug level")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
