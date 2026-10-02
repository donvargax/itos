package glob

import "testing"

// The globs' meaning, and the two places RE2 needed a different construction
// to keep it: JavaScript's dot, and a leading `?`.
func TestMatch(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"*.md", "README.md", true},
		{"*.md", "docs/a.md", false},
		{"docs/**", "docs/a/b.md", true},
		{"docs/**", "docs", false},
		{"**/*.md", "a.md", true},
		{"**/*.md", "x/y/b.md", true},
		{"*.{json,yaml}", "a.yaml", true},
		{"*.{json,yaml}", "c.yml", false},
		{"src/*.ts", "xsrc/a.ts", false},
		{"a+b(c).md", "a+b(c).md", true},
		{"a+b(c).md", "aab(c).md", false},
		// A `*` after a `.` is left as the regular expression's: any dots.
		{"a.*", "a...", true},
		{"a.*", "a.b", false},
		// JavaScript's dot stops at any line terminator, not only \n.
		{"**", "a\rb", false},
		{"**", "a b", false},
		{"**", "a/é", true},
	}
	for _, c := range cases {
		got, err := Match(c.glob, c.path)
		if err != nil || got != c.want {
			t.Errorf("Match(%q, %q) = %v, %v; want %v", c.glob, c.path, got, err, c.want)
		}
	}
}

// A glob JavaScript refuses is an error worded as V8 words it, RE2's `^?`
// included, and MatchesAny stops at the first match, before a bad glob.
func TestErrors(t *testing.T) {
	for glob, want := range map[string]string{
		"?a":    "Invalid regular expression: /^?a$/: Nothing to repeat",
		"{a,?}": "Invalid regular expression: /^(?:a|?)$/: Nothing to repeat",
		"*??":   "Invalid regular expression: /^[^/]*??$/: Nothing to repeat",
	} {
		if _, err := Compile(glob); err == nil || err.Error() != want {
			t.Errorf("Compile(%q) = %v; want %s", glob, err, want)
		}
	}
	if _, err := Compile("**?"); err != nil {
		t.Errorf("Compile(%q), a lazy repeat in both, = %v", "**?", err)
	}
	if ok, err := MatchesAny("a.md", []string{"*.md", "?a"}); !ok || err != nil {
		t.Errorf("MatchesAny past a match = %v, %v", ok, err)
	}
	if _, err := MatchesAny("b", []string{"a", "?a"}); err == nil {
		t.Error("MatchesAny reaching a bad glob is no error")
	}
}
