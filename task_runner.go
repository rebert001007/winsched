package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"
)

const maxOutputLen = 1024
const maxProxyWait = 30 * time.Second
const proxyRetryInterval = 2 * time.Second

// RunTask executes the command defined by cfg with a timeout context.
// If cfg.UseProxy is set, it first verifies proxy connectivity.
// Returns truncated combined output and any error.
func RunTask(cfg TaskConfig, proxy ProxyConfig, logger *Logger) (string, error) {
	if cfg.UseProxy {
		if err := waitForProxy(proxy, logger); err != nil {
			return "", fmt.Errorf("task %q: proxy check failed: %w", cfg.Name, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout.ToGo())
	defer cancel()

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	raw, err := cmd.CombinedOutput()

	out := truncate(string(raw), maxOutputLen)
	if out != "" {
		logger.Debug("Task %q output: %s", cfg.Name, out)
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("task %q timed out after %v", cfg.Name, cfg.Timeout.ToGo())
		}
		return out, fmt.Errorf("task %q failed: %w", cfg.Name, err)
	}

	return out, nil
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
