package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxTaskLogOutput = 64 * 1024 // 64KB per execution

// TaskLogEntry holds the full details of one task execution.
type TaskLogEntry struct {
	ID        string     `json:"id"`
	StartTime time.Time  `json:"start_time"`
	EndTime   time.Time  `json:"end_time"`
	Status    ExecStatus `json:"status"`
	Error     string     `json:"error,omitempty"`
	Output    string     `json:"output"`
}

var execIDCounter struct {
	mu    sync.Mutex
	count int64
}

// nextExecID returns the next execution ID as a string.
func nextExecID() string {
	execIDCounter.mu.Lock()
	execIDCounter.count++
	n := execIDCounter.count
	execIDCounter.mu.Unlock()
	return fmt.Sprintf("%d", n)
}

// InitExecID scans the logs directory and sets the ID counter to the
// maximum existing ID so that restarts produce non-conflicting IDs.
func InitExecID() {
	if logsDir == "" {
		return
	}
	var maxID int64
	dirs, _ := os.ReadDir(logsDir)
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(logsDir, d.Name()))
		for _, f := range files {
			name := f.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			idStr := name[:len(name)-5]
			if n, err := strconv.ParseInt(idStr, 10, 64); err == nil && n > maxID {
				maxID = n
			}
		}
	}
	execIDCounter.mu.Lock()
	if maxID > execIDCounter.count {
		execIDCounter.count = maxID
	}
	execIDCounter.mu.Unlock()
}
