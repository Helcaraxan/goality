package printer

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Helcaraxan/goality/lib/report"
)

func Print(w io.Writer, view *report.View) error {
	if len(view.SubViews) == 0 {
		return nil
	}
	subViewPaths, lintersList := viewLists(view)

	resultMatrix := [][]interface{}{}
	for _, subViewPath := range subViewPaths {
		resultMatrix = append(resultMatrix, getSubViewLine(view.SubViews[subViewPath], lintersList))
	}

	var headerWidths []int
	for idx := range lintersList {
		headerWidths = append(headerWidths, len(lintersList[idx]))
	}

	var columnWidths []int
	for column := 0; column < len(resultMatrix[0]); column++ {
		var maxFieldLength int
		for line := 0; line < len(resultMatrix); line++ {
			if len(resultMatrix[line][column].(string)) > maxFieldLength {
				maxFieldLength = len(resultMatrix[line][column].(string))
			}
		}
		columnWidths = append(columnWidths, maxFieldLength)
	}

	var header string
	var lineTemplate string
	if len("path") > columnWidths[0] {
		header = "path "
		lineTemplate = fmt.Sprintf("%%-%ds ", len("path"))
	} else {
		headerTemplate := fmt.Sprintf("%%-%ds ", columnWidths[0])
		header = fmt.Sprintf(headerTemplate, "path")
		lineTemplate = fmt.Sprintf("%%-%ds ", columnWidths[0])
	}

	for idx := range lintersList {
		if headerWidths[idx] > columnWidths[2*idx+1]+columnWidths[2*idx+2]+1 {
			header += lintersList[idx] + "  "
			lineTemplate += fmt.Sprintf("%%-%ds %%-%ds  ", columnWidths[2*idx+1], headerWidths[idx]-columnWidths[2*idx+1])
		} else {
			headerTemplate := fmt.Sprintf("%%-%ds  ", columnWidths[2*idx+1]+columnWidths[2*idx+2]+1)
			header += fmt.Sprintf(headerTemplate, lintersList[idx])
			lineTemplate += fmt.Sprintf("%%-%ds %%-%ds  ", columnWidths[2*idx+1], columnWidths[2*idx+2])
		}
	}
	lineTemplate += "\n"

	_, err := fmt.Fprintf(w, "Report for Go codebase located at '%s'\n\n%s\n", view.Path, header)
	if err != nil {
		return err
	}
	for _, line := range resultMatrix {
		_, err = fmt.Fprintf(w, lineTemplate, line...)
		if err != nil {
			return err
		}
	}
	return nil
}

func viewLists(view *report.View) ([]string, []string) {
	var subViewPaths []string
	lintersSet := map[string]struct{}{}
	for _, subView := range view.SubViews {
		subViewPaths = append(subViewPaths, subView.Path)
		for linter := range subView.Issues {
			lintersSet[linter] = struct{}{}
		}
	}
	var linters []string
	for linter := range lintersSet {
		linters = append(linters, linter)
	}
	sort.Sort(paths(subViewPaths))
	sort.Strings(linters)
	return subViewPaths, linters
}

func getSubViewLine(subView *report.SubView, linters []string) []interface{} {
	results := []interface{}{subView.Path}
	for _, linter := range linters {
		issueCount := len(subView.Issues[linter])
		results = append(results, fmt.Sprintf("%d", issueCount), fmt.Sprintf("(%5.2f)", 1000*float32(issueCount)/float32(subView.LineCount)))
	}
	return results
}

type paths []string

func (p paths) Len() int          { return len(p) }
func (p paths) Swap(i int, j int) { p[i], p[j] = p[j], p[i] }
func (p paths) Less(i int, j int) bool {
	iDepth := strings.Count(p[i], string(os.PathSeparator))
	jDepth := strings.Count(p[j], string(os.PathSeparator))
	if iDepth != jDepth {
		return iDepth < jDepth
	}
	return p[i] < p[j]
}
