package check

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/ledger"
	"github.com/donvargax/itos/v6/internal/shell"
	"github.com/donvargax/itos/v6/internal/value"
)

// Result is what a check came to: it passed (its exit status is the one its
// run: or fails: wants), it failed, or it waits for the push.
type Result string

const (
	Pass    Result = "pass"
	Fail    Result = "fail"
	Pending Result = "pending"
)

// Pushed is whether a remote branch holds HEAD (`git branch -r --contains
// HEAD`), when an after: push check may run; not when git cannot say.
func Pushed() bool {
	branches, err := git.Output("branch", "-r", "--contains", "HEAD")
	return err == nil && value.Trim(branches) != ""
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
// whitespace collapsed as the config's patterns read it, and the same
// timeout. Whether it must pass or fail is not part of it: each task reads
// the one exit status by its own run: or fails:.
func Key(cfg *config.Loaded, c ledger.Check) string {
	return config.Normal(c.Command()) + "\x00" + value.Number(Timeout(cfg, c))
}

// Runner runs checks for one invocation. Runs holds each distinct check's
// exit status, 0 or not, by its Key: a check already in it is not run again,
// and a verbose command line says so. Nothing is kept beyond the Runner, so
// nothing is kept between invocations. A Runner with nil Runs runs every
// check it is given.
type Runner struct {
	Cfg  *config.Loaded
	Runs map[string]bool
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
	if c.Pushed() && !Pushed() {
		return Pending
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
	if !ran {
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
	}
	if ok != c.MustFail() {
		return Pass
	}
	return Fail
}
