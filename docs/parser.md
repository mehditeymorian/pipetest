A **hand-written recursive descent** parser gives you clean control over indentation blocks + “flow shape” rules, and a **Pratt parser** keeps expressions flexible without turning your statement grammar into a mess.

Below is a concrete blueprint you can implement in Go.

---

## 1) High-level architecture

### Pipeline

1. **Lexer (with pre-pass behaviors baked in)**
   Produces tokens including: `NL`, `INDENT`, `DEDENT`, `{`, `}`, `->`, `?`, keywords, identifiers, strings, numbers, `PATH`, etc.
2. **Parser**

   * Recursive descent for `Program`, `req`, `flow`, directives, hooks…
   * Pratt parser for `Expr` used by `let`, `?`, directives (`json {}`, `header X = expr`, etc.)
3. **Semantic checks (post-parse)**
   Validate imports/flows, request inheritance, flow chain constraints, alias uniqueness, undefined vars, path param availability, etc.

---

## 2) Lexer pre-pass behaviors (how to do it in one lexer)

You don’t need a separate “line pre-pass” file—just implement these behaviors inside the lexer:

### State you’ll track

* `atLineStart bool`
* `indentStack []int` (start with `[0]`)
* `expectIndent bool` (set after a `:` line ends)
* `hookDepth int` (brace depth for hook bodies)
* `exprDepth` counters (optional but recommended if you allow multiline JSON objects/arrays):

  * `parenDepth`, `bracketDepth`, `braceExprDepth`
* string mode: `inString`, `inRawString`

### Rules (core)

* **Comments**: `#` starts comment only when not inside a string; ignore until newline.
* **Indent/Dedent**:

  * Only when `hookDepth==0`.
  * If `expectIndent==true`, the next non-empty line’s leading spaces decides whether to emit `INDENT` (push) or error (same indent is error if you want strict blocks).
  * On normal lines, if leading spaces < stack top, emit `DEDENT` until it matches; if no match → error.
* **Hook brace classification**:

  * When lexer sees tokens `pre` `hook` `{` OR `post` `hook` `{`, increment `hookDepth`.
  * While `hookDepth>0`, **do not emit INDENT/DEDENT** (braces control scope).
  * `}` decrements `hookDepth` when in hook mode; otherwise it’s a normal token (used in object literals).
* **`PATH` token**:

  * If the next non-space starts with `/` or `http://` or `https://`, lex a single `PATH` token consuming until whitespace or `#`.
  * This preserves `:group_id` inside paths without producing a `:` token.

---

## 3) Token set (minimal but complete)

```go
type Kind int

const (
	// special
	EOF Kind = iota
	ILLEGAL

	// layout
	NL
	INDENT
	DEDENT

	// literals
	IDENT
	STRING
	NUMBER
	DURATION
	PATH

	// keywords
	KW_REQ
	KW_FLOW
	KW_IMPORT
	KW_LET
	KW_BASE
	KW_TIMEOUT
	KW_JSON
	KW_HEADER
	KW_QUERY
	KW_AUTH
	KW_BEARER
	KW_PRE
	KW_POST
	KW_HOOK

	// http methods
	KW_GET
	KW_POST_M
	KW_PUT
	KW_PATCH
	KW_DELETE
	KW_HEAD
	KW_OPTIONS

	// operators / punct
	ARROW      // ->
	QUESTION   // ?
	COLON      // :
	COMMA      // ,
	DOT        // .
	ASSIGN     // =
	LPAREN     // (
	RPAREN     // )
	LBRACE     // {
	RBRACE     // }
	LBRACK     // [
	RBRACK     // ]

	// expr operators
	OP_OR
	OP_AND
	OP_NOT
	OP_EQ     // ==
	OP_NE     // !=
	OP_LT     // <
	OP_LTE    // <=
	OP_GT     // >
	OP_GTE    // >=
	OP_IN     // in
	OP_CONTAINS // contains
	OP_TILDE  // ~
	OP_PLUS   // +
	OP_MINUS  // -
	OP_MUL    // *
	OP_DIV    // /
	OP_MOD    // %
)

type Token struct {
	Kind Kind
	Lit  string
	Line int
	Col  int
}
```

---

## 4) Parser shape (recursive descent)

### Parser skeleton

```go
type Parser struct {
	lx    *Lexer
	cur   Token
	peek  Token
	errs  []error
}

func NewParser(lx *Lexer) *Parser {
	p := &Parser{lx: lx}
	p.cur = lx.Next()
	p.peek = lx.Next()
	return p
}

func (p *Parser) advance() {
	p.cur = p.peek
	p.peek = p.lx.Next()
}

func (p *Parser) match(k Kind) bool {
	if p.cur.Kind == k {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(k Kind, msg string) Token {
	if p.cur.Kind != k {
		p.errs = append(p.errs, fmt.Errorf("%s at %d:%d, got %v (%q)", msg, p.cur.Line, p.cur.Col, p.cur.Kind, p.cur.Lit))
		return Token{Kind: ILLEGAL}
	}
	t := p.cur
	p.advance()
	return t
}
```

### Program + top-level

```go
func (p *Parser) ParseProgram() *Program {
	var stmts []Stmt
	for p.cur.Kind != EOF {
		if p.match(NL) {
			continue
		}
		st := p.parseTopStmt()
		if st != nil {
			stmts = append(stmts, st)
		} else {
			p.syncTop()
		}
	}
	return &Program{Stmts: stmts}
}

func (p *Parser) parseTopStmt() Stmt {
	switch p.cur.Kind {
	case KW_BASE, KW_TIMEOUT:
		return p.parseSetting()
	case KW_IMPORT:
		return p.parseImport()
	case KW_LET:
		return p.parseLet()
	case KW_REQ:
		return p.parseReqDecl()
	case KW_FLOW:
		return p.parseFlowDecl()
	default:
		p.errs = append(p.errs, fmt.Errorf("unexpected token %v at %d:%d", p.cur.Kind, p.cur.Line, p.cur.Col))
		return nil
	}
}
```

---

## 5) Parsing `req` blocks (statements + hooks + assertions)

Key idea: parse “lines” until `DEDENT`. Inside, decide what kind of line it is by first token.

```go
func (p *Parser) parseReqDecl() Stmt {
	p.expect(KW_REQ, "expected 'req'")
	name := p.expect(IDENT, "expected request name").Lit

	var parent *string
	if p.match(LPAREN) {
		par := p.expect(IDENT, "expected parent request name").Lit
		parent = &par
		p.expect(RPAREN, "expected ')'")
	}

	p.expect(COLON, "expected ':' after req header")
	p.expect(NL, "expected newline after req header")
	p.expect(INDENT, "expected indented req block")

	var lines []ReqLine
	for p.cur.Kind != DEDENT && p.cur.Kind != EOF {
		if p.match(NL) { continue }

		switch p.cur.Kind {
		// http method -> http line
		case KW_GET, KW_POST_M, KW_PUT, KW_PATCH, KW_DELETE, KW_HEAD, KW_OPTIONS:
			lines = append(lines, p.parseHttpLine())
			p.expect(NL, "expected newline after http line")

		// directives
		case KW_JSON, KW_HEADER, KW_QUERY, KW_AUTH:
			lines = append(lines, p.parseDirective())
			p.expect(NL, "expected newline after directive")

		// hooks
		case KW_PRE, KW_POST:
			lines = append(lines, p.parseHookBlock())
			// hook block ends with optional NL; you can enforce NL if you want

		// request-level assert
		case QUESTION:
			lines = append(lines, p.parseAssertLine())
			p.expect(NL, "expected newline after assertion")

		// let
		case KW_LET:
			lines = append(lines, p.parseLet().(ReqLine))
			p.expect(NL, "expected newline after let")

		default:
			p.errs = append(p.errs, fmt.Errorf("invalid req line at %d:%d", p.cur.Line, p.cur.Col))
			p.syncLine()
		}
	}

	p.expect(DEDENT, "expected end of req block")
	return &ReqDecl{Name: name, Parent: parent, Lines: lines}
}
```

Hook block parsing (brace-scoped, statements separated by NL or `;`):

```go
func (p *Parser) parseHookBlock() ReqLine {
	kindTok := p.cur
	if !p.match(KW_PRE) && !p.match(KW_POST) {
		p.errs = append(p.errs, fmt.Errorf("expected pre/post hook"))
		return &HookBlock{}
	}
	p.expect(KW_HOOK, "expected 'hook'")
	p.expect(LBRACE, "expected '{'")

	var stmts []HookStmt
	for p.cur.Kind != RBRACE && p.cur.Kind != EOF {
		if p.match(NL) || p.match(SEMICOLON /* if you add */) {
			continue
		}
		stmts = append(stmts, p.parseHookStmt())
		// allow optional NL/;
		if p.cur.Kind == NL { p.advance() }
	}

	p.expect(RBRACE, "expected '}'")
	// optional NL
	if p.cur.Kind == NL { p.advance() }

	return &HookBlock{
		Kind:  kindTok.Kind, // KW_PRE or KW_POST
		Stmts: stmts,
	}
}
```

---

## 6) Parsing flows with your constraints

Flow format enforced at parse time:

* Prelude: `let` only (0+)
* Then exactly **one** chain line with **at least one** `->`
* Postlude: assertions only (0+)

Plus: allow alias `listOrders : orders1`

```go
func (p *Parser) parseFlowDecl() Stmt {
	p.expect(KW_FLOW, "expected 'flow'")
	name := p.expect(STRING, "expected flow name string").Lit

	p.expect(COLON, "expected ':' after flow name")
	p.expect(NL, "expected newline after flow header")
	p.expect(INDENT, "expected indented flow block")

	// Prelude: lets only
	var prelude []*LetStmt
	for p.cur.Kind == KW_LET || p.cur.Kind == NL {
		if p.match(NL) { continue }
		ls := p.parseLet().(*LetStmt)
		prelude = append(prelude, ls)
		p.expect(NL, "expected newline after let")
	}

	// Chain line (must contain at least one ->)
	chain := p.parseFlowChainLine()
	p.expect(NL, "expected newline after chain line")

	// Postlude: assertions only
	var asserts []*AssertStmt
	for p.cur.Kind != DEDENT && p.cur.Kind != EOF {
		if p.match(NL) { continue }
		if p.cur.Kind != QUESTION {
			p.errs = append(p.errs, fmt.Errorf("only assertions allowed after chain in flow (%d:%d)", p.cur.Line, p.cur.Col))
			p.syncLine()
			continue
		}
		as := p.parseAssertLine().(*AssertStmt)
		asserts = append(asserts, as)
		p.expect(NL, "expected newline after assertion")
	}

	p.expect(DEDENT, "expected end of flow block")
	return &FlowDecl{Name: name, Prelude: prelude, Chain: chain, Asserts: asserts}
}

func (p *Parser) parseFlowChainLine() []FlowStep {
	steps := []FlowStep{p.parseFlowStepRef()}
	arrowCount := 0
	for p.cur.Kind == ARROW {
		arrowCount++
		p.advance()
		steps = append(steps, p.parseFlowStepRef())
	}
	if arrowCount == 0 {
		p.errs = append(p.errs, fmt.Errorf("flow chain must use '->' format"))
	}
	return steps
}

func (p *Parser) parseFlowStepRef() FlowStep {
	reqName := p.expect(IDENT, "expected request name in flow").Lit
	var alias *string
	if p.match(COLON) {
		a := p.expect(IDENT, "expected alias after ':'").Lit
		alias = &a
	}
	return FlowStep{ReqName: reqName, Alias: alias}
}
```

---

## 7) Pratt parser for expressions (operators + postfix)

### Precedence table (suggested)

Highest → lowest:

1. Postfix: call `()`, field `.x`, index `[e]`
2. Prefix: `not`, unary `+ -`
3. Multiplicative: `* / %`
4. Additive: `+ -`
5. Compare/match/membership: `== != < <= > >= in contains ~`
6. `and`
7. `or`

### Pratt skeleton

```go
type Prec int

const (
	PREC_LOWEST Prec = iota
	PREC_OR
	PREC_AND
	PREC_CMP
	PREC_ADD
	PREC_MUL
	PREC_PREFIX
	PREC_POSTFIX
)

func infixPrec(k Kind) (Prec, bool) {
	switch k {
	case OP_OR:
		return PREC_OR, true
	case OP_AND:
		return PREC_AND, true
	case OP_EQ, OP_NE, OP_LT, OP_LTE, OP_GT, OP_GTE, OP_IN, OP_CONTAINS, OP_TILDE:
		return PREC_CMP, true
	case OP_PLUS, OP_MINUS:
		return PREC_ADD, true
	case OP_MUL, OP_DIV, OP_MOD:
		return PREC_MUL, true
	default:
		return 0, false
	}
}

func (p *Parser) parseExpr(min Prec) Expr {
	left := p.parsePrefix()

	// postfix loop (call/field/index)
	for {
		if p.cur.Kind == LPAREN {
			left = p.parseCall(left)
			continue
		}
		if p.cur.Kind == DOT {
			left = p.parseField(left)
			continue
		}
		if p.cur.Kind == LBRACK {
			left = p.parseIndex(left)
			continue
		}
		break
	}

	for {
		prec, ok := infixPrec(p.cur.Kind)
		if !ok || prec < min {
			break
		}
		op := p.cur
		p.advance()
		right := p.parseExpr(prec + 1)
		left = &BinaryExpr{Op: op.Kind, Left: left, Right: right}

		// allow postfix again after building a binary node
		for {
			if p.cur.Kind == LPAREN { left = p.parseCall(left); continue }
			if p.cur.Kind == DOT    { left = p.parseField(left); continue }
			if p.cur.Kind == LBRACK { left = p.parseIndex(left); continue }
			break
		}
	}
	return left
}

func (p *Parser) parsePrefix() Expr {
	switch p.cur.Kind {
	case OP_NOT, OP_PLUS, OP_MINUS:
		op := p.cur.Kind
		p.advance()
		x := p.parseExpr(PREC_PREFIX)
		return &UnaryExpr{Op: op, X: x}
	default:
		return p.parsePrimary()
	}
}
```

Primary supports:

* literals: string/number/bool/null
* identifier (`token`, `orders1`, `listOrders`)
* `$`
* object `{...}` and array `[...]`

Postfix gives you `orders1.res.items[0]`, `len(listOrders.res.items)`, etc.

---

## 8) Where semantic checks fit best (with this parser)

Do **not** overcomplicate parsing with semantics. Keep parsing deterministic, then run passes:

* **Pass: import graph** (no flows in imported files)
* **Pass: request table** (unique names)
* **Pass: inheritance expansion** (detect cycles)
* **Pass: flow validation** (unique bindings: alias or req name)
* **Pass: undefined variables** (including path params like `:group_id`)
* **Pass: hook restrictions** (e.g., no `res` usage in `pre hook`, no assignment to `res`)

Your earlier semantic checklist maps directly to these passes.

---

## 9) Practical dev workflow

* Start by writing golden tests that parse scripts into AST and snapshot them.
* Then add semantic tests: missing req in flow, alias collision, missing path var, flow asserts before chain, etc.
* Add fuzz tests for the lexer (especially indentation + hooks + strings).

