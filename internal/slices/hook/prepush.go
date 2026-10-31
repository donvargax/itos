package hook

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/engine/rewrite"
	"github.com/donvargax/itos/v8/internal/shell"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// zeros is a SHA git writes for no commit: a new ref's remote side, a
// deleted ref's local side.
var zeros = regexp.MustCompile(`^0+$`)

// tags and heads are where a tag's and a branch's ref are on the remote.
const (
	tags  = "refs/tags/"
	heads = "refs/heads/"
)

// pushLine is one of git's `<local ref> <local sha> <remote ref> <remote
// sha>` lines, the fields it has: what is pushed, to which ref of the remote
// and what that ref is at there.
type pushLine struct{ local, ref, remote string }

// deleted is whether the line deletes the remote's ref: it sends no commit.
func (l pushLine) deleted() bool { return l.local == "" || zeros.MatchString(l.local) }

// existing is whether the remote has the ref already.
func (l pushLine) existing() bool { return l.remote != "" && !zeros.MatchString(l.remote) }

// pushLines are git's ref lines, a line's carriage return, as a Windows
// editor or pipe may leave one, not part of its last field.
func pushLines(input string) []pushLine {
	var lines []pushLine
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, " ")
		field := func(i int) string {
			if i < len(fields) {
				return fields[i]
			}
			return ""
		}
		lines = append(lines, pushLine{local: field(1), ref: field(2), remote: field(3)})
	}
	return lines
}

// pushedRef is one ref a push sends: the commit it sends, the remote's
// commit it replaces when this clone has that one, else "" (a new ref, or a
// remote commit the clone lacks), and the remote ref with the commit it is
// at, "" for a new ref.
type pushedRef struct{ local, base, ref, remote string }

// pushedRefs are the refs a push sends, a deleted one left out, since it
// sends nothing.
func (h Handler) pushedRefs(ctx context.Context, lines []pushLine) []pushedRef {
	var refs []pushedRef
	for _, l := range lines {
		if l.deleted() {
			continue
		}
		ref := pushedRef{local: l.local, ref: l.ref}
		if l.existing() {
			ref.remote = l.remote
			if hasCommit(ctx, h.ports, l.remote) {
				ref.base = l.remote
			}
		}
		refs = append(refs, ref)
	}
	return refs
}

// movedTag is the first line, a deletion's too, that would update a tag the
// remote already has (slice 182): a tag is never moved or deleted, whether
// or not the new commit builds on the old and whether or not this clone has
// the old one.
func movedTag(lines []pushLine) (pushLine, bool) {
	for _, l := range lines {
		if strings.HasPrefix(l.ref, tags) && l.existing() {
			return l, true
		}
	}
	return pushLine{}, false
}

// refuseTag is the hook's refusal of a push that moves or deletes the
// remote's tag: it names the ref and the commits, and says there is no way
// round it but the remote's host. Nothing here offers itos push, which has
// nothing to offer a tag.
func refuseTag(l pushLine) Answer {
	var said strings.Builder
	what := fmt.Sprintf("move the remote's tag %s (at %s) to %s", l.ref, short(l.remote), short(l.local))
	if l.deleted() {
		what = fmt.Sprintf("delete the remote's tag %s (at %s)", l.ref, short(l.remote))
	}
	fmt.Fprintf(&said, "itos: the push would %s; a tag the remote has is never moved or deleted, so the push is "+
		"refused, and hooks.pre_push's commands did not run.\n", what)
	if !l.deleted() {
		fmt.Fprintln(&said, "Tag the new commit with a new name instead.")
	}
	return Answer{Code: policy, Said: said.String()}
}

// hasCommit is whether rev names a commit v has.
func hasCommit(ctx context.Context, v vcsport.VCS, rev string) bool {
	_, err := v.Resolve(ctx, rev)
	return err == nil
}

// rewrites judges the refs that would replace a remote commit the commit
// pushed does not build on (slice 128): one this clone lacks, or one that is
// not an ancestor of the commit pushed. Such a push rewrites what others may
// have built on, which only a forced push does, so the first is refused,
// unless it is a branch's own rewrite (ownRewrite, slice-193); an accepted one
// is returned as a new branch, to be judged as one is: by the commits no
// remote-tracking branch has, and the remote commits those grow from. Every
// ref that is a new branch or adds to the remote's commit is returned as it
// was.
func (h Handler) rewrites(ctx context.Context, remote string, refs []pushedRef) ([]pushedRef, Answer, bool) {
	judged := slices.Clone(refs)
	for i, r := range refs {
		if r.remote == "" || r.base != "" && h.ports.IsAncestor(ctx, r.base, r.local) {
			continue
		}
		dropped, ok := h.ownRewrite(ctx, remote, r)
		if !ok {
			return nil, refuseRewrite(r, dropped), true
		}
		judged[i] = pushedRef{local: r.local, ref: r.ref}
	}
	return judged, Answer{}, false
}

// ownRewrite is whether a rewrite is a branch's own, which drops no work
// (slice-193): the clone has the remote commit it replaces, the ref is a
// branch other than main, the branch the remote's HEAD names (asked of the
// remote, which must have it), and every commit of the remote commit that
// main lacks has a counterpart among the commit pushed's that main lacks
// (internal/engine/rewrite). Main is the clone's remote-tracking branch of it,
// all of a head's commits counting with none. The commits that would be
// dropped are returned when they are what refuses it.
func (h Handler) ownRewrite(ctx context.Context, remote string, r pushedRef) ([]vcsport.Entry, bool) {
	if r.base == "" || remote == "" || !strings.HasPrefix(r.ref, heads) {
		return nil, false
	}
	main, ok := h.mainBranch(ctx, remote)
	if !ok || r.ref == heads+main {
		return nil, false
	}
	tracking := "refs/remotes/" + remote + "/" + main
	if !hasCommit(ctx, h.ports, tracking) {
		tracking = ""
	}
	dropped, err := rewrite.Dropped(ctx, h.ports, tracking, r.base, r.local)
	return dropped, err == nil && len(dropped) == 0
}

// mainBranch is the branch the remote's HEAD names, asked of the remote
// itself as itos push asks it (git ls-remote --symref), when the remote has
// it; not ok when the remote cannot be asked or names no branch it has.
func (h Handler) mainBranch(ctx context.Context, remote string) (string, bool) {
	talk := vcsport.Talk{NoPrompt: true}
	main, err := h.ports.RemoteHead(ctx, remote, talk)
	if err != nil || main == "" {
		return "", false
	}
	has, err := h.ports.RemoteRefs(ctx, remote, []string{heads + main}, talk)
	_, found := has[heads+main]
	return main, err == nil && found
}

// refuseRewrite is the hook's refusal of a push that rewrites the remote's
// ref: it names the ref and its commit, each commit the rewrite would drop
// when that is why, and itos push, which rebases onto the remote's commits and
// never forces, and itos push --onto-main, which brings an item's branch up to
// main.
func refuseRewrite(r pushedRef, dropped []vcsport.Entry) Answer {
	why := "a commit that does not build on it"
	if r.base == "" {
		why = "one that cannot build on it, since this clone lacks it"
	}
	var said strings.Builder
	fmt.Fprintf(&said, "itos: the push would replace %s's commit %s on the remote with %s, rewriting what "+
		"others may have built on; it is refused, and hooks.pre_push's commands did not run.\n", r.ref, short(r.remote),
		why)
	if len(dropped) > 0 {
		fmt.Fprintln(&said, "It would drop these commits of the remote's branch, which main lacks and the commit pushed "+
			"has no counterpart of (the same author, author date and message):")
		for _, e := range dropped {
			fmt.Fprintf(&said, "  %s %s\n", e.Short, e.Subject)
		}
	}
	fmt.Fprintln(&said, "Run itos push instead: it fetches the remote's commits and rebases yours onto them, and never forces. "+
		"On an item's branch, itos push --onto-main brings it up to main, rewriting it only that far.")
	return Answer{Code: policy, Said: said.String()}
}

// pushBases are the remote commits a push builds on (pre-push.ts's
// pushBases, slice 182). An existing ref builds on the remote's commit it
// replaces, a remote commit this clone does not have gives no base to compare
// with, so the whole run is wanted. A new ref builds on the remote commits
// its own commits grow from (vcsport.UnpushedBases): none when it adds no
// commit, which wants nothing run, and the whole run when it adds some and
// no remote commit is one of them. A deleted ref runs nothing.
func (h Handler) pushBases(ctx context.Context, refs []pushedRef) ([]string, bool) {
	var bases []string
	seen := map[string]bool{}
	add := func(shas ...string) {
		for _, sha := range shas {
			if !seen[sha] {
				seen[sha] = true
				bases = append(bases, sha)
			}
		}
	}
	whole := false
	for _, ref := range refs {
		switch {
		case ref.remote == "":
			found, adds := h.ports.UnpushedBases(ctx, ref.local)
			whole = whole || adds && len(found) == 0
			add(found...)
		case ref.base == "":
			whole = true
		default:
			add(ref.base)
		}
	}
	return bases, whole
}

// span is a range of commits, from its start to its end, and whether only
// those of them no remote-tracking branch has are judged.
type span struct {
	From, To string
	Unpushed bool
}

// pushedRange is the range of the commits a pushed ref adds, as verify takes
// it: the ones after the remote's commit it replaces that are on no
// remote-tracking branch (<local> ^<base> --not --remotes, slice-213), its
// range checks starting at that commit; or, with no such commit in this clone,
// the ones on no remote-tracking branch (vcsport.Unpushed), as verify with no
// range judges them under a stealth config. Others' commits already on the
// remote, those a merge of main brings in among them, are not the pusher's to
// fix.
func (r pushedRef) pushedRange() span {
	if r.base == "" {
		return span{From: vcsport.Unpushed, To: r.local}
	}
	return span{From: r.base, To: r.local, Unpushed: true}
}

// verifyPush is the hook's first step: verify over what each pushed ref
// adds, each range once (judge.Over over the config's VCS), its report kept. It prints nothing when every
// commit passes and the range checks hold, so a push of passing commits reads
// as it did before the hook verified; else each failing range's report, how
// to fix the commits, and 1 (2 when commits.since is no commit here, as
// verify). A config with no commits section sets no commit rules, so there
// is nothing to verify, and the hook runs hooks.pre_push's commands as
// before.
func (h Handler) verifyPush(ctx context.Context, cfg *config.Loaded, refs []pushedRef, o Out) (Answer, error) {
	if !cfg.HasSection("commits") {
		return Answer{}, nil
	}
	if missing := cfg.SinceIssues(); len(missing) > 0 {
		var said strings.Builder
		for _, p := range missing {
			fmt.Fprintf(&said, "FAIL %s\n", p.Message)
		}
		return Answer{Code: usage, Said: said.String()}, nil
	}
	var report strings.Builder
	first := ""
	refused := false
	seen := map[span]bool{}
	for _, ref := range refs {
		r := ref.pushedRange()
		if seen[r] {
			continue
		}
		seen[r] = true
		var log strings.Builder
		over := judge.Over
		if r.Unpushed {
			over = judge.OverUnpushed
		}
		run, err := over(ctx, cfg, cfg.VCS, r.From, r.To, judge.Out{Stdout: &log, Stderr: &log, Tests: o.Tests})
		if err != nil {
			return Answer{}, err
		}
		if run.OK() {
			continue
		}
		refused = true
		report.WriteString(log.String())
		for _, c := range run.Results {
			if !c.OK && first == "" {
				first = c.SHA
			}
		}
	}
	if !refused {
		return Answer{}, nil
	}
	return Answer{Code: policy, Said: report.String() + h.fixAdvice(ctx, first)}, nil
}

// fixAdvice is what the refusal tells the person to do: amend the last
// commit, or rebase from the parent of the first commit that fails (--root
// for a root commit) to reword or edit an earlier one, then push again. With
// no failing commit, only the range checks failed.
func (h Handler) fixAdvice(ctx context.Context, first string) string {
	if first == "" {
		return "The push is refused: fix what the range checks above report, then push again.\n"
	}
	base := h.ports.Parent(ctx, first)
	if base == vcsport.EmptyTree {
		base = "--root"
	} else {
		base = short(base)
	}
	return "The push is refused: fix the commits above, then push again.\n" +
		"  - the last commit: git commit --amend\n" +
		"  - an earlier one: git rebase -i " + base + ", marking it reword or edit\n"
}

// prePushInput is git's ref lines on stdin, or under pre-commit or prek,
// which keep stdin, the one line their environment gives.
func prePushInput() (string, error) {
	to, ok := os.LookupEnv("PRE_COMMIT_TO_REF")
	if !ok {
		raw, err := io.ReadAll(os.Stdin)
		return string(raw), err
	}
	local, ok := os.LookupEnv("PRE_COMMIT_LOCAL_BRANCH")
	if !ok {
		local = "HEAD"
	}
	remote, ok := os.LookupEnv("PRE_COMMIT_REMOTE_BRANCH")
	if !ok {
		remote = "-"
	}
	from := os.Getenv("PRE_COMMIT_FROM_REF")
	if from == "" {
		from = strings.Repeat("0", 40)
	}
	return fmt.Sprintf("%s %s %s %s\n", local, to, remote, from), nil
}

// prePush is `hook pre-push <remote> <url>` (pre-push.ts's prePush, and
// slice 46): a push that would move or delete a tag the remote has refused
// first (slice 182, movedTag), then one that would rewrite a remote ref
// (slice 128, rewrites; a branch's own rewrite, slice-193, is judged as a new
// branch), then verify over what each pushed ref adds, then,
// when every commit passes, the commands of hooks.pre_push, per_base once
// per remote base with {base} filled in, else whole, each printed and run
// with the hook's streams; without hooks.pre_push (or a hooks section), the verify alone,
// where it was an error before. 0 when all pass, 1 when not.
func (h Handler) prePush(ctx context.Context, remote string, o Out) (Answer, error) {
	input, err := prePushInput()
	if err != nil {
		return Answer{}, err
	}
	lines := pushLines(input)
	if l, ok := movedTag(lines); ok {
		return refuseTag(l), nil
	}
	refs, refusal, refused := h.rewrites(ctx, remote, h.pushedRefs(ctx, lines))
	if refused {
		return refusal, nil
	}
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	if a, err := h.verifyPush(ctx, cfg, refs, o); a.Code != 0 || err != nil {
		return a, err
	}
	commands := cfg.Hooks.PrePush
	if commands == nil {
		return Answer{}, nil
	}
	run := func(line string) bool {
		fmt.Fprintf(o.Stdout, "$ %s\n", line)
		return h.ports.Shell(ctx, cfg, line, shell.Options{Stdin: os.Stdin, Stdout: o.Stdout, Stderr: o.Stderr}).OK()
	}
	bases, whole := h.pushBases(ctx, refs)
	ok := true
	if whole {
		ok = run(commands.Whole)
	} else {
		for _, base := range bases {
			ok = run(strings.ReplaceAll(commands.PerBase, "{base}", base)) && ok
		}
	}
	if !ok {
		return Answer{Code: policy}, nil
	}
	return Answer{}, nil
}
