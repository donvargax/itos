package work

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// Closed is what work done made of an item: the Line it says, unless -q,
// the registry's Change and the Fields --json adds between the item and the
// commit (ci, run, superseded and queue_commit).
type Closed struct {
	Line   string
	Change domain.Change
	Fields []out.Field
}

// done is `work done <id>`: the item done and the registry committed
// (work.Done), once its work has landed.
func (h Handler) done(ctx context.Context, id string, o Out) (Answer, error) {
	a, s, err := h.read(ctx, o)
	if s == nil {
		return a, err
	}
	change, a, err := closing(a, s, id)
	if change == nil {
		return a, err
	}
	cfg := s.cfg
	if cfg.Stealth {
		// The checks below run the task's checks and ask CI, which can take
		// minutes: the stealth registry's lock is given back meanwhile, and
		// taken again for the write below (bug 16).
		a.Release()
		a.Release = func() {}
	}
	var remote, main string
	var merged *mergedHead
	if cfg.Push.Mode == "pr" {
		var stop *refusedPR
		if remote, main, stop = prMain(ctx, h.ports, cfg, id, o); stop != nil {
			a.Refused, a.Code = stop.found, stop.code
			return a, nil
		}
	}
	tagged, examples, err := tests.TaggedExamples(cfg, "@"+id, "HEAD")
	if err != nil {
		return a, err
	}
	found := wipScenarios(tagged, examples, id)
	landing := ownCommit(cfg, id, tagged)
	switch {
	case cfg.Push.Mode == "pr":
		if len(found) == 0 {
			m, stop := prLanded(ctx, h.ports, cfg, id, remote, main, landing)
			if stop != nil {
				a.Refused, a.Code = stop.found, stop.code
				return a, nil
			}
			merged = headOf(m, remote, main)
		}
	case landing.sha == "":
		landing = judgedCommit(ctx, cfg.VCS, cfg.Work.Registry)
	}
	if len(found) == 0 {
		found = unpushed(ctx, cfg.VCS, id, landing, o.Stderr)
	}
	var ran taskRun
	if len(found) == 0 {
		if ran, found, err = h.failingTask(ctx, cfg, id, o.Stderr); err != nil {
			return a, err
		}
		a.Evidence = ran.evidence
	}
	if len(found) > 0 {
		a.Refused, a.Code = found, refused
		return a, nil
	}
	ran.say(id, o)
	ci, pulled, found, err := h.landedCI(ctx, cfg, id, landing, merged, o)
	if err != nil {
		return a, err
	}
	if ci.Code != 0 {
		a.Refused, a.Code, a.CI, a.PullRequest = found, ci.Code, &ci, pulled
		return a, nil
	}
	// The checks can take minutes, and the registry can change meanwhile: a
	// take or a queue change committed, someone's edit pulled in, another
	// worktree's write to a stealth registry. The close is made afresh on the
	// registry as it is now, read again and judged again, so a change made
	// during the wait is kept rather than written over with the text read
	// before it (bug 34), and an item no longer one work done may close is
	// refused, nothing written.
	if a, s, err = h.read(ctx, o); s == nil {
		return a, err
	}
	cfg = s.cfg
	if change, a, err = closing(a, s, id); change == nil {
		return a, err
	}
	if ci.Run != nil && ci.Run.URL != "" {
		if pulled != nil && pulled.Trusted {
			change.Body += fmt.Sprintf(" The CI run of %s, the head of %s's pull request %s, whose tree %s has but for the work registry and the ledger, passed: %s.",
				short(pulled.Head), id, pulled.URL, short(landing.sha), ci.Run.URL)
		} else {
			change.Body += " " + landing.runOf(ci.Run) + " passed: " + ci.Run.URL + "."
		}
	}
	files := []record.File{{Path: cfg.Work.Registry, Old: s.text, Text: change.Text}}
	checks, err := closedChecks(cfg, id, s.tracker.Load)
	if err != nil {
		return a, err
	}
	if checks != nil {
		files = append(files, record.File{Path: checks.Path, Old: checks.Old, Text: checks.Text})
		change.Body += " Its checks are taken out of " + checks.Path + ": they were its progress while it was open, and gated this close."
		if !cfg.Stealth {
			if head, err := cfg.VCS.Show(ctx, "HEAD"); err == nil {
				change.Body += " The commit before this one, " + head.Short + ", still holds them."
			}
		}
	}
	sha, found, err := writeAll(ctx, s, files, change.Header, change.Body, o)
	if err != nil || len(found) > 0 {
		return stopped(a, found, err)
	}
	line := fmt.Sprintf("%s is done: %s", id, committed(sha, change.Header))
	if checks != nil {
		line += "; its checks are out of " + checks.Path
	}
	fields := ci.Fields()
	if pulled != nil {
		fields = append(fields, out.Field{Key: "pull_request", Value: *pulled})
	}
	if len(ran.evidence) > 0 {
		fields = append(fields, out.Field{Key: "evidence", Value: ran.evidence})
	}
	if len(ran.late) > 0 {
		fields = append(fields, out.Field{Key: "left_to_ci", Value: ran.late})
	}
	unqueued, queueSHA, found, err := unqueueDone(ctx, s, id, o)
	if err != nil || len(found) > 0 {
		return stopped(a, found, err)
	}
	if unqueued != nil {
		line += "; out of the queue: " + committed(queueSHA, unqueued.Header)
		var commit any
		if queueSHA != "" {
			commit = queueSHA
		}
		fields = append(fields, out.Field{Key: "queue_commit", Value: commit})
	}
	a.Commit, a.Done = sha, &Closed{Line: line, Change: *change, Fields: fields}
	return a, nil
}

// unqueueDone takes the item just closed out of the registry's queue, in a
// commit of its own (slice 66): the queue holds the work still to do. The
// change is nil when the queue does not hold the item; the SHA is the
// commit's, "" under a stealth config, whose registry is written and not
// committed, its lock still held. The problems are the commit's refusal.
func unqueueDone(ctx context.Context, s *writing, id string, o Out) (*domain.Change, string, []out.Problem, error) {
	cfg := s.cfg
	raw, err := trackedText(ctx, s.tracker, cfg.Work.Registry)
	if err != nil {
		return nil, "", nil, err
	}
	registry, err := domain.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, "", nil, err
	}
	change, ok, err := domain.Unqueue(registry, raw, id)
	if err != nil {
		return nil, "", nil, uneditable(cfg.Work.Registry, err)
	}
	if !ok {
		return nil, "", nil, nil
	}
	sha, found, err := writeAll(ctx, s, []record.File{{Path: cfg.Work.Registry, Old: raw, Text: change.Text}}, change.Header, change.Body, o)
	if err != nil || len(found) > 0 {
		return nil, "", found, err
	}
	return &change, sha, nil, nil
}

// trackedText is the text of the document name, which the command found
// there: one gone since is an error naming it, fs.ErrNotExist.
func trackedText(ctx context.Context, t trackerport.Tracker, name string) (string, error) {
	docs, err := t.Load(ctx, []string{name})
	if err == nil && !docs[0].There {
		err = &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if err != nil {
		return "", err
	}
	return docs[0].Text, nil
}

// closing is the registry's change that makes the item done (work.Done),
// nil when there is none to make: a refusal (exit 1), an item already done
// (the Answer's Done saying so) or an error, with the Answer to end with.
func closing(a Answer, s *writing, id string) (*domain.Change, Answer, error) {
	change, found, err := domain.Done(s.cfg, s.registry, s.text, id)
	if err != nil {
		return nil, a, uneditable(s.cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		a.Refused, a.Code = found, refused
		return nil, a, nil
	}
	if change.Unchanged {
		a.Done = &Closed{Line: id + " is already done; nothing to change", Change: change}
		return nil, a, nil
	}
	return &change, a, nil
}

// wipScenarios are the problems of the item's scenarios at HEAD, those
// tagged @<id>, that are still @wip, and then of the Examples blocks still
// @wip under those that are live, each named by its name or, with none, by
// its line, <file>:<line>: a slice is done when every scenario it specified
// is live, and a block's rows never run while it is @wip (bug 129).
func wipScenarios(tagged []tests.Test, examples []tests.WipExample, id string) []out.Problem {
	var found []out.Problem
	for _, t := range tagged {
		if t.Live {
			continue
		}
		found = append(found, out.Problem{
			Rule:    "work-done-wip",
			Message: fmt.Sprintf("%s's scenario %s (%s) is still @wip at HEAD", id, t.ID, t.File),
			Fix:     wipFix(id),
		})
	}
	for _, e := range examples {
		block := fmt.Sprintf("%s:%d", e.Test.File, e.Line)
		if e.Name != "" {
			block = strconv.Quote(e.Name)
		}
		found = append(found, out.Problem{
			Rule:    "work-done-wip",
			Message: fmt.Sprintf("%s's scenario %s (%s) has its Examples block %s still @wip at HEAD", id, e.Test.ID, e.Test.File, block),
			Fix:     wipFix(id),
		})
	}
	return found
}

// wipFix is the fix line of a work-done-wip problem.
func wipFix(id string) string {
	return "implement it and commit it live, then run itos work done " + id + " again"
}

// doneCommit is the commit whose landing work done judges: its SHA, "" when
// HEAD names no commit, how work done names its CI run, at the start of a
// sentence, how it names the commit, "HEAD" or its short SHA and why, and
// whether it is the newest of the item's own commits (ownCommit).
type doneCommit struct {
	sha  string
	run  string
	what string
	own  bool
}

// ownCommit is the newest of the item's own commits in HEAD's history (slice
// 127): those whose footers link to it (linksTo, as work show lists them),
// its registry commits, which name it in their headers alone, aside, since a
// take or a promote changes nothing a run judges. The zero doneCommit when it
// has none, as a task taken and specified and nothing more has, or when the
// history cannot be read, as with no commit yet: judgedCommit judges it then.
func ownCommit(cfg *config.Loaded, id string, tagged []tests.Test) doneCommit {
	history, _ := message.History(cfg, "HEAD")
	links := linksTo(cfg, id, tagged)
	var j doneCommit
	for _, c := range history {
		if slices.ContainsFunc(c.Links, links) {
			what := c.Short + ", the newest commit of " + id
			j = doneCommit{sha: c.SHA, run: "The CI run of " + what + ",", what: what, own: true}
		}
	}
	return j
}

// runOf names the run that judged the commit, at the start of a sentence:
// its own, or the run of a head that has it (bug 49 and bug 41), naming both.
func (j doneCommit) runOf(run *providers.Run) string {
	if run == nil || run.HeadSHA == "" || run.HeadSHA == j.sha || j.what == "" {
		return j.run
	}
	return "The CI run of " + short(run.HeadSHA) + ", which has " + j.what + ","
}

// judgedCommit is HEAD, passing over the commits at HEAD that touch only the
// work registry (touchesOnlyRegistry, push's rule, slice 93): the newest
// commit of HEAD's first-parent line that touches more, its run named by
// the commit's short SHA when it is not HEAD. A history whose every commit touches only the
// registry has none, and HEAD is judged.
func judgedCommit(ctx context.Context, v vcsport.VCS, registry string) doneCommit {
	head, err := v.Resolve(ctx, "HEAD")
	if err != nil {
		return doneCommit{run: "HEAD's CI run"}
	}
	for at := head; ; {
		files, err := v.Diff(ctx, v.Parent(ctx, at), at)
		if !touchesOnlyRegistry(registry, files, err) {
			if at == head {
				return doneCommit{sha: head, run: "HEAD's CI run", what: "HEAD"}
			}
			what := short(at) + ", the newest commit of HEAD that touches more than " + registry
			return doneCommit{sha: at, run: "The CI run of " + what, what: what}
		}
		parent, err := v.Resolve(ctx, at+"^")
		if err != nil {
			return doneCommit{sha: head, run: "HEAD's CI run", what: "HEAD"}
		}
		at = parent
	}
}

// touchesOnlyRegistry is whether the paths some commits touch, each against
// its first parent, are the work registry and no other path: the one rule by
// which itos push does not wait for a run (slice 56) and work done passes
// over a commit (slice 93), since such a commit changes nothing a run
// judges. A merge counts what it brought in against its first parent, and a
// commit that touches no path adds nothing to the answer, so commits that
// touch nothing at all are not registry-only; nor are commits whose paths
// cannot be read.
func touchesOnlyRegistry(registry string, files []string, err error) bool {
	if err != nil {
		return false
	}
	registry = path.Clean(filepath.ToSlash(registry))
	touched := false
	for _, f := range files {
		if path.Clean(f) != registry {
			return false
		}
		touched = true
	}
	return touched
}

// unpushed is the problem of commits of the judged commit that no remote
// has: work is landed when it is pushed. The commits after it touch only the
// registry, or are not the item's (slice 127), and are not asked about. With
// no remote at all there is nowhere to push to, and done says so on stderr
// and goes on.
func unpushed(ctx context.Context, v vcsport.VCS, id string, j doneCommit, stderr io.Writer) []out.Problem {
	if j.sha == "" {
		return nil
	}
	if remotes, err := v.Remotes(ctx); err == nil && len(remotes) == 0 {
		fmt.Fprintln(stderr, "itos: the repository has no remote, so whether its commits are pushed was not checked")
		return nil
	}
	commits, err := v.Commits(ctx, vcsport.Range{From: vcsport.Unpushed, To: j.sha})
	if err != nil || len(commits) == 0 {
		return nil
	}
	what := "a commit"
	if len(commits) > 1 {
		what = fmt.Sprintf("%d commits", len(commits))
	}
	holder := "HEAD"
	if j.own {
		holder = "The history up to " + j.what + ","
	}
	return []out.Problem{{
		Rule:    "work-done-unpushed",
		Message: fmt.Sprintf("%s has %s no remote has, so %s has not landed", holder, what, id),
		Fix:     "push them with itos push, then run itos work done " + id + " again",
	}}
}

// taskRun is what work done found of a task's checks that passed: the
// evidence of those looked up and the late checks left to CI.
type taskRun struct {
	evidence []evidence.Entry
	late     []leftToCI
}

// leftToCI is a late check of the task: CI runs it, work done does not.
type leftToCI struct {
	Task    string `json:"task"`
	Command string `json:"command"`
}

// say names each late check as left to CI, on stdout, or stderr under JSON
// as the watch's lines go, and not at all under -q.
func (r taskRun) say(id string, o Out) {
	w := o.Stdout
	if o.JSON {
		w = o.Stderr
	}
	if o.Quiet {
		return
	}
	for _, c := range r.late {
		fmt.Fprintf(w, "%s's check `%s` is late: left to CI\n", id, c.Command)
	}
}

// failingTask is the problem of a task whose static checks fail, every one
// run in written order through the evidence engine on HEAD's tree (TaskChecks),
// each under its own timeout, the first failure's output on stderr. A task
// that passes names its late checks, left to CI. An item that is no task of
// the ledger has none, nor has a config with no ledger.id.
func (h Handler) failingTask(ctx context.Context, cfg *config.Loaded, id string, stderr io.Writer) (taskRun, []out.Problem, error) {
	var ran taskRun
	if cfg.Ledger.ID == nil || !ledger.IDPattern(cfg).MatchString(id) {
		return ran, nil, nil
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return ran, nil, err
	}
	for _, task := range tasks {
		if task.ID != id {
			continue
		}
		checked, err := h.ports.TaskChecks(ctx, cfg, task, stderr)
		ran.evidence = checked.Evidence
		if err != nil {
			return ran, nil, err
		}
		if f := checked.Failure; f != nil {
			return ran, []out.Problem{refusal(id, f)}, nil
		}
		for _, c := range check.LateChecks(cfg, task) {
			ran.late = append(ran.late, leftToCI{Task: id, Command: c.Check.Command()})
		}
	}
	return ran, nil, nil
}

// refusal is the problem of a task check that failed: its words, and under
// --json its task, command, exit (null when it did not exit by itself),
// timeout and output tail.
func refusal(id string, f *judge.Failure) out.Problem {
	run := out.CheckRun{Task: f.Task, Command: f.Command, TimedOut: f.TimedOut, Output: f.Output}
	if !f.TimedOut && f.Code >= 0 {
		run.Exit = &f.Code
	}
	return out.Problem{
		Rule:     "work-done-task-check",
		Message:  fmt.Sprintf("%s's check fails: %s", id, f.Why),
		Fix:      "itos task " + id + " shows what its checks want",
		CheckRun: &run,
	}
}

// closedChecks is the ledger with the closed task's checks taken out
// (ledger.WithoutChecks, slice 101), read now, after the wait, as the
// registry is: nil for an item that is no task of the ledger, a config with
// no ledger.id, or a task with no checks, whose close leaves the ledger as
// it is.
func closedChecks(cfg *config.Loaded, id string, load ledger.Load) (*ledger.Edited, error) {
	if cfg.Ledger.ID == nil || !ledger.IDPattern(cfg).MatchString(id) {
		return nil, nil
	}
	edited, ok, err := ledger.WithoutChecks(cfg, id, load)
	if err != nil || !ok {
		return nil, err
	}
	return &edited, nil
}

// landedCI is the judged commit's CI run with ci.watch: under push.mode pr,
// the merged pull request's head's run when it judges the commit (trustedRun,
// slice 181); else, for the newest of the item's own commits, any run that
// covers it and passed (passedRun, slice 127); else, or while there is none,
// its run, waited for as itos ci watch waits, or, while it has none, the run
// on its upstream branch that covers it (bug 49): Code 0 when it passed, else
// the refusal (1 for a run that did not pass, its problem naming its address,
// 3 for one that did not end or could not be looked at). The PullRequest is
// the merged pull request with a head, nil without one or without ci.watch.
// Without ci.watch nothing is watched, and done says so on stderr and goes on,
// its --json ci "unwatched".
func (h Handler) landedCI(ctx context.Context, cfg *config.Loaded, id string, j doneCommit, m *mergedHead, o Out) (ciwatch.Watched, *PullRequest, []out.Problem, error) {
	if cfg.CI.Watch.Provider == "none" {
		fmt.Fprintln(o.Stderr, "itos: ci.watch.provider is none, so CI was not checked")
		return ciwatch.Watched{Outcome: "unwatched"}, nil, nil, nil
	}
	sha := j.sha
	if sha == "" {
		return ciwatch.Watched{}, nil, nil, errors.New("HEAD names no commit, so it has no CI run")
	}
	remote, branch := "origin", ""
	if local := cfg.VCS.Branch(ctx); local != "" {
		var ref string
		remote, ref, _ = cfg.VCS.Upstream(ctx, local)
		branch = strings.TrimPrefix(ref, "refs/heads/")
	}
	var pulled *PullRequest
	if m != nil {
		pulled = &PullRequest{URL: m.url, Head: m.head}
	}
	w := ciwatch.Watched{Code: missing, Outcome: "error"}
	wr, _, err := h.ports.Watcher(ctx, cfg, remote)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
	} else if trusted, ok := trustedRun(ctx, cfg, id, m, j, wr, o); ok {
		w, pulled.Trusted = trusted, true
	} else if passed, ok := passedRun(ctx, cfg.VCS, wr, j, remote, branch, o); ok {
		w = passed
	} else {
		w = ciwatch.Watch(ctx, cfg, wr, cfg.VCS, sha, remote, branch, watchOut(o))
	}
	if w.Code == 0 || w.Outcome != "failure" {
		return w, pulled, nil, nil
	}
	return w, pulled, []out.Problem{{
		Rule:    "work-done-ci",
		Message: fmt.Sprintf("%s did not pass, so %s is not done: %s", j.runOf(w.Run), id, w.Run.URL),
		Fix:     "fix what failed, push the fix with itos push, then run itos work done " + id + " again",
	}}, nil
}

// passedRun is a run that passed and covers the newest of the item's own
// commits (slice 127): the commit's own run when it passed, else the newest
// run on the branch that passed and whose head has it (ciwatch.Covering).
// ok is false for a commit that is not the item's own, and when no such run
// is found or a look fails: ciwatch.Watch then waits for the commit's run,
// or the newest that covers it, as before, and reports what it finds.
func passedRun(ctx context.Context, v vcsport.VCS, wr providers.Watcher, j doneCommit, remote, branch string, o Out) (ciwatch.Watched, bool) {
	if !j.own {
		return ciwatch.Watched{}, false
	}
	run, _, _ := wr.Look(j.sha)
	if run.Conclusion != "success" {
		run, _, _ = ciwatch.Covering(ctx, wr.Runs, v, branch, j.sha, remote, nil, "success")
	}
	if run.Conclusion != "success" {
		return ciwatch.Watched{}, false
	}
	end := o.Stdout
	if o.JSON {
		end = o.Stderr
	}
	return ciwatch.Ended(run, watchOut(o), end), true
}

// watchOut is o as the watch engine's writers and flags.
func watchOut(o Out) ciwatch.Out {
	return ciwatch.Out{Stdout: o.Stdout, Stderr: o.Stderr, JSON: o.JSON, Quiet: o.Quiet}
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
