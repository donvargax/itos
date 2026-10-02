package cli

import (
	"errors"
	"fmt"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/ledger"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/tests"
	"github.com/donvargax/itos/internal/value"
	"github.com/donvargax/itos/internal/work"
)

// Found is a problem config check reports, with the area it is in: the
// config, the ledger, the registry or a smoke set.
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
			own, err = tests.SmokeIssues(cfg, name, smoke)
		}
		if errors.Is(err, tests.ErrCommandAdapter) {
			return nil, nil, NotPorted{"tests." + name + "'s command adapter"}
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

// configFindings is what config check finds and the lines it prints when it
// finds nothing (config-check.ts's configFindings). Its code is 2 when the
// config is invalid or the ledger's folder is missing, since nothing else can
// be read, else 1 for any problem.
func configFindings(ledgerFile string) (int, []Found, []string, error) {
	path := config.Path()
	cfg, err := config.Load(path)
	var invalid *config.Error
	if errors.As(err, &invalid) {
		return 2, prefixed(invalid, "config"), nil, nil
	}
	if err != nil {
		return 0, nil, nil, err
	}
	// A commits.since this repository does not have makes the config
	// unusable for verify, as an invalid key does.
	if missing := cfg.SinceIssue(); missing != nil {
		return 2, prefixed(&config.Error{File: path, Problems: []out.Problem{*missing}}, "config"), nil, nil
	}
	var files []string
	if ledgerFile != "" {
		files = []string{ledgerFile}
	} else {
		listed, err := ledger.Files(cfg)
		// The ledger's folder missing is a config error too: nothing of the
		// ledger can be read, so it is the one problem.
		if errors.As(err, &invalid) {
			return 2, prefixed(invalid, "ledger"), nil, nil
		}
		if err != nil {
			return 0, nil, nil, err
		}
		for _, f := range listed {
			files = append(files, f.Path)
		}
	}
	smoke, smokeLines, err := smokeFindings(cfg)
	if err != nil {
		return 0, nil, nil, err
	}
	found := tagged("ledger", ledger.Issues(cfg, files))
	registry, err := work.Problems(cfg)
	if err != nil {
		return 0, nil, nil, err
	}
	found = append(found, tagged("registry", registry)...)
	found = append(found, smoke...)
	lines := append([]string{
		fmt.Sprintf("%s is valid, and so are the %d ledger files it reads", path, len(files)),
		cfg.Work.Registry + ": sound",
	}, smokeLines...)
	code := 0
	if len(found) > 0 {
		code = 1
	}
	return code, found, lines, nil
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
// ledger, the work registry and each kind's smoke set, every problem with its
// rule id and, where one exists, a fix.
func configCheck(ledgerFile string, o Out) (int, error) {
	code, found, lines, err := configFindings(ledgerFile)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		if found == nil {
			found = []Found{}
		}
		err = out.Emit(o.Stdout,
			out.Field{Key: "config", Value: config.Path()},
			out.Field{Key: "valid", Value: len(found) == 0},
			out.Field{Key: "problems", Value: found})
	} else {
		for _, p := range found {
			fmt.Fprintf(o.Stderr, "FAIL %s\n", p.Message)
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
// config's ledger. It checks nothing, so a config that does not load gets the
// table's own values.
func printDefaults(o Out) (int, error) {
	var file *value.Map
	if cfg, err := config.Load(config.Path()); err == nil {
		file = cfg.File()
	}
	table := config.DefaultsFor(file)
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "defaults", Value: table})
	}
	_, err := fmt.Fprint(o.Stdout, value.YAML(table))
	return 0, err
}
