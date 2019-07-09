package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/golangci/golangci-lint/pkg/result"

	"github.com/Helcaraxan/goality/lib/report"
)

const defaultTolerance = 10

type IssueCategories []*IssueCategory

func (c IssueCategories) String() string {
	var output []string
	for idx := range c {
		output = append(output, c[idx].String())
	}
	return strings.Join(output, "\n")
}

type IssueCategory struct {
	Linter         string
	Representative string
	Issues         []*result.Issue
}

func (c *IssueCategory) String() string {
	return fmt.Sprintf("%s - %s - %d occurences", c.Linter, c.Representative, len(c.Issues))
}

func IssueRanking(view *report.View, tolerance int) IssueCategories {
	if tolerance == 0 {
		tolerance = defaultTolerance
	}

	linterMap := map[string][]*IssueCategory{}
	for _, subView := range view.SubViews {
		for name, issues := range subView.Issues {
			linterMap[name] = categorise(linterMap[name], issues, tolerance)
		}
	}

	var categories []*IssueCategory
	for _, categoryList := range linterMap {
		categories = append(categories, categoryList...)
	}
	sort.Slice(categories, func(i int, j int) bool { return len(categories[i].Issues) > len(categories[j].Issues) })
	return categories
}

func categorise(categories []*IssueCategory, issues []*result.Issue, tolerance int) IssueCategories {
	for _, issue := range issues {
		var categorised bool
		for _, category := range categories {
			if levenshtein.ComputeDistance(category.Representative, normalise(issue)) <= tolerance {
				category.Issues = append(category.Issues, issue)
				categorised = true
			}
		}
		if !categorised {
			categories = append(categories, &IssueCategory{
				Linter:         issue.FromLinter,
				Representative: normalise(issue),
				Issues:         []*result.Issue{issue},
			})
		}
	}
	return categories
}

// We remove string elements between quotes as those tend
// to be the personalised bits of issue messages.
func normalise(issue *result.Issue) string {
	representant := issue.Text
	for _, quoteChar := range []string{"\"", "`", "'"} {
		var parts []string
		for idx, element := range strings.Split(representant, quoteChar) {
			if idx%2 == 1 {
				parts = append(parts, "<identifier>")
			} else if len(element) > 0 {
				parts = append(parts, element)
			}
		}
		representant = strings.Join(parts, " ")
	}
	return representant
}
