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
	// We need to request a dedicated process group ID to be assigned so that we can cleanly kill
	// the entire process tree if necessary.
	lintCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &runner{
		logger: logger,
		cmd:    lintCmd,
	}
}

func (r *runner) getInterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt, os.Kill, syscall.SIGABRT, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM}
}

// We kill the linter process via it's process group ID in order to also kill any children it might
// have spawned.
func (r *runner) killLinterProcess() {
	// Don't try to kill a process that hasn't been started yet.
	if !r.started {
		return
	}

	// ... or one that does not exist.
	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err != nil {
		if !strings.Contains(err.Error(), "no such process") {
			r.logger.WithError(err).Errorf("Failed to get process group of linter child-process %d.", r.cmd.Process.Pid)
		}
		return
	}

	interruptErr := syscall.Kill(-pgid, syscall.SIGKILL)
	// Don't fail if the process already ended.
	if interruptErr != nil && strings.Contains(interruptErr.Error(), "process already finished") {
		r.logger.WithError(interruptErr).Error("Failed to interrupt linter with a KILL signal.")
	}
}
