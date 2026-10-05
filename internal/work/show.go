package work

// What work show (slice 57, features/show.feature) knows of the registry: an
// item with the items that wait on it, and which of itos's own registry
// commits name an item in their headers. The commits a footer links to an
// item are the footers' (internal/message); the command line puts the two
// together (internal/cli/workshow.go).

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v3/internal/out"
	"github.com/donvargax/itos/v3/internal/value"
)

// Shown is an item as work show gives it: the mapping as written, its
// depends_on a list as Load gives it, and the ids of the items whose
// depends_on name it, in the registry's order.
type Shown struct {
	Item         *value.Map
	DependedOnBy []string
	// LedgerWhy is a task's why, read from its ledger entry (slice 76: a
	// task's reasons live there, the registry being an index), "" for none
	// and for an item that is no task; Print writes it before the item's own.
	LedgerWhy string
}

// Show is the item with the id, or the problem of an id no item has.
func Show(r Registry, id string) (Shown, *out.Problem) {
	i := find(r, id)
	if i < 0 {
		return Shown{}, unknown(id)
	}
	shown := Shown{Item: r.Items[i], DependedOnBy: []string{}}
	for _, item := range r.Items {
		deps, _ := item.At("depends_on").([]any)
		for _, dep := range deps {
			if value.String(dep) == id {
				shown.DependedOnBy = append(shown.DependedOnBy, value.String(item.At("id")))
				break
			}
		}
	}
	return shown, nil
}

// SpecKind is where an item's why lives instead of the registry (slice 76):
// "slice" for a slice, whose why is its feature file's, "task" for a task,
// whose why is its ledger entry's, "" for an idea, whose why is the
// registry's, and for an item of neither. An item of no kind is read by its
// id, as the registry's header names them: slice-<n> a slice, an id taskID
// (ledger.id; nil when the config has none) matches a task.
func SpecKind(item *value.Map, taskID *regexp.Regexp) string {
	if kind := item.At("kind"); !absent(kind) {
		if k := value.String(kind); k == "slice" || k == "task" {
			return k
		}
		return ""
	}
	id := value.String(item.At("id"))
	switch {
	case sliceID.MatchString(id):
		return "slice"
	case taskID != nil && taskID.MatchString(id):
		return "task"
	}
	return ""
}

// sliceID is a slice's id, slice-<n>.
var sliceID = regexp.MustCompile(`^slice-\d+$`)

// registryVerbs are the words after "docs: " of the headers itos's registry
// commands commit with, each followed by the item's id: work take, work
// done, work add (and task add), work edit, work queue (and work done's
// taking the item out of the queue, slice 66) and work drop (slice 78).
var registryVerbs = []string{"take", "close", "add", "edit", "queue", "drop"}

// RegistryHeader is the item a header of itos's registry commits names
// (for work promote's, "docs: promote <idea> to <id>", the item it made);
// ok is false for any other header.
func RegistryHeader(header string) (id string, ok bool) {
	rest, isDocs := strings.CutPrefix(header, "docs: ")
	if !isDocs {
		return "", false
	}
	verb, words, _ := strings.Cut(rest, " ")
	fields := strings.Fields(words)
	if verb == "promote" {
		if len(fields) == 3 && fields[1] == "to" {
			return fields[2], true
		}
		return "", false
	}
	for _, v := range registryVerbs {
		if verb == v && len(fields) == 1 {
			return fields[0], true
		}
	}
	return "", false
}

// Print writes the item as work show's text begins: its id and title, then
// its kind, status, owner (nobody for none), phase, the items it depends on
// and those depending on it, its refs and issue when it has them, and its why
// wrapped at width, each paragraph after a blank line: a task's ledger why
// first (LedgerWhy), then the registry's.
func (s Shown) Print(w io.Writer, width int) {
	at := func(key string) string {
		if v := s.Item.At(key); !absent(v) {
			return value.String(v)
		}
		return "-"
	}
	list := func(key string) string {
		items, _ := s.Item.At(key).([]any)
		return join(items, ", ")
	}
	fmt.Fprintf(w, "%s  %s\n", at("id"), at("title"))
	owner := "nobody"
	if v := s.Item.At("owner"); !absent(v) {
		owner = value.String(v)
	}
	fmt.Fprintf(w, "  kind: %s, status: %s, owner: %s, phase: %s\n", at("kind"), at("status"), owner, at("phase"))
	if deps := list("depends_on"); deps != "" {
		fmt.Fprintf(w, "  depends on: %s\n", deps)
	}
	if len(s.DependedOnBy) > 0 {
		fmt.Fprintf(w, "  depended on by: %s\n", strings.Join(s.DependedOnBy, ", "))
	}
	if refs := list("refs"); refs != "" {
		fmt.Fprintf(w, "  refs: %s\n", refs)
	}
	if issue := s.Item.At("issue"); value.Truthy(issue) {
		fmt.Fprintf(w, "  issue: #%s\n", value.String(issue))
	}
	printWhy(w, s.LedgerWhy, width)
	if why := s.Item.At("why"); !absent(why) {
		printWhy(w, value.String(why), width)
	}
}

// printWhy writes a why's paragraphs wrapped at width, each after a blank
// line; nothing for an empty one.
func printWhy(w io.Writer, why string, width int) {
	// A folded why reads its paragraphs as lines, the blank line between two
	// written ones being a line break.
	for _, paragraph := range strings.Split(strings.TrimSpace(why), "\n") {
		if strings.TrimSpace(paragraph) == "" {
			continue
		}
		fmt.Fprintln(w)
		for _, line := range strings.Split(value.Wrap(paragraph, width-2), "\n") {
			fmt.Fprintln(w, "  "+line)
		}
	}
}
