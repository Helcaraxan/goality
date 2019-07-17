package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golangci/golangci-lint/pkg/config"
	"github.com/golangci/golangci-lint/pkg/logutils"
	"github.com/golangci/golangci-lint/pkg/report"
	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
)

func Parse(logger *logrus.Logger, path string, opts ...*LintOpts) (*Project, error) {
	if logger == nil {
		logger = logrus.New()
		logger.SetOutput(ioutil.Discard)
	}
	opt, err := aggregateLintOpts(opts...)
	if err != nil {
		return nil, err
	}

	parser := &parser{
		logger: logger,
		opts:   opt,
	}
	if parser.opts == nil {
		parser.opts = &LintOpts{}
	}

	logger.Infof("Parsing project at path %q.", path)
	if err = parser.parse(path); err != nil {
		return nil, err
	}
	logger.Infof("Linting project at path %q.", path)
	if err = parser.lint(); err != nil {
		return nil, err
	}
	return parser.project, nil
}

type LintOpts struct {
	linters      []string
	configPath   string
	excludePaths []string
}

func WithLinters(linters ...string) *LintOpts {
	return &LintOpts{linters: linters}
}

func WithConfig(configFilePath string) *LintOpts {
	return &LintOpts{configPath: configFilePath}
}

func WithExcludePaths(excludePaths ...string) *LintOpts {
	return &LintOpts{excludePaths: excludePaths}
}

func aggregateLintOpts(opts ...*LintOpts) (*LintOpts, error) {
	var configPaths, excludePaths, linters []string
	for _, opt := range opts {
		linters = append(linters, opt.linters...)
		excludePaths = append(excludePaths, opt.excludePaths...)
		if opt.configPath != "" {
			configPaths = append(configPaths, opt.configPath)
		}
	}
	if len(configPaths) > 1 {
		return nil, fmt.Errorf("conflicting options: multiple configuration files were specified: %v", configPaths)
	} else if len(configPaths) == 1 {
		skipDirs, err := retrieveExcludePathsFromConfig(configPaths[0])
		if err != nil {
			return nil, err
		}
		excludePaths = append(excludePaths, skipDirs...)
	}

	aggregate := &LintOpts{}

	var lastExcludePath, lastLinter string
	sort.Strings(linters)
	for idx := range linters {
		if linters[idx] != lastLinter {
			aggregate.linters = append(aggregate.linters, linters[idx])
			lastLinter = linters[idx]
		}
	}
	sort.Strings(excludePaths)
	for idx := range excludePaths {
		if excludePaths[idx] != lastExcludePath {
			aggregate.excludePaths = append(aggregate.excludePaths, excludePaths[idx])
			lastExcludePath = excludePaths[idx]
		}
	}
	if len(configPaths) == 1 {
		aggregate.configPath = configPaths[0]
	}
	return aggregate, nil
}

func retrieveExcludePathsFromConfig(configPath string) ([]string, error) {
	fakeConfig := config.Config{Run: config.Run{Config: configPath}}
	fakeLogger := logutils.NewStderrLog("config-parser")
	fakeLogger.SetLevel(logutils.LogLevelError)

	parsedConfig := config.Config{}
	configReader := config.NewFileReader(&parsedConfig, &fakeConfig, fakeLogger)
	if err := configReader.Read(); err != nil {
		return nil, fmt.Errorf("failed to read or parse config from '%s': %v", configPath, err)
	}
	return parsedConfig.Run.SkipDirs, nil
}

func (o *LintOpts) toArgs() []string {
	args := []string{}
	if o.configPath == "" {
		args = append(args, "--no-config")
	} else {
		args = append(args, "--config="+o.configPath)
	}

	if len(o.linters) > 0 {
		args = append(args, "--disable-all", "--enable="+strings.Join(o.linters, ","))
	}

	if len(o.excludePaths) > 0 {
		args = append(args, "--skip-dirs="+strings.Join(o.excludePaths, ","))
	}
	return args
}

type memoryMonitorFunc func(logger *logrus.Logger, wg *sync.WaitGroup, done chan struct{}, interrupt chan struct{})

type parser struct {
	logger        *logrus.Logger
	project       *Project
	opts          *LintOpts
	memoryMonitor memoryMonitorFunc
}

func (p *parser) parse(path string) error {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			p.logger.WithError(err).Error("Could not determine the current working directory.")
			return err
		}
		path = filepath.Join(cwd, path)
	}

	cwd, err := os.Getwd()
	if err != nil {
		p.logger.WithError(err).Errorf("Could not determine the current working directory.")
		return err
	}
	if err = os.Chdir(path); err != nil {
		p.logger.WithError(err).Errorf("Could not move to the project's root directory.")
		return err
	}
	defer func() {
		if moveErr := os.Chdir(cwd); moveErr != nil {
			p.logger.WithError(err).Errorf("Failed to move back to the original working directory %q.", cwd)
		}
	}()

	root, err := p.parseDirectory(".")
	if err != nil {
		return err
	}
	p.project = &Project{
		Path: path,
		root: root,
	}
	return nil
}

func (p *parser) parseDirectory(path string) (*Directory, error) {
	excludedDirNames := map[string]struct{}{
		"mocks":  {},
		"vendor": {},
	}
	for idx := range p.opts.excludePaths {
		excludedDirNames[p.opts.excludePaths[idx]] = struct{}{}
	}

	dir, err := os.Open(path)
	if err != nil {
		p.logger.WithError(err).Errorf("Could not open project directory %q.", path)
		return nil, err
	}
	defer func() { _ = dir.Close() }()

	dirContents, err := dir.Readdir(-1)
	if err != nil {
		p.logger.WithError(err).Errorf("Could not read content of project directory %q.", path)
		return nil, err
	}

	directory := &Directory{
		Path:           path,
		SubDirectories: map[string]*Directory{},
		Files:          map[string]*File{},
	}
	for _, dirContent := range dirContents {
		if dirContent.IsDir() {
			if _, ok := excludedDirNames[dirContent.Name()]; ok {
				continue
			}

			subDir, dirErr := p.parseDirectory(filepath.Join(path, dirContent.Name()))
			if dirErr != nil {
				return nil, dirErr
			}
			directory.SubDirectories[dirContent.Name()] = subDir
		} else if strings.HasSuffix(dirContent.Name(), ".go") {
			file, fileErr := p.parseFile(filepath.Join(path, dirContent.Name()))
			if fileErr != nil {
				return nil, fileErr
			}
			directory.Files[dirContent.Name()] = file
		}
	}
	return directory, nil
}

func (p *parser) parseFile(path string) (*File, error) {
	file, err := os.Open(path)
	if err != nil {
		p.logger.WithError(err).Errorf("Failed to open project file %q.", path)
		return nil, err
	}
	defer func() { _ = file.Close() }()

	locCount, err := locCounter(file)
	if err != nil {
		p.logger.WithError(err).Errorf("Failed to count lines of code in project file %q.", path)
		return nil, err
	}

	return &File{
		Path:      path,
		LineCount: locCount,
		Issues:    map[string][]*result.Issue{},
	}, nil
}

func (p *parser) lint() error {
	cliArgs := append([]string{
		"run",
		"--issues-exit-code=0",
		"--max-issues-per-linter=0",
		"--max-same-issues=0",
		"--new=false",
		"--new-from-rev=",
		"--out-format=json",
	}, p.opts.toArgs()...)

	todo := []*Directory{p.project.root}
	for len(todo) > 0 {
		current := todo[0]
		todo = todo[1:]

		if !current.hasFiles(true) {
			continue
		}

		interrupted, err := p.runLinter(cliArgs, current.Path+"/...")
		if err != nil {
			return err
		} else if !interrupted {
			continue
		}

		p.logger.Debugf("Spreading lint effort for '%s' over sub-directories.", current.Path)
		if current.hasFiles(false) {
			if _, err = p.runLinter(cliArgs, current.Path); err != nil {
				return err
			}
		}
		for _, subDir := range current.SubDirectories {
			todo = append(todo, subDir)
		}
	}
	return nil
}

func (p *parser) runLinter(cliArgs []string, path string) (bool, error) {
	p.logger.Debugf("Running linter on '%s'.", path)
	output, interrupted, err := p.runManagedLinter(append(cliArgs, path))
	if interrupted {
		p.logger.Debugf("Linter run was interrupted due to resource constraints.")
		return true, nil
	} else if err != nil {
		return false, err
	}

	lintOutput := &struct {
		Issues []*result.Issue
		Report *report.Data
	}{}
	if err = json.Unmarshal(output, lintOutput); err != nil {
		p.logger.WithError(err).Errorf("Could not parse linter output:\n%s", output)
		return false, err
	}

	// If not yet registered list all enabled linters.
	if len(p.project.linters) == 0 {
		for _, linter := range lintOutput.Report.Linters {
			if linter.Enabled {
				p.project.linters = append(p.project.linters, linter.Name)
			}
		}
		sort.Strings(p.project.linters)
	}

	p.logger.Debugf("Registering issues found on '%s'.", path)
	for _, issue := range lintOutput.Issues {
		p.project.addIssue(p.logger, issue)
	}
	return false, nil
}

var locCounterBufferSize = 32 * 1024

func locCounter(r io.Reader) (int, error) {
	buffer := make([]byte, locCounterBufferSize)

	var line string
	count := 0
	for {
		n, err := r.Read(buffer)
		if err != nil && err != io.EOF {
			return count, err
		}

		var idx int
		for idx < n {
			newIdx := bytes.IndexByte(buffer[idx:], '\n')
			if newIdx < 0 {
				line = strings.TrimSpace(string(buffer[idx:]))
				break
			}

			line += strings.TrimSpace(string(buffer[idx : idx+newIdx]))
			if line != "" && !strings.HasPrefix(line, "//") {
				count++
			}
			line = ""
			idx += newIdx + 1
		}

		if err == io.EOF {
			return count, nil
		}
	}
}

// The default method of monitoring memory usage uses a non-trivial strategy in order to satisfy the
// particular case of running 'golangci-lint', as well as doing so on varying platforms.
//
// 1. Running linters should be quick and not rely on swap space as that would slow down things
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
