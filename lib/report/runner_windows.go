// +build windows

package report

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

func (p *parser) runManagedLint(cliArgs []string) ([]byte, bool, error) {
	if p.memoryMonitor == nil {
		p.memoryMonitor = systemMemoryMonitor
	}
	var interrupted bool

	lintCmd := exec.Command("golangci-lint", cliArgs...)
	stdout, stderr := &bytes.Buffer{}, &strings.Builder{}
	lintCmd.Stdout, lintCmd.Stderr = stdout, stderr
	lintCmd.Dir = p.project.Path

	done, interrupt := make(chan struct{}), make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(2)
	go p.memoryMonitor(p.logger, &wg, done, interrupt)

	if err := lintCmd.Start(); err != nil {
		p.logger.WithError(err).Error("Unable to run linter.")
		return nil, false, err
	}
	go p.linterMonitorProcess(lintCmd.Process, &wg, done, interrupt, &interrupted)

	err := lintCmd.Wait()
	close(done)
	wg.Wait()

	if err != nil && !interrupted {
		p.logger.WithError(err).Errorf("Linter exited with an error. Output was:\n%s Error was:\n%s", stdout.Bytes(), stderr.String())
		return nil, false, err
	}
	return stdout.Bytes(), interrupted, nil
}

func (p *parser) linterMonitorProcess(process *os.Process, wg *sync.WaitGroup, done chan struct{}, interrupt chan struct{}, interrupted *bool) {
	defer wg.Done()

	select {
	case <-interrupt:
		p.logger.Infof("Killing 'golangci-lint' due to memory usage.")
		interruptErr := p.killProcessTree(process)
		if interruptErr != nil {
			if strings.Contains(interruptErr.Error(), "process already finished") {
				return
			}
			p.logger.WithError(interruptErr).Error("Failed to interrupt linter with a KILL signal.")
		}
		p.logger.Info("Killed.")
		*interrupted = true
	case <-done:
	}
}

func (p *parser) killProcessTree(process *os.Process) error {
	logger := p.logger.WithField("process_manager", "kill")
	killCmd := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(process.Pid))
	killCmd.Stdout = logger.WriterLevel(logrus.DebugLevel)
	killCmd.Stderr = logger.WriterLevel(logrus.DebugLevel)
	return killCmd.Run()
}
