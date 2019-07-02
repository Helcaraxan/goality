package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Parse(t *testing.T) {
	logger := logrus.New()

	cwd, err := os.Getwd()
	require.NoError(t, err, "Must be able to determine the current directory.")

	expectedProject := &Project{
		Path: filepath.Join(cwd, "testdata", "project"),
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
							Issues:    map[string][]*result.Issue{},
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
									Issues:    map[string][]*result.Issue{},
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
					Issues:    map[string][]*result.Issue{},
				},
			},
		},
	}

	parser := &parser{
		logger: logger,
		opts:   WithConfig(filepath.Join(cwd, "testdata", "project", ".golangci.yaml")),
	}

	err = parser.parse(filepath.Join("testdata", "project"))
	require.NoError(t, err, "Must be able to parse the project without errors.")
	assert.Equal(t, expectedProject, parser.project, "Should have returned the expected project structure.")

	err = parser.lint()
	require.NoError(t, err, "Must be able to lint the project without errors.")
	assert.Equal(t, fakeProject, parser.project, "Should have found the expected linter issues.")

	assert.Equal(t, fakeProject.root.SubDirectories["bar"], parser.project.root.SubDirectories["bar"])
	assert.Equal(t, fakeProject.root.SubDirectories["foo"], parser.project.root.SubDirectories["foo"])
	assert.Equal(t, fakeProject.root.Files["file.go"].Issues["govet"][0], parser.project.root.Files["file.go"].Issues["govet"][0])
}
