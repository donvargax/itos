package message

// The footers itos writes (itos commit, features/commit-command.feature):
// which footer of commits.footers a flag writes, found by its source as every
// reader finds it, never by its key (--task the ledger's, --item the work
// registry's, slice 63, --scenarios a kind of named tests'), and the lines that name its IDs, held to
// the limit the header lint holds a footer's lines to; the flags of the
// footers of free text, each named after its footer (slice 36); and the
// footers a commit's type requires that it lacks, so itos commit can refuse
// it before git runs.

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
)

// LedgerFooter is the key of the footer itos commit --task writes: the first
// of commits.footers whose source is the ledger, and whether there is one.
func LedgerFooter(cfg *config.Loaded) (string, bool) {
	return firstFooter(cfg, isLedger)
}

// RegistryFooter is the key of the footer itos commit --item writes: the
// first of commits.footers whose source is the work registry, and whether
// there is one.
func RegistryFooter(cfg *config.Loaded) (string, bool) {
	return firstFooter(cfg, config.Footer.Registry)
}

// TestsFooter is the key of the footer itos commit --scenarios writes: the
// first of commits.footers whose source is a kind of named tests, and whether
// there is one.
func TestsFooter(cfg *config.Loaded) (string, bool) {
	return firstFooter(cfg, func(f config.Footer) bool { return f.Source.Tests != "" })
}

func firstFooter(cfg *config.Loaded, is func(config.Footer) bool) (string, bool) {
	for _, key := range cfg.Commits.Footers.Keys {
		if is(cfg.Commits.Footers.Values[key]) {
			return key, true
		}
	}
	return "", false
}

// BuiltinFlags are itos commit's own footer flags, which win over a footer
// of free text whose name would give the same flag.
var BuiltinFlags = []string{"--task", "--item", "--scenarios", "--breaking"}

// TextFlag is the flag of itos commit that writes a footer of free text: its
// key in lower case, after "--" (--upgrading for Upgrading).
func TextFlag(key string) string { return "--" + strings.ToLower(key) }

// TextFlags are the flags of the config's footers of free text, each to its
// footer's key, in the config's order: a name clash with a built-in flag, or
// with an earlier footer, gives the footer no flag. None without a config.
func TextFlags(cfg *config.Loaded) map[string]string {
	flags := map[string]string{}
	if cfg == nil {
		return flags
	}
	for _, key := range cfg.Commits.Footers.Keys {
		flag := TextFlag(key)
		if _, taken := flags[flag]; taken || !cfg.Commits.Footers.Values[key].Text() || slices.Contains(BuiltinFlags, flag) {
			continue
		}
		flags[flag] = key
	}
	return flags
}

// Flag is the flag of itos commit that writes a footer and what it takes:
// --task <id> for the first footer of the ledger, --item <id> for the first
// of the work registry, --scenarios <ids> for the
// first of a kind of named tests, its own flag <text|none> for one of free
// text; "" when no flag writes it.
func Flag(cfg *config.Loaded, key string) (flag, what string) {
	f, _ := cfg.Commits.Footers.Get(key)
	if f.Text() {
		if flag := TextFlag(key); TextFlags(cfg)[flag] == key {
			return flag, "<text|none>"
		}
		return "", ""
	}
	switch flag := footerFlag(cfg, key, f); flag {
	case "--task", "--item":
		return flag, "<id>"
	case "--scenarios":
		return flag, "<ids>"
	}
	return "", ""
}

// Missing are the footers of commits.footers a commit of the type typ needs
// and does not carry, in the config's order: one of IDs that links names
// none of and no footer standing in for it names any of (in_place_of), one
// of free text that content gives none saying something of. What the hook's
// footer rules call missing, before a commit is made.
func Missing(cfg *config.Loaded, typ, links, content string) []string {
	var keys []string
	for _, key := range cfg.Commits.Footers.Keys {
		f := cfg.Commits.Footers.Values[key]
		if !applies(f.RequiredFor, typ) {
			continue
		}
		if f.Text() {
			if !slices.ContainsFunc(Texts(content, key, cfg.CommentChar()), func(t string) bool { return t != "" }) {
				keys = append(keys, key)
			}
		} else if len(IDs(cfg, links, key)) == 0 && !stoodIn(cfg, key, typ, links) {
			keys = append(keys, key)
		}
	}
	return keys
}

// Trailed is a message with footer lines added as git commit --trailer adds
// them: after its footers when it has some, else as a paragraph of their own,
// the lines starting with the comment character passed over.
func Trailed(message string, footers []string, comment string) string {
	if len(footers) == 0 {
		return message
	}
	lines := strings.Split(message, "\n")
	start, end := trailerBlock(lines, comment)
	gap := "\n\n"
	if start < end {
		gap = "\n"
	}
	return strings.Join(lines[:end], "\n") + gap + strings.Join(footers, "\n")
}

// SplitIDs are the IDs a list gives, separated as a footer separates them, by
// whitespace or commas.
func SplitIDs(list string) []string {
	var ids []string
	for _, id := range idSeparator.Split(list, -1) {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// FooterLines are the lines of the footer key naming the IDs, in order, as
// many to a line as fit in the header lint's limit for a footer's line, the
// key repeated on each: a footer may repeat, and IDs carried onto a line of
// their own without the key would be read by no one. An ID too long for any
// line has one to itself.
func FooterLines(key string, ids []string) []string {
	var lines []string
	var line strings.Builder
	for _, id := range ids {
		if line.Len() > 0 && line.Len()+1+len(id) > maxLength {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() == 0 {
			line.WriteString(key + ":")
		}
		line.WriteString(" " + id)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

// Written is one footer a flag of itos commit writes: its key, the flag that
// gives it, and its values, the IDs of a footer of IDs or the texts of one of
// free text (Text), as the flags gave them.
type Written struct {
	Key, Flag string
	Text      bool
	Values    []string
}

// Lines are the footer's lines as itos commit writes them.
func (w Written) Lines() []string {
	if !w.Text {
		return FooterLines(w.Key, w.Values)
	}
	lines := make([]string, len(w.Values))
	for i, v := range w.Values {
		lines[i] = w.Key + ": " + v
	}
	return lines
}

// Standing is how a message stands to a footer a flag would write.
type Standing int

const (
	// Absent: the message does not give the flag's own key with the flag's
	// values, and gives no other (BREAKING CHANGE, the spaced form, counting
	// only for the refusal); the flag's footer is written.
	Absent Standing = iota
	// Same: the message gives the footer with the flag's values; nothing is
	// written.
	Same
	// Differs: the message gives the footer with other values than the flag's.
	Differs
)

// breakingForms are the two keys a BREAKING-CHANGE footer is read under.
var breakingForms = []string{"BREAKING-CHANGE", breakingKey}

// Against compares the footer the flag would write with the message's, both
// read as the commit-msg hook reads footers (the trailer block, rawIDs and
// unfolded): the sets of the IDs, with the footer's strip_prefix off, or of
// the texts, of every line of the key, order, repeats and wrapping aside.
// A message that gives the key with other values is a problem naming both,
// rule <key>-footer-differs. BREAKING-CHANGE and BREAKING CHANGE are read as
// one footer for that refusal only: the texts of both are compared, but the
// footer is the same, and nothing is written, only when the message gives
// the flag's own key, BREAKING-CHANGE, with the flag's texts; the hook reads
// that key alone, so the spaced form's texts still get the flag's line.
func (w Written) Against(cfg *config.Loaded, message string) (Standing, out.Problem) {
	keys := []string{w.Key}
	if slices.Contains(breakingForms, w.Key) {
		keys = breakingForms
	}
	var said, own []string
	present := false
	prefix := ""
	flagged := w.Values
	if w.Text {
		flagged = make([]string, len(w.Values))
		for i, v := range w.Values {
			flagged[i] = unfolded(v)
		}
	} else {
		f, _ := cfg.Commits.Footers.Get(w.Key)
		prefix = strip(f)
	}
	footers := trailers(message, cfg.CommentChar())
	for _, key := range keys {
		present = present || slices.ContainsFunc(footers, func(t trailer) bool { return t.key == key })
		var given []string
		if w.Text {
			given = Texts(message, key, cfg.CommentChar())
		} else {
			given = rawIDs(footers, key)
		}
		said = append(said, given...)
		if key == w.Key {
			own = given
		}
	}
	if !present {
		return Absent, out.Problem{}
	}
	if sameSet(said, flagged, prefix) {
		if sameSet(own, flagged, prefix) {
			return Same, out.Problem{}
		}
		return Absent, out.Problem{}
	}
	return Differs, out.Problem{
		Rule: strings.ToLower(w.Key) + "-footer-differs",
		Message: fmt.Sprintf("%s: the message gives %s, %s gives %s",
			w.Key, listed(said, w.Text), w.Flag, listed(flagged, w.Text)),
		Fix: fmt.Sprintf("give %s: once, in the message or with %s", w.Key, w.Flag),
	}
}

// sameSet is whether two lists hold the same values, once the prefix is off
// each, whatever their order and repeats.
func sameSet(a, b []string, prefix string) bool {
	set := func(list []string) []string {
		trimmed := make([]string, len(list))
		for i, v := range list {
			trimmed[i] = strings.TrimPrefix(v, prefix)
		}
		slices.Sort(trimmed)
		return slices.Compact(trimmed)
	}
	return slices.Equal(set(a), set(b))
}

// listed are values as a rejection names them, each once in the order given,
// joined by ", ", texts in double quotes, "nothing" when none says anything.
func listed(values []string, text bool) string {
	var shown []string
	for _, v := range unique(values) {
		if v == "" {
			continue
		}
		if text {
			v = `"` + v + `"`
		}
		shown = append(shown, v)
	}
	if len(shown) == 0 {
		return "nothing"
	}
	return strings.Join(shown, ", ")
}
