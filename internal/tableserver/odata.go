package tableserver

import (
	"encoding/json"
	"strconv"
	"strings"

	"localaz.dev/internal/tablestore"
)

// entityTimeFmt is the ISO 8601 layout Azure uses for the Timestamp property.
const entityTimeFmt = "2006-01-02T15:04:05.0000000Z"

// --- URL key / predicate parsing ---

// splitPredicate separates a resource segment into the table name and the key
// predicate inside parentheses. "t", "t()" yield an empty predicate; the form
// "t(PartitionKey='p',RowKey='r')" yields the inner text.
func splitPredicate(seg string) (name, predicate string) {
	i := strings.IndexByte(seg, '(')
	if i < 0 {
		return seg, ""
	}
	name = seg[:i]
	predicate = strings.TrimSuffix(seg[i+1:], ")")
	return name, predicate
}

// insideParens returns the text between the first '(' and the last ')'.
func insideParens(seg string) string {
	i := strings.IndexByte(seg, '(')
	if i < 0 {
		return ""
	}
	return strings.TrimSuffix(seg[i+1:], ")")
}

// unquoteKey strips surrounding single quotes and unescapes doubled quotes.
func unquoteKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return strings.ReplaceAll(s, "''", "'")
}

// parseEntityKeys extracts the PartitionKey and RowKey from a predicate such as
// "PartitionKey='p',RowKey='r'" (either order).
func parseEntityKeys(predicate string) (pk, rk string) {
	for _, part := range splitTopLevel(predicate, ',') {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(part[:eq])
		val := unquoteKey(part[eq+1:])
		switch strings.ToLower(key) {
		case "partitionkey":
			pk = val
		case "rowkey":
			rk = val
		}
	}
	return pk, rk
}

// splitTopLevel splits on sep, ignoring separators inside single-quoted strings.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'':
			inQuote = !inQuote
			cur.WriteByte(c)
		case c == sep && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// --- entity rendering ---

// renderEntity produces the response map for an entity: its user properties
// plus the server-managed Timestamp and odata.etag.
func renderEntity(e *tablestore.Entity) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(e.Props)+2)
	for k, v := range e.Props {
		out[k] = v
	}
	out["Timestamp"], _ = json.Marshal(e.Timestamp.UTC().Format(entityTimeFmt))
	out["odata.etag"], _ = json.Marshal(e.ETag)
	return out
}

// projectSelect keeps only the named properties (always retaining odata.etag),
// returning the map unchanged when select is empty.
func projectSelect(m map[string]json.RawMessage, selectClause string) map[string]json.RawMessage {
	if selectClause == "" {
		return m
	}
	keep := map[string]struct{}{"odata.etag": {}}
	for _, f := range strings.Split(selectClause, ",") {
		keep[strings.TrimSpace(f)] = struct{}{}
	}
	out := make(map[string]json.RawMessage, len(keep))
	for k, v := range m {
		if _, ok := keep[k]; ok {
			out[k] = v
		}
	}
	return out
}

// --- $filter ---

// filterFunc reports whether an entity's properties satisfy a filter.
type filterFunc func(props map[string]json.RawMessage) bool

// parseFilter compiles an OData $filter expression. It supports comparisons
// (eq, ne, gt, ge, lt, le) over string, numeric and boolean literals, combined
// with and/or and parentheses. Unsupported syntax yields an error.
func parseFilter(expr string) (filterFunc, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return func(map[string]json.RawMessage) bool { return true }, nil
	}
	toks, err := tokenizeFilter(expr)
	if err != nil {
		return nil, err
	}
	p := &filterParser{tokens: toks}
	fn, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, errFilter
	}
	return fn, nil
}

// propValue decodes a property into a comparable Go value (string, float64 or
// bool).
func propValue(props map[string]json.RawMessage, key string) (any, bool) {
	raw, ok := props[key]
	if !ok {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return v, true
}

func compare(op string, left any, right any) bool {
	ln, lok := toNumber(left)
	rn, rok := toNumber(right)
	if lok && rok {
		return numericOp(op, ln, rn)
	}
	ls, lsok := left.(string)
	rs, rsok := right.(string)
	if lsok && rsok {
		return stringOp(op, ls, rs)
	}
	lb, lbok := left.(bool)
	rb, rbok := right.(bool)
	if lbok && rbok {
		switch op {
		case "eq":
			return lb == rb
		case "ne":
			return lb != rb
		}
	}
	return op == "ne"
}

func numericOp(op string, a, b float64) bool {
	switch op {
	case "eq":
		return a == b
	case "ne":
		return a != b
	case "gt":
		return a > b
	case "ge":
		return a >= b
	case "lt":
		return a < b
	case "le":
		return a <= b
	}
	return false
}

func stringOp(op string, a, b string) bool {
	switch op {
	case "eq":
		return a == b
	case "ne":
		return a != b
	case "gt":
		return a > b
	case "ge":
		return a >= b
	case "lt":
		return a < b
	case "le":
		return a <= b
	}
	return false
}

func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return fallback
}
