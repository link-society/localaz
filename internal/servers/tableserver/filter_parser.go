package tableserver

import (
	"encoding/json"
	"strconv"
)

// maxFilterDepth bounds how deeply parsePrimary may recurse through nested
// parentheses. Without it, attacker-supplied input with thousands of nested
// '(' overflows the goroutine stack (an unrecoverable fatal error).
const maxFilterDepth = 64

// filterParser is a recursive-descent parser over the token stream.
type filterParser struct {
	tokens []token
	pos    int
	depth  int
}

func (p *filterParser) peek() (token, bool) {
	if p.pos >= len(p.tokens) {
		return token{}, false
	}
	return p.tokens[p.pos], true
}

func (p *filterParser) next() (token, bool) {
	t, ok := p.peek()
	if ok {
		p.pos++
	}
	return t, ok
}

func (p *filterParser) parseOr() (filterFunc, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.kind != tokOr {
			return left, nil
		}
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l, r := left, right
		left = func(props map[string]json.RawMessage) bool { return l(props) || r(props) }
	}
}

func (p *filterParser) parseAnd() (filterFunc, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.kind != tokAnd {
			return left, nil
		}
		p.pos++
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		l, r := left, right
		left = func(props map[string]json.RawMessage) bool { return l(props) && r(props) }
	}
}

func (p *filterParser) parsePrimary() (filterFunc, error) {
	t, ok := p.next()
	if !ok {
		return nil, errFilter
	}
	if t.kind == tokLParen {
		p.depth++
		if p.depth > maxFilterDepth {
			return nil, errFilter
		}
		fn, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		p.depth--
		closing, ok := p.next()
		if !ok || closing.kind != tokRParen {
			return nil, errFilter
		}
		return fn, nil
	}
	if t.kind != tokIdent {
		return nil, errFilter
	}
	opTok, ok := p.next()
	if !ok || opTok.kind != tokOp {
		return nil, errFilter
	}
	valTok, ok := p.next()
	if !ok {
		return nil, errFilter
	}
	return p.comparison(t.text, opTok.text, valTok)
}

func (p *filterParser) comparison(field, op string, valTok token) (filterFunc, error) {
	var literal any
	switch valTok.kind {
	case tokString:
		literal = valTok.text
	case tokNumber:
		f, err := strconv.ParseFloat(valTok.text, 64)
		if err != nil {
			return nil, errFilter
		}
		literal = f
	case tokBool:
		literal = valTok.text == "true"
	default:
		return nil, errFilter
	}
	return func(props map[string]json.RawMessage) bool {
		actual, ok := propValue(props, field)
		if !ok {
			// Absent property: a $filter selects only entities that have the
			// property and satisfy it, so no operator matches (including ne).
			return false
		}
		return compare(op, actual, literal)
	}, nil
}
