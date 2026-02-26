package compiler

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mehditeymorian/pipetest/internal/ast"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
	"github.com/mehditeymorian/pipetest/internal/parser"
)

var update = flag.Bool("update-compiler", false, "update compiler golden files")

func TestCompileValidPlan(t *testing.T) {
	tests := []struct {
		name   string
		entry  string
		golden string
	}{
		{name: "multi-step-flow", entry: "../../testdata/compiler/valid/compile-single-flow.pt", golden: "../../testdata/compiler/golden/compile-single-flow.plan.json"},
		{name: "single-step-flow", entry: "../../testdata/compiler/valid/compile-single-step-flow.pt", golden: "../../testdata/compiler/golden/compile-single-step-flow.plan.json"},
		{name: "template-request-context-hooks", entry: "../../testdata/compiler/valid/template-request-context-in-hooks.pt", golden: "../../testdata/compiler/golden/template-request-context-in-hooks.plan.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mods := loadModules(t, tc.entry)
			plan, diags := Compile(tc.entry, mods)
			if len(diags) != 0 {
				t.Fatalf("expected no diagnostics, got %+v", diags)
			}
			got, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			assertGolden(t, tc.golden, got)
		})
	}
}

func TestCompileInvalidDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		entry  string
		files  []string
		golden string
	}{
		{name: "duplicate-request", entry: "../../testdata/compiler/invalid/duplicate-request-name.pt", files: []string{"../../testdata/compiler/invalid/duplicate-request-name.pt"}, golden: "../../testdata/compiler/golden/duplicate-request-name.errors.json"},
		{name: "undefined-path-var", entry: "../../testdata/compiler/invalid/undefined-variable-in-path.pt", files: []string{"../../testdata/compiler/invalid/undefined-variable-in-path.pt"}, golden: "../../testdata/compiler/golden/undefined-variable-in-path.errors.json"},
		{name: "undefined-template-var", entry: "../../testdata/compiler/invalid/undefined-variable-in-template.pt", files: []string{"../../testdata/compiler/invalid/undefined-variable-in-template.pt"}, golden: "../../testdata/compiler/golden/undefined-variable-in-template.errors.json"},
		{name: "template-status-in-pre-hook", entry: "../../testdata/compiler/invalid/template-status-in-pre-hook.pt", files: []string{"../../testdata/compiler/invalid/template-status-in-pre-hook.pt"}, golden: "../../testdata/compiler/golden/template-status-in-pre-hook.errors.json"},
		{name: "import-cycle", entry: "../../testdata/compiler/invalid/import-cycle-a.pt", files: []string{"../../testdata/compiler/invalid/import-cycle-a.pt", "../../testdata/compiler/invalid/import-cycle-b.pt"}, golden: "../../testdata/compiler/golden/import-cycle.errors.json"},
		{name: "inheritance-cycle", entry: "../../testdata/compiler/invalid/inheritance-cycle.pt", files: []string{"../../testdata/compiler/invalid/inheritance-cycle.pt"}, golden: "../../testdata/compiler/golden/inheritance-cycle.errors.json"},
		{name: "undefined-inherited-path-var", entry: "../../testdata/compiler/invalid/undefined-variable-in-inherited-path.pt", files: []string{"../../testdata/compiler/invalid/undefined-variable-in-inherited-path.pt"}, golden: "../../testdata/compiler/golden/undefined-variable-in-inherited-path.errors.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mods := loadModules(t, tc.files...)
			_, diags := Compile(tc.entry, mods)
			if len(diags) == 0 {
				t.Fatalf("expected diagnostics")
			}
			got, err := json.MarshalIndent(diags, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			assertGolden(t, tc.golden, got)
		})
	}
}

func loadModules(t *testing.T, paths ...string) []Module {
	t.Helper()
	sort.Strings(paths)
	mods := make([]Module, 0, len(paths))
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		prog := parseProgram(t, path, string(src))
		mods = append(mods, Module{Path: path, Program: prog})
	}
	return mods
}

func parseProgram(t *testing.T, path, src string) *ast.Program {
	t.Helper()
	prog, lexErrs, parseErrs := parser.Parse(path, src)
	if len(lexErrs) > 0 || len(parseErrs) > 0 {
		t.Fatalf("parse failed lex=%v parse=%v", lexErrs, parseErrs)
	}
	return prog
}

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestDiagnosticsSortAndDedup(t *testing.T) {
	in := []diagnostics.Diagnostic{
		{Code: "B", File: "b.pt", Line: 2, Column: 1, Message: "m", Severity: "error"},
		{Code: "A", File: "a.pt", Line: 1, Column: 1, Message: "m", Severity: "error"},
		{Code: "A", File: "a.pt", Line: 1, Column: 1, Message: "m", Severity: "error"},
	}
	out := diagnostics.SortAndDedupe(in)
	if len(out) != 2 {
		t.Fatalf("expected dedupe to keep 2, got %d", len(out))
	}
	if out[0].File != "a.pt" {
		t.Fatalf("unexpected sort order: %+v", out)
	}
}
