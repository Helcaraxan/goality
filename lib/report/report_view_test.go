package report

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fakeProject.Path = filepath.Join(cwd, "testdata", "project")
}

var (
	barUnusedIssue = &result.Issue{
		FromLinter: "unused",
		Text:       "U1000: func `unusedFunc` is unused",
		Pos: token.Position{
			Filename: "bar/file.go",
			Offset:   18,
			Line:     3,
			Column:   6,
		},
		SourceLines: []string{"func unusedFunc() (err error) {"},
	}
	fooDirGoVetIssue = &result.Issue{
		FromLinter: "govet",
		Text:       "shadow: declaration of \"myString\" shadows declaration at line 6",
		Pos: token.Position{
			Filename: "foo/dir/file.go",
			Offset:   102,
			Line:     8,
			Column:   3,
		},
		SourceLines: []string{"\t\tmyString := \"Let it snow!\""},
	}
	fooDirUnusedIssue = &result.Issue{
		FromLinter: "unused",
		Text:       "U1000: func `unworthy` is unused",
		Pos: token.Position{
			Filename: "foo/dir/file.go",
			Offset:   187,
			Line:     14,
			Column:   6,
		},
		SourceLines: []string{"func unworthy() {}"},
	}
	rootGoVetIssue = &result.Issue{
		FromLinter: "govet",
		Text:       "shadow: declaration of \"err\" shadows declaration at line 11",
		Pos: token.Position{
			Filename: "file.go",
			Offset:   206,
			Line:     19,
			Column:   3,
		},
		SourceLines: []string{"\t\terr := russianRoulette()"},
	}

	fakeProject = &Project{
		root: &Directory{
			Path: ".",
			SubDirectories: map[string]*Directory{
				"bar": {
					Path:           "bar",
					SubDirectories: map[string]*Directory{},
					Files: map[string]*File{
						"file.go": {
							Path:      "bar/file.go",
							LineCount: 4,
							Issues: map[string][]*result.Issue{
								"unused": {barUnusedIssue},
							},
						},
					},
				},
				"foo": {
					Path: "foo",
					SubDirectories: map[string]*Directory{
						"dir": {
							Path:           "foo/dir",
							SubDirectories: map[string]*Directory{},
							Files: map[string]*File{
								"file.go": {
									Path:      "foo/dir/file.go",
									LineCount: 11,
									Issues: map[string][]*result.Issue{
										"govet":  {fooDirGoVetIssue},
										"unused": {fooDirUnusedIssue},
									},
								},
							},
						},
					},
					Files: map[string]*File{},
				},
			},
			Files: map[string]*File{
				"file.go": {
					Path:      "file.go",
					LineCount: 32,
					Issues: map[string][]*result.Issue{
						"govet": {rootGoVetIssue},
					},
				},
			},
		},
	}
)

func Test_AddIssue(t *testing.T) {
	logger := logrus.New()

	project := &Project{root: &Directory{
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
				},
			},
		},
		Files: map[string]*File{},
	}}
	issue := &result.Issue{
		FromLinter: "mystaticanalysis",
		Pos:        token.Position{Filename: "bar/file.go"},
	}

	project.addIssue(logger, issue)
	require.Equal(t, []*result.Issue{issue}, project.root.SubDirectories["bar"].Files["file.go"].Issues["mystaticanalysis"])
}

func Test_Views(t *testing.T) {
	// Generate a global project view.
	view := fakeProject.GenerateView()
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
	view = fakeProject.GenerateView(WithPaths("foo/dir", "bar/file.go"))
	require.Len(t, view.SubViews, 2)
	require.Equal(t, &View{SubViews: map[string]*SubView{
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
	}}, view)

	// Generate a depth-specific view.
	view = fakeProject.GenerateView(WithDepth(1))
	require.Len(t, view.SubViews, 3)
	require.Equal(t, &View{SubViews: map[string]*SubView{
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
	}}, view)
}
