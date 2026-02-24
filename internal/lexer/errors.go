package lexer

import "fmt"

const (
	ErrTab                   = "E_PARSE_TAB"
	ErrIndent                = "E_PARSE_INDENT"
	ErrDedent                = "E_PARSE_DEDENT"
	ErrUnterminatedString    = "E_PARSE_UNTERMINATED_STRING"
	ErrUnterminatedRaw       = "E_PARSE_UNTERMINATED_RAW_STRING"
	ErrUnterminatedHook      = "E_PARSE_UNTERMINATED_HOOK"
	ErrUnterminatedDelimiter = "E_PARSE_UNTERMINATED_DELIM"
	ErrUnmatchedBrace        = "E_PARSE_UNMATCHED_BRACE"
	ErrUnexpectedChar        = "E_PARSE_UNEXPECTED_CHAR"
)

// LexError captures a lexer diagnostic.
type LexError struct {
	Code    string
	Message string
	Hint    string
	File    string
	Span    Span
}

func (e LexError) Error() string {
	return fmt.Sprintf("%s %s:%d:%d %s", e.Code, e.File, e.Span.Start.Line, e.Span.Start.Column, e.Message)
}
