package monitorserver

import (
	"fmt"
	"strings"

	"localaz.dev/internal/stores/monitorstore"
)

// predicate is a boolean expression evaluated against a single row. It is a
// disjunction (OR) of conjunctions (AND) of comparisons, matching the
// documented KQL where-clause subset (no parentheses or functions).
type predicate struct {
	// orGroups is satisfied when any group is satisfied; a group is satisfied
	// when all of its comparisons hold.
	orGroups [][]comparison
}

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
