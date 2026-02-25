package lexer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer converts source text into tokens with layout handling.
type Lexer struct {
	path string
	src  string

	pos  int
	line int
	col  int

	lineStart    bool
	lineStartPos Position

	indentStack  []int
	expectIndent bool
	lineHadColon bool

	hookCandidate    Kind
	pendingHookBrace bool

	allowPath    bool
	allowBareKey bool

	hookDepth      int
	parenDepth     int
	bracketDepth   int
	braceExprDepth int

	queue []Token
	errs  []LexError

	eofProcessed bool
}

// NewLexer returns a new lexer for the provided source.
func NewLexer(path, src string) *Lexer {
	return &Lexer{
		path:         path,
		src:          src,
		line:         1,
		col:          1,
		lineStart:    true,
		lineStartPos: Position{Offset: 0, Line: 1, Column: 1},
		indentStack:  []int{0},
	}
}

// Lex returns all tokens and lexer errors for the provided source.
func Lex(path, src string) ([]Token, []LexError) {
	lx := NewLexer(path, src)
	var toks []Token
	for {
		tok := lx.Next()
		toks = append(toks, tok)
		if tok.Kind == EOF {
			break
		}
	}
	return toks, lx.Errors()
}

// Errors returns accumulated lexer diagnostics.
func (l *Lexer) Errors() []LexError {
	return l.errs
}

// Path returns the source path for diagnostics.
func (l *Lexer) Path() string {
	return l.path
}

// Next returns the next token in the stream.
func (l *Lexer) Next() Token {
	if len(l.queue) > 0 {
		return l.pop()
	}

	for {
		if l.pos >= len(l.src) {
			return l.emitEOF()
		}

		if l.lineStart {
			l.handleLineStart()
			if len(l.queue) > 0 {
				return l.pop()
			}
			if l.pos >= len(l.src) {
				return l.emitEOF()
			}
		}

		r := l.peek()
		switch r {
		case ' ', '\t':
			l.consumeWhitespace()
			continue
		case '#':
			if l.hashStartsComment() {
				l.skipComment()
				continue
			}
			return l.scanToken()
		case '\n', '\r':
			l.handleNewline()
			if len(l.queue) > 0 {
				return l.pop()
			}
			continue
		default:
			return l.scanToken()
		}
	}
}

func (l *Lexer) pop() Token {
	tok := l.queue[0]
	l.queue = l.queue[1:]
	return tok
}

func (l *Lexer) emitEOF() Token {
	if !l.eofProcessed {
		l.eofProcessed = true
		l.allowPath = false
		l.allowBareKey = false
		l.hookCandidate = 0
		l.pendingHookBrace = false

		if l.hookDepth > 0 {
			l.addError(ErrUnterminatedHook, "unterminated hook block", "close the hook block with '}'", spanAt(l.position()))
			l.hookDepth = 0
		}
		if l.parenDepth > 0 || l.bracketDepth > 0 || l.braceExprDepth > 0 {
			l.addError(ErrUnterminatedDelimiter, "unterminated expression grouping", "close the open delimiter before EOF", spanAt(l.position()))
			l.parenDepth = 0
			l.bracketDepth = 0
			l.braceExprDepth = 0
		}

		for len(l.indentStack) > 1 {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			l.queue = append(l.queue, Token{Kind: DEDENT, Span: Span{Start: l.position(), End: l.position()}})
		}
	}

	if len(l.queue) > 0 {
		return l.pop()
	}
	pos := l.position()
	return Token{Kind: EOF, Span: Span{Start: pos, End: pos}}
}

func (l *Lexer) handleLineStart() {
	if !l.lineStart {
		return
	}

	indent := 0
	for {
		r := l.peek()
		if r == '\t' {
			if l.hookDepth == 0 && l.exprDepth() == 0 {
				indent++
			}
			l.advance()
			continue
		}
		if r == ' ' {
			if l.hookDepth == 0 && l.exprDepth() == 0 {
				l.addError(ErrTab, "space indentation is not allowed", "replace spaces with tabs", spanAt(l.position()))
				for l.peek() == ' ' {
					l.advance()
				}
				continue
			}
			l.advance()
			continue
		}
		break
	}

	r := l.peek()
	if r == '#' && l.hashStartsComment() {
		l.skipComment()
		return
	}
	if r == '\n' || r == '\r' || r == 0 {
		return
	}

	if l.hookDepth == 0 && l.exprDepth() == 0 {
		l.processIndent(indent)
		l.expectIndent = false
	}

	l.lineStart = false
}

func (l *Lexer) processIndent(indent int) {
	top := l.indentStack[len(l.indentStack)-1]
	if l.expectIndent {
		if indent <= top {
			l.addError(ErrIndent, "expected an indented block", "indent this line to start the block", spanAt(l.indentPos(indent)))
		} else {
			l.indentStack = append(l.indentStack, indent)
			l.queue = append(l.queue, Token{Kind: INDENT, Span: Span{Start: l.lineStartPos, End: l.indentPos(indent)}})
			return
		}
	}

	if indent > top {
		l.addError(ErrIndent, "unexpected indentation", "remove extra indentation or add ':' to open a block", spanAt(l.indentPos(indent)))
		l.indentStack = append(l.indentStack, indent)
		l.queue = append(l.queue, Token{Kind: INDENT, Span: Span{Start: l.lineStartPos, End: l.indentPos(indent)}})
		return
	}

	if indent < top {
		for len(l.indentStack) > 1 && indent < l.indentStack[len(l.indentStack)-1] {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			l.queue = append(l.queue, Token{Kind: DEDENT, Span: Span{Start: l.lineStartPos, End: l.indentPos(indent)}})
		}
		if indent != l.indentStack[len(l.indentStack)-1] {
			l.addError(ErrDedent, "dedent does not match any outer indentation level", "align indentation with an existing block", spanAt(l.indentPos(indent)))
		}
	}
}

func (l *Lexer) handleNewline() {
	start := l.position()
	l.consumeNewline()
	l.allowPath = false
	l.allowBareKey = false
	l.hookCandidate = 0
	l.pendingHookBrace = false

	if l.exprDepth() == 0 {
		l.queue = append(l.queue, Token{Kind: NL, Span: Span{Start: start, End: l.position()}})
		if l.lineHadColon {
			l.expectIndent = true
		}
		l.lineHadColon = false
	}
}

func (l *Lexer) consumeWhitespace() {
	for {
		r := l.peek()
		switch r {
		case ' ':
			l.advance()
		case '\t':
			l.advance()
		default:
			return
		}
	}
}

func (l *Lexer) hashStartsComment() bool {
	next := l.peekN(1)
	if next == 0 || next == '\n' || next == '\r' {
		return true
	}
	if unicode.IsSpace(next) {
		return true
	}
	return false
}

func (l *Lexer) skipComment() {
	for {
		r := l.peek()
		if r == 0 || r == '\n' || r == '\r' {
			return
		}
		l.advance()
	}
}

func (l *Lexer) scanToken() Token {
	start := l.position()
	l.lineStart = false

	if l.allowBareKey {
		r := l.peek()
		if r == '"' || r == '`' {
			l.allowBareKey = false
			return l.scanString()
		}
		if isBareKeyChar(r) {
			l.allowBareKey = false
			return l.scanBareKey()
		}
		l.allowBareKey = false
	}

	if l.allowPath {
		if path, ok := l.scanPathIfPresent(); ok {
			l.allowPath = false
			return path
		}
		l.allowPath = false
	}

	if tok, ok := l.scanOperatorOrPunct(); ok {
		return tok
	}

	r := l.peek()
	if r == '"' || r == '`' {
		return l.scanString()
	}

	if unicode.IsDigit(r) {
		return l.scanNumberOrDuration()
	}

	if l.peekPathPrefix() {
		return l.scanPath()
	}

	if isIdentStart(r) {
		return l.scanIdentOrKeyword()
	}

	// Unknown character
	l.advance()
	errSpan := Span{Start: start, End: l.position()}
	l.addError(ErrUnexpectedChar, "unexpected character", "remove or replace the character", errSpan)
	return Token{Kind: ILLEGAL, Lit: l.src[start.Offset:l.pos], Span: errSpan}
}

func (l *Lexer) scanOperatorOrPunct() (Token, bool) {
	rest := l.remaining()
	start := l.position()

	if strings.HasPrefix(rest, "->") {
		l.advanceN(2)
		return l.token(ARROW, "->", start), true
	}

	if strings.HasPrefix(rest, "<=") {
		l.advanceN(2)
		return l.token(OP_LTE, "<=", start), true
	}
	if strings.HasPrefix(rest, ">=") {
		l.advanceN(2)
		return l.token(OP_GTE, ">=", start), true
	}
	if strings.HasPrefix(rest, "==") {
		l.advanceN(2)
		return l.token(OP_EQ, "==", start), true
	}
	if strings.HasPrefix(rest, "!=") {
		l.advanceN(2)
		return l.token(OP_NE, "!=", start), true
	}

	switch l.peek() {
	case '<':
		l.advance()
		return l.token(OP_LT, "<", start), true
	case '>':
		l.advance()
		return l.token(OP_GT, ">", start), true
	case '=':
		l.advance()
		return l.token(ASSIGN, "=", start), true
	case ':':
		l.advance()
		return l.token(COLON, ":", start), true
	case '?':
		l.advance()
		return l.token(QUESTION, "?", start), true
	case '$':
		l.advance()
		return l.token(DOLLAR, "$", start), true
	case '#':
		l.advance()
		return l.token(HASH, "#", start), true
	case ',':
		l.advance()
		return l.token(COMMA, ",", start), true
	case '.':
		l.advance()
		return l.token(DOT, ".", start), true
	case ';':
		l.advance()
		return l.token(SEMICOLON, ";", start), true
	case '(':
		l.advance()
		l.parenDepth++
		return l.token(LPAREN, "(", start), true
	case ')':
		l.advance()
		if l.parenDepth > 0 {
			l.parenDepth--
		} else {
			l.addError(ErrUnmatchedBrace, "unmatched ')'", "remove the extra parenthesis", Span{Start: start, End: l.position()})
		}
		return l.token(RPAREN, ")", start), true
	case '[':
		l.advance()
		l.bracketDepth++
		return l.token(LBRACK, "[", start), true
	case ']':
		l.advance()
		if l.bracketDepth > 0 {
			l.bracketDepth--
		} else {
			l.addError(ErrUnmatchedBrace, "unmatched ']'", "remove the extra bracket", Span{Start: start, End: l.position()})
		}
		return l.token(RBRACK, "]", start), true
	case '{':
		l.advance()
		if l.pendingHookBrace {
			l.hookDepth++
			l.pendingHookBrace = false
		} else {
			l.braceExprDepth++
		}
		return l.token(LBRACE, "{", start), true
	case '}':
		l.advance()
		if l.braceExprDepth > 0 {
			l.braceExprDepth--
		} else if l.hookDepth > 0 {
			l.hookDepth--
		} else {
			l.addError(ErrUnmatchedBrace, "unmatched '}'", "remove the extra brace", Span{Start: start, End: l.position()})
		}
		return l.token(RBRACE, "}", start), true
	case '+':
		l.advance()
		return l.token(OP_PLUS, "+", start), true
	case '-':
		l.advance()
		return l.token(OP_MINUS, "-", start), true
	case '*':
		l.advance()
		return l.token(OP_MUL, "*", start), true
	case '/':
		l.advance()
		return l.token(OP_DIV, "/", start), true
	case '%':
		l.advance()
		return l.token(OP_MOD, "%", start), true
	case '~':
		l.advance()
		return l.token(OP_TILDE, "~", start), true
	default:
		return Token{}, false
	}
}

func (l *Lexer) scanString() Token {
	start := l.position()
	quote := l.peek()
	l.advance()

	if quote == '`' {
		for {
			r := l.peek()
			if r == 0 {
				l.addError(ErrUnterminatedRaw, "unterminated raw string", "close the string with a backtick", Span{Start: start, End: l.position()})
				break
			}
			if r == '`' {
				l.advance()
				break
			}
			l.advance()
		}
		return l.token(STRING, l.src[start.Offset:l.pos], start)
	}

	for {
		r := l.peek()
		if r == 0 || r == '\n' || r == '\r' {
			l.addError(ErrUnterminatedString, "unterminated string literal", "close the string with '\"'", Span{Start: start, End: l.position()})
			break
		}
		if r == '"' {
			l.advance()
			break
		}
		if r == '\\' {
			l.advance()
			if l.peek() == 0 {
				break
			}
			l.advance()
			continue
		}
		l.advance()
	}

	return l.token(STRING, l.src[start.Offset:l.pos], start)
}

func (l *Lexer) scanNumberOrDuration() Token {
	start := l.position()
	for unicode.IsDigit(l.peek()) {
		l.advance()
	}
	if l.peek() == '.' {
		next := l.peekN(1)
		if unicode.IsDigit(next) {
			l.advance() // '.'
			for unicode.IsDigit(l.peek()) {
				l.advance()
			}
		}
	}

	if _, ok := l.scanDurationUnit(); ok {
		return l.token(DURATION, l.src[start.Offset:l.pos], start)
	}
	return l.token(NUMBER, l.src[start.Offset:l.pos], start)
}

func (l *Lexer) scanDurationUnit() (string, bool) {
	rest := l.remaining()
	if strings.HasPrefix(rest, "ms") {
		after := rune(0)
		if len(rest) > 2 {
			after, _ = utf8.DecodeRuneInString(rest[2:])
		}
		if !isIdentChar(after) {
			l.advanceN(2)
			return "ms", true
		}
	}

	if len(rest) == 0 {
		return "", false
	}
	unit := rest[:1]
	if unit != "s" && unit != "m" && unit != "h" && unit != "d" {
		return "", false
	}
	after := rune(0)
	if len(rest) > 1 {
		after, _ = utf8.DecodeRuneInString(rest[1:])
	}
	if isIdentChar(after) {
		return "", false
	}
	l.advanceN(1)
	return unit, true
}

func (l *Lexer) scanBareKey() Token {
	start := l.position()
	for isBareKeyChar(l.peek()) {
		l.advance()
	}
	return l.token(BARE_KEY, l.src[start.Offset:l.pos], start)
}

func (l *Lexer) scanIdentOrKeyword() Token {
	start := l.position()
	for isIdentChar(l.peek()) {
		l.advance()
	}
	lit := l.src[start.Offset:l.pos]
	if kind, ok := keywordKinds[lit]; ok {
		return l.token(kind, lit, start)
	}
	return l.token(IDENT, lit, start)
}

func (l *Lexer) peekPathPrefix() bool {
	rest := l.remaining()
	return strings.HasPrefix(rest, "http://") || strings.HasPrefix(rest, "https://")
}

func (l *Lexer) scanPathIfPresent() (Token, bool) {
	if l.peek() == '/' {
		return l.scanPath(), true
	}
	if l.peekPathPrefix() {
		return l.scanPath(), true
	}
	return Token{}, false
}

func (l *Lexer) scanPath() Token {
	start := l.position()
	for {
		r := l.peek()
		if r == 0 || r == '\n' || r == '\r' || unicode.IsSpace(r) || r == '#' {
			break
		}
		l.advance()
	}
	return l.token(PATH, l.src[start.Offset:l.pos], start)
}

func (l *Lexer) token(kind Kind, lit string, start Position) Token {
	end := l.position()
	tok := Token{Kind: kind, Lit: lit, Span: Span{Start: start, End: end}}
	l.afterToken(tok)
	return tok
}

func (l *Lexer) afterToken(tok Token) {
	if tok.Kind == KW_GET || tok.Kind == KW_POST_M || tok.Kind == KW_PUT || tok.Kind == KW_PATCH || tok.Kind == KW_DELETE || tok.Kind == KW_HEAD || tok.Kind == KW_OPTIONS {
		l.allowPath = true
	}
	if tok.Kind == KW_HEADER || tok.Kind == KW_QUERY {
		l.allowBareKey = true
	}

	switch tok.Kind {
	case KW_PRE, KW_POST:
		l.hookCandidate = tok.Kind
	case KW_HOOK:
		if l.hookCandidate != 0 {
			l.pendingHookBrace = true
		}
		l.hookCandidate = 0
	case NL, INDENT, DEDENT:
		// ignore
	default:
		if l.pendingHookBrace && tok.Kind != LBRACE {
			l.pendingHookBrace = false
		}
		if tok.Kind != KW_PRE && tok.Kind != KW_POST && tok.Kind != KW_HOOK {
			l.hookCandidate = 0
		}
	}

	if tok.Kind == COLON && l.exprDepth() == 0 && l.hookDepth == 0 {
		l.lineHadColon = true
	} else if l.lineHadColon && tok.Kind != NL && tok.Kind != INDENT && tok.Kind != DEDENT {
		l.lineHadColon = false
	}
}

func (l *Lexer) exprDepth() int {
	return l.parenDepth + l.bracketDepth + l.braceExprDepth
}

func (l *Lexer) addError(code, msg, hint string, span Span) {
	l.errs = append(l.errs, LexError{Code: code, Message: msg, Hint: hint, File: l.path, Span: span})
}

func spanAt(pos Position) Span {
	return Span{Start: pos, End: pos}
}

func (l *Lexer) position() Position {
	return Position{Offset: l.pos, Line: l.line, Column: l.col}
}

func (l *Lexer) indentPos(indent int) Position {
	return Position{Offset: l.lineStartPos.Offset + indent, Line: l.lineStartPos.Line, Column: 1 + indent}
}

func (l *Lexer) remaining() string {
	if l.pos >= len(l.src) {
		return ""
	}
	return l.src[l.pos:]
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

func (l *Lexer) peekN(n int) rune {
	idx := l.pos
	for i := 0; i <= n; i++ {
		if idx >= len(l.src) {
			return 0
		}
		_, size := utf8.DecodeRuneInString(l.src[idx:])
		if i == n {
			r, _ := utf8.DecodeRuneInString(l.src[idx:])
			return r
		}
		idx += size
	}
	return 0
}

func (l *Lexer) advance() {
	if l.pos >= len(l.src) {
		return
	}
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += size
	if r == '\n' {
		l.line++
		l.col = 1
		l.lineStart = true
		l.lineStartPos = Position{Offset: l.pos, Line: l.line, Column: 1}
		return
	}
	if r == '\r' {
		l.line++
		l.col = 1
		l.lineStart = true
		l.lineStartPos = Position{Offset: l.pos, Line: l.line, Column: 1}
		return
	}
	l.col++
}

func (l *Lexer) advanceN(n int) {
	for i := 0; i < n; i++ {
		l.advance()
	}
}

func (l *Lexer) consumeNewline() {
	if l.pos >= len(l.src) {
		return
	}
	if strings.HasPrefix(l.src[l.pos:], "\r\n") {
		l.pos += 2
	} else {
		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		if r == '\r' || r == '\n' {
			l.pos += size
		}
	}
	l.line++
	l.col = 1
	l.lineStart = true
	l.lineStartPos = Position{Offset: l.pos, Line: l.line, Column: 1}
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isBareKeyChar(r rune) bool {
	return r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

var keywordKinds = map[string]Kind{
	"req":      KW_REQ,
	"flow":     KW_FLOW,
	"import":   KW_IMPORT,
	"let":      KW_LET,
	"base":     KW_BASE,
	"timeout":  KW_TIMEOUT,
	"json":     KW_JSON,
	"header":   KW_HEADER,
	"query":    KW_QUERY,
	"auth":     KW_AUTH,
	"bearer":   KW_BEARER,
	"pre":      KW_PRE,
	"post":     KW_POST,
	"hook":     KW_HOOK,
	"print":    KW_PRINT,
	"println":  KW_PRINTLN,
	"printf":   KW_PRINTF,
	"true":     KW_TRUE,
	"false":    KW_FALSE,
	"null":     KW_NULL,
	"GET":      KW_GET,
	"POST":     KW_POST_M,
	"PUT":      KW_PUT,
	"PATCH":    KW_PATCH,
	"DELETE":   KW_DELETE,
	"HEAD":     KW_HEAD,
	"OPTIONS":  KW_OPTIONS,
	"and":      OP_AND,
	"or":       OP_OR,
	"not":      OP_NOT,
	"in":       OP_IN,
	"contains": OP_CONTAINS,
}
