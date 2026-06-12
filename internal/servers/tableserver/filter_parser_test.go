package tableserver

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// props builds an entity property map from a JSON object literal for tests.
func props(t *testing.T, obj string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(obj), &m); err != nil {
		t.Fatalf("invalid test props %q: %v", obj, err)
	}
	return m
}

func TestParseFilterValid(t *testing.T) {
	entity := props(t, `{"Age":30,"Name":"alice","Active":true}`)

	cases := []struct {
		name string
		expr string
		want bool
	}{
		{"empty matches everything", "", true},

		// numeric comparisons
		{"eq number true", "Age eq 30", true},
		{"eq number false", "Age eq 31", false},
		{"ne number true", "Age ne 31", true},
		{"gt number", "Age gt 29", true},
		{"ge number equal", "Age ge 30", true},
		{"lt number", "Age lt 31", true},
		{"le number equal", "Age le 30", true},
		{"gt number false", "Age gt 30", false},

		// string comparisons
		{"eq string true", "Name eq 'alice'", true},
		{"eq string false", "Name eq 'bob'", false},
		{"ne string", "Name ne 'bob'", true},
		{"gt string lexical", "Name gt 'aaa'", true},

		// bool comparisons
		{"eq bool true", "Active eq true", true},
		{"eq bool false", "Active eq false", false},
		{"ne bool", "Active ne false", true},

		// and / or
		{"and both true", "Age eq 30 and Name eq 'alice'", true},
		{"and one false", "Age eq 30 and Name eq 'bob'", false},
		{"or one true", "Age eq 99 or Name eq 'alice'", true},
		{"or both false", "Age eq 99 or Name eq 'bob'", false},

		// parentheses
		{"parens grouping", "(Age eq 30)", true},
		{"parens or then and", "(Age eq 99 or Name eq 'alice') and Active eq true", true},
		{"parens or then and false", "(Age eq 99 or Name eq 'bob') and Active eq true", false},

		// operator precedence: AND binds tighter than OR.
		// false and false  =>  false; false or true => true.
		{"precedence and tighter than or", "Age eq 99 and Name eq 'bob' or Active eq true", true},
		// true and false => false; false or false => false. Confirms the AND is
		// grouped before the OR (otherwise true and (false or false) = false too,
		// so use a case that distinguishes): true or true and false.
		{"precedence or with trailing and", "Active eq true or Age eq 99 and Name eq 'bob'", true},

		// missing property: ne is true, others false (existing semantics).
		{"missing prop ne", "Missing ne 'x'", true},
		{"missing prop eq", "Missing eq 'x'", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, err := parseFilter(tc.expr)
			if err != nil {
				t.Fatalf("parseFilter(%q) returned error: %v", tc.expr, err)
			}
			if got := fn(entity); got != tc.want {
				t.Fatalf("parseFilter(%q) matched=%v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestParseFilterMalformed(t *testing.T) {
	cases := []struct {
		name string
		expr string
	}{
		{"trailing garbage", "Age eq 30 garbage"},
		{"missing operand", "Age eq"},
		{"missing operator", "Age 30"},
		{"bare operator", "eq"},
		{"unterminated string", "Name eq 'alice"},
		{"unbalanced open paren", "(Age eq 30"},
		{"unbalanced close paren", "Age eq 30)"},
		{"empty parens", "()"},
		{"dangling and", "Age eq 30 and"},
		{"leading or", "or Age eq 30"},
		{"operator only value", "Age eq eq"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, err := func() (fn filterFunc, err error) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("parseFilter(%q) panicked: %v", tc.expr, r)
					}
				}()
				return parseFilter(tc.expr)
			}()
			if err == nil {
				t.Fatalf("parseFilter(%q) = nil error, want errFilter (fn=%v)", tc.expr, fn != nil)
			}
			if !errors.Is(err, errFilter) {
				t.Fatalf("parseFilter(%q) error = %v, want errFilter", tc.expr, err)
			}
		})
	}
}

// TestParseFilterDeepNesting ensures attacker-supplied deeply nested
// parentheses are rejected with errFilter rather than overflowing the
// goroutine stack.
func TestParseFilterDeepNesting(t *testing.T) {
	expr := strings.Repeat("(", 1000) + "Age eq 30" + strings.Repeat(")", 1000)

	fn, err := func() (fn filterFunc, err error) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parseFilter on deeply nested input panicked: %v", r)
			}
		}()
		return parseFilter(expr)
	}()
	if err == nil {
		t.Fatalf("parseFilter on 1000 nested parens = nil error, want errFilter (fn=%v)", fn != nil)
	}
	if !errors.Is(err, errFilter) {
		t.Fatalf("parseFilter deep nesting error = %v, want errFilter", err)
	}
}

// TestParseFilterDepthBoundary checks the boundary around maxFilterDepth: a
// nesting depth at the limit parses, one beyond it is rejected.
func TestParseFilterDepthBoundary(t *testing.T) {
	atLimit := strings.Repeat("(", maxFilterDepth) + "Age eq 30" + strings.Repeat(")", maxFilterDepth)
	if _, err := parseFilter(atLimit); err != nil {
		t.Fatalf("parseFilter at depth %d returned error: %v", maxFilterDepth, err)
	}

	overLimit := strings.Repeat("(", maxFilterDepth+1) + "Age eq 30" + strings.Repeat(")", maxFilterDepth+1)
	_, err := parseFilter(overLimit)
	if !errors.Is(err, errFilter) {
		t.Fatalf("parseFilter at depth %d error = %v, want errFilter", maxFilterDepth+1, err)
	}
}
