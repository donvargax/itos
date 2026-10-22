package agentrules

import (
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	treetest "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// loaded is the config of text, read as itos.yaml.
func loaded(t *testing.T, text string) *config.Loaded {
	t.Helper()
	trees := treetest.Trees{}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcstest.VCS{}, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestCodeFencesWithMoreBackticksThanItHolds(t *testing.T) {
	cases := map[string]string{
		"go vet ./...":       "`go vet ./...`",
		"echo `date`":        "`` echo `date` ``",
		"a `b` c":            "``a `b` c``",
		"`x`":                "`` `x` ``",
		"a\n  b\n":           "`a   b`",
		"tasks/phase-{g}.md": "`tasks/phase-{g}.md`",
	}
	for in, want := range cases {
		if got := Code(in); got != want {
			t.Errorf("Code(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProse(t *testing.T) {
	for _, c := range []struct {
		words []string
		want  string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a or b"},
		{[]string{"a", "b", "c"}, "a, b or c"},
	} {
		if got := Prose(c.words, "or"); got != c.want {
			t.Errorf("Prose(%q) = %q, want %q", c.words, got, c.want)
		}
	}
}

// A footer required for all is required for every type the config lists;
// one with no types, or a word other than all, for none.
func TestTypesOf(t *testing.T) {
	cfg := loaded(t, `version: 1
commits:
  types: [feat, docs]
  footers:
    Ref: { source: text, required_for: all }
`)
	policy := strings.Join(CommitPolicy(cfg, Code), "\n\n")
	if want := "- `feat`: `Ref:`.\n- `docs`: `Ref:`."; !strings.Contains(policy, want) {
		t.Errorf("the policy does not say %q:\n%s", want, policy)
	}
	if got := typesOf(cfg, nil); got != nil {
		t.Errorf("no types: %v", got)
	}
	if got := typesOf(cfg, &config.Types{Word: "none"}); got != nil {
		t.Errorf("a word other than all: %v", got)
	}
}

// A scope's except is said beside its never.
func TestCommitPolicySaysExcept(t *testing.T) {
	cfg := loaded(t, `version: 1
commits:
  types: [chore]
  scopes:
    chore: { never: ["src/**"], except: ["src/games/*/themes/**"] }
`)
	policy := strings.Join(CommitPolicy(cfg, Code), "\n\n")
	want := "- `chore` may not touch `src/**`, except `src/games/*/themes/**`."
	if !strings.Contains(policy, want) {
		t.Errorf("the policy does not say %q:\n%s", want, policy)
	}
}

// The commit policy itos guide commit prints (slice 119) says each header lint, footer source and scope rule the config sets.
func TestCommitPolicy(t *testing.T) {
	for _, c := range []struct{ config, want string }{
		{`commits:
  types: [feat, docs]
  header_lint: { hook: "lint {file}" }
  footers:
    Task: { source: ledger, required_for: [docs], validate_for: all }
    Item: { source: registry, validate_for: all, in_place_of: { Task: [docs] } }
    Scenarios: { source: { tests: scenario }, strip_prefix: "@", required_for: [feat], validate_for: [feat], must_be_live: true }
  scopes:
    feat: { must_touch: ["src/**"] }
    docs: {}
tests:
  scenario: { root: features, id: "ID-\\d+", tag_prefix: "@", wip_tag: "@draft" }
`, "### Commit types\n\n" +
			"A commit's type is one of `feat` or `docs`. The header is linted by `lint {file}`.\n\n" +
			"### Footers\n\n" +
			"- `Task:` names tasks of the ledger, `tasks/phase-{group}.yaml`.\n" +
			"- `Item:` names items of the work registry, `tasks/work-items.yaml`.\n" +
			"- `Scenarios:` names `scenario` tests by their IDs, live ones only (not `@draft`).\n\n" +
			"The footers each type needs:\n\n" +
			"- `feat`: `Scenarios:`.\n" +
			"- `docs`: `Task:` or `Item:`.\n\n" +
			"### The paths each type may touch\n\n" +
			"- `feat` must touch at least one of `src/**`.\n" +
			"- `docs` may touch anything."},
		{`commits:
  types: [feat]
  header_lint: { use: builtin }
`, "### Commit types\n\nA commit's type is one of `feat`. The header is held to commitlint's config-conventional rules " +
			"by itos's own lint: `type: subject` or `type(scope): subject`.\n\n"},
		{`commits:
  types: []
  header_lint: { stdin: "sh lint.sh" }
`, "### Commit types\n\nThe header is linted by `sh lint.sh`.\n\n### The paths each type may touch"},
	} {
		cfg := loaded(t, "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n"+c.config)
		got := strings.Join(CommitPolicy(cfg, Code), "\n\n")
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("the policy is\n%s\nnot\n%s", got, c.want)
		}
	}
}
