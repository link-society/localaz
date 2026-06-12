package monitorserver

import (
	"fmt"
	"strconv"
	"strings"
)

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
