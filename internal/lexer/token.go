package lexer

// Kind represents a token kind.
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
	BARE_KEY
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
	KW_PRINT
	KW_PRINTLN
	KW_PRINTF
	KW_TRUE
	KW_FALSE
	KW_NULL

	// http methods
	KW_GET
	KW_POST_M
	KW_PUT
	KW_PATCH
	KW_DELETE
	KW_HEAD
	KW_OPTIONS

	// operators / punct
	ARROW     // ->
	QUESTION  // ?
	DOLLAR    // $
	COLON     // :
	COMMA     // ,
	DOT       // .
	ASSIGN    // =
	SEMICOLON // ;
	LPAREN    // (
	RPAREN    // )
	LBRACE    // {
	RBRACE    // }
	LBRACK    // [
	RBRACK    // ]

	// expr operators
	OP_OR
	OP_AND
	OP_NOT
	OP_EQ  // ==
	OP_NE  // !=
	OP_LT  // <
	OP_LTE // <=
	OP_GT  // >
	OP_GTE // >=
	OP_IN
	OP_CONTAINS
	OP_TILDE // ~
	OP_PLUS  // +
	OP_MINUS // -
	OP_MUL   // *
	OP_DIV   // /
	OP_MOD   // %
)

var kindNames = [...]string{
	EOF:         "EOF",
	ILLEGAL:     "ILLEGAL",
	NL:          "NL",
	INDENT:      "INDENT",
	DEDENT:      "DEDENT",
	IDENT:       "IDENT",
	BARE_KEY:    "BARE_KEY",
	STRING:      "STRING",
	NUMBER:      "NUMBER",
	DURATION:    "DURATION",
	PATH:        "PATH",
	KW_REQ:      "KW_REQ",
	KW_FLOW:     "KW_FLOW",
	KW_IMPORT:   "KW_IMPORT",
	KW_LET:      "KW_LET",
	KW_BASE:     "KW_BASE",
	KW_TIMEOUT:  "KW_TIMEOUT",
	KW_JSON:     "KW_JSON",
	KW_HEADER:   "KW_HEADER",
	KW_QUERY:    "KW_QUERY",
	KW_AUTH:     "KW_AUTH",
	KW_BEARER:   "KW_BEARER",
	KW_PRE:      "KW_PRE",
	KW_POST:     "KW_POST",
	KW_HOOK:     "KW_HOOK",
	KW_PRINT:    "KW_PRINT",
	KW_PRINTLN:  "KW_PRINTLN",
	KW_PRINTF:   "KW_PRINTF",
	KW_TRUE:     "KW_TRUE",
	KW_FALSE:    "KW_FALSE",
	KW_NULL:     "KW_NULL",
	KW_GET:      "KW_GET",
	KW_POST_M:   "KW_POST_M",
	KW_PUT:      "KW_PUT",
	KW_PATCH:    "KW_PATCH",
	KW_DELETE:   "KW_DELETE",
	KW_HEAD:     "KW_HEAD",
	KW_OPTIONS:  "KW_OPTIONS",
	ARROW:       "ARROW",
	QUESTION:    "QUESTION",
	DOLLAR:      "DOLLAR",
	COLON:       "COLON",
	COMMA:       "COMMA",
	DOT:         "DOT",
	ASSIGN:      "ASSIGN",
	SEMICOLON:   "SEMICOLON",
	LPAREN:      "LPAREN",
	RPAREN:      "RPAREN",
	LBRACE:      "LBRACE",
	RBRACE:      "RBRACE",
	LBRACK:      "LBRACK",
	RBRACK:      "RBRACK",
	OP_OR:       "OP_OR",
	OP_AND:      "OP_AND",
	OP_NOT:      "OP_NOT",
	OP_EQ:       "OP_EQ",
	OP_NE:       "OP_NE",
	OP_LT:       "OP_LT",
	OP_LTE:      "OP_LTE",
	OP_GT:       "OP_GT",
	OP_GTE:      "OP_GTE",
	OP_IN:       "OP_IN",
	OP_CONTAINS: "OP_CONTAINS",
	OP_TILDE:    "OP_TILDE",
	OP_PLUS:     "OP_PLUS",
	OP_MINUS:    "OP_MINUS",
	OP_MUL:      "OP_MUL",
	OP_DIV:      "OP_DIV",
	OP_MOD:      "OP_MOD",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "Kind(" + itoa(int(k)) + ")"
}

// Token represents a lexical token with a source span.
type Token struct {
	Kind Kind
	Lit  string
	Span Span
}

// Position represents a specific point in a source file.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span represents a half-open source range.
type Span struct {
	Start Position
	End   Position
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var buf [20]byte
	idx := len(buf)
	for v > 0 {
		idx--
		buf[idx] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		idx--
		buf[idx] = '-'
	}
	return string(buf[idx:])
}
