package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mehditeymorian/pipetest/internal/compiler"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
	"github.com/mehditeymorian/pipetest/internal/runtime"
)

// Model is the report model used for JSON and JUnit output.
type Model struct {
	Suites  []Suite `json:"suites"`
	Summary Summary `json:"summary"`
}

type Summary struct {
	Tests    int `json:"tests"`
	Failures int `json:"failures"`
	Errors   int `json:"errors"`
}

type Suite struct {
	Name      string     `json:"name"`
	Testcases []Testcase `json:"testcases"`
	Summary   Summary    `json:"summary"`
}

type Testcase struct {
	Name    string `json:"name"`
	Flow    string `json:"flow,omitempty"`
	Request string `json:"request,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func Build(plan *compiler.Plan, result runtime.Result) Model {
	if plan == nil {
		return Model{}
	}

	byFlow := map[string][]diagnostics.Diagnostic{}
	for _, d := range result.Diags {
		flow := ""
		if d.Flow != nil {
			flow = *d.Flow
		}
		if flow == "" {
			flow = "global"
		}
		byFlow[flow] = append(byFlow[flow], d)
	}

	model := Model{}
	for _, flow := range plan.Flows {
		suite := Suite{Name: flow.Name}
		stepIndex := 0
		for _, step := range flow.Decl.Chain {
			stepIndex++
			display := step.ReqName
			canonical := step.ReqName
			if step.Alias != nil {
				display = fmt.Sprintf("%s:%s", step.ReqName, *step.Alias)
				canonical = display
			}
			tc := Testcase{Name: fmt.Sprintf("%d %s", stepIndex, display), Flow: flow.Name, Request: canonical, Status: "passed"}
			if d := firstDiagFor(byFlow[flow.Name], canonical); d != nil {
				tc.Status = statusForCode(d.Code)
				tc.Message = diagMessage(*d)
			}
			suite.Testcases = append(suite.Testcases, tc)
		}

		flowAssertIndex := 0
		for _, d := range byFlow[flow.Name] {
			if d.Request != nil {
				continue
			}
			flowAssertIndex++
			tc := Testcase{
				Name:    fmt.Sprintf("flow :: assert %d", flowAssertIndex),
				Flow:    flow.Name,
				Status:  statusForCode(d.Code),
				Message: diagMessage(d),
			}
			suite.Testcases = append(suite.Testcases, tc)
		}
		suite.Summary = summarize(suite.Testcases)
		model.Suites = append(model.Suites, suite)
	}
	model.Summary = summarizeSuites(model.Suites)
	return model
}

func firstDiagFor(diags []diagnostics.Diagnostic, request string) *diagnostics.Diagnostic {
	for _, d := range diags {
		if d.Request != nil && *d.Request == request {
			copyD := d
			return &copyD
		}
	}
	return nil
}

func statusForCode(code string) string {
	if strings.HasPrefix(code, "E_ASSERT_") {
		return "failure"
	}
	return "error"
}

func diagMessage(d diagnostics.Diagnostic) string {
	return fmt.Sprintf("%s @ %s:%d:%d", d.Message, d.File, d.Line, d.Column)
}

func summarize(cases []Testcase) Summary {
	s := Summary{Tests: len(cases)}
	for _, tc := range cases {
		switch tc.Status {
		case "failure":
			s.Failures++
		case "error":
			s.Errors++
		}
	}
	return s
}

func summarizeSuites(suites []Suite) Summary {
	s := Summary{}
	for _, suite := range suites {
		s.Tests += suite.Summary.Tests
		s.Failures += suite.Summary.Failures
		s.Errors += suite.Summary.Errors
	}
	return s
}

func WriteJSONFile(path string, model Model) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(model)
}

func WriteJUnitFile(path string, model Model) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	top := junitSuites{Suites: make([]junitSuite, 0, len(model.Suites))}
	for _, s := range model.Suites {
		js := junitSuite{Name: s.Name, Tests: s.Summary.Tests, Failures: s.Summary.Failures, Errors: s.Summary.Errors}
		for _, tc := range s.Testcases {
			jtc := junitCase{Name: tc.Name}
			if tc.Status == "failure" {
				jtc.Failure = &junitFailure{Message: tc.Message}
			}
			if tc.Status == "error" {
				jtc.Error = &junitError{Message: tc.Message}
			}
			js.Cases = append(js.Cases, jtc)
		}
		top.Suites = append(top.Suites, js)
	}
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if _, err := f.WriteString(xml.Header); err != nil {
		return err
	}
	return enc.Encode(top)
}

type junitSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string        `xml:"name,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
	Error   *junitError   `xml:"error,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
}

type junitError struct {
	Message string `xml:"message,attr"`
}
