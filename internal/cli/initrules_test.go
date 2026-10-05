package cli

import (
	"os"
	"strings"
	"testing"
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
