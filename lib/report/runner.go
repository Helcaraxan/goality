package report

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
)

type runner struct {
	logger *logrus.Logger
	cmd    *exec.Cmd

	started     bool
	interrupted bool

	runLock  sync.Mutex
	killLock sync.Mutex

	memoryMonitorFunc func(*logrus.Logger, *sync.WaitGroup, chan struct{}, chan struct{})
}

func (r *runner) run() (bool, error) {
	r.runLock.Lock()
	defer r.runLock.Unlock()

	if r.memoryMonitorFunc == nil {
		r.memoryMonitorFunc = systemMemoryMonitor
	}

	// Register clean-up handlers so that we can clean up the running linter in
	// the case of an process-level interruption.
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, r.getInterruptSignals()...)

	go r.signalHandler(sigs)

	done, kill := make(chan struct{}), make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(2)

	go r.statusMonitor(&wg, done, kill)
	go r.memoryMonitorFunc(r.logger, &wg, done, kill)

	r.killLock.Lock()
	if r.interrupted {
		return true, nil
	}

	if err := r.cmd.Start(); err != nil {
		r.killLock.Unlock()
		r.logger.WithError(err).Error("Unable to run linter.")

		return false, err
	}

	r.started = true

	r.killLock.Unlock()

	err := r.cmd.Wait()

	close(done)
	close(sigs)
	wg.Wait()

	if err != nil && !strings.Contains(err.Error(), "killed") {
		return false, err
	}

	return r.interrupted, nil
}

func (r *runner) statusMonitor(wg *sync.WaitGroup, done chan struct{}, kill chan struct{}) {
	defer wg.Done()

	// The 'kill' channel will always be closed and tell that the time has come for clean-up. The
	// status of the 'done' channel, which is only closed once the process has exited, will indicate
	// whether we should actually kill the process or not.
	<-kill
	select {
	case <-done:
		// Don't do anything. The process has already exited.
	default:
		// Kill the process as it hasn't exited yet.
		r.killLock.Lock()
		r.logger.Info("Killing linter due to memory usage.")
		r.killLinterProcess()
		r.logger.Info("Killed.")
		r.interrupted = true
		r.killLock.Unlock()
	}
}

func (r *runner) signalHandler(sigs chan os.Signal) {
	// A kill request corresponds to a token being received whereas a simple "done" signal is
	// transmitted via the closing of the channel by the main goroutine.
	if _, ok := <-sigs; ok {
		r.killLock.Lock()
		r.killLinterProcess()

		signal.Reset()
		os.Exit(1)
	}

	signal.Reset()
}

// The default method of monitoring memory usage uses a non-trivial strategy in order to satisfy the
// particular case of running 'golangci-lint', as well as doing so on varying platforms.
//
// 1. Running linters should be quick and not rely on swap space as that would slow things down
//    considerably. Instead we should exit and rerun the linter on a smaller set of packages.
// 2. Running linters should not result in the machine running out of memory in order to preserve
//    responsiveness of any user interface.
//
// We achieve this by:
// - Taking a base-line of the swap memory that is being used and updating it whenever it decreases.
// - Adding any usage of swap over the base-line amount to the amount of virtual memory used.
// - Interrupting as soon as the overall memory usage (virtual memory & swap above base-line) passes
//   above 90% of the available amount of virtual memory.
//
// In particular, we cannot rely on the `UsedPercent` fields available in both the
// `VirtualMemoryStat` and `SwapMemoryStat` types. In the case of Darwin / OSX swap is dynamically
// allocated, grown and shrunk by the operating system, resulting in the reported percentage not
// reflecting the amount of data that is actually being held in swap.
func systemMemoryMonitor(logger *logrus.Logger, wg *sync.WaitGroup, done chan struct{}, kill chan struct{}) {
	defer wg.Done()
	defer close(kill)

	var swapUsedBaseline uint64 = math.MaxUint64

	for {
		select {
		case <-done:
			return
		case <-time.After(1 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		memStat, err := mem.VirtualMemoryWithContext(ctx)

		if err != nil {
			logger.WithError(err).Debugf("Failed to retrieve memory usage.")
		}

		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		swapStat, err := mem.SwapMemoryWithContext(ctx)

		if err != nil {
			logger.WithError(err).Debugf("Failed to retrieve swap usage.")
		}

		cancel()

		swapUsed := uint64(0)
		if swapStat.Used < swapUsedBaseline {
			swapUsed = swapStat.Used
		} else {
			swapUsed = swapStat.Used - swapUsedBaseline
		}

		used := float64(memStat.Used+swapUsed) / float64(memStat.Total)
		logger.Debugf(
			"Memory usage: %.2f%% - RAM %s / Swap %s.",
			used*100,
			humanBytes(memStat.Used),
			humanBytes(swapUsed),
		)

		if used > 0.9 {
			return
		}
	}
}

// Simple utility function to print out memory sizes in human readable format.
func humanBytes(byteCount uint64) string {
	const unit = 1024
	if byteCount < unit {
		return fmt.Sprintf("%d B", byteCount)
	}

	div, exp := int64(unit), 0
	for n := byteCount / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(byteCount)/float64(div), "KMGTPE"[exp])
}
