package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var beijingLoc *time.Location

func init() {
	var err error
	beijingLoc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		beijingLoc = time.Local
	}
}

// ExecStatus represents the outcome of a task execution.
type ExecStatus string

const (
	StatusRunning ExecStatus = "running"
	StatusSuccess ExecStatus = "success"
	StatusFailed  ExecStatus = "failed"
	StatusTimeout ExecStatus = "timeout"
)

// ExecRecord holds the summary details of one task execution.
type ExecRecord struct {
	ID        string     `json:"id"`
	TaskName  string     `json:"task_name"`
	StartTime time.Time  `json:"start_time"`
	EndTime   time.Time  `json:"end_time,omitempty"`
	Status    ExecStatus `json:"status"`
	Error     string     `json:"error,omitempty"`
	Output    string     `json:"output,omitempty"`
}

// --- Logs directory ---

var logsDir string

// SetLogsDir sets the directory for execution log files and creates it if needed.
func SetLogsDir(dir string) {
	logsDir = dir
	os.MkdirAll(dir, 0755)
}

// --- Running entries ---

var (
	runningMu sync.Mutex
	running   = map[string]*TaskLogEntry{} // key: taskName/execID
)

func runKey(taskName, id string) string { return taskName + "/" + id }

// RecordStart generates an execution ID and records a running entry.
func RecordStart(taskName string) string {
	id := nextExecID()
	runningMu.Lock()
	running[runKey(taskName, id)] = &TaskLogEntry{
		ID:        id,
		StartTime: time.Now().In(beijingLoc),
		Status:    StatusRunning,
	}
	runningMu.Unlock()
	return id
}

// RecordEnd writes the completed execution to a file and removes the running entry.
func RecordEnd(taskName, id string, status ExecStatus, errMsg, output string) {
	now := time.Now().In(beijingLoc)

	runningMu.Lock()
	k := runKey(taskName, id)
	var startTime time.Time
	if e, ok := running[k]; ok {
		startTime = e.StartTime
		delete(running, k)
	}
	runningMu.Unlock()

	entry := TaskLogEntry{
		ID:        id,
		StartTime: startTime,
		EndTime:   now,
		Status:    status,
		Error:     errMsg,
		Output:    output,
	}
	writeLogFile(taskName, id, entry)
}

// --- File paths ---

func taskLogDir(taskName string) string {
	return filepath.Join(logsDir, sanitizePath(taskName))
}

func logFilePath(taskName, id string) string {
	return filepath.Join(taskLogDir(taskName), id+".json")
}

func sanitizePath(name string) string {
	r := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", "\"", "_",
		"/", "_", "\\", "_", "|", "_", "?", "_", "*", "_",
	)
	return r.Replace(name)
}

// --- File I/O ---

func writeLogFile(taskName, id string, entry TaskLogEntry) {
	dir := taskLogDir(taskName)
	os.MkdirAll(dir, 0755)
	f, err := os.Create(logFilePath(taskName, id))
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(entry)
}

func readLogFileRaw(taskName, id string) (*TaskLogEntry, error) {
	f, err := os.Open(logFilePath(taskName, id))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var e TaskLogEntry
	if err := json.NewDecoder(f).Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

// --- Scanning ---

// listTaskIDs returns all execution IDs in a task dir, sorted newest-first (by numeric ID).
func listTaskIDs(taskName string) []string {
	dir := taskLogDir(taskName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, name[:len(name)-5])
	}
	sort.Slice(ids, func(i, j int) bool {
		a, _ := strconv.Atoi(ids[i])
		b, _ := strconv.Atoi(ids[j])
		return a > b
	})
	return ids
}

// --- List functions ---

// ListExecutions returns recent N execution records across all tasks.
func ListExecutions(n int) []ExecRecord {
	if n <= 0 {
		n = 20
	}
	// Collect all task dirs, then collect files with IDs for sorting.
	type entry struct {
		taskName string
		id       string
		idNum    int
	}
	var all []entry
	taskDirs, _ := os.ReadDir(logsDir)
	for _, td := range taskDirs {
		if !td.IsDir() {
			continue
		}
		for _, id := range listTaskIDs(td.Name()) {
			num, _ := strconv.Atoi(id)
			all = append(all, entry{taskName: td.Name(), id: id, idNum: num})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].idNum > all[j].idNum })
	if n > len(all) {
		n = len(all)
	}

	result := make([]ExecRecord, 0, n+len(running))

	// Prepend running entries.
	runningMu.Lock()
	for k, e := range running {
		taskName := k
		if idx := strings.LastIndex(k, "/"); idx >= 0 {
			taskName = k[:idx]
		}
		result = append(result, ExecRecord{
			ID:        e.ID,
			TaskName:  taskName,
			StartTime: e.StartTime,
			Status:    StatusRunning,
		})
	}
	runningMu.Unlock()

	for i := 0; i < n; i++ {
		e, err := readLogFileRaw(all[i].taskName, all[i].id)
		if err != nil {
			continue
		}
		out := e.Output
		if len(out) > 256 {
			out = out[:256] + "..."
		}
		result = append(result, ExecRecord{
			ID:        e.ID,
			TaskName:  all[i].taskName,
			StartTime: e.StartTime,
			EndTime:   e.EndTime,
			Status:    e.Status,
			Error:     e.Error,
			Output:    out,
		})
	}

	return result
}

// ListExecutionsByTask returns recent N execution records for a task.
func ListExecutionsByTask(taskName string, n int) []ExecRecord {
	if n <= 0 {
		n = 20
	}
	ids := listTaskIDs(taskName)
	if n > len(ids) {
		n = len(ids)
	}
	result := make([]ExecRecord, 0, n)
	for i := 0; i < n; i++ {
		e, err := readLogFileRaw(taskName, ids[i])
		if err != nil {
			continue
		}
		out := e.Output
		if len(out) > 256 {
			out = out[:256] + "..."
		}
		result = append(result, ExecRecord{
			ID:        e.ID,
			TaskName:  taskName,
			StartTime: e.StartTime,
			EndTime:   e.EndTime,
			Status:    e.Status,
			Error:     e.Error,
			Output:    out,
		})
	}
	return result
}

// ListTaskLogs returns recent N full log entries for a task.
func ListTaskLogs(taskName string, n int) []TaskLogEntry {
	if n <= 0 {
		n = 20
	}
	ids := listTaskIDs(taskName)
	if n > len(ids) {
		n = len(ids)
	}
	result := make([]TaskLogEntry, 0, n)
	for i := 0; i < n; i++ {
		e, err := readLogFileRaw(taskName, ids[i])
		if err != nil {
			continue
		}
		result = append(result, *e)
	}
	return result
}

// RunningRecords returns currently executing entries as ExecRecords.
func RunningRecords() []ExecRecord {
	runningMu.Lock()
	defer runningMu.Unlock()
	var result []ExecRecord
	for k, e := range running {
		taskName := k
		if idx := strings.LastIndex(k, "/"); idx >= 0 {
			taskName = k[:idx]
		}
		result = append(result, ExecRecord{
			ID:        e.ID,
			TaskName:  taskName,
			StartTime: e.StartTime,
			Status:    StatusRunning,
		})
	}
	return result
}
