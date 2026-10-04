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
	"slices"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
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
			if !slices.ContainsFunc(Texts(content, key), func(t string) bool { return t != "" }) {
				keys = append(keys, key)
			}
		} else if len(IDs(links, key, strip(f))) == 0 && !stoodIn(cfg, key, typ, links) {
			keys = append(keys, key)
		}
	}
	return keys
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
