package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron-based task execution.
type Scheduler struct {
	cron      *cron.Cron
	logger    *Logger
	proxy     ProxyConfig
	execStore *ExecutionStore
	mu        sync.Mutex
	entries   map[string]cron.EntryID // task name → cron entry ID
}

// NewScheduler creates a scheduler and registers all enabled tasks from config.
func NewScheduler(cfg *Config, logger *Logger) *Scheduler {
	s := &Scheduler{
		cron: cron.New(
			cron.WithParser(cron.NewParser(
				cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
			)),
		),
		logger:    logger,
		proxy:     cfg.Proxy,
		execStore: NewExecutionStore(200),
		entries:   make(map[string]cron.EntryID),
	}

	for _, task := range cfg.Tasks {
		if !task.Enabled {
			continue
		}
		if err := s.AddTask(task); err != nil {
			logger.Error("%v — skipped", err)
		}
	}

	return s
}

// ExecStore returns the execution history store (for API access).
func (s *Scheduler) ExecStore() *ExecutionStore { return s.execStore }

// makeFunc builds the cron execution closure for a task.
func (s *Scheduler) makeFunc(task TaskConfig) func() {
	t := task
	return func() {
		s.logger.Info("Executing task %q: %s", t.Name, t.Command)
		idx := s.execStore.RecordStart(t.Name)
		output, err := RunTask(t, s.proxy, s.logger)
		if err != nil {
			s.logger.Error("%v", err)
			status := StatusFailed
			if IsTimeout(err) {
				status = StatusTimeout
			}
			s.execStore.RecordEnd(idx, status, err.Error(), output)
		} else {
			s.logger.Info("Task %q completed", t.Name)
			s.execStore.RecordEnd(idx, StatusSuccess, "", output)
		}
	}
}

// IsTimeout reports whether err indicates a task timeout.
func IsTimeout(err error) bool {
	return err != nil && containsStr(err.Error(), "timed out")
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// AddTask registers a new task with the cron engine.
func (s *Scheduler) AddTask(task TaskConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if _, exists := s.entries[task.Name]; exists {
		return fmt.Errorf("task %q already exists", task.Name)
	}

	eid, err := s.cron.AddFunc(task.Cron, s.makeFunc(task))
	if err != nil {
		return fmt.Errorf("cannot register task %q (cron=%q): %w", task.Name, task.Cron, err)
	}

	s.entries[task.Name] = eid
	s.logger.Info("Registered task %q: cron=%q command=%s timeout=%v",
		task.Name, task.Cron, task.Command, task.Timeout.ToGo())
	return nil
}

// RemoveTask stops and removes a task by name.
func (s *Scheduler) RemoveTask(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	eid, exists := s.entries[name]
	if !exists {
		return fmt.Errorf("task %q not found", name)
	}

	s.cron.Remove(eid)
	delete(s.entries, name)
	s.logger.Info("Removed task %q", name)
	return nil
}

// UpdateTask replaces an existing task's schedule and config.
func (s *Scheduler) UpdateTask(task TaskConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	eid, exists := s.entries[task.Name]
	if !exists {
		return fmt.Errorf("task %q not found", task.Name)
	}

	s.cron.Remove(eid)
	delete(s.entries, task.Name)

	newEid, err := s.cron.AddFunc(task.Cron, s.makeFunc(task))
	if err != nil {
		return fmt.Errorf("cannot update task %q (cron=%q): %w", task.Name, task.Cron, err)
	}

	s.entries[task.Name] = newEid
	s.logger.Info("Updated task %q: cron=%q command=%s timeout=%v",
		task.Name, task.Cron, task.Command, task.Timeout.ToGo())
	return nil
}

// HasTask reports whether a task with the given name is registered.
func (s *Scheduler) HasTask(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.entries[name]
	return exists
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the scheduler and waits up to 30s for running tasks to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timed out waiting for running tasks to finish")
	}
}
