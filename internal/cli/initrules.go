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
// (commitPolicy, guide.go) for the session about to commit; how to work with
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

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
)

const (
	// agentsFile is the file agents read their instructions from.
	agentsFile = "AGENTS.md"
	// claudeFile is Claude Code's, which reads AGENTS.md through claudeImport.
	claudeFile   = "CLAUDE.md"
	claudeImport = "@AGENTS.md"
	// rulesBegin and rulesEnd are the lines the block is written between.
	rulesBegin = "<!-- itos:begin -->"
	rulesEnd   = "<!-- itos:end -->"
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
	text, _, err := readText(agentsFile)
	if err != nil {
		return failed(err.Error(), r.flag.given)
	}
	before, found, after, err := splitBlock(text)
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
		if !staleBlock(found) {
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

// staleBlock is whether a block found (nil for none) is other than the one
// this itos writes: an older itos's, say, or one edited by hand.
func staleBlock(found *string) bool { return found != nil && *found != rulesBlock }

// staleRulesLine is what a rerun says of a stale block, and itos status
// (status.go, slice 124) with it: AGENTS.md's, or under a stealth config
// that of the files init writes there.
func staleRulesLine(stealth bool) string {
	whose := agentsFile + "'s itos rules are"
	if stealth {
		whose = "The itos rules for agents are"
	}
	return whose + " not the ones this itos writes: itos init --agent-rules rewrites them, only between the markers."
}

// staleRules are the files holding a block of itos's rules for agents other
// than the one this itos writes, as init's rerun decides it without
// --agent-rules: AGENTS.md, or under a stealth config the copies init
// writes there (initrulesstealth.go). It writes nothing and never fails: no
// block is none (the rules are offered, never required), and a file that
// cannot be read, or whose markers cannot be placed, is none too, left for
// init to report.
func staleRules(stealth bool) []string {
	if stealth {
		s, err := readStealthRules()
		if err != nil {
			return nil
		}
		return s.staleBlocks()
	}
	text, _, err := readText(agentsFile)
	if err != nil {
		return nil
	}
	if _, found, _, err := splitBlock(text); err != nil || !staleBlock(found) {
		return nil
	}
	return []string{agentsFile}
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

// readText is a file's text with its line endings made LF, whether it used
// CRLF, and "" for a file that is not there.
func readText(p string) (string, bool, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	text := string(data)
	crlf := strings.Contains(text, "\r\n")
	return strings.ReplaceAll(text, "\r\n", "\n"), crlf, nil
}

// writeText writes LF text, with CRLF line endings when crlf.
func writeText(p, text string, crlf bool) error {
	if crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return os.WriteFile(p, []byte(text), 0o666)
}

// splitBlock is AGENTS.md's text (LF) cut around its block: the text before
// the begin marker's line, the block from that line through the end
// marker's (nil when the file has neither marker), and the text after it.
// A file with one marker and not the other, or more than one block, is an
// error: which text is itos's would be a guess.
func splitBlock(text string) (string, *string, string, error) {
	return splitBlockOf(text, agentsFile, rulesBegin, rulesEnd)
}

// splitBlockOf is splitBlock for the file name's text and a block between
// the lines begin and end.
func splitBlockOf(text, name, begin, end string) (string, *string, string, error) {
	lines := strings.SplitAfter(text, "\n")
	at, err := blockSpan(lines, name, begin, end, noSpan)
	if err != nil || at == noSpan {
		return text, nil, "", err
	}
	block := strings.Join(lines[at[0]:at[1]+1], "")
	if !strings.HasSuffix(block, "\n") {
		block += "\n"
	}
	return strings.Join(lines[:at[0]], ""), &block, strings.Join(lines[at[1]+1:], ""), nil
}

// noSpan is the span of no lines.
var noSpan = [2]int{-1, -1}

// blockSpan is the indexes of the lines begin and end that mark a block in
// the file name's lines, its lines from skip[0] through skip[1] not looked
// at (another block's, say); noSpan when it has neither marker. One marker
// and not the other, or more than one block, is an error.
func blockSpan(lines []string, name, begin, end string, skip [2]int) ([2]int, error) {
	find := func(marker string, from int) int {
		for i := from; i < len(lines); i++ {
			if (i < skip[0] || i > skip[1]) && strings.TrimSpace(lines[i]) == marker {
				return i
			}
		}
		return -1
	}
	b, e := find(begin, 0), find(end, 0)
	switch {
	case b < 0 && e < 0:
		return noSpan, nil
	case e < 0:
		return noSpan, refusedMarkers("%s has a %s line with no %s line after it; put the markers right by hand",
			name, begin, end)
	case b < 0 || e < b:
		return noSpan, refusedMarkers("%s has a %s line with no %s line before it; put the markers right by hand",
			name, end, begin)
	}
	if find(begin, e+1) >= 0 || find(end, e+1) >= 0 {
		return noSpan, refusedMarkers("%s has more than one block between %s and %s; leave one", name, begin, end)
	}
	return [2]int{b, e}, nil
}

// refusedMarkers is a marked block itos cannot place in a file it does not
// own, which the person puts right: kind.Usage (exit 2).
func refusedMarkers(format string, a ...any) error {
	return kind.Wrap(kind.Usage, fmt.Errorf(format, a...))
}

// appendBlock is text with the block at its end, after a blank line.
func appendBlock(text, block string) string {
	if text == "" {
		return block
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if strings.TrimSpace(text[strings.LastIndex(strings.TrimSuffix(text, "\n"), "\n")+1:]) != "" {
		text += "\n"
	}
	return text + block
}

// writeRules writes the block into AGENTS.md, in place of the one found or
// at its end, and the import into CLAUDE.md, and gives what it did to each.
func writeRules(text, before string, found *string, after, block string) ([]writtenFile, error) {
	_, crlf, err := readText(agentsFile)
	if err != nil {
		return nil, err
	}
	var files []writtenFile
	agents := writtenFile{agentsFile, "kept"}
	switch {
	case found != nil && *found == block:
	case found != nil:
		agents.Action = "updated"
		if err := writeText(agentsFile, before+block+after, crlf); err != nil {
			return nil, err
		}
	case text == "":
		agents.Action = "wrote"
		if exists(agentsFile) {
			agents.Action = "updated"
		}
		if err := writeText(agentsFile, block, crlf); err != nil {
			return nil, err
		}
	default:
		agents.Action = "updated"
		if err := writeText(agentsFile, appendBlock(text, block), crlf); err != nil {
			return nil, err
		}
	}
	files = append(files, agents)

	claude := writtenFile{claudeFile, "kept"}
	if !sameFile(claudeFile, agentsFile) {
		text, crlf, err := readText(claudeFile)
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
			if err := writeText(claudeFile, text+claudeImport+"\n", crlf); err != nil {
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

// code is a value from the config as a Markdown code span, on one line (a
// line break made a space), fenced with more backticks than it holds.
func code(s string) string {
	s = strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return fence + " " + s + " " + fence
	}
	return fence + s + fence
}

// codes is values as code spans, joined as a list in prose: a, b and c (or
// "or" for join).
func codes(values []string, join string) string {
	spans := make([]string, len(values))
	for i, v := range values {
		spans[i] = code(v)
	}
	return prose(spans, join)
}

// prose is words joined as a list in prose: a, b and c (or "or" for join).
func prose(words []string, join string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " " + join + " " + words[len(words)-1]
}

// typesOf are the commit types a footer's required_for or in_place_of
// names: its list, every type for all, none for nil.
func typesOf(cfg *config.Loaded, t *config.Types) []string {
	switch {
	case t == nil:
		return nil
	case t.IsList:
		return t.List
	case t.Word == "all":
		return cfg.Commits.Types
	}
	return nil
}

// rulesBlock is the block, its markers included, LF: the entry line and the
// staging boundary, the same in every repository.
const rulesBlock = rulesBegin + "\n\n" +
	"## itos\n\n" +
	"Start with `itos go` unless your brief names another guide.\n\n" +
	"Stage only your own files, by explicit path: untracked files you did not make are someone else's work in progress.\n\n" +
	rulesEnd + "\n"

// commitPolicy are the sections on commits the config sets, as itos guide
// commit prints them (guide.go, slice 119): the commit types, the footers
// each type needs and the paths each may touch; data is the path of a file
// of itos's own data the config names, in Markdown.
func commitPolicy(cfg *config.Loaded, data func(string) string) []string {
	var paras []string
	add := func(lines ...string) { paras = append(paras, strings.Join(lines, "\n")) }
	c := cfg.Commits
	add("### Commit types")
	header := ""
	switch {
	case c.HeaderLint.Builtin():
		header = "The header is held to commitlint's config-conventional rules by itos's own lint: " +
			code("type: subject") + " or " + code("type(scope): subject") + "."
	case c.HeaderLint.Hook != nil:
		header = "The header is linted by " + code(*c.HeaderLint.Hook) + "."
	case c.HeaderLint.Stdin != nil:
		header = "The header is linted by " + code(*c.HeaderLint.Stdin) + "."
	}
	if len(c.Types) > 0 {
		add(strings.TrimSpace("A commit's type is one of " + codes(c.Types, "or") + ". " + header))
	} else if header != "" {
		add(header)
	}

	if len(c.Footers.Keys) > 0 {
		add("### Footers")
		var items []string
		for _, key := range c.Footers.Keys {
			f := c.Footers.Values[key]
			what := "is free text"
			switch {
			case f.Source.IsName && f.Source.Name == "ledger":
				what = "names tasks of the ledger, " + data(cfg.Ledger.Files)
			case f.Registry():
				what = "names items of the work registry, " + data(cfg.Work.Registry)
			case f.Source.Tests != "":
				what = "names " + code(f.Source.Tests) + " tests by their IDs"
				if f.Live() {
					wip := "@wip"
					if k, ok := cfg.Tests.Get(f.Source.Tests); ok && k.WipTag != "" {
						wip = k.WipTag
					}
					what += ", live ones only (not " + code(wip) + ")"
				}
			}
			items = append(items, "- "+code(key+":")+" "+what+".")
		}
		add(items...)
		add("The footers each type needs:")
		items = nil
		for _, typ := range c.Types {
			var needs []string
			for _, key := range c.Footers.Keys {
				if !slices.Contains(typesOf(cfg, c.Footers.Values[key].RequiredFor), typ) {
					continue
				}
				alternatives := []string{key + ":"}
				for _, other := range c.Footers.Keys {
					in, ok := c.Footers.Values[other].InPlaceOf.Get(key)
					if ok && slices.Contains(typesOf(cfg, &in), typ) {
						alternatives = append(alternatives, other+":")
					}
				}
				needs = append(needs, codes(alternatives, "or"))
			}
			need := "none"
			if len(needs) > 0 {
				need = prose(needs, "and")
			}
			items = append(items, "- "+code(typ)+": "+need+".")
		}
		if len(items) > 0 {
			add(items...)
		}
	}

	add("### The paths each type may touch")
	var items []string
	for _, typ := range c.Scopes.Keys {
		s := c.Scopes.Values[typ]
		var rules []string
		if len(s.Only) > 0 {
			rules = append(rules, "may touch only "+codes(s.Only, "and"))
		}
		if len(s.Never) > 0 {
			never := "may not touch " + codes(s.Never, "or")
			if len(s.Except) > 0 {
				never += ", except " + codes(s.Except, "or")
			}
			rules = append(rules, never)
		}
		if len(s.MustTouch) > 0 {
			rules = append(rules, "must touch at least one of "+codes(s.MustTouch, "or"))
		}
		if len(rules) == 0 {
			rules = []string{"may touch anything"}
		}
		items = append(items, "- "+code(typ)+" "+strings.Join(rules, "; it ")+".")
	}
	var free []string
	for _, typ := range c.Types {
		if _, ok := c.Scopes.Get(typ); !ok {
			free = append(free, typ)
		}
	}
	switch {
	case len(items) == 0:
		add("Every type may touch anything.")
	case len(free) > 0:
		items = append(items, "- "+codes(free, "and")+" may touch anything.")
	}
	if len(items) > 0 {
		add(items...)
	}
	return paras
}
