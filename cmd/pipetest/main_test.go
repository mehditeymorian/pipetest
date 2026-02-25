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
	program := "\nreq only:\n\tGET " + srv.URL + "\n\nflow \"ok\":\n\tonly -> only:again\n"
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

func TestRequestCommandRunsSingleRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	program := "\nreq only:\n\tGET " + srv.URL + "\n\nreq other:\n\tGET " + srv.URL + "\n"
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	var out, errOut strings.Builder
	exitCode := run([]string{"request", path, "only"}, &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", exitCode, errOut.String())
	}
	if strings.Contains(out.String(), "ERROR") {
		t.Fatalf("unexpected diagnostics: %q", out.String())
	}
}

func TestRunPrintsAssertionResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	reportDir := filepath.Join(dir, "artifacts")
	program := "\nreq only:\n\tGET " + srv.URL + "\n\t? status == 200\n\nflow \"ok\":\n\tonly\n\t? only.status == 200\n"
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}

	var out, errOut strings.Builder
	exitCode := run([]string{"run", "--report-dir", reportDir, path}, &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "- flow ok") {
		t.Fatalf("expected flow node output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "  - only") {
		t.Fatalf("expected request node output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "    - assertion status == 200 ✅") {
		t.Fatalf("expected request assertion output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "  - assertion only.status == 200 ✅") {
		t.Fatalf("expected flow assertion output, got %q", out.String())
	}
}

func TestRunHidePassingAssertionsFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	reportDir := filepath.Join(dir, "artifacts")
	program := "\nreq only:\n\tGET " + srv.URL + "\n\t? status == 200\n\nflow \"ok\":\n\tonly\n"
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}

	var out, errOut strings.Builder
	exitCode := run([]string{"run", "--hide-passing-assertions", "--report-dir", reportDir, path}, &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", exitCode, errOut.String())
	}
	if strings.Contains(out.String(), "assertion status == 200 ✅") {
		t.Fatalf("did not expect successful assertion output, got %q", out.String())
	}
}

func TestRunAssertionFailureSkipsPrettyDiagnosticLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	reportDir := filepath.Join(dir, "artifacts")
	program := "\nreq only:\n\tGET " + srv.URL + "\n\t? status == 200\n\nflow \"broken\":\n\tonly\n"
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}

	var out, errOut strings.Builder
	exitCode := run([]string{"run", "--report-dir", reportDir, path}, &out, &errOut)
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "    - assertion status == 200 ❌") {
		t.Fatalf("expected failed assertion output, got %q", out.String())
	}
	if strings.Contains(out.String(), "E_ASSERT_EXPECTED_TRUE") {
		t.Fatalf("did not expect assertion diagnostic output, got %q", out.String())
	}
	if strings.Contains(out.String(), "request assertion failed") {
		t.Fatalf("did not expect request assertion failure message, got %q", out.String())
	}
}

func TestRunVerboseLogging(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	reportDir := filepath.Join(dir, "artifacts")
	program := "\nreq only:\n\tGET " + srv.URL + "\n\nflow \"ok\":\n\tonly\n"
	path := filepath.Join(dir, "program.pt")
	if err := os.WriteFile(path, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	var out, errOut strings.Builder
	exitCode := run([]string{"run", "--verbose", "--report-dir", reportDir, path}, &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", exitCode, errOut.String())
	}
	if !strings.Contains(out.String(), "[verbose]") {
		t.Fatalf("expected verbose output, got %q", out.String())
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
