package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golangci/golangci-lint/pkg/result"
	"github.com/sirupsen/logrus"
)

// View represents aggregated lint results.
type View struct {
	SubViews map[string]*SubView
}

// SubView represents the aggregated lint results for a single directory and it's subtree.
type SubView struct {
	Path      string
	Issues    map[string][]*result.Issue
	LineCount int

	recursive bool
}

func (s *SubView) String() string {
	printer := &strings.Builder{}
	fmt.Fprintf(printer, "Analysis for %s covering %.2fk lines of code.\n\nLinters:\n", s.Path, float32(s.LineCount)/1000)
	var issueCount int
	for linterName, issues := range s.Issues {
		issueCount += len(issues)
		fmt.Fprintf(printer, "- %s: %.2f issues / 1k LoC\n", linterName, float32(len(issues))/float32(s.LineCount)*1000)
	}
	fmt.Fprintf(printer, "\nTotal number of issues: %d\n", issueCount)
	return printer.String()
}

// ViewOpts contains options for generating a View.
type ViewOpts struct {
	depth int
	paths []string
}

// WithDepth generates a View containing SubViews rooted at directories at the specified depth.
func WithDepth(depth int) *ViewOpts {
	return &ViewOpts{depth: depth}
}

// WithPaths generates a View containing SubViews rooted at the specified paths.
func WithPaths(paths ...string) *ViewOpts {
	return &ViewOpts{
		depth: -1,
		paths: paths,
	}
}

// Project represents the analysis results of a single linter run on a directory tree.
type Project struct {
	Path string

	root *Directory
}

// Directory returns the information for the directory located at the given relative path in the
// project (if any exists).
func (p *Project) Directory(path string) *Directory {
	return p.root.getDirectory(strings.Split(filepath.Clean(path), string(os.PathSeparator)))
}

// SubView returns the aggregate linter results for the directory located at the given relative path
// in the project (if any exists).
func (p *Project) SubView(path string) *SubView {
	return p.root.subViewPath(strings.Split(filepath.Clean(path), string(os.PathSeparator)))
}

// GenerateView returns the aggregated analysis report for the sub-tree of the project rooted at the
// specified path.
func (p *Project) GenerateView(opts ...*ViewOpts) *View {
	opt := aggregateViewOpts(opts...)

	var subViews []*SubView
	if opt.depth >= 0 || len(opt.paths) == 0 {
		subViews = append(subViews, p.root.subViewDepth(opt.depth)...)
	}
	for _, path := range opt.paths {
		subViews = append(subViews, p.SubView(path))
	}

	view := &View{SubViews: map[string]*SubView{}}
	for _, subView := range subViews {
		view.SubViews[subView.Path] = subView
	}
	return view
}

// Directory contains the analysis results both full and aggregated for the sub-tree rooted at this
// directory.
type Directory struct {
	Path           string
	SubDirectories map[string]*Directory
	Files          map[string]*File

	// Cached instance of the report for this folder to prevent re-computation.
	recursiveView *SubView
	selfView      *SubView
}

// File represents the analysis results for a single given file.
type File struct {
	Path      string
	LineCount int
	Issues    map[string][]*result.Issue
}

// GenerateDirectory returns the Directory found at the given path (if any).
func (d *Directory) getDirectory(path []string) *Directory {
	if len(path) == 0 || path[0] == "." {
		return d
	}
	subDir, ok := d.SubDirectories[path[0]]
	if !ok {
		return nil
	}
	return subDir.getDirectory(path[1:])
}

func (d *Directory) subViewDepth(depth int) []*SubView {
	if depth <= 0 {
		return []*SubView{d.subViewRecursive()}
	}

	views := []*SubView{d.subViewSelf()}
	for _, subDir := range d.SubDirectories {
		views = append(views, subDir.subViewDepth(depth-1)...)
	}
	return views
}

func (d *Directory) subViewPath(path []string) *SubView {
	if len(path) > 0 {
		subDir, ok := d.SubDirectories[path[0]]
		if ok {
			return subDir.subViewPath(path[1:])
		}

		file, ok := d.Files[path[0]]
		if ok {
			return file.subView()
		}
		return nil
	}
	return d.subViewRecursive()
}

func (d *Directory) subViewRecursive() *SubView {
	if d.recursiveView == nil {
		childReports := []*SubView{d.subViewSelf()}
		for _, d := range d.SubDirectories {
			childReports = append(childReports, d.subViewRecursive())
		}
		d.recursiveView = fuse(childReports...)
		d.recursiveView.Path = d.Path + "/..."
		d.recursiveView.recursive = true

		for _, issues := range d.recursiveView.Issues {
			sort.Sort(sortableIssues(issues))
		}
	}
	return d.recursiveView
}

func (d *Directory) subViewSelf() *SubView {
	if d.selfView == nil {
		var childReports []*SubView
		for _, f := range d.Files {
			childReports = append(childReports, f.subView())
		}
		d.selfView = fuse(childReports...)
		d.selfView.Path = d.Path

		for _, issues := range d.selfView.Issues {
			sort.Sort(sortableIssues(issues))
		}
	}
	return d.selfView
}

func (p *Project) addIssue(logger *logrus.Logger, issue *result.Issue) {
	if p.root == nil {
		p.root = &Directory{}
	}
	p.root.addIssue(logger, strings.Split(issue.FilePath(), string(os.PathSeparator)), issue)
}

func (d *Directory) addIssue(logger *logrus.Logger, path []string, issue *result.Issue) {
	if len(path) > 1 {
		subDir, ok := d.SubDirectories[path[0]]
		if ok {
			subDir.addIssue(logger, path[1:], issue)
			return
		}
	}

	file, ok := d.Files[path[0]]
	if ok {
		file.addIssue(issue)
		return
	}
	logger.Warnf("Entry %q for issue %+v does not exist in %q.", path[0], issue, d.Path)
}

func (f *File) subView() *SubView {
	return &SubView{
		Path:      f.Path,
		Issues:    f.Issues,
		LineCount: f.LineCount,
	}
}

func (f *File) addIssue(issue *result.Issue) {
	f.Issues[issue.FromLinter] = append(f.Issues[issue.FromLinter], issue)
}

func fuse(subViews ...*SubView) *SubView {
	fused := &SubView{Issues: map[string][]*result.Issue{}}
	for _, subView := range subViews {
		if subView == nil {
			continue
		}
		for linter, issues := range subView.Issues {
			fused.Issues[linter] = append(fused.Issues[linter], issues...)
		}
		fused.LineCount += subView.LineCount
	}
	return fused
}

func aggregateViewOpts(opts ...*ViewOpts) *ViewOpts {
	aggregate := &ViewOpts{depth: -1}

	var paths []string
	for _, opt := range opts {
		if opt.depth >= 0 {
			aggregate.depth = opt.depth
		}
		paths = append(paths, opt.paths...)
	}
	sort.Strings(paths)

	var lastPath string
	for _, path := range paths {
		if len(strings.Split(path, string(os.PathSeparator))) <= aggregate.depth {
			continue
		}

		if path != lastPath {
			aggregate.paths = append(aggregate.paths, path)
			lastPath = path
		}
	}
	return aggregate
}

type sortableIssues []*result.Issue

func (s sortableIssues) Len() int          { return len(s) }
func (s sortableIssues) Swap(i int, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableIssues) Less(i int, j int) bool {
	switch {
	case s[i].FilePath() != s[j].FilePath():
		return s[i].FilePath() < s[j].FilePath()
	case s[i].Line() != s[j].Line():
		return s[i].Line() < s[j].Line()
	case s[i].Column() != s[j].Column():
		return s[i].Column() < s[j].Column()
	default:
		return s[i].Text < s[j].Text
	}
}