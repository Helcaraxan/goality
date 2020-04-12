package formatters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ValidateMatrix(t *testing.T) {
	testcases := map[string]struct {
		rows                 [][]string
		headers              []string
		ratios               []int
		valid                bool
		expectedCount        int
		expectedDelineations []int
	}{
		"SimpleMatrix": {
			rows:                 [][]string{{"", ""}},
			headers:              []string{"", ""},
			ratios:               []int{1, 1},
			valid:                true,
			expectedCount:        2,
			expectedDelineations: []int{-1, 0, 1},
		},
		"ComplexMatrix": {
			rows:                 [][]string{{"", "", "", "", "", ""}},
			headers:              []string{"", "", ""},
			ratios:               []int{1, 3, 2},
			valid:                true,
			expectedCount:        6,
			expectedDelineations: []int{-1, 0, 3, 5},
		},
		"RowsWithVaryingLength": {
			rows: [][]string{
				{"", ""},
				{""},
			},
			headers: []string{"", ""},
			ratios:  []int{1, 1},
			valid:   false,
		},
		"WrongHeaderCount": {
			rows:    [][]string{{"", ""}},
			headers: []string{"", "", ""},
			ratios:  []int{1, 1, 1},
			valid:   false,
		},
		"WrongRatioCount": {
			rows:    [][]string{{"", ""}},
			headers: []string{"", ""},
			ratios:  []int{1, 1, 1},
			valid:   false,
		},
		"MisalignedRatios": {
			rows:    [][]string{{"", ""}},
			headers: []string{"", ""},
			ratios:  []int{1, 2},
			valid:   false,
		},
	}

	for name := range testcases {
		testcase := testcases[name]
		t.Run(name, func(t *testing.T) {
			f := ScreenFormatter{}
			columnCount, delineations, err := f.validateMatrix(testcase.headers, testcase.rows, testcase.ratios)

			if !testcase.valid {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testcase.expectedCount, columnCount)
				assert.Equal(t, testcase.expectedDelineations, delineations)
			}
		})
	}
}

func Test_SplitRemainderAcrossColumns(t *testing.T) {
	testcases := map[string]struct {
		remainder       int
		columns         []int
		expectedColumns []int
	}{
		"NoRemainder": {
			remainder:       0,
			columns:         []int{0, 0},
			expectedColumns: []int{0, 0},
		},
		"OnePerColumn": {
			remainder:       3,
			columns:         []int{0, 0, 0},
			expectedColumns: []int{1, 1, 1},
		},
		"ThreePerColumn": {
			remainder:       9,
			columns:         []int{0, 0, 0},
			expectedColumns: []int{3, 3, 3},
		},
		"Leftovers": {
			remainder:       5,
			columns:         []int{0, 0, 0},
			expectedColumns: []int{2, 2, 1},
		},
		"UnequalColumns": {
			remainder:       5,
			columns:         []int{4, 2, 4},
			expectedColumns: []int{5, 5, 5},
		},
	}

	for name := range testcases {
		testcase := testcases[name]
		t.Run(name, func(t *testing.T) {
			f := ScreenFormatter{}

			f.splitRemainerAcrossColumns(testcase.remainder, testcase.columns)
			assert.Equal(t, testcase.expectedColumns, testcase.columns)
		})
	}
}

func Test_ComputeDimensions(t *testing.T) {
	testcases := map[string]struct {
		matrix               [][]string
		headers              []string
		ratios               []int
		expectedColumnWidths []int
		expectedHeaderWidths []int
	}{
		"Empty": {
			matrix:               [][]string{},
			headers:              []string{},
			ratios:               []int{},
			expectedColumnWidths: []int{},
			expectedHeaderWidths: []int{},
		},
		"SimpleWideHeaders": {
			matrix:               [][]string{{"", "", ""}},
			headers:              []string{"wide", "wide", "wide"},
			ratios:               []int{1, 1, 1},
			expectedColumnWidths: []int{4, 4, 4},
			expectedHeaderWidths: []int{4, 4, 4},
		},
		"SimpleWideColumns": {
			matrix:               [][]string{{"wide", "wide", "wide"}},
			headers:              []string{"", "", ""},
			ratios:               []int{1, 1, 1},
			expectedColumnWidths: []int{4, 4, 4},
			expectedHeaderWidths: []int{4, 4, 4},
		},
		"SimpleWideMixed": {
			matrix:               [][]string{{"wide", "", "wide"}},
			headers:              []string{"", "wide", ""},
			ratios:               []int{1, 1, 1},
			expectedColumnWidths: []int{4, 4, 4},
			expectedHeaderWidths: []int{4, 4, 4},
		},
		"ComplexWideHeaders": {
			matrix: [][]string{
				{"", "", "", "", "", ""},
				{"", "", "", "", "", ""},
			},
			headers:              []string{"wide", "wide_wide", "wide"},
			ratios:               []int{1, 3, 2},
			expectedColumnWidths: []int{4, 3, 2, 2, 2, 1},
			expectedHeaderWidths: []int{4, 9, 4},
		},
		"ComplexWideColumns": {
			matrix: [][]string{
				{"wide", "wide", "wide", "wide", "wide", "wide"},
				{"wide", "wide", "wide", "wide", "wide", "wide"},
			},
			headers:              []string{"", "", ""},
			ratios:               []int{1, 3, 2},
			expectedColumnWidths: []int{4, 4, 4, 4, 4, 4},
			expectedHeaderWidths: []int{4, 14, 9},
		},
		"ComplexWideMixed": {
			matrix: [][]string{
				{"wide", "", "", "", "wide", ""},
				{"", "wide", "", "wide", "", "wide"},
			},
			headers:              []string{"", "wide_wide_wide", ""},
			ratios:               []int{1, 3, 2},
			expectedColumnWidths: []int{4, 4, 4, 4, 4, 4},
			expectedHeaderWidths: []int{4, 14, 9},
		},
	}

	for name := range testcases {
		testcase := testcases[name]
		t.Run(name, func(t *testing.T) {
			f := ScreenFormatter{}
			hWidths, cWidths, err := f.computeDimensions(testcase.headers, testcase.matrix, testcase.ratios)

			require.NoError(t, err)
			assert.Equal(t, testcase.expectedColumnWidths, cWidths)
			assert.Equal(t, testcase.expectedHeaderWidths, hWidths)
		})
	}
}
