package message

// A history's commits with their links (work show, slice 57): which commits
// belong to a work item is itos's knowledge, read from the footers of IDs,
// in each commit's message or, under a stealth config, its itos note, as
// every reader of a made commit's links reads them (notes.go).

import (
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
)

// Link is one ID a footer of IDs gives: the footer's key, the ID with its
// strip_prefix taken off, and the kind of named tests the footer's source
// is, "" for the ledger and the work registry, whose IDs are items' own.
type Link struct {
	Key   string
	ID    string
	Tests string
}

// Links are the IDs every footer of IDs of the config gives in a text (a
// message, or a note), in the config's order of footers; a footer of free
// text gives none.
func Links(cfg *config.Loaded, text string) []Link {
	var links []Link
	for _, key := range cfg.Commits.Footers.Keys {
		f := cfg.Commits.Footers.Values[key]
		if !ownIDs(f) && f.Source.Tests == "" {
			continue
		}
		for _, id := range IDs(text, key, strip(f)) {
			links = append(links, Link{Key: key, ID: id, Tests: f.Source.Tests})
		}
	}
	return links
}

// Logged is one commit of a history: its SHA, short SHA, message, header
// (the message's first line) and links.
type Logged struct {
	SHA     string
	Short   string
	Message string
	Header  string
	Links   []Link
}

// History is every non-merge commit reachable from rev, oldest first, each
// with the links its message gives, or under a stealth config its itos note.
func History(cfg *config.Loaded, rev string) ([]Logged, error) {
	args := []string{"log", "--no-merges", "--reverse", "--no-notes"}
	if cfg.Stealth {
		args = append(args, "--notes="+NotesRef)
	}
	log, err := git.Read(append(args, "--format=%H%x00%h%x00%B%x00%N%x1e", rev)...)
	if err != nil {
		return nil, err
	}
	var commits []Logged
	for _, entry := range strings.Split(log, "\x1e") {
		parts := strings.SplitN(strings.TrimLeft(entry, "\n"), "\x00", 4)
		if len(parts) < 4 {
			continue
		}
		header, _, _ := strings.Cut(parts[2], "\n")
		links := parts[2]
		if cfg.Stealth {
			links = parts[3]
		}
		commits = append(commits, Logged{SHA: parts[0], Short: parts[1], Message: strings.TrimRight(parts[2], "\n"), Header: header, Links: Links(cfg, links)})
	}
	return commits, nil
}
