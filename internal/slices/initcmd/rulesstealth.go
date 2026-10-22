package initcmd

// itos init's rules for agents under a stealth config (slice 80,
// features/init.feature), the stealth half of rules.go. Nothing the
// project tracks may change, so the rules go where git never looks: the
// block, rulesBlock, to <git common dir>/itos/AGENTS.md,
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
// outside the rules block. The offer is rules.go's; a rerun with no
// --agent-rules reports a copy that no longer matches AGENTS.md, or a block
// other than the one this itos writes, and --agent-rules rewrites only
// between the markers.
//
// The files' reading, their staleness and each one's next text are
// internal/engine/agentrules' (Stealth); writing them is init's, here.

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/agentrules"
)

const (
	claudeLocalFile = agentrules.ClaudeLocalFile
	overrideFile    = agentrules.OverrideFile
	copyBegin       = agentrules.CopyBegin
	copyEnd         = agentrules.CopyEnd
)

// stealthHowTo is what the offer says under a stealth config where nobody
// was asked.
func stealthHowTo(s *agentrules.Stealth) string {
	return "itos's rules for agents are not written: itos init --agent-rules writes them into " +
		s.ImportPath + ", imports it from " + claudeLocalFile + " for Claude Code and writes it into " + overrideFile +
		" for Codex, both kept out of git by its info/exclude; --no-agent-rules declines."
}

// runStealth is run under a stealth config.
func (r rulesOffer) runStealth(ctx context.Context) (rulesOutcome, int) {
	say := func(format string, a ...any) { fmt.Fprintf(r.log, format+"\n", a...) }
	none := []writtenFile{}
	failed := func(problem string, asked bool) (rulesOutcome, int) {
		if !asked {
			say("The itos rules for agents cannot be checked: %s.", problem)
			return rulesOutcome{Action: "failed", Files: none, Problem: problem}, 0
		}
		say("The itos rules for agents are not written: %s.", problem)
		return rulesOutcome{Action: "failed", Files: none, Problem: problem}, exitPolicy
	}
	s, err := agentrules.ReadStealth(ctx, r.ports)
	if err != nil {
		return failed(err.Error(), r.flag.Given)
	}
	found := s.Found()
	if !r.flag.Given && !found {
		answer := ""
		if r.ask && !r.rerun {
			answer = r.question(fmt.Sprintf("Write itos's rules for agents into %s, imported from %s and written "+
				"into %s, both kept out of git?", s.ImportPath, claudeLocalFile, overrideFile))
		}
		switch {
		case answer == "no":
			return rulesOutcome{Action: "declined", Files: none}, 0
		case answer == "" && !r.rerun:
			say("%s", stealthHowTo(s))
			fallthrough
		case answer == "":
			return rulesOutcome{Action: "offered", Files: none}, 0
		}
	}
	targets := s.Plan(rulesBlock)

	if !r.flag.Given {
		var pending []string
		for _, t := range targets {
			if t.Next != t.Text {
				pending = append(pending, t.Path)
			}
		}
		if len(pending) == 0 {
			return rulesOutcome{Action: "current", Files: none}, 0
		}
		rulesStale := len(s.StaleBlocks()) > 0
		copyStale := s.CopyStale()
		switch {
		case rulesStale:
			say("%s", agentrules.StaleLine(true))
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
		if t.Top && t.Next != t.Text {
			if r.ports.Tracked(ctx, t.Disk) {
				return failed(t.Path+" is tracked by the project, and under a stealth config itos changes nothing tracked", true)
			}
		}
	}
	files := []writtenFile{}
	for _, t := range targets {
		action := "kept"
		if t.Next != t.Text {
			action = "updated"
			if !t.Exists {
				action = "wrote"
			}
			if err := r.ports.MkdirAll(ctx, filepath.Dir(t.Disk), 0o777); err != nil {
				return failed(err.Error(), true)
			}
			if err := r.ports.WriteText(ctx, t.Disk, t.Next, t.CRLF); err != nil {
				return failed(err.Error(), true)
			}
		}
		files = append(files, writtenFile{t.Path, action})
	}
	var excluded []string
	for _, t := range targets {
		if !t.Top {
			continue
		}
		if listed, err := excludeIfShown(ctx, r.ports, t.Path); err != nil {
			say("Could not list %s in the git folder's info/exclude: %v", t.Path, err)
		} else if listed {
			excluded = append(excluded, t.Path)
		}
	}
	changed := slices.ContainsFunc(files, func(f writtenFile) bool { return f.Action != "kept" })
	for _, f := range files {
		if f.Action != "kept" {
			say("%s %s", f.Action, f.Path)
		}
	}
	if len(excluded) > 0 {
		say("Listed %s in the git folder's info/exclude, so git status stays clean.", agentrules.Prose(excluded, "and"))
	}
	if !changed {
		say("The itos rules for agents are in place already, in %s, %s and %s.", s.ImportPath, claudeLocalFile, overrideFile)
		return rulesOutcome{Action: "current", Files: files}, 0
	}
	copied := ""
	if s.Ours && s.CopyBlock() != "" {
		copied = ", after a copy of " + agentsFile + " between " + copyBegin + " and " + copyEnd
	}
	say("%s holds itos's rules for agents, between %s and %s, which itos init --agent-rules rewrites; %s imports it for "+
		"Claude Code, and %s holds them for Codex%s.", s.ImportPath, rulesBegin, rulesEnd, claudeLocalFile,
		overrideFile, copied)
	return rulesOutcome{Action: "written", Files: files}, 0
}
