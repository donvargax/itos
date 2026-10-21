package corpus

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// v8.0.0's ask.yaml, as it stands in that release's tree: a question mark in a flow sequence's
// plain item, which YAML 1.2 reads and go.yaml.in/yaml/v3 alone refuses.
const v8Ask = `cases:
  - name: decision refuses a decision that is neither a record's number nor none, and leaves the file as it is
    argv: [decision, add, Which way?]
    files:
      asks.yaml: &baddecision "questions:\n  - { id: q-1, question: a, answer: b, decision: \"0003\" }\n"
    exit: 2
    files_after:
      asks.yaml: *baddecision
  - name: decision add with an item
    argv: [decision, add, "Which way?", --item, x]
    exit: 0
`

func writeFixture(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ask.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadsAReleasesCorpusAsItStands(t *testing.T) {
	f, err := ReadFixture(writeFixture(t, v8Ask))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Cases[0].Argv; !slices.Equal(got, []string{"decision", "add", "Which way?"}) {
		t.Errorf("argv %q", got)
	}
	if got := f.Cases[1].Argv; !slices.Equal(got, []string{"decision", "add", "Which way?", "--item", "x"}) {
		t.Errorf("argv %q", got)
	}
	if got := f.Cases[0].filesAfter; len(got) != 1 || !strings.Contains(got[0].text, `decision: \"0003\"`) && !strings.Contains(got[0].text, `decision: "0003"`) {
		t.Errorf("the alias should read as its anchor: %+v", got)
	}
	// A data file a run writes is read strictly: only a fixture is read as a release wrote it.
	if _, ok := yamlData("argv: [decision, add, Which way?]\n"); ok {
		t.Errorf("a written file go.yaml.in/yaml/v3 refuses should not be read as YAML")
	}
}

func TestQuoteFlowQuestions(t *testing.T) {
	for in, want := range map[string]string{
		"argv: [decision, add, Which way?]\n":          "argv: [decision, add, \"Which way?\"]\n",
		"argv: [a, b?c, \"d?\"]\n":                     "argv: [a, b?c, \"d?\"]\n", // a quote: left alone
		"argv: [x, why?, y]\n  - [z, w?]\n":            "argv: [x, \"why?\", y]\n  - [z, \"w?\"]\n",
		"argv: [plain, words]\n":                       "argv: [plain, words]\n",
		"argv: [? explicit, key?]   # what is this?\n": "argv: [? explicit, \"key?\"]   # what is this?\n",
	} {
		got, _ := quoteFlowQuestions([]byte(in))
		if string(got) != want {
			t.Errorf("quoteFlowQuestions(%q) = %q, want %q", in, got, want)
		}
	}
}

// A fixture's values are YAML 1.2 core types, as the TypeScript runner's parser read them.
func TestCoreSchema(t *testing.T) {
	v, err := parseYAML([]byte(`
version: 0.1.0
float: 1.0
int: 3
octal: 0o17
leading: 0755
hex: 0x1f
date: 2001-12-14
yes: yes
on: true
null: ~
empty:
quoted: "3"
tagged: !!str 3
block: |
  text
merge: { <<: x }
`), false)
	if err != nil {
		t.Fatal(err)
	}
	m := v.(*mapping)
	for key, want := range map[string]any{
		"version": "0.1.0", "float": 1.0, "int": 3.0, "octal": 15.0, "leading": 755.0, "hex": 31.0,
		"date": "2001-12-14", "yes": "yes", "on": true, "null": nil, "empty": nil, "quoted": "3",
		"tagged": "3", "block": "text\n",
	} {
		if got := m.vals[key]; got != want {
			t.Errorf("%s: %#v, want %#v", key, got, want)
		}
	}
	if got := m.vals["merge"].(*mapping).vals["<<"]; got != "x" {
		t.Errorf("a merge key should be a key like any other: %#v", got)
	}
}

func TestFixtureErrors(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"cases: 3\n", "has no list of cases"},
		{"cases: []\nextra: 1\n", "unknown key extra"},
		{"cases:\n  - { name: a, argv: [x], exit: 0, color: red }\n", "case 1 (a) unknown key color"},
		{"cases:\n  - { name: a, argv: [x], exit: 0 }\n  - { name: a, argv: [x], exit: 0 }\n", "case 2 (a) uses a name twice"},
		{"cases:\n  - { name: a, argv: [x, 1], exit: 0 }\n", "case 1 (a) has an argv that is not a list of strings"},
		{"cases:\n  - { name: a, argv: [x] }\n", "case 1 (a) has no exit"},
		{"cases:\n  - { name: a, argv: [x], exit: 0, git: [{ push: x }] }\n", "a git step has an unknown key push"},
		{"cases:\n  - [x]\n", "case 1 is not a mapping"},
		{"cases:\n  - { name: a, argv: [x], exit: 0, stdout_nonempty: false }\n", "case 1 (a) has a stdout_nonempty that is not true"},
	} {
		_, err := ReadFixture(writeFixture(t, c.text))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: said %v, want %q", c.text, err, c.want)
		}
	}
}

func TestDefaultsUnderCases(t *testing.T) {
	f, err := ReadFixture(writeFixture(t, `files: { a: "1", b: "2" }
env: { X: top, Y: top }
git: [{ commit: first }]
github: { refuse: true }
cases:
  - { name: own, argv: [x], exit: 0, files: { b: null, c: "3" }, env: { Y: own }, git: [], github: null }
  - { name: inherited, argv: [x], exit: 0 }
`))
	if err != nil {
		t.Fatal(err)
	}
	own, inherited := f.Cases[0], f.Cases[1]
	var paths []string
	for _, e := range own.files {
		paths = append(paths, e.path+"="+e.text)
	}
	if !slices.Equal(paths, []string{"a=1", "b=", "c=3"}) || !own.files[1].null {
		t.Errorf("a case's files laid over the file's, in order: %q", paths)
	}
	if len(own.env) != 2 || *own.env[1].value != "own" || *own.env[0].value != "top" {
		t.Errorf("env %+v", own.env)
	}
	if !own.hasGit || len(own.git) != 0 || own.github != nil {
		t.Errorf("a case's own git and github: %+v %+v", own.git, own.github)
	}
	if !inherited.hasGit || len(inherited.git) != 1 || inherited.github == nil {
		t.Errorf("a case's inherited git and github: %+v %+v", inherited.git, inherited.github)
	}
}

func TestMachine(t *testing.T) {
	file, err := ReadFile(writeFixture(t, `cases:
  - { name: a, argv: [x], exit: 0, stdout: "x\n", json: { ok: true, message: m, list: [{ fix: f, rule: r, text: t }] } }
  - { name: b, argv: [x], exit: 0, stderr_has: y }
  - { name: c, argv: [x], exit: 0, stdout_nonempty: true, json_has: { text: t, message: "<any>" } }
`))
	if err != nil {
		t.Fatal(err)
	}
	cases, plainOut, keys := file.Machine()
	if cases != 3 || plainOut != 2 || keys != 3 {
		t.Errorf("Machine said %d cases, %d plain, %d keys; want 3, 2, 3", cases, plainOut, keys)
	}
	f, err := file.Fixture()
	if err != nil {
		t.Fatal(err)
	}
	if f.Cases[0].stdout != nil || f.Cases[1].stderrHas != nil || show(f.Cases[0].json) != `{"ok":true,"list":[{"rule":"r"}]}` ||
		!f.Cases[2].stdoutNonempty || show(f.Cases[2].jsonHas) != `{"text":"t","message":"<any>"}` {
		t.Errorf("Machine left %v %v %s, %v %s", f.Cases[0].stdout, f.Cases[1].stderrHas, show(f.Cases[0].json), f.Cases[2].stdoutNonempty, show(f.Cases[2].jsonHas))
	}
	if !slices.Equal(file.Names(), []string{"a", "b", "c"}) {
		t.Errorf("names %q", file.Names())
	}
}

func TestNoMaintenanceAppends(t *testing.T) {
	env := map[string]string{"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.editor", "GIT_CONFIG_VALUE_0": "true"}
	noMaintenance(env)
	want := map[string]string{
		"GIT_CONFIG_COUNT": "3", "GIT_CONFIG_KEY_0": "core.editor", "GIT_CONFIG_VALUE_0": "true",
		"GIT_CONFIG_KEY_1": "maintenance.auto", "GIT_CONFIG_VALUE_1": "false",
		"GIT_CONFIG_KEY_2": "gc.auto", "GIT_CONFIG_VALUE_2": "0",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s=%q, want %q", k, env[k], v)
		}
	}
}

func TestJudging(t *testing.T) {
	if got := diff("a\nb\nc\n", "a\nx\nc\n"); !slices.Equal(got, []string{"  a", "- b", "+ x", "  c"}) {
		t.Errorf("diff %q", got)
	}
	if !holdsLines("a\nc\n", "a\nb\nc\nd\n") || holdsLines("c\na\n", "a\nb\nc\n") {
		t.Errorf("holdsLines should hold lines in order, whatever lies between them")
	}
	if got := markdownContent("<!-- top -->\n# T\n\nready <!-- now -->\n"); got != "# T\n\nready \n" {
		t.Errorf("markdownContent %q", got)
	}
	want, _ := parseYAML([]byte("{ a: 1, b: [x, { c: true }] }"), false)
	for actual, lacks := range map[string]string{
		`{"b":["x",{"c":true,"d":1}],"a":1,"e":2}`: "",
		`{"a":1}`:                        `.b: expected ["x",{"c":true}], got none`,
		`{"a":2,"b":["x",{"c":true}]}`:   ".a: expected 1, got 2",
		`{"a":1,"b":["x"]}`:              `.b: expected 2 elements, got 1: ["x"]`,
		`{"a":1,"b":["x",{"c":"true"}]}`: `.b[1].c: expected true, got "true"`,
	} {
		got, err := parseJSON(actual)
		if err != nil {
			t.Fatal(err)
		}
		if said := strings.Join(jsonLacks(want, got, ""), "\n"); said != lacks {
			t.Errorf("jsonLacks of %s said %q, want %q", actual, said, lacks)
		}
	}
	if _, err := parseJSON(`{"a":1} {"b":2}`); err == nil {
		t.Errorf("two JSON values should not read as one")
	}
}

// judged is what compare says of a case's YAML text run to exit 0 with stdout.
func judged(t *testing.T, kase, stdout string, additive bool) string {
	t.Helper()
	f, err := ReadFixture(writeFixture(t, "cases:\n  - "+kase+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	exit := 0
	return strings.Join(compare(f.Cases[0], &Outcome{Exit: &exit, Stdout: stdout}, &subst{}, additive), "\n")
}

// json: compares the whole object less every key named message, fix or text, at any depth
// (ADR-0035, ADR-0054): reworded sentences pass, any other key changed, added or removed fails.
func TestJSONLeavesOutSentences(t *testing.T) {
	const kase = `{ name: a, argv: [x], exit: 0, json: { rule: r, message: old words, problems: [{ rule: p, fix: old fix, detail: { text: old prose, line: 3 } }] } }`
	for stdout, want := range map[string]string{
		`{"rule":"r","message":"new words","problems":[{"rule":"p","fix":"new fix","detail":{"text":"new prose","line":3}}]}`: "",
		`{"rule":"r","problems":[{"rule":"p","detail":{"line":3}}]}`:                                                          "",
		`{"rule":"r","problems":[{"rule":"p","detail":{"line":3},"text":"added prose"}],"fix":"added"}`:                       "",
		`{"rule":"r","message":"new words","problems":[{"rule":"p","detail":{"line":4}}]}`:                                    "line",
		`{"rule":"r","problems":[{"rule":"p","detail":{"line":3}}],"extra":1}`:                                                "extra",
		`{"problems":[{"rule":"p","detail":{"line":3}}]}`:                                                                     "rule",
	} {
		got := judged(t, kase, stdout, false)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("json: of %s said %q, want it to name %q", stdout, got, want)
		}
		if strings.Contains(got, "words") || strings.Contains(got, "prose") {
			t.Errorf("json: of %s showed an unjudged key: %q", stdout, got)
		}
	}
	if got := judged(t, kase, `{"rule":"r","problems":[{"rule":"p","detail":{"line":3,"more":1}}],"added":true}`, true); got != "" {
		t.Errorf("additive json: should pass keys added, sentences aside: %q", got)
	}
}

// json_has holds a subset: the keys it names with their values, "<any>" for any value, an
// array's elements each among the output's, in any order.
func TestJSONHas(t *testing.T) {
	const kase = `{ name: a, argv: [x], exit: 0, json_has: { ok: true, link: "<any>", items: [{ id: b }, c], problems: [{ rule: r, message: "<any>" }] } }`
	const holds = `{"ok":true,"link":"/somewhere/bin/git","extra":{"x":1},"items":["c",{"id":"a"},{"id":"b","more":2}],"problems":[{"rule":"q"},{"rule":"r","message":"anything"}]}`
	if got := judged(t, kase, holds, false); got != "" {
		t.Errorf("json_has should hold a subset: %q", got)
	}
	if got := judged(t, kase, holds, true); got != "" {
		t.Errorf("json_has should judge alike in additive: %q", got)
	}
	for stdout, want := range map[string]string{
		`{"ok":false,"link":null,"items":[{"id":"b"},"c"],"problems":[{"rule":"r","message":""}]}`: ".ok: expected true, got false",
		`{"ok":true,"items":[{"id":"b"},"c"],"problems":[{"rule":"r","message":""}]}`:              `.link: expected "<any>", got none`,
		`{"ok":true,"link":1,"items":[{"id":"a"},"c"],"problems":[{"rule":"r","message":""}]}`:     `.items[0]: expected {"id":"b"} among [{"id":"a"},"c"]`,
		`{"ok":true,"link":1,"items":[{"id":"b"},"c"],"problems":[{"rule":"r"}]}`:                  `.problems[0]: expected {"rule":"r","message":"<any>"} among [{"rule":"r"}]`,
		`{"ok":true,"link":1,"items":{"id":"b"},"problems":[{"rule":"r","message":""}]}`:           `.items: expected [{"id":"b"},"c"], got {"id":"b"}`,
		`not json`: "stdout is not JSON",
	} {
		if got := judged(t, kase, stdout, false); !strings.Contains(got, want) {
			t.Errorf("json_has of %s said %q, want it to name %q", stdout, got, want)
		}
	}
	// It judges every key it names, a sentence's key too.
	if got := judged(t, `{ name: a, argv: [x], exit: 0, json_has: { message: old words } }`, `{"message":"new words"}`, false); got == "" {
		t.Errorf("json_has should judge a message it names")
	}
}

// stdout_nonempty: true wants stdout to hold something besides whitespace.
func TestStdoutNonempty(t *testing.T) {
	const kase = `{ name: a, argv: [x], exit: 0, stdout_nonempty: true }`
	if got := judged(t, kase, "usage: itos\n", false); got != "" {
		t.Errorf("a stdout with text should pass: %q", got)
	}
	for _, stdout := range []string{"", " \n\t\n"} {
		if got := judged(t, kase, stdout, false); !strings.Contains(got, "stdout is empty") {
			t.Errorf("stdout %q should fail: %q", stdout, got)
		}
	}
}
