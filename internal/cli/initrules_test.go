package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

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
		if got := code(in); got != want {
			t.Errorf("code(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitBlock(t *testing.T) {
	before, found, after, err := splitBlock("Mine.\n\n<!-- itos:begin -->\nold\n<!-- itos:end -->\nAfter.\n")
	if err != nil || found == nil {
		t.Fatalf("a file with a block: %v, %v", found, err)
	}
	if before != "Mine.\n\n" || *found != "<!-- itos:begin -->\nold\n<!-- itos:end -->\n" || after != "After.\n" {
		t.Errorf("cut as %q, %q, %q", before, *found, after)
	}
	if _, found, _, err := splitBlock("Mine.\n"); err != nil || found != nil {
		t.Errorf("a file without markers has no block: %v, %v", found, err)
	}
	for _, text := range []string{
		"<!-- itos:begin -->\nMine.\n",
		"Mine.\n<!-- itos:end -->\n",
		"<!-- itos:end -->\n<!-- itos:begin -->\n",
		"<!-- itos:begin -->\n<!-- itos:end -->\n<!-- itos:begin -->\n<!-- itos:end -->\n",
	} {
		if _, _, _, err := splitBlock(text); err == nil {
			t.Errorf("%q: no error", text)
		}
	}
}

// A file written with CRLF keeps CRLF, its text outside the markers as it
// was, and a CLAUDE.md with CRLF gains the import as its last line, CRLF too.
func TestWriteRulesKeepsCRLF(t *testing.T) {
	t.Chdir(t.TempDir())
	agents := "Mine.\r\n\r\n<!-- itos:begin -->\r\nold\r\n<!-- itos:end -->\r\nAfter.\r\n"
	if err := os.WriteFile(agentsFile, []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeFile, []byte("# Claude\r\nRead on."), 0o644); err != nil {
		t.Fatal(err)
	}
	text, crlf, err := readText(agentsFile)
	if err != nil || !crlf {
		t.Fatalf("readText: %v, crlf %v", err, crlf)
	}
	before, found, after, err := splitBlock(text)
	if err != nil {
		t.Fatal(err)
	}
	block := rulesBegin + "\n\nnew\n\n" + rulesEnd + "\n"
	files, err := writeRules(text, before, found, after, block)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Action != "updated" || files[1].Action != "updated" {
		t.Errorf("files: %v", files)
	}
	got, _ := os.ReadFile(agentsFile)
	want := "Mine.\r\n\r\n<!-- itos:begin -->\r\n\r\nnew\r\n\r\n<!-- itos:end -->\r\nAfter.\r\n"
	if string(got) != want {
		t.Errorf("AGENTS.md:\n%q\nwant\n%q", got, want)
	}
	got, _ = os.ReadFile(claudeFile)
	if string(got) != "# Claude\r\nRead on.\r\n@AGENTS.md\r\n" {
		t.Errorf("CLAUDE.md: %q", got)
	}

	// Run again, nothing is left to write.
	text, _, _ = readText(agentsFile)
	before, found, after, _ = splitBlock(text)
	files, err = writeRules(text, before, found, after, block)
	if err != nil || files[0].Action != "kept" || files[1].Action != "kept" {
		t.Errorf("a second run: %v, %v", files, err)
	}
}

// A CLAUDE.md that is a link to AGENTS.md reads the block already, and is
// never given an import of itself.
func TestWriteRulesLeavesACLAUDELinkedToAGENTS(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(agentsFile, []byte("Mine.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(agentsFile, claudeFile); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	block := rulesBegin + "\n" + rulesEnd + "\n"
	files, err := writeRules("Mine.\n", "Mine.\n", nil, "", block)
	if err != nil || files[1].Action != "kept" {
		t.Fatalf("files: %v, %v", files, err)
	}
	got, _ := os.ReadFile(agentsFile)
	if strings.Contains(string(got), claudeImport) || string(got) != "Mine.\n\n"+block {
		t.Errorf("AGENTS.md: %q", got)
	}
}

// A scope's except is said beside its never.
func TestCommitPolicySaysExcept(t *testing.T) {
	file := filepath.Join(t.TempDir(), "itos.yaml")
	text := `version: 1
commits:
  types: [chore]
  scopes:
    chore: { never: ["src/**"], except: ["src/games/*/themes/**"] }
`
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	policy := strings.Join(commitPolicy(cfg, code), "\n\n")
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
  types: []
  header_lint: { stdin: "sh lint.sh" }
`, "### Commit types\n\nThe header is linted by `sh lint.sh`.\n\n### The paths each type may touch"},
	} {
		file := filepath.Join(t.TempDir(), "itos.yaml")
		if err := os.WriteFile(file, []byte("version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n"+c.config), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Load(file)
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(commitPolicy(cfg, code), "\n\n")
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("the policy is\n%s\nnot\n%s", got, c.want)
		}
	}
}
