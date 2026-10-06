// Package kind is the kind of an error itos meets, which alone gives the
// exit code it ends with (decision 36, docs/CLI.md rule 31): a usage or
// config error exits 2, an environment that lacks something 3, a failure that
// may pass when run again unchanged 75 (sysexits' EX_TEMPFAIL), and an error
// of no kind 70 (EX_SOFTWARE), which is itos's to fix. An error takes its kind
// where what failed is known, wrapped with one of these; the command line
// reads it (cli.ExitCode), through any wrapping added after.
package kind

import "errors"

// Kind is what an error says about the run that met it.
type Kind int

const (
	// Unknown is an error nobody classified: exit 70.
	Unknown Kind = iota
	// Usage is a command called wrongly, or a config or data file itos
	// refuses: exit 2.
	Usage
	// Missing is an environment without something the command needs: a
	// tool, a token, a release, a git repository: exit 3.
	Missing
	// Temporary is a failure the same command may not meet when run again
	// unchanged: no network, a server error, a rate limit, a run not done in
	// time: exit 75.
	Temporary
)

// Error is an error with its kind.
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }

func (e *Error) Unwrap() error { return e.Err }

// Wrap gives err the kind; nil stays nil. The outermost kind wins, so a
// caller that knows better can reclassify.
func Wrap(k Kind, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Kind: k, Err: err}
}

// Else gives err the kind unless it already has one: what a caller knows of
// every failure but those classified where they were made.
func Else(k Kind, err error) error {
	if err == nil || Of(err) != Unknown {
		return err
	}
	return Wrap(k, err)
}

// Of is err's kind, the outermost one it was wrapped with; Unknown when it
// has none.
func Of(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return Unknown
}
