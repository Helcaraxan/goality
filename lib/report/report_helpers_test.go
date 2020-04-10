package report

import (
	"go/token"
	"testing"

	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/stretchr/testify/assert"
)

func Test_Fuse(t *testing.T) {
	views := []*SubView{
		{
			Path: "foo",
			Issues: map[string][]*result.Issue{
				"gofmt": {&result.Issue{
					FromLinter: "gofmt",
					Pos:        token.Position{Filename: "foo/foo.go"},
				}},
			},
			LineCount: 10,
		},
		nil,
		{
			Path: "bar",
			Issues: map[string][]*result.Issue{
				"gofmt": {&result.Issue{
					FromLinter: "gofmt",
					Pos:        token.Position{Filename: "bar/bar.go"},
				}},
				"govet": {&result.Issue{
					FromLinter: "govet",
					Pos:        token.Position{Filename: "bar/bar.go"},
				}},
			},
			LineCount: 20,
		},
	}
	expected := &SubView{
		Path: "",
		Issues: map[string][]*result.Issue{
			"gofmt": {
				&result.Issue{
					FromLinter: "gofmt",
					Pos:        token.Position{Filename: "foo/foo.go"},
				},
				&result.Issue{
					FromLinter: "gofmt",
					Pos:        token.Position{Filename: "bar/bar.go"},
				},
			},
			"govet": {&result.Issue{
				FromLinter: "govet",
				Pos:        token.Position{Filename: "bar/bar.go"},
			}},
		},
		LineCount: 30,
	}

	mergedViews := fuse(views...)
	assert.Equal(t, expected, mergedViews, "Should have retrieved the expected view.")
}

func Test_AggregateViewOpts(t *testing.T) {
	var (
		viewOptsA = &ViewOpts{}
		viewOptsB = &ViewOpts{paths: []string{"foo/bar"}}
		viewOptsC = &ViewOpts{
			paths: []string{
				"foo/bar/dir/subDir",
				"foo/bar",
			},
		}
		viewOptsD = &ViewOpts{depth: 3}
		viewOptsE = &ViewOpts{depth: 1}
		viewOptsF = &ViewOpts{
			depth: 2,
			paths: []string{
				"foo/bar/dir",
				"foo/bar/dir/subDir",
			},
		}
	)

	testcases := map[string]struct {
		viewOpts []*ViewOpts
		expected *ViewOpts
	}{
		"NoOpts": {expected: &ViewOpts{depth: -1}},
		"PathsOnly": {
			viewOpts: []*ViewOpts{viewOptsA, viewOptsB, viewOptsC},
			expected: &ViewOpts{
				paths: []string{
					"foo/bar",
					"foo/bar/dir/subDir",
				},
			},
		},
		"DepthsOnly": {
			viewOpts: []*ViewOpts{viewOptsD, viewOptsE},
			expected: &ViewOpts{depth: 1},
		},
		"DepthsAndPaths": {
			viewOpts: []*ViewOpts{viewOptsA, viewOptsC, viewOptsE, viewOptsF},
			expected: &ViewOpts{
				depth: 2,
				paths: []string{
					"foo/bar/dir",
					"foo/bar/dir/subDir",
				},
			},
		},
	}

	for name := range testcases {
		testcase := testcases[name]
		t.Run(name, func(t *testing.T) {
			viewOpt := aggregateViewOpts(testcase.viewOpts...)
			assert.Equal(t, testcase.expected, viewOpt, "Should have retrieved the expected aggregated ViewOpts.")
		})
	}
}
