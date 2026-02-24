package parser

import (
	"fmt"

	"github.com/mehditeymorian/pipetest/internal/ast"
)

const (
	ErrExpectedToken   = "E_PARSE_EXPECTED_TOKEN"
	ErrUnexpectedToken = "E_PARSE_UNEXPECTED_TOKEN"
	ErrInvalidLine     = "E_PARSE_INVALID_LINE"
	ErrInvalidExpr     = "E_PARSE_INVALID_EXPR"
	ErrInvalidFlow     = "E_PARSE_FLOW_SHAPE"
)

// ParseError captures a parser diagnostic.
type ParseError struct {
	Code    string
	Message string
	Hint    string
	File    string
	Span    ast.Span
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s %s:%d:%d %s", e.Code, e.File, e.Span.Start.Line, e.Span.Start.Column, e.Message)
}
