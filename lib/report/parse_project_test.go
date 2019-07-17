package report

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Parse(t *testing.T) {
	parser := &parser{
		logger: logrus.New(),
		opts:   &LintOpts{excludeDirs: map[string]struct{}{"my_exclude": {}, "vendor": {}}},
	}

	project, err := parser.parse(filepath.Join("testdata", "project"))
	require.NoError(t, err, "Must be able to parse the project without errors.")
	assert.Equal(t, createParsedProject(), project, "Should have returned the expected project structure.")
}

func Test_Lint(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")

	project := createParsedProject()
	linter := &linter{
		logger: logrus.New(),
		opts:   &LintOpts{configPath: filepath.Join(cwd, "testdata", "project", ".golangci.yaml")},
	}

	err = linter.lint(project)
	require.NoError(t, err, "Must be able to lint the project without errors.")
	assert.Equal(t, createLintedProject(), project, "Should have found the expected linter issues.")
}

func Test_ResourceAwareness(t *testing.T) {
	logger := logrus.New()

	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")
	project := createParsedProject()
	linter := &linter{
		logger:         logger,
		opts:           &LintOpts{configPath: filepath.Join(cwd, "testdata", "project", ".golangci.yaml")},
		memoryMonitory: testMemoryMonitor,
	}

	err = linter.lint(project)
	require.NoError(t, err, "Must be able to lint the project without errors.")
	assert.Equal(t, createLintedProject(), project, "Should have found the expected linter issues.")
}

var hasBeenInterrupted bool

func testMemoryMonitor(logger *logrus.Logger, wg *sync.WaitGroup, done chan struct{}, interrupt chan struct{}) {
	if !hasBeenInterrupted {
		hasBeenInterrupted = true
		close(interrupt)
		wg.Done()
	} else {
		systemMemoryMonitor(logger, wg, done, interrupt)
	}
}
