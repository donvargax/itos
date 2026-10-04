package cli

// The hooks (hooks.ts, commit-data.ts, commit-scope.ts, commit.ts,
// commit-tasks.ts and pre-push.ts): `hook commit-msg <file>` and `hook
// pre-push <remote> <url>`, the two entry points the shims call. The third,
// `hook pre-tool-use`, is Claude Code's, not git's (pretooluse.go).
//
// hook commit-msg runs four rules in the TypeScript's order, and the first to
// fail decides: itos's own data as staged, when the commit stages any, since
// the other rules read the config; the staged files' rules (the path rules,
// each kind's staged range commands and the built-in moves rule); the header
// lint beside the footer rules; then the static checks of the tasks the
// message names, the slowest last, run only for a commit every other rule
// lets through. Each rule is the judgement its own command already makes
// (config check over the index, scope's path rules, the moves rule, the
// footer rules, the cost rule), never a second copy of it. Under a stealth
// config the footers the rules judge are the ones the commit's note will
// hold (handedFooters), never the message's.

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v3/internal/adr"
	"github.com/donvargax/itos/v3/internal/check"
	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/ledger"
	"github.com/donvargax/itos/v3/internal/message"
	"github.com/donvargax/itos/v3/internal/out"
	"github.com/donvargax/itos/v3/internal/scope"
	"github.com/donvargax/itos/v3/internal/shell"
	"github.com/donvargax/itos/v3/internal/source"
	"github.com/donvargax/itos/v3/internal/tests"
	"github.com/donvargax/itos/v3/internal/value"
	"github.com/donvargax/itos/v3/internal/work"
)

// stagedFiles are the paths the commit stages, as git names them (repo.ts's
// stagedFiles): added, copied, modified, renamed or deleted, against base,
// or against HEAD when base is "".
func stagedFiles(base string) ([]string, error) {
	args := []string{"diff", "--cached", "--name-only", "--diff-filter=ACMRD"}
	if base != "" {
		args = append(args, base)
	}
	return git.Lines(args...)
}

// judgedBase is the commit the commit being made is judged against, "" for
// HEAD: an amend is judged as the commit it makes (bug 6), so against HEAD's
// parent, the empty tree for a root commit, never against the commit it
// replaces, which would leave a reworded amend touching nothing.
func judgedBase() string {
	if !amending() {
		return ""
	}
	return git.Parent("HEAD")
}

// hookCommitMsg is `hook commit-msg <file>` (hooks.ts's hookCommitMsg).
func hookCommitMsg(file string, o Out) (int, error) {
	base := judgedBase()
	if code, err := stagedDataRule(base, o); code != 0 || err != nil {
		return code, err
	}
	text, err := source.Worktree.Read(file)
	if err != nil {
		return 0, err
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	if code, err := stagedRule(cfg, text, base, o); code != 0 || err != nil {
		return code, err
	}
	reading := message.Reading{At: os.Getenv("ITOS_AT"), Warn: o.Stderr}
	footers := text
	if cfg.Stealth {
		if reading.Note, err = handedFooters(); err != nil {
			return 0, err
		}
		footers = reading.Note
	}
	code, err := message.LintFile(cfg, file, text, reading,
		message.HookStreams{Stdin: os.Stdin, Stdout: o.Stdout, Stderr: o.Stderr})
	if code != 0 || err != nil {
		return code, err
	}
	return tasksRule(cfg, footers, o)
}

// handedFooters are the footers a commit being made under a stealth config
// carries, which its note will hold: the lines itos commit hands over in
// ITOS_FOOTERS; else, for an amend, HEAD's note, which notes.rewriteRef
// carries to the new commit; else none.
func handedFooters() (string, error) {
	if lines, ok := os.LookupEnv(message.FootersEnv); ok {
		return lines, nil
	}
	if !amending() {
		return "", nil
	}
	return message.Note("HEAD")
}

// amending is whether the commit being made amends HEAD. itos commit says so
// in AmendEnv, always (slice 37), and its word is taken. A commit made
// without it is guessed at: git tells a hook nothing of an amend, but hands
// it the author in GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL and GIT_AUTHOR_DATE, and
// an amend keeps HEAD's, to the second, where a new commit's date is the time
// it is made. A new commit by HEAD's author in the same second as HEAD reads
// as one too, and an amend that sets another date (--date, --reset-author)
// does not; verify, reading the commits made, still judges both.
func amending() bool {
	if told, ok := os.LookupEnv(AmendEnv); ok {
		return told == "1"
	}
	name, okName := os.LookupEnv("GIT_AUTHOR_NAME")
	email, okEmail := os.LookupEnv("GIT_AUTHOR_EMAIL")
	date, okDate := os.LookupEnv("GIT_AUTHOR_DATE")
	if !okName || !okEmail || !okDate {
		return false
	}
	head, err := git.Output("log", "-1", "--date=raw", "--format=%an%x00%ae%x00%ad", "HEAD")
	if err != nil {
		return false
	}
	return strings.TrimSpace(head) == name+"\x00"+email+"\x00"+strings.TrimPrefix(date, "@")
}

// stagesData is whether a staged path is the config, a ledger file, the
// registry, a smoke set, or a decision record or their index in the folder
// work.decisions names (slice 74), as the staged config names them
// (commit-data.ts's stagesData), read in the current source. A config that
// does not load and is not staged leaves the commit to the hook's other
// rules, which read it too.
func stagesData(staged []string) bool {
	files := map[string]bool{}
	for _, f := range staged {
		files[path.Clean(f)] = true
	}
	if files[path.Clean(config.Path())] {
		return true
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return false
	}
	if cfg.HasSection("ledger") {
		if layout, err := ledger.LayoutOf(cfg); err == nil {
			for f := range files {
				if path.Dir(f) == path.Clean(layout.Dir) && layout.File.MatchString(path.Base(f)) {
					return true
				}
			}
		}
	}
	decisions := path.Clean(filepath.ToSlash(cfg.Work.Decisions))
	for f := range files {
		if path.Dir(f) == decisions && (path.Base(f) == adr.IndexName || adr.IsRecord(path.Base(f))) {
			return true
		}
	}
	named := []string{cfg.Work.Registry}
	for _, name := range cfg.Tests.Keys {
		if smoke := cfg.Tests.Values[name].Smoke.File; smoke != nil && *smoke != "" {
			named = append(named, *smoke)
		}
	}
	for _, f := range named {
		if files[path.Clean(f)] {
			return true
		}
	}
	return false
}

// stagedDataIssues are the staged data's problems, read from the index
// (commit-data.ts's stagedDataIssues): config check's findings when the
// commit stages any of itos's data, data that cannot be read one problem
// saying so; and the rejection's first line, commits.reject_message as
// staged, if it loads. The staged paths are against base, as stagedFiles.
func stagedDataIssues(base string) ([]out.Problem, string, error) {
	staged, err := stagedFiles(base)
	if err != nil {
		return nil, "", err
	}
	index, err := source.At("index")
	if err != nil {
		return nil, "", err
	}
	var found []out.Problem
	header := ""
	err = source.ReadingFrom(index, func() error {
		if !stagesData(staged) {
			return nil
		}
		_, findings, _, _, err := configFindings("")
		if err != nil {
			found = []out.Problem{{Rule: "data-unreadable", Message: err.Error()}}
		}
		for _, f := range findings {
			found = append(found, f.Problem)
		}
		header = "Commit rejected:"
		if cfg, err := config.Load(config.Path()); err == nil {
			header = cfg.Commits.RejectMessage
		}
		return nil
	})
	return found, header, err
}

// stagedDataRule is the hook's first rule (commit-data.ts's hook): 0 when the
// staged data is sound or none is staged, else the rejection, its problems
// one a line, and 1.
func stagedDataRule(base string, o Out) (int, error) {
	found, header, err := stagedDataIssues(base)
	if err != nil || len(found) == 0 {
		return 0, err
	}
	fmt.Fprintln(o.Stderr, header)
	for _, p := range found {
		fmt.Fprintf(o.Stderr, "  - %s\n", p.Message)
	}
	return ExitPolicy, nil
}

// problemLine is one problem a staged range command prints on stderr:
// `  - <sentence>`. JavaScript's \s, dot and \S, spelled out, since RE2's
// differ.
var problemLine = regexp.MustCompile(`^` + value.Space + `+- ([^\n\r\x{2028}\x{2029}]*[^` + value.SpaceChars + `])`)

// stagedCheckIssues are one kind's staged range command's problems
// (commit-scope.ts's stagedCheckIssues): its {type} filled in, run on the
// index; each `  - …` line it prints on stderr when it fails is one problem,
// and a failure without one is a problem too.
func stagedCheckIssues(cfg *config.Loaded, c config.RangeCheck, typ string) []out.Problem {
	command := strings.ReplaceAll(*c.Staged, "{type}", tests.ShellWord(typ))
	var stdout, stderr strings.Builder
	if shell.Run(cfg, command, shell.Options{Stdout: &stdout, Stderr: &stderr}).OK() {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(stderr.String(), "\n") {
		if m := problemLine.FindStringSubmatch(l); m != nil {
			lines = append(lines, m[1])
		}
	}
	if len(lines) == 0 {
		lines = append(lines, c.Name+" failed: "+value.Trim(stdout.String()+stderr.String()))
	}
	found := make([]out.Problem, len(lines))
	for i, l := range lines {
		found[i] = out.Problem{Rule: c.Name, Message: l}
	}
	return found
}

// stagedRule is the hook's second rule (commit-scope.ts's hook): the
// message's type against the staged paths, then the kinds' staged range
// commands that apply to the type, then the built-in moves rule, HEAD
// against the index. The paths and the moves rule judge an amend against
// base, HEAD's parent (judgedBase). The paths and the staged commands are
// nothing for a type with no path rule (merges, reverts and unknown types
// are the header lint's); the moves rule judges the types it says it
// judges. 0 when they hold, else the rejection and 1.
func stagedRule(cfg *config.Loaded, text, base string, o Out) (int, error) {
	typ := message.Type(text)
	rules, err := scope.Of(cfg)
	if err != nil {
		return 0, err
	}
	found := []out.Problem{}
	if rules.Ruled(typ) {
		staged, err := stagedFiles(base)
		if err != nil {
			return 0, err
		}
		paths, err := rules.Issues(typ, staged)
		if err != nil {
			return 0, err
		}
		found = append(found, paths...)
		for _, name := range cfg.Tests.Keys {
			for _, c := range cfg.Tests.Values[name].RangeChecks {
				if c.Staged != nil && *c.Staged != "" && !value.Includes(c.ExceptTypes, typ) {
					found = append(found, stagedCheckIssues(cfg, c, typ)...)
				}
			}
		}
	}
	before := "HEAD"
	if base != "" {
		before = base
	}
	moved, err := tests.NewMoves(cfg).Between(typ, before, "index")
	if err != nil {
		return 0, err
	}
	found = append(found, moved...)
	if len(found) == 0 {
		return 0, nil
	}
	rules.Reject(o.Stderr, found)
	return ExitPolicy, nil
}

// namedTask is a task the message names, with its work item's status as
// staged (none when it has no item) and the registry that says so.
type namedTask struct {
	task     ledger.Task
	status   any
	hasItem  bool
	registry string
}

// namedTaskIDs are the task IDs the message's ledger footers name, each
// once, in order.
func namedTaskIDs(cfg *config.Loaded, text string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, key := range cfg.Commits.Footers.Keys {
		f := cfg.Commits.Footers.Values[key]
		if !f.Source.IsName || f.Source.Name != "ledger" {
			continue
		}
		strip := ""
		if f.StripPrefix != nil {
			strip = *f.StripPrefix
		}
		for _, id := range message.IDs(text, key, strip) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// stagedTasks are the named tasks the staged ledger has, with their items'
// statuses, read from the index by the staged config (commit-tasks.ts's
// stagedTasks). A staged tree whose ledger cannot be read names none: the
// header lint has judged the footer by then.
func stagedTasks(ids []string) ([]namedTask, error) {
	index, err := source.At("index")
	if err != nil {
		return nil, err
	}
	var named []namedTask
	err = source.ReadingFrom(index, func() error {
		cfg, err := config.Load(config.Path())
		if err != nil {
			return nil
		}
		tasks, err := ledger.Tasks(cfg)
		if err != nil {
			return nil
		}
		registry := cfg.Work.Registry
		statuses := work.ItemStatuses(registry)
		for _, id := range ids {
			for _, t := range tasks {
				if t.ID == id {
					status, ok := statuses.Of(id)
					named = append(named, namedTask{t, status, ok, registry})
					break
				}
			}
		}
		return nil
	})
	return named, err
}

// checkEnv is the environment a task's check runs in under the hook: the
// hook's, less the index git made the commit from, so that a check's own git
// commands (in a scratch repository too) never read or write the commit's
// index, as under `itos task`, and less the footers and the amend itos commit
// handed the hook, which are this commit's and no commit a check makes.
func checkEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_INDEX_FILE=") && !strings.HasPrefix(kv, message.FootersEnv+"=") &&
			!strings.HasPrefix(kv, AmendEnv+"=") {
			env = append(env, kv)
		}
	}
	return env
}

// checkFailure is what went wrong with a failed check, in a few words.
func checkFailure(c ledger.Check, run check.Captured) string {
	command := "`" + c.Command() + "`"
	switch {
	case run.TimedOut && run.Capped:
		return fmt.Sprintf("%s ran past hooks.commit_msg.check_timeout (%ss)", command, value.Number(run.Seconds))
	case run.TimedOut:
		return fmt.Sprintf("%s ran past its timeout (%ss)", command, value.Number(run.Seconds))
	case c.MustFail() && run.Code == 0:
		return command + " passed, and must fail"
	case run.Code < 0:
		return command + " was stopped"
	}
	return fmt.Sprintf("%s exited %d", command, run.Code)
}

// firstFailure runs one task's checks before its first late one, an after:
// push check pending since the commit is not pushed, and gives the first
// failure, if any, with its output printed under the task's title and the
// check, as `itos task` prints them.
func firstFailure(cfg *config.Loaded, task ledger.Task, cap float64, env []string, stderr io.Writer) (string, error) {
	if task.Err != nil {
		return "", task.Err
	}
	for _, costed := range check.ChecksBeforeLate(cfg, task) {
		c := costed.Check
		if c.Pushed() {
			continue
		}
		run, err := check.RunCaptured(cfg, c, cap, env)
		if err != nil {
			return "", err
		}
		if run.Result == check.Pass {
			continue
		}
		mode := ""
		if c.MustFail() {
			mode = "   (must fail)"
		}
		fmt.Fprintf(stderr, "%s %s\n  $ %s%s\n", task.ID, task.Title, c.Command(), mode)
		if run.Output != "" {
			fmt.Fprint(stderr, run.Output)
			if !strings.HasSuffix(run.Output, "\n") {
				fmt.Fprintln(stderr)
			}
		}
		return checkFailure(c, run), nil
	}
	return "", nil
}

// verdict is what one failing task does to the commit: when its item is
// done, the line it adds to the rejection; else its report, printed now, and
// nothing.
func verdict(n namedTask, why string, stderr io.Writer) []string {
	id := n.task.ID
	if n.hasItem && n.status == "done" {
		return []string{fmt.Sprintf("failing %s: %s, and %s says %s is done", id, why, n.registry, id)}
	}
	where := "has no item in " + n.registry
	if n.hasItem && value.Truthy(n.status) {
		where = "is " + value.String(n.status) + " in " + n.registry
	}
	fmt.Fprintf(stderr, "failing %s: %s; %s %s, so the commit goes through, and CI judges the push\n", id, why, id, where)
	return nil
}

// tasksRule is the hook's last rule (commit-tasks.ts's hook): the static
// checks of the tasks the message's ledger footers name, each task up to its
// first late check and its first failure, each check captured and capped by
// hooks.commit_msg.check_timeout. 0 when every named task's checks pass, or
// every one that fails is not done; else the rejection, one line per done
// task that fails, and 1. hooks.commit_msg.task_checks: false turns it off.
func tasksRule(cfg *config.Loaded, text string, o Out) (int, error) {
	settings := cfg.Hooks.CommitMsg
	if !settings.TaskChecks {
		return 0, nil
	}
	ids := namedTaskIDs(cfg, text)
	if len(ids) == 0 {
		return 0, nil
	}
	cap := check.NoCap
	if settings.CheckTimeout != nil {
		cap = *settings.CheckTimeout
	}
	env := checkEnv()
	named, err := stagedTasks(ids)
	if err != nil {
		return 0, err
	}
	var rejected []string
	for _, n := range named {
		why, err := firstFailure(cfg, n.task, cap, env, o.Stderr)
		if err != nil {
			return 0, err
		}
		if why != "" {
			rejected = append(rejected, verdict(n, why, o.Stderr)...)
		}
	}
	if len(rejected) == 0 {
		return 0, nil
	}
	fmt.Fprintln(o.Stderr, cfg.Commits.RejectMessage)
	for _, line := range rejected {
		fmt.Fprintf(o.Stderr, "  - %s\n", line)
	}
	return ExitPolicy, nil
}

// zeros is a SHA git writes for no commit: a new branch's remote side, a
// deleted branch's local side.
var zeros = regexp.MustCompile(`^0+$`)

// pushedRef is one ref a push sends, from git's `<local ref> <local sha>
// <remote ref> <remote sha>` line: the commit it sends, and the remote's
// commit it replaces when this clone has that one, else "" (a new branch, or
// a remote commit the clone lacks).
type pushedRef struct{ local, base string }

// pushedRefs are the refs a push sends, a deleted one (zero local SHA) left
// out, since it sends nothing.
func pushedRefs(input string) []pushedRef {
	var refs []pushedRef
	for _, line := range strings.Split(input, "\n") {
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
		local, remote := field(1), field(3)
		if local == "" || zeros.MatchString(local) {
			continue
		}
		ref := pushedRef{local: local}
		if remote != "" && !zeros.MatchString(remote) && git.HasCommit(remote) {
			ref.base = remote
		}
		refs = append(refs, ref)
	}
	return refs
}

// pushBases are the remote commits a push builds on (pre-push.ts's
// pushBases). A new branch, or a remote commit this clone does not have,
// gives no base to compare with, so the whole run is wanted; a deleted branch
// runs nothing.
func pushBases(refs []pushedRef) ([]string, bool) {
	var bases []string
	seen := map[string]bool{}
	whole := false
	for _, ref := range refs {
		switch {
		case ref.base == "":
			whole = true
		case !seen[ref.base]:
			seen[ref.base] = true
			bases = append(bases, ref.base)
		}
	}
	return bases, whole
}

// pushedRange is the range of the commits a pushed ref adds, as verify takes
// it: the ones after the remote's commit it replaces, or, with no such commit
// in this clone, the ones on no remote-tracking branch (git.Unpushed), as
// verify with no range judges them under a stealth config. Others' commits
// already on the remote are not the pusher's to fix.
func (r pushedRef) pushedRange() span {
	if r.base == "" {
		return span{git.Unpushed, r.local}
	}
	return span{r.base, r.local}
}

// verifyPush is the hook's first step: verify over what each pushed ref
// adds, each range once, its report kept. It prints nothing when every
// commit passes and the range checks hold, so a push of passing commits reads
// as it did before the hook verified; else each failing range's report, how
// to fix the commits, and 1 (2 when commits.since is no commit here, as
// verify). A config with no commits section sets no commit rules, so there
// is nothing to verify, and the hook runs hooks.pre_push's commands as
// before.
func verifyPush(cfg *config.Loaded, refs []pushedRef, o Out) (int, error) {
	if !cfg.HasSection("commits") {
		return 0, nil
	}
	if missing := cfg.SinceIssues(); len(missing) > 0 {
		printSinceIssues(missing, o)
		return ExitUsage, nil
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
		run, err := newVerifier(cfg, Out{Stdout: &log, Stderr: &log}).over(r.From, r.To)
		if err != nil {
			return 0, err
		}
		if run.ok() {
			continue
		}
		refused = true
		report.WriteString(log.String())
		for _, c := range run.results {
			if !c.OK && first == "" {
				first = c.SHA
			}
		}
	}
	if !refused {
		return 0, nil
	}
	fmt.Fprint(o.Stderr, report.String())
	fmt.Fprint(o.Stderr, fixAdvice(first))
	return ExitPolicy, nil
}

// fixAdvice is what the refusal tells the person to do: amend the last
// commit, or rebase from the parent of the first commit that fails (--root
// for a root commit) to reword or edit an earlier one, then push again. With
// no failing commit, only the range checks failed.
func fixAdvice(first string) string {
	if first == "" {
		return "The push is refused: fix what the range checks above report, then push again.\n"
	}
	base := git.Parent(first)
	if base == git.EmptyTree {
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

// hookPrePush is `hook pre-push <remote> <url>` (pre-push.ts's prePush, and
// slice 46): verify over what each pushed ref adds, then, when every commit
// passes, the commands of hooks.pre_push, per_base once per remote base with
// {base} filled in, else whole, each printed and run with the hook's
// streams; without hooks.pre_push (or a hooks section), the verify alone,
// where it was an error before. 0 when all pass, 1 when not.
func hookPrePush(o Out) (int, error) {
	input, err := prePushInput()
	if err != nil {
		return 0, err
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	refs := pushedRefs(input)
	if code, err := verifyPush(cfg, refs, o); code != 0 || err != nil {
		return code, err
	}
	commands := cfg.Hooks.PrePush
	if commands == nil {
		return 0, nil
	}
	run := func(line string) bool {
		fmt.Fprintf(o.Stdout, "$ %s\n", line)
		return shell.Run(cfg, line, shell.Options{Stdin: os.Stdin, Stdout: o.Stdout, Stderr: o.Stderr}).OK()
	}
	bases, whole := pushBases(refs)
	ok := true
	if whole {
		ok = run(commands.Whole)
	} else {
		for _, base := range bases {
			ok = run(strings.ReplaceAll(commands.PerBase, "{base}", base)) && ok
		}
	}
	if !ok {
		return ExitPolicy, nil
	}
	return 0, nil
}
