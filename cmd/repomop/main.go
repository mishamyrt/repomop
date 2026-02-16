package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"repomop/internal/delete"
	"repomop/internal/format"
	"repomop/internal/pathutil"
	"repomop/internal/scanner"
	"repomop/internal/tui"
)

var (
	errHelp    = errors.New("help requested")
	appVersion = "dev"
)

type cliOptions struct {
	path     string
	maxDepth int
	dryRun   bool
	yes      bool
	version  bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseFlags(args, stdout)
	if err != nil {
		if errors.Is(err, errHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}

	if opts.version {
		fmt.Fprintln(stdout, appVersion)
		return 0
	}

	rootPath, err := filepath.Abs(opts.path)
	if err != nil {
		fmt.Fprintf(stderr, "resolve path: %v\n", err)
		return 1
	}

	scanOpts := scanner.ScanOptions{
		RootPath: rootPath,
		MaxDepth: opts.maxDepth,
	}

	if opts.dryRun || opts.yes {
		artifacts, warnings, err := scanner.ScanAndMeasure(scanOpts)
		if err != nil {
			fmt.Fprintf(stderr, "scan failed: %v\n", err)
			return 1
		}

		if opts.dryRun {
			printDryRun(stdout, rootPath, artifacts, warnings)
			return 0
		}

		result := delete.Artifacts(artifacts)
		printDeleteSummary(stdout, rootPath, artifacts, warnings, result)
		if len(result.Errors) > 0 {
			return 1
		}
		return 0
	}

	program := tea.NewProgram(tui.NewModel(scanOpts), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintf(stderr, "tui failed: %v\n", err)
		return 1
	}

	model, ok := finalModel.(tui.Model)
	if !ok {
		fmt.Fprintln(stderr, "unexpected tui state")
		return 1
	}

	if model.FatalError() != nil {
		fmt.Fprintf(stderr, "scan failed: %v\n", model.FatalError())
		return 1
	}

	return 0
}

func parseFlags(args []string, stdout io.Writer) (cliOptions, error) {
	var opts cliOptions
	cwd, err := os.Getwd()
	if err != nil {
		return opts, fmt.Errorf("get working directory: %w", err)
	}

	fs := flag.NewFlagSet("repomop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.path, "path", cwd, "root directory to scan")
	fs.IntVar(&opts.maxDepth, "max-depth", -1, "max traversal depth (-1 means unlimited)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "list artifacts without deleting")
	fs.BoolVar(&opts.yes, "yes", false, "delete all found artifacts without interactive confirmation")
	fs.BoolVar(&opts.version, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return opts, errHelp
		}
		return opts, err
	}
	if opts.maxDepth < -1 {
		return opts, fmt.Errorf("max-depth must be -1 or >= 0")
	}
	if opts.dryRun && opts.yes {
		return opts, fmt.Errorf("--dry-run and --yes cannot be used together")
	}

	return opts, nil
}

func printDryRun(stdout io.Writer, root string, artifacts []scanner.Artifact, warnings []error) {
	fmt.Fprintln(stdout, "repomop dry-run")
	if len(artifacts) == 0 {
		fmt.Fprintln(stdout, "No artifacts found.")
		return
	}

	total := int64(0)
	for _, artifact := range artifacts {
		total += artifact.SizeBytes
		fmt.Fprintf(stdout, "- %8s  %s  %s\n",
			format.Bytes(artifact.SizeBytes),
			pathutil.RelativePathOrSelf(root, artifact.Path),
			artifact.Kind,
		)
	}
	fmt.Fprintf(stdout, "Found: %d artifacts, Potential free space: %s\n", len(artifacts), format.Bytes(total))
	if len(warnings) > 0 {
		fmt.Fprintf(stdout, "Warnings: %d size calculation warnings\n", len(warnings))
	}
}

func printDeleteSummary(stdout io.Writer, root string, artifacts []scanner.Artifact, warnings []error, result delete.Result) {
	fmt.Fprintln(stdout, "repomop --yes")
	if len(artifacts) == 0 {
		fmt.Fprintln(stdout, "No artifacts found.")
		return
	}

	fmt.Fprintf(stdout, "Found artifacts: %d\n", len(artifacts))
	fmt.Fprintf(stdout, "Deleted: %d\n", len(result.Deleted))
	fmt.Fprintf(stdout, "Freed space: %s\n", format.Bytes(result.FreedBytes))

	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Delete errors: %d\n", len(result.Errors))
		for _, item := range result.Errors {
			fmt.Fprintf(stdout, "- %s: %v\n", pathutil.RelativePathOrSelf(root, item.Artifact.Path), item.Err)
		}
	}
	if len(warnings) > 0 {
		fmt.Fprintf(stdout, "Warnings: %d size calculation warnings\n", len(warnings))
	}
}
