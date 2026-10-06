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
// commits.footers: its source (the ledger, the work registry's items, or a
// kind's named tests as its adapter lists them), strip_prefix, required_for,
// validate_for, must_be_live, in_place_of (the footers it stands in for, and
// for which types: a type that requires one of them is satisfied by this one
// instead, slice 63), and read_at: `commit` reads the IDs that exist at the commit
// being checked (the staged tree by default, the commit Reading.At names),
// so a later commit that sets a test back to wip or drops a task does not
// fail an older one, but for the stealth config's ledger and registry, in no
// commit, read in the working tree; `worktree`, or none, the working tree; since, a commit
// that verify leaves out of required_for with its ancestors (Reading.Made).
//
// A footer whose source is text carries no IDs: `<Key>: <text>`, each line
// one footer, the text whatever a consumer of the project must do, or the
// word none. Its rule is only that it is there (required_for) and not empty
// (validate_for); Texts reads it, and a range's are gathered by
// `itos commit footers`.
//
// Under a stealth config a commit's links, its footers of IDs, are not in
// its message but in its note (notes.go): Reading.Note carries them, the
// rules read them there, and one typed into the message is refused, since it
// would show to everyone. A footer of free text is content, not a link, and
// stays in the message in either mode (slice 36).
package message

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/git"
	"github.com/donvargax/itos/v5/internal/ledger"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/source"
	"github.com/donvargax/itos/v5/internal/tests"
	"github.com/donvargax/itos/v5/internal/value"
	"github.com/donvargax/itos/v5/internal/work"
)

// Reading is where the footer rules read a read_at: commit footer's IDs: At
// is the commit, "" for the staged tree; Warn is where a footer read against
// the working tree instead says so. Made is whether the message is At's own,
// a commit already made, as verify reads it: a footer's since then leaves
// that commit and its ancestors out of its required_for. The commit-msg hook
// and check-message judge the commit being made, which every rule holds.
// Note is the commit's footers under a stealth config, read there instead
// of the message's: the lines itos commit hands the hook, or a made commit's
// itos note.
type Reading struct {
	At   string
	Made bool
	Note string
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

// Type is a message's commit type, as every rule judges it (the paths, the
// footers, the moves rule): the word its header starts with, "" when none.
// The headers git writes itself, which the header lint leaves alone as
// commitlint does, are judged by what they stand for (bug 30): an amend!,
// fixup! or squash! commit as the header it names, once the prefixes are
// off (Named), and a revert's or a reapply's, Revert "…" or Reapply "…", as
// revert.
func Type(message string) string {
	header := Named(message)
	if reverting.MatchString(header) {
		return "revert"
	}
	if m := typeWord.FindStringSubmatch(header); m != nil {
		return m[1]
	}
	return ""
}

var (
	// The prefixes git writes before the header a commit amends, fixes up or
	// squashes into, as many as it took.
	namingPrefixes = regexp.MustCompile(`^(?:(?:amend|fixup|squash)!` + value.Space + `*)+`)
	// The header git writes for a revert, and for a revert of one.
	reverting = regexp.MustCompile(`^[Rr](?:evert|eapply) `)
)

// Named is a message's header, less the amend!, fixup! and squash! prefixes
// that name the header of another commit: that header, or the message's own
// when it has none.
func Named(message string) string {
	header, _, _ := strings.Cut(message, "\n")
	return replaceFirst(namingPrefixes, header)
}

// namedProblems are the problems of a message that names another commit's
// header (amend!, fixup!, squash!): one when that header has no type of the
// commit types, as any header without one is refused; none for a revert's,
// judged as revert, or for a message without such a prefix, which the header
// lint judges.
func namedProblems(cfg *config.Loaded, message string) []out.Problem {
	header, _, _ := strings.Cut(message, "\n")
	prefixes := namingPrefixes.FindString(header)
	if prefixes == "" {
		return nil
	}
	named := header[len(prefixes):]
	if reverting.MatchString(named) {
		return nil
	}
	types := Types(cfg)
	typ := ""
	if m := headerPattern.FindStringSubmatch(named); m != nil {
		typ = m[1]
	}
	if typ != "" && value.Includes(types, typ) {
		return nil
	}
	prefix, _, _ := strings.Cut(prefixes, "!")
	return []out.Problem{{
		Rule: "named-type",
		Message: fmt.Sprintf("%s! names the header %q, which has no type of [%s]",
			prefix, named, strings.Join(types, ", ")),
		Fix: fmt.Sprintf("name a header that starts with one of the commit types, as in `%s! fix: …`", prefix),
	}}
}

// Typed is whether a type is one of the commit types: commits.types, or
// config-conventional's own when the config lists none, as the header lint
// takes them.
func Typed(cfg *config.Loaded, typ string) bool { return value.Includes(Types(cfg), typ) }

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

// ownIDs is whether a footer's IDs are its source's own, the ledger's tasks
// or the registry's items, rather than a kind's named tests.
func ownIDs(f config.Footer) bool { return isLedger(f) || f.Registry() }

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

// noun is what a footer's IDs are called: tasks, items, or the kind's name
// plural.
func noun(f config.Footer) string {
	if isLedger(f) {
		return "tasks"
	}
	if f.Registry() {
		return "items"
	}
	return f.Source.Tests + "s"
}

// notLiveWhy is why a named test is not live: its kind's wip tag, when the
// built-in Gherkin adapter, which reads that tag, lists the kind; a command
// adapter says what is live itself.
func notLiveWhy(cfg *config.Loaded, f config.Footer) (string, error) {
	if ownIDs(f) {
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
	if f.Registry() {
		ids, err := work.IDsAt(cfg.Work.Registry, tree)
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

// needSource is a ledger or registry footer's source in the current source,
// which a project may have configured without making yet: the ledger's folder
// or the registry's file missing is one problem naming it, exit 2 (a file the
// config names that cannot be read), not the read's own error.
func needSource(cfg *config.Loaded, key string, f config.Footer) error {
	if f.Registry() {
		if source.Has(cfg.Work.Registry) {
			return nil
		}
		return &config.Error{File: config.Path(), Problems: []out.Problem{{
			Rule:    "footer-source-missing",
			Message: fmt.Sprintf("the %s: footer names items of the work registry, and its file %s does not exist", key, cfg.Work.Registry),
			Fix:     "create " + cfg.Work.Registry + " with the work items, or point work.registry at the file that holds them",
		}}}
	}
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
// config's ledger and registry are in the git folder, which no commit
// carries: a footer of either is read against its file there, at every
// commit, saying nothing.
func knownFor(cfg *config.Loaded, key string, f config.Footer, r Reading) (known, error) {
	tree := r.tree(f)
	if cfg.Stealth && ownIDs(f) {
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

// CheckFooter is one footer's rule over the footers of a commit of the type
// typ, in its message or, under a stealth config, its note: "" when it
// holds, else why not. An error when its source cannot be read.
func CheckFooter(cfg *config.Loaded, key, typ, message string, r Reading) (string, error) {
	f, _ := cfg.Commits.Footers.Get(key)
	required := applies(f.RequiredFor, typ) && !(r.Made && config.Before(f, r.At))
	if f.Text() {
		return checkText(cfg, key, f, typ, message, required), nil
	}
	ids := IDs(message, key, strip(f))
	if required && len(ids) == 0 && !stoodIn(cfg, key, typ, message) {
		return fmt.Sprintf("%s commits need a %s footer%s", typ, example(cfg, key, f), stealthNeed(cfg, key, f)), nil
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

// stoodIn is whether another footer of IDs that the footers carry stands in
// for the footer key in a commit of the type typ (its in_place_of names key
// for typ), so the commit needs no footer of key. A stand-in's own IDs are
// its own rule's to judge.
func stoodIn(cfg *config.Loaded, key, typ, footers string) bool {
	for _, other := range cfg.Commits.Footers.Keys {
		f := cfg.Commits.Footers.Values[other]
		if other == key || f.Text() {
			continue
		}
		types, ok := f.InPlaceOf.Get(key)
		if ok && applies(&types, typ) && len(IDs(footers, other, strip(f))) > 0 {
			return true
		}
	}
	return false
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
// footer in the config's order, each with its rule (`task-footer`) and fix,
// after the one of a header named by amend!, fixup! or squash! that has no
// type (namedProblems), the header lint's to leave alone and itos's to judge.
// Under a stealth config the footers of IDs are read from r.Note, and one
// typed into the message is its footer's problem; a footer of free text is
// the message's in either mode.
func FooterProblems(cfg *config.Loaded, message string, r Reading) ([]out.Problem, error) {
	typ := Type(message)
	found := append([]out.Problem{}, namedProblems(cfg, message)...)
	for _, key := range cfg.Commits.Footers.Keys {
		why := ""
		footers := message
		if cfg.Stealth && !cfg.Commits.Footers.Values[key].Text() {
			footers = r.Note
			why = typedFooter(cfg, key, message)
		}
		if why == "" {
			var err error
			if why, err = CheckFooter(cfg, key, typ, footers, r); err != nil {
				return nil, err
			}
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
// from..to`, or each commit's itos note under a stealth config, newest
// commit first, each ID once in the order first given, the
// footer's strip_prefix taken off, and only those whose whole matches the
// footer's ID pattern (the ledger's id, or its kind's; any ID without one). A
// range that cannot be read, or has no end, gives none. The range is the one
// CI plans, from..to as git reads it: commits.since does not narrow it, as it
// narrows verify's, since the TypeScript's plan does not. An empty start is
// every commit up to to, as `itos ci range` gives it when no green start can
// be trusted (bug 33): the plan then runs what every one of them names.
func IDsIn(cfg *config.Loaded, from, to, key string) []string {
	if to == "" {
		return nil
	}
	revs := []string{to}
	if from != "" {
		revs = git.Revs(from, to)
	}
	log, err := git.Output(append(append([]string{"log"}, footersFormat(cfg)...), revs...)...)
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
// that is empty, or says none, asks nothing and is left out. A footer of free
// text is in the message under a stealth config too (slice 36).
func Gathered(from, to, key string) ([]Said, error) {
	span := []string{from + ".." + to}
	if strings.Trim(from, "0") == "" {
		span = []string{to}
	}
	args := append([]string{"log", "--no-merges", "--reverse", "--format=%H%x00%B%x1e"}, span...)
	log, err := git.Read(args...)
	if err != nil {
		return nil, err
	}
	said := []Said{}
	for _, entry := range strings.Split(log, "\x1e") {
		// The commit and its message.
		sha, body, ok := strings.Cut(strings.TrimLeft(entry, "\n"), "\x00")
		if !ok {
			continue
		}
		subject, _, _ := strings.Cut(body, "\n")
		for _, text := range Texts(body, key) {
			if text != "" && !None(text) {
				said = append(said, Said{SHA: sha, Subject: subject, Text: text})
			}
		}
	}
	return said, nil
}
