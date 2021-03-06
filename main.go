package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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
		commonArgs.logger.WithError(err).Debugf("Execution failed.")
		os.Exit(1)
	}

	commonArgs.logger.Debug("Execution successful.")
}

type runArgs struct {
	*commonArgs

	projectPath string

	config       string
	excludePaths []string
	linters      []string
	depth        int
	paths        []string
	format       printer.FormatType
}

func initRunCommand(commonArgs *commonArgs) *cobra.Command {
	cArgs := &runArgs{commonArgs: commonArgs}

	var formatValue string

	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Perform a quality analysis of the specified project.",
		Long: `Run an analysis over the directory tree rooted at the specified path. If no path is given this defaults to the current working directory.

Example:
  goality run
  goality run --config=~/.golangci.yaml --depth 1 ./cmd
  goality run src/github.com/me/project
`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch formatValue {
			case "csv":
				cArgs.format = printer.FormatTypeCSV
			case "screen":
				cArgs.format = printer.FormatTypeScreen
			default:
				return fmt.Errorf("unknown result output format %q", cArgs.format)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, ".")
			}
			cArgs.projectPath = args[0]

			return executeRunCommand(cArgs)
		},
	}

	cmd.Flags().StringVarP(&cArgs.config, "config", "c", "", "Path to a golangci-lint configuration file that should be used.")
	cmd.Flags().StringSliceVarP(&cArgs.excludePaths, "excludes", "e", nil, "Names of directories that should be skipped.")
	cmd.Flags().StringSliceVarP(&cArgs.linters, "linters", "l", nil, "Specific linters to run.")
	cmd.Flags().IntVarP(&cArgs.depth, "depth", "d", -1, "Path granularity at which to perform the quality analysis.")
	cmd.Flags().StringSliceVarP(&cArgs.paths, "paths", "p", nil, "Specific paths for which to provide aggregate quality analysis results.")
	cmd.Flags().StringVarP(&formatValue, "format", "f", "screen", "Format to use when printing the results.")

	return cmd
}

func executeRunCommand(args *runArgs) error {
	cwd, err := os.Getwd()
	if err != nil {
		args.logger.WithError(err).Error("Failed to determine the current working directory")
		return err
	}

	if args.config != "" && !filepath.IsAbs(args.config) {
		args.config = filepath.Join(cwd, args.config)
	}

	if !filepath.IsAbs(args.projectPath) {
		args.projectPath = filepath.Join(cwd, args.projectPath)
	}

	for idx := range args.paths {
		if filepath.IsAbs(args.paths[idx]) {
			relPath, relErr := filepath.Rel(args.projectPath, args.paths[idx])
			if relErr != nil {
				return relErr
			} else if strings.HasPrefix(relPath, "../") {
				return fmt.Errorf("specified path %q is outside of the targeted project at %q", args.paths[idx], args.projectPath)
			}

			args.paths[idx] = relPath
		}
	}

	project, err := report.Parse(
		args.logger,
		args.projectPath,
		report.WithConfig(args.config),
		report.WithLinters(args.linters...),
		report.WithExcludeDirs(args.excludePaths...),
	)
	if err != nil {
		return err
	}

	return printer.PrintView(os.Stdout, project.GenerateView(report.WithDepth(args.depth), report.WithPaths(args.paths...)), args.format)
}
