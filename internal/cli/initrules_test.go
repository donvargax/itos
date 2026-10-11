package cli

import (
	"os"
	"path/filepath"
	"slices"
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
  types: [feat]
  header_lint: { use: builtin }
`, "### Commit types\n\nA commit's type is one of `feat`. The header is held to commitlint's config-conventional rules " +
			"by itos's own lint: `type: subject` or `type(scope): subject`.\n\n"},
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

// offerIn makes the offer in a scratch folder holding the files (a path
// ending in / a folder), the terminal answering answers when there are
// any, and gives its outcome, its exit code and what it said.
func offerIn(t *testing.T, r rulesOffer, answers string, files map[string]string) (rulesOutcome, int, string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for p, text := range files {
		if dir, ok := strings.CutSuffix(p, "/"); ok {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		putFile(t, p, text)
	}
	var log strings.Builder
	r.log, r.answers, r.ask = &log, strings.NewReader(answers), answers != ""
	got, code := r.run()
	return got, code, log.String()
}

// The offer outside a stealth config, each way it ends: markers it cannot
// place, or an AGENTS.md it cannot read, fail it, exit 1 only when the block
// was asked for; a no at the terminal declines; the block in place already is
// current; a CLAUDE.md it cannot read fails the write.
func TestRulesOfferOutcomes(t *testing.T) {
	write, rerun := rulesOffer{flag: rulesFlag{given: true, write: true}}, rulesOffer{rerun: true}
	lone := map[string]string{agentsFile: rulesBegin + "\nMine.\n"}
	current := map[string]string{agentsFile: "Mine.\n\n" + rulesBlock, claudeFile: claudeImport + "\n"}
	for _, c := range []struct {
		name    string
		offer   rulesOffer
		answers string
		files   map[string]string
		action  string
		code    int
		says    string
	}{
		{"lone marker, rerun", rerun, "", lone, "failed", 0, "AGENTS.md's itos rules for agents cannot be checked: AGENTS.md has a"},
		{"lone marker, asked", write, "", lone, "failed", ExitPolicy, "AGENTS.md is not written: AGENTS.md has a"},
		{"AGENTS.md a folder", rerun, "", map[string]string{agentsFile + "/": ""}, "failed", 0, "cannot be checked"},
		{"a no at the terminal", rulesOffer{}, "n\n", nil, "declined", 0, "Write itos's rules for agents into AGENTS.md"},
		{"in place already", write, "", current, "current", 0, "AGENTS.md holds this itos's rules for agents already"},
		{"CLAUDE.md a folder", write, "", map[string]string{claudeFile + "/": ""}, "failed", ExitPolicy, "AGENTS.md is not written"},
		{"another itos's block, rerun", rerun, "", map[string]string{agentsFile: "Mine.\n\n" + olderBlock},
			"stale", 0, staleRulesLine(false)},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, code, log := offerIn(t, c.offer, c.answers, c.files)
			if got.Action != c.action || code != c.code || !strings.Contains(log, c.says) {
				t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
			}
		})
	}
}

// olderBlock is a block between itos's markers other than the one this itos
// writes.
const olderBlock = rulesBegin + "\n\n## The rules itos holds this repository to\n\n" + rulesEnd + "\n"

// The files holding a block other than this itos's, as status reads them
// (slice 124): AGENTS.md's, or under a stealth config the copies init writes
// there; no block, the current one, markers out of place and a file that
// cannot be read are none.
func TestStaleRules(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	if got := staleRules(false); got != nil {
		t.Errorf("no AGENTS.md: %v", got)
	}
	for text, want := range map[string][]string{
		"Mine.\n":                      nil,
		"Mine.\n\n" + rulesBlock:       nil,
		"Mine.\n\n" + olderBlock:       {agentsFile},
		rulesBegin + "\nMine.\n":       nil,
		olderBlock + "\n" + olderBlock: nil,
	} {
		putFile(t, agentsFile, text)
		if got := staleRules(false); !slices.Equal(got, want) {
			t.Errorf("AGENTS.md of %q: %v, not %v", text, got, want)
		}
	}
	if err := os.Remove(agentsFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(agentsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := staleRules(false); got != nil {
		t.Errorf("an AGENTS.md that cannot be read: %v", got)
	}

	rules := filepath.ToSlash(filepath.Join(".git", config.StealthFolder, agentsFile))
	if got := staleRules(true); got != nil {
		t.Errorf("stealth, AGENTS.md a folder: %v", got)
	}
	if err := os.Remove(agentsFile); err != nil {
		t.Fatal(err)
	}
	if got := staleRules(true); got != nil {
		t.Errorf("stealth, no rules written: %v", got)
	}
	putFile(t, rules, olderBlock)
	putFile(t, overrideFile, rulesBlock)
	if got := staleRules(true); !slices.Equal(got, []string{rules}) {
		t.Errorf("stealth, an older block in the git folder: %v", got)
	}
	putFile(t, overrideFile, "Mine.\n\n"+olderBlock)
	if got := staleRules(true); !slices.Equal(got, []string{rules, overrideFile}) {
		t.Errorf("stealth, older blocks in both: %v", got)
	}
	var log strings.Builder
	if got, code := (rulesOffer{stealth: true, rerun: true, log: &log}).run(); got.Action != "stale" || code != 0 ||
		!strings.Contains(log.String(), staleRulesLine(true)) {
		t.Errorf("init's rerun: %s, exit %d:\n%s", got.Action, code, log.String())
	}
	putFile(t, overrideFile, rulesBegin+"\nMine.\n")
	if got := staleRules(true); got != nil {
		t.Errorf("stealth, markers out of place: %v", got)
	}
	putFile(t, rules, rulesBlock)
	putFile(t, overrideFile, rulesBlock)
	if got := staleRules(true); got != nil {
		t.Errorf("stealth, the current block: %v", got)
	}
	t.Chdir(t.TempDir())
	if got := staleRules(true); got != nil {
		t.Errorf("stealth, outside a repository: %v", got)
	}
}

// upgrade's rewrite of a present block (slice 126): each file staleRules
// finds, its block in this itos's words and nothing else changed, CRLF kept;
// no block, the current one, markers out of place and a file that cannot be
// read give no edit. Under a stealth config the rules file and
// AGENTS.override.md are rewritten, the override's copy of AGENTS.md kept.
func TestRefreshedRules(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	if got := refreshedRules(false); got != nil {
		t.Errorf("no AGENTS.md: %v", got)
	}
	for _, text := range []string{"Mine.\n", "Mine.\n\n" + rulesBlock, rulesBegin + "\nMine.\n", olderBlock + "\n" + olderBlock} {
		putFile(t, agentsFile, text)
		if got := refreshedRules(false); got != nil {
			t.Errorf("AGENTS.md of %q: %v", text, got)
		}
	}
	putFile(t, agentsFile, "Mine.\r\n\r\n"+strings.ReplaceAll(olderBlock, "\n", "\r\n")+"\r\nAfter.\r\n")
	want := []rulesEdit{{agentsFile, agentsFile, "Mine.\n\n" + rulesBlock + "\nAfter.\n", true}}
	if got := refreshedRules(false); !slices.Equal(got, want) {
		t.Errorf("an older block, CRLF: %v, not %v", got, want)
	}
	if err := want[0].write(); err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(agentsFile); string(text) != "Mine.\r\n\r\n"+strings.ReplaceAll(rulesBlock, "\n", "\r\n")+"\r\nAfter.\r\n" {
		t.Errorf("written: %q", text)
	}
	if err := os.Remove(agentsFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(agentsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := refreshedRules(false); got != nil {
		t.Errorf("an AGENTS.md that cannot be read: %v", got)
	}

	// The rules file as said, with slashes, and on disk, as the code builds it:
	// backslashed on Windows.
	disk := filepath.Join(".git", config.StealthFolder, agentsFile)
	rules := filepath.ToSlash(disk)
	if got := refreshedRules(true); got != nil {
		t.Errorf("stealth, AGENTS.md a folder: %v", got)
	}
	if err := os.Remove(agentsFile); err != nil {
		t.Fatal(err)
	}
	if got := refreshedRules(true); got != nil {
		t.Errorf("stealth, no rules written: %v", got)
	}
	putFile(t, rules, olderBlock)
	putFile(t, overrideFile, rulesBlock)
	want = []rulesEdit{{rules, disk, rulesBlock, false}}
	if got := refreshedRules(true); !slices.Equal(got, want) {
		t.Errorf("stealth, an older block in the git folder: %v, not %v", got, want)
	}
	copied := copyBegin + "\n\nThe project's.\n\n" + copyEnd + "\n"
	putFile(t, rules, rulesBlock)
	putFile(t, overrideFile, copied+"\n"+strings.TrimSuffix(olderBlock, "\n"))
	want = []rulesEdit{{overrideFile, overrideFile, copied + "\n" + rulesBlock, false}}
	if got := refreshedRules(true); !slices.Equal(got, want) {
		t.Errorf("stealth, an older block after the copy: %v, not %v", got, want)
	}
	putFile(t, rules, olderBlock)
	putFile(t, overrideFile, "Mine.\n\n"+olderBlock+"After.\n")
	want = []rulesEdit{{rules, disk, rulesBlock, false}, {overrideFile, overrideFile, "Mine.\n\n" + rulesBlock + "After.\n", false}}
	if got := refreshedRules(true); !slices.Equal(got, want) {
		t.Errorf("stealth, older blocks in both: %v, not %v", got, want)
	}
	putFile(t, overrideFile, copied)
	if got := refreshedRules(true); !slices.Equal(got, want[:1]) {
		t.Errorf("stealth, an override with no rules block: %v", got)
	}
	putFile(t, overrideFile, rulesBegin+"\nMine.\n")
	if got := refreshedRules(true); got != nil {
		t.Errorf("stealth, markers out of place: %v", got)
	}
	putFile(t, rules, rulesBegin+"\nMine.\n")
	putFile(t, overrideFile, rulesBlock)
	if got := refreshedRules(true); got != nil {
		t.Errorf("stealth, markers out of place in the rules file: %v", got)
	}
}
