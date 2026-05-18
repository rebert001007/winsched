package main

import (
	"sync"
	"time"
)

// ExecStatus represents the outcome of a task execution.
type ExecStatus string

const (
	StatusRunning ExecStatus = "running"
	StatusSuccess ExecStatus = "success"
	StatusFailed  ExecStatus = "failed"
	StatusTimeout ExecStatus = "timeout"
)

// ExecRecord holds the details of one task execution.
type ExecRecord struct {
	TaskName  string     `json:"task_name"`
	StartTime time.Time  `json:"start_time"`
	EndTime   time.Time  `json:"end_time,omitempty"`
	Status    ExecStatus `json:"status"`
	Error     string     `json:"error,omitempty"`
	Output    string     `json:"output,omitempty"`
}

// ExecutionStore keeps a fixed-size ring buffer of execution records.
type ExecutionStore struct {
	mu       sync.Mutex
	records  []ExecRecord
	capacity int
	cursor   int // next write position
	count    int // total written (for ordering)
}

// NewExecutionStore creates a store holding up to capacity records.
func NewExecutionStore(capacity int) *ExecutionStore {
	return &ExecutionStore{
		records:  make([]ExecRecord, capacity),
		capacity: capacity,
	}
}

// RecordStart registers the start of an execution and returns the ring index
// where the record should be finalized.
func (s *ExecutionStore) RecordStart(taskName string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.cursor
	s.cursor = (s.cursor + 1) % s.capacity
	s.count++

	s.records[idx] = ExecRecord{
		TaskName:  taskName,
		StartTime: time.Now(),
		Status:    StatusRunning,
	}
	return idx
}

// RecordEnd finalizes the execution record at the given ring index.
func (s *ExecutionStore) RecordEnd(idx int, status ExecStatus, errMsg, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := &s.records[idx]
	r.EndTime = time.Now()
	r.Status = status
	r.Error = errMsg
	if len(output) > 256 {
		output = output[:256] + "..."
	}
	r.Output = output
}

// Latest returns the most recent n execution records (all tasks).
func (s *ExecutionStore) Latest(n int) []ExecRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.collect(n, nil)
}

// ByTask returns the most recent n execution records for a specific task name.
func (s *ExecutionStore) ByTask(name string, n int) []ExecRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.collect(n, func(r *ExecRecord) bool {
		return r.TaskName == name
	})
}

func (s *ExecutionStore) collect(n int, filter func(*ExecRecord) bool) []ExecRecord {
	if n <= 0 {
		n = 20
	}
	if n > s.capacity {
		n = s.capacity
	}

	result := make([]ExecRecord, 0, n)
	// Walk from newest (cursor-1) backwards.
	for i := 0; i < s.capacity && len(result) < n; i++ {
		idx := (s.cursor - 1 - i + s.capacity) % s.capacity
		if s.records[idx].TaskName == "" {
			continue
		}
		if filter != nil && !filter(&s.records[idx]) {
			continue
		}
		// Copy to avoid data race with future writes.
		rec := s.records[idx]
		result = append(result, rec)
	}
	return result
}
