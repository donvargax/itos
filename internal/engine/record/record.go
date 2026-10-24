package record

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// BodyWidth is the width a registry commit's body is wrapped at: the header
// lint's body-max-line-length (selfBody).
const BodyWidth = 100

// File is one file a command writes: its Path, its text before (Old) and
// after (Text), whether it is Created (not there before, so put back by
// removing it), and the Rule that refuses it with changes no commit holds,
// when it is neither the registry, the questions nor a ledger file.
type File struct {
	Path, Old, Text string
	Created         bool
	Rule            string
}

// Commit is the commit Write makes of the files: its Header, and its Body,
// which selfBody wraps; Quiet, Stdin, Words and Env as the tracker's
// Transaction has them.
type Commit struct {
	Header, Body string
	Quiet        bool
	Stdin        io.Reader
	Words        io.Writer
	Env          []string
}

// Sound is the registry and its text, when the registry is there and sound:
// a command writes only a registry work check passes. Otherwise its problems
// are the answer. The caller holds t from before Sound to after its write.
func Sound(ctx context.Context, t port.Tracker, cfg *config.Loaded) (work.Registry, string, []out.Problem, error) {
	found, err := work.Problems(cfg)
	if err != nil {
		return work.Registry{}, "", nil, err
	}
	if len(found) > 0 {
		return work.Registry{}, "", found, nil
	}
	file := cfg.Work.Registry
	text, err := trackedText(ctx, t, file)
	if err != nil {
		return work.Registry{}, "", nil, err
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return work.Registry{}, "", nil, err
	}
	return registry, text, nil, nil
}

// trackedText is the text of the document name, which the command found
// there: one gone since is an error naming it.
func trackedText(ctx context.Context, t port.Tracker, name string) (string, error) {
	docs, err := t.Load(ctx, []string{name})
	if err == nil && !docs[0].There {
		err = &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if err != nil {
		return "", err
	}
	return docs[0].Text, nil
}

// uncommitted is the problem of a file a command writes and commits that
// git does not hold as HEAD has it (the tracker's *Dirty), which the
// command's commit would sweep in beside its own change, a deletion among
// them (bug 13). The problem names the file, the registry, a ledger file,
// itos decision's questions, or the rule the command gives (a decision
// record's).
func uncommitted(cfg *config.Loaded, file, rule string) *out.Problem {
	switch {
	case rule != "":
	case file == cfg.Work.Registry:
		rule = "work-registry-uncommitted"
	case file == cfg.Work.Asks:
		rule = "asks-file-uncommitted"
	default:
		rule = "ledger-file-uncommitted"
	}
	return &out.Problem{
		Rule:    rule,
		Message: file + " has changes no commit holds, which itos's commit of it would sweep in",
		Fix:     "commit or drop the changes to " + file + " first",
	}
}

// Write writes the files and, in a project, commits them alone: git commit
// --only, with the header and body, through the hooks, what else is staged
// left staged (the tracker's Transact). Its result is the new commit's SHA,
// "" under a stealth config, which commits nothing. A file with changes no
// commit holds (uncommitted), a deletion among them, is refused before
// anything is written, a problem. Whatever fails after the first write, a
// write, git add, a git that cannot start or a commit a hook refuses, puts
// every file back as it was, file and index, a new one removed; a refusal is
// a problem, the rest are errors.
func Write(ctx context.Context, t port.Tracker, cfg *config.Loaded, files []File, c Commit) (string, []out.Problem, error) {
	tx := port.Transaction{
		Changes:    make([]port.Change, len(files)),
		Paragraphs: []string{c.Header, selfBody(cfg, c.Header, c.Body)},
		Quiet:      c.Quiet,
		Stdin:      c.Stdin,
		Words:      c.Words,
		Env:        c.Env,
	}
	paths := make([]string, len(files))
	for i, f := range files {
		tx.Changes[i] = port.Change{Name: f.Path, Old: f.Old, Text: f.Text, Created: f.Created}
		paths[i] = f.Path
	}
	sha, err := t.Transact(ctx, tx)
	var refused *port.Refused
	switch {
	case errors.As(err, &refused) && refused.Restore != nil:
		return "", nil, fmt.Errorf("%s failed (git exited %d): %w", refused.Step, refused.Code, refused.Restore)
	case errors.As(err, &refused):
		as := "is as it was"
		if len(files) > 1 {
			as = "are as they were"
		}
		return "", []out.Problem{{
			Rule:    "work-commit-failed",
			Message: fmt.Sprintf("%s failed (git exited %d), so %s %s", refused.Step, refused.Code, and(paths), as),
			Fix:     "fix what git or the hook above says, then run the command again",
		}}, nil
	}
	if p, err := dirty(cfg, files, err); err != nil {
		return "", nil, err
	} else if p != nil {
		return "", []out.Problem{*p}, nil
	}
	return sha, nil, nil
}

// Ready is what Write refuses before it writes anything, in a project (the
// tracker's Ready): the hooks that check the commit not ready, an error,
// since the commit runs through the hooks as any commit and git would run
// none of itos's (slice 91); and the first file with changes no commit holds
// (uncommitted), a problem. A command that reserves an id asks it before it
// does (work promote, slice 138). Under a stealth config, which commits
// nothing, nothing is.
func Ready(ctx context.Context, t port.Tracker, cfg *config.Loaded, files []File) (*out.Problem, error) {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return dirty(cfg, files, t.Ready(ctx, paths))
}

// dirty is the tracker's error as Write gives it: a *Dirty file the problem
// its rule names (uncommitted), anything else the error.
func dirty(cfg *config.Loaded, files []File, err error) (*out.Problem, error) {
	var d *port.Dirty
	if !errors.As(err, &d) {
		return nil, err
	}
	rule := ""
	if at := slices.IndexFunc(files, func(f File) bool { return f.Path == d.Name }); at >= 0 {
		rule = files[at].Rule
	}
	return uncommitted(cfg, d.Name, rule), nil
}

// selfBody is the body of a commit itos makes of its own files, its words
// on one line, then wrapped at BodyWidth by the wrap itos commit uses
// (message.Wrap, bug 15): no line starts with a token the header lint reads
// as a footer, a breaking-change note, a configured footer's key and its
// colon, or git's comment char, a break there moving back a word. A plain
// wrap started lines with whatever word came next, so a question or a why
// holding "word:" drew the lint's footer-leading-blank warning on itos's own
// commit (bug 18). A word longer than the line is then broken inside
// (message.BreakLong, bug 132), which the lint would refuse whole. A text the
// person gave never leads the body: led by "Word:", its line was a footer to
// git.
func selfBody(cfg *config.Loaded, header, body string) string {
	parts := []string{header, strings.Join(strings.Fields(body), " ")}
	wrapped := message.Wrap(parts, BodyWidth, cfg.CommentChar(), cfg.Commits.Footers.Keys)[1]
	return message.BreakLong(wrapped, BodyWidth, cfg.CommentChar(), cfg.Commits.Footers.Keys)
}

// and is the names joined as a list is said: "a", "a and b", "a, b and c".
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
