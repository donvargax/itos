package evidence

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/shell"
	privport "github.com/donvargax/itos/v8/internal/store/private/port"
)

// Source is where a record came from; local records are all there are.
const Source = "local"

// OutputTail is how much of a check's output a record keeps: its last bytes.
const OutputTail = 32 << 10

// The reasons a check that ran left no record. A rerun that does not hit
// always has one when it recorded nothing.
const (
	// ReasonSnapshotUnusable: the checkout of the index tree could not be
	// made (a gitlink in the index, the snapshots folder unusable, a declared
	// input that could not be copied), so the check ran in the working
	// folder, uncached.
	ReasonSnapshotUnusable = "snapshot-unusable"
	// ReasonLockTimeout: the record's lock was held past the wait.
	ReasonLockTimeout = "lock-timeout"
	// ReasonStoreUnusable: the store could not be written.
	ReasonStoreUnusable = "store-unusable"
	// ReasonAlreadyRecorded: the check ran, but another writer's whole record
	// of its key took the record's place first, and is kept as it is; the
	// evidence carries that record's time and provenance.
	ReasonAlreadyRecorded = "already-recorded"
	// ReasonUnkeyable: what the key binds could not be read.
	ReasonUnkeyable = "unkeyable"
	// ReasonInconclusive: the check timed out or did not exit by itself, a
	// result that is not the check's to reuse.
	ReasonInconclusive = "inconclusive"
)

// Git is what the engine asks of the repository, which internal/cli binds
// to the VCS's adapter: the index captured as one tree, the paths the index
// holds, the tracked paths whose working copy differs from it, and a
// disposable checkout of a tree (bug-112's snapshot).
type Git interface {
	IndexTree(ctx context.Context) (string, error)
	IndexPaths(ctx context.Context) ([]string, error)
	UnstagedPaths(ctx context.Context) ([]string, error)
	Snapshot(ctx context.Context, tree string) (Checkout, error)
}

// Checkout is a disposable checkout of a tree: the folder a check runs in,
// the environment it runs with and the removal of both.
type Checkout struct {
	Dir    string
	Env    []string
	Remove func() error
}

// Request is one check to run, looked up or recorded: its command, its
// timeout in seconds, its declaration, and where it reports. Verbose lets
// its output through as it runs, to Log. RunSeconds, when set, is how long a
// miss may run, less than Seconds when something caps it (the commit-msg
// hook's hooks.commit_msg.check_timeout): the key binds Seconds alone, so a
// run the cap stops is not recorded and a record made under no cap answers
// it. Task, when set, names the task the check is for in the line a hit
// prints.
type Request struct {
	Command    string
	Seconds    float64
	RunSeconds *float64
	Decl       config.Evidence
	Task       string
	Verbose    bool
	Log        io.Writer
}

// Outcome is how the check ended: its exit code (-1 when it did not exit by
// itself), whether that is a pass, whether its timeout stopped it, the tail
// of what it printed and the evidence of the run. Err is set only for an
// Engine with Hook set, when the check could not be run on the staged tree:
// it ran nothing.
type Outcome struct {
	Code     int
	OK       bool
	TimedOut bool
	Output   string
	Info     Info
	Err      error
}

// Info is what --json says of an opted-in check's evidence. Recorded is true
// only on the occurrence that ran the check and wrote its record; Reason says
// why a run that did not hit recorded nothing; Provenance says where the
// record was made.
type Info struct {
	Source     string            `json:"source"`
	Hit        bool              `json:"hit"`
	Recorded   bool              `json:"recorded"`
	Key        string            `json:"key,omitempty"`
	Tree       string            `json:"tree,omitempty"`
	RecordedAt string            `json:"recorded_at,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Provenance map[string]string `json:"provenance,omitempty"`
}

// Entry is the evidence of one command an invocation looked up, ci run's
// list: the command, then the Info.
type Entry struct {
	Command string `json:"command"`
	Info
}

// Engine looks opted-in checks up by the index tree and runs a miss in a
// checkout of it, one for each check, recording its result. One Engine is
// one invocation: it captures the index once, and keeps what it found.
type Engine struct {
	Cfg   *config.Loaded
	Git   Git
	Store privport.Store
	// Binding is the OS, architecture and version the key binds; ThisBinary's
	// in a run.
	Binding Binding
	// Work is the working folder, where declared inputs git does not track
	// are read from; "" is the folder itos runs in.
	Work string
	// CIEnv is ci.env when the run applies it, bound beside the declared
	// environment; nil when it does not.
	CIEnv map[string]string
	// Mode is who looked the check up, task or ci, for the record's
	// provenance; Hint is what the note on unstaged changes adds.
	Mode, Hint string
	// Now is the clock.
	Now func() time.Time
	// Hook is set for the commit-msg hook's checks, which judge the staged
	// tree or nothing: a check whose key cannot be read or whose checkout
	// cannot be made is refused (Outcome.Err), never run in the working
	// folder; and the engine says nothing of a miss or of the changes left
	// out, only that a hit answered.
	Hook bool

	spare     *Checkout
	used      bool
	treeKnown bool
	tree      string
	treeErr   error
	tracked   map[string]bool
	unstaged  int
	counted   bool
	entries   []Entry
}

// Used is whether the invocation looked an opted-in check up.
func (e *Engine) Used() bool { return e.used }

// Unstaged is how many tracked paths differ in the working folder from the
// index, counted when the first opted-in check was looked up; false when
// none was or git could not say.
func (e *Engine) Unstaged() (int, bool) { return e.unstaged, e.counted }

// Entries are the evidence of each command looked up, in run order.
func (e *Engine) Entries() []Entry { return e.entries }

// Spare hands the engine a checkout of the tree it looks up, which the first
// miss runs in instead of making its own. The caller removes it, as it would
// had no check used it.
func (e *Engine) Spare(co Checkout) { e.spare = &co }

// Hit is whether the check would be answered from a record now: nothing is
// printed, run, locked or counted.
func (e *Engine) Hit(ctx context.Context, req Request) bool {
	tree, err := e.indexTree(ctx)
	if err != nil {
		return false
	}
	b, err := e.bind(ctx, req, tree)
	if err != nil {
		return false
	}
	_, ok := e.lookup(ctx, b.Key())
	return ok
}

// checkout is the checkout a miss runs in: the spare, once, else a new one.
func (e *Engine) checkout(ctx context.Context, tree string) (Checkout, error) {
	if e.spare != nil {
		co := *e.spare
		e.spare = nil
		return co, nil
	}
	return e.Git.Snapshot(ctx, tree)
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}

// indexTree is the index captured as a tree, once.
func (e *Engine) indexTree(ctx context.Context) (string, error) {
	if !e.treeKnown {
		e.tree, e.treeErr = e.Git.IndexTree(ctx)
		e.treeKnown = true
	}
	return e.tree, e.treeErr
}

// indexed are the paths the index holds, once.
func (e *Engine) indexed(ctx context.Context) (map[string]bool, error) {
	if e.tracked == nil {
		paths, err := e.Git.IndexPaths(ctx)
		if err != nil {
			return nil, err
		}
		e.tracked = make(map[string]bool, len(paths))
		for _, p := range paths {
			e.tracked[p] = true
		}
	}
	return e.tracked, nil
}

// note counts the changes the run leaves out, the first time a check is
// looked up, and says so when there are any.
func (e *Engine) note(ctx context.Context, log io.Writer) {
	if e.counted || e.Hook {
		return
	}
	paths, err := e.Git.UnstagedPaths(ctx)
	if err != nil {
		return
	}
	e.unstaged, e.counted = len(paths), true
	if len(paths) == 0 {
		return
	}
	noun := "paths"
	if len(paths) == 1 {
		noun = "path"
	}
	fmt.Fprintf(log, "%d changed %s not staged: opted-in checks run on the staged tree.%s\n", len(paths), noun, e.Hint)
}

// say prints one status line of a check, after its command when the run is
// not verbose, which has printed it, and after the task it is for when the
// request names one.
func (e *Engine) say(req Request, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if req.Verbose {
		fmt.Fprintf(req.Log, "  %s\n", line)
		return
	}
	if req.Task != "" {
		fmt.Fprintf(req.Log, "  %s %s: %s\n", req.Task, req.Command, line)
		return
	}
	fmt.Fprintf(req.Log, "  %s: %s\n", req.Command, line)
}

// progress is say for what a run does before its result, which the hook
// keeps to itself.
func (e *Engine) progress(req Request, format string, args ...any) {
	if !e.Hook {
		e.say(req, format, args...)
	}
}

// Run looks the check up by the index tree and key. A hit answers with the
// recorded exit status and output, without a checkout. A miss takes the
// record's lock, looks again, runs the check in a checkout of the tree with
// the declared untracked inputs copied in, and records the result by
// renaming it into place. Whatever cannot be done (the index, the key, the
// lock, the checkout, the store) never changes the check's verdict: it runs
// uncached, in the checkout when there is one and else in the working
// folder, and Info says why it recorded nothing.
func (e *Engine) Run(ctx context.Context, req Request) Outcome {
	e.used = true
	info := Info{Source: Source}
	tree, err := e.indexTree(ctx)
	if err != nil {
		return e.finish(req, e.uncached(ctx, req, info, ReasonSnapshotUnusable, err))
	}
	info.Tree = tree
	e.note(ctx, req.Log)
	b, err := e.bind(ctx, req, tree)
	if err != nil {
		return e.finish(req, e.uncached(ctx, req, info, ReasonUnkeyable, err))
	}
	info.Key = b.Key()
	if rec, ok := e.lookup(ctx, info.Key); ok {
		return e.finish(req, e.replay(req, info, rec))
	}
	held, err := e.Store.Hold(ctx, filepath.Join(Folder, info.Key))
	if err != nil {
		reason := ReasonStoreUnusable
		if kind.Of(err) == kind.Temporary {
			reason = ReasonLockTimeout
		}
		return e.finish(req, e.execute(ctx, req, info, b, reason))
	}
	defer held.Release(ctx)
	if rec, ok := e.lookup(ctx, info.Key); ok {
		return e.finish(req, e.replay(req, info, rec))
	}
	return e.finish(req, e.execute(ctx, req, info, b, ""))
}

// finish keeps the outcome's evidence for the invocation's list.
func (e *Engine) finish(req Request, o Outcome) Outcome {
	e.entries = append(e.entries, Entry{Command: req.Command, Info: o.Info})
	return o
}

// replay answers from a record: its exit status, and, verbose, its output.
func (e *Engine) replay(req Request, info Info, rec record) Outcome {
	info.Hit, info.RecordedAt, info.Provenance = true, rec.result.Ended, rec.result.Provenance
	e.say(req, "cached evidence (%s, recorded %s): exit %d", Source, rec.result.Ended, rec.result.Exit)
	if req.Verbose {
		fmt.Fprint(req.Log, rec.result.Output)
		if rec.result.Output != "" && rec.result.Output[len(rec.result.Output)-1] != '\n' {
			fmt.Fprintln(req.Log)
		}
	}
	return Outcome{Code: rec.result.Exit, OK: rec.result.Exit == 0, Output: rec.result.Output, Info: info}
}

// uncached runs a check that cannot be keyed, in the working folder; the
// hook's refuses it, since its checks judge the staged tree or nothing.
func (e *Engine) uncached(ctx context.Context, req Request, info Info, reason string, cause error) Outcome {
	info.Reason = reason
	if e.Hook {
		return Outcome{Info: info, Err: kind.Else(kind.Missing, fmt.Errorf("cannot run %s on the staged tree (%s): %w", req.Command, reason, cause))}
	}
	e.say(req, "running (not recorded: %s)", reason)
	o, _ := e.run(ctx, req, info, "", nil)
	return o
}

// execute runs a miss in a checkout of its tree, and records it unless a
// reason, found before, says it cannot be. No checkout means the working
// folder, and no record.
func (e *Engine) execute(ctx context.Context, req Request, info Info, b bound, reason string) Outcome {
	if reason != "" {
		e.progress(req, "running (not recorded: %s)", reason)
	} else {
		e.progress(req, "running")
	}
	co, err := e.checkout(ctx, info.Tree)
	if err == nil {
		if err = copyInputs(co.Dir, e.Work, b.copies); err != nil {
			_ = co.Remove()
		}
	}
	if err != nil {
		info.Reason = ReasonSnapshotUnusable
		if e.Hook {
			return Outcome{Info: info, Err: kind.Else(kind.Missing, err)}
		}
		o, _ := e.run(ctx, req, info, "", nil)
		return o
	}
	defer co.Remove()
	info.Reason = reason
	o, res := e.run(ctx, req, info, co.Dir, co.Env)
	if reason != "" || o.Info.Reason != "" {
		return o
	}
	placed, kept, err := e.write(ctx, info.Key, b, res)
	if err != nil {
		o.Info.Reason = ReasonStoreUnusable
		return o
	}
	if !placed {
		o.Info.Reason, o.Info.RecordedAt, o.Info.Provenance = ReasonAlreadyRecorded, kept.result.Ended, kept.result.Provenance
		return o
	}
	o.Info.Recorded, o.Info.RecordedAt, o.Info.Provenance = true, res.Ended, res.Provenance
	return o
}

// run runs the command in dir (the working folder when ""), with env
// (itos's own when nil), keeping the tail of what it prints. A result that
// is not the check's to reuse, a timeout or a command that did not exit by
// itself, says so as the reason it recorded nothing.
func (e *Engine) run(ctx context.Context, req Request, info Info, dir string, env []string) (Outcome, Result) {
	tail := &tail{max: OutputTail}
	var out io.Writer = tail
	if req.Verbose {
		out = io.MultiWriter(tail, req.Log)
	}
	seconds := req.Seconds
	if req.RunSeconds != nil {
		seconds = *req.RunSeconds
	}
	started := e.now()
	r := shell.Run(e.Cfg, req.Command, shell.Options{
		Stdout: out, Stderr: out, Env: env, Dir: dir, Timeout: time.Duration(seconds * float64(time.Second)),
	})
	text, cut := tail.Text()
	res := Result{
		Exit: r.Code, Output: text, Truncated: cut,
		Started: started.Format(time.RFC3339), Ended: e.now().Format(time.RFC3339),
		Provenance: map[string]string{
			"itos": e.Binding.Version, "os": e.Binding.OS, "arch": e.Binding.Arch, "mode": e.Mode,
		},
	}
	if info.Reason == "" && (r.TimedOut || r.Err != nil || r.Code < 0) {
		info.Reason = ReasonInconclusive
	}
	return Outcome{Code: r.Code, OK: r.OK(), TimedOut: r.TimedOut, Output: text, Info: info}, res
}

// copyInputs copies the declared inputs git does not track from the working
// folder into a checkout: folders made, files copied with their executable
// bit, links made to the targets the key hashed.
func copyInputs(dir, work string, list []copying) error {
	for _, c := range list {
		to := filepath.Join(dir, filepath.FromSlash(c.path))
		var err error
		switch c.state {
		case "dir":
			err = os.MkdirAll(to, 0o755)
		case "link":
			if err = os.MkdirAll(filepath.Dir(to), 0o755); err == nil {
				err = os.Symlink(c.target, to)
			}
		default:
			err = copyFile(filepath.Join(work, filepath.FromSlash(c.path)), to, c.exec)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies one file into a folder made for it, executable or not.
func copyFile(from, to string, exec bool) error {
	text, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if exec {
		mode |= 0o100
	}
	if err = os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, text, mode)
}
