package initcmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/agentrules"
)

// writeIn is writeRules over f, its block block, what AGENTS.md holds
// read and split first.
func writeIn(t *testing.T, f *fake, block string) ([]writtenFile, error) {
	t.Helper()
	text, _, err := agentrules.ReadText(agentsFile)
	if err != nil {
		t.Fatal(err)
	}
	before, found, after, err := agentrules.Split(text)
	if err != nil {
		t.Fatal(err)
	}
	return rulesOffer{ports: f}.writeRules(context.Background(), text, before, found, after, block)
}

// A file written with CRLF keeps CRLF, its text outside the markers as it
// was, and a CLAUDE.md with CRLF gains the import as its last line, CRLF too.
func TestWriteRulesKeepsCRLF(t *testing.T) {
	f := scratch(t, "", map[string]string{
		agentsFile: "Mine.\r\n\r\n<!-- itos:begin -->\r\nold\r\n<!-- itos:end -->\r\nAfter.\r\n",
		claudeFile: "# Claude\r\nRead on.",
	})
	block := rulesBegin + "\n\nnew\n\n" + rulesEnd + "\n"
	files, err := writeIn(t, f, block)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Action != "updated" || files[1].Action != "updated" {
		t.Errorf("files: %v", files)
	}
	want := "Mine.\r\n\r\n<!-- itos:begin -->\r\n\r\nnew\r\n\r\n<!-- itos:end -->\r\nAfter.\r\n"
	if got := read(agentsFile); got != want {
		t.Errorf("AGENTS.md:\n%q\nwant\n%q", got, want)
	}
	if got := read(claudeFile); got != "# Claude\r\nRead on.\r\n@AGENTS.md\r\n" {
		t.Errorf("CLAUDE.md: %q", got)
	}

	// Run again, nothing is left to write.
	files, err = writeIn(t, f, block)
	if err != nil || files[0].Action != "kept" || files[1].Action != "kept" {
		t.Errorf("a second run: %v, %v", files, err)
	}
}

// A CLAUDE.md that is a link to AGENTS.md reads the block already, and is
// never given an import of itself.
func TestWriteRulesLeavesACLAUDELinkedToAGENTS(t *testing.T) {
	f := scratch(t, "", map[string]string{agentsFile: "Mine.\n"})
	if err := os.Symlink(agentsFile, claudeFile); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	block := rulesBegin + "\n" + rulesEnd + "\n"
	files, err := writeIn(t, f, block)
	if err != nil || files[1].Action != "kept" {
		t.Fatalf("files: %v, %v", files, err)
	}
	if got := read(agentsFile); strings.Contains(got, claudeImport) || got != "Mine.\n\n"+block {
		t.Errorf("AGENTS.md: %q", got)
	}
}

// Each write that fails stops writeRules with its error: AGENTS.md's, its
// block replaced, written fresh or appended, and CLAUDE.md's; and an
// AGENTS.md it cannot read.
func TestWriteRulesWhereItCannotWrite(t *testing.T) {
	failed := errors.New("injected write failure")
	for name, c := range map[string]struct {
		files  map[string]string
		failed string
	}{
		"the block replaced": {map[string]string{agentsFile: olderBlock}, agentsFile},
		"AGENTS.md written":  {nil, agentsFile},
		"the block appended": {map[string]string{agentsFile: "Mine.\n"}, agentsFile},
		"CLAUDE.md":          {nil, claudeFile},
	} {
		t.Run(name, func(t *testing.T) {
			f := scratch(t, "", c.files)
			f.writeErrs = map[string]error{c.failed: failed}
			if _, err := writeIn(t, f, rulesBlock); err != failed {
				t.Errorf("%v", err)
			}
		})
	}
	f := scratch(t, "", map[string]string{agentsFile + "/x": ""})
	if _, err := (rulesOffer{ports: f}).writeRules(context.Background(), "", "", nil, "", rulesBlock); err == nil {
		t.Error("AGENTS.md a folder: no error")
	}
}

// offerIn makes the offer in a scratch folder holding the files (a path
// ending in / a folder), the terminal answering answers when there are
// any, and gives its outcome, its exit code and what it said.
func offerIn(t *testing.T, r rulesOffer, answers string, files map[string]string) (rulesOutcome, int, string) {
	t.Helper()
	f := scratch(t, "commit", nil)
	for p, text := range files {
		if dir, ok := strings.CutSuffix(p, "/"); ok {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		put(t, p, text)
	}
	var log strings.Builder
	r.log, r.answers, r.ask, r.ports = &log, strings.NewReader(answers), answers != "", f
	got, code := r.run(context.Background())
	return got, code, log.String()
}

// The offer outside a stealth config, each way it ends: markers it cannot
// place, or an AGENTS.md it cannot read, fail it, exit 1 only when the block
// was asked for; a no at the terminal declines; the block in place already is
// current; a CLAUDE.md it cannot read fails the write.
func TestRulesOfferOutcomes(t *testing.T) {
	write, rerun := rulesOffer{flag: RulesFlag{Given: true, Write: true}}, rulesOffer{rerun: true}
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
		{"--no-agent-rules", rulesOffer{flag: RulesFlag{Given: true}}, "", nil, "declined", 0, ""},
		{"lone marker, rerun", rerun, "", lone, "failed", 0, "AGENTS.md's itos rules for agents cannot be checked: AGENTS.md has a"},
		{"lone marker, asked", write, "", lone, "failed", exitPolicy, "AGENTS.md is not written: AGENTS.md has a"},
		{"AGENTS.md a folder", rerun, "", map[string]string{agentsFile + "/": ""}, "failed", 0, "cannot be checked"},
		{"a no at the terminal", rulesOffer{}, "n\n", nil, "declined", 0, "Write itos's rules for agents into AGENTS.md"},
		{"a yes at the terminal", rulesOffer{}, "y\n", nil, "written", 0, "CLAUDE.md imports it (@AGENTS.md).\n"},
		{"unasked", rulesOffer{}, "", nil, "offered", 0, rulesHowTo},
		{"unasked, rerun", rerun, "", nil, "offered", 0, ""},
		{"an empty AGENTS.md", write, "", map[string]string{agentsFile: ""}, "written", 0, "updated AGENTS.md\nwrote CLAUDE.md\n"},
		{"in place already", write, "", current, "current", 0, "AGENTS.md holds this itos's rules for agents already"},
		{"in place already, rerun", rerun, "", current, "current", 0, ""},
		{"CLAUDE.md a folder", write, "", map[string]string{claudeFile + "/": ""}, "failed", exitPolicy, "AGENTS.md is not written"},
		{"another itos's block, rerun", rerun, "", map[string]string{agentsFile: "Mine.\n\n" + olderBlock},
			"stale", 0, agentrules.StaleLine(false)},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, code, log := offerIn(t, c.offer, c.answers, c.files)
			if got.Action != c.action || code != c.code || !strings.Contains(log, c.says) || (c.says == "" && log != "") {
				t.Errorf("%s, exit %d:\n%s", got.Action, code, log)
			}
		})
	}
}

// olderBlock is a block between itos's markers other than the one this itos
// writes.
const olderBlock = rulesBegin + "\n\n## The rules itos holds this repository to\n\n" + rulesEnd + "\n"

func TestRulesQuestion(t *testing.T) {
	for input, want := range map[string]string{"\n": "yes", "yes\n": "yes", "N\n": "no", "maybe\ny\n": "yes", "maybe": "", "": ""} {
		var log strings.Builder
		if got := (rulesOffer{log: &log, answers: strings.NewReader(input)}).question("Write?"); got != want {
			t.Errorf("%q: %q, want %q\n%s", input, got, want, log.String())
		}
	}
}
