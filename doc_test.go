package main

import (
	"slices"
	"testing"
)

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

func TestMerge(t *testing.T) {
	page := manPage{
		Sections: []section{
			{
				Contents: []Span{
					textSpan{Typ: tagPlain, Text: "hello"},
					textSpan{Typ: tagPlain, Text: "world"},
					textSpan{Typ: tagPlain, Text: "man"},
					textSpan{Typ: tagBold, Text: "bold"},
				},
			},
		},
	}
	page.mergeSpans()
	expected := []Span{
		textSpan{Typ: tagPlain, Text: "hello world man"},
		textSpan{Typ: tagBold, Text: "bold"},
	}
	if !slices.Equal(page.Sections[0].Contents, expected) {
		t.Errorf("%+v did not equal %+v", page.Sections[0].Contents, expected)
	}
}

func TestMergeSpansParagraphs(t *testing.T) {
	manText := `.Sh DESCRIPTION
First paragraph.
.Pp
Second paragraph.`

	parser := parser{}
	page := parser.parseMdoc(manText)
	page.mergeSpans()

	expected := []Span{
		textSpan{Typ: tagPlain, Text: "First paragraph.", NoSpace: false},
		textSpan{Typ: tagPlain, Text: "\n\n", NoSpace: true},
		textSpan{Typ: tagPlain, Text: "Second paragraph.", NoSpace: false},
	}

	if !slices.Equal(page.Sections[0].Contents, expected) {
		t.Errorf("Got: %+v\nExpected: %+v", page.Sections[0].Contents, expected)
	}
}
