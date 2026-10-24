package cli

// The writes of the work registry (slice 52, features/work.feature) are the
// work slice's (internal/slices/work); what task add and draft write the
// registry through is here, over the record engine (internal/engine/record):
// a sound registry before anything is written (soundRegistry), and
// writeCommitted, which writes the files and commits them alone, through the
// hooks as any commit, whatever else the person has staged (git commit
// --only), with the header and body given. Under a stealth config the
// registry is in the git folder, tracked by nothing, so it is written and
// nothing is committed; and since every linked worktree writes the same
// files there, soundRegistry takes the registry's lock (the tracker store's
// Hold) before it reads, held until the command's write is done (bug 16).

import (
	"context"
	"fmt"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/work"
)

// refuseWork reports a registry command's problems and exits with code: on
// stderr, or under --json as {"ok": false, "problems": […]} on stdout.
func refuseWork(found []out.Problem, code int, o Out) (int, error) {
	if o.JSON {
		return code, out.Emit(o.Stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	for _, p := range found {
		fmt.Fprintln(o.Stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(o.Stderr, "  fix: %s\n", p.Fix)
		}
	}
	return code, nil
}

// soundRegistry is the config, the registry and its text, when the registry
// is there and sound: a command writes only a registry work check passes.
// Otherwise its problems are reported, exit 1, and the config is nil.
//
// Under a stealth config it first takes the registry's lock, which release,
// never nil and deferred by the caller, gives back: every command that
// writes the registry or a ledger file reads them here, and writes the
// registry, so the lock held from here to the write keeps another itos, in
// another worktree, from reading the files before this one's change and
// writing them over it (bug 16). A project's registry needs none: its
// writes are commits, which git's own index lock takes in turn.
func soundRegistry(o Out) (cfg *config.Loaded, registry work.Registry, text string, release func(), code int, err error) {
	release = func() {}
	if cfg, err = loadConfig(o.Dir); err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	t := tracker(cfg, o.OS)
	if release, err = holdTracker(t, o); err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	var found []out.Problem
	if registry, text, found, err = record.Sound(context.Background(), t, cfg); err != nil {
		return nil, work.Registry{}, "", release, 0, err
	}
	if len(found) > 0 {
		code, err := refuseWork(found, ExitPolicy, o)
		return nil, work.Registry{}, "", release, code, err
	}
	return cfg, registry, text, release, 0, nil
}

// written is one file a command writes: its path, its text before and
// after, whether it is new (not there before, so put back by removing it),
// and the rule that refuses it with changes no commit holds, when it is
// neither the registry, the questions nor a ledger file.
type written struct {
	path, old, text string
	created         bool
	rule            string
}

// writeCommitted is the record engine's Write (record.Write) through the
// config's tracker: the files written and, in a project, committed alone,
// through the hooks, git's words and the hooks' on stderr. Its result is
// the new commit's SHA, "" under a stealth config, which commits nothing. A
// file with changes no commit holds, or a commit refused, exits 1, every
// file as it was; the rest are errors. task add writes a ledger file and
// the registry (slice 55).
func writeCommitted(cfg *config.Loaded, files []written, header, body string, o Out) (string, int, error) {
	sha, found, err := record.Write(context.Background(), tracker(cfg, o.OS), cfg, records(files), record.Commit{
		Header: header,
		Body:   body,
		Quiet:  o.Quiet,
		// git's words and the hooks' go to stderr: stdout is the command's
		// own result.
		Stdin: os.Stdin,
		Words: o.Stderr,
		Env:   append(withoutFooters(os.Environ()), AmendEnv+"=0"),
	})
	if err != nil || len(found) == 0 {
		return sha, 0, err
	}
	code, err := refuseWork(found, ExitPolicy, o)
	return "", code, err
}

// records are the files as the record engine takes them.
func records(files []written) []record.File {
	converted := make([]record.File, len(files))
	for i, f := range files {
		converted[i] = record.File{Path: f.path, Old: f.old, Text: f.text, Created: f.created, Rule: f.rule}
	}
	return converted
}
