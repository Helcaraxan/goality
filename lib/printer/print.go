package printer

import (
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/Helcaraxan/goality/lib/report"
)

func Print(logger *logrus.Logger, w io.Writer, r *report.Project) error {
	output := []string{fmt.Sprintf("Report for Go codebase found at '%s'", r.Path)}

	if _, err := w.Write([]byte(strings.Join(output, "\n"))); err != nil {
		logger.WithError(err).Error("Could not print report.")
		return err
	}
	return nil
}
