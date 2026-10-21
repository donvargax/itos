package corpus

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// --actual writes what a run did as YAML a case can hold: JSON as json: less its sentences, text
// otherwise, files as the run left them, and the case's folder, commits and version as their
// placeholders.
func TestWriteActuals(t *testing.T) {
	f, err := ReadFixture(writeFixture(t, `cases:
  - name: prints JSON
    argv: [work, show, --json]
    exit: 9
    files_after: { a.yaml: x, gone.txt: null }
  - name: prints text
    argv: [hook, commit-msg]
    exit: 0
  - name: cannot run
    argv: [x]
    exit: 0
`))
	if err != nil {
		t.Fatal(err)
	}
	sha := "0123456789abcdef0123456789abcdef01234567"
	s := &subst{dir: "/tmp/case", version: "8.1.0-dev.3.gabc", labels: map[string]string{"base": sha}}
	zero, one := 0, 1
	actuals := []Actual{
		{Fixture: f, Case: f.Cases[0], s: s, Outcome: &Outcome{
			Exit:   &one,
			Stdout: `{"schema":1,"message":"m","text":"t","id":"/tmp/case/x","at":"` + sha + `","list":[{"rule":"r","fix":"f"}],"n":2.5,"say":"\"b\" {c}, d"}`,
			Stderr: "",
			files:  map[string]*written{"a.yaml": {text: "built: 0123456\nby: 8.1.0-dev.3.gabc\n"}},
		}},
		{Fixture: f, Case: f.Cases[1], s: s, Outcome: &Outcome{Exit: &zero, Stdout: "ok\n", Stderr: "line one\nline two\n"}},
		{Fixture: f, Case: f.Cases[2], Err: errors.New("could not build")},
	}
	var b strings.Builder
	if err := WriteActuals(&b, actuals); err != nil {
		t.Fatal(err)
	}
	want := `- file: ask.yaml
  name: prints JSON
  argv: [work, show, --json]
  exit: 1
  stderr: ""
  json: { schema: 1, id: "{{dir}}/x", at: "{{sha.base}}", list: [{ rule: r }], n: 2.5, say: "\"b\" {c}, d" }
  messages: [{ message: m }, { at: "list[0]", rule: r, fix: f }]
  files_after:
    a.yaml: |
      built: {{short.base}}
      by: {{version}}
    gone.txt: null
- file: ask.yaml
  name: prints text
  argv: [hook, commit-msg]
  exit: 0
  stdout: |
    ok
  stderr: |
    line one
    line two
- file: ask.yaml
  name: cannot run
  argv: [x]
  error: could not build
`
	if b.String() != want {
		t.Errorf("WriteActuals wrote\n%s\nwant\n%s", b.String(), want)
	}
}

// What --actual writes, pasted as a case's expectations less its messages and its stderr, which
// are for reading (a case pins no stderr but the empty one), passes.
func TestActualsPasteAsPassingExpectations(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	s := &subst{dir: "/tmp/case", version: "8.1.0", labels: map[string]string{"base": sha}}
	exit := 3
	out := &Outcome{
		Exit:   &exit,
		Stdout: `{"schema":1,"message":"m","path":"/tmp/case/a b","items":[{"id":"T-1","at":"` + sha + `","fix":"f"}],"say":"a \"b\" {c}, d: e","n":"1","none":{},"long":"` + strings.Repeat("word ", 30) + `"}`,
		Stderr: "warning: /tmp/case/x\n  indented\n",
		files:  map[string]*written{"notes.md": {text: "# 8.1.0\n\nby " + sha[:7] + "\n"}},
	}
	f, err := ReadFixture(writeFixture(t, "cases:\n  - { name: a, argv: [x], exit: 0, files_after: { notes.md: x } }\n"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := WriteActuals(&b, []Actual{{Fixture: f, Case: f.Cases[0], Outcome: out, s: s}}); err != nil {
		t.Fatal(err)
	}
	pasted := strings.Replace(b.String(), "- file: ask.yaml\n  name: a\n", "- name: a\n", 1)
	pasted = regexp.MustCompile(`(?m)^  (?:messages|stderr):.*\n(?:    .*\n)*`).ReplaceAllString(pasted, "")
	g, err := ReadFixture(writeFixture(t, "cases:\n"+pasted))
	if err != nil {
		t.Fatalf("%v\n%s", err, b.String())
	}
	if problems := compare(g.Cases[0], out, s, false); len(problems) > 0 {
		t.Errorf("the pasted case fails:\n%s\n%s", strings.Join(problems, "\n"), b.String())
	}
}
