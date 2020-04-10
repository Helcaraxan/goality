package printer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Helcaraxan/goality/lib/analysis"
	"github.com/Helcaraxan/goality/lib/report"
)

func Test_PrintView(t *testing.T) {
	project := testProject(t)

	expectedOutput := fmt.Sprintf(`Quality report for Go codebase located at '%s'

path           typecheck unused     
.              0  (0.00) 0 (0.00)   
bar            0  (0.00) 1 (250.00) 
foo            0  (0.00) 0 (0.00)   
foo/dir/...    0  (0.00) 1 (90.91)  
foo/non-go/... 0  (0.00) 0 (0.00)   

Data-format: total-issues (average issues per 1K LoC)
`, project.Path)

	view := project.GenerateView(report.WithDepth(2))

	w := &strings.Builder{}
	require.NoError(t, PrintView(w, view))
	assert.Equal(t, expectedOutput, w.String())
}

func Test_PrintCategories(t *testing.T) {
	project := testProject(t)

	expectedOutput := `occurrences linter issue                
2           unused func  <i....s unused 
`

	maxIssueTextWidth = 20
	categories := analysis.IssueRanking(project.GenerateView(), 0)

	w := &strings.Builder{}
	require.NoError(t, PrintCategories(w, categories))
	assert.Equal(t, expectedOutput, w.String())
}

var (
	cachedProject     *report.Project
	cachedProjectLock sync.Mutex
)

func testProject(t *testing.T) *report.Project {
	cachedProjectLock.Lock()
	defer cachedProjectLock.Unlock()

	if cachedProject == nil {
		logger := logrus.New()
		logger.SetOutput(ioutil.Discard)

		wd, err := os.Getwd()
		require.NoError(t, err)

		testProjectPath := filepath.Join(wd, "..", "report", "testdata", "project")
		cachedProject, err = report.Parse(
			logger,
			testProjectPath,
			report.WithLinters("unused", "typecheck"),
			report.WithExcludeDirs("my_exclude"),
		)
		require.NoError(t, err)
	}

	return cachedProject
}
