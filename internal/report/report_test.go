package report

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/mehditeymorian/pipetest/internal/ast"
	"github.com/mehditeymorian/pipetest/internal/compiler"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
	"github.com/mehditeymorian/pipetest/internal/runtime"
)

func TestBuildNilPlanReturnsEmptyModel(t *testing.T) {
	got := Build(nil, runtime.Result{})
	if len(got.Suites) != 0 || got.Summary != (Summary{}) {
		t.Fatalf("expected empty model for nil plan, got %+v", got)
	}
}

func TestBuildMapsRequestAndFlowDiagnostics(t *testing.T) {
	alias := "checkout"
	flow := "smoke"
	reqDiag := diagnostics.Diagnostic{Code: "E_ASSERT_EXPECTED_TRUE", Message: "request assertion failed", File: "a.pt", Line: 11, Column: 5, Flow: &flow, Request: strPtr("getUser:" + alias)}
	flowDiag := diagnostics.Diagnostic{Code: "E_RUNTIME_TRANSPORT", Message: "transport", File: "a.pt", Line: 12, Column: 6, Flow: &flow}

	plan := &compiler.Plan{
		Flows: []compiler.PlanFlow{
			{
				Name: flow,
				Decl: &ast.FlowDecl{Chain: []ast.FlowStep{{ReqName: "getUser", Alias: &alias}}},
			},
		},
	}

	model := Build(plan, runtime.Result{Diags: []diagnostics.Diagnostic{flowDiag, reqDiag}})
	if len(model.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(model.Suites))
	}
	suite := model.Suites[0]
	if len(suite.Testcases) != 2 {
		t.Fatalf("expected request row + flow assertion row, got %+v", suite.Testcases)
	}
	if suite.Testcases[0].Name != "1 getUser:checkout" || suite.Testcases[0].Status != "failure" {
		t.Fatalf("unexpected mapped request testcase: %+v", suite.Testcases[0])
	}
	if suite.Testcases[1].Name != "flow :: assert 1" || suite.Testcases[1].Status != "error" {
		t.Fatalf("unexpected mapped flow testcase: %+v", suite.Testcases[1])
	}
	if suite.Summary.Tests != 2 || suite.Summary.Failures != 1 || suite.Summary.Errors != 1 {
		t.Fatalf("unexpected suite summary: %+v", suite.Summary)
	}
	if model.Summary != suite.Summary {
		t.Fatalf("expected top-level summary to match single suite, got %+v vs %+v", model.Summary, suite.Summary)
	}
}

func TestBuildUsesGlobalBucketForDiagnosticsWithoutFlow(t *testing.T) {
	plan := &compiler.Plan{
		Flows: []compiler.PlanFlow{
			{Name: "flow-1", Decl: &ast.FlowDecl{Chain: []ast.FlowStep{{ReqName: "a"}}}},
		},
	}
	res := runtime.Result{Diags: []diagnostics.Diagnostic{{Code: "E_RUNTIME_TRANSPORT", Message: "global", File: "a.pt", Line: 1, Column: 1}}}
	model := Build(plan, res)
	if model.Summary.Tests != 1 || model.Summary.Errors != 0 {
		t.Fatalf("expected global diagnostics to not be attached to unrelated flow suite, got %+v", model)
	}
	if model.Suites[0].Testcases[0].Status != "passed" {
		t.Fatalf("expected request testcase to remain passed, got %+v", model.Suites[0].Testcases[0])
	}
}

func TestWriteJSONAndJUnitFiles(t *testing.T) {
	model := Model{
		Suites: []Suite{{
			Name:      "smoke",
			Testcases: []Testcase{{Name: "1 ping", Status: "passed"}, {Name: "flow :: assert 1", Status: "failure", Message: "boom"}},
			Summary:   Summary{Tests: 2, Failures: 1},
		}},
		Summary: Summary{Tests: 2, Failures: 1},
	}

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "nested", "report.json")
	xmlPath := filepath.Join(dir, "nested", "report.xml")

	if err := WriteJSONFile(jsonPath, model); err != nil {
		t.Fatalf("WriteJSONFile failed: %v", err)
	}
	if err := WriteJUnitFile(xmlPath, model); err != nil {
		t.Fatalf("WriteJUnitFile failed: %v", err)
	}

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json failed: %v", err)
	}
	var gotModel Model
	if err := json.Unmarshal(jsonBytes, &gotModel); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if gotModel.Summary.Tests != 2 || gotModel.Summary.Failures != 1 {
		t.Fatalf("unexpected json content: %+v", gotModel)
	}

	xmlBytes, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Fatalf("read xml failed: %v", err)
	}
	if len(xmlBytes) == 0 || string(xmlBytes[:5]) != "<?xml" {
		t.Fatalf("expected xml header, got %q", string(xmlBytes))
	}
	var suites junitSuites
	if err := xml.Unmarshal(xmlBytes, &suites); err != nil {
		t.Fatalf("xml unmarshal failed: %v", err)
	}
	if len(suites.Suites) != 1 || suites.Suites[0].Failures != 1 {
		t.Fatalf("unexpected junit suites: %+v", suites)
	}
	if suites.Suites[0].Cases[1].Failure == nil {
		t.Fatalf("expected failure element for failing testcase: %+v", suites.Suites[0].Cases[1])
	}
}

func strPtr(s string) *string { return &s }
