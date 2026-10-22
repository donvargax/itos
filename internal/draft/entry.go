package draft

import (
	"fmt"
	"strings"
)

// Entry is a draft as draft --json and status --json list it.
type Entry struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Header  string   `json:"header,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Command []string `json:"command,omitempty"`
	Minted  string   `json:"minted,omitempty"`
}

// EntryOf is the draft as --json lists it: a change, or a command.
func EntryOf(d Draft) Entry {
	if d.IsChange() {
		return Entry{ID: d.ID, Kind: "change", Header: d.Header(), Paths: d.Paths}
	}
	return Entry{ID: d.ID, Kind: "command", Command: d.Command, Minted: d.Minted}
}

// Summary is what a list says of a draft: a change's header and its
// paths, or a command's line, and the id it makes when it mints one.
func Summary(d Draft) string {
	switch {
	case d.IsChange():
		return d.Header() + "  (" + strings.Join(d.Paths, ", ") + ")"
	case d.Minted != "":
		return d.Line() + "  (makes " + d.Minted + ")"
	}
	return d.Line()
}

// Lines are the drafts as lists print them, indented, ids aligned.
func Lines(drafts []Draft) []string {
	width := 0
	for _, d := range drafts {
		width = max(width, len(d.ID))
	}
	lines := make([]string, len(drafts))
	for i, d := range drafts {
		lines[i] = fmt.Sprintf("  %-*s  %s", width, d.ID, Summary(d))
	}
	return lines
}
