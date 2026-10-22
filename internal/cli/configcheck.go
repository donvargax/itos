package cli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/donvargax/itos/v8/internal/adr"
	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	"github.com/donvargax/itos/v8/internal/work"
)

// Found is a problem config check reports, with the area it is in: the
// config, the ledger, the registry, a smoke set, the decision records or, as
// a warning, the ledger or the people.
type Found struct {
	out.Problem
	Area string `json:"area"`
}

func tagged(area string, problems []out.Problem) []Found {
	found := make([]Found, len(problems))
	for i, p := range problems {
		found[i] = Found{p, area}
	}
	return found
}

// smokeFindings are each kind's smoke set against its tests, and the line
// each sound one prints; a set that cannot be read is one problem.
func smokeFindings(cfg *config.Loaded, ws *tests.Warnings) ([]Found, []string, error) {
	return judgedSmokeFindings(cfg, nil, ws)
}

// judgedSmokeFindings are smokeFindings for the kinds judged picks by name
// and smoke file (the commit-msg hook's, slice 117), or for every kind with a
// smoke set when judged is nil.
func judgedSmokeFindings(cfg *config.Loaded, judged func(name, file string) bool, ws *tests.Warnings) ([]Found, []string, error) {
	var found []Found
	var lines []string
	for _, name := range cfg.Tests.Keys {
		k := cfg.Tests.Values[name]
		if k.Smoke.File == nil || *k.Smoke.File == "" {
			continue
		}
		file := *k.Smoke.File
		if judged != nil && !judged(name, file) {
			continue
		}
		smoke, err := tests.LoadSmoke(cfg, name)
		var own []out.Problem
		if err == nil {
			own, err = tests.SmokeIssues(cfg, name, smoke, nil, ws)
		}
		if err != nil {
			found = append(found, Found{out.Problem{Rule: "smoke-unreadable", Message: err.Error(), Fix: "correct " + file}, "smoke"})
			continue
		}
		found = append(found, tagged("smoke", own)...)
		count := len(tests.SmokeIDs(k, smoke))
		if k.Smoke.EveryFile {
			lines = append(lines, fmt.Sprintf("%s: every file has a smoke test (%d in all)", file, count))
		} else {
			lines = append(lines, fmt.Sprintf("%s: every smoke test is live (%d in all)", file, count))
		}
	}
	return found, lines, nil
}

// configFindings is what config check finds, what it warns of, and the
// lines it prints when it finds nothing (config-check.ts's configFindings).
// Its code is 2 when the config is invalid or the ledger's folder is
// missing, since nothing else can be read, else 1 for any problem; a warning
// (a check static by a ci.cost.static pattern that written order runs late,
// ledger.Findings, a project's people file missing or unreadable,
// work.PeopleProblem, or an answered question naming an item the registry
// does not have, asksProblems) never sets it. The config and its data are
// read from src.
func configFindings(src port.Source, ledgerFile string, ws *tests.Warnings) (int, []Found, []Found, []string, error) {
	path := config.Path(vcs.Git{}, src)
	cfg, err := config.Load(vcs.Git{}, trees.Trees{}, src, path)
	var invalid *config.Error
	if errors.As(err, &invalid) {
		return 2, prefixed(invalid, "config"), nil, nil, nil
	}
	if err != nil {
		return 0, nil, nil, nil, err
	}
	// A commits.since, or a footer's since, this repository does not have makes the config
	// unusable for verify, as an invalid key does.
	if missing := cfg.SinceIssues(); len(missing) > 0 {
		return 2, prefixed(&config.Error{File: path, Problems: missing}, "config"), nil, nil, nil
	}
	// A ledger layout that reads nothing, or reads the registry, is judged
	// before its folder is listed, which {group} as a folder would fail as
	// missing; its files are then not read (slice 130).
	layout := ledger.LayoutProblems(cfg)
	var files []string
	if ledgerFile != "" {
		files = []string{ledgerFile}
	} else if len(layout) == 0 {
		listed, err := ledger.Files(cfg)
		// The ledger's folder missing is a config error too: nothing of the
		// ledger can be read, so it is the one problem.
		if errors.As(err, &invalid) {
			return 2, prefixed(invalid, "ledger"), nil, nil, nil
		}
		if err != nil {
			return 0, nil, nil, nil, err
		}
		for _, f := range listed {
			files = append(files, f.Path)
		}
	}
	smoke, smokeLines, err := smokeFindings(cfg, ws)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	ledgerProblems, ledgerWarnings := ledger.Findings(cfg, files)
	// Values the config loads with that nothing reads as written (slice 129),
	// and the ledger layout's.
	unusable := append(cfg.Unusable(), groupLabelProblems(cfg)...)
	found := prefixed(&config.Error{File: path, Problems: append(unusable, layout...)}, "config")
	found = append(found, tagged("ledger", ledgerProblems)...)
	registry, err := work.Problems(cfg)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	warnings := tagged("ledger", ledgerWarnings)
	if p := work.PeopleProblem(cfg); p != nil {
		warnings = append(warnings, Found{*p, "people"})
	}
	found = append(found, tagged("registry", registry)...)
	asks, stale, err := asksProblems(cfg)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	found = append(found, tagged("asks", asks)...)
	warnings = append(warnings, tagged("asks", stale)...)
	found = append(found, smoke...)
	// The decision records in the folder work.decisions names (slice 74);
	// a folder that is not there holds none to check.
	records, err := adr.List(cfg.Source, cfg.Work.Decisions)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	found = append(found, tagged("decisions", adr.Problems(cfg.Work.Decisions, records))...)
	lines := append([]string{
		fmt.Sprintf("%s is valid, and so are the %d ledger files it reads", path, len(files)),
		cfg.Work.Registry + ": sound",
	}, smokeLines...)
	code := 0
	if len(found) > 0 {
		code = 1
	}
	return code, found, warnings, lines, nil
}

// asksProblems are the problems of the questions' file, work.asks, read
// from the config's source as the ledger and the registry are (slice 130): a
// file not as itos decision writes it (ask.Parse: an unknown key, an id given
// twice, ...), else each open question naming an item the registry does not
// have, since it holds that item up. Its warnings are each answered question
// naming one (bug 105): an answered question is a record, and its item gone,
// renamed or dropped, breaks nothing a command does. None when the file is
// not there; a registry that cannot be read is work.Problems' to report.
func asksProblems(cfg *config.Loaded) (problems, warnings []out.Problem, err error) {
	file := cfg.Work.Asks
	if !cfg.Source.Has(context.Background(), file) {
		return nil, nil, nil
	}
	text, err := cfg.Source.Read(context.Background(), file)
	if err != nil {
		return nil, nil, err
	}
	f, err := ask.Parse(file, text)
	if err != nil {
		return []out.Problem{{
			Rule:    "asks-invalid",
			Message: err.Error(),
			Fix:     "correct " + file + " to what itos decision writes, or undo the hand edit",
		}}, nil, nil
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, nil, nil
	}
	items := map[string]bool{}
	for _, it := range registry.Items {
		items[value.String(it.At("id"))] = true
	}
	for _, q := range f.Questions {
		if q.Item == "" || items[q.Item] {
			continue
		}
		if q.Status() == ask.Answered {
			warnings = append(warnings, out.Problem{
				Rule:    "asks-answered-unknown-item",
				Message: fmt.Sprintf("%s: %s, answered, names the item %s, which %s does not have", file, q.ID, q.Item, cfg.Work.Registry),
				Fix:     "name the item's new id in " + q.ID + "'s item, or remove its item",
			})
			continue
		}
		problems = append(problems, out.Problem{
			Rule:    "asks-unknown-item",
			Message: fmt.Sprintf("%s: %s names the item %s, which %s does not have", file, q.ID, q.Item, cfg.Work.Registry),
			Fix:     "name an item of " + cfg.Work.Registry + " in " + q.ID + "'s item, or remove its item",
		})
	}
	return problems, warnings, nil
}

// groupLabelProblems: itos task takes a group by --<ledger.group.label> as
// by --group, so a label that cannot be typed as one flag (empty, a space, an
// = or a leading -), or that is another flag of itos task's (skip, pending, a
// global one such as json), never takes a group: --skip T-005 selected
// nothing.
func groupLabelProblems(cfg *config.Loaded) []out.Problem {
	label := cfg.Ledger.Group.Label
	flag := "--" + label
	switch {
	case label == "" || strings.HasPrefix(label, "-") || strings.ContainsRune(label, '=') ||
		strings.IndexFunc(label, unicode.IsSpace) >= 0:
		return []out.Problem{{
			Rule:    "config-group-label",
			Message: fmt.Sprintf("ledger.group.label %q cannot be typed as itos task's flag %s", label, flag),
			Fix:     "set ledger.group.label to one word, with no space, = or leading -, such as phase or milestone",
		}}
	case !slices.Contains(group, val(flag)) && slices.Contains(taskFlags(), flag):
		return []out.Problem{{
			Rule:    "config-group-label",
			Message: "ledger.group.label is " + label + ", and " + flag + " is already another flag of itos task, so it never takes a group",
			Fix:     "set ledger.group.label to a word that is no other flag of itos task, such as phase or milestone",
		}}
	}
	return nil
}

// taskFlags are the flags itos task and its subcommands take, the global
// ones with them, by name.
func taskFlags() []string {
	names := []string{"--config", "--root"}
	for name := range switches {
		names = append(names, name)
	}
	task := specs["task"]
	for _, s := range append([]spec{task}, slices.Collect(maps.Values(task.subs))...) {
		for _, f := range s.flags {
			names = append(names, f.name)
		}
	}
	return names
}

// prefixed are a config error's problems, each message after the file's
// name, in the area given.
func prefixed(e *config.Error, area string) []Found {
	found := tagged(area, e.Problems)
	for i := range found {
		found[i].Message = e.File + ": " + found[i].Message
	}
	return found
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
