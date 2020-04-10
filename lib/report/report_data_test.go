package report

import (
	"go/token"
	"os"
	"path/filepath"

	"github.com/golangci/golangci-lint/pkg/result"
)

var (
	barUnusedIssue = &result.Issue{
		FromLinter: "unused",
		Text:       "func `unusedFunc` is unused",
		Pos: token.Position{
			Filename: "bar/file.go",
			Offset:   18,
			Line:     3,
			Column:   6,
		},
		Replacement: &result.Replacement{
			NeedOnlyDelete: true,
		},
		LineRange: &result.Range{
			From: 3,
			To:   0,
		},
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
		Text:       "func `unworthy` is unused",
		Pos: token.Position{
			Filename: "foo/dir/file.go",
			Offset:   187,
			Line:     14,
			Column:   6,
		},
		Replacement: &result.Replacement{
			NeedOnlyDelete: true,
		},
		LineRange: &result.Range{
			From: 14,
			To:   0,
		},
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

	linters = []string{
		"errcheck",
		"gofmt",
		"goimports",
		"golint",
		"govet",
		"misspell",
		"nakedret",
		"unparam",
		"unused",
	}
)

func createParsedProject() *Project {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return &Project{
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
						"non-go": {
							Path:           "foo/non-go",
							SubDirectories: map[string]*Directory{},
							Files:          map[string]*File{},
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
}

func createLintedProject() *Project {
	project := createParsedProject()
	project.linters = linters

	project.root.SubDirectories["bar"].Files["file.go"].Issues["unused"] = []*result.Issue{barUnusedIssue}
	project.root.SubDirectories["foo"].SubDirectories["dir"].Files["file.go"].Issues["govet"] = []*result.Issue{fooDirGoVetIssue}
	project.root.SubDirectories["foo"].SubDirectories["dir"].Files["file.go"].Issues["unused"] = []*result.Issue{fooDirUnusedIssue}
	project.root.Files["file.go"].Issues["govet"] = []*result.Issue{rootGoVetIssue}

	return project
}
