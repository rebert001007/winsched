package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunTaskWithOutputStreamsChunks(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	var chunks []string
	out, err := RunTaskWithOutput(TaskConfig{
		Name:    "stream-test",
		Command: "cmd.exe",
		Args:    []string{"/c", "echo first && echo second 1>&2"},
		Timeout: Duration(5 * time.Second),
	}, ProxyConfig{}, logger, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Fatalf("expected combined output to contain stdout and stderr, got %q", out)
	}
	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "first") || !strings.Contains(joined, "second") {
		t.Fatalf("expected callback chunks to contain stdout and stderr, got %q", joined)
	}
}

func TestRunTaskWithContextZeroTimeoutUsesParentCancellation(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := RunTaskWithContext(ctx, TaskConfig{
		Name:    "cancel-test",
		Command: "cmd.exe",
		Args:    []string{"/c", "ping -n 6 127.0.0.1 >nul"},
		Timeout: 0,
	}, ProxyConfig{}, logger, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("expected canceled error, got %v", err)
	}
}
