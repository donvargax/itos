package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/git"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/version"
)

// A command takes the arguments after its name and gives its exit code, or
// an error that failure reports.
type command func(args []string, o Out) (int, error)

// commands is the command table, main.ts's COMMANDS.
var commands = map[string]command{
	"task":     task,
	"work":     workCommand,
	"commit":   commit,
	"verify":   verify,
	"tests":    testsCommand,
	"ci":       ciCommand,
	"hook":     hook,
	"guard":    guardCommand,
	"config":   configCommand,
	"version":  versionCommand,
	"push":     push,
	"git-shim": gitShim,
	"pin":      pinCommand,
	"upgrade":  upgradeCommand,
	"init":     initCommand,
	"followup": followCommand,
	"question": askCommand,
	"go":       goCommand,
	"guide":    guideCommand,
	"status":   statusCommand,
}

// flagValue is the value after a flag, and whether there is one.
func flagValue(args []string, name string) (string, bool) {
	i := slices.Index(args, name)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

// positional is the arguments that are neither flags nor the value of one of
// the valued flags.
func positional(args []string, valued ...string) []string {
	var found []string
	for i, a := range args {
		if strings.HasPrefix(a, "-") || (i > 0 && slices.Contains(valued, args[i-1])) {
			continue
		}
		found = append(found, a)
	}
	return found
}

// first is a list's first element, and whether it has one.
func first(list []string) (string, bool) {
	if len(list) == 0 {
		return "", false
	}
	return list[0], true
}

// split is a command's subcommand and the arguments after it.
func split(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

// workCommand is `work check [<file>]`, the file the argument after check
// whatever it is, as main.ts takes it, `work list [--all]`, which takes
// nothing else (plain work list lists the open items, --all every one,
// slice 79), `work take` and `work promote` (workwrite.go), `work done`
// (workdone.go), `work add` and `work edit` (workedit.go), `work queue`
// (workwrite.go), `work drop` (workdrop.go), `work show` (workshow.go), or
// `work [--as <handle>]`, which takes nothing else: an argument it does not
// know is a usage error, not one ignored (bug 11).
func workCommand(args []string, o Out) (int, error) {
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
	case "show":
		return workShow(rest, o)
	case "check":
		file, named := first(rest)
		return workCheck(typed(file), named, o)
	case "list":
		var others []string
		all := false
		for _, arg := range rest {
			if arg == "--all" {
				all = true
			} else {
				others = append(others, arg)
			}
		}
		if len(others) > 0 {
			return 0, usage("work list takes only --all: %s", strings.Join(others, " "))
		}
		return workList(all, o)
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
const workTakes = "it takes list, show, take, promote, done, add, edit, queue, drop or check, else --as <handle>"

// commit is a commit itself (gitCommit), every argument git commit's but
// itos's own flags, when there is no argument or the first is a flag or "--";
// a bare first word is itos's, one of commit's subcommands or a usage error
// (bug 5), so a mistyped subcommand never reaches git as a pathspec. Paths for
// git go after a flag or after "--".
func commit(args []string, o Out) (int, error) {
	sub, rest := split(args)
	if sub == "" || strings.HasPrefix(sub, "-") {
		return gitCommit(args, o)
	}
	switch sub {
	case "check-message":
		file, ok := first(positional(rest, "--at"))
		if !ok && !slices.Contains(rest, "-") {
			return 0, usage("commit check-message needs <file|->")
		}
		if !ok {
			file = "-"
		}
		at, _ := flagValue(rest, "--at")
		if err := commitRefs("commit check-message", at); err != nil {
			return 0, err
		}
		return checkMessage(typed(file), at, o)
	case "check-paths":
		typ, ok := flagValue(rest, "--type")
		if !ok {
			return 0, usage("commit check-paths needs --type <type>")
		}
		var files []string
		for _, f := range positional(rest, "--type") {
			files = append(files, filepath.ToSlash(typed(f)))
		}
		return checkPaths(typ, files, o)
	case "footers":
		footers := positional(rest)
		if len(footers) < 3 {
			return 0, usage("commit footers needs <name> <from> <to>")
		}
		if err := commitRefs("commit footers", footers[1], footers[2]); err != nil {
			return 0, err
		}
		return listFooters(footers[0], footers[1], footers[2], o)
	}
	return 0, usage("unknown command: commit %s", sub)
}

func verify(args []string, o Out) (int, error) {
	if len(args) == 0 && config.IsStealth(config.Path()) {
		return verifyRange(git.Unpushed, "HEAD", o)
	}
	if len(args) < 2 {
		return 0, usage("verify needs <from> <to>")
	}
	if err := commitRefs("verify", args[0], args[1]); err != nil {
		return 0, err
	}
	return verifyRange(args[0], args[1], o)
}

func testsCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "list":
		name, ok := first(positional(rest, "--at"))
		if !ok {
			return 0, usage("tests list needs <kind>")
		}
		at, _ := flagValue(rest, "--at")
		if at != "worktree" && at != "index" {
			if err := commitRefs("tests list", at); err != nil {
				return 0, err
			}
		}
		return testsList(name, at, o)
	case "moves":
		name, ok := first(positional(rest))
		if !ok {
			return 0, usage("tests moves needs <kind>")
		}
		return testsMoves(name, o)
	case "next-id":
		return testsNextID(rest, o)
	case "smoke":
	default:
		return 0, usage("unknown command: tests %s", sub)
	}
	action, more := split(rest)
	name, _ := first(positional(more, "--features"))
	if name == "" {
		if len(rest) == 0 {
			action = "check|ids|run"
		}
		return 0, usage("tests smoke %s needs <kind>", action)
	}
	switch action {
	case "check":
		var root *string
		if features, ok := flagValue(more, "--features"); ok {
			features = typed(features)
			root = &features
		}
		return smokeCheck(name, root, o)
	case "ids":
		return smokeIDs(name, o)
	case "run":
		// Everything after the kind is the runner's, as given; a -- right
		// after it is taken, so what looks like an itos flag can follow it,
		// and is not passed on (bug 8: they were read only after a --, and
		// --workers=1 without one was dropped without a word).
		runner := more[slices.Index(more, name)+1:]
		if len(runner) > 0 && runner[0] == "--" {
			runner = runner[1:]
		}
		return smokeRun(name, runner, o)
	}
	return 0, usage("unknown command: tests smoke %s", action)
}

func ciCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	range_ := positional(rest, "--data-at", "--head", "--base")
	from, to := "", ""
	if len(range_) > 0 {
		from = range_[0]
	}
	if len(range_) > 1 {
		to = range_[1]
	}
	nightly := slices.Contains(rest, "--nightly")
	switch sub {
	case "plan":
		whole := slices.Contains(rest, "--whole")
		if !nightly && !whole && len(range_) == 0 && config.IsStealth(config.Path()) {
			from, to = git.Unpushed, "HEAD"
		}
		if !nightly && !whole && to == "" {
			return 0, usage("ci plan needs <from> <to>, --nightly or --whole")
		}
		dataAt, _ := flagValue(rest, "--data-at")
		if err := rangeRefs("ci plan", from, to, dataAt); err != nil {
			return 0, err
		}
		return ciPlan(from, to, nightly, dataAt, o)
	case "run":
		if err := rangeRefs("ci run", from, to); err != nil {
			return 0, err
		}
		return ciRun(from, to, nightly, o)
	case "scope":
		if to == "" {
			return 0, usage("ci scope needs <from> <to>")
		}
		if err := rangeRefs("ci scope", from, to); err != nil {
			return 0, err
		}
		return ciScope(from, to, o)
	case "watch":
		return ciWatch(positional(rest), o)
	case "range":
		head, ok := flagValue(rest, "--head")
		if !ok {
			return 0, usage("ci range needs --head <sha>")
		}
		base, _ := flagValue(rest, "--base")
		return ciRange(head, base, o)
	}
	return 0, usage("unknown command: ci %s", sub)
}

func hook(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "commit-msg":
		if len(rest) == 0 {
			return 0, usage("hook commit-msg needs <file>")
		}
		return hookCommitMsg(typed(rest[0]), o)
	case "pre-push":
		return hookPrePush(o)
	case "install":
		manager, ok := flagValue(rest, "--manager")
		if ok && !slices.Contains(managers, manager) {
			return 0, usage("hook install --manager takes %s", strings.Join(managers, "|"))
		}
		return hookInstall(manager, slices.Contains(rest, "--print"), slices.Contains(rest, "--force"), o)
	}
	return 0, usage("unknown command: hook %s", sub)
}

// managers are the hook managers `hook install --manager` takes: the ones
// hooks.manager may name (hooks.ts's MANAGERS is config.ts's HOOK_MANAGERS).
var managers = config.HookManagers

func configCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	if sub == "get" {
		keys := positional(rest)
		if len(keys) != 1 || keys[0] == "" {
			return 0, usage("config get needs one <key>, a dotted path such as hooks.bin")
		}
		return configGet(keys[0], o)
	}
	if sub != "check" {
		return 0, usage("unknown command: config %s", sub)
	}
	if slices.Contains(args, "--print-defaults") {
		return printDefaults(o)
	}
	ledger, _ := flagValue(args, "--ledger")
	return configCheck(typed(ledger), o)
}

// versionCommand prints the version, or under --json the version with the
// config's requires and whether it holds; with --check, a version that does
// not satisfy requires exits 1. Only --check needs the config: a global itos
// is run outside any project first of all (issue #2), so without it a config
// that is missing or cannot be read leaves requires out rather than failing.
func versionCommand(args []string, o Out) (int, error) {
	check := slices.Contains(args, "--check")
	var requires *string
	cfg, err := config.Load(config.Path())
	switch {
	case err == nil:
		requires = cfg.Requires
	case check:
		return 0, err
	}
	v := version.Version()
	ok := requires == nil || version.Satisfies(v, *requires)
	if o.JSON {
		fields := []out.Field{{Key: "version", Value: v}}
		if requires != nil {
			fields = append(fields,
				out.Field{Key: "requires", Value: *requires},
				out.Field{Key: "satisfied", Value: ok})
		}
		if err := out.Emit(o.Stdout, fields...); err != nil {
			return 0, err
		}
	} else {
		fmt.Fprintf(o.Stdout, "itos %s\n", v)
	}
	if !check || ok {
		return 0, nil
	}
	fmt.Fprintf(o.Stderr, "itos %s does not satisfy %s (the config's requires)\n", v, *requires)
	return ExitPolicy, nil
}
