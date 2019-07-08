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
	subViewList := viewList(view)

	resultMatrix := [][]interface{}{}
	for _, subViewPath := range subViewList {
		resultMatrix = append(resultMatrix, getSubViewLine(view.SubViews[subViewPath], view.Linters))
	}

	header, lineTemplate := generateHeaderAndLineTemplate(view.Linters, resultMatrix)
	output := []string{
		fmt.Sprintf("Quality report for Go codebase located at '%s'\n\n%s", view.Path, header),
	}
	for _, line := range resultMatrix {
		output = append(output, fmt.Sprintf(lineTemplate, line...))
	}
	output = append(output, "\nData-format: total-issues (average issues per 1K LoC)")

	_, err := fmt.Fprintln(w, strings.Join(output, "\n"))
	return err
}

func viewList(view *report.View) []string {
	var subViewList []string
	for _, subView := range view.SubViews {
		subViewList = append(subViewList, subView.Path)
	}
	sort.Sort(paths(subViewList))
	return subViewList
}

func getSubViewLine(subView *report.SubView, linters []string) []interface{} {
	results := []interface{}{subView.Path}
	for _, linter := range linters {
		issueCount := len(subView.Issues[linter])
		var occurenceRate float32
		if issueCount > 0 {
			occurenceRate = 1000 * float32(issueCount) / float32(subView.LineCount)
		}
		results = append(results, fmt.Sprintf("%d", issueCount), fmt.Sprintf("(%5.2f)", occurenceRate))
	}
	return results
}

func generateHeaderAndLineTemplate(linterList []string, resultMatrix [][]interface{}) (string, string) {
	columnWidths := computeColumnWidths(linterList, resultMatrix)

	lineTemplate := fmt.Sprintf("%%-%ds ", columnWidths[0])
	header := fmt.Sprintf(lineTemplate, "path")

	for idx := range columnWidths[1:] {
		lineTemplate += fmt.Sprintf("%%-%ds ", columnWidths[idx+1])
		if idx%2 == 1 {
			lineTemplate += " "
			headerFieldTemplate := fmt.Sprintf("%%-%ds  ", columnWidths[idx]+columnWidths[idx+1]+1)
			header += fmt.Sprintf(headerFieldTemplate, linterList[idx/2])
		}
	}
	return header, lineTemplate
}

func computeColumnWidths(linterList []string, resultMatrix [][]interface{}) []int {
	var headerWidths []int
	for idx := range linterList {
		headerWidths = append(headerWidths, len(linterList[idx]))
	}

	maxFieldLength := len("path")
	for _, row := range resultMatrix {
		if len(row[0].(string)) > maxFieldLength {
			maxFieldLength = len(row[0].(string))
		}
	}

	columnWidths := []int{maxFieldLength}
	for column := range resultMatrix[0][1:] {
		maxFieldLength = 0
		for line := range resultMatrix {
			if len(resultMatrix[line][column+1].(string)) > maxFieldLength {
				maxFieldLength = len(resultMatrix[line][column+1].(string))
			}
		}
		if column%2 == 1 && maxFieldLength < headerWidths[column/2]-columnWidths[column]-1 {
			maxFieldLength = headerWidths[column/2] - columnWidths[column] - 1
		}
		columnWidths = append(columnWidths, maxFieldLength)
	}
	return columnWidths
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
