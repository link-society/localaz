package tableserver

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// errFilter is returned for malformed or unsupported $filter expressions.
var errFilter = errors.New("tableserver: unsupported filter")

// token kinds for the $filter tokenizer.
type tokenKind int

const (
	tokIdent tokenKind = iota
	tokString
	tokNumber
	tokBool
	tokOp // eq ne gt ge lt le
	tokAnd
	tokOr
	tokLParen
	tokRParen
)

type token struct {
	kind tokenKind
	text string
}

var compareOps = map[string]struct{}{
	"eq": {}, "ne": {}, "gt": {}, "ge": {}, "lt": {}, "le": {},
}

// tokenizeFilter splits an OData filter expression into tokens, treating
// single-quoted strings (with ” escapes) as opaque literals.
func tokenizeFilter(expr string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(expr) {
		c := expr[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '(':
			toks = append(toks, token{kind: tokLParen})
			i++
		case c == ')':
			toks = append(toks, token{kind: tokRParen})
			i++
		case c == '\'':
			j := i + 1
			var sb strings.Builder
			for j < len(expr) {
				if expr[j] == '\'' {
					if j+1 < len(expr) && expr[j+1] == '\'' {
						sb.WriteByte('\'')
						j += 2
						continue
					}
					break
				}
				sb.WriteByte(expr[j])
				j++
			}
			if j >= len(expr) {
				return nil, errFilter
			}
			toks = append(toks, token{kind: tokString, text: sb.String()})
			i = j + 1
		default:
			j := i
			for j < len(expr) && expr[j] != ' ' && expr[j] != '\t' && expr[j] != '(' && expr[j] != ')' {
				j++
			}
			word := expr[i:j]
			toks = append(toks, classifyWord(word))
			i = j
		}
	}
	return toks, nil
}

func classifyWord(word string) token {
	lower := strings.ToLower(word)
	switch {
	case lower == "and":
		return token{kind: tokAnd}
	case lower == "or":
		return token{kind: tokOr}
	case lower == "true" || lower == "false":
		return token{kind: tokBool, text: lower}
	}
	if _, ok := compareOps[lower]; ok {
		return token{kind: tokOp, text: lower}
	}
	if _, err := strconv.ParseFloat(word, 64); err == nil {
		return token{kind: tokNumber, text: word}
	}
	return token{kind: tokIdent, text: word}
}

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
