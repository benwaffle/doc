package main

import "testing"

func TestNextToken(t *testing.T) {
	tests := []struct {
		line  string
		token string
		rest  string
	}{
		{"word", "word", ""},
		{"a b c", "a", "b c"},
		{".SH NAME", ".SH", "NAME"},

		{".Fl t Ns Ar man ,", ".Fl", "t Ns Ar man ,"},
		{"t Ns Ar man ,", "t", "Ns Ar man ,"},
		{"Ns Ar man ,", "Ns", "Ar man ,"},
		{"Ar man ,", "Ar", "man ,"},
		{"man ,", "man", ","},

		{`normal\fBbold`, "normal", `\fBbold`},
		{`"quoted words" are handled`, "quoted words", "are handled"},

		{`hel\fBlo\fR`, "hel", `\fBlo\fR`},
		{`\fBhello`, `\fB`, "hello"},
		{`\-\- ok`, `--`, `ok`},
		{`"\-b\fIn\fP or \-\-buffers=\fIn\fP"`, `-b\fIn\fP or --buffers=\fIn\fP`, ""},
	}

	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			token, rest := nextToken(test.line)
			if token != test.token {
				t.Errorf("nextToken(%q) = [%q, %q] wanted token %q", test.line, token, rest, test.token)
			}
			if rest != test.rest {
				t.Errorf("nextToken(%q) = [%q, %q] wanted rest %q", test.line, token, rest, test.rest)
			}
		})
	}
}
