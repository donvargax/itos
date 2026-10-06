package cli

// work done (slice 53, features/work.feature): the landing's check, run by
// the agent after its push, then the item marked done and the registry
// committed alone, as work take commits it (workwrite.go). An item is done
// when its work has landed: none of its scenarios (those tagged @<id>, so
// @slice-<n> for the item slice-<n>) still @wip, no commit of HEAD that no
// remote has, a task's static checks passing as the commit-msg hook runs
// them, and with ci.watch HEAD's CI run green, waited for as itos ci watch
// waits when it is still going. The first that is not refuses, naming what
// to do; without ci.watch CI is not checked, and done says so. The registry
// is then read again and the close made on it as it is after the wait (bug
// 34). An item the registry's queue holds is then taken out of it, in a
// commit of its own (slice 66).

import (
	"fmt"
	"os"
	"strings"

	"github.com/donvargax/itos/v6/internal/check"
	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/ledger"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/tests"
	"github.com/donvargax/itos/v6/internal/work"
)

// workDone is `work done <id>`: the item done and the registry committed
// (work.Done), once its work has landed.
func workDone(args []string, o Out) (int, error) {
	id, _, err := workArgs("done", args)
	if err != nil {
		return 0, err
	}
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer func() { release() }()
	if cfg == nil {
		return code, err
	}
	change, code, err := doneChange(cfg, registry, text, id, o)
	if change == nil {
		return code, err
	}
	if cfg.Stealth {
		// The checks below run the task's checks and ask CI, which can take
		// minutes: the stealth registry's lock is given back meanwhile, and
		// taken again for the write below (bug 16).
		release()
		release = func() {}
	}
	found, err := wipScenarios(cfg, id)
	if err != nil {
		return 0, err
	}
	if len(found) == 0 {
		found = unpushed(id, o)
	}
	if len(found) == 0 {
		if found, err = failingTask(cfg, id, o); err != nil {
			return 0, err
		}
	}
	if len(found) > 0 {
		return refuseWork(found, ExitPolicy, o)
	}
	ci, code, err := landedCI(cfg, id, o)
	if err != nil || code != 0 {
		return code, err
	}
	// The checks can take minutes, and the registry can change meanwhile: a
	// take or a queue change committed, someone's edit pulled in, another
	// worktree's write to a stealth registry. The close is made afresh on the
	// registry as it is now, read again and judged again, so a change made
	// during the wait is kept rather than written over with the text read
	// before it (bug 34), and an item no longer one work done may close is
	// refused, nothing written.
	if cfg, registry, text, release, code, err = soundRegistry(o); cfg == nil {
		return code, err
	}
	if change, code, err = doneChange(cfg, registry, text, id, o); change == nil {
		return code, err
	}
	if ci.run != nil && ci.run.URL != "" {
		change.Body += " HEAD's CI run passed: " + ci.run.URL + "."
	}
	sha, code, err := writeRegistry(cfg, text, *change, o)
	if err != nil || code != 0 {
		return code, err
	}
	line := fmt.Sprintf("%s is done: %s", id, committed(sha, change.Header))
	fields := ci.fields()
	unqueued, queueSHA, code, err := unqueueDone(cfg, id, o)
	if err != nil || code != 0 {
		return code, err
	}
	if unqueued != nil {
		line += "; out of the queue: " + committed(queueSHA, unqueued.Header)
		var commit any
		if queueSHA != "" {
			commit = queueSHA
		}
		fields = append(fields, out.Field{Key: "queue_commit", Value: commit})
	}
	return reportWork(line, *change, sha, fields, o)
}

// unqueueDone takes the item just closed out of the registry's queue, in a
// commit of its own (slice 66): the queue holds the work still to do. The
// change is nil when the queue does not hold the item; the SHA is the
// commit's, "" under a stealth config, whose registry is written and not
// committed, its lock still held.
func unqueueDone(cfg *config.Loaded, id string, o Out) (*work.Change, string, int, error) {
	raw, err := os.ReadFile(cfg.Work.Registry)
	if err != nil {
		return nil, "", 0, err
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, "", 0, err
	}
	change, ok, err := work.Unqueue(registry, string(raw), id)
	if err != nil {
		return nil, "", 0, uneditable(cfg.Work.Registry, err)
	}
	if !ok {
		return nil, "", 0, nil
	}
	sha, code, err := writeRegistry(cfg, string(raw), change, o)
	if err != nil || code != 0 {
		return nil, "", code, err
	}
	return &change, sha, 0, nil
}

// doneChange is the registry's change that makes the item done (work.Done),
// nil when there is none to make: a refusal, an item already done (reported
// as such) or an error, with the exit code and error to end with.
func doneChange(cfg *config.Loaded, registry work.Registry, text, id string, o Out) (*work.Change, int, error) {
	change, problem, err := work.Done(registry, text, id)
	if err != nil {
		return nil, 0, uneditable(cfg.Work.Registry, err)
	}
	if problem != nil {
		code, err := refuseWork([]out.Problem{*problem}, ExitPolicy, o)
		return nil, code, err
	}
	if change.Unchanged {
		code, err := reportWork(id+" is already done; nothing to change", change, "", nil, o)
		return nil, code, err
	}
	return &change, 0, nil
}

// wipScenarios are the problems of the item's scenarios at HEAD, those
// tagged @<id>, that are still @wip: a slice is done when every scenario it
// specified is live.
func wipScenarios(cfg *config.Loaded, id string) ([]out.Problem, error) {
	tagged, err := tests.Tagged(cfg, "@"+id, "HEAD")
	if err != nil {
		return nil, err
	}
	var found []out.Problem
	for _, t := range tagged {
		if t.Live {
			continue
		}
		found = append(found, out.Problem{
			Rule:    "work-done-wip",
			Message: fmt.Sprintf("%s's scenario %s (%s) is still @wip at HEAD", id, t.ID, t.File),
			Fix:     "implement it and commit it live, then run itos work done " + id + " again",
		})
	}
	return found, nil
}

// unpushed is the problem of commits of HEAD that no remote has: work is
// landed when it is pushed. With no remote at all there is nowhere to push
// to, and done says so and goes on.
func unpushed(id string, o Out) []out.Problem {
	if !git.Succeeds("rev-parse", "--verify", "--quiet", "HEAD") {
		return nil
	}
	if remotes, err := git.Output("remote"); err == nil && strings.TrimSpace(remotes) == "" {
		fmt.Fprintln(o.Stderr, "itos: the repository has no remote, so whether its commits are pushed was not checked")
		return nil
	}
	commits, err := git.Lines("rev-list", "HEAD", "--not", "--remotes")
	if err != nil || len(commits) == 0 {
		return nil
	}
	what := "a commit"
	if len(commits) > 1 {
		what = fmt.Sprintf("%d commits", len(commits))
	}
	return []out.Problem{{
		Rule:    "work-done-unpushed",
		Message: fmt.Sprintf("HEAD has %s no remote has, so %s has not landed", what, id),
		Fix:     "push them with itos push, then run itos work done " + id + " again",
	}}
}

// failingTask is the problem of a task whose static checks fail, run as the
// commit-msg hook runs them (firstFailure, capped by
// hooks.commit_msg.check_timeout), each failure's output on stderr. An item
// that is no task of the ledger has none, nor has a config with no ledger.id.
func failingTask(cfg *config.Loaded, id string, o Out) ([]out.Problem, error) {
	if cfg.Ledger.ID == nil || !ledger.IDPattern(cfg).MatchString(id) {
		return nil, nil
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task.ID != id {
			continue
		}
		limit := check.NoCap
		if t := cfg.Hooks.CommitMsg.CheckTimeout; t != nil {
			limit = *t
		}
		why, err := firstFailure(cfg, task, limit, checkEnv(), o.Stderr)
		if err != nil || why == "" {
			return nil, err
		}
		return []out.Problem{{
			Rule:    "work-done-task-check",
			Message: fmt.Sprintf("%s's check fails: %s", id, why),
			Fix:     "itos task " + id + " shows what its checks want",
		}}, nil
	}
	return nil, nil
}

// landedCI is HEAD's CI run with ci.watch, waited for as itos ci watch
// waits: 0 when it passed, else the refusal (1 for a run that did not pass,
// naming its address, 3 for one that did not end or could not be looked
// at), reported. Without ci.watch nothing is watched, and done says so on
// stderr and goes on, its --json ci "unwatched".
func landedCI(cfg *config.Loaded, id string, o Out) (watched, int, error) {
	if cfg.CI.Watch.Provider == "none" {
		fmt.Fprintln(o.Stderr, "itos: ci.watch.provider is none, so CI was not checked")
		return watched{outcome: "unwatched"}, 0, nil
	}
	sha, err := git.Output("rev-parse", "HEAD")
	if err != nil {
		return watched{}, 0, err
	}
	sha = strings.TrimSpace(sha)
	remote := "origin"
	if branch := git.Branch(); branch != "" {
		remote, _, _ = git.Upstream(branch)
	}
	w := watched{code: ExitMissing, outcome: "error"}
	look, _, err := watcher(cfg, remote, o)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
	} else {
		w = watchRun(cfg, look, sha, o)
	}
	if w.code == 0 {
		return w, 0, nil
	}
	var found []out.Problem
	if w.outcome == "failure" {
		found = []out.Problem{{
			Rule:    "work-done-ci",
			Message: fmt.Sprintf("HEAD's CI run did not pass, so %s is not done: %s", id, w.run.URL),
			Fix:     "fix what failed, push the fix with itos push, then run itos work done " + id + " again",
		}}
	}
	if o.JSON {
		if found == nil {
			found = []out.Problem{}
		}
		fields := append([]out.Field{{Key: "ok", Value: false}, {Key: "problems", Value: found}}, w.fields()...)
		return w, w.code, out.Emit(o.Stdout, fields...)
	}
	code, err := refuseWork(found, w.code, o)
	return w, code, err
}
