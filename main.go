package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Helcaraxan/goality/lib/printer"
	"github.com/Helcaraxan/goality/lib/report"
)

func main() {
	logger := logrus.New()

	var verbose bool
	rootCmd := &cobra.Command{
		Use: "goality",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if verbose {
				logger.Infof("Using verbose logging")
				logger.SetLevel(logrus.DebugLevel)
			}
		},
		RunE: func(_ *cobra.Command, args []string) error {
			project, err := report.Parse(logger, args[0], report.WithConfig(".golangci.yaml"))
			if err != nil {
				return err
			}

			view := project.GenerateView(report.WithDepth(1))
			return printer.Print(os.Stdout, view)
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output.")

	if err := rootCmd.Execute(); err != nil {
		logger.WithError(err).Error("Execution failed")
		os.Exit(1)
	}
}
