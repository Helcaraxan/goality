package printer

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func printTable(w io.Writer, headers []string, rows [][]string, headerToColumnRatios []int) error {
	headerWidths, columnWidths, err := computeDimensions(headers, rows, headerToColumnRatios)
	if err != nil {
		return err
	}

	headerTemplate, rowTemplate := computeTemplates(headerWidths, columnWidths)

	output := []string{printRow(headerTemplate, headers)}
	for idx := range rows {
		output = append(output, printRow(rowTemplate, rows[idx]))
	}
	_, err = fmt.Fprintln(w, strings.Join(output, "\n"))
	return err
}

func computeDimensions(headers []string, rows [][]string, headerToColumnRatio []int) ([]int, []int, error) {
	columnCount, delineations, err := validateMatrix(headers, rows, headerToColumnRatio)
	if err != nil {
		return nil, nil, err
	}

	columnWidths := []int{}
	headerWidths := []int{}
	for idx := range headers {
		headerWidths = append(headerWidths, len(headers[idx]))
	}

	var groupWidth int
	var headerIdx int
	for column := 0; column < columnCount; column++ {
		var maxFieldLength int
		for row := range rows {
			if len(rows[row][column]) > maxFieldLength {
				maxFieldLength = len(rows[row][column])
			}
		}
		columnWidths = append(columnWidths, maxFieldLength)

		groupWidth += maxFieldLength + 1
		if column == delineations[headerIdx+1] {
			if groupWidth-1 > headerWidths[headerIdx] {
				headerWidths[headerIdx] += groupWidth - 1 - headerWidths[headerIdx]
			} else if headerWidths[headerIdx] > groupWidth-1 {
				columnGroup := columnWidths[delineations[headerIdx]+1 : delineations[headerIdx+1]+1]
				splitRemainerAcrossColumns(headerWidths[headerIdx]-groupWidth+1, columnGroup)
			}
			groupWidth = 0
			headerIdx++
		}
	}
	return headerWidths, columnWidths, nil
}

func validateMatrix(headers []string, rows [][]string, headerToColumnRatio []int) (int, []int, error) {
	if len(headers) != len(headerToColumnRatio) {
		return 0, nil, fmt.Errorf("malformed content: got %d headers for but %d header-to-column ratios", len(headers), len(headerToColumnRatio))
	}
	delineations := []int{-1}
	var expectedColumnCount int
	for _, ratio := range headerToColumnRatio {
		expectedColumnCount += ratio
		delineations = append(delineations, expectedColumnCount-1)
	}
	for idx := range rows {
		if len(rows[idx]) != expectedColumnCount {
			return 0, nil, fmt.Errorf("malformed content: row %d has %d fields instead of the expected %d", idx, len(rows[idx]), expectedColumnCount)
		}
	}
	return expectedColumnCount, delineations, nil
}

func splitRemainerAcrossColumns(remainder int, columns []int) {
	if remainder <= 0 {
		return
	}

	wrappedColumns := newIndexedColumns(columns)
	sort.Stable(sortedIntSlice{indexedIntSlice: wrappedColumns})

	var currIdx int
	for remainder > 0 {
		wrappedColumns.content[currIdx]++
		remainder--

		if currIdx == wrappedColumns.Len()-1 || wrappedColumns.content[currIdx] < wrappedColumns.content[currIdx+1] {
			currIdx = 0
		} else {
			currIdx++
		}
	}
	sort.Stable(orderedIntSlice{indexedIntSlice: wrappedColumns})
}

func computeTemplates(headerWidths []int, columnWidths []int) (string, string) {
	var headerTemplate string
	for _, width := range headerWidths {
		headerTemplate += fmt.Sprintf("%%-%ds ", width)
	}
	var rowTemplate string
	for _, width := range columnWidths {
		rowTemplate += fmt.Sprintf("%%-%ds ", width)
	}
	return headerTemplate, rowTemplate
}

func printRow(template string, rowData []string) string {
	data := []interface{}{}
	for idx := range rowData {
		data = append(data, rowData[idx])
	}
	return fmt.Sprintf(template, data...)
}

// `indexedIntSlice` is a wrapper struct that allows to sort a slice of integers while keeping track
// of their original ordering and reordering the slice back to the original order respectively via
// the meta-wrappers `orderedColumns` and `orderedColumns`.
type indexedIntSlice struct {
	content []int
	indices []int
}

func newIndexedColumns(content []int) indexedIntSlice {
	indices := make([]int, len(content))
	for idx := range content {
		indices[idx] = idx
	}
	return indexedIntSlice{
		content: content,
		indices: indices,
	}
}

func (c indexedIntSlice) Len() int { return len(c.content) }
func (c indexedIntSlice) Swap(i, j int) {
	c.content[i], c.content[j] = c.content[j], c.content[i]
	c.indices[i], c.indices[j] = c.indices[j], c.indices[i]
}

type sortedIntSlice struct{ indexedIntSlice }

func (c sortedIntSlice) Less(i, j int) bool { return c.content[i] < c.content[j] }

type orderedIntSlice struct{ indexedIntSlice }

func (c orderedIntSlice) Less(i, j int) bool { return c.indices[i] < c.indices[j] }
