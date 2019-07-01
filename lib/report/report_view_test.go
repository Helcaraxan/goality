package report

import (
	"go/token"
	"io/ioutil"
	"testing"

	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var fakeProject = &Project{
	Path: "/my/project",
	root: &Directory{
		Path: ".",
		SubDirectories: map[string]*Directory{
			"foo": {
				Path: "foo",
				SubDirectories: map[string]*Directory{
					"dir": {
						Path:           "foo/dir",
						SubDirectories: map[string]*Directory{},
						Files: map[string]*File{
							"file.go": {
								Path:      "file.go",
								LineCount: 30,
								Issues: map[string][]*result.Issue{
									"mylinter": {
										{
											FromLinter: "mylinter",
											Pos:        token.Position{Filename: "foo/dir/file.go"},
										},
										{
											FromLinter: "mylinter",
											Pos:        token.Position{Filename: "foo/dir/file.go"},
										},
									},
									"mystaticanalysis": {
										{
											FromLinter: "mystaticanalysis",
											Pos:        token.Position{Filename: "foo/dir/file.go"},
										},
									},
								},
							},
						},
					},
				},
				Files: map[string]*File{},
			},
			"bar": {
				Path:           "bar",
				SubDirectories: map[string]*Directory{},
				Files: map[string]*File{
					"file.go": {
						Path:      "bar/file.go",
						LineCount: 15,
						Issues:    map[string][]*result.Issue{},
					},
				},
			},
		},
		Files: map[string]*File{
			"file.go": {
				Path:      "file.go",
				LineCount: 10,
				Issues: map[string][]*result.Issue{
					"mylinter": {
						{
							FromLinter: "mylinter",
							Pos:        token.Position{Filename: "file.go"},
						},
					},
				},
			},
		},
	},
}

func Test_Views(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)

	// Add a new issue.
	issue := &result.Issue{
		FromLinter: "mystaticanalysis",
		Pos:        token.Position{Filename: "bar/file.go"},
	}

	fakeProject.addIssue(logger, issue)
	require.Equal(t, []*result.Issue{issue}, fakeProject.root.SubDirectories["bar"].Files["file.go"].Issues["mystaticanalysis"])

	// Generate a global project view.
	view := fakeProject.GenerateView()
	require.Len(t, view.SubViews, 1)
	require.Equal(t, &SubView{
		Path:      "./...",
		LineCount: 55,
		Issues: map[string][]*result.Issue{
			"mylinter": {
				{
					FromLinter: "mylinter",
					Pos:        token.Position{Filename: "file.go"},
				},
				{
					FromLinter: "mylinter",
					Pos:        token.Position{Filename: "foo/dir/file.go"},
				},
				{
					FromLinter: "mylinter",
					Pos:        token.Position{Filename: "foo/dir/file.go"},
				},
			},
			"mystaticanalysis": {
				{
					FromLinter: "mystaticanalysis",
					Pos:        token.Position{Filename: "bar/file.go"},
				},
				{
					FromLinter: "mystaticanalysis",
					Pos:        token.Position{Filename: "foo/dir/file.go"},
				},
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
			LineCount: 15,
			Issues: map[string][]*result.Issue{
				"mystaticanalysis": {
					{
						FromLinter: "mystaticanalysis",
						Pos:        token.Position{Filename: "bar/file.go"},
					},
				},
			},
		},
		"foo/dir/...": {
			Path:      "foo/dir/...",
			LineCount: 30,
			Issues: map[string][]*result.Issue{
				"mylinter": {
					{
						FromLinter: "mylinter",
						Pos:        token.Position{Filename: "foo/dir/file.go"},
					},
					{
						FromLinter: "mylinter",
						Pos:        token.Position{Filename: "foo/dir/file.go"},
					},
				},
				"mystaticanalysis": {
					{
						FromLinter: "mystaticanalysis",
						Pos:        token.Position{Filename: "foo/dir/file.go"},
					},
				},
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
			LineCount: 10,
			Issues: map[string][]*result.Issue{
				"mylinter": {
					{
						FromLinter: "mylinter",
						Pos:        token.Position{Filename: "file.go"},
					},
				},
			},
		},
		"bar/...": {
			Path:      "bar/...",
			LineCount: 15,
			Issues: map[string][]*result.Issue{
				"mystaticanalysis": {
					{
						FromLinter: "mystaticanalysis",
						Pos:        token.Position{Filename: "bar/file.go"},
					},
				},
			},
			recursive: true,
		},
		"foo/...": {
			Path:      "foo/...",
			LineCount: 30,
			Issues: map[string][]*result.Issue{
				"mylinter": {
					{
						FromLinter: "mylinter",
						Pos:        token.Position{Filename: "foo/dir/file.go"},
					},
					{
						FromLinter: "mylinter",
						Pos:        token.Position{Filename: "foo/dir/file.go"},
					},
				},
				"mystaticanalysis": {
					{
						FromLinter: "mystaticanalysis",
						Pos:        token.Position{Filename: "foo/dir/file.go"},
					},
				},
			},
			recursive: true,
		},
	}}, view)
}
