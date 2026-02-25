package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mehditeymorian/pipetest/internal/compiler"
	"github.com/mehditeymorian/pipetest/internal/parser"
)

func TestExecuteFlowChainWithHooksAndPropagation(t *testing.T) {
	order := []string{}
	firstHeader := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/first":
			firstHeader = r.Header.Get("X-From-Pre")
			_, _ = w.Write([]byte(`{"token":"abc"}`))
		case "/second/abc":
			_, _ = w.Write([]byte(`{"seen":"abc","fromHeader":"yes","final":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req first:
  GET /first
  pre hook {
    req.header["X-From-Pre"] = "yes"
  }
  ? status == 200
  let token = $.token

req second:
  GET /second/:token
  post hook {
    seen = $.seen
  }
  ? status == 200
  ? $.fromHeader == "yes"
  let final = $.final

flow "runtime-flow":
  first -> second : secondAlias
  ? secondAlias.res.seen == token
  ? final == "ok"
`
	plan := mustCompilePlan(t, "runtime-valid.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diags)
	}
	if got, want := strings.Join(order, ","), "/first,/second/abc"; got != want {
		t.Fatalf("unexpected execution order: got %s want %s", got, want)
	}
	if len(result.Flows) != 1 || len(result.Flows[0].Steps) != 2 {
		t.Fatalf("unexpected flow result: %+v", result.Flows)
	}
	if firstHeader != "yes" {
		t.Fatalf("expected pre hook header mutation, got %q", firstHeader)
	}
}

func TestExecuteSingleStepFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req ping:
  GET /health
  ? status == 200

flow "single-step":
  ping
  ? ping.status == 200
`
	plan := mustCompilePlan(t, "runtime-single-step.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diags)
	}
	if len(result.Flows) != 1 || len(result.Flows[0].Steps) != 1 {
		t.Fatalf("unexpected flow result: %+v", result.Flows)
	}
}

func TestExecuteTransportFailureDiagnostic(t *testing.T) {
	src := `
req only:
  GET http://127.0.0.1:1/unreachable

flow "broken":
  only -> only : again
`
	plan := mustCompilePlan(t, "runtime-invalid.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) == 0 {
		t.Fatalf("expected diagnostics")
	}
	if result.Diags[0].Code != "E_RUNTIME_TRANSPORT" {
		t.Fatalf("expected E_RUNTIME_TRANSPORT, got %s", result.Diags[0].Code)
	}
}

func mustCompilePlan(t *testing.T, path, src string) *compiler.Plan {
	t.Helper()
	prog, lexErrs, parseErrs := parser.Parse(path, src)
	if len(lexErrs) != 0 || len(parseErrs) != 0 {
		t.Fatalf("parse failed: lex=%+v parse=%+v", lexErrs, parseErrs)
	}
	plan, diags := compiler.Compile(path, []compiler.Module{{Path: path, Program: prog}})
	if len(diags) != 0 {
		t.Fatalf("compile failed: %+v", diags)
	}
	return plan
}
