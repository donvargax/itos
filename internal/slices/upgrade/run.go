package upgrade

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
)

// The exit codes upgrade gives itself (itos --help): a move refused, and a
// release that cannot be fetched, by its kind.
const (
	policy    = 1
	usage     = 2
	missing   = 3
	software  = 70
	temporary = 75
)

// Run moves the project and reports each release's edits and what is left
// by hand, or under --json the one object; a project already on the version
// says so, exit 0, and a move refused says why, exit 1. A release that
// cannot be fetched is a line on stderr, every file left as it was, exit by
// its kind. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	switch {
	case err != nil:
		return 0, err
	case a.Unfetched != nil:
		fmt.Fprintf(stderr, "itos: %s; every file is left as it was\n", a.Unfetched)
		return exitCode(a.Unfetched), nil
	case a.Action == refused:
		if c.JSON {
			return policy, emit(stdout, a)
		}
		fmt.Fprintf(stderr, "itos: %s\n", a.Why)
		return policy, nil
	case c.JSON:
		return 0, emit(stdout, a)
	case a.Action == already:
		fmt.Fprintf(stdout, "This project is already on itos %s: nothing to upgrade.\n", a.To)
		reportWorkflows(stdout, a)
		return 0, nil
	}
	report(stdout, a)
	return 0, nil
}

// json is the edit as --json prints it.
func (a appliedStep) json() *value.Map {
	m := value.NewMap("version", a.version, "kind", a.kind, "key", a.key)
	if a.kind == "renamed" {
		m.Set("from", a.from)
	} else {
		m.Set("value", a.value)
	}
	return m
}

// manualStep is what a release leaves the person to do: a breaking change's
// or an Upgrading footer's text, or the notes of a release with no
// upgrading.json.
type manualStep struct {
	Version string `json:"version"`
	Text    string `json:"text"`
}

// emit is --json's one object: the action, the versions, what each release
// asks, the files edited, the config edits made and what is left by hand.
func emit(stdout io.Writer, a Answer) error {
	applied := make([]any, len(a.Applied))
	for i, s := range a.Applied {
		applied[i] = s.json()
	}
	uses, problems := a.Uses, a.Problems
	if uses == nil {
		uses = []actionUse{}
	}
	if problems == nil {
		problems = []scanProblem{}
	}
	return out.Emit(stdout, out.Field{Key: "action", Value: a.Action}, out.Field{Key: "from", Value: a.From},
		out.Field{Key: "to", Value: a.To}, out.Field{Key: "releases", Value: a.Releases},
		out.Field{Key: "edited", Value: a.Edited}, out.Field{Key: "applied", Value: applied},
		out.Field{Key: "manual", Value: manual(a)}, out.Field{Key: "action_uses", Value: uses},
		out.Field{Key: "action_scan_problems", Value: problems})
}

// manual is what every release crossed leaves the person to do, oldest
// first: its breaking changes and its Upgrading footers, or for a release
// with no upgrading.json the notes to read.
func manual(a Answer) []manualStep {
	steps := []manualStep{}
	for _, r := range a.Releases {
		switch r := r.(type) {
		case releaseNotes:
			steps = append(steps, manualStep{r.Version, fmt.Sprintf("itos %s publishes no upgrading.json: read the "+
				"Upgrading section of its notes, and of each release before it since %s: %s", r.Version, a.From, r.Notes)})
		case releaseAsks:
			for _, e := range append(slices.Clone(r.Breaking), r.Upgrading...) {
				steps = append(steps, manualStep{r.Version, e.Text})
			}
		}
	}
	return steps
}

// report is the move told to a person: the versions, then each release, oldest
// first, with the config edits made for it and what it changes, then By
// hand: what is left to do, and the checks the notes end with.
func report(w io.Writer, a Answer) {
	edited := "nothing needed editing"
	if len(a.Edited) > 0 {
		edited = "edited " + agentrules.Prose(a.Edited, "and")
	}
	fmt.Fprintf(w, "Upgraded itos from %s to %s: %s, and committed nothing.\n", a.From, a.To, edited)
	fmt.Fprintf(w, "Each release since %s, oldest first:\n", a.From)
	for _, r := range a.Releases {
		if r, ok := r.(releaseAsks); ok {
			printRelease(w, a, r)
		}
	}
	reportWorkflows(w, a)
	if len(manual(a)) == 0 {
		fmt.Fprintln(w, "\nBy hand: nothing.")
		fmt.Fprintf(w, "\nCheck: itos version prints itos %s, itos version --check exits 0 and itos config check "+
			"passes. Commit the move as the project's own commit.\n", a.To)
		return
	}
	fmt.Fprintln(w, "\nBy hand:")
	for _, r := range a.Releases {
		switch r := r.(type) {
		case releaseNotes:
			fmt.Fprintf(w, "\nitos %s publishes no upgrading.json: read the Upgrading section of its notes, "+
				"and of each release before it since %s, by hand: %s\n", r.Version, a.From, r.Notes)
		case releaseAsks:
			if len(r.Breaking)+len(r.Upgrading) > 0 {
				fmt.Fprintf(w, "\nitos %s (%s):\n", r.Version, release.NotesURL(r.Version))
				entries(w, "Breaking", r.Breaking)
				entries(w, "Upgrading", r.Upgrading)
			}
		}
	}
	fmt.Fprintf(w, "\nDo what they ask, then check: itos version prints itos %s, itos version --check exits 0 "+
		"and itos config check passes. Commit the move as the project's own commit.\n", a.To)
}

// reportWorkflows is the workflow steps that launch the itos action, one a
// line, the workflows that could not be read, then what chooses the itos
// that runs: the pin's version when the config pins one, else the release
// each line installs. It prints nothing when no step names the action and no
// workflow failed. Nothing it names is moved.
func reportWorkflows(w io.Writer, a Answer) {
	if len(a.Uses)+len(a.Problems) == 0 {
		return
	}
	fmt.Fprintln(w, "\nWorkflow steps that launch the itos action, none of which upgrade moves:")
	for _, u := range a.Uses {
		fmt.Fprintf(w, "  %s\n", u.line())
	}
	for _, p := range a.Problems {
		fmt.Fprintf(w, "  %s %s, so its steps are not named\n", p.File, p.Message)
	}
	switch {
	case len(a.Uses) == 0:
	case a.Pinned:
		fmt.Fprintf(w, "\nThe action installs a launcher, the release its ref or its version input names, and the "+
			"repository's pin chooses the itos that runs: itos %s, whatever these lines say.\n", a.To)
	default:
		fmt.Fprintln(w, "\nThe config pins no itos, so each of these lines runs the release it installs; "+
			"itos pin pins one.")
	}
}

// printRelease is a release's config edits, as made, and what it changes
// that asks nothing of the person: the cases it changes on purpose and its
// config keys.
func printRelease(w io.Writer, a Answer, r releaseAsks) {
	fmt.Fprintf(w, "\nitos %s (%s):\n", r.Version, release.NotesURL(r.Version))
	edits := 0
	for _, s := range a.Applied {
		if s.version != r.Version {
			continue
		}
		edits++
		if s.kind == "renamed" {
			fmt.Fprintf(w, "  Renamed %s to %s.\n", s.from, s.key)
		} else {
			fmt.Fprintf(w, "  Removed %s, which held %s.\n", s.key, value.JSON(s.value))
		}
	}
	if edits+len(r.Changes)+len(r.Config) == 0 {
		fmt.Fprintln(w, "  nothing to edit.")
		return
	}
	entries(w, "Changes on purpose", r.Changes)
	for _, k := range r.Config {
		fmt.Fprintf(w, "  Config key %s: %s\n", k.Key, k.Change)
	}
}

// entries is what commits ask, each under its kind, commit and header.
func entries(w io.Writer, what string, list []upgradingEntry) {
	for _, e := range list {
		fmt.Fprintf(w, "  %s (%s %s):\n", what, shortSHA(e.Commit), e.Header)
		for _, line := range strings.Split(strings.TrimRight(e.Text, "\n"), "\n") {
			fmt.Fprintf(w, "    %s\n", line)
		}
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// exitCode is the exit code of a release that cannot be fetched, by the
// error's kind, as internal/cli's ExitCode gives it.
func exitCode(err error) int {
	switch kind.Of(err) {
	case kind.Usage:
		return usage
	case kind.Missing:
		return missing
	case kind.Temporary:
		return temporary
	}
	return software
}
