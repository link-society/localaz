package monitorserver

import (
	"fmt"
	"strings"

	"localaz.dev/internal/stores/monitorstore"
)

// comparison is a single "<column> <op> <literal>" test.
type comparison struct {
	column string
	op     string
	lit    literal
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
