package printer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Helcaraxan/goality/lib/printer/formatters"
	"github.com/Helcaraxan/goality/lib/report"
)

type FormatType uint8

const (
	FormatTypeUnknown = iota
	FormatTypeScreen
	FormatTypeCSV
)

type Formatter interface {
	PrintTable(io.Writer, []string, [][]string, []int) error
}

func PrintView(w io.Writer, view *report.View, format FormatType) error {
	if len(view.SubViews) == 0 {
		return nil
	}

	var subViewList []string
	for _, subView := range view.SubViews {
		subViewList = append(subViewList, subView.Path)
	}

	sort.Slice(subViewList, func(i, j int) bool {
		iDepth := strings.Count(subViewList[i], string(os.PathSeparator))
		jDepth := strings.Count(subViewList[j], string(os.PathSeparator))
		if iDepth != jDepth {
			return iDepth < jDepth
		}
		return subViewList[i] < subViewList[j]
	})

	ratios := []int{1}
	for i := 0; i < len(view.Linters); i++ {
		ratios = append(ratios, 2)
	}

	headers := append([]string{"path"}, view.Linters...)

	resultMatrix := [][]string{}
	for _, subViewPath := range subViewList {
		resultMatrix = append(resultMatrix, getSubViewLine(view.SubViews[subViewPath], view.Linters))
	}

	var formatter Formatter

	switch format {
	case FormatTypeUnknown:
		return errors.New("unknown format type specified for result printing")
	case FormatTypeCSV:
		formatter = &formatters.CSVFormatter{}
	case FormatTypeScreen:
		if _, err := fmt.Fprintf(w, "Quality report for Go codebase located at '%s'\n\n", view.Path); err != nil {
			return err
		}

		formatter = &formatters.ScreenFormatter{}
	}

	if err := formatter.PrintTable(w, headers, resultMatrix, ratios); err != nil {
		return err
	}

	switch format {
	case FormatTypeScreen:
		if _, err := fmt.Fprint(w, "\nData-format: total-issues (average issues per 1K LoC)\n"); err != nil {
			return err
		}
	default: // Nothing.
	}

	return nil
}

func getSubViewLine(subView *report.SubView, linters []string) []string {
	results := []string{subView.Path}

	for _, linter := range linters {
		issueCount := len(subView.Issues[linter])
		occurenceRate := float32(0)

		if issueCount > 0 {
			occurenceRate = 1000 * float32(issueCount) / float32(subView.LineCount)
		}

		results = append(results, fmt.Sprintf("%d", issueCount), fmt.Sprintf("(%4.2f)", occurenceRate))
	}

	return results
}
