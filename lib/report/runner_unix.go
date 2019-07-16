// +build darwin linux

package report

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"syscall"
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
	lintCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	done, interrupt := make(chan struct{}), make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(2)
	go p.memoryMonitor(p.logger, &wg, done, interrupt)

	if err := lintCmd.Start(); err != nil {
		p.logger.WithError(err).Error("Unable to run linter.")
		return nil, false, err
	}
	go func() {
		defer wg.Done()

		pgid, err := syscall.Getpgid(lintCmd.Process.Pid)
		if err != nil {
			p.logger.WithError(err).Errorf("Failed to get process group of linter child-process %d.", lintCmd.Process.Pid)
		}

		select {
		case <-interrupt:
			p.logger.Infof("Killing 'golangci-lint' due to memory usage.")
			interruptErr := syscall.Kill(-pgid, syscall.SIGKILL)
			if interruptErr != nil {
				if strings.Contains(interruptErr.Error(), "process already finished") {
					return
				}
				p.logger.WithError(interruptErr).Error("Failed to interrupt linter with a KILL signal.")
			}
			p.logger.Info("Killed.")
			interrupted = true
		case <-done:
		}
	}()

	err := lintCmd.Wait()
	close(done)
	wg.Wait()

	if err != nil && !interrupted {
		p.logger.WithError(err).Errorf("Linter exited with an error. Output was:\n%s Error was:\n%s", stdout.Bytes(), stderr.String())
		return nil, false, err
	}
	return stdout.Bytes(), interrupted, nil
}
