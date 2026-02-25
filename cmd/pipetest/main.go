package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mehditeymorian/pipetest/internal/ast"
	"github.com/mehditeymorian/pipetest/internal/compiler"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
	"github.com/mehditeymorian/pipetest/internal/parser"
	"github.com/mehditeymorian/pipetest/internal/report"
	"github.com/mehditeymorian/pipetest/internal/runtime"
	"github.com/spf13/cobra"
)

const (
	evalUsage    = "pipetest eval <program.pt> [--format pretty|json]"
	runUsage     = "pipetest run <program.pt> [--report-dir dir] [--format pretty|json] [--timeout duration] [--verbose]"
	requestUsage = "pipetest request <program.pt> <request-name> [--format pretty|json] [--timeout duration] [--verbose]"
)

type cliExitError struct {
	code  int
	msg   string
	usage string
}

func (e *cliExitError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	if e.usage != "" {
		return e.usage
	}
	return fmt.Sprintf("exit code %d", e.code)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd := newRootCmd(stdout, stderr)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		var exitErr *cliExitError
		if errors.As(err, &exitErr) {
			if exitErr.msg != "" {
				_, _ = fmt.Fprintln(stderr, exitErr.msg)
			}
			if exitErr.usage != "" {
				_, _ = fmt.Fprintln(stderr, strings.TrimSpace(exitErr.usage))
			}
			return exitErr.code
		}
		_, _ = fmt.Fprintln(stderr, err.Error())
		printUsage(stderr)
		return 2
	}
	return 0
}

func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "pipetest",
		Short:         "pipetest CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return &cliExitError{code: 2, usage: rootUsage()}
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newEvalCmd(stdout), newRunCmd(stdout), newRequestCmd(stdout))
	return root
}

func newEvalCmd(stdout io.Writer) *cobra.Command {
	var format string
	evalCmd := &cobra.Command{
		Use:   "eval <program.pt>",
		Short: "Static analysis only",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &cliExitError{code: 2, msg: "usage: " + evalUsage}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return &cliExitError{code: 2, msg: err.Error()}
			}
			_, _, allDiags := compileProgram(args[0])
			allDiags = diagnostics.SortAndDedupe(allDiags)
			if err := printCommandResult(stdout, "eval", format, allDiags, nil); err != nil {
				return &cliExitError{code: 1, msg: fmt.Sprintf("failed to write output: %v", err)}
			}
			if len(allDiags) > 0 {
				return &cliExitError{code: 1}
			}
			return nil
		},
	}
	evalCmd.Flags().StringVar(&format, "format", "pretty", "stdout format: pretty|json")
	return evalCmd
}

func newRunCmd(stdout io.Writer) *cobra.Command {
	var (
		format    string
		reportDir string
		timeout   string
		verbose   bool
	)

	runCmd := &cobra.Command{
		Use:   "run <program.pt>",
		Short: "Compile and execute flows",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &cliExitError{code: 2, msg: "usage: " + runUsage}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return &cliExitError{code: 2, msg: err.Error()}
			}
			runtimeOpt := runtime.Options{Verbose: verbose, LogWriter: stdout}
			if timeout != "" {
				d, err := time.ParseDuration(timeout)
				if err != nil {
					return &cliExitError{code: 2, msg: fmt.Sprintf("invalid --timeout value: %v", err)}
				}
				runtimeOpt.TimeoutOverride = &d
			}

			plan, _, allDiags := compileProgram(args[0])
			allDiags = diagnostics.SortAndDedupe(allDiags)
			if len(allDiags) > 0 {
				if err := printCommandResult(stdout, "run", format, allDiags, nil); err != nil {
					return &cliExitError{code: 1, msg: fmt.Sprintf("failed to write output: %v", err)}
				}
				return &cliExitError{code: 1}
			}

			if err := os.MkdirAll(reportDir, 0o755); err != nil {
				return &cliExitError{code: 1, msg: fmt.Sprintf("failed to create report directory: %v", err)}
			}

			result := runtime.Execute(context.Background(), plan, runtimeOpt)
			result.Diags = diagnostics.SortAndDedupe(result.Diags)
			model := report.Build(plan, result)

			if err := writeRunReports(reportDir, model); err != nil {
				return &cliExitError{code: 1, msg: fmt.Sprintf("failed to write reports: %v", err)}
			}

			if err := printCommandResult(stdout, "run", format, result.Diags, &model); err != nil {
				return &cliExitError{code: 1, msg: fmt.Sprintf("failed to write output: %v", err)}
			}
			if len(result.Diags) > 0 {
				return &cliExitError{code: 1}
			}
			return nil
		},
	}
	runCmd.Flags().StringVar(&format, "format", "pretty", "stdout format: pretty|json")
	runCmd.Flags().StringVar(&reportDir, "report-dir", "./pipetest-report", "directory for report artifacts")
	runCmd.Flags().StringVar(&timeout, "timeout", "", "override timeout setting, e.g. 2s")
	runCmd.Flags().BoolVar(&verbose, "verbose", false, "print verbose execution logs")
	return runCmd
}

func newRequestCmd(stdout io.Writer) *cobra.Command {
	var (
		format  string
		timeout string
		verbose bool
	)

	requestCmd := &cobra.Command{
		Use:   "request <program.pt> <request-name>",
		Short: "Compile and execute a single request",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return &cliExitError{code: 2, msg: "usage: " + requestUsage}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return &cliExitError{code: 2, msg: err.Error()}
			}
			runtimeOpt := runtime.Options{Verbose: verbose, LogWriter: stdout}
			if timeout != "" {
				d, err := time.ParseDuration(timeout)
				if err != nil {
					return &cliExitError{code: 2, msg: fmt.Sprintf("invalid --timeout value: %v", err)}
				}
				runtimeOpt.TimeoutOverride = &d
			}

			plan, _, allDiags := compileProgram(args[0])
			allDiags = diagnostics.SortAndDedupe(allDiags)
			if len(allDiags) > 0 {
				if err := printCommandResult(stdout, "request", format, allDiags, nil); err != nil {
					return &cliExitError{code: 1, msg: fmt.Sprintf("failed to write output: %v", err)}
				}
				return &cliExitError{code: 1}
			}

			requestName := args[1]
			found := false
			for _, req := range plan.Requests {
				if req.Name == requestName {
					found = true
					break
				}
			}
			if !found {
				return &cliExitError{code: 2, msg: fmt.Sprintf("request %q not found in %s", requestName, args[0])}
			}

			single := *plan
			single.Flows = []compiler.PlanFlow{{
				Name:  "request:" + requestName,
				Steps: []compiler.PlanStep{{Request: requestName, Binding: requestName}},
			}}

			result := runtime.Execute(context.Background(), &single, runtimeOpt)
			result.Diags = diagnostics.SortAndDedupe(result.Diags)
			if err := printCommandResult(stdout, "request", format, result.Diags, nil); err != nil {
				return &cliExitError{code: 1, msg: fmt.Sprintf("failed to write output: %v", err)}
			}
			if len(result.Diags) > 0 {
				return &cliExitError{code: 1}
			}
			return nil
		},
	}
	requestCmd.Flags().StringVar(&format, "format", "pretty", "stdout format: pretty|json")
	requestCmd.Flags().StringVar(&timeout, "timeout", "", "override timeout setting, e.g. 2s")
	requestCmd.Flags().BoolVar(&verbose, "verbose", false, "print verbose execution logs")
	return requestCmd
}

func validateFormat(format string) error {
	if format != "pretty" && format != "json" {
		return fmt.Errorf("unknown --format %q (expected pretty|json)", format)
	}
	return nil
}

func writeRunReports(reportDir string, model report.Model) error {
	junitPath := filepath.Join(reportDir, "pipetest-junit.xml")
	legacyXMLPath := filepath.Join(reportDir, "pipetest-report.xml")
	jsonPath := filepath.Join(reportDir, "pipetest-report.json")
	if err := report.WriteJUnitFile(junitPath, model); err != nil {
		return err
	}
	if err := report.WriteJUnitFile(legacyXMLPath, model); err != nil {
		return err
	}
	if err := report.WriteJSONFile(jsonPath, model); err != nil {
		return err
	}
	return nil
}

func compileProgram(entryPath string) (*compiler.Plan, []compiler.Module, []diagnostics.Diagnostic) {
	mods, parseDiags := loadModules(entryPath)
	if len(parseDiags) > 0 {
		return nil, mods, parseDiags
	}
	plan, compDiags := compiler.Compile(entryPath, mods)
	if len(compDiags) > 0 {
		return nil, mods, compDiags
	}
	return plan, mods, nil
}

func loadModules(entryPath string) ([]compiler.Module, []diagnostics.Diagnostic) {
	entryPath = filepath.Clean(entryPath)
	loaded := map[string]compiler.Module{}
	var diags []diagnostics.Diagnostic
	var visit func(string)
	visit = func(path string) {
		path = filepath.Clean(path)
		if _, ok := loaded[path]; ok {
			return
		}
		src, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				diags = append(diags, diagnostics.Diagnostic{Severity: "error", Code: "E_IMPORT_NOT_FOUND", Message: fmt.Sprintf("import not found: %s", path), File: path, Line: 1, Column: 1, Hint: "load the imported file"})
				return
			}
			diags = append(diags, diagnostics.Diagnostic{Severity: "error", Code: "E_IMPORT_READ", Message: err.Error(), File: path, Line: 1, Column: 1, Hint: "check file permissions and path"})
			return
		}
		prog, lexErrs, parseErrs := parser.Parse(path, string(src))
		for _, e := range lexErrs {
			diags = append(diags, diagnostics.Diagnostic{Severity: "error", Code: e.Code, Message: e.Message, File: e.File, Line: e.Span.Start.Line, Column: e.Span.Start.Column, Hint: e.Hint})
		}
		for _, e := range parseErrs {
			diags = append(diags, diagnostics.Diagnostic{Severity: "error", Code: e.Code, Message: e.Message, File: e.File, Line: e.Span.Start.Line, Column: e.Span.Start.Column, Hint: e.Hint})
		}
		loaded[path] = compiler.Module{Path: path, Program: prog}
		if len(lexErrs) > 0 || len(parseErrs) > 0 {
			return
		}
		for _, stmt := range prog.Stmts {
			imp, ok := stmt.(*ast.ImportStmt)
			if !ok {
				continue
			}
			visit(filepath.Join(filepath.Dir(path), imp.Path.Value))
		}
	}
	visit(entryPath)

	modules := make([]compiler.Module, 0, len(loaded))
	for _, m := range loaded {
		modules = append(modules, m)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	return modules, diagnostics.SortAndDedupe(diags)
}

func printCommandResult(stdout io.Writer, cmd, format string, diags []diagnostics.Diagnostic, model *report.Model) error {
	switch format {
	case "pretty":
		for _, d := range diags {
			_, _ = fmt.Fprintf(stdout, "ERROR %s %s:%d:%d %s\n", d.Code, d.File, d.Line, d.Column, d.Message)
			if d.Hint != "" {
				_, _ = fmt.Fprintf(stdout, "  hint: %s\n", d.Hint)
			}
			if d.Related != nil {
				_, _ = fmt.Fprintf(stdout, "  related: %s:%d:%d %s\n", d.Related.File, d.Related.Line, d.Related.Column, d.Related.Message)
			}
		}
		if model != nil {
			_, _ = fmt.Fprintf(stdout, "flows=%d tests=%d failures=%d errors=%d\n", len(model.Suites), model.Summary.Tests, model.Summary.Failures, model.Summary.Errors)
		}
		if len(diags) == 0 && cmd == "eval" {
			_, _ = fmt.Fprintln(stdout, "OK")
		}
		return nil
	case "json":
		payload := map[string]any{"command": cmd, "ok": len(diags) == 0, "diagnostics": diags, "summary": map[string]int{"error_count": len(diags)}}
		if model != nil {
			payload["report"] = model
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	default:
		return fmt.Errorf("unknown --format %q (expected pretty|json)", format)
	}
}

func printUsage(stderr io.Writer) {
	_, _ = fmt.Fprintln(stderr, strings.TrimSpace(rootUsage()))
}

func rootUsage() string {
	return `Usage:
  ` + evalUsage + `
  ` + runUsage + `
  ` + requestUsage
}
