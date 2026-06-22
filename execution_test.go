package main

import (
	"strings"
	"testing"
)

func TestRunningRecordsIncludeOutput(t *testing.T) {
	SetLogsDir(t.TempDir())
	runningMu.Lock()
	running = map[string]*TaskLogEntry{}
	runningMu.Unlock()

	id := RecordStart("live-task")
	RecordOutput("live-task", id, "first\n")
	RecordOutput("live-task", id, "second\n")

	records := ListExecutionsByTask("live-task", 10)
	if len(records) != 1 {
		t.Fatalf("expected 1 running record, got %d", len(records))
	}
	if records[0].Status != StatusRunning {
		t.Fatalf("expected running status, got %s", records[0].Status)
	}
	if !strings.Contains(records[0].Output, "first") || !strings.Contains(records[0].Output, "second") {
		t.Fatalf("expected running output to include appended chunks, got %q", records[0].Output)
	}

	logs := ListTaskLogs("live-task", 10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 running log, got %d", len(logs))
	}
	if logs[0].Output != "first\nsecond\n" {
		t.Fatalf("expected full running log output, got %q", logs[0].Output)
	}
}

func TestRecordOutputTruncatesRunningOutput(t *testing.T) {
	SetLogsDir(t.TempDir())
	runningMu.Lock()
	running = map[string]*TaskLogEntry{}
	runningMu.Unlock()

	id := RecordStart("large-output")
	RecordOutput("large-output", id, strings.Repeat("x", maxTaskLogOutput+10))

	logs := ListTaskLogs("large-output", 10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 running log, got %d", len(logs))
	}
	if len(logs[0].Output) != maxTaskLogOutput+3 {
		t.Fatalf("expected truncated output length %d, got %d", maxTaskLogOutput+3, len(logs[0].Output))
	}
	if !strings.HasSuffix(logs[0].Output, "...") {
		t.Fatalf("expected truncated output suffix, got %q", logs[0].Output[len(logs[0].Output)-10:])
	}
}
