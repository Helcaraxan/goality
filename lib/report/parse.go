package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

	logger.Infof("Parsing project at path %q.", path)
	parser.project, err = parser.parseProject(path)
	if err != nil {
		return nil, err
	}
	logger.Infof("Linting project at path %q.", path)
	if err = parser.lint(); err != nil {
		return nil, err
	}
	return parser.project, nil
}

type LintOpts struct {
	linters    []string
	configPath string
}

func WithLinters(linters ...string) *LintOpts {
	return &LintOpts{linters: linters}
}

func WithConfig(configFilePath string) *LintOpts {
	return &LintOpts{configPath: configFilePath}
}

func aggregateLintOpts(opts ...*LintOpts) (*LintOpts, error) {
	var configPaths, linters []string
	for _, opt := range opts {
		linters = append(linters, opt.linters...)
		if opt.configPath != "" {
			configPaths = append(configPaths, opt.configPath)
		}
	}
	if len(configPaths) > 1 {
		return nil, fmt.Errorf("conflicting options: multiple configuration files were specified: %v", configPaths)
	}

	aggregate := &LintOpts{}

	var lastLinter string
	sort.Strings(linters)
	for _, linter := range linters {
		if linter != lastLinter {
			aggregate.linters = append(aggregate.linters, linter)
			lastLinter = linter
		}
	}
	if len(configPaths) == 1 {
		aggregate.configPath = configPaths[0]
	}
	return aggregate, nil
}

func (o *LintOpts) toArgs() []string {
	var args []string
	if o.configPath == "" && len(o.linters) > 0 {
		args = append(args, "--no-config")
	} else if o.configPath != "" {
		args = append(args, "--config="+o.configPath)
	}

	if len(o.linters) != 0 {
		args = append(args, "--disable-all", "--enable="+strings.Join(o.linters, ","))
	}
	return args
}

type parser struct {
	logger  *logrus.Logger
	project *Project
	opts    *LintOpts
}

func (p *parser) parseProject(path string) (*Project, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			p.logger.WithError(err).Error("Could not determine the current working directory.")
			return nil, err
		}
		path = filepath.Join(cwd, path)
	}

	if err := os.Chdir(path); err != nil {
		p.logger.WithError(err).Errorf("Could not move to the project's root directory.")
		return nil, err
	}

	root, err := p.parseDirectory(".")
	if err != nil {
		return nil, err
	}
	return &Project{
		Path: path,
		root: root,
	}, nil
}

func (p *parser) parseDirectory(path string) (*Directory, error) {
	excludedDirNames := map[string]struct{}{
		"mocks":  {},
		"vendor": {},
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
		"--new=false",
		"--new-from-rev=",
		"--out-format=json",
	}, p.opts.toArgs()...)

	todo := []*Directory{p.project.root}
	for len(todo) > 0 {
		current := todo[0]
		todo = todo[1:]

		interrupted, err := p.runLint(append(cliArgs, current.Path+"/..."))
		if err != nil {
			return err
		} else if !interrupted {
			continue
		}

		if _, err = p.runLint(append(cliArgs, current.Path)); err != nil {
			return err
		}
		for _, subDir := range current.SubDirectories {
			todo = append(todo, subDir)
		}
	}
	return nil
}

func (p *parser) runLint(cliArgs []string) (bool, error) {
	output, interrupted, err := p.runManagedLint(cliArgs)
	if interrupted {
		return true, nil
	} else if err != nil {
		return false, err
	}

	lintOutput := &struct{ Issues []*result.Issue }{}
	if err = json.Unmarshal(output, lintOutput); err != nil {
		p.logger.WithError(err).Error("Could not parse linter output.")
		return false, err
	}

	for _, issue := range lintOutput.Issues {
		p.project.addIssue(p.logger, issue)
	}
	return false, nil
}

func (p *parser) runManagedLint(cliArgs []string) ([]byte, bool, error) {
	var interrupted bool

	lintCmd := exec.Command("golangci-lint", cliArgs...)
	output, errs := &bytes.Buffer{}, &strings.Builder{}
	lintCmd.Stdout, lintCmd.Stderr = output, errs

	done, interrupt := make(chan struct{}), make(chan struct{})
	go p.memoryMonitor(done, interrupt)

	if err := lintCmd.Start(); err != nil {
		p.logger.WithError(err).Error("Unable to run linter.")
		return nil, false, err
	}
	go func() {
		select {
		case <-interrupt:
			if err := lintCmd.Process.Signal(os.Kill); err != nil {
				p.logger.WithError(err).Error("Failed to interrupt linter with a KILL signal.")
			}
			interrupted = true
		case <-done:
		}
	}()

	if err := lintCmd.Wait(); err != nil {
		p.logger.WithError(err).Errorf("Linter exited with an error. Output was:\n%s Error was:\n%s", output.Bytes(), errs.String())
		return nil, interrupted, err
	}
	close(done)

	return output.Bytes(), interrupted, nil
}

func (p *parser) memoryMonitor(done chan struct{}, interrupt chan struct{}) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		memStat, err := mem.VirtualMemoryWithContext(ctx)
		cancel()
		if err != nil {
			p.logger.WithError(err).Debugf("Failed to retrieve memory usage.")
		}
		p.logger.Debugf("Memory usage: %.2f%%.", memStat.UsedPercent)
		if memStat.UsedPercent > 90.0 {
			close(interrupt)
		}

		select {
		case <-done:
			close(interrupt)
			return
		case <-time.After(1 * time.Second):
		}
	}
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
