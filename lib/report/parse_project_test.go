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
	parser := &parser{logger: logrus.New()}

	err := parser.parse(filepath.Join("testdata", "project"))
	require.NoError(t, err, "Must be able to parse the project without errors.")
	assert.Equal(t, createParsedProject(), parser.project, "Should have returned the expected project structure.")
}

func Test_Lint(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")

	parser := &parser{
		logger:  logrus.New(),
		project: createParsedProject(),
		opts:    WithConfig(filepath.Join(cwd, "testdata", "project", ".golangci.yaml")),
	}

	err = parser.lint()
	require.NoError(t, err, "Must be able to lint the project without errors.")
	assert.Equal(t, createLintedProject(), parser.project, "Should have found the expected linter issues.")
}

func Test_ResourceAwareness(t *testing.T) {
	// We use a specialised memory monitor function in order to provoke a shutdown of the linter
	// that's being run as part of the test at an arbitrary point.
	var hasBeenInterrupted bool
	testingMemoryMonitor := func(logger *logrus.Logger, wg *sync.WaitGroup, done chan struct{}, interrupt chan struct{}) {
		if hasBeenInterrupted {
			systemMemoryMonitor(logger, wg, done, interrupt)
		} else {
			hasBeenInterrupted = true
			close(interrupt)
			wg.Done()
		}
	}

	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")
	parser := &parser{
		logger:        logrus.New(),
		project:       createParsedProject(),
		opts:          WithConfig(filepath.Join(cwd, "testdata", "project", ".golangci.yaml")),
		memoryMonitor: testingMemoryMonitor,
	}

	err = parser.lint()
	require.NoError(t, err, "Must be able to lint the project without errors.")
	assert.Equal(t, createLintedProject(), parser.project, "Should have found the expected linter issues.")
}
