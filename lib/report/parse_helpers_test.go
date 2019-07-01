package report

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_AggregateLintOpts(t *testing.T) {
	var (
		lintOptsA = &LintOpts{}
		lintOptsB = &LintOpts{linters: []string{"mylinter"}}
		lintOptsC = &LintOpts{
			linters: []string{
				"mylinter",
				"mystaticanalysis",
			},
		}
		lintOptsD = &LintOpts{configPath: "foo.yaml"}
		lintOptsE = &LintOpts{configPath: "bar.yaml"}
	)

	testcases := map[string]struct {
		lintOpts      []*LintOpts
		expectedErr   bool
		expectedValue *LintOpts
	}{
		"NoOpts": {expectedValue: &LintOpts{}},
		"LintersOnly": {
			lintOpts:      []*LintOpts{lintOptsA, lintOptsB, lintOptsC},
			expectedValue: &LintOpts{linters: []string{"mylinter", "mystaticanalysis"}},
		},
		"OneConfig": {
			lintOpts:      []*LintOpts{lintOptsA, lintOptsD},
			expectedValue: lintOptsD,
		},
		"TwoConfigs": {
			lintOpts:    []*LintOpts{lintOptsA, lintOptsD, lintOptsE},
			expectedErr: true,
		},
		"LintersAndConfig": {
			lintOpts: []*LintOpts{lintOptsA, lintOptsB, lintOptsC, lintOptsE},
			expectedValue: &LintOpts{
				linters:    []string{"mylinter", "mystaticanalysis"},
				configPath: "bar.yaml",
			},
		},
	}

	for name, testcase := range testcases {
		t.Run(name, func(t *testing.T) {
			lintOpt, err := aggregateLintOpts(testcase.lintOpts...)
			if testcase.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, lintOpt)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testcase.expectedValue, lintOpt)
			}
		})
	}
}

func Test_LintOptsToArgs(t *testing.T) {
	testcases := map[string]struct {
		lintOpts *LintOpts
		expected []string
	}{
		"NoOpts": {
			lintOpts: &LintOpts{},
			expected: nil,
		},
		"ConfigOnly": {
			lintOpts: &LintOpts{configPath: "bar.yaml"},
			expected: []string{"--config=bar.yaml"},
		},
		"LintersOnly": {
			lintOpts: &LintOpts{linters: []string{"mylinter", "mystaticanalysis"}},
			expected: []string{"--no-config", "--disable-all", "--enable=mylinter,mystaticanalysis"},
		},
		"ConfigAndLinters": {
			lintOpts: &LintOpts{
				linters:    []string{"mystaticanalysis"},
				configPath: "foo.yaml",
			},
			expected: []string{"--config=foo.yaml", "--disable-all", "--enable=mystaticanalysis"},
		},
	}

	for name, testcase := range testcases {
		t.Run(name, func(t *testing.T) {
			cliArgs := testcase.lintOpts.toArgs()
			assert.Equal(t, testcase.expected, cliArgs)
		})
	}
}

func Test_LoCCounter(t *testing.T) {
	const fakeFile = `// Test file

package fake

// Super cool main function.
func main() {
	/* Guess we're not doing anything */
	os.Exit(1)
}
`

	locCounterBufferSize = 32

	lines, err := locCounter(strings.NewReader(fakeFile))
	assert.NoError(t, err)
	assert.Equal(t, 5, lines)

	locCounterBufferSize = 32 * 1024
}
