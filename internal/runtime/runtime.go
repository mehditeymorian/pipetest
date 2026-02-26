package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mehditeymorian/pipetest/internal/ast"
	"github.com/mehditeymorian/pipetest/internal/compiler"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
)

var pathParamRuntimeRE = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
var templateVarRuntimeRE = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)

type Options struct {
	BaseOverride              *string
	TimeoutOverride           *time.Duration
	Client                    *http.Client
	Verbose                   bool
	LogWriter                 io.Writer
	SuppressPassingAssertions bool
}

type Result struct {
	Flows []FlowResult
	Diags []diagnostics.Diagnostic
}

type FlowResult struct {
	Name  string
	Steps []StepResult
}

type StepResult struct {
	Request string
	Binding string
	Status  int
}

type flowBinding struct {
	Res    any
	Req    map[string]any
	Status int
	Header map[string]any
}

type requestContext struct {
	reqObj    map[string]any
	flowVars  map[string]any
	resJSON   any
	status    int
	headers   map[string]any
	flowViews map[string]flowBinding
}

func Execute(ctx context.Context, plan *compiler.Plan, opt Options) Result {
	res := Result{}
	if plan == nil {
		return res
	}
	assertionLog := newAssertionLogger(opt)
	client := opt.Client
	if client == nil {
		client = &http.Client{}
	}
	if d := resolveTimeout(plan, opt); d > 0 {
		client.Timeout = d
	}
	requests := map[string]compiler.PlanRequest{}
	for _, req := range plan.Requests {
		requests[req.Name] = req
	}
	globals := map[string]any{}
	for _, g := range plan.Globals {
		val, err := evalExpr(g.Value, requestContext{flowVars: globals})
		if err != nil {
			res.Diags = append(res.Diags, runtimeDiag("E_RUNTIME_EXPRESSION", fmt.Sprintf("failed to evaluate global let %s", g.Name), plan.EntryPath, g.Span, err.Error(), "", ""))
			continue
		}
		globals[g.Name] = val
	}

	for _, flow := range plan.Flows {
		verbosef(opt, "flow %q: start", flow.Name)
		fr := FlowResult{Name: flow.Name}
		flowVars := copyMap(globals)
		prelude := []*ast.LetStmt{}
		asserts := []*ast.AssertStmt{}
		if flow.Decl != nil {
			prelude = flow.Decl.Prelude
			asserts = flow.Decl.Asserts
		}
		for _, pre := range prelude {
			val, err := evalExpr(pre.Value, requestContext{flowVars: flowVars})
			if err != nil {
				res.Diags = append(res.Diags, runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate flow prelude let", plan.EntryPath, pre.Span, err.Error(), flow.Name, ""))
				continue
			}
			flowVars[pre.Name] = val
		}
		flowViews := map[string]flowBinding{}
		for _, step := range flow.Steps {
			verbosef(opt, "flow %q: request %q (binding=%q) start", flow.Name, step.Request, step.Binding)
			pr, ok := requests[step.Request]
			if !ok {
				res.Diags = append(res.Diags, runtimeDiag("E_RUNTIME_UNKNOWN_REQUEST", "request not found in runtime plan", plan.EntryPath, flow.Span, step.Request, flow.Name, step.Request))
				continue
			}
			stepResult, diag := executeRequest(ctx, plan, pr, step, flow.Name, flowVars, flowViews, client, opt, assertionLog)
			if diag != nil {
				res.Diags = append(res.Diags, *diag)
				continue
			}
			flowViews[step.Binding] = flowBinding{Res: stepResult.res, Req: stepResult.reqSnapshot, Status: stepResult.status, Header: stepResult.headers}
			fr.Steps = append(fr.Steps, StepResult{Request: step.Request, Binding: step.Binding, Status: stepResult.status})
			verbosef(opt, "flow %q: request %q done (status=%d)", flow.Name, step.Binding, stepResult.status)
		}
		for _, as := range asserts {
			v, err := evalExpr(as.Expr, requestContext{flowVars: flowVars, flowViews: flowViews})
			if err != nil {
				assertionLog.log(flow.Name, "", as.Expr, false)
				res.Diags = append(res.Diags, runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate flow assertion", plan.EntryPath, as.Span, err.Error(), flow.Name, ""))
				continue
			}
			ok, cast := asBool(v)
			assertionLog.log(flow.Name, "", as.Expr, cast == nil && ok)
			if cast != nil || !ok {
				hint := "assertion must evaluate to true"
				if cast != nil {
					hint = cast.Error()
				}
				res.Diags = append(res.Diags, runtimeDiag("E_ASSERT_EXPECTED_TRUE", "flow assertion failed", plan.EntryPath, as.Span, hint, flow.Name, ""))
			}
		}
		res.Flows = append(res.Flows, fr)
		verbosef(opt, "flow %q: done", flow.Name)
	}

	return res
}

type stepExecutionResult struct {
	status      int
	headers     map[string]any
	res         any
	reqSnapshot map[string]any
}

func executeRequest(ctx context.Context, plan *compiler.Plan, req compiler.PlanRequest, step compiler.PlanStep, flowName string, flowVars map[string]any, flowViews map[string]flowBinding, client *http.Client, opt Options, assertionLog *assertionLogger) (*stepExecutionResult, *diagnostics.Diagnostic) {
	lines := resolveLines(req, plan)
	requestID := stepDisplayName(step)
	httpLine := req.HTTP
	if httpLine == nil {
		return nil, ptr(runtimeDiag("E_RUNTIME_REQUEST_SHAPE", "missing http line at runtime", plan.EntryPath, req.Decl.Span, "compiler should ensure requests contain one HTTP line", flowName, requestID))
	}
	base := ""
	if plan.Base != nil {
		base = *plan.Base
	}
	if opt.BaseOverride != nil {
		base = *opt.BaseOverride
	}
	pathWithTemplates, err := interpolateString(httpLine.Path, flowVars)
	if err != nil {
		return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render request path", plan.EntryPath, httpLine.Span, err.Error(), flowName, requestID))
	}
	path, err := renderPath(pathWithTemplates, flowVars)
	if err != nil {
		return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_PATH_PARAM", err.Error(), plan.EntryPath, httpLine.Span, "define the missing variable in global/flow/request scope", flowName, requestID))
	}
	urlStr := combineURL(base, path)
	reqObj := map[string]any{
		"method": httpMethodString(httpLine.Method),
		"url":    urlStr,
		"header": map[string]any{},
		"query":  map[string]any{},
		"json":   nil,
	}
	rctx := requestContext{reqObj: reqObj, flowVars: flowVars, flowViews: flowViews}

	for _, line := range lines {
		h, ok := line.(*ast.HookBlock)
		if !ok || h.Kind != ast.HookPre {
			continue
		}
		if err := execHook(h, rctx); err != nil {
			if isMissingTemplateVariableError(err) {
				return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render pre hook print statement", plan.EntryPath, h.Span, err.Error(), flowName, requestID))
			}
			return nil, ptr(runtimeDiag("E_RUNTIME_HOOK", "pre hook execution failed", plan.EntryPath, h.Span, err.Error(), flowName, requestID))
		}
	}
	for _, line := range lines {
		switch l := line.(type) {
		case *ast.HeaderDirective:
			v, err := evalExpr(l.Value, rctx)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate header directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			v, err = interpolateValue(v, flowVars)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render header directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			reqObj["header"].(map[string]any)[l.Key.Name] = fmt.Sprint(v)
		case *ast.QueryDirective:
			v, err := evalExpr(l.Value, rctx)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate query directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			v, err = interpolateValue(v, flowVars)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render query directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			reqObj["query"].(map[string]any)[l.Key.Name] = fmt.Sprint(v)
		case *ast.AuthDirective:
			v, err := evalExpr(l.Value, rctx)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate auth directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			v, err = interpolateValue(v, flowVars)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render auth directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			if l.Scheme == ast.AuthBearer {
				reqObj["header"].(map[string]any)["Authorization"] = "Bearer " + fmt.Sprint(v)
			}
		case *ast.JsonDirective:
			v, err := evalExpr(l.Value, rctx)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate json directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			v, err = interpolateValue(v, flowVars)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render json directive", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			reqObj["json"] = v
		}
	}
	finalURL := applyQuery(reqObj["url"].(string), reqObj["query"].(map[string]any))
	reqObj["url"] = finalURL
	body := io.Reader(nil)
	if reqObj["json"] != nil {
		raw, err := json.Marshal(reqObj["json"])
		if err != nil {
			return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to serialize json body", plan.EntryPath, req.Decl.Span, err.Error(), flowName, requestID))
		}
		body = bytes.NewReader(raw)
		reqObj["header"].(map[string]any)["Content-Type"] = "application/json"
	}
	httpReq, err := http.NewRequestWithContext(ctx, reqObj["method"].(string), reqObj["url"].(string), body)
	if err != nil {
		return nil, ptr(runtimeDiag("E_RUNTIME_TRANSPORT", "failed to build request", plan.EntryPath, req.Decl.Span, err.Error(), flowName, requestID))
	}
	for k, v := range reqObj["header"].(map[string]any) {
		httpReq.Header.Set(k, fmt.Sprint(v))
	}
	httpRes, err := client.Do(httpReq)
	if err != nil {
		return nil, ptr(runtimeDiag("E_RUNTIME_TRANSPORT", "http request failed", plan.EntryPath, req.Decl.Span, err.Error(), flowName, requestID))
	}
	defer func() { _ = httpRes.Body.Close() }()
	respRaw, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, ptr(runtimeDiag("E_RUNTIME_TRANSPORT", "failed to read response", plan.EntryPath, req.Decl.Span, err.Error(), flowName, requestID))
	}
	var resJSON any
	if len(bytes.TrimSpace(respRaw)) > 0 {
		if err := json.Unmarshal(respRaw, &resJSON); err != nil {
			return nil, ptr(runtimeDiag("E_RUNTIME_TRANSPORT", "response is not valid json", plan.EntryPath, req.Decl.Span, err.Error(), flowName, requestID))
		}
	}
	headers := map[string]any{}
	for k, vals := range httpRes.Header {
		if len(vals) == 1 {
			headers[k] = vals[0]
		} else {
			arr := make([]any, 0, len(vals))
			for _, v := range vals {
				arr = append(arr, v)
			}
			headers[k] = arr
		}
	}
	rctx.resJSON = resJSON
	rctx.status = httpRes.StatusCode
	rctx.headers = headers

	for _, line := range lines {
		h, ok := line.(*ast.HookBlock)
		if !ok || h.Kind != ast.HookPost {
			continue
		}
		if err := execHook(h, rctx); err != nil {
			if isMissingTemplateVariableError(err) {
				return nil, ptr(runtimeDiag("E_RUNTIME_MISSING_VARIABLE", "failed to render post hook print statement", plan.EntryPath, h.Span, err.Error(), flowName, requestID))
			}
			return nil, ptr(runtimeDiag("E_RUNTIME_HOOK", "post hook execution failed", plan.EntryPath, h.Span, err.Error(), flowName, requestID))
		}
	}
	for _, line := range lines {
		switch l := line.(type) {
		case *ast.AssertStmt:
			v, err := evalExpr(l.Expr, rctx)
			if err != nil {
				assertionLog.log(flowName, requestID, l.Expr, false)
				return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate request assertion", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			ok, cast := asBool(v)
			assertionLog.log(flowName, requestID, l.Expr, cast == nil && ok)
			if cast != nil || !ok {
				hint := "assertion must evaluate to true"
				if cast != nil {
					hint = cast.Error()
				}
				return nil, ptr(runtimeDiag("E_ASSERT_EXPECTED_TRUE", "request assertion failed", plan.EntryPath, l.Span, hint, flowName, requestID))
			}
		case *ast.LetStmt:
			v, err := evalExpr(l.Value, rctx)
			if err != nil {
				return nil, ptr(runtimeDiag("E_RUNTIME_EXPRESSION", "failed to evaluate request let", plan.EntryPath, l.Span, err.Error(), flowName, requestID))
			}
			flowVars[l.Name] = v
		}
	}
	return &stepExecutionResult{status: httpRes.StatusCode, headers: headers, res: resJSON, reqSnapshot: copyMap(reqObj)}, nil
}

func verbosef(opt Options, format string, args ...any) {
	if !opt.Verbose || opt.LogWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(opt.LogWriter, "[verbose] "+format+"\n", args...)
}

type assertionLogger struct {
	writer               io.Writer
	suppressPassing      bool
	currentFlowName      string
	currentRequestTarget string
}

func newAssertionLogger(opt Options) *assertionLogger {
	if opt.LogWriter == nil {
		return nil
	}
	return &assertionLogger{
		writer:          opt.LogWriter,
		suppressPassing: opt.SuppressPassingAssertions,
	}
}

func (l *assertionLogger) log(flowName, requestTarget string, expr ast.Expr, ok bool) {
	if l == nil {
		return
	}
	if ok && l.suppressPassing {
		return
	}
	status := "❌"
	if ok {
		status = "✅"
	}
	if flowName != "" && flowName != l.currentFlowName {
		_, _ = fmt.Fprintf(l.writer, "- flow %s\n", flowName)
		l.currentFlowName = flowName
		l.currentRequestTarget = ""
	}
	if requestTarget != "" {
		if requestTarget != l.currentRequestTarget {
			_, _ = fmt.Fprintf(l.writer, "  - %s\n", requestTarget)
			l.currentRequestTarget = requestTarget
		}
		_, _ = fmt.Fprintf(l.writer, "    - assertion %s %s\n", formatExpr(expr), status)
		return
	}
	l.currentRequestTarget = ""
	_, _ = fmt.Fprintf(l.writer, "  - assertion %s %s\n", formatExpr(expr), status)
}

func stepDisplayName(step compiler.PlanStep) string {
	if step.Binding == "" || step.Binding == step.Request {
		return step.Request
	}
	return step.Request + ":" + step.Binding
}

func formatExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StringLit:
		return strconv.Quote(e.Value)
	case *ast.NumberLit:
		return e.Raw
	case *ast.BoolLit:
		if e.Value {
			return "true"
		}
		return "false"
	case *ast.NullLit:
		return "null"
	case *ast.ArrayLit:
		parts := make([]string, 0, len(e.Elements))
		for _, el := range e.Elements {
			parts = append(parts, formatExpr(el))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *ast.ObjectLit:
		parts := make([]string, 0, len(e.Pairs))
		for _, pair := range e.Pairs {
			parts = append(parts, pair.Key.Name+": "+formatExpr(pair.Value))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *ast.DollarExpr:
		return "$"
	case *ast.HashExpr:
		return "#"
	case *ast.IdentExpr:
		return e.Name
	case *ast.ParenExpr:
		return "(" + formatExpr(e.X) + ")"
	case *ast.UnaryExpr:
		return unaryOpString(e.Op) + formatExpr(e.X)
	case *ast.BinaryExpr:
		return formatExpr(e.Left) + " " + binaryOpString(e.Op) + " " + formatExpr(e.Right)
	case *ast.FieldExpr:
		return formatExpr(e.X) + "." + e.Name
	case *ast.IndexExpr:
		return formatExpr(e.X) + "[" + formatExpr(e.Index) + "]"
	case *ast.CallExpr:
		parts := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			parts = append(parts, formatExpr(arg))
		}
		return formatExpr(e.Callee) + "(" + strings.Join(parts, ", ") + ")"
	default:
		return "<expr>"
	}
}

func unaryOpString(op ast.UnaryOp) string {
	switch op {
	case ast.UnaryNot:
		return "!"
	case ast.UnaryMinus:
		return "-"
	case ast.UnaryPlus:
		return "+"
	default:
		return ""
	}
}

func binaryOpString(op ast.BinaryOp) string {
	switch op {
	case ast.BinaryEq:
		return "=="
	case ast.BinaryNe:
		return "!="
	case ast.BinaryGt:
		return ">"
	case ast.BinaryGte:
		return ">="
	case ast.BinaryLt:
		return "<"
	case ast.BinaryLte:
		return "<="
	case ast.BinaryAnd:
		return "&&"
	case ast.BinaryOr:
		return "||"
	case ast.BinaryContains:
		return "contains"
	case ast.BinaryIn:
		return "in"
	case ast.BinaryAdd:
		return "+"
	case ast.BinarySub:
		return "-"
	case ast.BinaryMul:
		return "*"
	case ast.BinaryDiv:
		return "/"
	case ast.BinaryMod:
		return "%"
	default:
		return "?"
	}
}

func resolveLines(req compiler.PlanRequest, plan *compiler.Plan) []ast.ReqLine {
	if len(req.Lines) > 0 {
		return req.Lines
	}
	if req.Decl == nil {
		return nil
	}
	if req.Parent == nil {
		return req.Decl.Lines
	}
	seen := map[string]bool{}
	var build func(name string) []ast.ReqLine
	requestMap := map[string]compiler.PlanRequest{}
	for _, r := range plan.Requests {
		requestMap[r.Name] = r
	}
	build = func(name string) []ast.ReqLine {
		r, ok := requestMap[name]
		if !ok || r.Decl == nil || seen[name] {
			return nil
		}
		seen[name] = true
		lines := []ast.ReqLine{}
		if r.Parent != nil {
			lines = append(lines, build(*r.Parent)...)
		}
		lines = append(lines, r.Decl.Lines...)
		return lines
	}
	return build(req.Name)
}

func resolveTimeout(plan *compiler.Plan, opt Options) time.Duration {
	if opt.TimeoutOverride != nil {
		return *opt.TimeoutOverride
	}
	if plan.Timeout == nil {
		return 0
	}
	d, err := time.ParseDuration(*plan.Timeout)
	if err != nil {
		return 0
	}
	return d
}

func renderPath(path string, vars map[string]any) (string, error) {
	for _, m := range pathParamRuntimeRE.FindAllStringSubmatch(path, -1) {
		if _, ok := vars[m[1]]; !ok {
			return "", fmt.Errorf("missing variable %s for path param", m[1])
		}
	}
	out := pathParamRuntimeRE.ReplaceAllStringFunc(path, func(token string) string {
		name := strings.TrimPrefix(token, ":")
		v := vars[name]
		return url.PathEscape(fmt.Sprint(v))
	})
	return out, nil
}

type missingTemplateVariableError struct {
	name string
}

func (e *missingTemplateVariableError) Error() string {
	return fmt.Sprintf("missing variable %s for template placeholder", e.name)
}

func interpolateString(in string, vars map[string]any) (string, error) {
	out := in
	for _, m := range templateVarRuntimeRE.FindAllStringSubmatch(in, -1) {
		if _, ok := vars[m[1]]; !ok {
			return "", &missingTemplateVariableError{name: m[1]}
		}
		out = strings.ReplaceAll(out, m[0], fmt.Sprint(vars[m[1]]))
	}
	return out, nil
}

func isMissingTemplateVariableError(err error) bool {
	var target *missingTemplateVariableError
	return errors.As(err, &target)
}

func interpolateValue(v any, vars map[string]any) (any, error) {
	switch x := v.(type) {
	case string:
		return interpolateString(x, vars)
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			rendered, err := interpolateValue(item, vars)
			if err != nil {
				return nil, err
			}
			out = append(out, rendered)
		}
		return out, nil
	case map[string]any:
		out := map[string]any{}
		for k, item := range x {
			rendered, err := interpolateValue(item, vars)
			if err != nil {
				return nil, err
			}
			out[k] = rendered
		}
		return out, nil
	default:
		return v, nil
	}
}

func combineURL(base, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func applyQuery(urlStr string, q map[string]any) string {
	if len(q) == 0 {
		return urlStr
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	query := u.Query()
	for k, v := range q {
		query.Set(k, fmt.Sprint(v))
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func execHook(block *ast.HookBlock, rctx requestContext) error {
	for _, stmt := range block.Stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			v, err := evalExpr(s.Value, rctx)
			if err != nil {
				return err
			}
			if err := assignLValue(s.Target, v, rctx); err != nil {
				return err
			}
		case *ast.ExprStmt:
			if _, err := evalExpr(s.Expr, rctx); err != nil {
				return err
			}
		case *ast.PrintStmt:
			if err := execPrintStmt(s, rctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func execPrintStmt(stmt *ast.PrintStmt, rctx requestContext) error {
	args := make([]any, 0, len(stmt.Args))
	for _, arg := range stmt.Args {
		v, err := evalExpr(arg, rctx)
		if err != nil {
			return err
		}
		v, err = interpolateValue(v, rctx.flowVars)
		if err != nil {
			return err
		}
		args = append(args, v)
	}
	switch stmt.Kind {
	case ast.Print:
		fmt.Print(args...)
	case ast.Println:
		fmt.Println(args...)
	case ast.Printf:
		if len(args) == 0 {
			return fmt.Errorf("printf expects at least one argument")
		}
		format := fmt.Sprint(args[0])
		fmt.Printf(format, normalizePrintfArgs(format, args[1:])...)
	}
	return nil
}

func normalizePrintfArgs(format string, args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	copy(out, args)

	argIndex := 0
	for i := 0; i < len(format) && argIndex < len(out); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i >= len(format) {
			break
		}
		if format[i] == '%' {
			continue
		}
		for i < len(format) && strings.ContainsRune("#0- +", rune(format[i])) {
			i++
		}
		if i < len(format) && format[i] == '*' {
			out[argIndex] = coercePrintfIntArg(out[argIndex])
			argIndex++
			i++
		} else {
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}
		if i < len(format) && format[i] == '.' {
			i++
			if i < len(format) && format[i] == '*' {
				if argIndex < len(out) {
					out[argIndex] = coercePrintfIntArg(out[argIndex])
					argIndex++
				}
				i++
			} else {
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					i++
				}
			}
		}
		if i >= len(format) || argIndex >= len(out) {
			break
		}
		switch format[i] {
		case 'b', 'c', 'd', 'o', 'O', 'U', 'x', 'X':
			out[argIndex] = coercePrintfIntArg(out[argIndex])
		}
		argIndex++
	}
	return out
}

func coercePrintfIntArg(v any) any {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f {
		return v
	}
	if f < math.MinInt64 || f > math.MaxInt64 {
		return v
	}
	return int64(f)
}

func assignLValue(target *ast.LValue, value any, rctx requestContext) error {
	if target == nil {
		return fmt.Errorf("nil assignment target")
	}
	if target.Root.Kind == ast.LValueIdent && len(target.Postfix) == 0 {
		rctx.flowVars[target.Root.Name] = value
		return nil
	}
	var current any
	switch target.Root.Kind {
	case ast.LValueReq:
		current = rctx.reqObj
	case ast.LValueIdent:
		current = rctx.flowVars[target.Root.Name]
	default:
		return fmt.Errorf("unsupported assignment target")
	}
	for i, pf := range target.Postfix {
		last := i == len(target.Postfix)-1
		switch pf.Kind {
		case ast.LValueField:
			obj, ok := current.(map[string]any)
			if !ok {
				return fmt.Errorf("field assignment on non-object")
			}
			if last {
				obj[pf.Name] = value
				return nil
			}
			next, ok := obj[pf.Name]
			if !ok {
				next = map[string]any{}
				obj[pf.Name] = next
			}
			current = next
		case ast.LValueIndex:
			obj, ok := current.(map[string]any)
			if !ok {
				return fmt.Errorf("index assignment on non-object")
			}
			idx, err := evalExpr(pf.Index, rctx)
			if err != nil {
				return err
			}
			key := fmt.Sprint(idx)
			if last {
				obj[key] = value
				return nil
			}
			next, ok := obj[key]
			if !ok {
				next = map[string]any{}
				obj[key] = next
			}
			current = next
		}
	}
	return nil
}

func evalExpr(expr ast.Expr, rctx requestContext) (any, error) {
	switch e := expr.(type) {
	case *ast.StringLit:
		return e.Value, nil
	case *ast.NumberLit:
		f, err := strconv.ParseFloat(e.Raw, 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	case *ast.BoolLit:
		return e.Value, nil
	case *ast.NullLit:
		return nil, nil
	case *ast.ArrayLit:
		arr := make([]any, 0, len(e.Elements))
		for _, el := range e.Elements {
			v, err := evalExpr(el, rctx)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case *ast.ObjectLit:
		obj := map[string]any{}
		for _, p := range e.Pairs {
			v, err := evalExpr(p.Value, rctx)
			if err != nil {
				return nil, err
			}
			obj[p.Key.Name] = v
		}
		return obj, nil
	case *ast.DollarExpr:
		return rctx.reqObj, nil
	case *ast.HashExpr:
		return rctx.resJSON, nil
	case *ast.IdentExpr:
		switch e.Name {
		case "status":
			return float64(rctx.status), nil
		case "header":
			return rctx.headers, nil
		case "req":
			return rctx.reqObj, nil
		case "res":
			return rctx.resJSON, nil
		}
		if v, ok := rctx.flowVars[e.Name]; ok {
			return v, nil
		}
		if b, ok := rctx.flowViews[e.Name]; ok {
			return map[string]any{"res": b.Res, "req": b.Req, "status": float64(b.Status), "header": b.Header}, nil
		}
		return nil, fmt.Errorf("undefined identifier %s", e.Name)
	case *ast.ParenExpr:
		return evalExpr(e.X, rctx)
	case *ast.UnaryExpr:
		x, err := evalExpr(e.X, rctx)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case ast.UnaryNot:
			b, err := asBool(x)
			if err != nil {
				return nil, err
			}
			return !b, nil
		case ast.UnaryMinus:
			n, err := asNumber(x)
			if err != nil {
				return nil, err
			}
			return -n, nil
		case ast.UnaryPlus:
			return asNumber(x)
		}
	case *ast.BinaryExpr:
		left, err := evalExpr(e.Left, rctx)
		if err != nil {
			return nil, err
		}
		right, err := evalExpr(e.Right, rctx)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case ast.BinaryEq:
			return deepEqual(left, right), nil
		case ast.BinaryNe:
			return !deepEqual(left, right), nil
		case ast.BinaryGt:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l > r, nil
		case ast.BinaryGte:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l >= r, nil
		case ast.BinaryLt:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l < r, nil
		case ast.BinaryLte:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l <= r, nil
		case ast.BinaryAnd:
			l, err := asBool(left)
			if err != nil {
				return nil, err
			}
			r, err := asBool(right)
			if err != nil {
				return nil, err
			}
			return l && r, nil
		case ast.BinaryOr:
			l, err := asBool(left)
			if err != nil {
				return nil, err
			}
			r, err := asBool(right)
			if err != nil {
				return nil, err
			}
			return l || r, nil
		case ast.BinaryContains:
			return contains(left, right), nil
		case ast.BinaryIn:
			arr, ok := right.([]any)
			if !ok {
				return nil, fmt.Errorf("in requires array on right side")
			}
			for _, item := range arr {
				if deepEqual(left, item) {
					return true, nil
				}
			}
			return false, nil
		case ast.BinaryAdd:
			if ls, ok := left.(string); ok {
				return ls + fmt.Sprint(right), nil
			}
			if rs, ok := right.(string); ok {
				return fmt.Sprint(left) + rs, nil
			}
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l + r, nil
		case ast.BinarySub:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l - r, nil
		case ast.BinaryMul:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			return l * r, nil
		case ast.BinaryDiv:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			if r == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return l / r, nil
		case ast.BinaryMod:
			l, err := asNumber(left)
			if err != nil {
				return nil, err
			}
			r, err := asNumber(right)
			if err != nil {
				return nil, err
			}
			if r == 0 {
				return nil, fmt.Errorf("modulo by zero")
			}
			return math.Mod(l, r), nil
		}
	case *ast.FieldExpr:
		x, err := evalExpr(e.X, rctx)
		if err != nil {
			return nil, err
		}
		obj, ok := x.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("field access on non-object")
		}
		return obj[e.Name], nil
	case *ast.IndexExpr:
		x, err := evalExpr(e.X, rctx)
		if err != nil {
			return nil, err
		}
		idx, err := evalExpr(e.Index, rctx)
		if err != nil {
			return nil, err
		}
		switch v := x.(type) {
		case map[string]any:
			return v[fmt.Sprint(idx)], nil
		case []any:
			n, err := asNumber(idx)
			if err != nil {
				return nil, err
			}
			i := int(n)
			if i < 0 || i >= len(v) {
				return nil, fmt.Errorf("index out of range")
			}
			return v[i], nil
		default:
			return nil, fmt.Errorf("index access on unsupported type")
		}
	case *ast.CallExpr:
		callee, ok := e.Callee.(*ast.IdentExpr)
		if !ok {
			return nil, fmt.Errorf("callee must be identifier")
		}
		args := make([]any, 0, len(e.Args))
		for _, a := range e.Args {
			v, err := evalExpr(a, rctx)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		switch callee.Name {
		case "env":
			if len(args) != 1 {
				return nil, fmt.Errorf("env expects 1 arg")
			}
			return os.Getenv(fmt.Sprint(args[0])), nil
		case "uuid":
			if len(args) != 0 {
				return nil, fmt.Errorf("uuid expects no args")
			}
			return randomID(), nil
		case "len":
			if len(args) != 1 {
				return nil, fmt.Errorf("len expects 1 arg")
			}
			switch v := args[0].(type) {
			case []any:
				return float64(len(v)), nil
			case map[string]any:
				return float64(len(v)), nil
			case string:
				return float64(len(v)), nil
			default:
				return nil, fmt.Errorf("len unsupported for type")
			}
		case "regex":
			if len(args) != 2 {
				return nil, fmt.Errorf("regex expects 2 args")
			}
			re, err := regexp.Compile(fmt.Sprint(args[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid regex: %w", err)
			}
			return re.MatchString(fmt.Sprint(args[1])), nil
		case "jsonpath":
			if len(args) != 2 {
				return nil, fmt.Errorf("jsonpath expects 2 args")
			}
			return jsonPathLookup(args[0], fmt.Sprint(args[1]))
		case "now":
			if len(args) != 0 {
				return nil, fmt.Errorf("now expects no args")
			}
			return time.Now().UTC().Format(time.RFC3339Nano), nil
		case "urlencode":
			if len(args) != 1 {
				return nil, fmt.Errorf("urlencode expects 1 arg")
			}
			return url.QueryEscape(fmt.Sprint(args[0])), nil
		default:
			return nil, fmt.Errorf("unknown function %s", callee.Name)
		}
	}
	return nil, fmt.Errorf("unsupported expression")
}

func asNumber(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case string:
		return strconv.ParseFloat(n, 64)
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func asBool(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expected boolean")
	}
	return b, nil
}

func contains(left, right any) bool {
	switch v := left.(type) {
	case string:
		return strings.Contains(v, fmt.Sprint(right))
	case []any:
		for _, item := range v {
			if deepEqual(item, right) {
				return true
			}
		}
		return false
	}
	return false
}

func deepEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}

func copyMap[V any](in map[string]V) map[string]V {
	out := map[string]V{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func httpMethodString(m ast.HttpMethod) string {
	switch m {
	case ast.MethodGet:
		return http.MethodGet
	case ast.MethodPost:
		return http.MethodPost
	case ast.MethodPut:
		return http.MethodPut
	case ast.MethodPatch:
		return http.MethodPatch
	case ast.MethodDelete:
		return http.MethodDelete
	case ast.MethodHead:
		return http.MethodHead
	case ast.MethodOptions:
		return http.MethodOptions
	default:
		return http.MethodGet
	}
}

func runtimeDiag(code, message, file string, span ast.Span, hint, flow, req string) diagnostics.Diagnostic {
	d := diagnostics.Diagnostic{Severity: "error", Code: code, Message: message, File: file, Line: span.Start.Line, Column: span.Start.Column, Hint: hint}
	if flow != "" {
		d.Flow = ptr(flow)
	}
	if req != "" {
		d.Request = ptr(req)
	}
	return d
}

func jsonPathLookup(root any, path string) (any, error) {
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("jsonpath must start with $")
	}
	cur := root
	i := 1
	for i < len(path) {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && ((path[i] >= 'a' && path[i] <= 'z') || (path[i] >= 'A' && path[i] <= 'Z') || (path[i] >= '0' && path[i] <= '9') || path[i] == '_') {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("invalid jsonpath segment")
			}
			obj, ok := cur.(map[string]any)
			if !ok {
				return nil, nil
			}
			cur = obj[path[start:i]]
		case '[':
			i++
			start := i
			for i < len(path) && path[i] >= '0' && path[i] <= '9' {
				i++
			}
			if start == i || i >= len(path) || path[i] != ']' {
				return nil, fmt.Errorf("invalid jsonpath index")
			}
			idx, err := strconv.Atoi(path[start:i])
			if err != nil {
				return nil, fmt.Errorf("invalid jsonpath index: %w", err)
			}
			i++
			arr, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, nil
			}
			cur = arr[idx]
		default:
			return nil, fmt.Errorf("invalid jsonpath syntax")
		}
	}
	return cur, nil
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
func ptr[T any](v T) *T { return &v }
