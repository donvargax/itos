package message

// A history's commits with their links (work show, slice 57): which commits
// belong to a work item is itos's knowledge, read from the footers of IDs,
// in each commit's message or, under a stealth config, its itos note, as
// every reader of a made commit's links reads them (notes.go).

import (
	"context"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
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
		for _, id := range IDs(cfg, text, key) {
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
	log, err := cfg.VCS.Log(context.Background(), vcs.Range{To: rev})
	if err != nil {
		return nil, err
	}
	var commits []Logged
	for _, e := range log {
		if e.Merge {
			continue
		}
		header, _, _ := strings.Cut(e.Message, "\n")
		links := e.Message
		if cfg.Stealth {
			links = e.Note
		}
		commits = append(commits, Logged{SHA: e.SHA, Short: e.Short, Message: strings.TrimRight(e.Message, "\n"), Header: header, Links: Links(cfg, links)})
	}
	return commits, nil
}
