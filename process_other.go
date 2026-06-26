//go:build !windows

package main

import (
	"os"
	"os/exec"
)

type processController struct{}

func newProcessController(cmd *exec.Cmd, logger *Logger) (*processController, error) {
	return &processController{}, nil
}

func (p *processController) AfterStart(cmd *exec.Cmd) error {
	return nil
}

func (p *processController) RequestGracefulStop(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
	}
}

func (p *processController) ForceKill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (p *processController) Close() {}
