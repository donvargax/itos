package agentrules

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/vcs/port"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// olderBlock is a block between itos's markers other than the one this itos
// writes.
const olderBlock = Begin + "\n\n## The rules itos holds this repository to\n\n" + End + "\n"

// putFile writes text to path, making its folder.
func putFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// inRepository stands the test in an empty folder that the fake VCS
// locates as a repository's top, its git folder .git.
func inRepository(t *testing.T) *vcstest.VCS {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return &vcstest.VCS{Root: dir, Located: map[string]port.Location{"": {Top: dir, GitDir: ".git", Common: ".git"}}}
}

func TestSplit(t *testing.T) {
	before, found, after, err := Split("Mine.\n\n<!-- itos:begin -->\nold\n<!-- itos:end -->\nAfter.\n")
	if err != nil || found == nil {
		t.Fatalf("a file with a block: %v, %v", found, err)
	}
	if before != "Mine.\n\n" || *found != "<!-- itos:begin -->\nold\n<!-- itos:end -->\n" || after != "After.\n" {
		t.Errorf("cut as %q, %q, %q", before, *found, after)
	}
	if _, found, _, err := Split("Mine.\n"); err != nil || found != nil {
		t.Errorf("a file without markers has no block: %v, %v", found, err)
	}
	if _, found, _, err := Split("<!-- itos:begin -->\n<!-- itos:end -->"); err != nil || found == nil ||
		*found != "<!-- itos:begin -->\n<!-- itos:end -->\n" {
		t.Errorf("a block ending the file without a newline gains one: %v, %v", found, err)
	}
	for _, text := range []string{
		"<!-- itos:begin -->\nMine.\n",
		"Mine.\n<!-- itos:end -->\n",
		"<!-- itos:end -->\n<!-- itos:begin -->\n",
		"<!-- itos:begin -->\n<!-- itos:end -->\n<!-- itos:begin -->\n<!-- itos:end -->\n",
	} {
		if _, _, _, err := Split(text); err == nil || kind.Of(err) != kind.Usage {
			t.Errorf("%q: %v, not a usage error", text, err)
		}
	}
}

// A block is appended after a blank line, the text's last line ended first.
func TestAppendBlock(t *testing.T) {
	for text, want := range map[string]string{
		"":          "B\n",
		"Mine.":     "Mine.\n\nB\n",
		"Mine.\n":   "Mine.\n\nB\n",
		"Mine.\n\n": "Mine.\n\nB\n",
	} {
		if got := AppendBlock(text, "B\n"); got != want {
			t.Errorf("AppendBlock(%q) = %q, not %q", text, got, want)
		}
	}
}

// A file is read LF, whether it used CRLF said, and written back with the
// line endings it had; a file not there is "".
func TestReadAndWriteText(t *testing.T) {
	t.Chdir(t.TempDir())
	if text, crlf, err := ReadText("none.md"); text != "" || crlf || err != nil {
		t.Errorf("a file not there: %q, %v, %v", text, crlf, err)
	}
	if err := WriteText("crlf.md", "a\nb\n", true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile("crlf.md"); string(data) != "a\r\nb\r\n" {
		t.Errorf("written: %q", data)
	}
	if text, crlf, err := ReadText("crlf.md"); text != "a\nb\n" || !crlf || err != nil {
		t.Errorf("read: %q, %v, %v", text, crlf, err)
	}
	if err := WriteText("lf.md", "a\n", false); err != nil {
		t.Fatal(err)
	}
	if text, crlf, err := ReadText("lf.md"); text != "a\n" || crlf || err != nil {
		t.Errorf("read: %q, %v, %v", text, crlf, err)
	}
}

func TestStaleLine(t *testing.T) {
	if got := StaleLine(false); !strings.HasPrefix(got, "AGENTS.md's itos rules are not the ones this itos writes: ") {
		t.Errorf("AGENTS.md's: %q", got)
	}
	if got := StaleLine(true); !strings.HasPrefix(got, "The itos rules for agents are not the ones this itos writes: ") {
		t.Errorf("stealth: %q", got)
	}
}

// The files holding a block other than this itos's, as status reads them
// (slice 124): AGENTS.md's, or under a stealth config the copies init writes
// there; no block, the current one, markers out of place and a file that
// cannot be read are none.
func TestStale(t *testing.T) {
	ctx := t.Context()
	v := inRepository(t)
	if got := Stale(ctx, v, false); got != nil {
		t.Errorf("no AGENTS.md: %v", got)
	}
	for text, want := range map[string][]string{
		"Mine.\n":                      nil,
		"Mine.\n\n" + Block:            nil,
		"Mine.\n\n" + olderBlock:       {AgentsFile},
		Begin + "\nMine.\n":            nil,
		olderBlock + "\n" + olderBlock: nil,
	} {
		putFile(t, AgentsFile, text)
		if got := Stale(ctx, v, false); !slices.Equal(got, want) {
			t.Errorf("AGENTS.md of %q: %v, not %v", text, got, want)
		}
	}
	if err := os.Remove(AgentsFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(AgentsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Stale(ctx, v, false); got != nil {
		t.Errorf("an AGENTS.md that cannot be read: %v", got)
	}

	rules := filepath.ToSlash(filepath.Join(".git", config.StealthFolder, AgentsFile))
	if got := Stale(ctx, v, true); got != nil {
		t.Errorf("stealth, AGENTS.md a folder: %v", got)
	}
	if err := os.Remove(AgentsFile); err != nil {
		t.Fatal(err)
	}
	if got := Stale(ctx, v, true); got != nil {
		t.Errorf("stealth, no rules written: %v", got)
	}
	putFile(t, rules, olderBlock)
	putFile(t, OverrideFile, Block)
	if got := Stale(ctx, v, true); !slices.Equal(got, []string{rules}) {
		t.Errorf("stealth, an older block in the git folder: %v", got)
	}
	putFile(t, OverrideFile, "Mine.\n\n"+olderBlock)
	if got := Stale(ctx, v, true); !slices.Equal(got, []string{rules, OverrideFile}) {
		t.Errorf("stealth, older blocks in both: %v", got)
	}
	putFile(t, OverrideFile, Begin+"\nMine.\n")
	if got := Stale(ctx, v, true); got != nil {
		t.Errorf("stealth, markers out of place: %v", got)
	}
	putFile(t, rules, Block)
	putFile(t, OverrideFile, Block)
	if got := Stale(ctx, v, true); got != nil {
		t.Errorf("stealth, the current block: %v", got)
	}
	if got := Stale(ctx, &vcstest.VCS{}, true); got != nil {
		t.Errorf("stealth, outside a repository: %v", got)
	}
	if got := Stale(ctx, &vcstest.VCS{}, false); got != nil {
		t.Errorf("outside a repository, the current block: %v", got)
	}
}

// upgrade's rewrite of a present block (slice 126): each file Stale finds,
// its block in this itos's words and nothing else changed, CRLF kept; no
// block, the current one, markers out of place and a file that cannot be
// read give no edit. Under a stealth config the rules file and
// AGENTS.override.md are rewritten, the override's copy of AGENTS.md kept.
func TestRefreshed(t *testing.T) {
	ctx := t.Context()
	v := inRepository(t)
	if got := Refreshed(ctx, v, false); got != nil {
		t.Errorf("no AGENTS.md: %v", got)
	}
	for _, text := range []string{"Mine.\n", "Mine.\n\n" + Block, Begin + "\nMine.\n", olderBlock + "\n" + olderBlock} {
		putFile(t, AgentsFile, text)
		if got := Refreshed(ctx, v, false); got != nil {
			t.Errorf("AGENTS.md of %q: %v", text, got)
		}
	}
	putFile(t, AgentsFile, "Mine.\r\n\r\n"+strings.ReplaceAll(olderBlock, "\n", "\r\n")+"\r\nAfter.\r\n")
	want := []Edit{{AgentsFile, AgentsFile, "Mine.\n\n" + Block + "\nAfter.\n", true}}
	if got := Refreshed(ctx, v, false); !slices.Equal(got, want) {
		t.Errorf("an older block, CRLF: %v, not %v", got, want)
	}
	if err := want[0].Write(); err != nil {
		t.Fatal(err)
	}
	if text, _ := os.ReadFile(AgentsFile); string(text) != "Mine.\r\n\r\n"+strings.ReplaceAll(Block, "\n", "\r\n")+"\r\nAfter.\r\n" {
		t.Errorf("written: %q", text)
	}
	if err := os.Remove(AgentsFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(AgentsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Refreshed(ctx, v, false); got != nil {
		t.Errorf("an AGENTS.md that cannot be read: %v", got)
	}

	// The rules file as said, with slashes, and on disk, as the code builds it:
	// backslashed on Windows.
	disk := filepath.Join(".git", config.StealthFolder, AgentsFile)
	rules := filepath.ToSlash(disk)
	if got := Refreshed(ctx, v, true); got != nil {
		t.Errorf("stealth, AGENTS.md a folder: %v", got)
	}
	if err := os.Remove(AgentsFile); err != nil {
		t.Fatal(err)
	}
	if got := Refreshed(ctx, v, true); got != nil {
		t.Errorf("stealth, no rules written: %v", got)
	}
	if got := Refreshed(ctx, &vcstest.VCS{}, true); got != nil {
		t.Errorf("stealth, outside a repository: %v", got)
	}
	putFile(t, rules, olderBlock)
	putFile(t, OverrideFile, Block)
	want = []Edit{{rules, disk, Block, false}}
	if got := Refreshed(ctx, v, true); !slices.Equal(got, want) {
		t.Errorf("stealth, an older block in the git folder: %v, not %v", got, want)
	}
	copied := CopyBegin + "\n\nThe project's.\n\n" + CopyEnd + "\n"
	putFile(t, rules, Block)
	putFile(t, OverrideFile, copied+"\n"+strings.TrimSuffix(olderBlock, "\n"))
	want = []Edit{{OverrideFile, OverrideFile, copied + "\n" + Block, false}}
	if got := Refreshed(ctx, v, true); !slices.Equal(got, want) {
		t.Errorf("stealth, an older block after the copy: %v, not %v", got, want)
	}
	putFile(t, rules, olderBlock)
	putFile(t, OverrideFile, "Mine.\n\n"+olderBlock+"After.\n")
	want = []Edit{{rules, disk, Block, false}, {OverrideFile, OverrideFile, "Mine.\n\n" + Block + "After.\n", false}}
	if got := Refreshed(ctx, v, true); !slices.Equal(got, want) {
		t.Errorf("stealth, older blocks in both: %v, not %v", got, want)
	}
	putFile(t, OverrideFile, copied)
	if got := Refreshed(ctx, v, true); !slices.Equal(got, want[:1]) {
		t.Errorf("stealth, an override with no rules block: %v", got)
	}
	putFile(t, OverrideFile, Begin+"\nMine.\n")
	if got := Refreshed(ctx, v, true); got != nil {
		t.Errorf("stealth, markers out of place: %v", got)
	}
	putFile(t, rules, Begin+"\nMine.\n")
	putFile(t, OverrideFile, Block)
	if got := Refreshed(ctx, v, true); got != nil {
		t.Errorf("stealth, markers out of place in the rules file: %v", got)
	}
}
