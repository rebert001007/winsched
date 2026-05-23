package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows/svc/eventlog"
)

// Logger writes to both a log file and the Windows Event Log.
type Logger struct {
	level       LogLevel
	file        *os.File
	elog        *eventlog.Log
	interactive bool
	mu          sync.Mutex
}

// NewLogger creates a dual-output logger. If filePath is empty, file logging is disabled.
// On failure to open file or event log, it continues without that output.
func NewLogger(level LogLevel, filePath string, interactive bool) (*Logger, error) {
	l := &Logger{level: level, interactive: interactive}

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "winsched: cannot open log file %s: %v\n", filePath, err)
		} else {
			l.file = f
		}
	}

	elog, err := eventlog.Open("winsched")
	if err != nil {
		if interactive {
			fmt.Fprintf(os.Stderr, "winsched: cannot open event log: %v\n", err)
		}
	} else {
		l.elog = elog
	}

	return l, nil
}

// Close flushes and closes log resources.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
	}
	if l.elog != nil {
		l.elog.Close()
	}
}

func (l *Logger) Debug(format string, args ...any) { l.log(DebugLevel, format, args...) }
func (l *Logger) Info(format string, args ...any)  { l.log(InfoLevel, format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.log(WarnLevel, format, args...) }
func (l *Logger) Error(format string, args ...any) { l.log(ErrorLevel, format, args...) }

func (l *Logger) log(level LogLevel, format string, args ...any) {
	if level < l.level {
		return
	}

	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] %s: %s\n", time.Now().In(beijingLoc).Format("2006-01-02 15:04:05"), level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.file.WriteString(line)
	}
	if l.interactive {
		os.Stdout.WriteString(line)
	}
	if l.elog != nil {
		switch level {
		case ErrorLevel:
			l.elog.Error(1, msg)
		case WarnLevel:
			l.elog.Warning(1, msg)
		default:
			l.elog.Info(1, msg)
		}
	}
}
