package cli

// work show (slice 57, features/show.feature): one item's spec and commits,
// the reading list of the agent that builds it and the input of a review of
// it once landed. Which commits belong to an item is itos's knowledge, not
// git's: those whose footers of IDs name the item (Task:) or one of its
// scenarios (Scenarios:), read from the note under a stealth config as every
// reader of links reads them, and itos's own registry commits, which name it
// in their headers (docs: take <id>, docs: close <id>, docs: add <id>,
// docs: edit <id>, docs: promote <idea> to <id>). It reads, never writes.

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/message"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/source"
	"github.com/donvargax/itos/v2/internal/tests"
	"github.com/donvargax/itos/v2/internal/work"
)

// showWidth is the width an item's why is wrapped at.
const showWidth = 100

// shownCommit is a commit as work show --json gives it: with --patch, its
// whole message and its diff as git show prints them.
type shownCommit struct {
	SHA     string  `json:"sha"`
	Short   string  `json:"short"`
	Header  string  `json:"header"`
	Message *string `json:"message,omitempty"`
	Patch   *string `json:"patch,omitempty"`
}

// workShow is `work show <id> [--patch]`: the item, its scenarios and its
// commits. A registry that is not there, or no item with the id, exits 1;
// the registry is not judged otherwise, as work list does not judge it.
func workShow(args []string, o Out) (int, error) {
	var ids []string
	patch := false
	for _, arg := range args {
		switch {
		case arg == "--patch":
			patch = true
		case strings.HasPrefix(arg, "-"):
			return 0, usage("work show does not take %s", arg)
		default:
			ids = append(ids, arg)
		}
	}
	if len(ids) != 1 {
		return 0, usage("work show needs one <id>")
	}
	id := ids[0]
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	file := cfg.Work.Registry
	if !source.Has(file) {
		return refuseWork([]out.Problem{work.Missing(file, false)}, ExitPolicy, o)
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return 0, err
	}
	shown, problem := work.Show(registry, id)
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	hasHead := git.Succeeds("rev-parse", "--verify", "--quiet", "HEAD")
	at := "HEAD"
	if !hasHead {
		at = "worktree"
	}
	scenarios, err := tests.Tagged(cfg, "@"+id, at)
	if err != nil {
		return 0, err
	}
	var commits []message.Logged
	if hasHead {
		history, err := message.History(cfg, "HEAD")
		if err != nil {
			return 0, err
		}
		commits = belonging(cfg, id, scenarios, history)
	}
	if o.JSON {
		return 0, emitShown(shown, scenarios, commits, patch, o)
	}
	shown.Print(o.Stdout, showWidth)
	printScenarios(scenarios, id, o)
	if len(commits) == 0 {
		fmt.Fprintln(o.Stdout, "\nCommits: none yet")
		return 0, nil
	}
	fmt.Fprintln(o.Stdout, "\nCommits, oldest first:")
	shas := make([]string, len(commits))
	for i, c := range commits {
		fmt.Fprintf(o.Stdout, "  %s %s\n", c.Short, c.Header)
		shas[i] = c.SHA
	}
	if !patch {
		return 0, nil
	}
	shows, err := git.Read(append(showArgs(cfg), shas...)...)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(o.Stdout, "\n%s", shows)
	return 0, nil
}

// belonging are the commits of the history that belong to the item, oldest
// first: a registry commit of itos's naming it in its header, or one whose
// footers link to it or to one of its scenarios.
func belonging(cfg *config.Loaded, id string, scenarios []tests.Test, history []message.Logged) []message.Logged {
	tagged := map[string]bool{}
	for _, t := range scenarios {
		tagged[t.ID] = true
	}
	links := func(l message.Link) bool {
		if l.Tests == "" {
			return l.ID == id
		}
		k, _ := cfg.Tests.Get(l.Tests)
		return tagged[l.ID] || tagged[k.TagPrefix+l.ID]
	}
	var found []message.Logged
	for _, c := range history {
		named, ok := work.RegistryHeader(c.Header)
		belongs := ok && named == id
		for _, l := range c.Links {
			belongs = belongs || links(l)
		}
		if belongs {
			found = append(found, c)
		}
	}
	return found
}

// printScenarios writes the item's scenarios, a line each: its ID, its file
// and whether it is live or @wip.
func printScenarios(scenarios []tests.Test, id string, o Out) {
	if len(scenarios) == 0 {
		fmt.Fprintf(o.Stdout, "\nScenarios: none tagged @%s\n", id)
		return
	}
	fmt.Fprintf(o.Stdout, "\nScenarios tagged @%s:\n", id)
	width := 0
	for _, t := range scenarios {
		width = max(width, utf8.RuneCountInString(t.ID))
	}
	for _, t := range scenarios {
		state := "wip "
		if t.Live {
			state = "live"
		}
		fmt.Fprintf(o.Stdout, "  %s%s  %s  %s\n", t.ID, strings.Repeat(" ", width-utf8.RuneCountInString(t.ID)), state, t.File)
	}
}

// showArgs are git show's arguments for a commit's message and diff as work
// show --patch prints them: no color, and the itos notes alone under a
// stealth config, where a commit's links are.
func showArgs(cfg *config.Loaded) []string {
	args := []string{"show", "--no-color", "--patch", "--no-notes"}
	if cfg.Stealth {
		args = append(args, "--notes="+message.NotesRef)
	}
	return args
}

// emitShown prints work show --json: the item, the ids of the items
// depending on it, its scenarios and its commits, with --patch each one's
// message and diff.
func emitShown(shown work.Shown, scenarios []tests.Test, commits []message.Logged, patch bool, o Out) error {
	if scenarios == nil {
		scenarios = []tests.Test{}
	}
	listed := []shownCommit{}
	for _, c := range commits {
		s := shownCommit{SHA: c.SHA, Short: c.Short, Header: c.Header}
		if patch {
			diff, err := git.Read("show", "--no-color", "--patch", "--format=", c.SHA)
			if err != nil {
				return err
			}
			diff = strings.TrimLeft(diff, "\n")
			s.Message, s.Patch = &c.Message, &diff
		}
		listed = append(listed, s)
	}
	return out.Emit(o.Stdout,
		out.Field{Key: "ok", Value: true},
		out.Field{Key: "item", Value: shown.Item},
		out.Field{Key: "depended_on_by", Value: shown.DependedOnBy},
		out.Field{Key: "scenarios", Value: scenarios},
		out.Field{Key: "commits", Value: listed})
}
