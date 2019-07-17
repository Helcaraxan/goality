package report

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golangci/golangci-lint/pkg/config"
	"github.com/golangci/golangci-lint/pkg/logutils"
	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
)

// These dirs excluded by default reflect the default excludes of 'golangci-lint'.
var defaultExcludeDirs = []string{
	"builtin",
	"Godeps",
	"examples",
	"testdata",
	"third_party",
	"vendor",
}

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
	project, err := parser.parse(path)
	if err != nil {
		return nil, err
	}

	linter := &linter{
		logger: logger,
		opts:   opt,
	}
	logger.Infof("Linting project at path %q.", path)
	if err = linter.lint(project); err != nil {
		return nil, err
	}
	return project, nil
}

type LintOpts struct {
	linters     []string
	configPath  string
	excludeDirs map[string]struct{}
}

func WithLinters(linters ...string) *LintOpts {
	return &LintOpts{
		linters:     linters,
		excludeDirs: map[string]struct{}{},
	}
}

func WithConfig(configFilePath string) *LintOpts {
	return &LintOpts{
		configPath:  configFilePath,
		excludeDirs: map[string]struct{}{},
	}
}

func WithExcludeDirs(dirsToExclude ...string) *LintOpts {
	lintOpts := &LintOpts{excludeDirs: map[string]struct{}{}}
	for idx := range dirsToExclude {
		lintOpts.excludeDirs[dirsToExclude[idx]] = struct{}{}
	}
	return lintOpts
}

func (o *LintOpts) mergeLintOpts(optsToMerge *LintOpts) error {
	if o.configPath != "" && optsToMerge.configPath != "" {
		return fmt.Errorf("conflicting options: multiple configuration files were specified: '%s' and '%s'", o.configPath, optsToMerge.configPath)
	} else if optsToMerge.configPath != "" {
		o.configPath = optsToMerge.configPath
	}

	var lastLinter string
	var newLinters []string
	o.linters = append(o.linters, optsToMerge.linters...)
	sort.Strings(o.linters)
	for idx := range o.linters {
		if o.linters[idx] != lastLinter {
			newLinters = append(newLinters, o.linters[idx])
			lastLinter = o.linters[idx]
		}
	}
	o.linters = newLinters

	for excludeDir := range optsToMerge.excludeDirs {
		o.excludeDirs[excludeDir] = struct{}{}
	}
	return nil
}

func aggregateLintOpts(opts ...*LintOpts) (*LintOpts, error) {
	accumulator := &LintOpts{excludeDirs: map[string]struct{}{}}
	for idx := range defaultExcludeDirs {
		accumulator.excludeDirs[defaultExcludeDirs[idx]] = struct{}{}
	}
	for idx := range opts {
		if err := accumulator.mergeLintOpts(opts[idx]); err != nil {
			return nil, err
		}
	}

	if accumulator.configPath != "" {
		fakeConfig := config.Config{Run: config.Run{Config: accumulator.configPath}}
		fakeLogger := logutils.NewStderrLog("config-parser")
		fakeLogger.SetLevel(logutils.LogLevelError)

		parsedConfig := config.Config{}
		configReader := config.NewFileReader(&parsedConfig, &fakeConfig, fakeLogger)
		if err := configReader.Read(); err != nil {
			return nil, fmt.Errorf("failed to read or parse config from '%s': %v", accumulator.configPath, err)
		}

		for _, excludeDir := range parsedConfig.Run.SkipDirs {
			accumulator.excludeDirs[excludeDir] = struct{}{}
		}
	}
	return accumulator, nil
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

	if len(o.excludeDirs) > 0 {
		var excludeList []string
		for excludeDir := range o.excludeDirs {
			excludeList = append(excludeList, excludeDir)
		}
		sort.Strings(excludeList)
		args = append(args, "--skip-dirs="+strings.Join(excludeList, ","))
	}
	return args
}

type parser struct {
	logger *logrus.Logger
	opts   *LintOpts
}

func (p *parser) parse(path string) (*Project, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			p.logger.WithError(err).Error("Could not determine the current working directory.")
			return nil, err
		}
		path = filepath.Join(cwd, path)
	}

	cwd, err := os.Getwd()
	if err != nil {
		p.logger.WithError(err).Errorf("Could not determine the current working directory.")
		return nil, err
	}
	if err = os.Chdir(path); err != nil {
		p.logger.WithError(err).Errorf("Could not move to the project's root directory.")
		return nil, err
	}
	defer func() {
		if moveErr := os.Chdir(cwd); moveErr != nil {
			p.logger.WithError(err).Errorf("Failed to move back to the original working directory %q.", cwd)
		}
	}()

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
			if _, ok := p.opts.excludeDirs[dirContent.Name()]; ok {
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
