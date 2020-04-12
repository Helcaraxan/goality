package formatters

import (
	"fmt"
	"io"
	"strings"
)

type CSVFormatter struct{}

func (f *CSVFormatter) PrintTable(w io.Writer, headers []string, resultMatrix [][]string, headerToColumnRatios []int) error {
	ratioedHeaders := []string{}
	for idx := range headers {
		ratioedHeaders = append(ratioedHeaders, headers[idx]+strings.Repeat(",", headerToColumnRatios[idx]-1))
	}

	if _, err := fmt.Fprintln(w, strings.Join(ratioedHeaders, ",")); err != nil {
		return err
	}

	for _, line := range resultMatrix {
		if _, err := fmt.Fprintln(w, strings.NewReplacer("(", "", ")", "").Replace(strings.Join(line, ","))); err != nil {
			return err
		}
	}

	return nil
}
