package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v3/internal/adr"
	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/ledger"
	"github.com/donvargax/itos/v3/internal/out"
	"github.com/donvargax/itos/v3/internal/tests"
	"github.com/donvargax/itos/v3/internal/value"
	"github.com/donvargax/itos/v3/internal/work"
)

// Found is a problem config check reports, with the area it is in: the
// config, the ledger, the registry, a smoke set, the decision records or, as
// a warning, the people.
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
func smokeFindings(cfg *config.Loaded) ([]Found, []string, error) {
	var found []Found
	var lines []string
	for _, name := range cfg.Tests.Keys {
		k := cfg.Tests.Values[name]
		if k.Smoke.File == nil || *k.Smoke.File == "" {
			continue
		}
		file := *k.Smoke.File
		smoke, err := tests.LoadSmoke(cfg, name)
		var own []out.Problem
		if err == nil {
			own, err = tests.SmokeIssues(cfg, name, smoke, nil)
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
// (a project's people file missing or unreadable, work.PeopleProblem) never
// sets it.
func configFindings(ledgerFile string) (int, []Found, []Found, []string, error) {
	path := config.Path()
	cfg, err := config.Load(path)
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
	var files []string
	if ledgerFile != "" {
		files = []string{ledgerFile}
	} else {
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
	smoke, smokeLines, err := smokeFindings(cfg)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	found := tagged("ledger", ledger.Issues(cfg, files))
	registry, err := work.Problems(cfg)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	var warnings []Found
	if p := work.PeopleProblem(cfg); p != nil {
		warnings = append(warnings, Found{*p, "people"})
	}
	found = append(found, tagged("registry", registry)...)
	found = append(found, smoke...)
	// The decision records in the folder work.decisions names (slice 74);
	// a folder that is not there holds none to check.
	records, err := adr.List(cfg.Work.Decisions)
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
	code, found, warnings, lines, err := configFindings(ledgerFile)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		if found == nil {
			found = []Found{}
		}
		fields := []out.Field{
			{Key: "config", Value: config.Path()},
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
	path := config.Path()
	var file *value.Map
	if cfg, err := config.Load(path); err == nil {
		file = cfg.File()
	}
	table := config.DefaultsFor(file, config.IsStealth(path))
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
		if len(keys) > 0 {
			return 0, usage("config get: unknown key %s; the keys there are %s", key, strings.Join(keys, ", "))
		}
		return 0, usage("config get: unknown key %s", key)
	}
	cfg, err := config.Load(config.Path())
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
