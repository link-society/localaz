package monitorserver

import (
	"fmt"
	"strconv"
	"strings"

	"localaz.dev/internal/monitorstore"
)

// predicate is a boolean expression evaluated against a single row. It is a
// disjunction (OR) of conjunctions (AND) of comparisons, matching the
// documented KQL where-clause subset (no parentheses or functions).
type predicate struct {
	// orGroups is satisfied when any group is satisfied; a group is satisfied
	// when all of its comparisons hold.
	orGroups [][]comparison
}

// comparison is a single "<column> <op> <literal>" test.
type comparison struct {
	column string
	op     string
	lit    literal
}

// literal is a parsed right-hand-side value.
type literal struct {
	str    string
	num    float64
	b      bool
	kind   litKind
	hasNum bool
}

type litKind int

const (
	litString litKind = iota
	litNumber
	litBool
)

// parsePredicate parses a where-clause expression into a predicate.
func parsePredicate(expr string) (predicate, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return predicate{}, fmt.Errorf("empty where expression")
	}
	var p predicate
	for _, orPart := range splitKeyword(expr, "or") {
		var group []comparison
		for _, andPart := range splitKeyword(orPart, "and") {
			c, err := parseComparison(andPart)
			if err != nil {
				return predicate{}, err
			}
			group = append(group, c)
		}
		p.orGroups = append(p.orGroups, group)
	}
	return p, nil
}

// eval reports whether the row satisfies the predicate.
func (p predicate) eval(row monitorstore.Row) (bool, error) {
	for _, group := range p.orGroups {
		all := true
		for _, c := range group {
			ok, err := c.eval(row)
			if err != nil {
				return false, err
			}
			if !ok {
				all = false
				break
			}
		}
		if all {
			return true, nil
		}
	}
	return false, nil
}

// parseComparison parses "<column> <op> <literal>".
func parseComparison(s string) (comparison, error) {
	s = strings.TrimSpace(s)
	// Operators are checked longest-first so "<=" wins over "<".
	for _, op := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		idx := indexOperator(s, op)
		if idx < 0 {
			continue
		}
		col := strings.TrimSpace(s[:idx])
		rhs := strings.TrimSpace(s[idx+len(op):])
		if col == "" || rhs == "" {
			return comparison{}, fmt.Errorf("malformed comparison %q", s)
		}
		lit, err := parseLiteral(rhs)
		if err != nil {
			return comparison{}, err
		}
		return comparison{column: col, op: op, lit: lit}, nil
	}
	return comparison{}, fmt.Errorf("no comparison operator in %q", s)
}

// eval reports whether the comparison holds for the row.
func (c comparison) eval(row monitorstore.Row) (bool, error) {
	val := row[c.column]
	switch c.lit.kind {
	case litBool:
		b, ok := val.(bool)
		if !ok {
			return false, nil
		}
		switch c.op {
		case "==":
			return b == c.lit.b, nil
		case "!=":
			return b != c.lit.b, nil
		default:
			return false, fmt.Errorf("operator %s not supported for boolean literals", c.op)
		}
	case litNumber:
		f, ok := toFloat(val)
		if !ok {
			return false, nil
		}
		return numericResult(f, c.op, c.lit.num), nil
	default: // litString
		s, ok := val.(string)
		if !ok {
			return false, nil
		}
		return stringResult(s, c.op, c.lit.str), nil
	}
}

// parseLiteral parses a quoted string, a number, or a boolean keyword.
func parseLiteral(s string) (literal, error) {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return literal{kind: litString, str: s[1 : len(s)-1]}, nil
		}
	}
	switch strings.ToLower(s) {
	case "true":
		return literal{kind: litBool, b: true}, nil
	case "false":
		return literal{kind: litBool, b: false}, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return literal{kind: litNumber, num: f, hasNum: true}, nil
	}
	return literal{}, fmt.Errorf("unsupported literal %q (use a quoted string, number, or true/false)", s)
}

// numericResult applies op to two numbers.
func numericResult(a float64, op string, b float64) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

// stringResult applies op to two strings lexicographically.
func stringResult(a, op, b string) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

// toFloat coerces a JSON-decoded value to float64.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// compareValues orders two JSON values for sorting: -1, 0, or 1. Numbers sort
// numerically, strings lexically, and unlike/absent values are treated as
// equal so the sort stays stable.
func compareValues(a, b any) int {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return strings.Compare(as, bs)
	}
	return 0
}

// splitKeyword splits s on the whitespace-delimited keyword (case-insensitive),
// ignoring occurrences inside quoted string literals.
func splitKeyword(s, keyword string) []string {
	var parts []string
	var buf strings.Builder
	var quote byte
	tokens := strings.Fields(s)
	for _, tok := range tokens {
		if quote == 0 && strings.EqualFold(tok, keyword) {
			parts = append(parts, strings.TrimSpace(buf.String()))
			buf.Reset()
			continue
		}
		for i := 0; i < len(tok); i++ {
			ch := tok[i]
			if quote == 0 && (ch == '\'' || ch == '"') {
				quote = ch
			} else if quote != 0 && ch == quote {
				quote = 0
			}
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(tok)
	}
	parts = append(parts, strings.TrimSpace(buf.String()))
	return parts
}

// indexOperator finds op in s while skipping quoted string literals.
func indexOperator(s, op string) int {
	var quote byte
	for i := 0; i+len(op) <= len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if strings.HasPrefix(s[i:], op) {
			return i
		}
	}
	return -1
}
