package parser

import (
	"strconv"

	"github.com/mehditeymorian/pipetest/internal/ast"
	"github.com/mehditeymorian/pipetest/internal/lexer"
)

// Parser converts lexer tokens into AST nodes.
type Parser struct {
	lx   *lexer.Lexer
	cur  lexer.Token
	peek lexer.Token
	errs []ParseError
}

// Parse parses the provided source into an AST.
func Parse(path, src string) (*ast.Program, []lexer.LexError, []ParseError) {
	lx := lexer.NewLexer(path, src)
	p := NewParser(lx)
	program := p.ParseProgram()
	return program, lx.Errors(), p.Errors()
}

// NewParser creates a parser for the lexer stream.
func NewParser(lx *lexer.Lexer) *Parser {
	p := &Parser{lx: lx}
	p.cur = lx.Next()
	p.peek = lx.Next()
	return p
}

// Errors returns parser errors.
func (p *Parser) Errors() []ParseError {
	return p.errs
}

func (p *Parser) advance() {
	p.cur = p.peek
	p.peek = p.lx.Next()
}

func (p *Parser) match(kind lexer.Kind) bool {
	if p.cur.Kind == kind {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(kind lexer.Kind, msg, hint string) lexer.Token {
	if p.cur.Kind != kind {
		p.addError(ErrExpectedToken, msg, hint, p.cur.Span)
		tok := p.cur
		p.advance()
		return lexer.Token{Kind: lexer.ILLEGAL, Lit: tok.Lit, Span: tok.Span}
	}
	tok := p.cur
	p.advance()
	return tok
}

func (p *Parser) addError(code, msg, hint string, span lexer.Span) {
	p.errs = append(p.errs, ParseError{
		Code:    code,
		Message: msg,
		Hint:    hint,
		File:    p.lx.Path(),
		Span:    toASTSpan(span),
	})
}

// ParseProgram parses the entire token stream.
func (p *Parser) ParseProgram() *ast.Program {
	var stmts []ast.Stmt
	var span ast.Span
	for p.cur.Kind != lexer.EOF {
		if p.match(lexer.NL) {
			continue
		}
		stmt := p.parseTopStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
			span = mergeProgramSpan(span, stmt)
			continue
		}
		p.syncTop()
	}
	return &ast.Program{Stmts: stmts, Span: span}
}

func mergeProgramSpan(span ast.Span, stmt ast.Stmt) ast.Span {
	stmtSpan := stmtSpan(stmt)
	if isZeroSpan(span) {
		return stmtSpan
	}
	return joinSpan(span, stmtSpan)
}

func stmtSpan(stmt ast.Stmt) ast.Span {
	switch s := stmt.(type) {
	case *ast.SettingStmt:
		return s.Span
	case *ast.ImportStmt:
		return s.Span
	case *ast.LetStmt:
		return s.Span
	case *ast.ReqDecl:
		return s.Span
	case *ast.FlowDecl:
		return s.Span
	default:
		return ast.Span{}
	}
}

func isZeroSpan(span ast.Span) bool {
	return span.Start == (ast.Position{}) && span.End == (ast.Position{})
}

func (p *Parser) parseTopStmt() ast.Stmt {
	switch p.cur.Kind {
	case lexer.KW_BASE, lexer.KW_TIMEOUT:
		stmt := p.parseSetting()
		p.expect(lexer.NL, "expected newline after setting", "add a newline after the setting")
		return stmt
	case lexer.KW_IMPORT:
		stmt := p.parseImport()
		p.expect(lexer.NL, "expected newline after import", "add a newline after the import")
		return stmt
	case lexer.KW_LET:
		stmt := p.parseLet()
		p.expect(lexer.NL, "expected newline after let", "add a newline after the let")
		return stmt
	case lexer.KW_REQ:
		return p.parseReqDecl()
	case lexer.KW_FLOW:
		return p.parseFlowDecl()
	default:
		p.addError(ErrUnexpectedToken, "unexpected token at top level", "start with a declaration", p.cur.Span)
		return nil
	}
}

func (p *Parser) parseSetting() *ast.SettingStmt {
	startTok := p.cur
	if p.match(lexer.KW_BASE) {
		valTok := p.expect(lexer.STRING, "expected string literal after base", "provide a base URL string")
		lit := p.stringLit(valTok)
		return &ast.SettingStmt{
			Kind:  ast.SettingBase,
			Value: lit,
			Span:  joinSpan(toASTSpan(startTok.Span), lit.Span),
		}
	}

	p.expect(lexer.KW_TIMEOUT, "expected timeout", "use timeout <duration>")
	valTok := p.expect(lexer.DURATION, "expected duration literal after timeout", "provide a duration like 5s")
	lit := &ast.DurationLit{Raw: valTok.Lit, Span: toASTSpan(valTok.Span)}
	return &ast.SettingStmt{
		Kind:  ast.SettingTimeout,
		Value: lit,
		Span:  joinSpan(toASTSpan(startTok.Span), lit.Span),
	}
}

func (p *Parser) parseImport() *ast.ImportStmt {
	startTok := p.expect(lexer.KW_IMPORT, "expected import", "use import \"file.pt\"")
	valTok := p.expect(lexer.STRING, "expected string literal after import", "provide a path string")
	lit := p.stringLit(valTok)
	return &ast.ImportStmt{Path: lit, Span: joinSpan(toASTSpan(startTok.Span), lit.Span)}
}

func (p *Parser) parseLet() *ast.LetStmt {
	startTok := p.expect(lexer.KW_LET, "expected let", "use let name = expr")
	nameTok := p.expect(lexer.IDENT, "expected identifier after let", "provide a variable name")
	p.expect(lexer.ASSIGN, "expected '=' in let statement", "assign a value to the variable")
	val := p.parseExpr(precLowest)
	valSpan := exprSpan(val)
	return &ast.LetStmt{
		Name:  nameTok.Lit,
		Value: val,
		Span:  joinSpan(toASTSpan(startTok.Span), valSpan),
	}
}

func (p *Parser) parseReqDecl() *ast.ReqDecl {
	startTok := p.expect(lexer.KW_REQ, "expected req", "use req <name>:")
	nameTok := p.expect(lexer.IDENT, "expected request name", "provide a request name")
	var parent *string
	if p.match(lexer.LPAREN) {
		parTok := p.expect(lexer.IDENT, "expected parent request name", "provide a parent request name")
		val := parTok.Lit
		parent = &val
		p.expect(lexer.RPAREN, "expected ')' after parent name", "close the parent list")
	}
	p.expect(lexer.COLON, "expected ':' after req header", "add ':' to start the request block")
	p.expect(lexer.NL, "expected newline after req header", "add a newline after the header")
	p.expect(lexer.INDENT, "expected indented req block", "indent request lines")

	var lines []ast.ReqLine
	for p.cur.Kind != lexer.DEDENT && p.cur.Kind != lexer.EOF {
		if p.match(lexer.NL) {
			continue
		}
		switch p.cur.Kind {
		case lexer.KW_GET, lexer.KW_POST_M, lexer.KW_PUT, lexer.KW_PATCH, lexer.KW_DELETE, lexer.KW_HEAD, lexer.KW_OPTIONS:
			line := p.parseHttpLine()
			lines = append(lines, line)
			p.expect(lexer.NL, "expected newline after http line", "add a newline after the HTTP line")
		case lexer.KW_JSON, lexer.KW_HEADER, lexer.KW_QUERY, lexer.KW_AUTH:
			line := p.parseDirective()
			lines = append(lines, line)
			p.expect(lexer.NL, "expected newline after directive", "add a newline after the directive")
		case lexer.KW_PRE, lexer.KW_POST:
			line := p.parseHookBlock()
			lines = append(lines, line)
		case lexer.QUESTION:
			line := p.parseAssertLine()
			lines = append(lines, line)
			p.expect(lexer.NL, "expected newline after assertion", "add a newline after the assertion")
		case lexer.KW_LET:
			line := p.parseLet()
			lines = append(lines, line)
			p.expect(lexer.NL, "expected newline after let", "add a newline after the let")
		default:
			p.addError(ErrInvalidLine, "invalid request line", "use an http line, directive, hook, assertion, or let", p.cur.Span)
			p.syncLine()
		}
	}
	endTok := p.expect(lexer.DEDENT, "expected end of req block", "dedent to close the req block")
	return &ast.ReqDecl{
		Name:   nameTok.Lit,
		Parent: parent,
		Lines:  lines,
		Span:   joinSpan(toASTSpan(startTok.Span), toASTSpan(endTok.Span)),
	}
}

func (p *Parser) parseHttpLine() *ast.HttpLine {
	startTok := p.cur
	method := ast.MethodGet
	switch p.cur.Kind {
	case lexer.KW_GET:
		method = ast.MethodGet
	case lexer.KW_POST_M:
		method = ast.MethodPost
	case lexer.KW_PUT:
		method = ast.MethodPut
	case lexer.KW_PATCH:
		method = ast.MethodPatch
	case lexer.KW_DELETE:
		method = ast.MethodDelete
	case lexer.KW_HEAD:
		method = ast.MethodHead
	case lexer.KW_OPTIONS:
		method = ast.MethodOptions
	default:
		p.addError(ErrExpectedToken, "expected HTTP method", "start with GET/POST/etc", p.cur.Span)
	}
	p.advance()
	pathTok := p.expect(lexer.PATH, "expected path or URL after method", "provide a path like /orders")
	return &ast.HttpLine{
		Method: method,
		Path:   pathTok.Lit,
		Span:   joinSpan(toASTSpan(startTok.Span), toASTSpan(pathTok.Span)),
	}
}

func (p *Parser) parseDirective() ast.ReqLine {
	switch p.cur.Kind {
	case lexer.KW_JSON:
		startTok := p.expect(lexer.KW_JSON, "expected json", "use json { ... }")
		obj := p.parseObjectLit()
		return &ast.JsonDirective{Value: obj, Span: joinSpan(toASTSpan(startTok.Span), obj.Span)}
	case lexer.KW_HEADER:
		startTok := p.expect(lexer.KW_HEADER, "expected header", "use header Key = expr")
		key := p.parseKey()
		p.expect(lexer.ASSIGN, "expected '=' after header key", "assign a header value")
		val := p.parseExpr(precLowest)
		return &ast.HeaderDirective{Key: key, Value: val, Span: joinSpan(toASTSpan(startTok.Span), exprSpan(val))}
	case lexer.KW_QUERY:
		startTok := p.expect(lexer.KW_QUERY, "expected query", "use query Key = expr")
		key := p.parseKey()
		p.expect(lexer.ASSIGN, "expected '=' after query key", "assign a query value")
		val := p.parseExpr(precLowest)
		return &ast.QueryDirective{Key: key, Value: val, Span: joinSpan(toASTSpan(startTok.Span), exprSpan(val))}
	case lexer.KW_AUTH:
		startTok := p.expect(lexer.KW_AUTH, "expected auth", "use auth bearer expr")
		p.expect(lexer.KW_BEARER, "expected bearer auth", "use bearer auth")
		val := p.parseExpr(precLowest)
		return &ast.AuthDirective{Scheme: ast.AuthBearer, Value: val, Span: joinSpan(toASTSpan(startTok.Span), exprSpan(val))}
	default:
		p.addError(ErrInvalidLine, "invalid directive", "use json/header/query/auth", p.cur.Span)
		return &ast.JsonDirective{Span: toASTSpan(p.cur.Span)}
	}
}

func (p *Parser) parseHookBlock() *ast.HookBlock {
	startTok := p.cur
	kind := ast.HookPre
	if p.match(lexer.KW_PRE) {
		kind = ast.HookPre
	} else if p.match(lexer.KW_POST) {
		kind = ast.HookPost
	} else {
		p.addError(ErrExpectedToken, "expected pre/post hook", "use pre hook or post hook", p.cur.Span)
	}
	p.expect(lexer.KW_HOOK, "expected 'hook'", "use pre hook { ... }")
	p.expect(lexer.LBRACE, "expected '{' to start hook", "open the hook block")

	var stmts []ast.HookStmt
	for p.cur.Kind != lexer.RBRACE && p.cur.Kind != lexer.EOF {
		if p.match(lexer.NL) || p.match(lexer.SEMICOLON) {
			continue
		}
		stmt := p.parseHookStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		if p.cur.Kind == lexer.NL || p.cur.Kind == lexer.SEMICOLON {
			p.advance()
		}
	}
	endTok := p.expect(lexer.RBRACE, "expected '}' to end hook", "close the hook block")
	if p.cur.Kind == lexer.NL {
		p.advance()
	}
	return &ast.HookBlock{Kind: kind, Stmts: stmts, Span: joinSpan(toASTSpan(startTok.Span), toASTSpan(endTok.Span))}
}

func (p *Parser) parseHookStmt() ast.HookStmt {
	if p.cur.Kind == lexer.KW_LET {
		return p.parseLet()
	}
	left := p.parseExpr(precLowest)
	if p.cur.Kind == lexer.ASSIGN {
		p.advance()
		val := p.parseExpr(precLowest)
		lv, ok := p.exprToLValue(left)
		if !ok {
			p.addError(ErrInvalidLine, "invalid assignment target", "assign to req, res, $ or an identifier path", toLexSpan(exprSpan(left)))
			return &ast.ExprStmt{Expr: val, Span: exprSpan(val)}
		}
		return &ast.AssignStmt{Target: lv, Value: val, Span: joinSpan(lv.Span, exprSpan(val))}
	}
	return &ast.ExprStmt{Expr: left, Span: exprSpan(left)}
}

func (p *Parser) parseAssertLine() *ast.AssertStmt {
	startTok := p.expect(lexer.QUESTION, "expected '?'", "start assertion with '?'")
	val := p.parseExpr(precLowest)
	return &ast.AssertStmt{Expr: val, Span: joinSpan(toASTSpan(startTok.Span), exprSpan(val))}
}

func (p *Parser) parseFlowDecl() *ast.FlowDecl {
	startTok := p.expect(lexer.KW_FLOW, "expected flow", "use flow \"name\":")
	nameTok := p.expect(lexer.STRING, "expected flow name string", "provide a flow name")
	name := p.stringLit(nameTok)
	p.expect(lexer.COLON, "expected ':' after flow name", "add ':' to start the flow block")
	p.expect(lexer.NL, "expected newline after flow header", "add a newline after the header")
	p.expect(lexer.INDENT, "expected indented flow block", "indent flow lines")

	var prelude []*ast.LetStmt
	for p.cur.Kind == lexer.KW_LET || p.cur.Kind == lexer.NL {
		if p.match(lexer.NL) {
			continue
		}
		ls := p.parseLet()
		prelude = append(prelude, ls)
		p.expect(lexer.NL, "expected newline after let", "add a newline after the let")
	}

	var chain []ast.FlowStep
	if p.cur.Kind == lexer.IDENT {
		chain = p.parseFlowChainLine()
		p.expect(lexer.NL, "expected newline after chain line", "add a newline after the chain line")
	} else {
		p.addError(ErrInvalidFlow, "flow missing chain line", "add a chain line with '->'", p.cur.Span)
		if p.cur.Kind != lexer.DEDENT && p.cur.Kind != lexer.EOF {
			p.syncLine()
		}
	}

	var asserts []*ast.AssertStmt
	for p.cur.Kind != lexer.DEDENT && p.cur.Kind != lexer.EOF {
		if p.match(lexer.NL) {
			continue
		}
		if p.cur.Kind != lexer.QUESTION {
			p.addError(ErrInvalidFlow, "only assertions allowed after flow chain", "move non-assert lines before the chain", p.cur.Span)
			p.syncLine()
			continue
		}
		as := p.parseAssertLine()
		asserts = append(asserts, as)
		p.expect(lexer.NL, "expected newline after assertion", "add a newline after the assertion")
	}
	endTok := p.expect(lexer.DEDENT, "expected end of flow block", "dedent to close the flow block")

	return &ast.FlowDecl{
		Name:    name,
		Prelude: prelude,
		Chain:   chain,
		Asserts: asserts,
		Span:    joinSpan(toASTSpan(startTok.Span), toASTSpan(endTok.Span)),
	}
}

func (p *Parser) parseFlowChainLine() []ast.FlowStep {
	steps := []ast.FlowStep{p.parseFlowStepRef()}
	arrowCount := 0
	for p.cur.Kind == lexer.ARROW {
		arrowCount++
		p.advance()
		steps = append(steps, p.parseFlowStepRef())
	}
	if arrowCount == 0 {
		p.addError(ErrInvalidFlow, "flow chain must use '->' format", "add '->' between steps", p.cur.Span)
	}
	return steps
}

func (p *Parser) parseFlowStepRef() ast.FlowStep {
	nameTok := p.expect(lexer.IDENT, "expected request name in flow", "provide a request name")
	span := toASTSpan(nameTok.Span)
	var alias *string
	if p.match(lexer.COLON) {
		aliasTok := p.expect(lexer.IDENT, "expected alias after ':'", "provide an alias name")
		val := aliasTok.Lit
		alias = &val
		span = joinSpan(span, toASTSpan(aliasTok.Span))
	}
	return ast.FlowStep{ReqName: nameTok.Lit, Alias: alias, Span: span}
}

func (p *Parser) parseKey() ast.Key {
	switch p.cur.Kind {
	case lexer.IDENT:
		tok := p.cur
		p.advance()
		return ast.Key{Kind: ast.KeyIdent, Name: tok.Lit, Span: toASTSpan(tok.Span)}
	case lexer.BARE_KEY:
		tok := p.cur
		p.advance()
		return ast.Key{Kind: ast.KeyBare, Name: tok.Lit, Span: toASTSpan(tok.Span)}
	case lexer.STRING:
		tok := p.cur
		p.advance()
		lit := p.stringLit(tok)
		return ast.Key{Kind: ast.KeyString, Name: lit.Value, Raw: lit.Raw, Span: lit.Span}
	default:
		p.addError(ErrExpectedToken, "expected key", "use an identifier or string", p.cur.Span)
		tok := p.cur
		p.advance()
		return ast.Key{Kind: ast.KeyIdent, Span: toASTSpan(tok.Span)}
	}
}

func (p *Parser) expectFieldName() lexer.Token {
	switch p.cur.Kind {
	case lexer.IDENT, lexer.KW_REQ, lexer.KW_HEADER, lexer.KW_QUERY:
		tok := p.cur
		p.advance()
		return tok
	default:
		p.addError(ErrExpectedToken, "expected field name", "provide a field name", p.cur.Span)
		tok := p.cur
		p.advance()
		return lexer.Token{Kind: lexer.ILLEGAL, Lit: tok.Lit, Span: tok.Span}
	}
}

func (p *Parser) parseObjectKey() (ast.ObjectKey, bool) {
	switch p.cur.Kind {
	case lexer.IDENT:
		tok := p.cur
		p.advance()
		return ast.ObjectKey{Kind: ast.ObjectKeyIdent, Name: tok.Lit, Span: toASTSpan(tok.Span)}, true
	case lexer.STRING:
		tok := p.cur
		p.advance()
		lit := p.stringLit(tok)
		return ast.ObjectKey{Kind: ast.ObjectKeyString, Name: lit.Value, Raw: lit.Raw, Span: lit.Span}, true
	default:
		p.addError(ErrExpectedToken, "expected object key", "use an identifier or string literal", p.cur.Span)
		tok := p.cur
		p.advance()
		return ast.ObjectKey{Span: toASTSpan(tok.Span)}, false
	}
}

func (p *Parser) parseExpr(min prec) ast.Expr {
	left := p.parsePrefix()

	left = p.parsePostfix(left)

	for {
		prec, ok := infixPrec(p.cur.Kind)
		if !ok || prec < min {
			break
		}
		opTok := p.cur
		p.advance()
		right := p.parseExpr(prec + 1)
		left = &ast.BinaryExpr{
			Op:    toBinaryOp(opTok.Kind),
			Left:  left,
			Right: right,
			Span:  joinSpan(exprSpan(left), exprSpan(right)),
		}
		left = p.parsePostfix(left)
	}
	return left
}

func (p *Parser) parsePostfix(left ast.Expr) ast.Expr {
	for {
		switch p.cur.Kind {
		case lexer.LPAREN:
			left = p.parseCall(left)
		case lexer.DOT:
			left = p.parseField(left)
		case lexer.LBRACK:
			left = p.parseIndex(left)
		default:
			return left
		}
	}
}

func (p *Parser) parsePrefix() ast.Expr {
	switch p.cur.Kind {
	case lexer.OP_NOT, lexer.OP_PLUS, lexer.OP_MINUS:
		opTok := p.cur
		p.advance()
		val := p.parseExpr(precPrefix)
		return &ast.UnaryExpr{
			Op:   toUnaryOp(opTok.Kind),
			X:    val,
			Span: joinSpan(toASTSpan(opTok.Span), exprSpan(val)),
		}
	default:
		return p.parsePrimary()
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	switch p.cur.Kind {
	case lexer.IDENT:
		tok := p.cur
		p.advance()
		return &ast.IdentExpr{Name: tok.Lit, Span: toASTSpan(tok.Span)}
	case lexer.KW_REQ:
		tok := p.cur
		p.advance()
		return &ast.IdentExpr{Name: tok.Lit, Span: toASTSpan(tok.Span)}
	case lexer.KW_HEADER, lexer.KW_QUERY:
		tok := p.cur
		p.advance()
		return &ast.IdentExpr{Name: tok.Lit, Span: toASTSpan(tok.Span)}
	case lexer.STRING:
		tok := p.cur
		p.advance()
		return p.stringLit(tok)
	case lexer.NUMBER:
		tok := p.cur
		p.advance()
		return &ast.NumberLit{Raw: tok.Lit, Span: toASTSpan(tok.Span)}
	case lexer.KW_TRUE:
		tok := p.cur
		p.advance()
		return &ast.BoolLit{Value: true, Span: toASTSpan(tok.Span)}
	case lexer.KW_FALSE:
		tok := p.cur
		p.advance()
		return &ast.BoolLit{Value: false, Span: toASTSpan(tok.Span)}
	case lexer.KW_NULL:
		tok := p.cur
		p.advance()
		return &ast.NullLit{Span: toASTSpan(tok.Span)}
	case lexer.DOLLAR:
		tok := p.cur
		p.advance()
		return &ast.DollarExpr{Span: toASTSpan(tok.Span)}
	case lexer.LPAREN:
		startTok := p.cur
		p.advance()
		expr := p.parseExpr(precLowest)
		endTok := p.expect(lexer.RPAREN, "expected ')'", "close the expression")
		return &ast.ParenExpr{X: expr, Span: joinSpan(toASTSpan(startTok.Span), toASTSpan(endTok.Span))}
	case lexer.LBRACK:
		return p.parseArrayLit()
	case lexer.LBRACE:
		return p.parseObjectLit()
	default:
		p.addError(ErrInvalidExpr, "unexpected token in expression", "provide a valid expression", p.cur.Span)
		tok := p.cur
		p.advance()
		return &ast.BadExpr{Span: toASTSpan(tok.Span)}
	}
}

func (p *Parser) exprToLValue(expr ast.Expr) (*ast.LValue, bool) {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		root := ast.LValueRoot{Kind: ast.LValueIdent, Name: e.Name, Span: e.Span}
		if e.Name == "req" {
			root.Kind = ast.LValueReq
		}
		if e.Name == "res" {
			root.Kind = ast.LValueRes
		}
		return &ast.LValue{Root: root, Span: e.Span}, true
	case *ast.DollarExpr:
		root := ast.LValueRoot{Kind: ast.LValueDollar, Name: "$", Span: e.Span}
		return &ast.LValue{Root: root, Span: e.Span}, true
	case *ast.FieldExpr:
		base, ok := p.exprToLValue(e.X)
		if !ok {
			return nil, false
		}
		post := ast.LValuePostfix{
			Kind: ast.LValueField,
			Name: e.Name,
			Span: ast.Span{Start: base.Span.End, End: e.Span.End},
		}
		base.Postfix = append(base.Postfix, post)
		base.Span = joinSpan(base.Span, post.Span)
		return base, true
	case *ast.IndexExpr:
		base, ok := p.exprToLValue(e.X)
		if !ok {
			return nil, false
		}
		post := ast.LValuePostfix{
			Kind:  ast.LValueIndex,
			Index: e.Index,
			Span:  ast.Span{Start: base.Span.End, End: e.Span.End},
		}
		base.Postfix = append(base.Postfix, post)
		base.Span = joinSpan(base.Span, post.Span)
		return base, true
	default:
		return nil, false
	}
}

func (p *Parser) parseCall(callee ast.Expr) ast.Expr {
	start := exprSpan(callee)
	p.expect(lexer.LPAREN, "expected '('", "start argument list")
	var args []ast.Expr
	if p.cur.Kind != lexer.RPAREN {
		for {
			args = append(args, p.parseExpr(precLowest))
			if p.match(lexer.COMMA) {
				if p.cur.Kind == lexer.RPAREN {
					break
				}
				continue
			}
			break
		}
	}
	endTok := p.expect(lexer.RPAREN, "expected ')' after call", "close the call")
	return &ast.CallExpr{Callee: callee, Args: args, Span: joinSpan(start, toASTSpan(endTok.Span))}
}

func (p *Parser) parseField(left ast.Expr) ast.Expr {
	p.expect(lexer.DOT, "expected '.'", "use .field to access a field")
	nameTok := p.expectFieldName()
	return &ast.FieldExpr{X: left, Name: nameTok.Lit, Span: joinSpan(exprSpan(left), toASTSpan(nameTok.Span))}
}

func (p *Parser) parseIndex(left ast.Expr) ast.Expr {
	start := exprSpan(left)
	p.expect(lexer.LBRACK, "expected '['", "start index expression")
	idx := p.parseExpr(precLowest)
	endTok := p.expect(lexer.RBRACK, "expected ']'", "close index expression")
	return &ast.IndexExpr{X: left, Index: idx, Span: joinSpan(start, toASTSpan(endTok.Span))}
}

func (p *Parser) parseArrayLit() *ast.ArrayLit {
	startTok := p.expect(lexer.LBRACK, "expected '['", "start array literal")
	var elems []ast.Expr
	if p.cur.Kind != lexer.RBRACK {
		for {
			elems = append(elems, p.parseExpr(precLowest))
			if p.match(lexer.COMMA) {
				if p.cur.Kind == lexer.RBRACK {
					break
				}
				continue
			}
			break
		}
	}
	endTok := p.expect(lexer.RBRACK, "expected ']'", "close array literal")
	return &ast.ArrayLit{Elements: elems, Span: joinSpan(toASTSpan(startTok.Span), toASTSpan(endTok.Span))}
}

func (p *Parser) parseObjectLit() *ast.ObjectLit {
	startTok := p.expect(lexer.LBRACE, "expected '{'", "start object literal")
	var pairs []ast.ObjectPair
	if p.cur.Kind != lexer.RBRACE {
		for {
			key, ok := p.parseObjectKey()
			p.expect(lexer.COLON, "expected ':' after object key", "separate key and value with ':'")
			val := p.parseExpr(precLowest)
			if ok {
				pairs = append(pairs, ast.ObjectPair{Key: key, Value: val, Span: joinSpan(key.Span, exprSpan(val))})
			}
			if p.match(lexer.COMMA) {
				if p.cur.Kind == lexer.RBRACE {
					break
				}
				continue
			}
			break
		}
	}
	endTok := p.expect(lexer.RBRACE, "expected '}'", "close object literal")
	return &ast.ObjectLit{Pairs: pairs, Span: joinSpan(toASTSpan(startTok.Span), toASTSpan(endTok.Span))}
}

func (p *Parser) stringLit(tok lexer.Token) *ast.StringLit {
	val, err := strconv.Unquote(tok.Lit)
	if err != nil {
		val = tok.Lit
	}
	return &ast.StringLit{Raw: tok.Lit, Value: val, Span: toASTSpan(tok.Span)}
}

func (p *Parser) syncLine() {
	for p.cur.Kind != lexer.NL && p.cur.Kind != lexer.DEDENT && p.cur.Kind != lexer.EOF {
		p.advance()
	}
	if p.cur.Kind == lexer.NL {
		p.advance()
	}
}

func (p *Parser) syncTop() {
	for p.cur.Kind != lexer.NL && p.cur.Kind != lexer.EOF {
		p.advance()
	}
	if p.cur.Kind == lexer.NL {
		p.advance()
	}
}

// precedence handling

type prec int

const (
	precLowest prec = iota
	precOr
	precAnd
	precCompare
	precAdd
	precMul
	precPrefix
)

func infixPrec(kind lexer.Kind) (prec, bool) {
	switch kind {
	case lexer.OP_OR:
		return precOr, true
	case lexer.OP_AND:
		return precAnd, true
	case lexer.OP_EQ, lexer.OP_NE, lexer.OP_LT, lexer.OP_LTE, lexer.OP_GT, lexer.OP_GTE, lexer.OP_IN, lexer.OP_CONTAINS, lexer.OP_TILDE:
		return precCompare, true
	case lexer.OP_PLUS, lexer.OP_MINUS:
		return precAdd, true
	case lexer.OP_MUL, lexer.OP_DIV, lexer.OP_MOD:
		return precMul, true
	default:
		return 0, false
	}
}

func toUnaryOp(kind lexer.Kind) ast.UnaryOp {
	switch kind {
	case lexer.OP_NOT:
		return ast.UnaryNot
	case lexer.OP_MINUS:
		return ast.UnaryMinus
	case lexer.OP_PLUS:
		return ast.UnaryPlus
	default:
		return ast.UnaryPlus
	}
}

func toBinaryOp(kind lexer.Kind) ast.BinaryOp {
	switch kind {
	case lexer.OP_OR:
		return ast.BinaryOr
	case lexer.OP_AND:
		return ast.BinaryAnd
	case lexer.OP_EQ:
		return ast.BinaryEq
	case lexer.OP_NE:
		return ast.BinaryNe
	case lexer.OP_LT:
		return ast.BinaryLt
	case lexer.OP_LTE:
		return ast.BinaryLte
	case lexer.OP_GT:
		return ast.BinaryGt
	case lexer.OP_GTE:
		return ast.BinaryGte
	case lexer.OP_IN:
		return ast.BinaryIn
	case lexer.OP_CONTAINS:
		return ast.BinaryContains
	case lexer.OP_TILDE:
		return ast.BinaryMatch
	case lexer.OP_PLUS:
		return ast.BinaryAdd
	case lexer.OP_MINUS:
		return ast.BinarySub
	case lexer.OP_MUL:
		return ast.BinaryMul
	case lexer.OP_DIV:
		return ast.BinaryDiv
	case lexer.OP_MOD:
		return ast.BinaryMod
	default:
		return ast.BinaryAdd
	}
}

func exprSpan(expr ast.Expr) ast.Span {
	switch e := expr.(type) {
	case *ast.IdentExpr:
		return e.Span
	case *ast.StringLit:
		return e.Span
	case *ast.NumberLit:
		return e.Span
	case *ast.DurationLit:
		return e.Span
	case *ast.BoolLit:
		return e.Span
	case *ast.NullLit:
		return e.Span
	case *ast.DollarExpr:
		return e.Span
	case *ast.ArrayLit:
		return e.Span
	case *ast.ObjectLit:
		return e.Span
	case *ast.UnaryExpr:
		return e.Span
	case *ast.BinaryExpr:
		return e.Span
	case *ast.CallExpr:
		return e.Span
	case *ast.FieldExpr:
		return e.Span
	case *ast.IndexExpr:
		return e.Span
	case *ast.ParenExpr:
		return e.Span
	case *ast.BadExpr:
		return e.Span
	default:
		return ast.Span{}
	}
}

func toASTSpan(span lexer.Span) ast.Span {
	return ast.Span{
		Start: ast.Position{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column},
		End:   ast.Position{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column},
	}
}

func toLexSpan(span ast.Span) lexer.Span {
	return lexer.Span{
		Start: lexer.Position{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column},
		End:   lexer.Position{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column},
	}
}

func joinSpan(a, b ast.Span) ast.Span {
	if isZeroSpan(a) {
		return b
	}
	if isZeroSpan(b) {
		return a
	}
	return ast.Span{Start: a.Start, End: b.End}
}
