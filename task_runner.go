package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"
)

const maxOutputLen = 256 * 1024
const maxProxyWait = 30 * time.Second
const proxyRetryInterval = 2 * time.Second

// RunTask executes the command defined by cfg with a timeout context.
// If cfg.UseProxy is set, it first verifies proxy connectivity.
// Returns truncated combined output and any error.
func RunTask(cfg TaskConfig, proxy ProxyConfig, logger *Logger) (string, error) {
	return RunTaskWithOutput(cfg, proxy, logger, nil)
}

// RunTaskWithOutput executes the command and calls onOutput as stdout/stderr
// chunks arrive. The returned output is the same bounded combined stream.
func RunTaskWithOutput(cfg TaskConfig, proxy ProxyConfig, logger *Logger, onOutput func(string)) (string, error) {
	return RunTaskWithContext(context.Background(), cfg, proxy, logger, onOutput)
}

// RunTaskWithContext executes the command using parent as the cancellation root.
// A zero timeout disables the per-task timeout and relies on parent cancellation.
func RunTaskWithContext(parent context.Context, cfg TaskConfig, proxy ProxyConfig, logger *Logger, onOutput func(string)) (string, error) {
	if cfg.UseProxy {
		if err := waitForProxy(proxy, logger); err != nil {
			return "", fmt.Errorf("task %q: proxy check failed: %w", cfg.Name, err)
		}
	}

	ctx := parent
	cancel := func() {}
	hasTaskTimeout := cfg.Timeout.ToGo() > 0
	if hasTaskTimeout {
		ctx, cancel = context.WithTimeout(parent, cfg.Timeout.ToGo())
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("task %q: stdout pipe failed: %w", cfg.Name, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("task %q: stderr pipe failed: %w", cfg.Name, err)
	}

	var outMu sync.Mutex
	out := ""
	appendOutput := func(chunk string) {
		if chunk == "" {
			return
		}
		outMu.Lock()
		out = truncate(out+chunk, maxOutputLen)
		outMu.Unlock()
		if onOutput != nil {
			onOutput(chunk)
		}
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("task %q failed to start: %w", cfg.Name, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamOutput(stdout, appendOutput, &wg)
	go streamOutput(stderr, appendOutput, &wg)

	err = cmd.Wait()
	wg.Wait()

	if out != "" {
		logger.Debug("Task %q output: %s", cfg.Name, out)
	}

	if err != nil {
		if hasTaskTimeout && ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("task %q timed out after %v", cfg.Name, cfg.Timeout.ToGo())
		}
		if parent.Err() != nil {
			return out, fmt.Errorf("task %q canceled: %w", cfg.Name, parent.Err())
		}
		return out, fmt.Errorf("task %q failed: %w", cfg.Name, err)
	}

	return out, nil
}

func streamOutput(r io.Reader, onChunk func(string), wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			onChunk(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// waitForProxy tests TCP connectivity to the proxy address, retrying up to
// maxProxyWait total. Returns nil when the proxy is reachable.
func waitForProxy(proxy ProxyConfig, logger *Logger) error {
	addr := net.JoinHostPort(proxy.Host, fmt.Sprintf("%d", proxy.Port))
	deadline := time.Now().Add(maxProxyWait)

	for {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("proxy %s unreachable after %v", addr, maxProxyWait)
		}

		logger.Debug("Proxy %s not ready (%v), retrying...", addr, err)
		time.Sleep(proxyRetryInterval)
	}
}
