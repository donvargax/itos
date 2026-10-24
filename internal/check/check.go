package check

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/shell"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Result is what a check came to: it passed (its exit status is the one its
// run: or fails: wants), it failed, or it waits for the push.
type Result string

const (
	Pass    Result = "pass"
	Fail    Result = "fail"
	Pending Result = "pending"
)

// Pushed is whether a remote branch holds HEAD: no commit of HEAD is on no
// remote branch, when an after: push check may run; not when v cannot say.
func Pushed(v vcs.VCS) bool {
	unpushed, err := v.Commits(context.Background(), vcs.Range{From: vcs.Unpushed, To: "HEAD"})
	return err == nil && len(unpushed) == 0
}

// Timeout is a check's own timeout in seconds: its timeout:, else the
// ledger's (ledger.check.timeout).
func Timeout(cfg *config.Loaded, c ledger.Check) float64 {
	if c.Timeout != nil {
		return *c.Timeout
	}
	return cfg.Ledger.Check.Timeout
}

// duration is a timeout in seconds as a time.Duration; 0 is none, as
// spawnSync reads a timeout of 0.
func duration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

// Key is what makes two checks the same check: the same command, its
// whitespace collapsed as the config's patterns read it, the same timeout
// and the same evidence declaration, or none: a check that opts in and one
// that does not are two runs, each recorded, or not, by its own declaration.
// Whether it must pass or fail is not part of it: each task reads the one
// exit status by its own run: or fails:. A check that declares evidence and
// runs in the working folder (snapshot: false) is not the one that runs in
// the snapshot, whatever else they share; without evidence both run in the
// working folder and stay one run.
func Key(cfg *config.Loaded, c ledger.Check) string {
	key := config.Normal(c.Command()) + "\x00" + value.Number(Timeout(cfg, c))
	if c.Evidence != nil {
		key += "\x00evidence:" + c.Evidence.Text()
		if c.NoSnapshot {
			key += "\x00snapshot:false"
		}
	}
	return key
}

// Runner runs checks for one invocation. Runs holds each distinct check's
// exit status, 0 or not, by its Key: a check already in it is not run again,
// and a verbose command line says so. Nothing is kept beyond the Runner, so
// nothing is kept between invocations. A Runner with nil Runs runs every
// check it is given.
type Runner struct {
	Cfg  *config.Loaded
	Runs map[string]bool
	// Evidence looks up and records the checks that opt in with an evidence
	// declaration; nil runs every check as it always did, in the working
	// folder, nothing looked up or written.
	Evidence *evidence.Engine
	// infos is the evidence of each run kept in Runs, which a later check
	// that reads its exit status reports as its own.
	infos map[string]evidence.Info
	// Verbose prints each check's command line and lets its output through;
	// otherwise a check runs with its streams on /dev/null.
	Verbose bool
	// Stdout and Stderr are itos's streams. ToStderr sends a verbose check's
	// command line and output to Stderr, so that --json keeps Stdout for its
	// one object.
	Stdout, Stderr io.Writer
	ToStderr       bool
	// Stdin is what a verbose check reads; os.Stdin when nil.
	Stdin io.Reader
}

// NewRunner is a Runner that keeps its runs, writing to the given streams.
func NewRunner(cfg *config.Loaded, stdout, stderr io.Writer) *Runner {
	return &Runner{Cfg: cfg, Runs: map[string]bool{}, Stdout: stdout, Stderr: stderr}
}

// Run runs one check (checks.ts's runCheck): pending while an after: push
// check waits for the push, whatever another task found running its command;
// else its exit status, reused when the invocation has run the same check,
// read by its run: or fails:. The command runs through the config's shell
// with its timeout; one past it is stopped, its status not 0.
func (r *Runner) Run(c ledger.Check) Result {
	result, _ := r.RunInfo(c)
	return result
}

// RunInfo is Run, with the evidence of the check when it opted in and the
// Runner has an Evidence engine: a check opted in is looked up by the index
// tree and runs, on a miss, in a checkout of it (never an after: push check,
// which is neither looked up nor recorded, whether it runs or waits); a check
// that reads the exit
// status of a run its invocation already made reports that run's evidence,
// not recorded again. A check that did not opt in has none.
func (r *Runner) RunInfo(c ledger.Check) (Result, *evidence.Info) {
	if c.Pushed() && !Pushed(r.Cfg.VCS) {
		return Pending, nil
	}
	key := Key(r.Cfg, c)
	reused, ran := r.Runs[key]
	log := r.Stdout
	if r.ToStderr {
		log = r.Stderr
	}
	if r.Verbose {
		mode, note := "", ""
		if c.MustFail() {
			mode = "   (must fail)"
		}
		if ran {
			note = "   (ran above; its exit status reused)"
		}
		fmt.Fprintf(log, "  $ %s%s%s\n", c.Command(), mode, note)
	}
	ok := reused
	var info *evidence.Info
	switch {
	case ran:
		if kept, has := r.infos[key]; has {
			kept.Recorded = false
			info = &kept
		}
	case c.Evidenced() && r.Evidence != nil && !c.Pushed():
		o := r.Evidence.Run(context.Background(), evidence.Request{
			Command: c.Command(), Seconds: Timeout(r.Cfg, c), Decl: *c.Evidence, Verbose: r.Verbose, Log: log,
		})
		ok, info = o.OK, &o.Info
	default:
		var o shell.Options
		if r.Verbose {
			o = shell.Options{Stdin: r.Stdin, Stdout: log, Stderr: r.Stderr}
			if o.Stdin == nil {
				o.Stdin = os.Stdin
			}
		}
		o.Timeout = duration(Timeout(r.Cfg, c))
		ok = shell.Run(r.Cfg, c.Command(), o).OK()
	}
	if r.Runs != nil {
		r.Runs[key] = ok
		if info != nil {
			if r.infos == nil {
				r.infos = map[string]evidence.Info{}
			}
			r.infos[key] = *info
		}
	}
	if ok != c.MustFail() {
		return Pass, info
	}
	return Fail, info
}
