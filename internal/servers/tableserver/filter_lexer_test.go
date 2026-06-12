package tableserver

import (
	"errors"
	"testing"
)

func TestTokenizeFilterKinds(t *testing.T) {
	toks, err := tokenizeFilter("Name eq 'a''b' and Age ge 10 or (Active eq true)")
	if err != nil {
		t.Fatalf("tokenizeFilter returned error: %v", err)
	}
	want := []token{
		{kind: tokIdent, text: "Name"},
		{kind: tokOp, text: "eq"},
		{kind: tokString, text: "a'b"}, // doubled quote unescaped to a single quote
		{kind: tokAnd},
		{kind: tokIdent, text: "Age"},
		{kind: tokOp, text: "ge"},
		{kind: tokNumber, text: "10"},
		{kind: tokOr},
		{kind: tokLParen},
		{kind: tokIdent, text: "Active"},
		{kind: tokOp, text: "eq"},
		{kind: tokBool, text: "true"},
		{kind: tokRParen},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %#v", len(toks), len(want), toks)
	}
	for i := range want {
		if toks[i].kind != want[i].kind || toks[i].text != want[i].text {
			t.Fatalf("token %d = %#v, want %#v", i, toks[i], want[i])
		}
	}
}

func TestTokenizeFilterUnterminatedString(t *testing.T) {
	if _, err := tokenizeFilter("Name eq 'unterminated"); !errors.Is(err, errFilter) {
		t.Fatalf("tokenizeFilter unterminated string error = %v, want errFilter", err)
	}
}

func TestClassifyWord(t *testing.T) {
	cases := []struct {
		word string
		want tokenKind
	}{
		{"and", tokAnd},
		{"AND", tokAnd},
		{"or", tokOr},
		{"eq", tokOp},
		{"GE", tokOp},
		{"true", tokBool},
		{"False", tokBool},
		{"42", tokNumber},
		{"-3.5", tokNumber},
		{"Name", tokIdent},
	}
	for _, tc := range cases {
		if got := classifyWord(tc.word); got.kind != tc.want {
			t.Fatalf("classifyWord(%q).kind = %v, want %v", tc.word, got.kind, tc.want)
		}
	}
}
