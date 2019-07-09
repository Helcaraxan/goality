package printer

import (
	"fmt"
	"io"

	"github.com/Helcaraxan/goality/lib/analysis"
)

var maxIssueTextWidth = 100

func PrintCategories(w io.Writer, categories analysis.IssueCategories) error {
	headers := []string{"occurences", "linter", "issue"}
	var categoryMatrix [][]string
	for idx := range categories {
		issueContent := categories[idx].Representative
		if len(issueContent) > maxIssueTextWidth {
			issueContent = issueContent[:maxIssueTextWidth/2+maxIssueTextWidth%2-2] +
				"...." +
				issueContent[len(issueContent)-maxIssueTextWidth/2+2:]
		}
		occurences := fmt.Sprintf("%d", len(categories[idx].Issues))
		categoryMatrix = append(categoryMatrix, []string{occurences, categories[idx].Linter, issueContent})
	}

	return printTable(w, headers, categoryMatrix, []int{1, 1, 1})
}
