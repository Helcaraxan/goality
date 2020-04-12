package printer

import (
	"errors"
	"fmt"
	"io"

	"github.com/Helcaraxan/goality/lib/analysis"
	"github.com/Helcaraxan/goality/lib/printer/formatters"
)

var maxIssueTextWidth = 100

func PrintCategories(w io.Writer, categories analysis.IssueCategories, format FormatType) error {
	var (
		categoryMatrix = [][]string{}
		headers        = []string{"occurrences", "linter", "issue"}
	)

	for idx := range categories {
		issueContent := categories[idx].Representative
		if len(issueContent) > maxIssueTextWidth {
			issueContent = issueContent[:maxIssueTextWidth/2+maxIssueTextWidth%2-2] +
				"...." +
				issueContent[len(issueContent)-maxIssueTextWidth/2+2:]
		}

		occurrences := fmt.Sprintf("%d", len(categories[idx].Issues))
		categoryMatrix = append(categoryMatrix, []string{occurrences, categories[idx].Linter, issueContent})
	}

	var formatter Formatter

	switch format {
	case FormatTypeCSV:
		formatter = &formatters.CSVFormatter{}
	case FormatTypeScreen:
		formatter = &formatters.ScreenFormatter{}
	default:
		return errors.New("unknown format type specified for result printing")
	}

	return formatter.PrintTable(w, headers, categoryMatrix, []int{1, 1, 1})
}
