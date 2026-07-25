package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLoggerLevelFiltering(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.log")

	logger, err := NewLogger(InfoLevel, filePath, false)
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("should not appear")
	logger.Info("should appear")
	logger.Warn("warning")
	logger.Error("error")
	logger.Close()

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
	logger.Debug("debug msg")
	logger.Close()
	data, _ := os.ReadFile(filePath)
	if !contains(string(data), "debug msg") {
		t.Error("debug message should appear at debug level")
	}
}

func TestLoggerDoesNotBlockWhenEventQueueIsFull(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.log")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	logger := &Logger{
		level:      InfoLevel,
		file:       file,
		localQueue: make(chan string, localLogQueueSize),
		localDone:  make(chan struct{}),
		eventQueue: make(chan logEvent, 1),
	}
	go logger.writeLocalLogs()
	logger.eventQueue <- logEvent{}

	done := make(chan struct{})
	go func() {
		logger.Info("still writes local logs")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("logging blocked on a full event queue")
	}
	logger.Close()

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "still writes local logs") {
		t.Fatal("local log entry was not written")
	}
}

type blockingWriteCloser struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockingWriteCloser) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	return len(p), nil
}

func (w *blockingWriteCloser) Close() error { return nil }

func TestLoggerDoesNotBlockWhenLocalWriterStalls(t *testing.T) {
	writer := &blockingWriteCloser{started: make(chan struct{}), release: make(chan struct{})}
	logger := &Logger{
		level:      InfoLevel,
		file:       writer,
		localQueue: make(chan string, localLogQueueSize),
		localDone:  make(chan struct{}),
	}
	go logger.writeLocalLogs()

	logger.Info("first entry blocks the writer")
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("local log worker did not start")
	}

	done := make(chan struct{})
	go func() {
		logger.Info("scheduler must not wait for local I/O")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("logging blocked on a stalled local writer")
	}

	close(writer.release)
	logger.Close()
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
