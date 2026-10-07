package message

import (
	"reflect"
	"strings"
	"testing"
)

// The built-in lint's verdicts, each as commitlint with config-conventional
// gives it for the same message (checked against commitlint 21 when
// written): the rules named, errors before warnings, in its order.
func TestHeaderProblems(t *testing.T) {
	cfg := load(t, "version: 1\ncommits:\n  header_lint: { use: builtin }\n")
	long := strings.Repeat("a", 101)
	for _, c := range []struct {
		message, comment string
		want             []string
	}{
		{"update things", "", []string{"subject-empty", "type-empty"}},
		{"Feat: Add Thing.", "", []string{"subject-case", "subject-full-stop", "type-case", "type-enum"}},
		{" feat: x ", "", []string{"header-trim", "subject-empty", "type-empty"}},
		{"feat: ", "", []string{"header-trim", "subject-empty"}},
		{"feat:", "", []string{"subject-empty", "type-empty"}},
		{"feat!: x", "", nil},
		{"feat(a): x", "", nil},
		{"1fix: x", "", []string{"type-enum"}},
		{"feat: `Eslint` config", "", nil},
		{"feat: x...", "", nil},
		{"chore: " + strings.Repeat("a", 94), "", []string{"header-max-length"}},
		{"chore: " + strings.Repeat("a", 93), "", nil},
		{"chore: x\nbody", "", []string{"body-leading-blank"}},
		{"chore: x\n\nbody\nTask: T-1", "", []string{"footer-leading-blank"}},
		{"chore: x\n\n" + long + "\n\nTask: " + long, "", []string{"body-max-line-length", "footer-max-line-length"}},
		{"chore: x\n\nsee https://example.com/" + long, "", nil},
		{"chore: x\n\nBREAKING CHANGE: " + long, "", []string{"footer-max-line-length"}},
		{"Merge branch 'a' into main", "", nil},
		{"feat: x\n\nbody\nMerge branch 'a'", "", nil},
		{"Revert \"feat: x\"", "", nil},
		{"fixup! x", "", nil},
		{"v1.2.3", "", nil},
		{"chore(release): 1.2.3 [skip ci]", "", nil},
		{"\n", "", []string{"subject-empty", "type-empty"}},
		// commitlint --edit: the comment lines out, a line break added.
		{"feat: x\n\n# " + long + "\n", "#", nil},
		{"feat: x\n\n# " + long + "\n", "", []string{"body-max-line-length"}},
		{"feat: x\n\nbody\n# ------------------------ >8 ------------------------\n" + long + "\n", "#", nil},
		{"# only a comment\n", "#", nil},
		{"\n", "#", nil},
		{"  \n\n", "#", nil},
		{"# a\n\n\n# b\n", "#", []string{"header-max-length", "subject-empty", "type-empty"}},
	} {
		found, err := HeaderProblems(cfg, c.message, c.comment)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range found {
			got = append(got, p.Rule)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: %v, want %v", c.message, got, c.want)
		}
	}
}

// Each problem in commitlint's words, with its level; commits.types is the
// type list when the config has one.
func TestHeaderWords(t *testing.T) {
	cfg := load(t, "version: 1\ncommits:\n  types: [feat, docs]\n  header_lint: { use: builtin }\n")
	found, err := HeaderProblems(cfg, "Chore: X\nbody\n", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range found {
		got = append(got, p.Level+" "+p.Message)
	}
	want := []string{
		"error subject must not be sentence-case, start-case, pascal-case, upper-case",
		"error type must be lower-case",
		"error type must be one of [feat, docs]",
		"warning body must have leading blank line",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	found, _ = HeaderProblems(cfg, "docs: "+strings.Repeat("a", 95)+"\n", "")
	if len(found) != 1 || found[0].Message != "header must not be longer than 100 characters, current length is 101" {
		t.Errorf("header-max-length %+v", found)
	}
}

// The cases subject-case names, as es-toolkit's case functions find them.
func TestSubjectCases(t *testing.T) {
	for subject, want := range map[string][]string{
		"add a thing":  nil,
		"Add a thing":  {"sentence-case"},
		"Add Thing":    {"sentence-case", "start-case"},
		"AddThing":     {"sentence-case", "pascal-case"},
		"Tidy":         {"sentence-case", "start-case", "pascal-case"},
		"ADD THING":    {"sentence-case", "start-case", "upper-case"},
		"Élan Vital":   {"sentence-case"},
		"Hello 2nd":    {"sentence-case", "start-case"},
		"`Eslint` fix": nil,
		"1st thing":    nil,
		"ß":            nil,
		"𐐨x":           {"sentence-case", "start-case", "pascal-case"},
	} {
		if got := subjectCases(subject); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %v, want %v", subject, got, want)
		}
	}
}

// The message the hook judges is the lint's reading of its file (bug 36):
// blank and comment lines above the header, and everything from the scissor
// line down, are out, so its type is the header's.
func TestCleaned(t *testing.T) {
	for _, c := range []struct{ raw, comment, want string }{
		{"\nchore: tidy\n\nTask: T-1\n", "#", "chore: tidy\n\nTask: T-1"},
		{"# Please enter\nchore: tidy\n", "#", "chore: tidy"},
		{"; note\nfix: it\n# kept\n", ";", "fix: it\n# kept"},
		{"feat: a\n# ------------------------ >8 ------------------------\nTask: T-1\n", "#", "feat: a"},
		{"# only\n", "#", ""},
	} {
		got := Cleaned(c.raw, c.comment)
		if got != c.want {
			t.Errorf("Cleaned(%q, %q) = %q, want %q", c.raw, c.comment, got, c.want)
		}
		if typ := Type(got); c.want != "" && typ != strings.SplitN(c.want, ":", 2)[0] {
			t.Errorf("Type(Cleaned(%q)) = %q", c.raw, typ)
		}
	}
}
