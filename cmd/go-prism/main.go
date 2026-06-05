package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Athena900/go-prism/internal/app"
	"github.com/Athena900/go-prism/internal/doctor"
	"github.com/Athena900/go-prism/internal/report"
)

const version = "0.1.0"

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "go-prism: %v\n", err)
		code := 1
		var coded interface{ ExitCode() int }
		if errors.As(err, &coded) {
			code = coded.ExitCode()
		}
		os.Exit(code)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "pr":
		return runPR(ctx, args[1:], stdout)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout)
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runPR(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("pr", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	opts := app.PROptions{}
	opts.Version = version
	flags.StringVar(&opts.Base, "base", "origin/main", "base git ref")
	flags.StringVar(&opts.Head, "head", "HEAD", "head git ref")
	flags.StringVar(&opts.ConfigPath, "config", ".go-prism.yml", "config file path")
	flags.StringVar(&opts.Format, "format", "markdown", "output format: markdown or json")
	flags.StringVar(&opts.OutputPath, "output", "", "write output to file")
	flags.StringVar(&opts.WorkDir, "workdir", ".", "target repository directory")
	flags.StringVar(&opts.ModuleOverride, "module", "", "module path override")
	timeout := flags.Duration("timeout", 30*time.Second, "analysis timeout")

	if err := flags.Parse(args); err != nil {
		return err
	}
	opts.Timeout = *timeout

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	evidenceReport, err := app.RunPR(ctx, opts)
	if err != nil {
		return err
	}

	rendered, err := report.Render(evidenceReport, opts.Format)
	if err != nil {
		return err
	}

	if opts.OutputPath != "" {
		return os.WriteFile(opts.OutputPath, rendered, 0o644)
	}

	_, err = stdout.Write(rendered)
	return err
}

func runDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	opts := doctor.Options{
		Version: version,
	}
	flags.StringVar(&opts.ConfigPath, "config", ".go-prism.yml", "config file path")
	flags.StringVar(&opts.Format, "format", "text", "output format: text or json")
	flags.StringVar(&opts.WorkDir, "workdir", ".", "target repository directory")
	flags.StringVar(&opts.ModuleOverride, "module", "", "module path override")
	timeout := flags.Duration("timeout", 10*time.Second, "diagnostic timeout")

	if err := flags.Parse(args); err != nil {
		return exitError{code: 2, err: err}
	}
	opts.Timeout = *timeout

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	doctorReport := doctor.Run(ctx, opts)
	rendered, err := doctor.Render(doctorReport, opts.Format)
	if err != nil {
		return exitError{code: 2, err: err}
	}

	if _, err := stdout.Write(rendered); err != nil {
		return err
	}
	if doctorReport.Status == doctor.StatusFail {
		return exitError{code: 1, err: errors.New("doctor found failing checks")}
	}
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `go-prism - PR evidence reports for Go modules

Usage:
  go-prism pr [flags]
  go-prism doctor [flags]
  go-prism version

Examples:
  go-prism pr --base origin/main --head HEAD --format markdown
  go-prism pr --format json --output evidence.json
  go-prism doctor --format json`)
}

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	return e.err.Error()
}

func (e exitError) Unwrap() error {
	return e.err
}

func (e exitError) ExitCode() int {
	return e.code
}
