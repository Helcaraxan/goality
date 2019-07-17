package report

import (
	"context"
	"math"
	"os"
	"os/exec"
	"os/signal"
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
	sigs := make(chan os.Signal)
	signal.Notify(sigs, r.getInterruptSignals()...)
	go r.signalHandler(sigs)

	done, interrupt := make(chan struct{}), make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(2)
	go r.statusMonitor(&wg, done, interrupt)
	go r.memoryMonitorFunc(r.logger, &wg, done, interrupt)

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

	if err != nil {
		return false, err
	}
	return r.interrupted, nil
}

func (r *runner) statusMonitor(wg *sync.WaitGroup, done chan struct{}, interrupt chan struct{}) {
	defer wg.Done()
	select {
	case <-interrupt:
		r.killLock.Lock()
		r.logger.Info("Killing linter due to memory usage.")
		r.killLinterProcess()
		r.logger.Info("Killed.")
		r.interrupted = true
		r.killLock.Unlock()
	case <-done:
	}
}

func (r *runner) signalHandler(sigs chan os.Signal) {
	// Always unregister the signal handlers on exit.
	defer signal.Reset()

	if _, ok := <-sigs; ok {
		r.killLock.Lock()
		r.killLinterProcess()
		os.Exit(1)
	}
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
func systemMemoryMonitor(logger *logrus.Logger, wg *sync.WaitGroup, done chan struct{}, interrupt chan struct{}) {
	defer wg.Done()
	defer close(interrupt)

	var swapUsedBase uint64 = math.MaxUint64
	for {
		select {
		case <-done:
			return
		case <-time.After(1 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		memStat, err := mem.VirtualMemoryWithContext(ctx)
		cancel()
		if err != nil {
			logger.WithError(err).Debugf("Failed to retrieve memory usage.")
		}
		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		swapStat, err := mem.SwapMemoryWithContext(ctx)
		cancel()
		if err != nil {
			logger.WithError(err).Debugf("Failed to retrieve swap usage.")
		}

		var swapUsed uint64
		if swapStat.Used < swapUsedBase {
			swapUsedBase = swapStat.Used
		} else {
			swapUsed = swapStat.Used - swapUsedBase
		}
		usedPercent := float64(memStat.Used+swapUsed) / float64(memStat.Total)
		logger.Debugf("Memory usage: %.2f%%.", usedPercent*100)
		if usedPercent > 0.9 {
			return
		}
	}
}
