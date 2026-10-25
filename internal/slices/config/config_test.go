package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	itosconfig "github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are the config command's ports over the fakes: a repository, the
// working tree's files and the flags of itos task.
type ports struct {
	*vcsfake.VCS
	trees treefake.Trees
}

func (p ports) Trees(context.Context) treeport.Trees { return p.trees }

func (p ports) Worktree(ctx context.Context) treeport.Source {
	src, _ := p.trees.At(ctx, "worktree")
	return src
}

func (ports) TaskFlags(context.Context) []string {
	return []string{"--config", "--root", "--json", "--skip"}
}

// configText names the ledger and the people.
const configText = "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { people: { source: yaml, file: people.yaml } }\n"

// sound is a project config check finds nothing in.
func sound() map[string]string {
	return map[string]string{
		"itos.yaml":             configText,
		"tasks/phase-1.yaml":    "[]\n",
		"tasks/work-items.yaml": "phases: { 1: null }\nitems: [{ id: p1-a, title: A, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }]\n",
		"people.yaml":           "- a\n",
	}
}

// with is files, each of more set over them, "" removing one.
func with(files map[string]string, more map[string]string) map[string]string {
	all := maps.Clone(files)
	for p, text := range more {
		if text == "" {
			delete(all, p)
			continue
		}
		all[p] = text
	}
	return all
}

// over is the ports of a working tree holding files, from an empty folder
// with no ITOS_CONFIG, in no repository.
func over(t *testing.T, files map[string]string) ports {
	t.Helper()
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(t.TempDir())
	return ports{VCS: &vcsfake.VCS{}, trees: treefake.Trees{Files: map[string]map[string]string{"worktree": files}}}
}

// call is the command's line run over p: its exit, stdout, stderr and
// error; a --json or --quiet first sets the global flag.
func call(t *testing.T, p ports, args ...string) (int, string, string, error) {
	t.Helper()
	asJSON, quiet := false, false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "--quiet") {
		asJSON, quiet = asJSON || args[0] == "--json", quiet || args[0] == "--quiet"
		args = args[1:]
	}
	c, err := Parse(args, "", asJSON, quiet, &tests.Warnings{})
	if err != nil {
		t.Fatalf("Parse(%q) = %v", args, err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// compact is a --json object on one line.
func compact(text string) string {
	var b bytes.Buffer
	if json.Compact(&b, []byte(text)) != nil {
		return text
	}
	return b.String()
}

// usageOf is whether err is the usage error saying text.
func usageOf(err error, text string) bool {
	var u out.UsageError
	return errors.As(err, &u) && err.Error() == text
}

// Parse refuses a get without one key and an unknown subcommand, and reads
// check's --print-defaults and --ledger, the ledger file from where itos was
// started.
func TestParse(t *testing.T) {
	for line, want := range map[string]string{
		"get":       "config get needs one <key>, a dotted path such as hooks.bin",
		"get a b":   "config get needs one <key>, a dotted path such as hooks.bin",
		"":          "unknown command: config",
		"nope":      "unknown command: config nope",
		"nope --ok": "unknown command: config nope",
	} {
		if _, err := Parse(strings.Fields(line), "", false, false, nil); !usageOf(err, want) {
			t.Errorf("Parse(%q) = %v, want %q", line, err, want)
		}
	}
	if _, err := Parse([]string{"get", ""}, "", false, false, nil); !usageOf(err, "config get needs one <key>, a dotted path such as hooks.bin") {
		t.Errorf(`Parse(get "") = %v, want the key needed`, err)
	}
	ws := &tests.Warnings{}
	c, err := Parse([]string{"get", "hooks.bin"}, "sub", true, true, ws)
	if err != nil || c.Query != (Query{Sub: "get", Key: "hooks.bin"}) || !c.JSON || !c.Quiet || c.Tests != ws {
		t.Errorf("Parse(get hooks.bin) = %+v, %v", c, err)
	}
	abs, _ := filepath.Abs("x.yaml")
	for _, tc := range []struct {
		args   []string
		origin string
		want   Query
	}{
		{[]string{"check"}, "sub", Query{Sub: "check"}},
		{[]string{"check", "--print-defaults"}, "", Query{Sub: "check", Defaults: true}},
		{[]string{"check", "--ledger", "x.yaml"}, "", Query{Sub: "check", Ledger: "x.yaml"}},
		{[]string{"check", "--ledger", "x.yaml"}, "sub", Query{Sub: "check", Ledger: filepath.Join("sub", "x.yaml")}},
		{[]string{"check", "--ledger", abs}, "sub", Query{Sub: "check", Ledger: abs}},
		{[]string{"check", "--ledger", "-"}, "sub", Query{Sub: "check", Ledger: "-"}},
	} {
		c, err := Parse(tc.args, tc.origin, false, false, nil)
		if err != nil || c.Query != tc.want {
			t.Errorf("Parse(%q) from %q = %+v, %v, want %+v", tc.args, tc.origin, c.Query, err, tc.want)
		}
	}
}

// A sound project: the lines that say so, none under --quiet, and under
// --json the config's path, valid and no problem, with no warnings key.
func TestCheckSound(t *testing.T) {
	p := over(t, sound())
	code, stdout, stderr, err := call(t, p, "check")
	want := "itos.yaml is valid, and so are the 1 ledger files it reads\ntasks/work-items.yaml: sound\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("check = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if code, stdout, stderr, err := call(t, p, "--quiet", "check"); code != 0 || err != nil || stdout != "" || stderr != "" {
		t.Errorf("--quiet check = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = call(t, p, "--json", "check")
	if code != 0 || err != nil || compact(stdout) != `{"schema":1,"config":"itos.yaml","valid":true,"problems":[]}` || stderr != "" {
		t.Errorf("--json check = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p.trees.Files["worktree"]["tasks/phase-2.yaml"] = "[]\n"
	if _, stdout, _, _ := call(t, p, "check", "--ledger", "tasks/phase-1.yaml"); !strings.HasPrefix(stdout, "itos.yaml is valid, and so are the 1 ledger files it reads\n") {
		t.Errorf("check --ledger = stdout %q, want the one file read", stdout)
	}
}

// Problems are FAIL lines and warnings WARN lines on stderr, exit 1, and
// no line saying the config is valid; under --json, valid false, each
// problem with its area, and the warnings.
func TestCheckProblems(t *testing.T) {
	p := over(t, with(sound(), map[string]string{
		"itos.yaml":   "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", group: { label: skip } }\nwork: { people: { source: yaml, file: people.yaml } }\n",
		"people.yaml": "",
	}))
	code, stdout, stderr, err := call(t, p, "check")
	fail := "FAIL itos.yaml: ledger.group.label is skip, and --skip is already another flag of itos task, so it never takes a group\n"
	if code != 1 || err != nil || stdout != "" || !strings.HasPrefix(stderr, fail) || !strings.Contains(stderr, "\nWARN ") {
		t.Errorf("check = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = call(t, p, "--json", "check")
	got := compact(stdout)
	if code != 1 || err != nil || stderr != "" || !strings.HasPrefix(got, `{"schema":1,"config":"itos.yaml","valid":false,"problems":[{"rule":"config-group-label",`) || !strings.Contains(got, `"area":"config"}],"warnings":[{`) || !strings.Contains(got, `"area":"people"}]}`) {
		t.Errorf("--json check = exit %d, %v, stdout %s, stderr %q", code, err, got, stderr)
	}
	// A problem with no warning prints no warnings key.
	p.trees.Files["worktree"]["people.yaml"] = "- a\n"
	if _, stdout, _, _ := call(t, p, "--json", "check"); strings.Contains(stdout, "warnings") || !strings.Contains(stdout, `"valid": false`) {
		t.Errorf("--json check, no warning = %s", stdout)
	}
	// Warnings alone fail nothing, and the lines still print.
	p = over(t, with(sound(), map[string]string{"people.yaml": ""}))
	code, stdout, stderr, err = call(t, p, "check")
	if code != 0 || err != nil || !strings.HasPrefix(stderr, "WARN ") || !strings.HasPrefix(stdout, "itos.yaml is valid") {
		t.Errorf("check, a warning = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// Data that cannot be read at all is an error, exit 0, with nothing printed.
func TestCheckCannotRead(t *testing.T) {
	p := over(t, with(sound(), map[string]string{
		"itos.yaml":  "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { decisions: decided.md, people: { source: yaml, file: people.yaml } }\n",
		"decided.md": "x",
	}))
	code, stdout, stderr, err := call(t, p, "--json", "check")
	if code != 0 || err == nil || stdout != "" || stderr != "" {
		t.Errorf("check = exit %d, %v, stdout %q, stderr %q, want the error", code, err, stdout, stderr)
	}
}

// Under --json no problem is an empty list, never null.
func TestCheckedListsNoProblemAsEmpty(t *testing.T) {
	var stdout, stderr strings.Builder
	c := CLI{Query: Query{Sub: "check"}, JSON: true}
	if err := c.checked(&stdout, &stderr, Answer{Path: "itos.yaml"}); err != nil || compact(stdout.String()) != `{"schema":1,"config":"itos.yaml","valid":true,"problems":[]}` {
		t.Errorf("checked = %v, stdout %q", err, stdout.String())
	}
}

// The ci.steps example ends the ci mapping, before the next key, and is a
// valid ci.steps once its comment marks are taken off; a table with no ci is
// printed as it is.
func TestWithStepsExample(t *testing.T) {
	got := withStepsExample("a: 1\nci:\n  x: 1\n  y:\n    - z\nwork:\n  w: 1\n")
	if want := "a: 1\nci:\n  x: 1\n  y:\n    - z\n" + stepsExample + "work:\n  w: 1\n"; got != want {
		t.Errorf("ci before another key = %q, want %q", got, want)
	}
	if got, want := withStepsExample("ci:\n  x: 1\n"), "ci:\n  x: 1\n"+stepsExample; got != want {
		t.Errorf("ci last = %q, want %q", got, want)
	}
	if got := withStepsExample("a: 1\n"); got != "a: 1\n" {
		t.Errorf("no ci = %q", got)
	}
	_, example, _ := strings.Cut(stepsExample, "  # steps:\n")
	var doc struct {
		CI struct {
			Steps []any `yaml:"steps"`
		} `yaml:"ci"`
	}
	file := "ci:\n  steps:\n" + strings.ReplaceAll(example, "  # ", "  ")
	if err := yaml.Unmarshal([]byte(file), &doc); err != nil || len(doc.CI.Steps) != 3 {
		t.Errorf("the example uncommented = %+v, %v\n%s", doc, err, file)
	}
}

// --print-defaults prints the table of defaults for this config's ledger,
// as YAML or JSON, the table's own when the config does not load, and a
// stealth config's when it is the stealth one.
func TestPrintDefaults(t *testing.T) {
	p := over(t, map[string]string{"itos.yaml": "version: 1\nledger: { files: \"ledger/phase-{group}.yaml\" }\n"})
	code, stdout, stderr, err := call(t, p, "check", "--print-defaults")
	if code != 0 || err != nil || stderr != "" || !strings.Contains(stdout, "registry: ledger/work-items.yaml\n") {
		t.Errorf("check --print-defaults = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = call(t, p, "--json", "check", "--print-defaults")
	if code != 0 || err != nil || !strings.HasPrefix(compact(stdout), `{"schema":1,"defaults":{`) || !strings.Contains(compact(stdout), `"registry":"ledger/work-items.yaml"`) {
		t.Errorf("--json check --print-defaults = exit %d, %v, stdout %q", code, err, stdout)
	}
	p.trees.Files["worktree"]["itos.yaml"] = "version: 1\nnope: 1\nledger: { files: \"ledger/phase-{group}.yaml\" }\n"
	if code, stdout, _, err := call(t, p, "check", "--print-defaults"); code != 0 || err != nil || stdout != withStepsExample(value.YAML(itosconfig.DefaultsFor(nil, false))) {
		t.Errorf("check --print-defaults, a config that does not load = exit %d, %v, stdout %q", code, err, stdout)
	}
	// The stealth config, in the git common dir the repository names.
	stealth := filepath.Join(".git", "itos", "itos.yaml")
	if err := os.MkdirAll(filepath.Dir(stealth), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stealth, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_CONFIG", stealth)
	p.VCS.Located = map[string]vcsport.Location{"": {Top: ".", GitDir: ".git", Common: ".git"}}
	p.trees.Files["worktree"][filepath.ToSlash(stealth)] = "version: 1\n"
	if _, stdout, _, err := call(t, p, "check", "--print-defaults"); err != nil || stdout != withStepsExample(value.YAML(itosconfig.DefaultsFor(nil, true))) || stdout == withStepsExample(value.YAML(itosconfig.DefaultsFor(nil, false))) {
		t.Errorf("check --print-defaults, stealth = %v, stdout %q", err, stdout)
	}
}

// get refuses a key the schema does not have, naming a removed key's fix
// or the keys there are, before any config is read.
func TestGetRefusesAnUnknownKey(t *testing.T) {
	p := over(t, map[string]string{"itos.yaml": "nope: 1\n"})
	for key, want := range map[string]string{
		"nope":             "config get: unknown key nope",
		"commits.nope":     "config get: unknown key commits.nope; the keys there are ",
		"ci.range.command": "config get: ci.range.command was removed in v5; remove ci.range.command, and set ci.range.provider to github or none",
	} {
		_, stdout, _, err := call(t, p, "get", key)
		var u out.UsageError
		if !errors.As(err, &u) || !strings.HasPrefix(err.Error(), want) || stdout != "" {
			t.Errorf("get %s = %v, stdout %q, want %q", key, err, stdout, want)
		}
	}
	if _, _, _, err := call(t, p, "get", "hooks.bin.x"); !usageOf(err, "config get: unknown key hooks.bin.x") {
		t.Errorf("get hooks.bin.x = %v, want the key unknown, with no keys there", err)
	}
}

// get prints a scalar bare, a mapping or a list as JSON on one line and no
// value nothing; under --json the key and its value, null for none. A
// config that does not load is its error.
func TestGet(t *testing.T) {
	p := over(t, map[string]string{"itos.yaml": "version: 1\nhooks: { bin: go run ./cmd/itos }\ncommits: { types: [feat, fix] }\n"})
	for key, want := range map[string]string{
		"hooks.bin":     "go run ./cmd/itos\n",
		"version":       "1\n",
		"commits.types": "[\"feat\",\"fix\"]\n",
		"commits.since": "",
	} {
		code, stdout, stderr, err := call(t, p, "get", key)
		if code != 0 || err != nil || stdout != want || stderr != "" {
			t.Errorf("get %s = exit %d, %v, stdout %q, stderr %q, want %q", key, code, err, stdout, stderr, want)
		}
	}
	if _, stdout, _, _ := call(t, p, "get", "hooks"); !strings.HasPrefix(stdout, `{"bin":"go run ./cmd/itos"`) || strings.Count(stdout, "\n") != 1 {
		t.Errorf("get hooks = %q, want a JSON mapping on one line", stdout)
	}
	if _, stdout, _, _ := call(t, p, "get", "ci.stop_at_first_failure"); stdout != "true\n" {
		t.Errorf("get ci.stop_at_first_failure = %q, want its default", stdout)
	}
	for key, want := range map[string]string{
		"hooks.bin":     `{"schema":1,"key":"hooks.bin","value":"go run ./cmd/itos"}`,
		"commits.since": `{"schema":1,"key":"commits.since","value":null}`,
	} {
		code, stdout, _, err := call(t, p, "--json", "get", key)
		if code != 0 || err != nil || compact(stdout) != want {
			t.Errorf("--json get %s = exit %d, %v, stdout %q, want %s", key, code, err, stdout, want)
		}
	}
	p.trees.Files["worktree"]["itos.yaml"] = "version: 1\nnope: 1\n"
	code, stdout, _, err := call(t, p, "get", "hooks.bin")
	var c *itosconfig.Error
	if code != 0 || !errors.As(err, &c) || stdout != "" {
		t.Errorf("get over a config that does not load = exit %d, %v, stdout %q, want its error", code, err, stdout)
	}
}
