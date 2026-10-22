package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/work"
)

// runWork is `work check [<file>]`, the file the argument after check
// whatever it is, as main.ts takes it, `work list [--all]`, which takes
// nothing else (plain work list lists the open items, --all every one,
// slice 79), `work take` and `work promote` (workwrite.go), `work done`
// (workdone.go), `work add` and `work edit` (workedit.go), `work queue`
// (workwrite.go), `work drop` (workdrop.go), `work defer` and `work resume`
// (workdefer.go), `work give-back` (workgiveback.go), `work show`
// (workshow.go), or
// `work [--as <handle>]`, which takes nothing else: an argument it does not
// know is a usage error, not one ignored (bug 11).
func runWork(args []string, o Out) (int, error) {
	switch sub, rest := split(args); sub {
	case "take":
		return workTake(rest, o)
	case "promote":
		return workPromote(rest, o)
	case "done":
		return workDone(rest, o)
	case "add":
		return workAdd(rest, o)
	case "edit":
		return workEdit(rest, o)
	case "queue":
		return workQueue(rest, o)
	case "drop":
		return workDrop(rest, o)
	case "defer":
		return workDefer(rest, o)
	case "resume":
		return workResume(rest, o)
	case "give-back":
		return workGiveBack(rest, o)
	case "show":
		return workShow(rest, o)
	case "check":
		file, named := first(rest)
		return workCheck(typed(o.Origin, file), named, o)
	case "list":
		others, values, set, err := subArgs("work", "list", rest, []string{"--tag"}, []string{"--all"})
		if err != nil {
			return 0, err
		}
		if len(others) > 0 {
			return 0, usage("work list takes only --all and --tag <tag>: %s", strings.Join(others, " "))
		}
		return workList(set["--all"], values["--tag"], o)
	}
	as := ""
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--as":
			if i+1 >= len(args) {
				return 0, usage("work --as needs a handle")
			}
			i++
			as = args[i]
		case strings.HasPrefix(arg, "--as="):
			as = strings.TrimPrefix(arg, "--as=")
		case strings.HasPrefix(arg, "-"):
			return 0, usage("work does not take %s: %s", arg, workTakes)
		default:
			return 0, usage("work has no subcommand %s: %s", arg, workTakes)
		}
	}
	return workProposal(as, o)
}

// workTakes is what work takes, as a usage error names it.
const workTakes = "it takes list, show, take, promote, done, add, edit, queue, drop, defer, resume, give-back or check, else --as <handle>"

// workCheck is `work check [<file>]` (work.ts's workCheck): the registry's
// problems, each message on stderr, exit 1 on one; `<file>: sound` on stdout
// otherwise, unless -q. A named file that is not there is a problem found
// before the config is read, as the TypeScript reads it only to load the
// registry.
func workCheck(path string, named bool, o Out) (int, error) {
	var found []out.Problem
	file := path
	if named && !trees.Worktree.Has(context.Background(), path) {
		found = []out.Problem{work.Missing(path, true)}
	} else {
		cfg, err := loadConfig()
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
// owns nothing yet, and stderr says so. The provider is asked only when the
// people list several (bug 46): one person is the session, and with no one
// in the people (a people file missing, unreadable or listing nobody) the
// session is nobody and nothing is said of them. Under a stealth
// config with no --as nobody is asked (slice 38): the person is the only
// one, so the session owns every item, whatever owner it names; --as still
// proposes that handle's, as in a project.
func workProposal(as string, o Out) (int, error) {
	if code, err := workCheck("", false, Out{Quiet: true, Stdout: o.Stdout, Stderr: o.Stderr, Tests: o.Tests, Origin: o.Origin}); code != 0 || err != nil {
		return code, err
	}
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return 0, err
	}
	var proposal work.Proposal
	if cfg.Stealth && as == "" {
		proposal = work.ProposeEvery(registry)
	} else {
		listedIn := cfg.Work.People.File
		who := work.Whoami(registry, listedIn, as, providers.IdentityProvider(cfg))
		switch {
		case who.Problem != "":
			fmt.Fprintln(o.Stderr, who.Problem)
			if as != "" {
				return ExitMissing, nil
			}
		case who.Handle != "" && !who.Listed:
			fmt.Fprintf(o.Stderr, "%s is not in %s: nothing is theirs yet\n", who.Handle, listedIn)
		}
		proposal = work.Propose(registry, who.Handle)
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, proposal.Fields()...)
	}
	proposal.Print(o.Stdout)
	return 0, nil
}

// workList is `work list` (slice 43), the open items of the registry
// (every one neither done nor dropped, slice 79 and bug 25) in its order,
// whatever their kind or owner, a deferred one marked so; with --all (slice 77) every item, done and
// dropped ones too; with --tag <tag> (slice 97) only those carrying the tag.
// Plain work list printed every item until v4.0.0, so a
// caller that needs every item (the Claude Code plugin's titles) asks for
// --all. It judges nothing, as task list does not, so a registry with
// problems still lists; a registry that is not there is the one problem it
// reports, on stderr whatever --json says, exit 1. Under --json each item
// is written as work --json writes it, so the two describe an item alike,
// and file says where the registry was read.
func workList(all bool, tag string, o Out) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	file := cfg.Work.Registry
	if !cfg.Source.Has(context.Background(), file) {
		fmt.Fprintln(o.Stderr, work.Missing(file, false).Message)
		return ExitPolicy, nil
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return 0, err
	}
	listed := registry.Items
	if !all {
		listed = work.Open(listed)
	}
	if tag != "" {
		listed = work.Tagged(listed, tag)
	}
	if o.JSON {
		items := make([]any, len(listed))
		for i, item := range listed {
			items[i] = item
		}
		return 0, out.Emit(o.Stdout,
			out.Field{Key: "file", Value: file},
			out.Field{Key: "items", Value: items})
	}
	work.PrintList(o.Stdout, listed)
	return 0, nil
}
