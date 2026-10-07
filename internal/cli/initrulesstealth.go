package cli

// itos init's rules for agents under a stealth config (slice 80,
// features/init.feature), the stealth half of initrules.go. Nothing the
// project tracks may change, so the rules go where git never looks: the
// block the config decides, rulesBlock's, to <git common dir>/itos/AGENTS.md,
// beside the stealth config and shared by every worktree. Each agent then
// gets a file of its own at the worktree's top, listed in the git folder's
// info/exclude:
//
//   - Claude Code, CLAUDE.local.md, whose block imports that file, by the path
//     git names the common dir by (relative in the main worktree, absolute in
//     a linked one, so it resolves from that worktree), and @AGENTS.md first
//     when the project has one, since a CLAUDE.local.md stops Claude Code
//     falling back to AGENTS.md.
//   - Codex, which has no imports and reads one file a folder,
//     AGENTS.override.md winning over AGENTS.md, an AGENTS.override.md holding
//     a copy of the project's AGENTS.md between <!-- itos:agents-md:begin -->
//     and <!-- itos:agents-md:end -->, then the rules block; with no AGENTS.md,
//     the rules block alone. Never an AGENTS.md: an untracked one would make
//     git refuse the pull the day the project adds its own.
//
// A CLAUDE.local.md or an AGENTS.override.md already there is the person's:
// it gains only the block, at its end, never the copy, its own text kept. An
// AGENTS.override.md is itos's when it holds the copy's markers, or nothing
// outside the rules block. The offer is initrules.go's; a rerun with no
// --agent-rules reports a copy that no longer matches AGENTS.md, or rules that
// no longer match the config, and --agent-rules rewrites only between the
// markers.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/value"
)

const (
	// claudeLocalFile is Claude Code's file of one person's, never committed.
	claudeLocalFile = "CLAUDE.local.md"
	// overrideFile is Codex's, which it reads in place of AGENTS.md.
	overrideFile = "AGENTS.override.md"
	// copyBegin and copyEnd are the lines the copy of AGENTS.md in
	// overrideFile is written between.
	copyBegin = "<!-- itos:agents-md:begin -->"
	copyEnd   = "<!-- itos:agents-md:end -->"
)

// stealthTarget is a file the stealth rules are written to: its path as said
// (slashes) and on disk, its text (LF), whether it is there and used CRLF,
// whether it holds a block of itos's, whether it is at the worktree's top
// (kept out of git by info/exclude), and the text itos would give it.
type stealthTarget struct {
	path, disk string
	text       string
	exists     bool
	crlf       bool
	found      bool
	top        bool
	next       string
}

// stealthRules is what the stealth rules are made from, as read: the three
// files and the project's AGENTS.md (LF, "" when it is not there).
type stealthRules struct {
	rules, claude, override stealthTarget
	agents                  string
	hasAgents               bool
	importPath              string
	// The spans of overrideFile's lines (strings.SplitAfter) that hold the
	// copy and the rules block, noSpan for none, and whether the file is
	// itos's own, so that it holds the copy.
	lines           []string
	copyAt, rulesAt [2]int
	ours            bool
}

// readStealthRules reads the files of the stealth rules, from the
// worktree's top; markers out of order are an error, as in AGENTS.md.
func readStealthRules() (*stealthRules, error) {
	common, err := git.Read("rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	disk := filepath.Join(value.Trim(common), config.StealthFolder, agentsFile)
	s := &stealthRules{importPath: filepath.ToSlash(disk)}
	read := func(t *stealthTarget, path, disk string, top bool) error {
		t.path, t.disk, t.top = path, disk, top
		var err error
		t.text, t.crlf, err = readText(disk)
		t.exists = exists(disk)
		return err
	}
	if err := read(&s.rules, s.importPath, disk, false); err != nil {
		return nil, err
	}
	if err := read(&s.claude, claudeLocalFile, claudeLocalFile, true); err != nil {
		return nil, err
	}
	if err := read(&s.override, overrideFile, overrideFile, true); err != nil {
		return nil, err
	}
	s.agents, _, err = readText(agentsFile)
	if err != nil {
		return nil, err
	}
	s.hasAgents = exists(agentsFile)
	return s, s.parse()
}

// parse finds itos's blocks in the files as read.
func (s *stealthRules) parse() error {
	var err error
	for _, t := range []*stealthTarget{&s.rules, &s.claude} {
		_, found, _, err := splitBlockOf(t.text, t.path, rulesBegin, rulesEnd)
		if err != nil {
			return err
		}
		t.found = found != nil
	}
	s.lines = strings.SplitAfter(s.override.text, "\n")
	if s.copyAt, err = blockSpan(s.lines, overrideFile, copyBegin, copyEnd, noSpan); err != nil {
		return err
	}
	if s.rulesAt, err = blockSpan(s.lines, overrideFile, rulesBegin, rulesEnd, s.copyAt); err != nil {
		return err
	}
	s.override.found = s.copyAt != noSpan || s.rulesAt != noSpan
	outside := ""
	for i, l := range s.lines {
		if i < s.rulesAt[0] || i > s.rulesAt[1] {
			outside += l
		}
	}
	s.ours = !s.override.exists || s.copyAt != noSpan || strings.TrimSpace(outside) == ""
	return nil
}

// found is whether any of the files holds a block of itos's.
func (s *stealthRules) found() bool { return s.rules.found || s.claude.found || s.override.found }

// copyBlock is the marked copy of the project's AGENTS.md, "" when it has
// none or an empty one.
func (s *stealthRules) copyBlock() string {
	body := strings.Trim(s.agents, "\n")
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return copyBegin + "\n\n" + body + "\n\n" + copyEnd + "\n"
}

// importBlock is CLAUDE.local.md's block: @AGENTS.md when the project has
// one, and the rules, each import a paragraph of its own so a formatter
// never joins them into one line.
func (s *stealthRules) importBlock() string {
	paras := []string{rulesBegin}
	if s.hasAgents {
		paras = append(paras, claudeImport)
	}
	paras = append(paras, "@"+s.importPath, rulesEnd)
	return strings.Join(paras, "\n\n") + "\n"
}

// plan sets the text each file would have with the rules block, and gives
// the files in the order they are written.
func (s *stealthRules) plan(block string) []*stealthTarget {
	put := func(t *stealthTarget, block string) {
		before, found, after, _ := splitBlockOf(t.text, t.path, rulesBegin, rulesEnd)
		if found != nil {
			t.next = before + block + after
		} else {
			t.next = appendBlock(t.text, block)
		}
	}
	put(&s.rules, block)
	put(&s.claude, s.importBlock())

	copied := s.copyBlock()
	switch {
	case !s.ours:
		put(&s.override, block)
	case s.copyAt == noSpan:
		s.override.next = block
		if copied != "" {
			s.override.next = copied + "\n" + block
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
			next = appendBlock(next, block)
		}
		s.override.next = next
	}
	return []*stealthTarget{&s.rules, &s.claude, &s.override}
}

// stealthSource names the stealth config in the rules block the same way
// from every worktree, whose paths to it differ, so that the block they
// share is the same from each.
var stealthSource = "the stealth config, " + code(config.StealthFolder+"/itos.yaml") + " in the git common dir"

// stealthData names a file of the stealth config's own data, which it reads
// beside itself in dir, by its path from there, the same from every
// worktree; one elsewhere by its path.
func stealthData(dir string) func(string) string {
	return func(p string) string {
		rel, err := filepath.Rel(dir, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return code(filepath.ToSlash(p))
		}
		return code(filepath.ToSlash(rel)) + " beside the config"
	}
}

// howTo is what the offer says under a stealth config where nobody
// was asked.
func (s *stealthRules) howTo() string {
	return "itos's rules for agents are not written: itos init --agent-rules writes what the config decides into " +
		s.importPath + ", imports it from " + claudeLocalFile + " for Claude Code and writes it into " + overrideFile +
		" for Codex, both kept out of git by its info/exclude; --no-agent-rules declines."
}

// runStealth is run under a stealth config.
func (r rulesOffer) runStealth() (rulesOutcome, int) {
	say := func(format string, a ...any) { fmt.Fprintf(r.log, format+"\n", a...) }
	none := []writtenFile{}
	failed := func(problem string, asked bool) (rulesOutcome, int) {
		if !asked {
			say("The itos rules for agents cannot be checked: %s.", problem)
			return rulesOutcome{Action: "failed", Files: none, Problem: problem}, 0
		}
		say("The itos rules for agents are not written: %s.", problem)
		return rulesOutcome{Action: "failed", Files: none, Problem: problem}, ExitPolicy
	}
	s, err := readStealthRules()
	if err != nil {
		return failed(err.Error(), r.flag.given)
	}
	found := s.found()
	if !r.flag.given && !found {
		answer := ""
		if r.ask && !r.rerun {
			answer = r.question(fmt.Sprintf("Write the rules the config decides into %s, imported from %s and written "+
				"into %s, both kept out of git?", s.importPath, claudeLocalFile, overrideFile))
		}
		switch {
		case answer == "no":
			return rulesOutcome{Action: "declined", Files: none}, 0
		case answer == "" && !r.rerun:
			say("%s", s.howTo())
			fallthrough
		case answer == "":
			return rulesOutcome{Action: "offered", Files: none}, 0
		}
	}
	cfg, err := config.Load(r.file)
	if err != nil {
		return failed("the config "+r.file+" does not load, so there are no rules to write", r.flag.given || !found)
	}
	block := rulesBlock(cfg, stealthSource, stealthData(filepath.Dir(r.file)))
	targets := s.plan(block)

	if !r.flag.given {
		var pending []string
		for _, t := range targets {
			if t.next != t.text {
				pending = append(pending, t.path)
			}
		}
		if len(pending) == 0 {
			return rulesOutcome{Action: "current", Files: none}, 0
		}
		rulesStale := false
		for _, t := range []stealthTarget{s.rules, s.override} {
			if _, at, _, _ := splitBlockOf(t.text, t.path, rulesBegin, rulesEnd); at != nil && *at != block {
				rulesStale = true
			}
		}
		copyStale := s.copyAt != noSpan &&
			strings.Join(s.lines[s.copyAt[0]:s.copyAt[1]+1], "") != s.copyBlock()
		switch {
		case rulesStale:
			say("The itos rules for agents no longer match %s: itos init --agent-rules rewrites them, only between the markers.",
				r.file)
		case !copyStale:
			say("The itos rules for agents are not all in place here (%s): itos init --agent-rules writes them, "+
				"only between the markers.", strings.Join(pending, ", "))
		}
		if copyStale {
			say("%s's copy of %s no longer matches it: itos init --agent-rules rewrites it, only between the markers.",
				overrideFile, agentsFile)
		}
		return rulesOutcome{Action: "stale", Files: none}, 0
	}

	for _, t := range targets {
		if t.top && t.next != t.text {
			if tracked, _ := git.Output("ls-files", "--", t.disk); value.Trim(tracked) != "" {
				return failed(t.path+" is tracked by the project, and under a stealth config itos changes nothing tracked", true)
			}
		}
	}
	files := []writtenFile{}
	for _, t := range targets {
		action := "kept"
		if t.next != t.text {
			action = "updated"
			if !t.exists {
				action = "wrote"
			}
			if err := os.MkdirAll(filepath.Dir(t.disk), 0o777); err != nil {
				return failed(err.Error(), true)
			}
			if err := writeText(t.disk, t.next, t.crlf); err != nil {
				return failed(err.Error(), true)
			}
		}
		files = append(files, writtenFile{t.path, action})
	}
	var excluded []string
	for _, t := range targets {
		if !t.top {
			continue
		}
		if listed, err := excludeIfShown(t.path); err != nil {
			say("Could not list %s in the git folder's info/exclude: %v", t.path, err)
		} else if listed {
			excluded = append(excluded, t.path)
		}
	}
	changed := slices.ContainsFunc(files, func(f writtenFile) bool { return f.Action != "kept" })
	for _, f := range files {
		if f.Action != "kept" {
			say("%s %s", f.Action, f.Path)
		}
	}
	if len(excluded) > 0 {
		say("Listed %s in the git folder's info/exclude, so git status stays clean.", prose(excluded, "and"))
	}
	if !changed {
		say("The itos rules for agents match %s already, in %s, %s and %s.", r.file, s.importPath, claudeLocalFile, overrideFile)
		return rulesOutcome{Action: "current", Files: files}, 0
	}
	copied := ""
	if s.ours && s.copyBlock() != "" {
		copied = ", after a copy of " + agentsFile + " between " + copyBegin + " and " + copyEnd
	}
	say("%s holds the rules %s decides, between %s and %s, which itos init --agent-rules rewrites; %s imports it for "+
		"Claude Code, and %s holds them for Codex%s.", s.importPath, r.file, rulesBegin, rulesEnd, claudeLocalFile,
		overrideFile, copied)
	return rulesOutcome{Action: "written", Files: files}, 0
}
