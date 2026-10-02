package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/version"
)

// A command takes the arguments after its name and gives its exit code, or
// an error that failure reports.
type command func(args []string, o Out) (int, error)

// commands is the command table, main.ts's COMMANDS.
var commands = map[string]command{
	"task":    task,
	"work":    workCommand,
	"commit":  commit,
	"verify":  verify,
	"tests":   testsCommand,
	"ci":      ciCommand,
	"hook":    hook,
	"hooks":   hooks,
	"config":  configCommand,
	"version": versionCommand,
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

func notPorted(path string) (int, error) { return 0, NotPorted{path} }

func workCommand(args []string, _ Out) (int, error) {
	if sub, _ := split(args); sub == "check" {
		return notPorted("work check")
	}
	return notPorted("work")
}

func commit(args []string, o Out) (int, error) {
	sub, rest := split(args)
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
		return checkMessage(file, at, o)
	case "check-paths":
		typ, ok := flagValue(rest, "--type")
		if !ok {
			return 0, usage("commit check-paths needs --type <type>")
		}
		return checkPaths(typ, positional(rest, "--type"), o)
	}
	return 0, usage("unknown command: commit %s", sub)
}

func verify(args []string, o Out) (int, error) {
	if len(args) < 2 {
		return 0, usage("verify needs <from> <to>")
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
		return testsList(name, at, o)
	case "moves":
		name, ok := first(positional(rest))
		if !ok {
			return 0, usage("tests moves needs <kind>")
		}
		return testsMoves(name, o)
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
			root = &features
		}
		return smokeCheck(name, root, o)
	case "ids":
		return smokeIDs(name, o)
	case "run":
		var runner []string
		if dash := slices.Index(more, "--"); dash >= 0 {
			runner = more[dash+1:]
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
		if !nightly && !slices.Contains(rest, "--whole") && to == "" {
			return 0, usage("ci plan needs <from> <to>, --nightly or --whole")
		}
		dataAt, _ := flagValue(rest, "--data-at")
		return ciPlan(from, to, nightly, dataAt, o)
	case "run":
		return ciRun(from, to, nightly, o)
	case "scope":
		if to == "" {
			return 0, usage("ci scope needs <from> <to>")
		}
		return ciScope(from, to, o)
	case "range":
		if _, ok := flagValue(rest, "--head"); !ok {
			return 0, usage("ci range needs --head <sha>")
		}
		return notPorted("ci range")
	}
	return 0, usage("unknown command: ci %s", sub)
}

func hook(args []string, _ Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "commit-msg":
		if len(rest) == 0 {
			return 0, usage("hook commit-msg needs <file>")
		}
		return notPorted("hook commit-msg")
	case "pre-push":
		return notPorted("hook pre-push")
	}
	return 0, usage("unknown command: hook %s", sub)
}

// managers are the hook managers `hooks install --manager` takes: the ones
// hooks.manager may name (hooks.ts's MANAGERS is config.ts's HOOK_MANAGERS);
// the hooks group ports what each one writes.
var managers = config.HookManagers

func hooks(args []string, _ Out) (int, error) {
	if sub, _ := split(args); sub != "install" {
		return 0, usage("unknown command: hooks %s", sub)
	}
	if manager, ok := flagValue(args, "--manager"); ok && !slices.Contains(managers, manager) {
		return 0, usage("hooks install --manager takes %s", strings.Join(managers, "|"))
	}
	return notPorted("hooks install")
}

func configCommand(args []string, o Out) (int, error) {
	if sub, _ := split(args); sub != "check" {
		return 0, usage("unknown command: config %s", sub)
	}
	if slices.Contains(args, "--print-defaults") {
		return printDefaults(o)
	}
	ledger, _ := flagValue(args, "--ledger")
	return configCheck(ledger, o)
}

// versionCommand prints the version, or under --json the version with the
// config's requires and whether it holds; with --check, a version that does
// not satisfy requires exits 1.
func versionCommand(args []string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	v := version.Version()
	requires := cfg.Requires
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
	if !slices.Contains(args, "--check") || ok {
		return 0, nil
	}
	fmt.Fprintf(o.Stderr, "itos %s does not satisfy %s (the config's requires)\n", v, *requires)
	return ExitPolicy, nil
}
