package report

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shirou/gopsutil/mem"
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
	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")
	parser := &parser{
		logger:  logrus.New(),
		project: createParsedProject(),
		opts:    WithConfig(filepath.Join(cwd, "testdata", "project", ".golangci.yaml")),
	}

	// We replace the memory usage getter by a custom one that ensures that we
	// interrupt the first linter run. This can obviously not be guaranteed and
	// is dependent on the machine and OS but it's the best we can get without
	// significantly complicating the flow.
	formerGetMemoryUsage := getMemoryUsage
	defer func() { getMemoryUsage = formerGetMemoryUsage }()

	var interrupted bool
	getMemoryUsage = func(ctx context.Context) (*mem.VirtualMemoryStat, error) {
		if !interrupted {
			interrupted = true
			return &mem.VirtualMemoryStat{UsedPercent: 95.0}, nil
		}
		return mem.VirtualMemoryWithContext(ctx)
	}

	err = parser.lint()
	require.NoError(t, err, "Must be able to lint the project without errors.")
	assert.Equal(t, createLintedProject(), parser.project, "Should have found the expected linter issues.")
}
