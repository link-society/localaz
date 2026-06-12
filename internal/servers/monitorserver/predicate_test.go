package monitorserver

import (
	"testing"

	"localaz.dev/internal/stores/monitorstore"
)

func TestParseComparison(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantCol string
		wantOp  string
		wantErr bool
	}{
		{"eq string", `Level == "Error"`, "Level", "==", false},
		{"ne number", "Code != 500", "Code", "!=", false},
		{"le longest match wins", "Code <= 10", "Code", "<=", false},
		{"ge longest match wins", "Code >= 10", "Code", ">=", false},
		{"lt", "Code < 10", "Code", "<", false},
		{"gt", "Code > 10", "Code", ">", false},
		{"no operator", "Level", "", "", true},
		{"missing lhs", `== "Error"`, "", "", true},
		{"missing rhs", "Code ==", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := parseComparison(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseComparison(%q) = nil error, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseComparison(%q) error: %v", tc.input, err)
			}
			if c.column != tc.wantCol || c.op != tc.wantOp {
				t.Fatalf("parseComparison(%q) = col %q op %q, want col %q op %q",
					tc.input, c.column, c.op, tc.wantCol, tc.wantOp)
			}
		})
	}
}

// TestIndexOperatorQuoteAware verifies the operator splitter ignores operators
// that appear inside quoted string literals.
func TestIndexOperatorQuoteAware(t *testing.T) {
	// The "==" inside the quoted value must be ignored; only the real one counts.
	c, err := parseComparison(`Msg == "a == b"`)
	if err != nil {
		t.Fatalf("parseComparison error: %v", err)
	}
	if c.column != "Msg" || c.op != "==" || c.lit.str != "a == b" {
		t.Fatalf("got col=%q op=%q str=%q, want Msg == 'a == b'", c.column, c.op, c.lit.str)
	}

	// A '<' living inside quotes on the LHS-side value should not be picked.
	if idx := indexOperator(`"<" == x`, "<"); idx != -1 {
		t.Fatalf("indexOperator found '<' inside quotes at %d, want -1", idx)
	}
}

func TestSplitKeywordQuoteAware(t *testing.T) {
	// "or" inside the quoted string must not split the expression.
	parts := splitKeyword(`Msg == "a or b" and Code > 1`, "or")
	if len(parts) != 1 {
		t.Fatalf("splitKeyword on 'or' split quoted value: %v", parts)
	}

	parts = splitKeyword(`A == 1 or B == 2 or C == 3`, "or")
	if len(parts) != 3 {
		t.Fatalf("splitKeyword got %d parts, want 3: %v", len(parts), parts)
	}
}

func TestPredicateEval(t *testing.T) {
	row := monitorstore.Row{"Level": "Error", "Code": float64(500), "Active": true}

	cases := []struct {
		name string
		expr string
		want bool
	}{
		{"simple match", `Level == "Error"`, true},
		{"simple miss", `Level == "Info"`, false},
		{"and match", `Level == "Error" and Code == 500`, true},
		{"and miss", `Level == "Error" and Code == 1`, false},
		{"or match", `Level == "Info" or Code == 500`, true},
		{"or miss", `Level == "Info" or Code == 1`, false},
		{"bool eq", "Active == true", true},
		{"number gt", "Code > 100", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := parsePredicate(tc.expr)
			if err != nil {
				t.Fatalf("parsePredicate(%q) error: %v", tc.expr, err)
			}
			got, err := p.eval(row)
			if err != nil {
				t.Fatalf("eval(%q) error: %v", tc.expr, err)
			}
			if got != tc.want {
				t.Fatalf("predicate %q eval=%v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestParseLiteral(t *testing.T) {
	cases := []struct {
		input    string
		wantKind litKind
		wantErr  bool
	}{
		{`"hello"`, litString, false},
		{`'hello'`, litString, false},
		{"true", litBool, false},
		{"FALSE", litBool, false},
		{"42", litNumber, false},
		{"-3.14", litNumber, false},
		{"bareword", 0, true},
	}
	for _, tc := range cases {
		lit, err := parseLiteral(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseLiteral(%q) = nil error, want error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseLiteral(%q) error: %v", tc.input, err)
		}
		if lit.kind != tc.wantKind {
			t.Fatalf("parseLiteral(%q).kind = %v, want %v", tc.input, lit.kind, tc.wantKind)
		}
	}
}

func TestComparisonBoolOperatorUnsupported(t *testing.T) {
	c, err := parseComparison("Active > true")
	if err != nil {
		t.Fatalf("parseComparison error: %v", err)
	}
	// '>' is not valid for boolean literals; eval should report an error.
	if _, err := c.eval(monitorstore.Row{"Active": true}); err == nil {
		t.Fatalf("eval of '>' on bool literal = nil error, want error")
	}
}
