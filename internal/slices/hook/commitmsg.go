package hook

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v8/internal/adr"
	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/scope"
	"github.com/donvargax/itos/v8/internal/shell"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// stagedFiles are the paths the commit stages, as git names them (repo.ts's
// stagedFiles), read NUL-separated so that none comes back quoted (bug 31),
// against base, or against HEAD when base is "". Every change type is listed,
// a type change (T) among them, and with rename detection off, so a rename is
// its old path's deletion and its new path's addition (bug 32): a rename out
// of what a type may touch is judged by the path it leaves, as verify and the
// CI plan judge the commit once it is made.
func (h Handler) stagedFiles(ctx context.Context, base string) ([]string, error) {
	return h.ports.Staged(ctx, base)
}

// judgedBase is the commit the commit being made is judged against, "" for
// HEAD: an amend is judged as the commit it makes (bug 6), so against HEAD's
// parent, the empty tree for a root commit, never against the commit it
// replaces, which would leave a reworded amend touching nothing.
func (h Handler) judgedBase(ctx context.Context) string {
	if !h.amending(ctx) {
		return ""
	}
	return h.ports.Parent(ctx, "HEAD")
}

// commitMsg is `hook commit-msg <file>` (hooks.ts's hookCommitMsg).
func (h Handler) commitMsg(ctx context.Context, file string, o Out) (Answer, error) {
	base := h.judgedBase(ctx)
	if a, err := h.stagedDataRule(ctx, base, o); a.Code != 0 || err != nil {
		return a, err
	}
	text, err := h.ports.Read(ctx, file)
	if err != nil {
		return Answer{}, err
	}
	cfg, err := h.ports.LoadConfig(ctx)
	if err != nil {
		return Answer{}, err
	}
	merge, err := h.stagedMerge(ctx)
	if err != nil {
		return Answer{}, err
	}
	// A merge with no change of its own passes whatever its message says
	// (bug 30): what it brings was judged in the commits that made it.
	if merge != nil && len(merge.Files) == 0 {
		return Answer{}, nil
	}
	// The message git will store, as the header lint reads it (bug 36): a
	// blank or comment first line, as an editor leaves one, does not hide
	// the type from the paths and the footers.
	stored := message.Cleaned(text, cfg.CommentChar())
	if a, err := h.stagedRule(ctx, cfg, stored, base, merge, o); a.Code != 0 || err != nil {
		return a, err
	}
	reading := message.Reading{At: os.Getenv("ITOS_AT"), Warn: o.Stderr, Tests: o.Tests}
	footers := stored
	if cfg.Stealth {
		if reading.Note, err = h.handedFooters(ctx); err != nil {
			return Answer{}, err
		}
		footers = reading.Note
	}
	code, err := message.LintFile(cfg, file, text, reading,
		message.HookStreams{Stdin: os.Stdin, Stdout: o.Stdout, Stderr: o.Stderr})
	if code != 0 || err != nil {
		return Answer{Code: code}, err
	}
	return h.tasksRule(ctx, cfg, footers, o)
}

// stagedMerge is the commit being made when it is a merge, as verify will
// judge it once made (bug 30): its own changes (VCS.StagedOwnPaths) and its
// first parent; nil when it is not a merge. Its parents are HEAD and the
// commits a merge in progress brings (MERGE_HEAD), with which git refuses an
// amend, or for an amend HEAD's own.
func (h Handler) stagedMerge(ctx context.Context) (*judge.Judged, error) {
	v := h.ports
	var parents []string
	if heads := v.MergeHeads(ctx); len(heads) > 0 {
		head, err := v.Resolve(ctx, "HEAD")
		if err != nil {
			return nil, err
		}
		parents = append([]string{head}, heads...)
	} else if h.amending(ctx) {
		var err error
		if parents, err = v.Parents(ctx, "HEAD"); err != nil {
			return nil, err
		}
	}
	if len(parents) < 2 {
		return nil, nil
	}
	own, err := v.StagedOwnPaths(ctx, parents)
	if err != nil {
		return nil, err
	}
	return &judge.Judged{Files: own, After: "index", Parent: parents[0], Merge: true}, nil
}

// handedFooters are the footers a commit being made under a stealth config
// carries, which its note will hold: the lines itos commit hands over in
// ITOS_FOOTERS; else, for an amend, HEAD's note, which notes.rewriteRef
// carries to the new commit; else none.
func (h Handler) handedFooters(ctx context.Context) (string, error) {
	if lines, ok := os.LookupEnv(message.FootersEnv); ok {
		return lines, nil
	}
	if !h.amending(ctx) {
		return "", nil
	}
	head, err := h.ports.Show(ctx, "HEAD")
	return head.Note, err
}

// amending is whether the commit being made amends HEAD. itos commit says so
// in AmendEnv, always (slice 37), and its word is taken. A commit made
// without it is guessed at: git tells a hook nothing of an amend, but hands
// it the author in GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL and GIT_AUTHOR_DATE, and
// an amend keeps HEAD's, to the second, where a new commit's date is the time
// it is made. A new commit by HEAD's author in the same second as HEAD reads
// as one too, and an amend that sets another date (--date, --reset-author)
// does not; verify, reading the commits made, still judges both.
func (h Handler) amending(ctx context.Context) bool {
	if told, ok := os.LookupEnv(message.AmendEnv); ok {
		return told == "1"
	}
	name, okName := os.LookupEnv("GIT_AUTHOR_NAME")
	email, okEmail := os.LookupEnv("GIT_AUTHOR_EMAIL")
	date, okDate := os.LookupEnv("GIT_AUTHOR_DATE")
	if !okName || !okEmail || !okDate {
		return false
	}
	head, err := h.ports.Show(ctx, "HEAD")
	if err != nil {
		return false
	}
	return head.Author == name && head.Email == email && head.Date == strings.TrimPrefix(date, "@")
}

// stagesData is whether a staged path is the config, a ledger file, the
// registry, the questions' file (slice 130), a smoke set, or a decision record or their index in the folder
// work.decisions names (slice 74), as the staged config names them
// (commit-data.ts's stagesData), read in src. A config that does not load
// and is not staged leaves the commit to the hook's other rules, which read
// it too.
func (h Handler) stagesData(ctx context.Context, src port.Source, staged []string) bool {
	files := map[string]bool{}
	for _, f := range staged {
		files[path.Clean(f)] = true
	}
	file := config.Path(h.ports, src)
	if files[path.Clean(file)] {
		return true
	}
	cfg, err := config.Load(h.ports, h.ports.Trees(ctx), src, file)
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
	named := []string{cfg.Work.Registry, cfg.Work.Asks}
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
// saying so, else the smoke rule's for each kind whose test files it stages
// (stagedSmokeIssues); and the rejection's first line, commits.reject_message
// as staged, if it loads. The staged paths are against base, as stagedFiles.
func (h Handler) stagedDataIssues(ctx context.Context, base string, ws *tests.Warnings) ([]out.Problem, string, error) {
	staged, err := h.stagedFiles(ctx, base)
	if err != nil {
		return nil, "", err
	}
	index, err := h.ports.Trees(ctx).At(ctx, "index")
	if err != nil {
		return nil, "", err
	}
	var found []out.Problem
	if !h.stagesData(ctx, index, staged) {
		found = h.stagedSmokeIssues(ctx, index, staged, base, ws)
	} else {
		_, findings, _, _, err := configcheck.Findings(ctx, h.ports, h.ports.Trees(ctx), index, "", h.ports.TaskFlags(ctx), ws)
		if err != nil {
			found = []out.Problem{{Rule: "data-unreadable", Message: err.Error()}}
		}
		for _, f := range findings {
			found = append(found, f.Problem)
		}
	}
	if len(found) == 0 {
		return nil, "", nil
	}
	header := "Commit rejected:"
	if cfg, err := config.Load(h.ports, h.ports.Trees(ctx), index, config.Path(h.ports, index)); err == nil {
		header = cfg.Commits.RejectMessage
	}
	return found, header, nil
}

// stagedSmokeIssues are the smoke rule's problems, as itos tests smoke check
// finds them and config check reports them, for each kind whose smoke set
// index holds, the config and its data read from there, and whose test files
// a staged path is (tests.StagesTestFile, a deletion's against base, HEAD
// when ""): slice 117, since a feat that makes a file's tests live stages no
// smoke set, and CI's smoke step was the first to see it. A kind whose set
// the index lacks is config check's to report, and a config that does not
// load the hook's other rules'.
func (h Handler) stagedSmokeIssues(ctx context.Context, index port.Source, staged []string, base string, ws *tests.Warnings) []out.Problem {
	cfg, err := config.Load(h.ports, h.ports.Trees(ctx), index, config.Path(h.ports, index))
	if err != nil {
		return nil
	}
	if base == "" {
		base = "HEAD"
	}
	found, _ := configcheck.JudgedSmokeFindings(cfg, func(name, file string) bool {
		return index.Has(ctx, file) && tests.StagesTestFile(cfg, name, staged, base, ws)
	}, ws)
	var problems []out.Problem
	for _, f := range found {
		problems = append(problems, f.Problem)
	}
	return problems
}

// stagedDataRule is the hook's first rule (commit-data.ts's hook): 0 when the
// staged data is sound or none is staged, else the rejection, its problems
// one a line, and 1.
func (h Handler) stagedDataRule(ctx context.Context, base string, o Out) (Answer, error) {
	found, header, err := h.stagedDataIssues(ctx, base, o.Tests)
	if err != nil || len(found) == 0 {
		return Answer{}, err
	}
	a := Answer{Code: policy, Header: header}
	for _, p := range found {
		a.Lines = append(a.Lines, p.Message)
	}
	return a, nil
}

// problemLine is one problem a staged range command prints on stderr:
// `  - <sentence>`. JavaScript's \s, dot and \S, spelled out, since RE2's
// differ.
var problemLine = regexp.MustCompile(`^` + value.Space + `+- ([^\n\r\x{2028}\x{2029}]*[^` + value.SpaceChars + `])`)

// stagedCheckIssues are one kind's staged range command's problems
// (commit-scope.ts's stagedCheckIssues): its {type} filled in, run on the
// index; each `  - …` line it prints on stderr when it fails is one problem,
// and a failure without one is a problem too.
func (h Handler) stagedCheckIssues(ctx context.Context, cfg *config.Loaded, c config.RangeCheck, typ string) []out.Problem {
	command := strings.ReplaceAll(*c.Staged, "{type}", tests.ShellWord(typ))
	var stdout, stderr strings.Builder
	if h.ports.Shell(ctx, cfg, command, shell.Options{Stdout: &stdout, Stderr: &stderr}).OK() {
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
// base, HEAD's parent (judgedBase), and a merge (merge not nil) by its own
// changes against its first parent, as verify does, refusing one whose type
// is none of the commit types (judge.UntypedMerge). The paths and the staged
// commands are nothing for a type with no path rule (unknown types are the
// header lint's); the moves rule judges the types it says it judges. The
// type is the one the message is judged as (message.Type: a revert git wrote
// is a revert, a fixup! commit the type of the header it names). 0 when they
// hold, else the rejection and 1.
func (h Handler) stagedRule(ctx context.Context, cfg *config.Loaded, text, base string, merge *judge.Judged, o Out) (Answer, error) {
	typ := message.Type(text)
	rules, err := scope.Of(cfg)
	if err != nil {
		return Answer{}, err
	}
	found := []out.Problem{}
	if merge != nil {
		merge.Type = typ
		found = append(found, judge.UntypedMerge(cfg, *merge)...)
	}
	if rules.Ruled(typ) {
		var staged []string
		if merge != nil {
			staged = merge.Files
		} else if staged, err = h.stagedFiles(ctx, base); err != nil {
			return Answer{}, err
		}
		paths, err := rules.Issues(typ, staged)
		if err != nil {
			return Answer{}, err
		}
		found = append(found, paths...)
		for _, name := range cfg.Tests.Keys {
			for _, c := range cfg.Tests.Values[name].RangeChecks {
				if c.Staged != nil && *c.Staged != "" && !value.Includes(c.ExceptTypes, typ) {
					found = append(found, h.stagedCheckIssues(ctx, cfg, c, typ)...)
				}
			}
		}
	}
	before := "HEAD"
	if base != "" {
		before = base
	}
	change, err := h.stagedChange(ctx, cfg, text, base, merge)
	if err != nil {
		return Answer{}, err
	}
	var moved []out.Problem
	if merge != nil {
		moved, err = tests.NewMoves(cfg, o.Tests).Merge(typ, merge.Parent, "index", merge.Files, change)
	} else {
		moved, err = tests.NewMoves(cfg, o.Tests).Between(typ, before, "index", change)
	}
	if err != nil {
		return Answer{}, err
	}
	found = append(found, moved...)
	if len(found) == 0 {
		return Answer{}, nil
	}
	return Answer{Code: policy, Rules: rules, Problems: found}, nil
}

// stagedChange is the commit being made as the moves rule reads it beside
// its trees (slice 134): the tests its footers name, the message's own or,
// under a stealth config, those its note will hold (handedFooters), and the
// paths it stages, a merge's own.
func (h Handler) stagedChange(ctx context.Context, cfg *config.Loaded, text, base string, merge *judge.Judged) (tests.Change, error) {
	footers := text
	if cfg.Stealth {
		var err error
		if footers, err = h.handedFooters(ctx); err != nil {
			return tests.Change{}, err
		}
	}
	if merge != nil {
		return judge.MovesChange(cfg, footers, merge.Files), nil
	}
	staged, err := h.stagedFiles(ctx, base)
	if err != nil {
		return tests.Change{}, err
	}
	return judge.MovesChange(cfg, footers, staged), nil
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
		for _, id := range message.IDs(cfg, text, key) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// tasksAt reads named tasks and statuses from the captured tree. Unreadable
// task data names none: the header lint has judged the footer by then.
func (h Handler) tasksAt(ctx context.Context, ids []string, index port.Source) []namedTask {
	cfg, err := config.Load(h.ports, h.ports.Trees(ctx), index, config.Path(h.ports, index))
	if err != nil {
		return nil
	}
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return nil
	}
	var named []namedTask
	registry := cfg.Work.Registry
	statuses := work.ItemStatuses(index, registry)
	for _, id := range ids {
		for _, t := range tasks {
			if t.ID == id {
				status, ok := statuses.Of(id)
				named = append(named, namedTask{t, status, ok, registry})
				break
			}
		}
	}
	return named
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
func (h Handler) tasksRule(ctx context.Context, cfg *config.Loaded, text string, o Out) (Answer, error) {
	settings := cfg.Hooks.CommitMsg
	if !settings.TaskChecks {
		return Answer{}, nil
	}
	ids := namedTaskIDs(cfg, text)
	if len(ids) == 0 {
		return Answer{}, nil
	}
	cap := check.NoCap
	if settings.CheckTimeout != nil {
		cap = *settings.CheckTimeout
	}
	index, err := h.ports.CaptureIndex(ctx)
	if err != nil {
		return Answer{}, err
	}
	named := h.tasksAt(ctx, ids, index)
	var runnable []ledger.Check
	for _, n := range named {
		if n.task.Err != nil {
			return Answer{}, n.task.Err
		}
		for _, c := range check.ChecksBeforeLate(cfg, n.task) {
			if !c.Check.Pushed() {
				runnable = append(runnable, c.Check)
			}
		}
	}
	if len(runnable) == 0 {
		return Answer{}, nil
	}
	execution, env, release, err := h.checkouts(ctx, cfg, index.Tree(ctx), runnable)
	if err != nil {
		return Answer{}, err
	}
	defer release()
	var rejected []string
	for _, n := range named {
		why, err := judge.FirstFailure(cfg, n.task, cap, env, o.Stderr, execution)
		if err != nil {
			return Answer{}, err
		}
		if why != "" {
			rejected = append(rejected, verdict(n, why, o.Stderr)...)
		}
	}
	if len(rejected) == 0 {
		return Answer{}, nil
	}
	return Answer{Code: policy, Header: cfg.Commits.RejectMessage, Lines: rejected}, nil
}

// checkouts sets up where the runnable checks run, on the captured tree: the
// shared checkout of the checks that do not opt in to evidence, and, for the
// ones that do, the engine that answers them from a record (nothing run, no
// checkout) or runs a miss in a checkout of its own. A check with snapshot:
// false uses neither: it runs in the working folder, in the environment
// execution.WorkEnv holds. A checkout is made only
// when some check must run, before any does, so a snapshot that cannot be
// made refuses the commit (bug-112) and one that every check is spared never
// does. With no plain check the checkout made is handed to the engine for its
// first miss rather than another being made. release removes what was made.
func (h Handler) checkouts(ctx context.Context, cfg *config.Loaded, tree string, runnable []ledger.Check) (check.Execution, []string, func(), error) {
	var engine *evidence.Engine
	for _, c := range runnable {
		if c.Evidenced() {
			engine = h.ports.Evidence(ctx, cfg, tree)
			break
		}
	}
	plain, missed := false, false
	for _, c := range runnable {
		switch {
		case c.NoSnapshot:
		case !c.Evidenced() || engine == nil:
			plain = true
		case !check.Answers(engine, cfg, c):
			missed = true
		}
	}
	execution := check.Execution{Evidence: engine, WorkEnv: h.ports.CheckEnv(ctx)}
	if !plain && !missed {
		return execution, nil, func() {}, nil
	}
	snapshot, err := h.ports.TaskSnapshot(ctx, tree)
	if err != nil {
		return execution, nil, nil, err
	}
	env := message.WithoutFooters(snapshot.Env)
	if plain {
		execution.Dir, execution.TempDir = snapshot.Dir, snapshot.TempDir
	} else {
		engine.Spare(evidence.Checkout{Dir: snapshot.Dir, Env: env, Remove: snapshot.Remove})
	}
	return execution, env, func() { _ = snapshot.Remove() }, nil
}
