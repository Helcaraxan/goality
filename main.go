package main

import (
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Helcaraxan/goality/lib/printer"
	"github.com/Helcaraxan/goality/lib/report"
)

type commonArgs struct {
	logger *logrus.Logger
}

func main() {
	commonArgs := &commonArgs{logger: logrus.New()}

	var verbose, quiet bool
	rootCmd := &cobra.Command{
		Use: "goality",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if quiet {
				commonArgs.logger.SetOutput(ioutil.Discard)
			}
			if verbose {
				commonArgs.logger.SetLevel(logrus.DebugLevel)
				commonArgs.logger.Debugf("Using verbose logging")
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output.")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Do not perform any logging.")

	rootCmd.AddCommand(
		initRunCommand(commonArgs),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type runArgs struct {
	*commonArgs

	projectPath string

	config  string
	linters []string
	depth   int
	paths   []string
}

func initRunCommand(commonArgs *commonArgs) *cobra.Command {
	cArgs := &runArgs{commonArgs: commonArgs}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Perform a quality analysis of the specified project.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, ".")
			}
			cArgs.projectPath = args[0]
			return executeRunCommand(cArgs)
		},
	}

	cmd.Flags().StringVarP(&cArgs.config, "config", "c", "", "Path to a golangci-lint configuration file that should be used.")
	cmd.Flags().StringSliceVarP(&cArgs.linters, "linters", "l", nil, "Specific linters to run.")
	cmd.Flags().IntVarP(&cArgs.depth, "depth", "d", -1, "Path granularity at which to perform the quality analysis.")
	cmd.Flags().StringSliceVarP(&cArgs.paths, "paths", "p", nil, "Specific paths for which to provide aggregate quality analysis results.")

	return cmd
}

func executeRunCommand(args *runArgs) error {
	project, err := report.Parse(args.logger, args.projectPath, report.WithConfig(args.config), report.WithLinters(args.linters...))
	if err != nil {
		return err
	}
	return printer.Print(os.Stdout, project.GenerateView(report.WithDepth(args.depth), report.WithPaths(args.paths...)))
}
