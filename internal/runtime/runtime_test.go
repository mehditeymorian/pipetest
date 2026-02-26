package runtime

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mehditeymorian/pipetest/internal/compiler"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
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
	let token = #.token

req second:
	GET /second/:token
	post hook {
	  seen = #.seen
	}
	? status == 200
	? #.fromHeader == "yes"
	let final = #.final

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

func TestExecuteSingleStepFlowWithRelativePathWithoutLeadingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req ping:
	GET health
	? status == 200

flow "single-step":
	ping
	? ping.status == 200
`
	plan := mustCompilePlan(t, "runtime-single-step-relative.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diags)
	}
	if len(result.Flows) != 1 || len(result.Flows[0].Steps) != 1 {
		t.Fatalf("unexpected flow result: %+v", result.Flows)
	}
}

func TestExecuteFlowWithNilDecl(t *testing.T) {
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
`
	plan := mustCompilePlan(t, "runtime-nil-decl.pt", src)
	plan.Flows[0].Decl = nil

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

func TestCombineURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{name: "absolute-url", base: "https://api.example.com", path: "https://override.example.com/health", want: "https://override.example.com/health"},
		{name: "relative-path-with-leading-slash", base: "https://api.example.com", path: "/health", want: "https://api.example.com/health"},
		{name: "relative-path-without-leading-slash", base: "https://api.example.com", path: "health", want: "https://api.example.com/health"},
		{name: "no-base", base: "", path: "health", want: "health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := combineURL(tt.base, tt.path); got != tt.want {
				t.Fatalf("combineURL(%q, %q) = %q; want %q", tt.base, tt.path, got, tt.want)
			}
		})
	}
}

func TestExecuteHookPrintStatements(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"abc"}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req only:
	GET /print
	post hook {
	  print "token="
	  println #.token
	  printf "status=%d", status
	}
	? status == 200

flow "print-flow":
	only
	? only.status == 200
`
	plan := mustCompilePlan(t, "runtime-print.pt", src)
	out := captureStdout(t, func() {
		result := Execute(context.Background(), plan, Options{})
		if len(result.Diags) != 0 {
			t.Fatalf("expected no diagnostics, got %+v", result.Diags)
		}
	})
	if !strings.Contains(out, "status=200") {
		t.Fatalf("expected formatted status output, got %q", out)
	}
	if strings.Contains(out, "%!") {
		t.Fatalf("unexpected fmt mismatch output: %q", out)
	}
}

func TestExecuteHookPrintfMathExpressionWithPercentD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req only:
	GET /get
	post hook {
	  printf "sum 2 + 2 is %d", 2 + 2
	}
	? status == 200

flow "print-int":
	only
	? only.status == 200
`
	plan := mustCompilePlan(t, "runtime-print-int.pt", src)
	out := captureStdout(t, func() {
		result := Execute(context.Background(), plan, Options{})
		if len(result.Diags) != 0 {
			t.Fatalf("expected no diagnostics, got %+v", result.Diags)
		}
	})
	if !strings.Contains(out, "sum 2 + 2 is 4") {
		t.Fatalf("expected math-expression formatted output, got %q", out)
	}
	if strings.Contains(out, "%!d(") {
		t.Fatalf("unexpected fmt mismatch output: %q", out)
	}
}

func TestExecuteHookPrintStatementsTemplateVariables(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

let token = "abc123"
let audience = "orders"

req only:
	GET /print
	post hook {
	  print "audience={{audience}} "
	  println "token={{token}}"
	  printf "status=%d token=%s", status, "{{token}}"
	}
	? status == 200

flow "print-template-vars":
	only
`
	plan := mustCompilePlan(t, "runtime-print-template-vars.pt", src)
	out := captureStdout(t, func() {
		result := Execute(context.Background(), plan, Options{})
		if len(result.Diags) != 0 {
			t.Fatalf("expected no diagnostics, got %+v", result.Diags)
		}
	})
	if !strings.Contains(out, "audience=orders token=abc123") {
		t.Fatalf("expected interpolated print output, got %q", out)
	}
	if !strings.Contains(out, "status=200 token=abc123") {
		t.Fatalf("expected interpolated printf output, got %q", out)
	}
}

func TestExecuteBuiltinUtilityFunctions(t *testing.T) {
	t.Setenv("PIPETEST_EMAIL", "qa+dev@example.com")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"name":"alice"},"items":[{"id":7}]}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req builtins:
	GET /get
	? regex("^qa.+dev", env("PIPETEST_EMAIL"))
	? jsonpath(#, "$.user.name") == "alice"
	? jsonpath(#, "$.items[0].id") == 7
	? urlencode(env("PIPETEST_EMAIL")) == "qa%2Bdev%40example.com"
	? len(now()) > 10
	? len(uuid()) == 32

flow "builtins":
	builtins
	? builtins.status == 200
`

	plan := mustCompilePlan(t, "runtime-builtins.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diags)
	}
}

func TestExecuteTemplateVariablesInStrings(t *testing.T) {
	tokenSeen := ""
	msgSeen := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenSeen = r.Header.Get("Authorization")
		msgSeen = r.URL.Query().Get("msg")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

let token = "abc123"
let audience = "orders"

req list_orders:
	GET /orders/{{audience}}
	header Authorization = "Bearer {{token}}"
	query msg = "hello-{{audience}}"
	json { tokenValue: "{{token}}" }
	? status == 200

flow "template-vars":
	list_orders
	? list_orders.status == 200
`
	plan := mustCompilePlan(t, "runtime-template-vars.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diags)
	}
	if tokenSeen != "Bearer abc123" {
		t.Fatalf("expected templated authorization header, got %q", tokenSeen)
	}
	if msgSeen != "hello-orders" {
		t.Fatalf("expected templated query value, got %q", msgSeen)
	}
}

func TestCompileTemplateVariablesMissingDiagnostic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req list_orders:
	GET /orders
	header Authorization = "Bearer {{token}}"

flow "template-vars-missing":
	list_orders
`
	_, diags := compilePlan(t, "runtime-template-vars-missing.pt", src)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics")
	}
	if diags[0].Code != "E_SEM_UNDEFINED_VARIABLE" {
		t.Fatalf("expected E_SEM_UNDEFINED_VARIABLE, got %s", diags[0].Code)
	}
}

func TestCompileHookPrintTemplateVariableMissingDiagnostic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"

req only:
	GET /print
	post hook {
	  println "token={{token}}"
	}
	? status == 200

flow "print-template-vars-missing":
	only
`
	_, diags := compilePlan(t, "runtime-print-template-vars-missing.pt", src)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics")
	}
	if diags[0].Code != "E_SEM_UNDEFINED_VARIABLE" {
		t.Fatalf("expected E_SEM_UNDEFINED_VARIABLE, got %s", diags[0].Code)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
		_ = r.Close()
	}()

	fn()

	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	return buf.String()
}

func mustCompilePlan(t *testing.T, path, src string) *compiler.Plan {
	t.Helper()
	plan, diags := compilePlan(t, path, src)
	if len(diags) != 0 {
		t.Fatalf("compile failed: %+v", diags)
	}
	return plan
}

func compilePlan(t *testing.T, path, src string) (*compiler.Plan, []diagnostics.Diagnostic) {
	t.Helper()
	prog, lexErrs, parseErrs := parser.Parse(path, src)
	if len(lexErrs) != 0 || len(parseErrs) != 0 {
		t.Fatalf("parse failed: lex=%+v parse=%+v", lexErrs, parseErrs)
	}
	return compiler.Compile(path, []compiler.Module{{Path: path, Program: prog}})
}

func TestExecuteRequestInheritanceChildOverridesParent(t *testing.T) {
	fromPre := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fromPre = r.Header.Get("X-From-Pre")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"child","value":"child"}`))
	}))
	defer srv.Close()

	src := `
base "` + srv.URL + `"
let id = "abc"

req parent:
	GET /parent/:id
	header XReq = "parent"
	pre hook {
	  req.header["X-From-Pre"] = "parent"
	}
	post hook {
	  seen = "parent"
	}
	? status == 201
	let token = "parent"

req child(parent):
	GET /child/:id
	header XReq = "child"
	pre hook {
	  req.header["X-From-Pre"] = "child"
	}
	post hook {
	  seen = #.value
	}
	? status == 200
	let token = #.token

flow "inheritance":
	child
	? token == "child"
	? child.res.value == "child"
`
	plan := mustCompilePlan(t, "runtime-inheritance-override.pt", src)
	result := Execute(context.Background(), plan, Options{})
	if len(result.Diags) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diags)
	}
	if fromPre != "child" {
		t.Fatalf("expected child pre hook header, got %q", fromPre)
	}
}
