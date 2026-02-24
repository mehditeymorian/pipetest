package lexer

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

type tokenSnapshot struct {
	Kind  string           `json:"kind"`
	Lit   string           `json:"lit"`
	Start positionSnapshot `json:"start"`
	End   positionSnapshot `json:"end"`
}

type positionSnapshot struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

func snapshotTokens(tokens []Token) []tokenSnapshot {
	out := make([]tokenSnapshot, 0, len(tokens))
	for _, tok := range tokens {
		out = append(out, tokenSnapshot{
			Kind: tok.Kind.String(),
			Lit:  tok.Lit,
			Start: positionSnapshot{
				Offset: tok.Span.Start.Offset,
				Line:   tok.Span.Start.Line,
				Column: tok.Span.Start.Column,
			},
			End: positionSnapshot{
				Offset: tok.Span.End.Offset,
				Line:   tok.Span.End.Line,
				Column: tok.Span.End.Column,
			},
		})
	}
	return out
}

func TestLexerValidFiles(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "lexer")
	paths, err := filepath.Glob(filepath.Join(root, "valid", "*.pt"))
	if err != nil {
		t.Fatalf("glob valid: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no valid lexer fixtures found")
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, errs := Lex(path, string(src))
		if len(errs) > 0 {
			t.Fatalf("expected no errors for %s, got %d", path, len(errs))
		}
	}
}

func TestLexerInvalidFiles(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "lexer")
	paths, err := filepath.Glob(filepath.Join(root, "invalid", "*.pt"))
	if err != nil {
		t.Fatalf("glob invalid: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no invalid lexer fixtures found")
	}

	expected := map[string]string{
		"tab-indentation.pt":     ErrTab,
		"invalid-dedent.pt":      ErrDedent,
		"unterminated-string.pt": ErrUnterminatedString,
		"unclosed-hook.pt":       ErrUnterminatedHook,
	}

	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, errs := Lex(path, string(src))
		if len(errs) == 0 {
			t.Fatalf("expected errors for %s", path)
		}
		if code, ok := expected[filepath.Base(path)]; ok {
			found := false
			for _, le := range errs {
				if le.Code == code {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected error code %s for %s", code, path)
			}
		}
	}
}

func TestLexerGolden(t *testing.T) {
	cases := []struct {
		name       string
		inputPath  string
		goldenPath string
	}{
		{
			name:       "request-with-directives",
			inputPath:  filepath.Join("..", "..", "testdata", "lexer", "valid", "request-with-directives.pt"),
			goldenPath: filepath.Join("..", "..", "testdata", "lexer", "golden", "request-with-directives.tokens.json"),
		},
		{
			name:       "flow-with-chain",
			inputPath:  filepath.Join("..", "..", "testdata", "lexer", "valid", "flow-with-chain.pt"),
			goldenPath: filepath.Join("..", "..", "testdata", "lexer", "golden", "flow-with-chain.tokens.json"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.inputPath)
			if err != nil {
				t.Fatalf("read %s: %v", tc.inputPath, err)
			}
			tokens, errs := Lex(tc.inputPath, string(src))
			if len(errs) > 0 {
				t.Fatalf("unexpected lexer errors: %v", errs)
			}

			got := snapshotTokens(tokens)
			if *updateGolden {
				data, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatalf("marshal golden: %v", err)
				}
				if err := os.WriteFile(tc.goldenPath, append(data, '\n'), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			data, err := os.ReadFile(tc.goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want []tokenSnapshot
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatalf("unmarshal golden: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				if *updateGolden {
					return
				}
				t.Fatalf("tokens mismatch for %s (re-run with -update to refresh)", tc.name)
			}
		})
	}
}
