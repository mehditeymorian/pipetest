package diagnostics

import (
	"sort"
	"strconv"
)

// Related points to a secondary source location.
type Related struct {
	File    string
	Line    int
	Column  int
	Message string
}

// Diagnostic is the canonical compiler/runtime diagnostic contract.
type Diagnostic struct {
	Severity string
	Code     string
	Message  string
	File     string
	Line     int
	Column   int
	Hint     string
	Related  *Related
	Flow     *string `json:",omitempty"`
	Request  *string `json:",omitempty"`
}

// SortAndDedupe enforces deterministic output ordering and duplicate removal.
func SortAndDedupe(in []Diagnostic) []Diagnostic {
	if len(in) == 0 {
		return nil
	}
	out := append([]Diagnostic(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		ar, br := relatedSortKey(a.Related), relatedSortKey(b.Related)
		if ar.file != br.file {
			return ar.file < br.file
		}
		if ar.line != br.line {
			return ar.line < br.line
		}
		return ar.column < br.column
	})
	seen := map[string]struct{}{}
	result := make([]Diagnostic, 0, len(out))
	for _, d := range out {
		key := dedupeKey(d)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, d)
	}
	return result
}

type relatedKey struct {
	file   string
	line   int
	column int
}

func relatedSortKey(r *Related) relatedKey {
	if r == nil {
		return relatedKey{}
	}
	return relatedKey{file: r.File, line: r.Line, column: r.Column}
}

func dedupeKey(d Diagnostic) string {
	rk := relatedSortKey(d.Related)
	return d.Code + "|" + d.File + "|" + strconv.Itoa(d.Line) + "|" + strconv.Itoa(d.Column) + "|" + d.Message + "|" + rk.file + "|" + strconv.Itoa(rk.line) + "|" + strconv.Itoa(rk.column) + "|" + ptrStr(d.Flow) + "|" + ptrStr(d.Request)
}

func ptrStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
