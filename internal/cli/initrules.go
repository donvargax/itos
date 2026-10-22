package cli

// itos init's rules for agents (slice 59, features/init.feature;
// docs/decisions/0033-itos-s-own-guides-say-how-to-work-with-itos-not-the-plugin.md):
// a marked block of AGENTS.md, between <!-- itos:begin --> and
// <!-- itos:end -->, and a line importing it, @AGENTS.md, in CLAUDE.md, so
// Claude Code reads it too. Since slice 120 (v8, q-45 and q-46) the block
// holds two lines under a heading, the same in every repository: the entry
// line, start with itos go unless your brief names another guide, and the
// one boundary no gate holds yet, staging only your own files. AGENTS.md is
// read by every session for every task, so the commit policy the config
// sets, which the block held before, is printed by itos guide commit
// (internal/engine/guides) for the session about to commit; how to work with
// itos is itos's own guides (itos go, itos guide work).
//
// The block is written formatter-stable: one line per paragraph, a blank
// line around the heading and every marker. A project's Markdown formatter
// then leaves it as written, and a rerun's comparison stays exact.
//
// itos touches only what is inside the markers. An AGENTS.md without them
// gains the block at its end, its text kept; a CLAUDE.md lacking the import
// gains it as its last line, and a missing one is created holding it; one
// that is AGENTS.md itself (a link to it) is left alone, as it reads the
// block already. A file written with CRLF line endings keeps them.
//
// The offer is made as the plugin's is (initplugin.go): --agent-rules writes
// the block, --no-agent-rules declines, a terminal is asked on the first run,
// and anywhere else nothing is written and the report says how. Run again
// where a config is, it never asks: a block other than the one this itos
// writes (an older itos's, say) is reported, naming itos init --agent-rules,
// never counted as missing, and --agent-rules rewrites it; the config does
// not enter into it. Under a stealth config nothing tracked may change, so
// the block goes where git never looks (initrulesstealth.go).
//
// The block, finding it, its staleness and the commit policy are
// internal/engine/agentrules', which status, upgrade and guide share; the
// wrappers below bind it to the real git.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

const (
	agentsFile   = agentrules.AgentsFile
	claudeFile   = agentrules.ClaudeFile
	claudeImport = agentrules.ClaudeImport
	rulesBegin   = agentrules.Begin
	rulesEnd     = agentrules.End
	rulesBlock   = agentrules.Block
)

// rulesFlag is how init's command line answered the offer: given is whether
// it has --agent-rules or --no-agent-rules, write which.
type rulesFlag struct {
	given bool
	write bool
}

// parseRulesFlag reads --agent-rules or --no-agent-rules out of init's
// arguments at i, and gives how many arguments it read, 0 when args[i] is
// neither.
func parseRulesFlag(args []string, i int, f *rulesFlag) (int, error) {
	a := args[i]
	if a != "--agent-rules" && a != "--no-agent-rules" {
		return 0, nil
	}
	write := a == "--agent-rules"
	if f.given && f.write != write {
		return 0, usage("init takes --agent-rules or --no-agent-rules, not both")
	}
	f.given, f.write = true, write
	return 1, nil
}

// rulesOutcome is what came of the offer, init's --json "agent_rules":
// action one of written (the block written or rewritten, or CLAUDE.md given
// its import), current (the block is the one this itos writes, nothing to
// write), stale (a rerun's block other than this itos's, or under a stealth
// config a copy of AGENTS.md that no longer matches it, not rewritten),
// offered (no block, and nobody asked), declined or failed (exit 1 when
// --agent-rules or a terminal asked for it); files each file written, or
// kept as it was, as the starter's are; problem why it failed.
type rulesOutcome struct {
	Action  string        `json:"action"`
	Files   []writtenFile `json:"files"`
	Problem string        `json:"problem,omitempty"`
}

// rulesOffer is the offer's context, as pluginOffer's: the flag, the mode,
// whether this is a rerun where a config was there, whether a terminal may be
// asked, where it says what it did, and where an answer is read from.
type rulesOffer struct {
	flag    rulesFlag
	stealth bool
	rerun   bool
	ask     bool
	log     io.Writer
	answers io.Reader
}

// run makes the offer, and gives its outcome and its exit code: 1 when the
// block asked for could not be written, else 0.
func (r rulesOffer) run() (rulesOutcome, int) {
	say := func(format string, a ...any) { fmt.Fprintf(r.log, format+"\n", a...) }
	none := []writtenFile{}
	switch {
	case r.flag.given && !r.flag.write:
		return rulesOutcome{Action: "declined", Files: none}, 0
	case r.stealth:
		return r.runStealth()
	}
	failed := func(problem string, asked bool) (rulesOutcome, int) {
		if !asked {
			say("%s's itos rules for agents cannot be checked: %s.", agentsFile, problem)
			return rulesOutcome{Action: "failed", Files: none, Problem: problem}, 0
		}
		say("%s is not written: %s.", agentsFile, problem)
		return rulesOutcome{Action: "failed", Files: none, Problem: problem}, ExitPolicy
	}
	text, _, err := agentrules.ReadText(agentsFile)
	if err != nil {
		return failed(err.Error(), r.flag.given)
	}
	before, found, after, err := agentrules.Split(text)
	if err != nil {
		return failed(err.Error(), r.flag.given)
	}
	if !r.flag.given && found == nil {
		answer := ""
		if r.ask && !r.rerun {
			answer = r.question(fmt.Sprintf("Write itos's rules for agents into %s, between itos's markers, "+
				"and import it from %s?", agentsFile, claudeFile))
		}
		switch {
		case answer == "no":
			return rulesOutcome{Action: "declined", Files: none}, 0
		case answer == "" && !r.rerun:
			say("%s", rulesHowTo)
			fallthrough
		case answer == "":
			return rulesOutcome{Action: "offered", Files: none}, 0
		}
	}
	if !r.flag.given && found != nil {
		if !agentrules.StaleBlock(found) {
			return rulesOutcome{Action: "current", Files: none}, 0
		}
		say("%s", staleRulesLine(false))
		return rulesOutcome{Action: "stale", Files: none}, 0
	}

	files, err := writeRules(text, before, found, after, rulesBlock)
	if err != nil {
		return failed(err.Error(), true)
	}
	changed := slices.ContainsFunc(files, func(f writtenFile) bool { return f.Action != "kept" })
	if !changed {
		say("%s holds this itos's rules for agents already, and %s imports them.", agentsFile, claudeFile)
		return rulesOutcome{Action: "current", Files: files}, 0
	}
	for _, f := range files {
		if f.Action != "kept" {
			say("%s %s", f.Action, f.Path)
		}
	}
	say("%s holds itos's rules for agents, between %s and %s, which itos init --agent-rules rewrites; %s imports it (%s).",
		agentsFile, rulesBegin, rulesEnd, claudeFile, claudeImport)
	return rulesOutcome{Action: "written", Files: files}, 0
}

// staleRulesLine is agentrules.StaleLine: what a rerun says of a stale
// block, and itos status (status.go, slice 124) with it.
func staleRulesLine(stealth bool) string { return agentrules.StaleLine(stealth) }

// staleRules is agentrules.Stale over the real git: the files holding a
// block of the rules other than this itos's (status.go, slice 124).
func staleRules(stealth bool) []string {
	return agentrules.Stale(context.Background(), vcs.Git{}, stealth)
}

// rulesEdit is an agentrules.Edit as refreshedRules gives it.
type rulesEdit struct {
	path, disk, text string
	crlf             bool
}

// refreshedRules is agentrules.Refreshed over the real git: each stale
// block rewritten in this itos's words (upgrade.go, slice 126).
func refreshedRules(stealth bool) []rulesEdit {
	var edits []rulesEdit
	for _, e := range agentrules.Refreshed(context.Background(), vcs.Git{}, stealth) {
		edits = append(edits, rulesEdit{e.Path, e.Disk, e.Text, e.CRLF})
	}
	return edits
}

// rulesHowTo is what the offer says where nobody was asked.
var rulesHowTo = "itos's rules for agents are not written: itos init --agent-rules writes them " +
	"into " + agentsFile + ", between " + rulesBegin + " and " + rulesEnd + ", and imports it from " + claudeFile +
	" (" + claudeImport + "); --no-agent-rules declines."

// question asks the terminal whether to write the block, until it answers:
// "yes", "no", or "" when its input ends first.
func (r rulesOffer) question(prompt string) string {
	lines := bufio.NewReader(r.answers)
	for {
		fmt.Fprintf(r.log, "%s [Y/n]: ", prompt)
		line, err := lines.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		switch {
		case answer == "" && err != nil:
			fmt.Fprintln(r.log)
			return ""
		case answer == "" || answer == "y" || answer == "yes":
			return "yes"
		case answer == "n" || answer == "no":
			return "no"
		}
		fmt.Fprintln(r.log, "Answer yes or no.")
		if err != nil {
			return ""
		}
	}
}

// writeRules writes the block into AGENTS.md, in place of the one found or
// at its end, and the import into CLAUDE.md, and gives what it did to each.
func writeRules(text, before string, found *string, after, block string) ([]writtenFile, error) {
	_, crlf, err := agentrules.ReadText(agentsFile)
	if err != nil {
		return nil, err
	}
	var files []writtenFile
	agents := writtenFile{agentsFile, "kept"}
	switch {
	case found != nil && *found == block:
	case found != nil:
		agents.Action = "updated"
		if err := agentrules.WriteText(agentsFile, before+block+after, crlf); err != nil {
			return nil, err
		}
	case text == "":
		agents.Action = "wrote"
		if exists(agentsFile) {
			agents.Action = "updated"
		}
		if err := agentrules.WriteText(agentsFile, block, crlf); err != nil {
			return nil, err
		}
	default:
		agents.Action = "updated"
		if err := agentrules.WriteText(agentsFile, agentrules.AppendBlock(text, block), crlf); err != nil {
			return nil, err
		}
	}
	files = append(files, agents)

	claude := writtenFile{claudeFile, "kept"}
	if !sameFile(claudeFile, agentsFile) {
		text, crlf, err := agentrules.ReadText(claudeFile)
		if err != nil {
			return nil, err
		}
		imports := slices.ContainsFunc(strings.Split(text, "\n"), func(l string) bool {
			return strings.TrimSpace(l) == claudeImport
		})
		if !imports {
			claude.Action = "updated"
			if !exists(claudeFile) {
				claude.Action = "wrote"
			}
			if text != "" && !strings.HasSuffix(text, "\n") {
				text += "\n"
			}
			if err := agentrules.WriteText(claudeFile, text+claudeImport+"\n", crlf); err != nil {
				return nil, err
			}
		}
	}
	return append(files, claude), nil
}

// sameFile is whether two paths are one file, a link to the other say.
func sameFile(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

// prose is agentrules.Prose: words joined as a list in prose.
func prose(words []string, join string) string { return agentrules.Prose(words, join) }
