package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron-based task execution.
type Scheduler struct {
	cron     *cron.Cron
	logger   *Logger
	proxy    ProxyConfig
	mu       sync.Mutex
	entries  map[string]cron.EntryID // task name → cron entry ID
	resident map[string]context.CancelFunc
	notifier *TelegramNotifier
}

// NewScheduler creates a scheduler and registers all enabled tasks from config.
func NewScheduler(cfg *Config, logger *Logger) *Scheduler {
	var notifier *TelegramNotifier
	if cfg.Telegram.Enabled {
		notifier = NewTelegramNotifier(cfg.Telegram, cfg.Proxy, logger)
		if notifier != nil {
			logger.Info("Telegram notifications enabled (chat_id=%s)", cfg.Telegram.ChatID)
		}
	}

	s := &Scheduler{
		cron: cron.New(
			cron.WithLocation(beijingLoc),
			cron.WithParser(cron.NewParser(
				cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
			)),
		),
		logger:   logger,
		proxy:    cfg.Proxy,
		entries:  make(map[string]cron.EntryID),
		resident: make(map[string]context.CancelFunc),
		notifier: notifier,
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

// makeFunc builds the cron execution closure for a task.
func (s *Scheduler) makeFunc(task TaskConfig) func() {
	t := task
	return func() {
		s.executeTask(context.Background(), t)
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

func (s *Scheduler) executeTask(ctx context.Context, task TaskConfig) (ExecStatus, string, error) {
	s.logger.Info("Executing task %q: %s", task.Name, task.Command)
	startedAt := time.Now()

	execID := RecordStart(task.Name)
	output, err := RunTaskWithContext(ctx, task, s.proxy, s.logger, func(chunk string) {
		RecordOutput(task.Name, execID, chunk)
	})
	duration := time.Since(startedAt).Round(time.Second).String()

	if err != nil {
		s.logger.Error("%v", err)
		status := StatusFailed
		if IsTimeout(err) {
			status = StatusTimeout
		}
		RecordEnd(task.Name, execID, status, err.Error(), output)

		if ctx.Err() == nil && s.notifier != nil {
			s.notifier.SendFailure(task.Name, duration, string(status), err.Error(), output)
		}
		return status, output, err
	}

	s.logger.Info("Task %q completed", task.Name)
	RecordEnd(task.Name, execID, StatusSuccess, "", output)
	return StatusSuccess, output, nil
}

func (s *Scheduler) startResidentLocked(task TaskConfig) {
	ctx, cancel := context.WithCancel(context.Background())
	s.resident[task.Name] = cancel
	go s.runResident(ctx, task)
	s.logger.Info("Started resident task %q: command=%s restart_interval=%v timeout=%v",
		task.Name, task.Command, task.RestartInterval.ToGo(), task.Timeout.ToGo())
}

func (s *Scheduler) runResident(ctx context.Context, task TaskConfig) {
	interval := task.RestartInterval.ToGo()
	if interval <= 0 {
		interval = 10 * time.Second
	}

	for {
		_, _, err := s.executeTask(ctx, task)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			s.logger.Info("Resident task %q exited successfully; not restarting", task.Name)
			return
		}

		s.logger.Warn("Resident task %q exited abnormally; restarting in %v", task.Name, interval)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Scheduler) removeTaskLocked(name string) error {
	if cancel, exists := s.resident[name]; exists {
		cancel()
		delete(s.resident, name)
		s.logger.Info("Stopped resident task %q", name)
		return nil
	}

	eid, exists := s.entries[name]
	if !exists {
		return fmt.Errorf("task %q not found", name)
	}

	s.cron.Remove(eid)
	delete(s.entries, name)
	s.logger.Info("Removed task %q", name)
	return nil
}

// AddTask registers a new task with the scheduler.
func (s *Scheduler) AddTask(task TaskConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if _, exists := s.entries[task.Name]; exists {
		return fmt.Errorf("task %q already exists", task.Name)
	}
	if _, exists := s.resident[task.Name]; exists {
		return fmt.Errorf("task %q already exists", task.Name)
	}
	if task.Command == "" {
		return fmt.Errorf("command is required")
	}
	if task.Resident {
		s.startResidentLocked(task)
		return nil
	}
	if task.Cron == "" {
		return fmt.Errorf("cron expression is required")
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
	return s.removeTaskLocked(name)
}

// UpdateTask replaces an existing task's schedule and config.
func (s *Scheduler) UpdateTask(task TaskConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[task.Name]; !exists {
		if _, exists := s.resident[task.Name]; !exists {
			return fmt.Errorf("task %q not found", task.Name)
		}
	}

	if err := s.removeTaskLocked(task.Name); err != nil {
		return fmt.Errorf("task %q not found", task.Name)
	}

	if task.Resident {
		s.startResidentLocked(task)
		return nil
	}
	if task.Cron == "" {
		return fmt.Errorf("cron expression is required")
	}

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
	if exists {
		return true
	}
	_, exists = s.resident[name]
	return exists
}

// RunNow runs the given task immediately in a background goroutine,
// independent of its cron schedule.
func (s *Scheduler) RunNow(task TaskConfig) error {
	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if task.Command == "" {
		return fmt.Errorf("command is required")
	}
	go s.executeTask(context.Background(), task)
	return nil
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the scheduler and waits up to 30s for running tasks to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	for name, cancel := range s.resident {
		cancel()
		delete(s.resident, name)
	}
	s.mu.Unlock()

	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timed out waiting for running tasks to finish")
	}
}
