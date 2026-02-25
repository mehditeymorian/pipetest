package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalSuccess(t *testing.T) {
	dir := t.TempDir()
	program := `
req ping:
  GET https://example.com

flow "ok":
  ping -> ping:again
`
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	var out, errOut strings.Builder
	exitCode := run([]string{"eval", path}, &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Fatalf("expected OK output, got %q", out.String())
	}
}

func TestRunWritesReportsOnFailure(t *testing.T) {
	dir := t.TempDir()
	reportDir := filepath.Join(dir, "artifacts")
	program := `
req only:
  GET http://127.0.0.1:1/unreachable

flow "broken":
  only -> only:again
`
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	var out, errOut strings.Builder
	exitCode := run([]string{"run", "--report-dir", reportDir, path}, &out, &errOut)
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%s", exitCode, errOut.String())
	}
	for _, name := range []string{"pipetest-junit.xml", "pipetest-report.xml", "pipetest-report.json"} {
		if _, err := os.Stat(filepath.Join(reportDir, name)); err != nil {
			t.Fatalf("expected report %s: %v", name, err)
		}
	}
}

func TestRunSuccessSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	reportDir := filepath.Join(dir, "artifacts")
	program := "\nreq only:\n  GET " + srv.URL + "\n\nflow \"ok\":\n  only -> only:again\n"
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	var out, errOut strings.Builder
	exitCode := run([]string{"run", "--report-dir", reportDir, path}, &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "flows=1 tests=2 failures=0 errors=0") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestUnknownCommandUsage(t *testing.T) {
	var out, errOut strings.Builder
	exitCode := run([]string{"bogus"}, &out, &errOut)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("expected unknown command error, got %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errOut.String())
	}
}

func TestMissingCommandUsage(t *testing.T) {
	var out, errOut strings.Builder
	exitCode := run(nil, &out, &errOut)
	if exitCode != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errOut.String())
	}
}
