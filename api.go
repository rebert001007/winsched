package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// APIServer provides an HTTP API for dynamic task management.
type APIServer struct {
	mu          sync.Mutex
	config      *Config
	configPath  string
	logFilePath string
	sched       *Scheduler
	logger      *Logger
	httpServer  *http.Server
}

// NewAPIServer creates the API server (does not start listening).
func NewAPIServer(cfg *Config, configPath string, sched *Scheduler, logger *Logger, logFilePath string) *APIServer {
	a := &APIServer{
		config:      cfg,
		configPath:  configPath,
		logFilePath: logFilePath,
		sched:       sched,
		logger:      logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/tasks", a.handleListTasks)
	mux.HandleFunc("POST /api/tasks", a.handleAddTask)
	mux.HandleFunc("POST /api/tasks/{name}/run", a.handleRunTask)
	mux.HandleFunc("POST /api/tasks/{name}/start", a.handleStartResidentTask)
	mux.HandleFunc("POST /api/tasks/{name}/stop", a.handleStopResidentTask)
	mux.HandleFunc("POST /api/tasks/{name}/restart", a.handleRestartResidentTask)
	mux.HandleFunc("PUT /api/tasks/{name}", a.handleUpdateTask)
	mux.HandleFunc("DELETE /api/tasks/{name}", a.handleDeleteTask)
	mux.HandleFunc("GET /api/executions", a.handleExecutions)
	mux.HandleFunc("GET /api/tasks/{name}/executions", a.handleTaskExecutions)
	mux.HandleFunc("GET /api/tasks/{name}/logs", a.handleTaskLogs)
	mux.HandleFunc("GET /api/logs", HandleLogs(logFilePath))
	mux.HandleFunc("GET /", DashboardHandler())

	a.httpServer = &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%d", cfg.API.Port),
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	return a
}

// Start begins listening in a background goroutine.
func (a *APIServer) Start() {
	a.logger.Info("API server listening on %s", a.httpServer.Addr)
	go func() {
		if err := a.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			a.logger.Error("API server error: %v", err)
		}
	}()
}

// Stop gracefully shuts down the HTTP server.
func (a *APIServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.httpServer.Shutdown(ctx); err != nil {
		a.logger.Warn("API server shutdown: %v", err)
	} else {
		a.logger.Info("API server stopped")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

func (a *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"status": "ok"})
}

func (a *APIServer) handleListTasks(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	tasks := make([]TaskConfig, len(a.config.Tasks))
	copy(tasks, a.config.Tasks)
	a.mu.Unlock()

	writeOK(w, tasks)
}

func (a *APIServer) handleAddTask(w http.ResponseWriter, r *http.Request) {
	var task TaskConfig
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if task.Name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	if task.Cron == "" && !task.Resident {
		writeError(w, http.StatusBadRequest, "cron expression is required")
		return
	}

	if task.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	// Apply defaults for optional fields.
	if task.Timeout.ToGo() == 0 {
		if task.Resident {
			task.Timeout = 0
		} else {
			task.Timeout = Duration(30 * time.Minute)
		}
	}
	if task.Resident && task.RestartInterval.ToGo() == 0 {
		task.RestartInterval = Duration(10 * time.Second)
	}
	if !task.Enabled {
		writeError(w, http.StatusBadRequest, "enabled must be true for new tasks")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check for duplicate name in config (belt-and-suspenders with scheduler check).
	for _, t := range a.config.Tasks {
		if strings.EqualFold(t.Name, task.Name) {
			writeError(w, http.StatusConflict, fmt.Sprintf("task %q already exists", task.Name))
			return
		}
	}

	if err := a.sched.AddTask(task); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a.config.Tasks = append(a.config.Tasks, task)
	if err := SaveConfig(a.configPath, a.config); err != nil {
		a.logger.Error("Failed to save config after adding task %q: %v", task.Name, err)
	} else {
		a.logger.Info("Task %q added and config persisted", task.Name)
	}

	writeOK(w, task)
}

func (a *APIServer) handleExecutions(w http.ResponseWriter, r *http.Request) {
	n := 20
	if v := r.URL.Query().Get("n"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	writeOK(w, ListExecutions(n))
}

func (a *APIServer) handleTaskExecutions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}
	n := 20
	if v := r.URL.Query().Get("n"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	writeOK(w, ListExecutionsByTask(name, n))
}

func (a *APIServer) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}
	n := 20
	if v := r.URL.Query().Get("n"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	writeOK(w, ListTaskLogs(name, n))
}

func (a *APIServer) findTask(name string) (TaskConfig, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, t := range a.config.Tasks {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return TaskConfig{}, false
}

func (a *APIServer) handleRunTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	task, found := a.findTask(name)
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("task %q not found", name))
		return
	}

	if err := a.sched.RunNow(task); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeOK(w, map[string]string{"triggered": name})
}

func (a *APIServer) handleStartResidentTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	task, found := a.findTask(name)
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("task %q not found", name))
		return
	}
	if !task.Resident {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("task %q is not a resident task", name))
		return
	}
	if !task.Enabled {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("task %q is disabled", name))
		return
	}

	alreadyRunning, err := a.sched.StartResident(task)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data := map[string]any{"started": name}
	if alreadyRunning {
		data["already_running"] = true
	}
	writeOK(w, data)
}

func (a *APIServer) handleStopResidentTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	task, found := a.findTask(name)
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("task %q not found", name))
		return
	}
	if !task.Resident {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("task %q is not a resident task", name))
		return
	}

	alreadyStopped, err := a.sched.StopResident(task.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data := map[string]any{"stopped": name}
	if alreadyStopped {
		data["already_stopped"] = true
	}
	writeOK(w, data)
}

func (a *APIServer) handleRestartResidentTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	task, found := a.findTask(name)
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("task %q not found", name))
		return
	}
	if !task.Resident {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("task %q is not a resident task", name))
		return
	}
	if !task.Enabled {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("task %q is disabled", name))
		return
	}

	if err := a.sched.RestartResident(task); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeOK(w, map[string]string{"restarted": name})
}

func (a *APIServer) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	var task TaskConfig
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Name from URL path overrides body.
	task.Name = name
	if task.Cron == "" && task.Command == "" && !task.Resident {
		writeError(w, http.StatusBadRequest, "cron or command is required for update")
		return
	}

	if task.Timeout.ToGo() == 0 {
		if task.Resident {
			task.Timeout = 0
		} else {
			task.Timeout = Duration(30 * time.Minute)
		}
	}
	if task.Resident && task.RestartInterval.ToGo() == 0 {
		task.RestartInterval = Duration(10 * time.Second)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	foundIndex := -1
	updatedTask := task
	for i, t := range a.config.Tasks {
		if strings.EqualFold(t.Name, name) {
			foundIndex = i
			updatedTask = t
			if task.Cron != "" || task.Resident {
				updatedTask.Cron = task.Cron
			}
			if task.Command != "" {
				updatedTask.Command = task.Command
			}
			if task.Description != "" {
				updatedTask.Description = task.Description
			}
			if task.Args != nil {
				updatedTask.Args = task.Args
			}
			updatedTask.Timeout = task.Timeout
			updatedTask.Enabled = task.Enabled
			updatedTask.UseProxy = task.UseProxy
			updatedTask.Resident = task.Resident
			updatedTask.RestartInterval = task.RestartInterval
			break
		}
	}

	if foundIndex < 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("task %q not found", name))
		return
	}

	var schedErr error
	if updatedTask.Enabled {
		if a.sched.HasTask(name) {
			schedErr = a.sched.UpdateTask(updatedTask)
		} else {
			schedErr = a.sched.AddTask(updatedTask)
		}
	} else if a.sched.HasTask(name) {
		schedErr = a.sched.RemoveTask(name)
	}
	if schedErr != nil {
		writeError(w, http.StatusBadRequest, schedErr.Error())
		return
	}

	a.config.Tasks[foundIndex] = updatedTask
	if err := SaveConfig(a.configPath, a.config); err != nil {
		a.logger.Error("Failed to save config after updating task %q: %v", name, err)
	} else {
		a.logger.Info("Task %q updated and config persisted", name)
	}

	writeOK(w, map[string]string{"updated": name})
}

func (a *APIServer) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "task name is required")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sched.HasTask(name) {
		if err := a.sched.RemoveTask(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
	}

	found := false
	for _, t := range a.config.Tasks {
		if strings.EqualFold(t.Name, name) {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, fmt.Sprintf("task %q not found", name))
		return
	}

	// Remove from config.Tasks.
	filtered := a.config.Tasks[:0]
	for _, t := range a.config.Tasks {
		if !strings.EqualFold(t.Name, name) {
			filtered = append(filtered, t)
		}
	}
	a.config.Tasks = filtered

	if err := SaveConfig(a.configPath, a.config); err != nil {
		a.logger.Error("Failed to save config after removing task %q: %v", name, err)
	} else {
		a.logger.Info("Task %q removed and config persisted", name)
	}

	writeOK(w, map[string]string{"removed": name})
}
