package cli

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	"github.com/donvargax/itos/v8/internal/out"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func runConfig(args []string, o Out) (int, error) {
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
	return configCheck(typed(o.Origin, ledger), o)
}

// Found is configcheck.Found.
type Found = configcheck.Found

// tagged is configcheck.Tagged.
func tagged(area string, problems []out.Problem) []Found {
	return configcheck.Tagged(area, problems)
}

// judgedSmokeFindings is configcheck.JudgedSmokeFindings.
func judgedSmokeFindings(cfg *config.Loaded, judged func(name, file string) bool, ws *tests.Warnings) ([]Found, []string) {
	return configcheck.JudgedSmokeFindings(cfg, judged, ws)
}

// configFindings is configcheck.Findings over git.
func configFindings(src port.Source, ledgerFile string, ws *tests.Warnings) (int, []Found, []Found, []string, error) {
	return configcheck.Findings(context.Background(), vcs.Git{}, trees.Trees{VCS: vcs.Git{}}, src, ledgerFile, ungroupedTaskFlags(), ws)
}

// ungroupedTaskFlags are the taskFlags that are none of groupNames: the
// flags ledger.group.label may not name.
func ungroupedTaskFlags() []string {
	return slices.DeleteFunc(taskFlags(), func(f string) bool { return slices.Contains(groupNames, f) })
}

// taskFlags are the flags itos task and its subcommands take, the global
// ones with them, by name.
func taskFlags() []string {
	names := []string{"--config", "--root"}
	for name := range switches {
		names = append(names, name)
	}
	task := specs["task"]
	for _, s := range append([]spec{task}, slices.Collect(maps.Values(task.Subs))...) {
		for _, f := range s.Flags {
			names = append(names, f.Name)
		}
	}
	return names
}

// configCheck is `config check [--ledger <file>]`: the config, then the
// ledger, the work registry, each kind's smoke set and the decision records, every problem with its
// rule id and, where one exists, a fix; then its warnings, printed as WARN
// and never failing it.
func configCheck(ledgerFile string, o Out) (int, error) {
	code, found, warnings, lines, err := configFindings(trees.Worktree, ledgerFile, o.Tests)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		if found == nil {
			found = []Found{}
		}
		fields := []out.Field{
			{Key: "config", Value: configPath()},
			{Key: "valid", Value: len(found) == 0},
			{Key: "problems", Value: found},
		}
		// Only when there is one, so a report without a warning is as it was.
		if len(warnings) > 0 {
			fields = append(fields, out.Field{Key: "warnings", Value: warnings})
		}
		err = out.Emit(o.Stdout, fields...)
	} else {
		for _, p := range found {
			fmt.Fprintf(o.Stderr, "FAIL %s\n", p.Message)
		}
		for _, p := range warnings {
			fmt.Fprintf(o.Stderr, "WARN %s\n", p.Message)
		}
	}
	if len(found) == 0 && !o.JSON && !o.Quiet {
		for _, line := range lines {
			fmt.Fprintln(o.Stdout, line)
		}
	}
	return code, err
}

// printDefaults is `config check --print-defaults`: the one table of
// defaults the loader applies, as YAML or JSON, as it applies to this
// config's ledger and to a stealth config. It checks nothing, so a config that does not load gets the
// table's own values.
func printDefaults(o Out) (int, error) {
	path := configPath()
	var file *value.Map
	if cfg, err := loadConfigFile(path); err == nil {
		file = cfg.File()
	}
	table := config.DefaultsFor(file, config.IsStealth(vcs.Git{}, path))
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "defaults", Value: table})
	}
	_, err := fmt.Fprint(o.Stdout, value.YAML(table))
	return 0, err
}

// configGet is `config get <key>`: the value at a dotted key path as the
// tools read it, the config's own or its default, from the config every
// command finds. A scalar prints bare on one line, a mapping or a list as
// JSON on one line, and a key with no value and no default prints nothing;
// under --json, {"schema":1,"key","value"}, value null for no value. A key
// the schema does not have is a usage error, before any config is read; a
// config that cannot be loaded fails as it does for every command.
func configGet(key string, o Out) (int, error) {
	if known, keys := config.KnownKey(key); !known {
		if fix, ok := config.RemovedKey(key); ok {
			return 0, usage("config get: %s was removed in v5; %s", key, fix)
		}
		if len(keys) > 0 {
			return 0, usage("config get: unknown key %s; the keys there are %s", key, strings.Join(keys, ", "))
		}
		return 0, usage("config get: unknown key %s", key)
	}
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	got := cfg.Get(key)
	if o.JSON {
		var v any = got
		if got == value.Undefined {
			v = nil
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "key", Value: key}, out.Field{Key: "value", Value: v})
	}
	switch got.(type) {
	case string, float64, bool:
		_, err = fmt.Fprintln(o.Stdout, value.String(got))
	case *value.Map, []any:
		_, err = fmt.Fprintln(o.Stdout, value.JSON(got))
	}
	return 0, err
}
