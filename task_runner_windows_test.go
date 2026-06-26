//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunTaskWithContextCancellationKillsPowerShellChildProcess(t *testing.T) {
	logger, _ := NewLogger(DebugLevel, "", false)
	defer logger.Close()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "spawn-child.ps1")
	if err := os.WriteFile(script, []byte(fmt.Sprintf(`
$p = Start-Process -FilePath cmd.exe -ArgumentList '/c ping -n 60 127.0.0.1 >nul' -PassThru -WindowStyle Hidden
Set-Content -LiteralPath %q -Value $p.Id
while ($true) { Start-Sleep -Milliseconds 200 }
`, pidFile)), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := RunTaskWithContext(ctx, TaskConfig{
		Name:    "kill-tree-test",
		Command: "pwsh.exe",
		Args:    []string{"-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script},
		Timeout: 0,
	}, ProxyConfig{}, logger, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	pidBytes, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("child pid was not written: %v", readErr)
	}
	pid := strings.TrimSpace(string(pidBytes))
	if pid == "" {
		t.Fatal("child pid is empty")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("child process %s still exists after cancellation", pid)
}

func processExists(pid string) bool {
	cmd := exec.Command("cmd.exe", "/c", "tasklist /FI \"PID eq "+pid+"\" /NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	text := string(out)
	return strings.Contains(text, pid) && !strings.Contains(text, "No tasks")
}
