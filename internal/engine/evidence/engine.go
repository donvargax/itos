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
// always has one when it recorded nothing, except a writer whose record lost
// its place to a whole one that another writer made.
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
// its output through as it runs, to Log.
type Request struct {
	Command string
	Seconds float64
	Decl    config.Evidence
	Verbose bool
	Log     io.Writer
}

// Outcome is how the check ended: its exit code (-1 when it did not exit by
// itself), whether that is a pass, and the evidence of the run.
type Outcome struct {
	Code int
	OK   bool
	Info Info
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
	if e.counted {
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
// not verbose, which has printed it.
func (e *Engine) say(req Request, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if req.Verbose {
		fmt.Fprintf(req.Log, "  %s\n", line)
		return
	}
	fmt.Fprintf(req.Log, "  %s: %s\n", req.Command, line)
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
		return e.finish(req, e.uncached(ctx, req, info, ReasonSnapshotUnusable))
	}
	info.Tree = tree
	e.note(ctx, req.Log)
	b, err := e.bind(ctx, req, tree)
	if err != nil {
		return e.finish(req, e.uncached(ctx, req, info, ReasonUnkeyable))
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
	return Outcome{Code: rec.result.Exit, OK: rec.result.Exit == 0, Info: info}
}

// uncached runs a check that cannot be keyed, in the working folder.
func (e *Engine) uncached(ctx context.Context, req Request, info Info, reason string) Outcome {
	e.say(req, "running (not recorded: %s)", reason)
	info.Reason = reason
	o, _ := e.run(ctx, req, info, "", nil)
	return o
}

// execute runs a miss in a checkout of its tree, and records it unless a
// reason, found before, says it cannot be. No checkout means the working
// folder, and no record.
func (e *Engine) execute(ctx context.Context, req Request, info Info, b bound, reason string) Outcome {
	if reason != "" {
		e.say(req, "running (not recorded: %s)", reason)
	} else {
		e.say(req, "running")
	}
	co, err := e.Git.Snapshot(ctx, info.Tree)
	if err == nil {
		if err = copyInputs(co.Dir, e.Work, b.copies); err != nil {
			_ = co.Remove()
		}
	}
	if err != nil {
		info.Reason = ReasonSnapshotUnusable
		o, _ := e.run(ctx, req, info, "", nil)
		return o
	}
	defer co.Remove()
	info.Reason = reason
	o, res := e.run(ctx, req, info, co.Dir, co.Env)
	if reason != "" || o.Info.Reason != "" {
		return o
	}
	placed, err := e.write(ctx, info.Key, b, res)
	if err != nil {
		o.Info.Reason = ReasonStoreUnusable
		return o
	}
	if !placed {
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
	started := e.now()
	r := shell.Run(e.Cfg, req.Command, shell.Options{
		Stdout: out, Stderr: out, Env: env, Dir: dir, Timeout: time.Duration(req.Seconds * float64(time.Second)),
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
	return Outcome{Code: r.Code, OK: r.OK(), Info: info}, res
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
