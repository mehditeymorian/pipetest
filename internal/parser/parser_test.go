package parser

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mehditeymorian/pipetest/internal/ast"
)

var updateGolden = flag.Bool("update", false, "update golden files")

type nodeSnapshot struct {
	Type   string                 `json:"type"`
	Span   spanSnapshot           `json:"span"`
	Fields map[string]interface{} `json:"fields,omitempty"`
}

type spanSnapshot struct {
	Start positionSnapshot `json:"start"`
	End   positionSnapshot `json:"end"`
}

type positionSnapshot struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

func snapshotSpan(span ast.Span) spanSnapshot {
	return spanSnapshot{
		Start: positionSnapshot{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column},
		End:   positionSnapshot{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column},
	}
}

func snapshotNode(node interface{}) interface{} {
	switch n := node.(type) {
	case *ast.Program:
		return nodeSnapshot{
			Type: "Program",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"stmts": snapshotStmtList(n.Stmts),
			},
		}
	case *ast.SettingStmt:
		return nodeSnapshot{
			Type: "SettingStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"kind":  settingKindString(n.Kind),
				"value": snapshotNode(n.Value),
			},
		}
	case *ast.ImportStmt:
		return nodeSnapshot{
			Type: "ImportStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"path": snapshotNode(n.Path),
			},
		}
	case *ast.LetStmt:
		return nodeSnapshot{
			Type: "LetStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"name":  n.Name,
				"value": snapshotNode(n.Value),
			},
		}
	case *ast.ReqDecl:
		return nodeSnapshot{
			Type: "ReqDecl",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"name":   n.Name,
				"parent": n.Parent,
				"lines":  snapshotReqLines(n.Lines),
			},
		}
	case *ast.FlowDecl:
		return nodeSnapshot{
			Type: "FlowDecl",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"name":    snapshotNode(n.Name),
				"prelude": snapshotLetList(n.Prelude),
				"chain":   snapshotFlowSteps(n.Chain),
				"asserts": snapshotAssertList(n.Asserts),
			},
		}
	case *ast.HttpLine:
		return nodeSnapshot{
			Type: "HttpLine",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"method": httpMethodString(n.Method),
				"path":   n.Path,
			},
		}
	case *ast.JsonDirective:
		return nodeSnapshot{
			Type: "JsonDirective",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"value": snapshotNode(n.Value),
			},
		}
	case *ast.HeaderDirective:
		return nodeSnapshot{
			Type: "HeaderDirective",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"key":   snapshotKey(n.Key),
				"value": snapshotNode(n.Value),
			},
		}
	case *ast.QueryDirective:
		return nodeSnapshot{
			Type: "QueryDirective",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"key":   snapshotKey(n.Key),
				"value": snapshotNode(n.Value),
			},
		}
	case *ast.AuthDirective:
		return nodeSnapshot{
			Type: "AuthDirective",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"scheme": authSchemeString(n.Scheme),
				"value":  snapshotNode(n.Value),
			},
		}
	case *ast.HookBlock:
		return nodeSnapshot{
			Type: "HookBlock",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"kind":  hookKindString(n.Kind),
				"stmts": snapshotHookStmts(n.Stmts),
			},
		}
	case *ast.AssertStmt:
		return nodeSnapshot{
			Type: "AssertStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"expr": snapshotNode(n.Expr),
			},
		}
	case *ast.AssignStmt:
		return nodeSnapshot{
			Type: "AssignStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"target": snapshotLValue(n.Target),
				"value":  snapshotNode(n.Value),
			},
		}
	case *ast.ExprStmt:
		return nodeSnapshot{
			Type: "ExprStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"expr": snapshotNode(n.Expr),
			},
		}
	case *ast.PrintStmt:
		return nodeSnapshot{
			Type: "PrintStmt",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"kind": printKindString(n.Kind),
				"args": snapshotExprList(n.Args),
			},
		}
	case *ast.IdentExpr:
		return nodeSnapshot{
			Type: "IdentExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"name": n.Name,
			},
		}
	case *ast.StringLit:
		return nodeSnapshot{
			Type: "StringLit",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"raw":   n.Raw,
				"value": n.Value,
			},
		}
	case *ast.NumberLit:
		return nodeSnapshot{
			Type: "NumberLit",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"raw": n.Raw,
			},
		}
	case *ast.DurationLit:
		return nodeSnapshot{
			Type: "DurationLit",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"raw": n.Raw,
			},
		}
	case *ast.BoolLit:
		return nodeSnapshot{
			Type: "BoolLit",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"value": n.Value,
			},
		}
	case *ast.NullLit:
		return nodeSnapshot{
			Type: "NullLit",
			Span: snapshotSpan(n.Span),
		}
	case *ast.DollarExpr:
		return nodeSnapshot{
			Type: "DollarExpr",
			Span: snapshotSpan(n.Span),
		}
	case *ast.HashExpr:
		return nodeSnapshot{
			Type: "HashExpr",
			Span: snapshotSpan(n.Span),
		}
	case *ast.ArrayLit:
		return nodeSnapshot{
			Type: "ArrayLit",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"elements": snapshotExprList(n.Elements),
			},
		}
	case *ast.ObjectLit:
		return nodeSnapshot{
			Type: "ObjectLit",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"pairs": snapshotObjectPairs(n.Pairs),
			},
		}
	case *ast.UnaryExpr:
		return nodeSnapshot{
			Type: "UnaryExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"op":   unaryOpString(n.Op),
				"expr": snapshotNode(n.X),
			},
		}
	case *ast.BinaryExpr:
		return nodeSnapshot{
			Type: "BinaryExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"op":    binaryOpString(n.Op),
				"left":  snapshotNode(n.Left),
				"right": snapshotNode(n.Right),
			},
		}
	case *ast.CallExpr:
		return nodeSnapshot{
			Type: "CallExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"callee": snapshotNode(n.Callee),
				"args":   snapshotExprList(n.Args),
			},
		}
	case *ast.FieldExpr:
		return nodeSnapshot{
			Type: "FieldExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"expr": snapshotNode(n.X),
				"name": n.Name,
			},
		}
	case *ast.IndexExpr:
		return nodeSnapshot{
			Type: "IndexExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"expr":  snapshotNode(n.X),
				"index": snapshotNode(n.Index),
			},
		}
	case *ast.ParenExpr:
		return nodeSnapshot{
			Type: "ParenExpr",
			Span: snapshotSpan(n.Span),
			Fields: map[string]interface{}{
				"expr": snapshotNode(n.X),
			},
		}
	case *ast.BadExpr:
		return nodeSnapshot{
			Type: "BadExpr",
			Span: snapshotSpan(n.Span),
		}
	default:
		return nil
	}
}

func snapshotStmtList(stmts []ast.Stmt) []interface{} {
	out := make([]interface{}, 0, len(stmts))
	for _, stmt := range stmts {
		out = append(out, snapshotNode(stmt))
	}
	return out
}

func snapshotReqLines(lines []ast.ReqLine) []interface{} {
	out := make([]interface{}, 0, len(lines))
	for _, line := range lines {
		out = append(out, snapshotNode(line))
	}
	return out
}

func snapshotHookStmts(stmts []ast.HookStmt) []interface{} {
	out := make([]interface{}, 0, len(stmts))
	for _, stmt := range stmts {
		out = append(out, snapshotNode(stmt))
	}
	return out
}

func snapshotExprList(exprs []ast.Expr) []interface{} {
	out := make([]interface{}, 0, len(exprs))
	for _, expr := range exprs {
		out = append(out, snapshotNode(expr))
	}
	return out
}

func snapshotLetList(stmts []*ast.LetStmt) []interface{} {
	out := make([]interface{}, 0, len(stmts))
	for _, stmt := range stmts {
		out = append(out, snapshotNode(stmt))
	}
	return out
}

func snapshotAssertList(stmts []*ast.AssertStmt) []interface{} {
	out := make([]interface{}, 0, len(stmts))
	for _, stmt := range stmts {
		out = append(out, snapshotNode(stmt))
	}
	return out
}

func snapshotFlowSteps(steps []ast.FlowStep) []interface{} {
	out := make([]interface{}, 0, len(steps))
	for _, step := range steps {
		out = append(out, map[string]interface{}{
			"req_name": step.ReqName,
			"alias":    step.Alias,
			"span":     snapshotSpan(step.Span),
		})
	}
	return out
}

func snapshotKey(key ast.Key) map[string]interface{} {
	return map[string]interface{}{
		"kind": keyKindString(key.Kind),
		"name": key.Name,
		"raw":  key.Raw,
		"span": snapshotSpan(key.Span),
	}
}

func snapshotObjectPairs(pairs []ast.ObjectPair) []interface{} {
	out := make([]interface{}, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, map[string]interface{}{
			"key":   snapshotObjectKey(pair.Key),
			"value": snapshotNode(pair.Value),
			"span":  snapshotSpan(pair.Span),
		})
	}
	return out
}

func snapshotObjectKey(key ast.ObjectKey) map[string]interface{} {
	return map[string]interface{}{
		"kind": objectKeyKindString(key.Kind),
		"name": key.Name,
		"raw":  key.Raw,
		"span": snapshotSpan(key.Span),
	}
}

func snapshotLValue(lv *ast.LValue) map[string]interface{} {
	postfix := make([]interface{}, 0, len(lv.Postfix))
	for _, post := range lv.Postfix {
		entry := map[string]interface{}{
			"kind": lvaluePostfixKindString(post.Kind),
			"span": snapshotSpan(post.Span),
		}
		if post.Kind == ast.LValueField {
			entry["name"] = post.Name
		}
		if post.Kind == ast.LValueIndex {
			entry["index"] = snapshotNode(post.Index)
		}
		postfix = append(postfix, entry)
	}
	return map[string]interface{}{
		"root": map[string]interface{}{
			"kind": lvalueRootKindString(lv.Root.Kind),
			"name": lv.Root.Name,
			"span": snapshotSpan(lv.Root.Span),
		},
		"postfix": postfix,
		"span":    snapshotSpan(lv.Span),
	}
}

func settingKindString(kind ast.SettingKind) string {
	switch kind {
	case ast.SettingBase:
		return "base"
	case ast.SettingTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

func httpMethodString(method ast.HttpMethod) string {
	switch method {
	case ast.MethodGet:
		return "GET"
	case ast.MethodPost:
		return "POST"
	case ast.MethodPut:
		return "PUT"
	case ast.MethodPatch:
		return "PATCH"
	case ast.MethodDelete:
		return "DELETE"
	case ast.MethodHead:
		return "HEAD"
	case ast.MethodOptions:
		return "OPTIONS"
	default:
		return "UNKNOWN"
	}
}

func authSchemeString(scheme ast.AuthScheme) string {
	switch scheme {
	case ast.AuthBearer:
		return "bearer"
	default:
		return "unknown"
	}
}

func hookKindString(kind ast.HookKind) string {
	switch kind {
	case ast.HookPre:
		return "pre"
	case ast.HookPost:
		return "post"
	default:
		return "unknown"
	}
}

func printKindString(kind ast.PrintKind) string {
	switch kind {
	case ast.Print:
		return "print"
	case ast.Println:
		return "println"
	case ast.Printf:
		return "printf"
	default:
		return "unknown"
	}
}
func unaryOpString(op ast.UnaryOp) string {
	switch op {
	case ast.UnaryNot:
		return "not"
	case ast.UnaryPlus:
		return "+"
	case ast.UnaryMinus:
		return "-"
	default:
		return "unknown"
	}
}

func binaryOpString(op ast.BinaryOp) string {
	switch op {
	case ast.BinaryOr:
		return "or"
	case ast.BinaryAnd:
		return "and"
	case ast.BinaryEq:
		return "=="
	case ast.BinaryNe:
		return "!="
	case ast.BinaryLt:
		return "<"
	case ast.BinaryLte:
		return "<="
	case ast.BinaryGt:
		return ">"
	case ast.BinaryGte:
		return ">="
	case ast.BinaryIn:
		return "in"
	case ast.BinaryContains:
		return "contains"
	case ast.BinaryMatch:
		return "~"
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
		return "unknown"
	}
}

func keyKindString(kind ast.KeyKind) string {
	switch kind {
	case ast.KeyIdent:
		return "ident"
	case ast.KeyBare:
		return "bare"
	case ast.KeyString:
		return "string"
	default:
		return "unknown"
	}
}

func lvalueRootKindString(kind ast.LValueRootKind) string {
	switch kind {
	case ast.LValueIdent:
		return "ident"
	case ast.LValueReq:
		return "req"
	case ast.LValueRes:
		return "res"
	case ast.LValueDollar:
		return "dollar"
	default:
		return "unknown"
	}
}

func lvaluePostfixKindString(kind ast.LValuePostfixKind) string {
	switch kind {
	case ast.LValueField:
		return "field"
	case ast.LValueIndex:
		return "index"
	default:
		return "unknown"
	}
}

func objectKeyKindString(kind ast.ObjectKeyKind) string {
	switch kind {
	case ast.ObjectKeyIdent:
		return "ident"
	case ast.ObjectKeyString:
		return "string"
	default:
		return "unknown"
	}
}

func TestParserValidFiles(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "parser")
	paths, err := filepath.Glob(filepath.Join(root, "valid", "*.pt"))
	if err != nil {
		t.Fatalf("glob valid: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no valid parser fixtures found")
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, lexErrs, parseErrs := Parse(path, string(src))
		if len(lexErrs) > 0 {
			t.Fatalf("unexpected lexer errors for %s: %v", path, lexErrs)
		}
		if len(parseErrs) > 0 {
			t.Fatalf("unexpected parser errors for %s: %v", path, parseErrs)
		}
	}
}

func TestParserInvalidFiles(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "parser")
	paths, err := filepath.Glob(filepath.Join(root, "invalid", "*.pt"))
	if err != nil {
		t.Fatalf("glob invalid: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no invalid parser fixtures found")
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, lexErrs, parseErrs := Parse(path, string(src))
		if len(lexErrs) == 0 && len(parseErrs) == 0 {
			t.Fatalf("expected errors for %s", path)
		}
	}
}

func TestParserGolden(t *testing.T) {
	cases := []struct {
		name       string
		inputPath  string
		goldenPath string
	}{
		{
			name:       "minimal-program",
			inputPath:  filepath.Join("..", "..", "testdata", "parser", "valid", "minimal-program.pt"),
			goldenPath: filepath.Join("..", "..", "testdata", "parser", "golden", "minimal-program.ast.json"),
		},
		{
			name:       "flow-with-aliases",
			inputPath:  filepath.Join("..", "..", "testdata", "parser", "valid", "flow-with-aliases.pt"),
			goldenPath: filepath.Join("..", "..", "testdata", "parser", "golden", "flow-with-aliases.ast.json"),
		},
		{
			name:       "hook-print-statements",
			inputPath:  filepath.Join("..", "..", "testdata", "parser", "valid", "hook-print-statements.pt"),
			goldenPath: filepath.Join("..", "..", "testdata", "parser", "golden", "hook-print-statements.ast.json"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.inputPath)
			if err != nil {
				t.Fatalf("read %s: %v", tc.inputPath, err)
			}
			program, lexErrs, parseErrs := Parse(tc.inputPath, string(src))
			if len(lexErrs) > 0 {
				t.Fatalf("unexpected lexer errors: %v", lexErrs)
			}
			if len(parseErrs) > 0 {
				t.Fatalf("unexpected parser errors: %v", parseErrs)
			}

			got := snapshotNode(program)
			data, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal golden: %v", err)
			}
			data = append(data, '\n')
			if *updateGolden {
				if err := os.WriteFile(tc.goldenPath, data, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(tc.goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(data, want) {
				t.Fatalf("AST mismatch for %s (re-run with -update to refresh)", tc.name)
			}
		})
	}
}
