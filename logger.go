package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows/svc/eventlog"
)

const localLogQueueSize = 1024

// Logger writes to both a log file and the Windows Event Log.
type Logger struct {
	level       LogLevel
	file        io.WriteCloser
	localQueue  chan string
	localDone   chan struct{}
	elog        *eventlog.Log
	eventQueue  chan logEvent
	interactive bool
	closeMu     sync.RWMutex
	closed      bool
	dropped     atomic.Uint64
}

type logEvent struct {
	level LogLevel
	msg   string
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
	if l.file != nil || interactive {
		l.localQueue = make(chan string, localLogQueueSize)
		l.localDone = make(chan struct{})
		go l.writeLocalLogs()
	}

	elog, err := eventlog.Open("winsched")
	if err != nil {
		if interactive {
			fmt.Fprintf(os.Stderr, "winsched: cannot open event log: %v\n", err)
		}
	} else {
		l.elog = elog
		l.eventQueue = make(chan logEvent, 256)
		go l.writeEvents()
	}

	return l, nil
}

// Close flushes and closes log resources.
func (l *Logger) Close() {
	l.closeMu.Lock()
	if l.closed {
		l.closeMu.Unlock()
		return
	}
	l.closed = true
	if l.localQueue != nil {
		close(l.localQueue)
	}
	l.closeMu.Unlock()

	if l.localDone != nil {
		<-l.localDone
	}
	if l.file != nil {
		_ = l.file.Close()
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

	l.closeMu.RLock()
	if !l.closed && l.localQueue != nil {
		select {
		case l.localQueue <- line:
		default:
			l.dropped.Add(1)
		}
	}
	l.closeMu.RUnlock()

	if l.eventQueue != nil {
		select {
		case l.eventQueue <- logEvent{level: level, msg: msg}:
		default:
		}
	}
}

// writeLocalLogs serializes output lines without making callers wait for I/O.
func (l *Logger) writeLocalLogs() {
	defer close(l.localDone)
	for line := range l.localQueue {
		if l.file != nil {
			_, _ = io.WriteString(l.file, line)
		}
		if l.interactive {
			_, _ = os.Stdout.WriteString(line)
		}
	}
}

func (l *Logger) writeEvents() {
	for entry := range l.eventQueue {
		if l.elog == nil {
			continue
		}
		switch entry.level {
		case ErrorLevel:
			l.elog.Error(1, entry.msg)
		case WarnLevel:
			l.elog.Warning(1, entry.msg)
		default:
			l.elog.Info(1, entry.msg)
		}
	}
}
