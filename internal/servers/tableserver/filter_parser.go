package tableserver

import (
	"encoding/json"
	"strconv"
)

// filterParser is a recursive-descent parser over the token stream.
type filterParser struct {
	tokens []token
	pos    int
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
		fn, err := p.parseOr()
		if err != nil {
			return nil, err
		}
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
			return op == "ne"
		}
		return compare(op, actual, literal)
	}, nil
}
