package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AggregateLintOpts(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")

	var (
		lintOptsA = &LintOpts{}
		lintOptsB = WithLinters("mylinter")
		lintOptsC = WithLinters("mylinter", "mystaticanalysis")
		lintOptsD = WithConfig(filepath.Join(cwd, "testdata", "project", ".golangci.yaml"))
		lintOptsE = WithConfig("bar.yaml")
		lintOptsF = WithExcludeDirs("vendor")
		lintOptsG = WithExcludeDirs("mocks", "vendor")
	)

	testcases := map[string]struct {
		lintOpts      []*LintOpts
		expectedErr   bool
		expectedValue *LintOpts
	}{
		"NoOpts": {expectedValue: &LintOpts{excludeDirs: map[string]struct{}{
			"builtin":     {},
			"examples":    {},
			"Godeps":      {},
			"testdata":    {},
			"third_party": {},
			"vendor":      {},
		}}},
		"LintersOnly": {
			lintOpts: []*LintOpts{lintOptsA, lintOptsB, lintOptsC},
			expectedValue: &LintOpts{
				linters: []string{"mylinter", "mystaticanalysis"},
				excludeDirs: map[string]struct{}{
					"builtin":     {},
					"examples":    {},
					"Godeps":      {},
					"testdata":    {},
					"third_party": {},
					"vendor":      {},
				},
			},
		},
		"ExcludePathsOnly": {
			lintOpts: []*LintOpts{lintOptsF, lintOptsG},
			expectedValue: &LintOpts{excludeDirs: map[string]struct{}{
				"builtin":     {},
				"examples":    {},
				"Godeps":      {},
				"mocks":       {},
				"testdata":    {},
				"third_party": {},
				"vendor":      {},
			}},
		},
		"OneConfig": {
			lintOpts: []*LintOpts{lintOptsA, lintOptsD},
			expectedValue: &LintOpts{
				configPath: lintOptsD.configPath,
				excludeDirs: map[string]struct{}{
					"builtin":     {},
					"examples":    {},
					"Godeps":      {},
					"my_exclude":  {},
					"testdata":    {},
					"third_party": {},
					"vendor":      {},
				},
			},
		},
		"TwoConfigs": {
			lintOpts:    []*LintOpts{lintOptsA, lintOptsD, lintOptsE},
			expectedErr: true,
		},
		"MultiOptions": {
			lintOpts: []*LintOpts{lintOptsA, lintOptsB, lintOptsC, lintOptsD, lintOptsF, lintOptsG},
			expectedValue: &LintOpts{
				linters:    []string{"mylinter", "mystaticanalysis"},
				configPath: lintOptsD.configPath,
				excludeDirs: map[string]struct{}{
					"builtin":     {},
					"examples":    {},
					"Godeps":      {},
					"mocks":       {},
					"my_exclude":  {},
					"testdata":    {},
					"third_party": {},
					"vendor":      {},
				},
			},
		},
	}

	for name := range testcases {
		testcase := testcases[name]
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
			expected: []string{"--no-config"},
		},
		"ConfigOnly": {
			lintOpts: &LintOpts{configPath: "bar.yaml"},
			expected: []string{"--config=bar.yaml"},
		},
		"LintersOnly": {
			lintOpts: &LintOpts{linters: []string{"mylinter", "mystaticanalysis"}},
			expected: []string{"--no-config", "--disable-all", "--enable=mylinter,mystaticanalysis"},
		},
		"ExcludePathsOnly": {
			lintOpts: &LintOpts{excludeDirs: map[string]struct{}{"mocks": {}, "vendor": {}}},
			expected: []string{"--no-config", "--skip-dirs=mocks,vendor"},
		},
		"MultiOptions": {
			lintOpts: &LintOpts{
				linters:     []string{"mystaticanalysis"},
				configPath:  "foo.yaml",
				excludeDirs: map[string]struct{}{"vendor": {}},
			},
			expected: []string{"--config=foo.yaml", "--disable-all", "--enable=mystaticanalysis", "--skip-dirs=vendor"},
		},
	}

	for name := range testcases {
		testcase := testcases[name]
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
