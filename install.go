package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// installService registers winsched as a Windows service with manual start type.
func installService(interactive bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to Service Control Manager (run as Administrator): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService("winsched")
	if err == nil {
		s.Close()
		return fmt.Errorf("service 'winsched' already exists — run 'winsched.exe uninstall' first")
	}

	s, err = m.CreateService("winsched", exePath, mgr.Config{
		DisplayName: "WinSched",
		Description: "Windows scheduled task service",
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("cannot create service: %w", err)
	}
	s.Close()

	if err := eventlog.InstallAsEventCreate("winsched", eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		// Event log source may already exist — not fatal.
		if !interactive {
			fmt.Printf("Note: event log source may already exist: %v\n", err)
		}
	} else if interactive {
		fmt.Println("Event log source registered.")
	}

	if interactive {
		fmt.Println("Service 'winsched' installed successfully (StartType: automatic).")
	}
	return nil
}

// uninstallService removes the winsched Windows service.
func uninstallService(interactive bool) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to Service Control Manager (run as Administrator): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService("winsched")
	if err != nil {
		return fmt.Errorf("service 'winsched' is not installed")
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("cannot delete service: %w", err)
	}

	if interactive {
		fmt.Println("Service 'winsched' uninstalled successfully.")
	}
	return nil
}
