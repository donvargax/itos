package agentrules

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

const (
	// ClaudeLocalFile is Claude Code's file of one person's, never committed.
	ClaudeLocalFile = "CLAUDE.local.md"
	// OverrideFile is Codex's, which it reads in place of AGENTS.md.
	OverrideFile = "AGENTS.override.md"
	// CopyBegin and CopyEnd are the lines the copy of AGENTS.md in
	// OverrideFile is written between.
	CopyBegin = "<!-- itos:agents-md:begin -->"
	CopyEnd   = "<!-- itos:agents-md:end -->"
)

// Target is a file the stealth rules are written to: its path as said
// (slashes) and on disk, relative to the folder itos runs in, its text (LF), whether it is there and used CRLF,
// whether it holds a block of itos's, whether it is at the worktree's top
// (kept out of git by info/exclude), and the text itos would give it.
type Target struct {
	Path, Disk string
	Text       string
	Exists     bool
	CRLF       bool
	found      bool
	Top        bool
	Next       string
}

// Stealth is what the stealth rules are made from, as read: the three
// files and the project's AGENTS.md (LF, "" when it is not there).
// ImportPath is the rules file's path, as CLAUDE.local.md imports it.
type Stealth struct {
	rules, claude, override Target
	agents                  string
	hasAgents               bool
	ImportPath              string
	// The spans of OverrideFile's lines (strings.SplitAfter) that hold the
	// copy and the rules block, noSpan for none, and whether the file is
	// itos's own, so that it holds the copy.
	lines           []string
	copyAt, rulesAt [2]int
	Ours            bool
	// dir is the folder itos runs in, which a Disk path is read from.
	dir string
}

// ReadStealth reads the files of the stealth rules, from the worktree's
// top, v locating the git common dir; markers out of order are an error,
// as in AGENTS.md.
func ReadStealth(ctx context.Context, v port.VCS) (*Stealth, error) {
	l, err := v.Locate(ctx, "")
	if err != nil {
		return nil, err
	}
	disk := filepath.Join(l.Common, config.StealthFolder, AgentsFile)
	s := &Stealth{ImportPath: filepath.ToSlash(disk), dir: v.Folder(ctx)}
	read := func(t *Target, path, disk string, top bool) error {
		t.Path, t.Disk, t.Top = path, disk, top
		var err error
		t.Text, t.CRLF, err = ReadText(port.In(s.dir, disk))
		t.Exists = exists(port.In(s.dir, disk))
		return err
	}
	if err := read(&s.rules, s.ImportPath, disk, false); err != nil {
		return nil, err
	}
	if err := read(&s.claude, ClaudeLocalFile, ClaudeLocalFile, true); err != nil {
		return nil, err
	}
	if err := read(&s.override, OverrideFile, OverrideFile, true); err != nil {
		return nil, err
	}
	s.agents, _, err = ReadText(port.In(s.dir, AgentsFile))
	if err != nil {
		return nil, err
	}
	s.hasAgents = exists(port.In(s.dir, AgentsFile))
	return s, s.parse()
}

// parse finds itos's blocks in the files as read.
func (s *Stealth) parse() error {
	var err error
	for _, t := range []*Target{&s.rules, &s.claude} {
		_, found, _, err := splitBlockOf(t.Text, t.Path, Begin, End)
		if err != nil {
			return err
		}
		t.found = found != nil
	}
	s.lines = strings.SplitAfter(s.override.Text, "\n")
	if s.copyAt, err = blockSpan(s.lines, OverrideFile, CopyBegin, CopyEnd, noSpan); err != nil {
		return err
	}
	if s.rulesAt, err = blockSpan(s.lines, OverrideFile, Begin, End, s.copyAt); err != nil {
		return err
	}
	s.override.found = s.copyAt != noSpan || s.rulesAt != noSpan
	outside := ""
	for i, l := range s.lines {
		if i < s.rulesAt[0] || i > s.rulesAt[1] {
			outside += l
		}
	}
	s.Ours = !s.override.Exists || s.copyAt != noSpan || strings.TrimSpace(outside) == ""
	return nil
}

// Found is whether any of the files holds a block of itos's.
func (s *Stealth) Found() bool { return s.rules.found || s.claude.found || s.override.found }

// StaleBlocks are the paths of the rules file and OverrideFile, as read,
// that hold a rules block other than the one this itos writes.
func (s *Stealth) StaleBlocks() []string {
	var stale []string
	for _, t := range []Target{s.rules, s.override} {
		if _, at, _, _ := splitBlockOf(t.Text, t.Path, Begin, End); StaleBlock(at) {
			stale = append(stale, t.Path)
		}
	}
	return stale
}

// CopyStale is whether OverrideFile holds a copy of AGENTS.md that no
// longer matches it.
func (s *Stealth) CopyStale() bool {
	return s.copyAt != noSpan &&
		strings.Join(s.lines[s.copyAt[0]:s.copyAt[1]+1], "") != s.CopyBlock()
}

// refreshed are the rules file and OverrideFile, as read, that hold a rules
// block other than the one this itos writes, each with that block, and
// nothing else, rewritten in this itos's words: OverrideFile's copy of
// AGENTS.md is left as it is.
func (s *Stealth) refreshed() []Edit {
	var edits []Edit
	if before, found, after, err := splitBlockOf(s.rules.Text, s.rules.Path, Begin, End); err == nil &&
		StaleBlock(found) {
		edits = append(edits, Edit{s.rules.Path, port.In(s.dir, s.rules.Disk), before + Block + after, s.rules.CRLF})
	}
	if s.rulesAt == noSpan {
		return edits
	}
	block := strings.Join(s.lines[s.rulesAt[0]:s.rulesAt[1]+1], "")
	if !strings.HasSuffix(block, "\n") {
		block += "\n"
	}
	if block != Block {
		text := strings.Join(s.lines[:s.rulesAt[0]], "") + Block + strings.Join(s.lines[s.rulesAt[1]+1:], "")
		edits = append(edits, Edit{s.override.Path, port.In(s.dir, s.override.Disk), text, s.override.CRLF})
	}
	return edits
}

// CopyBlock is the marked copy of the project's AGENTS.md, "" when it has
// none or an empty one.
func (s *Stealth) CopyBlock() string {
	body := strings.Trim(s.agents, "\n")
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return CopyBegin + "\n\n" + body + "\n\n" + CopyEnd + "\n"
}

// importBlock is CLAUDE.local.md's block: @AGENTS.md when the project has
// one, and the rules, each import a paragraph of its own so a formatter
// never joins them into one line.
func (s *Stealth) importBlock() string {
	paras := []string{Begin}
	if s.hasAgents {
		paras = append(paras, ClaudeImport)
	}
	paras = append(paras, "@"+s.ImportPath, End)
	return strings.Join(paras, "\n\n") + "\n"
}

// Plan sets the text each file would have with the rules block, and gives
// the files in the order they are written.
func (s *Stealth) Plan(block string) []*Target {
	put := func(t *Target, block string) {
		before, found, after, _ := splitBlockOf(t.Text, t.Path, Begin, End)
		if found != nil {
			t.Next = before + block + after
		} else {
			t.Next = AppendBlock(t.Text, block)
		}
	}
	put(&s.rules, block)
	put(&s.claude, s.importBlock())

	copied := s.CopyBlock()
	switch {
	case !s.Ours:
		put(&s.override, block)
	case s.copyAt == noSpan:
		s.override.Next = block
		if copied != "" {
			s.override.Next = copied + "\n" + block
		}
	default:
		var b strings.Builder
		for i := 0; i < len(s.lines); i++ {
			switch {
			case i == s.copyAt[0]:
				b.WriteString(copied)
				i = s.copyAt[1]
			case i == s.rulesAt[0]:
				b.WriteString(block)
				i = s.rulesAt[1]
			default:
				b.WriteString(s.lines[i])
			}
		}
		next := b.String()
		if copied == "" {
			next = strings.TrimLeft(next, "\n")
		}
		if s.rulesAt == noSpan {
			next = AppendBlock(next, block)
		}
		s.override.Next = next
	}
	return []*Target{&s.rules, &s.claude, &s.override}
}
