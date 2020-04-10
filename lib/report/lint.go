package report

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/golangci/golangci-lint/pkg/report"
	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
)

type memoryMonitorFunc func(*logrus.Logger, *sync.WaitGroup, chan struct{}, chan struct{})

type linter struct {
	logger *logrus.Logger
	opts   *LintOpts

	// This field must only be used for testing.
	memoryMonitory memoryMonitorFunc
}

func (l *linter) lint(project *Project) error {
	cliArgs := append([]string{
		"run",
		"--issues-exit-code=0",
		"--max-issues-per-linter=0",
		"--max-same-issues=0",
		"--new=false",
		"--new-from-rev=",
		"--out-format=json",
	}, l.opts.toArgs()...)

	todo := []*Directory{project.root}
	for len(todo) > 0 {
		current := todo[0]
		todo = todo[1:]

		if !current.hasFiles(true) {
			continue
		}

		interrupted, err := l.runLinter(project, cliArgs, current.Path+"/...")
		if err != nil {
			return err
		} else if !interrupted {
			continue
		}

		l.logger.Debugf("Spreading lint effort for '%s' over sub-directories.", current.Path)

		if current.hasFiles(false) {
			if _, err = l.runLinter(project, cliArgs, current.Path); err != nil {
				return err
			}
		}

		for _, subDir := range current.SubDirectories {
			todo = append(todo, subDir)
		}
	}

	return nil
}

func (l *linter) runLinter(project *Project, cliArgs []string, path string) (bool, error) {
	l.logger.Debugf("Running linter on '%s'.", path)

	output, interrupted, err := l.runManagedLinter(project, append(cliArgs, path))
	if interrupted {
		l.logger.Debugf("Linter run was interrupted due to resource constraints.")
		return true, nil
	} else if err != nil {
		return false, err
	}

	lintOutput := &struct {
		Issues []*result.Issue
		Report *report.Data
	}{}
	if err = json.Unmarshal(output, lintOutput); err != nil {
		l.logger.WithError(err).Errorf("Could not parse linter output:\n%s", output)
		return false, err
	}

	// If not yet registered list all enabled linters.
	if len(project.linters) == 0 {
		for _, linter := range lintOutput.Report.Linters {
			if linter.Enabled {
				project.linters = append(project.linters, linter.Name)
			}
		}

		sort.Strings(project.linters)
	}

	l.logger.Debugf("Registering issues found on '%s'.", path)

	for _, issue := range lintOutput.Issues {
		project.addIssue(l.logger, issue)
	}

	return false, nil
}

func (l *linter) runManagedLinter(project *Project, cliArgs []string) ([]byte, bool, error) {
	runner := newRunner(l.logger, cliArgs)

	stdout, stderr := &bytes.Buffer{}, &strings.Builder{}
	runner.cmd.Stdout, runner.cmd.Stderr = stdout, stderr
	runner.cmd.Dir = project.Path

	interrupted, err := runner.run()
	if err != nil && !interrupted {
		l.logger.WithError(err).Errorf("Linter exited with an error. Output was:\n%s Error was:\n%s", stdout.Bytes(), stderr.String())
		return nil, false, err
	}

	return stdout.Bytes(), interrupted, nil
}
