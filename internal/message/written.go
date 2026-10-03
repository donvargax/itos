package message

// The footers itos writes (itos commit, features/commit-command.feature):
// which footer of commits.footers a flag writes, found by its source as every
// reader finds it, never by its key, and the lines that name its IDs, held to
// the limit the header lint holds a footer's lines to.

import (
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
)

// LedgerFooter is the key of the footer itos commit --task writes: the first
// of commits.footers whose source is the ledger, and whether there is one.
func LedgerFooter(cfg *config.Loaded) (string, bool) {
	return firstFooter(cfg, isLedger)
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
