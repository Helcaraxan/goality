// +build darwin linux

package report

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
)

func newRunner(logger *logrus.Logger, cliArgs []string) *runner {
	lintCmd := exec.Command("golangci-lint", cliArgs...)
	lintCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &runner{
		logger: logger,
		cmd:    lintCmd,
	}
}

func (r *runner) getInterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt, os.Kill, syscall.SIGABRT, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM}
}

func (r *runner) killLinterProcess() {
	// Don't try to kill a process that hasn't been started yet or has already ended.
	if !r.started || r.cmd.ProcessState.Exited() {
		return
	}

	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err != nil {
		r.logger.WithError(err).Errorf("Failed to get process group of linter child-process %d.", r.cmd.Process.Pid)
	}

	interruptErr := syscall.Kill(-pgid, syscall.SIGKILL)
	if interruptErr != nil {
		if strings.Contains(interruptErr.Error(), "process already finished") {
			return
		}
		r.logger.WithError(interruptErr).Error("Failed to interrupt linter with a KILL signal.")
	}
}
