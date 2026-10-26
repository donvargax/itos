package work

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/prtrust"
	"github.com/donvargax/itos/v8/internal/providers"
)

// mergedHead is the head of the item's merged pull request under push.mode
// pr (slice 181): its address, its number, its head's full SHA, the remote it
// merged on and <remote>/<main>, which holds the item's commit.
type mergedHead struct {
	url, head, remote, on string
	number                int
}

// headOf is the head of the merged pull request, nil when gh gave it none.
func headOf(p *pull, remote, main string) *mergedHead {
	if p == nil || p.HeadRefOid == "" {
		return nil
	}
	return &mergedHead{url: p.URL, head: p.HeadRefOid, remote: remote, on: remote + "/" + main, number: p.Number}
}

// PullRequest is --json's pull_request of a close, or of work-done-ci's
// refusal: the merged pull request, its head's full SHA, and whether the
// head's CI run judged the close.
type PullRequest struct {
	URL     string `json:"url"`
	Head    string `json:"head"`
	Trusted bool   `json:"trusted"`
}

// trustedRun is the merged pull request's head's CI run when it judges the
// landing commit (slice 181), ended as the watch ends a run: the tree the
// merge made differs from the head's only in the work registry and ledger
// files, and the head's run, looked at once, completed with success and lists
// a job that was not skipped. A head the clone lacks is fetched first, once,
// from refs/pull/<n>/head, which GitHub keeps after the branch is gone. For a
// task of the ledger whose files the diff lists, its own checks must also be
// the same at the head and at the commit (prtrust.SameChecks, bug 137). ok is false,
// and a line says why, where the watch's progress goes, for a head the clone
// cannot have, a tree that differs in another path, the item's checks that
// differ or a ledger that cannot be read, and a head's run that is no pass; a
// diff git cannot give is no comparison, said nowhere.
func trustedRun(ctx context.Context, cfg *config.Loaded, id string, m *mergedHead, landing doneCommit, wr providers.Watcher, o Out) (ciwatch.Watched, bool) {
	if m == nil {
		return ciwatch.Watched{}, false
	}
	v := cfg.VCS
	say, end := progress(o), o.Stdout
	if o.JSON {
		end = o.Stderr
	}
	h, l := short(m.head), short(landing.sha)
	judges := fmt.Sprintf("so %s's CI run judges %s", m.on, l)
	merged := fmt.Sprintf("%s's pull request %s merged with the head %s", id, m.url, h)
	if _, err := v.Resolve(ctx, m.head); err != nil {
		prtrust.FetchHead(ctx, v, m.remote, m.number)
		if _, err := v.Resolve(ctx, m.head); err != nil {
			fmt.Fprintf(say, "%s, which this clone does not have and could not fetch from %s's refs/pull/%d/head, %s\n", merged, m.remote, m.number, judges)
			return ciwatch.Watched{}, false
		}
	}
	files, err := v.Diff(ctx, m.head, landing.sha)
	if err != nil {
		return ciwatch.Watched{}, false
	}
	merged += fmt.Sprintf(", and %s, %s's newest commit on %s,", l, id, m.on)
	others, ledgers := prtrust.OutsideBookkeeping(cfg, files)
	if len(others) > 0 {
		more := ""
		if len(others) > 1 {
			more = fmt.Sprintf(" and %d more", len(others)-1)
		}
		fmt.Fprintf(say, "%s differs from it in %s%s, %s\n", merged, others[0], more, judges)
		return ciwatch.Watched{}, false
	}
	merged += " has its tree but for the work registry and the ledger"
	if ledgers && isTask(cfg, id) {
		same, unreadable := prtrust.SameChecks(ctx, cfg, id, m.head, landing.sha)
		if unreadable != "" {
			fmt.Fprintf(say, "%s, but the ledger at %s cannot be read, %s\n", merged, short(unreadable), judges)
			return ciwatch.Watched{}, false
		}
		if !same {
			fmt.Fprintf(say, "%s, but %s's checks in the ledger differ from the head's, %s\n", merged, id, judges)
			return ciwatch.Watched{}, false
		}
	}
	run, found, err := wr.Look(m.head)
	if err != nil || !found {
		fmt.Fprintf(say, "%s, but no CI run of the head was found, %s\n", merged, judges)
		return ciwatch.Watched{}, false
	}
	if run.Conclusion != "success" || ciwatch.DraftsRun(run) {
		fmt.Fprintf(say, "%s, but the head's CI run %s has not passed, %s\n", merged, run.URL, judges)
		return ciwatch.Watched{}, false
	}
	fmt.Fprintf(say, "%s, so the head's CI run judges %s\n", merged, l)
	return ciwatch.Ended(run, watchOut(o), end), true
}

// progress is where the watch's progress goes: stdout, stderr under --json,
// nowhere under -q.
func progress(o Out) io.Writer {
	switch {
	case o.Quiet:
		return io.Discard
	case o.JSON:
		return o.Stderr
	}
	return o.Stdout
}
