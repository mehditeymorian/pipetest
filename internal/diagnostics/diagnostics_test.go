package diagnostics

import "testing"

func TestSortAndDedupeNilAndEmpty(t *testing.T) {
	if got := SortAndDedupe(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %#v", got)
	}
	if got := SortAndDedupe([]Diagnostic{}); got != nil {
		t.Fatalf("expected nil for empty input, got %#v", got)
	}
}

func TestSortAndDedupeOrdersByCanonicalKey(t *testing.T) {
	flowA := "flow-a"
	flowB := "flow-b"
	in := []Diagnostic{
		{Code: "E_B", File: "z.pt", Line: 2, Column: 3, Message: "z", Flow: &flowA},
		{Code: "E_A", File: "a.pt", Line: 2, Column: 3, Message: "b", Flow: &flowB},
		{Code: "E_A", File: "a.pt", Line: 1, Column: 1, Message: "b"},
		{Code: "E_A", File: "a.pt", Line: 2, Column: 1, Message: "b"},
		{Code: "E_A", File: "a.pt", Line: 2, Column: 1, Message: "a"},
		{Code: "E_A", File: "a.pt", Line: 2, Column: 1, Message: "a", Related: &Related{File: "r.pt", Line: 3, Column: 2}},
	}

	got := SortAndDedupe(in)
	if len(got) != len(in) {
		t.Fatalf("expected no dedupe in this set, got %d entries", len(got))
	}

	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.File > cur.File {
			t.Fatalf("diagnostics are not sorted by file: %+v then %+v", prev, cur)
		}
	}
	if got[0].Line != 1 || got[0].Column != 1 {
		t.Fatalf("expected earliest source location first, got %+v", got[0])
	}
	if got[len(got)-1].File != "z.pt" {
		t.Fatalf("expected z.pt to be last, got %+v", got[len(got)-1])
	}
}

func TestSortAndDedupeUsesCanonicalTupleWithoutFlowRequest(t *testing.T) {
	flowA := "flow-a"
	flowB := "flow-b"
	reqA := "req-a"
	reqB := "req-b"
	in := []Diagnostic{
		{Code: "E_X", File: "a.pt", Line: 10, Column: 2, Message: "same", Flow: &flowA, Request: &reqA},
		{Code: "E_X", File: "a.pt", Line: 10, Column: 2, Message: "same", Flow: &flowB, Request: &reqB},
	}

	got := SortAndDedupe(in)
	if len(got) != 1 {
		t.Fatalf("expected canonical dedupe to collapse duplicates regardless of flow/request, got %d", len(got))
	}
	if got[0].Flow == nil || *got[0].Flow != flowA {
		t.Fatalf("expected first instance to be preserved, got %+v", got[0])
	}
}

func TestSortAndDedupeIncludesRelatedLocationInDeduping(t *testing.T) {
	in := []Diagnostic{
		{Code: "E_X", File: "a.pt", Line: 10, Column: 2, Message: "same", Related: &Related{File: "r.pt", Line: 1, Column: 1}},
		{Code: "E_X", File: "a.pt", Line: 10, Column: 2, Message: "same", Related: &Related{File: "r.pt", Line: 1, Column: 2}},
	}

	got := SortAndDedupe(in)
	if len(got) != 2 {
		t.Fatalf("expected distinct related locations to remain distinct, got %d", len(got))
	}
}
