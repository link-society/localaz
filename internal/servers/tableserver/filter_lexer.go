package tableserver

import (
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
