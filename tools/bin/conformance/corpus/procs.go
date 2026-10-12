package corpus

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// DeadlineError is a case that ran past its deadline: every process it started is killed, and
// the case fails, saying so.
type DeadlineError struct{ Deadline time.Duration }

func (e *DeadlineError) Error() string {
	return fmt.Sprintf("conformance case exceeded its %dms deadline", e.Deadline.Milliseconds())
}

// procs are the processes one case starts, to set it up, to run it and to inspect what it left,
// each the leader of a process group of its own (a process tree on windows). One deadline covers
// them all; when it passes, every group still running is killed. A process's group is killed as
// soon as it exits, so a descendant that holds its output open cannot hold the case.
type procs struct {
	mu       sync.Mutex
	active   map[int]bool
	expired  bool
	deadline time.Duration
	timer    *time.Timer
}

func newProcs(deadline time.Duration) *procs {
	p := &procs{active: map[int]bool{}, deadline: deadline}
	p.timer = time.AfterFunc(deadline, p.expire)
	return p
}

func (p *procs) expire() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expired = true
	for pid := range p.active {
		killGroup(pid)
	}
}

// expiredErr is the deadline's error once it has passed, else nil.
func (p *procs) expiredErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.expired {
		return &DeadlineError{p.deadline}
	}
	return nil
}

// close stops the deadline and kills every group still running.
func (p *procs) close() {
	p.timer.Stop()
	p.mu.Lock()
	defer p.mu.Unlock()
	for pid := range p.active {
		killGroup(pid)
	}
}

// result is a finished process: its exit code (nil when a signal ended it) and its output.
type result struct {
	code           *int
	stdout, stderr string
}

// errUnclosed is a process whose output stayed open after its group was killed.
var errUnclosed = &FixtureError{"owned conformance processes did not close after termination"}

// run runs a command to its end, input on its stdin. An error is one it could not start, a
// deadline passed, or output that stayed open after its group was killed.
func (p *procs) run(name string, args []string, dir string, env []string, input string) (result, error) {
	if err := p.expiredErr(); err != nil {
		return result{}, err
	}
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	ownGroup(cmd)
	inR, inW, err := os.Pipe()
	if err != nil {
		return result{}, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return result{}, errors.Join(err, inR.Close(), inW.Close())
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		return result{}, errors.Join(err, inR.Close(), inW.Close(), outR.Close(), outW.Close())
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, errW
	started := cmd.Start()
	_, _, _ = inR.Close(), outW.Close(), errW.Close()
	if started != nil {
		_, _, _ = inW.Close(), outR.Close(), errR.Close()
		return result{}, started
	}
	pid := cmd.Process.Pid
	p.mu.Lock()
	p.active[pid] = true
	if p.expired {
		killGroup(pid)
	}
	p.mu.Unlock()

	go func() {
		_, _ = io.WriteString(inW, input)
		_ = inW.Close()
	}()
	type read struct {
		text string
		err  error
	}
	reader := func(r *os.File) chan read {
		done := make(chan read, 1)
		go func() {
			b, err := io.ReadAll(r)
			done <- read{string(b), err}
		}()
		return done
	}
	stdout, stderr := reader(outR), reader(errR)
	_ = cmd.Wait()
	killGroup(pid)
	p.mu.Lock()
	delete(p.active, pid)
	p.mu.Unlock()

	var res result
	// A group killed closes its output at once; three seconds is a bound for a process that
	// left the group and kept the output, never for the command's own work.
	bound := time.NewTimer(3 * time.Second)
	defer bound.Stop()
	for _, r := range []struct {
		from chan read
		to   *string
		file *os.File
	}{{stdout, &res.stdout, outR}, {stderr, &res.stderr, errR}} {
		select {
		case got := <-r.from:
			*r.to = got.text
		case <-bound.C:
			_, _ = outR.Close(), errR.Close()
			return result{}, errUnclosed
		}
		_ = r.file.Close()
	}
	if err := p.expiredErr(); err != nil {
		return result{}, err
	}
	if ws := cmd.ProcessState; ws != nil && ws.Exited() {
		code := ws.ExitCode()
		res.code = &code
	}
	return res, nil
}
