// +build windows

package report

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/sirupsen/logrus"
)

func newRunner(logger *logrus.Logger, cliArgs []string) *runner {
	return &runner{cmd: exec.Command("golangci-lint.exe", cliArgs...)}
}

func (r *runner) getInterruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt, os.Kill}
}

func (r *runner) killLinterProcess() {
	// Don't try to kill a process that hasn't been started yet.
	if !r.started {
		return
	}

	// We kill the process via the 'taskkill' utility in order to also (attempt to) kill any
	// potential child-process that were spawned.
	logger := r.logger.WithField("process_manager", "kill")
	killCmd := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(r.cmd.Process.Pid))
	killCmd.Stdout = logger.WriterLevel(logrus.DebugLevel)
	killCmd.Stderr = logger.WriterLevel(logrus.DebugLevel)
	if err := killCmd.Run(); err != nil {
		// TODO: filter out the issue of killing a non-existent process.
		r.logger.WithError(err).Warn("Failed to taskkill process %d.", r.cmd.Process.Pid)
	}
}
