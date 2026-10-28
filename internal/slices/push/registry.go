package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// plan is how an item's push divides the commits it sends (slice-192): main
// is the head main is pushed to, "" when it gets nothing; item the head the
// item's branch is pushed to, which is the remote's own when it gets nothing;
// first the commit that starts what the pull request carries; moved whether
// registry commits left the branch, so the clone's branch ends at item; bare
// whether nothing is left for the item's branch to gain; and sent the "Sent
// ..." lines for the commits each branch gains.
type plan struct {
	main, item, first string
	moved, bare       bool
	sent              []string
}

// divide puts the commits of an item's push where they go (slice-192): those
// the remote's item branch lacks, all of them with no such branch. The prefix,
// the registry's commits that start at main's head, is built as it has always
// been: main and the item's branch both get them, under the same SHAs, with
// the work on top unchanged. Every other registry commit is carried onto
// main's head, after the prefix, in order, and left out of the item's branch,
// whose other new commits go onto the remote's item branch (or the prefix, or
// main) in theirs. A commit the remote's item branch has stays as it is, and
// so does one nothing was taken from under. Nothing is pushed or changed in the
// clone, so a commit that does not apply ends the push with its refusal: done.
func (r *pushRun) divide(ctx context.Context, main, theirMain, tip string, exists bool, ahead []string, prefix int) (p plan, a Answer, done bool) {
	v := r.ports
	kept := 0
	for kept < len(ahead) && exists && v.IsAncestor(ctx, ahead[kept], tip) {
		kept++
	}
	mainTip, itemTip := theirMain, theirMain
	switch {
	case prefix > 0:
		mainTip, itemTip = ahead[prefix-1], ahead[prefix-1]
	case kept > 0:
		itemTip = ahead[kept-1]
		p.first = ahead[0]
	}
	mainNew, itemNew := slices.Clone(ahead[:prefix]), slices.Clone(ahead[:prefix])
	var moved []string
	left := 0
	for _, c := range ahead[max(prefix, kept):] {
		if r.registryCommit(ctx, c) {
			made, conflicts, bad, failed := r.carry(ctx, c, mainTip)
			if failed {
				return p, bad, true
			}
			if conflicts != nil {
				return p, r.registryCannotMove(ctx, c, conflicts, mainTip, main), true
			}
			mainTip = made
			mainNew = append(mainNew, made)
			moved = append(moved, c)
			continue
		}
		if p.first == "" {
			p.first = c
		}
		if len(moved) > 0 && v.Parent(ctx, c) != itemTip {
			made, conflicts, bad, failed := r.carry(ctx, c, itemTip)
			if failed {
				return p, bad, true
			}
			if conflicts != nil {
				return p, r.needsRegistryCommit(ctx, c, moved, conflicts), true
			}
			c = made
		}
		itemTip = c
		itemNew = append(itemNew, c)
		left++
	}
	p.item, p.moved, p.bare = itemTip, len(moved) > 0, left == 0
	if len(mainNew) > 0 {
		p.main = mainTip
		for _, c := range mainNew {
			p.sent = append(p.sent, r.sentLine(ctx, c, main))
		}
		if !exists || tip != itemTip {
			for _, c := range itemNew {
				p.sent = append(p.sent, r.sentLine(ctx, c, r.branch))
			}
		}
	}
	return p, Answer{}, false
}

// carry makes the commit again on onto (vcsport.VCS.CherryPick), with its itos
// note when it has one, so that a push whose footers sit in notes judges the
// new commit as the old. A change that does not apply is conflicts, and the
// caller's to word; any other failure is failed, reported with nothing pushed.
func (r *pushRun) carry(ctx context.Context, commit, onto string) (made string, conflicts *vcsport.Conflicts, a Answer, failed bool) {
	v := r.ports
	made, err := v.CherryPick(ctx, commit, onto)
	if errors.As(err, &conflicts) {
		return "", conflicts, Answer{}, false
	}
	if entry, shown := v.Show(ctx, commit); err == nil && shown == nil && entry.Note != "" {
		err = v.Note(ctx, made, entry.Note)
	}
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot carry %s onto %s: %s\n", short(commit), short(onto), err)
		return "", nil, Answer{Code: missing}, true
	}
	return made, nil, Answer{}, false
}

// sentLine says a commit was sent to a branch of the remote.
func (r pushRun) sentLine(ctx context.Context, commit, branch string) string {
	shown, _ := r.ports.Show(ctx, commit)
	return fmt.Sprintf("Sent %s %s to %s/%s.", short(commit), shown.Subject, r.remote, branch)
}

// refuse is a refusal of the push before anything is pushed (exit 1): under
// --json its lines also go to stderr, since --json's stdout is one object.
func (r pushRun) refuse(outcome string, lines ...string) Answer {
	if r.o.JSON {
		for _, l := range lines {
			fmt.Fprintln(r.o.Stderr, l)
		}
	}
	return r.report(policy, outcome, lines...)
}

// named is a commit as the refusals name it: its short SHA and subject.
func (r pushRun) named(ctx context.Context, commit string) string {
	shown, _ := r.ports.Show(ctx, commit)
	return short(commit) + " " + shown.Subject
}

// registryCannotMove is the refusal of a registry commit that goes to main and
// does not apply onto its head (outcome registry-conflict, exit 1), as the
// rebase onto main refuses one: the work registry named when it is among the
// conflicts, and each item both sides changed with the owner main gives it.
// No rebase is in progress, so it does not advise git rebase --skip but
// dropping the commit from the branch.
func (r pushRun) registryCannotMove(ctx context.Context, commit string, conflicts *vcsport.Conflicts, mainTip, main string) Answer {
	v := r.ports
	onto := r.remote + "/" + main
	registry := path.Clean(filepath.ToSlash(r.cfg.Work.Registry))
	lines := []string{fmt.Sprintf("itos push: %s is the registry's and goes to %s, where it does not apply; nothing was pushed.", r.named(ctx, commit), onto)}
	if slices.ContainsFunc(conflicts.Paths, func(p string) bool { return path.Clean(p) == registry }) {
		show := func(rev string) string {
			text, _, _ := v.File(ctx, rev, registry)
			return text
		}
		clashes := work.Clashes(show(v.Parent(ctx, commit)), show(mainTip), show(commit))
		if len(clashes) == 0 {
			lines = append(lines, fmt.Sprintf("The conflict is in the work registry, %s: a commit on %s changed it near where this commit does.", registry, onto))
		} else {
			lines = append(lines, fmt.Sprintf("The conflict is in the work registry, %s: a commit on %s changed the same item as this commit,", registry, onto),
				"so it was taken, or changed, there first, and the remote keeps that.")
		}
		for _, c := range clashes {
			if c.Taken && c.Owner != "" {
				lines = append(lines, fmt.Sprintf("  %s is taken: a commit on %s made %s its owner first.", c.ID, onto, c.Owner))
			} else {
				lines = append(lines, fmt.Sprintf("  %s was changed on %s too.", c.ID, onto))
			}
		}
	} else {
		lines = append(lines, fmt.Sprintf("It conflicts in %s with a commit on %s.", strings.Join(conflicts.Paths, ", "), onto))
	}
	sha := short(commit)
	return r.refuse("registry-conflict", append(lines,
		fmt.Sprintf("To leave it to them, drop the commit from the branch with git rebase --onto %s^ %s, then itos push again;", sha, sha),
		"to keep yours, make the change again on top of "+onto+" (git switch main, make it there, itos push).")...)
}

// needsRegistryCommit is the refusal of a commit that is not the registry's and
// does not apply onto the item's branch once the registry's commits are taken
// out from under it (outcome needs-registry-commit, exit 1): it names the commit
// and the registry commit it needs, the latest taken out that changes a path
// the commit conflicts in, else the latest, and says to make that one on main
// or fold the two; one is always taken out, since only a push that took one out
// carries a commit again.
func (r pushRun) needsRegistryCommit(ctx context.Context, commit string, moved []string, conflicts *vcsport.Conflicts) Answer {
	v := r.ports
	needs := moved[len(moved)-1]
	for _, m := range moved {
		files, _ := v.Diff(ctx, v.Parent(ctx, m), m)
		if slices.ContainsFunc(files, func(f string) bool { return slices.Contains(conflicts.Paths, f) }) {
			needs = m
		}
	}
	return r.refuse("needs-registry-commit",
		fmt.Sprintf("itos push: %s does not apply without %s, which is the registry's and goes to %s/main, not the item's branch; nothing was pushed.",
			r.named(ctx, commit), r.named(ctx, needs), r.remote),
		fmt.Sprintf("Make the registry commit on main instead (git switch main, make it again there, itos push), or fold %s and %s into one commit.",
			short(needs), short(commit)))
}

// resetTo sets the clone's branch to the head pushed to the item's branch once
// the push is done, without the registry commits that went to main.
func (r *pushRun) resetTo(ctx context.Context, head string) (Answer, bool) {
	if err := r.ports.Reset(ctx, head); err != nil {
		io.WriteString(r.o.Stderr, "itos: the commits are pushed, but the branch could not be set to "+short(head)+": "+err.Error()+"\n")
		return Answer{Code: missing}, true
	}
	return Answer{}, false
}
