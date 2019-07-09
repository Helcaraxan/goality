package report

import (
	"go/token"
	"testing"

	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_AddIssue(t *testing.T) {
	logger := logrus.New()

	project := &Project{
		root: &Directory{
			Path: ".",
			SubDirectories: map[string]*Directory{
				"bar": {
					Path:           "bar",
					SubDirectories: map[string]*Directory{},
					Files: map[string]*File{
						"file.go": {
							Path:   "bar/file.go",
							Issues: map[string][]*result.Issue{},
						},
						"non-go": {
							Path:   "foo/non-go",
							Issues: map[string][]*result.Issue{},
						},
					},
				},
			},
			Files: map[string]*File{},
		},
	}
	issue := &result.Issue{
		FromLinter: "mystaticanalysis",
		Pos:        token.Position{Filename: "bar/file.go"},
	}

	project.addIssue(logger, issue)
	require.Equal(t, []*result.Issue{issue}, project.root.SubDirectories["bar"].Files["file.go"].Issues["mystaticanalysis"])
}

func Test_Views(t *testing.T) {
	project := createLintedProject()
	// Generate a global project view.
	view := project.GenerateView()
	require.Len(t, view.SubViews, 1)
	require.Equal(t, &SubView{
		Path:      "./...",
		LineCount: 47,
		Issues: map[string][]*result.Issue{
			"govet": {
				rootGoVetIssue,
				fooDirGoVetIssue,
			},
			"unused": {
				barUnusedIssue,
				fooDirUnusedIssue,
			},
		},
		recursive: true,
	}, view.SubViews["./..."])

	// Generate a path-specific view.
	view = project.GenerateView(WithPaths("foo/dir", "bar/file.go"))
	require.Len(t, view.SubViews, 2)
	require.Equal(t, &View{
		Path: project.Path,
		SubViews: map[string]*SubView{
			"bar/file.go": {
				Path:      "bar/file.go",
				LineCount: 4,
				Issues: map[string][]*result.Issue{
					"unused": {barUnusedIssue},
				},
			},
			"foo/dir/...": {
				Path:      "foo/dir/...",
				LineCount: 11,
				Issues: map[string][]*result.Issue{
					"govet":  {fooDirGoVetIssue},
					"unused": {fooDirUnusedIssue},
				},
				recursive: true,
			},
		},
		Linters: linters,
	}, view)

	// Generate a depth-specific view.
	view = project.GenerateView(WithDepth(1))
	require.Len(t, view.SubViews, 3)
	require.Equal(t, &View{
		Path: project.Path,
		SubViews: map[string]*SubView{
			".": {
				Path:      ".",
				LineCount: 32,
				Issues: map[string][]*result.Issue{
					"govet": {rootGoVetIssue},
				},
			},
			"bar/...": {
				Path:      "bar/...",
				LineCount: 4,
				Issues: map[string][]*result.Issue{
					"unused": {barUnusedIssue},
				},
				recursive: true,
			},
			"foo/...": {
				Path:      "foo/...",
				LineCount: 11,
				Issues: map[string][]*result.Issue{
					"govet":  {fooDirGoVetIssue},
					"unused": {fooDirUnusedIssue},
				},
				recursive: true,
			},
		},
		Linters: linters,
	}, view)
}
