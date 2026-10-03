// Package message is one commit message through itos's commit rules
// (tools/itos/footers.ts and commit.ts): the footer rules, the one footer
// reader, and the header lint beside them, built in (header.go) or a
// delegate. The footer rules are itos's and run always, after the header
// lint, whatever it is: the header lint judges the header and body, and one
// without the footer rules never skips them. Both report before the exit, so
// a header problem does not hide a footer one.
//
// A footer is `<Key>: <id> <id>, …` at the start of a line, and may repeat
// over several lines to stay within the line length limit. Per key of
// commits.footers: its source (the ledger, or a kind's named tests as its
// adapter lists them), strip_prefix, required_for, validate_for,
// must_be_live, and read_at: `commit` reads the IDs that exist at the commit
// being checked (the staged tree by default, the commit Reading.At names),
// so a later commit that sets a test back to wip or drops a task does not
// fail an older one, but for the stealth config's ledger, in no commit, read
// in the working tree; `worktree`, or none, the working tree; since, a commit
// that verify leaves out of required_for with its ancestors (Reading.Made).
//
// A footer whose source is text carries no IDs: `<Key>: <text>`, each line
// one footer, the text whatever a consumer of the project must do, or the
// word none. Its rule is only that it is there (required_for) and not empty
// (validate_for); Texts reads it, and a range's are gathered by
// `itos commit footers`.
package message

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/ledger"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/source"
	"github.com/donvargax/itos/v2/internal/tests"
	"github.com/donvargax/itos/v2/internal/value"
)

// Reading is where the footer rules read a read_at: commit footer's IDs: At
// is the commit, "" for the staged tree; Warn is where a footer read against
// the working tree instead says so. Made is whether the message is At's own,
// a commit already made, as verify reads it: a footer's since then leaves
// that commit and its ancestors out of its required_for. The commit-msg hook
// and check-message judge the commit being made, which every rule holds.
type Reading struct {
	At   string
	Made bool
	Warn io.Writer
}

// tree is the tree a footer's IDs are read at: a commit, "index", or "" for
// the working tree.
func (r Reading) tree(f config.Footer) string {
	if f.ReadAt == nil || *f.ReadAt != "commit" {
		return ""
	}
	if r.At != "" {
		return r.At
	}
	return "index"
}

var (
	idSeparator = regexp.MustCompile("[" + value.SpaceChars + ",]+")
	typeWord    = regexp.MustCompile(`^(\w+)`)
	idHint      = regexp.MustCompile(`^[\w-]*`)
)

// IDs are the IDs one footer gives in a text (a message, or a range's
// messages), in order, strip taken off the front of each, not deduplicated.
func IDs(text, key, strip string) []string {
	var ids []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		for _, id := range idSeparator.Split(line[len(key)+1:], -1) {
			if id == "" {
				continue
			}
			if strip != "" && strings.HasPrefix(id, strip) {
				id = id[len(strip):]
			}
			ids = append(ids, id)
		}
	}
	return ids
}

// Texts are the texts one free-text footer gives in a message, a line each,
// in order, trimmed, the empty ones kept as "".
func Texts(message, key string) []string {
	var texts []string
	for _, line := range strings.Split(message, "\n") {
		if strings.HasPrefix(line, key+":") {
			texts = append(texts, value.Trim(line[len(key)+1:]))
		}
	}
	return texts
}

// None is whether a free-text footer says a consumer changes nothing: the
// word none alone, in lower case as written, around it only whitespace.
func None(text string) bool { return text == "none" }

// Type is a message's commit type, as the footer rules read it: the word it
// starts with, "" when none.
func Type(message string) string {
	if m := typeWord.FindStringSubmatch(message); m != nil {
		return m[1]
	}
	return ""
}

func applies(types *config.Types, typ string) bool {
	if types == nil {
		return false
	}
	if !types.IsList {
		return types.Word == "all"
	}
	return value.Includes(types.List, typ)
}

func isLedger(f config.Footer) bool { return f.Source.IsName && f.Source.Name == "ledger" }

func strip(f config.Footer) string {
	if f.StripPrefix == nil {
		return ""
	}
	return *f.StripPrefix
}

// example is how a message names a footer: `"Task: T-…"`, or for free text
// `"Upgrading: <what a consumer must do, or none>"`.
func example(cfg *config.Loaded, key string, f config.Footer) string {
	if f.Text() {
		return fmt.Sprintf(`"%s: <what a consumer must do, or none>"`, key)
	}
	var pattern *string
	if isLedger(f) {
		pattern = cfg.Ledger.ID
	} else if k, ok := cfg.Tests.Get(f.Source.Tests); ok {
		pattern = k.ID
	}
	hint := ""
	if pattern != nil {
		hint = idHint.FindString(*pattern)
	}
	return fmt.Sprintf(`"%s: %s%s…"`, key, strip(f), hint)
}

// noun is what a footer's IDs are called: tasks, or the kind's name plural.
func noun(f config.Footer) string {
	if isLedger(f) {
		return "tasks"
	}
	return f.Source.Tests + "s"
}

// notLiveWhy is why a named test is not live: its kind's wip tag, when the
// built-in Gherkin adapter, which reads that tag, lists the kind; a command
// adapter says what is live itself.
func notLiveWhy(cfg *config.Loaded, f config.Footer) (string, error) {
	if isLedger(f) {
		return "not live", nil
	}
	if err := cfg.Section("tests"); err != nil {
		return "", err
	}
	if k, ok := cfg.Tests.Get(f.Source.Tests); ok && k.Adapter.Command == "" && k.Adapter.Name == "gherkin" {
		return "still tagged " + k.WipTag, nil
	}
	return "not live", nil
}

// known are the IDs that exist for a footer at a tree ("" for the working
// tree), and the ones not live.
type known struct{ all, notLive map[string]bool }

func knownAt(cfg *config.Loaded, f config.Footer, tree string) (known, error) {
	if isLedger(f) {
		ids, err := ledger.IDs(cfg, tree)
		return known{all: ids, notLive: map[string]bool{}}, err
	}
	if tree == "" {
		tree = "worktree"
	}
	list, err := tests.ListTests(cfg, f.Source.Tests, tree)
	if err != nil {
		return known{}, err
	}
	k := known{all: map[string]bool{}, notLive: map[string]bool{}}
	for _, t := range list.Tests {
		k.all[t.ID] = true
		if !t.Live {
			k.notLive[t.ID] = true
		}
	}
	return k, nil
}

// needSource is a ledger footer's source in the current source, which a
// project may have configured without making yet: its folder missing is one
// problem naming it, exit 2 (a file the config names that cannot be read),
// not the read's own error.
func needSource(cfg *config.Loaded, key string, f config.Footer) error {
	if !isLedger(f) {
		return nil
	}
	layout, err := ledger.LayoutOf(cfg)
	if err != nil {
		return err
	}
	if source.Has(layout.Dir) {
		return nil
	}
	return &config.Error{File: config.Path(), Problems: []out.Problem{{
		Rule:    "footer-source-missing",
		Message: fmt.Sprintf("the %s: footer names tasks of the ledger, and its folder %s does not exist", key, layout.Dir),
		Fix:     fmt.Sprintf("create %s with the ledger's files (ledger.files is %s), or point ledger.files at the folder that holds them", layout.Dir, cfg.Ledger.Files),
	}}}
}

// knownFor are the IDs that exist for a footer where its read_at says. A
// commit with none at all predates its source (a project's first commits may
// name tasks before the ledger is committed), so there is nothing to read at
// it: the working tree is read instead, and a warning says so. The stealth
// config's ledger is in the git folder, which no commit carries: a footer of
// the ledger is read against its file there, at every commit, saying nothing.
func knownFor(cfg *config.Loaded, key string, f config.Footer, r Reading) (known, error) {
	tree := r.tree(f)
	if cfg.Stealth && isLedger(f) {
		tree = ""
	}
	if tree == "" {
		if err := needSource(cfg, key, f); err != nil {
			return known{}, err
		}
	}
	found, err := knownAt(cfg, f, tree)
	if err != nil || tree == "" || len(found.all) > 0 {
		return found, err
	}
	if err := needSource(cfg, key, f); err != nil {
		return known{}, err
	}
	fmt.Fprintf(r.Warn, "%s has no %s; its %s: footer is read against the working tree\n", tree, noun(f), key)
	return knownAt(cfg, f, "")
}

// CheckFooter is one footer's rule over one message: "" when it holds, else
// why not. An error when its source cannot be read.
func CheckFooter(cfg *config.Loaded, key, typ, message string, r Reading) (string, error) {
	f, _ := cfg.Commits.Footers.Get(key)
	required := applies(f.RequiredFor, typ) && !(r.Made && config.Before(f, r.At))
	if f.Text() {
		return checkText(cfg, key, f, typ, message, required), nil
	}
	ids := IDs(message, key, strip(f))
	if required && len(ids) == 0 {
		return fmt.Sprintf("%s commits need a %s footer", typ, example(cfg, key, f)), nil
	}
	// No ID, nothing to check: the footer's source is not read.
	if !applies(f.ValidateFor, typ) || len(ids) == 0 {
		return "", nil
	}
	k, err := knownFor(cfg, key, f, r)
	if err != nil {
		return "", err
	}
	var unknown, pending []string
	for _, id := range ids {
		if !k.all[id] {
			unknown = append(unknown, id)
		} else if f.Live() && k.notLive[id] {
			pending = append(pending, id)
		}
	}
	if len(unknown) > 0 {
		return fmt.Sprintf("unknown %s: %s", noun(f), strings.Join(unknown, ", ")), nil
	}
	if len(pending) > 0 {
		why, err := notLiveWhy(cfg, f)
		if err != nil {
			return "", err
		}
		return why + ": " + strings.Join(pending, ", "), nil
	}
	return "", nil
}

// checkText is a free-text footer's rule: a type it is required for carries
// one that says something, and a type it validates carries none empty.
func checkText(cfg *config.Loaded, key string, f config.Footer, typ, message string, required bool) string {
	texts := Texts(message, key)
	said := slices.ContainsFunc(texts, func(t string) bool { return t != "" })
	if required && !said {
		return fmt.Sprintf("%s commits need a %s footer", typ, example(cfg, key, f))
	}
	if applies(f.ValidateFor, typ) && slices.Contains(texts, "") {
		return fmt.Sprintf("the %s: footer is empty", key)
	}
	return ""
}

// FooterProblems are the footer rules' problems with a message, footer by
// footer in the config's order, each with its rule (`task-footer`) and fix.
func FooterProblems(cfg *config.Loaded, message string, r Reading) ([]out.Problem, error) {
	typ := Type(message)
	found := []out.Problem{}
	for _, key := range cfg.Commits.Footers.Keys {
		why, err := CheckFooter(cfg, key, typ, message, r)
		if err != nil {
			return nil, err
		}
		if why == "" {
			continue
		}
		rule := strings.ToLower(key) + "-footer"
		fix, err := fixFor(cfg, rule, why)
		if err != nil {
			return nil, err
		}
		found = append(found, out.Problem{Rule: rule, Message: why, Fix: fix})
	}
	return found, nil
}

// PrintFooters prints footer problems as commitlint prints a problem.
func PrintFooters(w io.Writer, found []out.Problem) {
	for _, p := range found {
		fmt.Fprintf(w, "✖   %s [%s]\n", p.Message, p.Rule)
	}
}

// IDsIn are the IDs a pushed range's commits give in one footer (footers.ts's
// footerIdsIn, the range log CI's plan reads): `git log --format=%B
// from..to`, newest commit first, each ID once in the order first given, the
// footer's strip_prefix taken off, and only those whose whole matches the
// footer's ID pattern (the ledger's id, or its kind's; any ID without one). A
// range that cannot be read, or has no start or end, gives none. The range is
// the one CI plans, from..to as git reads it: commits.since does not narrow
// it, as it narrows verify's, since the TypeScript's plan does not.
func IDsIn(cfg *config.Loaded, from, to, key string) []string {
	if from == "" || to == "" {
		return nil
	}
	log, err := git.Output("log", "--format=%B", from+".."+to)
	if err != nil {
		return nil
	}
	f, _ := cfg.Commits.Footers.Get(key)
	var pattern *string
	if isLedger(f) {
		pattern = cfg.Ledger.ID
	} else if k, ok := cfg.Tests.Get(f.Source.Tests); ok {
		pattern = k.ID
	}
	whole := ".+"
	if pattern != nil {
		whole = *pattern
	}
	matches, err := regexp.Compile("^(?:" + whole + ")$")
	if err != nil {
		return nil
	}
	var ids []string
	for _, id := range unique(IDs(log, key, "")) {
		if s := strip(f); s != "" && strings.HasPrefix(id, s) {
			id = id[len(s):]
		}
		if matches.MatchString(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// unique is a list without its repeats, in the order first seen.
func unique(list []string) []string {
	seen := map[string]bool{}
	var kept []string
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			kept = append(kept, s)
		}
	}
	return kept
}

// Said is one free-text footer a commit of a range carries: the commit, its
// subject and the footer's text.
type Said struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

// Gathered are the free-text footers of one key that a range's non-merge
// commits carry, oldest commit first and each commit's in written order: what
// a release's notes are gathered from. from..to as git reads it, every commit
// up to to when from is empty or all zeros; commits.since does not narrow it,
// since a footer written before the rules still says what it says. A footer
// that is empty, or says none, asks nothing and is left out.
func Gathered(from, to, key string) ([]Said, error) {
	span := []string{from + ".." + to}
	if strings.Trim(from, "0") == "" {
		span = []string{to}
	}
	log, err := git.Read(append([]string{"log", "--no-merges", "--reverse", "--format=%H%x00%B%x1e"}, span...)...)
	if err != nil {
		return nil, err
	}
	said := []Said{}
	for _, entry := range strings.Split(log, "\x1e") {
		sha, message, ok := strings.Cut(strings.TrimLeft(entry, "\n"), "\x00")
		if !ok {
			continue
		}
		subject, _, _ := strings.Cut(message, "\n")
		for _, text := range Texts(message, key) {
			if text != "" && !None(text) {
				said = append(said, Said{SHA: sha, Subject: subject, Text: text})
			}
		}
	}
	return said, nil
}
