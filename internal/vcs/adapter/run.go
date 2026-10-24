package adapter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// run runs git with the arguments as talk asks, its stdout written to out
// as well when out is not nil; a port.Failed error when git exits non-zero,
// holding its words on stderr, read as a remote's failure when remote is
// set; git's own error when it does not run.
func (g Git) run(ctx context.Context, talk port.Talk, out io.Writer, remote bool, args ...string) error {
	cmd := g.git().Command(ctx, args...)
	cmd.Env, cmd.Stdin = talk.Env, talk.Stdin
	if talk.Dir != "" {
		cmd.Dir = port.In(g.Dir, talk.Dir)
	}
	if talk.NoPrompt {
		env := talk.Env
		if env == nil {
			env = os.Environ()
		}
		cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0")
	}
	var words bytes.Buffer
	cmd.Stdout = writers(talk.Stdout, out)
	cmd.Stderr = writers(talk.Stderr, &words)
	if talk.Stdout != nil && talk.Stdout == talk.Stderr && out == nil {
		// One writer for both streams, so that git's writes to it never
		// race, as exec gives them one pipe.
		cmd.Stdout = cmd.Stderr
	}
	if talk.Owns {
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
	}
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return err
	}
	failed := &port.Failed{Command: "git " + strings.Join(args, " "), Code: exit.ExitCode(), Words: words.String()}
	if remote {
		failed.Failure = failure(failed.Words)
	}
	return failed
}

// writers are the writers given, none of them nil, as one; nil for none.
func writers(ws ...io.Writer) io.Writer {
	var to []io.Writer
	for _, w := range ws {
		if w != nil {
			to = append(to, w)
		}
	}
	switch len(to) {
	case 0:
		return nil
	case 1:
		return to[0]
	}
	return io.MultiWriter(to...)
}

// output is git's stdout as run gives it.
func (g Git) output(ctx context.Context, talk port.Talk, remote bool, args ...string) (string, error) {
	var out bytes.Buffer
	err := g.run(ctx, talk, &out, remote, args...)
	return out.String(), err
}

// failure is why a fetch, a push or an ls-remote failed, from git's words
// (internal/git's RemoteFailure and Refused).
func failure(words string) port.Failure {
	if git.Refused(words) {
		return port.Refused
	}
	switch git.RemoteFailure(words) {
	case kind.Temporary:
		return port.Unreachable
	case kind.Missing:
		return port.NoRepository
	}
	return port.Other
}

// literal is talk with every pathspec git reads taken as the path it is,
// never a pattern.
func literal(talk port.Talk) port.Talk {
	env := talk.Env
	if env == nil {
		env = os.Environ()
	}
	talk.Env = append(env[:len(env):len(env)], "GIT_LITERAL_PATHSPECS=1")
	return talk
}
