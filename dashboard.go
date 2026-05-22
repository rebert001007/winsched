package main

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

//go:embed dashboard/index.html
var dashboardHTML string

// readLogTail reads the last maxLines lines from the log file.
func readLogTail(filePath string, maxLines int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	const bufSize = 4096
	lines := make([]string, 0, maxLines)
	buf := make([]byte, bufSize)
	pos := stat.Size()
	remnant := ""

	for pos > 0 && len(lines) < maxLines {
		readSize := int64(bufSize)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize

		f.Seek(pos, io.SeekStart)
		n, _ := io.ReadFull(f, buf[:readSize])
		chunk := string(buf[:n]) + remnant

		split := strings.Split(chunk, "\n")
		if pos > 0 {
			remnant = split[0]
			split = split[1:]
		}

		for i := len(split) - 1; i >= 0 && len(lines) < maxLines; i-- {
			if split[i] != "" {
				lines = append(lines, split[i])
			}
		}
	}

	// Reverse so lines are chronological.
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines, nil
}

// DashboardHandler serves the embedded admin dashboard HTML.
func DashboardHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dashboardHTML))
	}
}

// HandleLogs returns an HTTP handler that reads the log file tail.
func HandleLogs(logFilePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 100
		if v := r.URL.Query().Get("n"); v != "" {
			fmt.Sscanf(v, "%d", &n)
		}
		if n > 1000 {
			n = 1000
		}
		lines, err := readLogTail(logFilePath, n)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read log: %v", err))
			return
		}
		if lines == nil {
			lines = []string{}
		}
		writeOK(w, lines)
	}
}
