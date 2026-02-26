package compiler

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/mehditeymorian/pipetest/internal/ast"
	"github.com/mehditeymorian/pipetest/internal/diagnostics"
)

var pathParamRE = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
var templateVarRE = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)

var builtins = map[string]struct{}{
	"env": {}, "uuid": {}, "len": {}, "jsonpath": {}, "regex": {}, "now": {}, "urlencode": {},
}

var reservedNames = map[string]struct{}{
	"req": {}, "res": {}, "status": {}, "header": {}, "$": {}, "#": {},
}

// Module binds a parsed program to its canonical path.
type Module struct {
	Path    string
	Program *ast.Program
}

// Plan is the validated execution plan IR.
type Plan struct {
	EntryPath string         `json:"entry_path"`
	Requests  []PlanRequest  `json:"requests"`
	Flows     []PlanFlow     `json:"flows"`
	Base      *string        `json:"-"`
	Timeout   *string        `json:"-"`
	Globals   []*ast.LetStmt `json:"-"`
}

// PlanRequest is a semantically validated request.
type PlanRequest struct {
	Name   string        `json:"name"`
	Parent *string       `json:"parent,omitempty"`
	HTTP   *ast.HttpLine `json:"http,omitempty"`
	Lines  []ast.ReqLine `json:"-"`
	Decl   *ast.ReqDecl  `json:"-"`
}

// PlanFlow is a semantically validated flow.
type PlanFlow struct {
	Name  string        `json:"name"`
	Steps []PlanStep    `json:"steps"`
	Lets  []string      `json:"lets"`
	Check []ast.Expr    `json:"-"`
	Span  ast.Span      `json:"-"`
	Decl  *ast.FlowDecl `json:"-"`
}

// PlanStep is one request invocation in a flow.
type PlanStep struct {
	Request string `json:"request"`
	Binding string `json:"binding"`
}

// Compile validates a module graph and returns a deterministic plan and diagnostics.
func Compile(entryPath string, modules []Module) (*Plan, []diagnostics.Diagnostic) {
	c := &compiler{
		entryPath: normalizePath(entryPath),
		modules:   map[string]*ast.Program{},
	}
	for _, m := range modules {
		c.modules[normalizePath(m.Path)] = m.Program
	}
	c.run()
	if len(c.diags) > 0 {
		return nil, diagnostics.SortAndDedupe(c.diags)
	}
	return c.plan, nil
}

type compiler struct {
	entryPath string
	modules   map[string]*ast.Program
	ordered   []string
	diags     []diagnostics.Diagnostic
	plan      *Plan

	reqs    map[string]*reqInfo
	effReqs map[string][]ast.ReqLine
	globals map[string]struct{}
}

type reqInfo struct {
	Decl *ast.ReqDecl
	File string
}

func (c *compiler) run() {
	c.passImports()
	c.passSymbols()
	c.passRequestInheritance()
	c.passRequests()
	c.passFlows()
	if len(c.diags) > 0 {
		return
	}
	c.buildPlan()
}

func (c *compiler) passRequestInheritance() {
	c.effReqs = map[string][]ast.ReqLine{}
	state := map[string]int{}

	var resolve func(name string) []ast.ReqLine
	resolve = func(name string) []ast.ReqLine {
		if lines, ok := c.effReqs[name]; ok {
			return lines
		}
		st := state[name]
		if st == 1 {
			req := c.reqs[name]
			if req != nil {
				c.addDiagAt("E_SEM_INHERITANCE_CYCLE", "request inheritance cycle detected", req.File, req.Decl.Span, "remove circular parent chains")
			}
			return nil
		}
		if st == 2 {
			return c.effReqs[name]
		}

		req := c.reqs[name]
		if req == nil {
			return nil
		}
		state[name] = 1
		var parent []ast.ReqLine
		if req.Decl.Parent != nil {
			parent = resolve(*req.Decl.Parent)
		}
		merged := mergeRequestLines(parent, req.Decl.Lines)
		c.effReqs[name] = merged
		state[name] = 2
		return merged
	}

	names := make([]string, 0, len(c.reqs))
	for name := range c.reqs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		resolve(name)
	}
}

func (c *compiler) passImports() {
	if _, ok := c.modules[c.entryPath]; !ok {
		c.addDiag("E_IMPORT_NOT_FOUND", "entry module not found", c.entryPath, ast.Span{}, "ensure the entry file is loaded")
		return
	}
	vis := map[string]int{}
	var dfs func(path string)
	dfs = func(path string) {
		state := vis[path]
		if state == 1 {
			c.addDiag("E_IMPORT_CYCLE", "import cycle detected", path, ast.Span{}, "remove circular imports")
			return
		}
		if state == 2 {
			return
		}
		vis[path] = 1
		prog := c.modules[path]
		for _, stmt := range prog.Stmts {
			imp, ok := stmt.(*ast.ImportStmt)
			if !ok {
				continue
			}
			target := normalizePath(filepath.Join(filepath.Dir(path), imp.Path.Value))
			if _, ok := c.modules[target]; !ok {
				c.addDiagAt("E_IMPORT_NOT_FOUND", fmt.Sprintf("import not found: %s", imp.Path.Value), path, imp.Span, "load the imported file")
				continue
			}
			dfs(target)
		}
		vis[path] = 2
		c.ordered = append(c.ordered, path)
	}
	dfs(c.entryPath)
	sort.Strings(c.ordered)
}

func (c *compiler) passSymbols() {
	c.reqs = map[string]*reqInfo{}
	flowNames := map[string]ast.Span{}
	c.globals = map[string]struct{}{}
	for _, path := range c.ordered {
		prog := c.modules[path]
		for _, stmt := range prog.Stmts {
			switch s := stmt.(type) {
			case *ast.FlowDecl:
				if path != c.entryPath {
					c.addDiagAt("E_IMPORT_FLOW_IN_IMPORTED_FILE", "flows are not allowed in imported files", path, s.Span, "move flow declarations to the entry file")
				}
				if prev, ok := flowNames[s.Name.Value]; ok {
					c.addRelatedDiag("E_SEM_DUPLICATE_FLOW_NAME", "duplicate flow name", path, s.Span, c.entryPath, prev, "rename one of the flow declarations")
				} else {
					flowNames[s.Name.Value] = s.Span
				}
			case *ast.ReqDecl:
				if prev, ok := c.reqs[s.Name]; ok {
					c.addRelatedDiag("E_SEM_DUPLICATE_REQ_NAME", "duplicate request name", path, s.Span, prev.File, prev.Decl.Span, "rename one of the request declarations")
				} else {
					c.reqs[s.Name] = &reqInfo{Decl: s, File: path}
				}
			case *ast.LetStmt:
				c.globals[s.Name] = struct{}{}
			}
		}
	}
	for _, req := range c.reqs {
		if req.Decl.Parent != nil {
			if _, ok := c.reqs[*req.Decl.Parent]; !ok {
				c.addDiagAt("E_SEM_UNKNOWN_PARENT_REQ", "unknown parent request", req.File, req.Decl.Span, "reference an existing request as parent")
			}
		}
	}
}

func (c *compiler) passRequests() {
	for _, req := range c.reqs {
		httpCount, jsonCount := 0, 0
		preHook, postHook := 0, 0
		lines := c.effReqs[req.Decl.Name]
		for _, line := range lines {
			switch l := line.(type) {
			case *ast.HttpLine:
				httpCount++
			case *ast.JsonDirective:
				jsonCount++
			case *ast.HookBlock:
				if l.Kind == ast.HookPre {
					preHook++
					if refsExprInHook(l, isResRef) {
						c.addDiagAt("E_SEM_PRE_HOOK_REFERENCES_RES", "pre hook cannot reference res", req.File, l.Span, "use req or flow variables in pre hook")
					}
					if refsExprInHook(l, isHashRef) {
						c.addDiagAt("E_SEM_PRE_HOOK_REFERENCES_RES", "pre hook cannot reference #", req.File, l.Span, "move response access to post hook")
					}
				}
				if l.Kind == ast.HookPost {
					postHook++
				}
				for _, stmt := range l.Stmts {
					if asn, ok := stmt.(*ast.AssignStmt); ok && asn.Target.Root.Kind == ast.LValueRes {
						c.addDiagAt("E_SEM_ASSIGN_TO_RES_FORBIDDEN", "assignment to res is forbidden", req.File, asn.Span, "assign to req or a variable instead")
					}
				}
			}
		}
		if httpCount == 0 {
			c.addDiagAt("E_SEM_REQ_MISSING_HTTP_LINE", "request must include exactly one HTTP line", req.File, req.Decl.Span, "add GET/POST/etc line")
		}
		if httpCount > 1 {
			c.addDiagAt("E_SEM_REQ_MULTIPLE_HTTP_LINES", "request has multiple HTTP lines", req.File, req.Decl.Span, "keep only one HTTP line")
		}
		if preHook > 1 {
			c.addDiagAt("E_SEM_DUPLICATE_PRE_HOOK", "request has multiple pre hooks", req.File, req.Decl.Span, "keep only one pre hook")
		}
		if postHook > 1 {
			c.addDiagAt("E_SEM_DUPLICATE_POST_HOOK", "request has multiple post hooks", req.File, req.Decl.Span, "keep only one post hook")
		}
		if jsonCount > 1 {
			c.addDiagAt("E_SEM_MULTIPLE_BODIES", "request has multiple body directives", req.File, req.Decl.Span, "keep only one json body directive")
		}
	}
}

func (c *compiler) passFlows() {
	for _, stmt := range c.modules[c.entryPath].Stmts {
		flow, ok := stmt.(*ast.FlowDecl)
		if !ok {
			continue
		}
		if len(flow.Chain) == 0 {
			c.addDiagAt("E_SEM_FLOW_MISSING_CHAIN", "flow must contain a chain", c.entryPath, flow.Span, "add a chain line using ->")
			continue
		}
		bindings := map[string]struct{}{}
		defined := map[string]struct{}{}
		for name := range c.globals {
			defined[name] = struct{}{}
		}
		for _, pre := range flow.Prelude {
			defined[pre.Name] = struct{}{}
		}
		for _, step := range flow.Chain {
			req, ok := c.reqs[step.ReqName]
			if !ok {
				c.addDiagAt("E_SEM_UNKNOWN_REQ_IN_FLOW", fmt.Sprintf("unknown request in flow: %s", step.ReqName), c.entryPath, step.Span, "reference an existing request")
				continue
			}
			binding := step.ReqName
			if step.Alias != nil {
				binding = *step.Alias
			}
			if _, ok := bindings[binding]; ok {
				c.addDiagAt("E_SEM_DUPLICATE_FLOW_BINDING", fmt.Sprintf("duplicate flow binding: %s", binding), c.entryPath, step.Span, "use unique aliases in the chain")
			} else {
				bindings[binding] = struct{}{}
			}
			required := c.requiredVars(c.effReqs[step.ReqName])
			for _, name := range required {
				if _, ok := defined[name]; !ok {
					code := "E_SEM_UNDEFINED_VARIABLE"
					if reqUsesPathParam(c.effReqs[step.ReqName], name) {
						code = "E_SEM_MISSING_PATH_PARAM_VAR"
					}
					c.addDiagAt(code, fmt.Sprintf("undefined variable: %s", name), req.File, req.Decl.Span, "define variable globally, in flow prelude, or in prior request lets")
				}
			}
			for _, line := range c.effReqs[step.ReqName] {
				if l, ok := line.(*ast.LetStmt); ok {
					defined[l.Name] = struct{}{}
				}
			}
		}
		for _, as := range flow.Asserts {
			for _, ident := range collectExprIdents(as.Expr) {
				if _, ok := defined[ident]; ok {
					continue
				}
				if _, ok := bindings[ident]; ok {
					continue
				}
				c.addDiagAt("E_SEM_UNKNOWN_FLOW_BINDING", fmt.Sprintf("unknown flow binding or variable: %s", ident), c.entryPath, as.Span, "use a binding from the chain or a defined variable")
			}
		}
	}
}

func (c *compiler) buildPlan() {
	plan := &Plan{EntryPath: c.entryPath}
	for _, stmt := range c.modules[c.entryPath].Stmts {
		switch s := stmt.(type) {
		case *ast.SettingStmt:
			switch v := s.Value.(type) {
			case *ast.StringLit:
				if s.Kind == ast.SettingBase {
					value := v.Value
					plan.Base = &value
				}
			case *ast.DurationLit:
				if s.Kind == ast.SettingTimeout {
					value := v.Raw
					plan.Timeout = &value
				}
			}
		case *ast.LetStmt:
			plan.Globals = append(plan.Globals, s)
		}
	}
	for name, req := range c.reqs {
		lines := c.effReqs[name]
		pr := PlanRequest{Name: name, Parent: req.Decl.Parent, Decl: req.Decl, Lines: lines}
		for _, line := range lines {
			if http, ok := line.(*ast.HttpLine); ok {
				pr.HTTP = http
				break
			}
		}
		plan.Requests = append(plan.Requests, pr)
	}
	sort.Slice(plan.Requests, func(i, j int) bool { return plan.Requests[i].Name < plan.Requests[j].Name })
	for _, stmt := range c.modules[c.entryPath].Stmts {
		flow, ok := stmt.(*ast.FlowDecl)
		if !ok {
			continue
		}
		pf := PlanFlow{Name: flow.Name.Value, Span: flow.Span, Decl: flow}
		for _, let := range flow.Prelude {
			pf.Lets = append(pf.Lets, let.Name)
		}
		for _, step := range flow.Chain {
			binding := step.ReqName
			if step.Alias != nil {
				binding = *step.Alias
			}
			pf.Steps = append(pf.Steps, PlanStep{Request: step.ReqName, Binding: binding})
		}
		for _, as := range flow.Asserts {
			pf.Check = append(pf.Check, as.Expr)
		}
		plan.Flows = append(plan.Flows, pf)
	}
	sort.Slice(plan.Flows, func(i, j int) bool { return plan.Flows[i].Name < plan.Flows[j].Name })
	c.plan = plan
}

func (c *compiler) addDiag(code, msg, file string, span ast.Span, hint string) {
	c.addDiagAt(code, msg, file, span, hint)
}

func (c *compiler) addDiagAt(code, msg, file string, span ast.Span, hint string) {
	c.diags = append(c.diags, diagnostics.Diagnostic{Severity: "error", Code: code, Message: msg, File: file, Line: span.Start.Line, Column: span.Start.Column, Hint: hint})
}

func (c *compiler) addRelatedDiag(code, msg, file string, span ast.Span, relatedFile string, related ast.Span, hint string) {
	c.diags = append(c.diags, diagnostics.Diagnostic{Severity: "error", Code: code, Message: msg, File: file, Line: span.Start.Line, Column: span.Start.Column, Hint: hint, Related: &diagnostics.Related{File: relatedFile, Line: related.Start.Line, Column: related.Start.Column, Message: "first declaration"}})
}

func refsExprInHook(block *ast.HookBlock, fn func(ast.Expr) bool) bool {
	for _, stmt := range block.Stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if fn(s.Value) {
				return true
			}
		case *ast.ExprStmt:
			if fn(s.Expr) {
				return true
			}
		case *ast.PrintStmt:
			for _, arg := range s.Args {
				if fn(arg) {
					return true
				}
			}
		}
	}
	return false
}

func isResRef(expr ast.Expr) bool {
	for _, id := range collectExprIdents(expr) {
		if id == "res" {
			return true
		}
	}
	return false
}

func isHashRef(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.HashExpr:
		return true
	case *ast.UnaryExpr:
		return isHashRef(e.X)
	case *ast.BinaryExpr:
		return isHashRef(e.Left) || isHashRef(e.Right)
	case *ast.CallExpr:
		if isHashRef(e.Callee) {
			return true
		}
		for _, a := range e.Args {
			if isHashRef(a) {
				return true
			}
		}
	case *ast.FieldExpr:
		return isHashRef(e.X)
	case *ast.IndexExpr:
		return isHashRef(e.X) || isHashRef(e.Index)
	case *ast.ParenExpr:
		return isHashRef(e.X)
	case *ast.ArrayLit:
		for _, el := range e.Elements {
			if isHashRef(el) {
				return true
			}
		}
	case *ast.ObjectLit:
		for _, p := range e.Pairs {
			if isHashRef(p.Value) {
				return true
			}
		}
	}
	return false
}

func (c *compiler) requiredVars(lines []ast.ReqLine) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, line := range lines {
		switch l := line.(type) {
		case *ast.HttpLine:
			for _, m := range pathParamRE.FindAllStringSubmatch(l.Path, -1) {
				add(m[1])
			}
			for _, name := range collectTemplateVarsInString(l.Path) {
				add(name)
			}
		case *ast.HeaderDirective:
			for _, name := range collectTemplateVarsInExpr(l.Value) {
				add(name)
			}
			for _, id := range collectExprIdents(l.Value) {
				add(id)
			}
		case *ast.QueryDirective:
			for _, name := range collectTemplateVarsInExpr(l.Value) {
				add(name)
			}
			for _, id := range collectExprIdents(l.Value) {
				add(id)
			}
		case *ast.AuthDirective:
			for _, name := range collectTemplateVarsInExpr(l.Value) {
				add(name)
			}
			for _, id := range collectExprIdents(l.Value) {
				add(id)
			}
		case *ast.JsonDirective:
			for _, name := range collectTemplateVarsInExpr(l.Value) {
				add(name)
			}
			for _, id := range collectExprIdents(l.Value) {
				add(id)
			}
		case *ast.AssertStmt:
			for _, name := range collectTemplateVarsInExpr(l.Expr) {
				add(name)
			}
			for _, id := range collectExprIdents(l.Expr) {
				add(id)
			}
		case *ast.LetStmt:
			for _, name := range collectTemplateVarsInExpr(l.Value) {
				add(name)
			}
			for _, id := range collectExprIdents(l.Value) {
				add(id)
			}
		case *ast.HookBlock:
			for _, s := range l.Stmts {
				switch hs := s.(type) {
				case *ast.AssignStmt:
					for _, name := range collectTemplateVarsInExpr(hs.Value) {
						add(name)
					}
					for _, id := range collectExprIdents(hs.Value) {
						add(id)
					}
				case *ast.ExprStmt:
					for _, name := range collectTemplateVarsInExpr(hs.Expr) {
						add(name)
					}
					for _, id := range collectExprIdents(hs.Expr) {
						add(id)
					}
				case *ast.PrintStmt:
					for _, arg := range hs.Args {
						for _, name := range collectTemplateVarsInExpr(arg) {
							add(name)
						}
						for _, id := range collectExprIdents(arg) {
							add(id)
						}
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func reqUsesPathParam(lines []ast.ReqLine, name string) bool {
	for _, line := range lines {
		http, ok := line.(*ast.HttpLine)
		if !ok {
			continue
		}
		for _, m := range pathParamRE.FindAllStringSubmatch(http.Path, -1) {
			if m[1] == name {
				return true
			}
		}
	}
	return false
}

func mergeRequestLines(parent, child []ast.ReqLine) []ast.ReqLine {
	type shape struct {
		http    *ast.HttpLine
		auth    *ast.AuthDirective
		json    *ast.JsonDirective
		pre     *ast.HookBlock
		post    *ast.HookBlock
		headers map[string]*ast.HeaderDirective
		headerK []string
		queries map[string]*ast.QueryDirective
		queryK  []string
		asserts []*ast.AssertStmt
		lets    map[string]*ast.LetStmt
		letK    []string
	}
	s := shape{headers: map[string]*ast.HeaderDirective{}, queries: map[string]*ast.QueryDirective{}, lets: map[string]*ast.LetStmt{}}

	applyLines := func(lines []ast.ReqLine, isChild bool) {
		childAsserts := []*ast.AssertStmt{}
		for _, line := range lines {
			switch l := line.(type) {
			case *ast.HttpLine:
				s.http = l
			case *ast.AuthDirective:
				s.auth = l
			case *ast.JsonDirective:
				s.json = l
			case *ast.HookBlock:
				if l.Kind == ast.HookPre {
					s.pre = l
				}
				if l.Kind == ast.HookPost {
					s.post = l
				}
			case *ast.HeaderDirective:
				key := l.Key.Name
				if _, ok := s.headers[key]; !ok {
					s.headerK = append(s.headerK, key)
				}
				s.headers[key] = l
			case *ast.QueryDirective:
				key := l.Key.Name
				if _, ok := s.queries[key]; !ok {
					s.queryK = append(s.queryK, key)
				}
				s.queries[key] = l
			case *ast.AssertStmt:
				if isChild {
					childAsserts = append(childAsserts, l)
				} else {
					s.asserts = append(s.asserts, l)
				}
			case *ast.LetStmt:
				if _, ok := s.lets[l.Name]; !ok {
					s.letK = append(s.letK, l.Name)
				}
				s.lets[l.Name] = l
			}
		}
		if isChild && len(childAsserts) > 0 {
			s.asserts = childAsserts
		}
	}

	applyLines(parent, false)
	applyLines(child, true)

	out := []ast.ReqLine{}
	if s.http != nil {
		out = append(out, s.http)
	}
	if s.auth != nil {
		out = append(out, s.auth)
	}
	for _, key := range s.headerK {
		out = append(out, s.headers[key])
	}
	for _, key := range s.queryK {
		out = append(out, s.queries[key])
	}
	if s.json != nil {
		out = append(out, s.json)
	}
	if s.pre != nil {
		out = append(out, s.pre)
	}
	if s.post != nil {
		out = append(out, s.post)
	}
	for _, as := range s.asserts {
		out = append(out, as)
	}
	for _, key := range s.letK {
		out = append(out, s.lets[key])
	}
	return out
}

func collectExprIdents(expr ast.Expr) []string {
	seen := map[string]struct{}{}
	var out []string
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.IdentExpr:
			if _, skip := builtins[n.Name]; skip {
				return
			}
			if _, skip := reservedNames[n.Name]; skip {
				return
			}
			if _, ok := seen[n.Name]; !ok {
				seen[n.Name] = struct{}{}
				out = append(out, n.Name)
			}
		case *ast.UnaryExpr:
			walk(n.X)
		case *ast.BinaryExpr:
			walk(n.Left)
			walk(n.Right)
		case *ast.CallExpr:
			walk(n.Callee)
			for _, a := range n.Args {
				walk(a)
			}
		case *ast.FieldExpr:
			walk(n.X)
		case *ast.IndexExpr:
			walk(n.X)
			walk(n.Index)
		case *ast.ParenExpr:
			walk(n.X)
		case *ast.ArrayLit:
			for _, el := range n.Elements {
				walk(el)
			}
		case *ast.ObjectLit:
			for _, p := range n.Pairs {
				walk(p.Value)
			}
		}
	}
	walk(expr)
	sort.Strings(out)
	return out
}

func collectTemplateVarsInString(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, m := range templateVarRE.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func collectTemplateVarsInExpr(expr ast.Expr) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.StringLit:
			for _, name := range collectTemplateVarsInString(n.Value) {
				add(name)
			}
		case *ast.UnaryExpr:
			walk(n.X)
		case *ast.BinaryExpr:
			walk(n.Left)
			walk(n.Right)
		case *ast.CallExpr:
			walk(n.Callee)
			for _, a := range n.Args {
				walk(a)
			}
		case *ast.FieldExpr:
			walk(n.X)
		case *ast.IndexExpr:
			walk(n.X)
			walk(n.Index)
		case *ast.ParenExpr:
			walk(n.X)
		case *ast.ArrayLit:
			for _, el := range n.Elements {
				walk(el)
			}
		case *ast.ObjectLit:
			for _, p := range n.Pairs {
				walk(p.Value)
			}
		}
	}

	walk(expr)
	sort.Strings(out)
	return out
}

func normalizePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
