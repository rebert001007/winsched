//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole         = kernel32.NewProc("AttachConsole")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
)

type processController struct {
	job    windows.Handle
	logger *Logger
}

func newProcessController(cmd *exec.Cmd, logger *Logger) (*processController, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NEW_CONSOLE,
		HideWindow:    true,
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}

	return &processController{job: job, logger: logger}, nil
}

func (p *processController) AfterStart(cmd *exec.Cmd) error {
	if p == nil || p.job == 0 || cmd.Process == nil {
		return nil
	}
	process, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_SET_QUOTA, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	return windows.AssignProcessToJobObject(p.job, process)
}

func (p *processController) RequestGracefulStop(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	attached := p.attachConsole(cmd.Process.Pid)
	if attached {
		defer p.freeConsole()
	}
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)); err != nil && p.logger != nil {
		p.logger.Debug("Graceful stop signal failed for pid %d: %v", cmd.Process.Pid, err)
	}
}

func (p *processController) ForceKill(cmd *exec.Cmd) {
	if p != nil && p.job != 0 {
		if err := windows.TerminateJobObject(p.job, 1); err == nil {
			return
		} else if p.logger != nil && cmd.Process != nil {
			p.logger.Warn("TerminateJobObject failed for pid %d: %v", cmd.Process.Pid, err)
		}
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (p *processController) Close() {
	if p == nil || p.job == 0 {
		return
	}
	_ = windows.CloseHandle(p.job)
	p.job = 0
}

func (p *processController) attachConsole(pid int) bool {
	if r1, _, err := procAttachConsole.Call(uintptr(pid)); r1 == 0 {
		if p.logger != nil {
			p.logger.Debug("AttachConsole failed for pid %d: %v", pid, err)
		}
		return false
	}
	if r1, _, err := procSetConsoleCtrlHandler.Call(0, 1); r1 == 0 && p.logger != nil {
		p.logger.Debug("SetConsoleCtrlHandler(ignore=true) failed: %v", err)
	}
	return true
}

func (p *processController) freeConsole() {
	if r1, _, err := procSetConsoleCtrlHandler.Call(0, 0); r1 == 0 && p.logger != nil {
		p.logger.Debug("SetConsoleCtrlHandler(ignore=false) failed: %v", err)
	}
	if r1, _, err := procFreeConsole.Call(); r1 == 0 && p.logger != nil {
		p.logger.Debug("FreeConsole failed: %v", err)
	}
}
