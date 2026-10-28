package features

import (
	"slices"
	"testing"
)

func TestShellWords(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
		err  bool
	}{
		{name: "plain words", line: "work show  T-1\tx", want: []string{"work", "show", "T-1", "x"}},
		{name: "empty line", line: "", want: nil},
		{name: "single quotes keep spaces", line: "commit -m 'a b  c'", want: []string{"commit", "-m", "a b  c"}},
		{name: "double quotes keep spaces", line: `commit -m "a b  c"`, want: []string{"commit", "-m", "a b  c"}},
		{name: "double quotes holding an apostrophe", line: `--reply "it's fixed"`, want: []string{"--reply", "it's fixed"}},
		{name: "single quotes holding a double quote", line: `--reply 'say "hi" now'`, want: []string{"--reply", `say "hi" now`}},
		{name: "quotes inside a word", line: `--title="it's"'"x"'`, want: []string{`--title=it's"x"`}},
		{name: "empty quotes are a word", line: `a '' "" b`, want: []string{"a", "", "", "b"}},
		{name: "backslash is plain", line: `--dir C:\a\b 'x\' "y\"`, want: []string{"--dir", `C:\a\b`, `x\`, `y\`}},
		{name: "unterminated single quote", line: "a 'b", err: true},
		{name: "unterminated double quote", line: `a "b`, err: true},
		{name: "apostrophe unterminated by a double quote", line: `a "b'`, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shellWords(tt.line)
			if (err != nil) != tt.err {
				t.Fatalf("shellWords(%q) error = %v, want error %v", tt.line, err, tt.err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("shellWords(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}
