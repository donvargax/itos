package cli

import (
	"fmt"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/providers"
	"github.com/donvargax/itos/v2/internal/source"
	"github.com/donvargax/itos/v2/internal/work"
)

// workCheck is `work check [<file>]` (work.ts's workCheck): the registry's
// problems, each message on stderr, exit 1 on one; `<file>: sound` on stdout
// otherwise, unless -q. A named file that is not there is a problem found
// before the config is read, as the TypeScript reads it only to load the
// registry.
func workCheck(path string, named bool, o Out) (int, error) {
	var found []out.Problem
	file := path
	if named && !source.Has(path) {
		found = []out.Problem{work.Missing(path, true)}
	} else {
		cfg, err := config.Load(config.Path())
		if err != nil {
			return 0, err
		}
		if !named {
			file = cfg.Work.Registry
		}
		if found, err = work.ProblemsAt(cfg, file, named); err != nil {
			return 0, err
		}
	}
	if o.JSON {
		if found == nil {
			found = []out.Problem{}
		}
		if err := out.Emit(o.Stdout,
			out.Field{Key: "file", Value: file},
			out.Field{Key: "sound", Value: len(found) == 0},
			out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	} else {
		for _, p := range found {
			fmt.Fprintln(o.Stderr, p.Message)
		}
	}
	if len(found) > 0 {
		return ExitPolicy, nil
	}
	if !o.JSON && !o.Quiet {
		fmt.Fprintf(o.Stdout, "%s: sound\n", file)
	}
	return 0, nil
}

// workProposal is `work [--as <handle>]` (work.ts's work): what the person
// can start. Exit 1 when the registry is not sound, its problems on stderr
// whatever --json says; 3 (a missing environment) when --as is not among the
// people. An identity provider that cannot say who the session is, or names
// someone the people do not list, is not an error: the session is nobody, or
// owns nothing yet, and stderr says so.
func workProposal(as string, o Out) (int, error) {
	if code, err := workCheck("", false, Out{Quiet: true, Stdout: o.Stdout, Stderr: o.Stderr}); code != 0 || err != nil {
		return code, err
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return 0, err
	}
	listedIn := cfg.Work.People.File
	who := work.Whoami(registry.Logins, listedIn, as, providers.IdentityProvider(cfg, o.Stderr))
	switch {
	case who.Problem != "":
		fmt.Fprintln(o.Stderr, who.Problem)
		if as != "" {
			return ExitMissing, nil
		}
	case !who.Listed:
		fmt.Fprintf(o.Stderr, "%s is not in %s: nothing is theirs yet\n", who.Handle, listedIn)
	}
	proposal := work.Propose(registry, who.Handle)
	if o.JSON {
		return 0, out.Emit(o.Stdout, proposal.Fields()...)
	}
	proposal.Print(o.Stdout)
	return 0, nil
}
