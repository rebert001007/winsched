# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, Test, and Run

```powershell
go build -o winsched.exe .                          # Dev build
go build -ldflags="-s -w" -trimpath -o winsched.exe . # Production build (stripped, smaller)
go test ./...                                       # Run all tests
go test -run TestName .                             # Run a single test
go run . run                                       # Run interactively with default config path
go run . run .\config.yaml                          # Run interactively with local config
```

All source files are in a single flat `package main` — no sub-packages. Tests are in-package (same `package main`). In tests, use `t.TempDir()` for scratch files and `NewLogger(DebugLevel, "", false)` to create a silent logger that writes to neither file nor event log.

## Windows Service Management

```powershell
.\winsched.exe install           # Install as Windows service (as Admin)
.\winsched.exe uninstall         # Remove the Windows service (as Admin)
sc stop winsched                 # Stop service
sc start winsched                # Start service
sc query winsched                # Check status
```

## Architecture

Single-binary Go Windows service that runs cron-scheduled tasks. No framework — just the standard library plus `robfig/cron/v3`, `golang.org/x/sys/windows/svc`, and `gopkg.in/yaml.v3`.

**Startup flow:** `main.go` → `service.go:Execute()` / `runInteractive()` — both load config, start scheduler, and (if enabled) start the HTTP API.

**Scheduler** (`scheduler.go`) wraps `robfig/cron/v3` with timezone fixed to `Asia/Shanghai` (Beijing time). Each cron entry maps to `RunTask()` which shells out via `os/exec` with a per-task timeout. Failed tasks are logged, not retried. Tasks with `enabled: false` in config are skipped.

**API** (`api.go`) listens on `127.0.0.1:15732`. Uses Go 1.22+ enhanced `http.ServeMux` routing patterns (`"GET /api/tasks"`, `"PUT /api/tasks/{name}"`). All responses use the envelope `{"ok": true, "data": ...}` or `{"ok": false, "error": "..."}`. Endpoints: `GET /api/health`, `GET /api/tasks`, `POST /api/tasks`, `PUT /api/tasks/{name}`, `DELETE /api/tasks/{name}`, `GET /api/executions?n=20`, `GET /api/tasks/{name}/executions?n=10`, `GET /api/logs?n=100`. Mutations update the in-memory scheduler and persist back to `config.yaml`.

**Config** (`config.go`) lives at `C:\ProgramData\winsched\config.yaml`. `LoadConfig()` never returns nil — on missing/invalid file it returns `DefaultConfig()` and logs a warning. Task timeouts default to 30min. API port defaults to 15732.

**Execution store** (`execution.go`) is a fixed-size (200) ring buffer in memory. Each run records task name, start/end time, status (running/success/failed/timeout), error, and truncated output (256 chars max in store).

**Task runner** (`task_runner.go`) executes commands via `os/exec.CommandContext` with a context timeout. If `UseProxy` is set, it waits up to 30s for TCP connectivity to the configured proxy before launching the command. Output is truncated to 1024 chars for logging.

**Logger** (`logger.go`) writes to file + Windows Event Log (service mode) or stdout + file (interactive mode). Log levels: debug, info, warn, error.

**Dashboard** (`dashboard.go`, `dashboard/index.html`) is a self-contained admin SPA embedded via `//go:embed dashboard/index.html`, served at `GET /`. The HTML fetches all data client-side from the API endpoints above — no server-side templating.

**Install/uninstall** (`install.go`) registers the service with the Windows SCM as auto-start. The service binary path is `os.Executable()` at install time.
