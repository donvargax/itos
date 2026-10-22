package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
)

// The wrappers bind internal/engine/agentrules to the real git, which
// names the git folder a stealth config's rules file is in: status's stale
// blocks (slice 124) and upgrade's rewrite of them (slice 126), none outside
// a repository.
func TestStaleRulesBindTheRealGit(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	putFile(t, agentsFile, "Mine.\n\n"+olderBlock)
	disk := filepath.Join(".git", config.StealthFolder, agentsFile)
	rules := filepath.ToSlash(disk)
	putFile(t, rules, olderBlock)
	if got := staleRules(false); !slices.Equal(got, []string{agentsFile}) {
		t.Errorf("AGENTS.md: %v", got)
	}
	if got := staleRules(true); !slices.Equal(got, []string{rules}) {
		t.Errorf("stealth: %v", got)
	}
	if got, want := refreshedRules(false), []rulesEdit{{agentsFile, agentsFile, "Mine.\n\n" + rulesBlock, false}}; !slices.Equal(got, want) {
		t.Errorf("AGENTS.md rewritten: %v, not %v", got, want)
	}
	if got, want := refreshedRules(true), []rulesEdit{{rules, disk, rulesBlock, false}}; !slices.Equal(got, want) {
		t.Errorf("stealth, rewritten: %v, not %v", got, want)
	}
	t.Chdir(t.TempDir())
	if got := staleRules(true); got != nil {
		t.Errorf("stealth, outside a repository: %v", got)
	}
	if got := refreshedRules(true); got != nil {
		t.Errorf("stealth, rewritten outside a repository: %v", got)
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
	text, crlf, err := agentrules.ReadText(agentsFile)
	if err != nil || !crlf {
		t.Fatalf("readText: %v, crlf %v", err, crlf)
	}
	before, found, after, err := agentrules.Split(text)
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
	text, _, _ = agentrules.ReadText(agentsFile)
	before, found, after, _ = agentrules.Split(text)
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

// --agent-rules and --no-agent-rules are read wherever they stand, each as
// often as wanted, but not both.
func TestParseRulesFlag(t *testing.T) {
	for _, c := range []struct {
		args       []string
		given, err bool
		write      bool
	}{
		{[]string{"--plugin", "--agent-rules"}, true, false, true},
		{[]string{"--no-agent-rules", "--no-agent-rules"}, true, false, false},
		{[]string{"--agent-rules", "--agent-rules"}, true, false, true},
		{[]string{"--agent-rules", "--no-agent-rules"}, true, true, true},
		{[]string{"--no-agent-rules", "--agent-rules"}, true, true, false},
		{[]string{"--plugin"}, false, false, false},
	} {
		var f rulesFlag
		var err error
		for i := 0; i < len(c.args) && err == nil; i++ {
			var n int
			n, err = parseRulesFlag(c.args, i, &f)
			if want := strings.HasSuffix(c.args[i], "agent-rules"); err == nil && (n == 1) != want {
				t.Errorf("%q at %d: read %d", c.args, i, n)
			}
		}
		if f.given != c.given || f.write != c.write || (err != nil) != c.err ||
			(err != nil && (ExitCode(err) != ExitUsage || !strings.Contains(err.Error(), "not both"))) {
			t.Errorf("%q: %+v, %v", c.args, f, err)
		}
	}
}
