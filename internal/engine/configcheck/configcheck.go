package configcheck

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/donvargax/itos/v8/internal/adr"
	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// Found is a problem config check reports, with the area it is in: the
// config, the ledger, the registry, a smoke set, the decision records or, as
// a warning, the ledger or the people.
type Found struct {
	out.Problem
	Area string `json:"area"`
}

// Tagged is each problem in the area given.
func Tagged(area string, problems []out.Problem) []Found {
	found := make([]Found, len(problems))
	for i, p := range problems {
		found[i] = Found{p, area}
	}
	return found
}

// SmokeFindings are each kind's smoke set against its tests, and the line
// each sound one prints; a set that cannot be read is one problem.
func SmokeFindings(cfg *config.Loaded, ws *tests.Warnings) ([]Found, []string) {
	return JudgedSmokeFindings(cfg, nil, ws)
}

// JudgedSmokeFindings are SmokeFindings for the kinds judged picks by name
// and smoke file (the commit-msg hook's, slice 117), or for every kind with a
// smoke set when judged is nil.
func JudgedSmokeFindings(cfg *config.Loaded, judged func(name, file string) bool, ws *tests.Warnings) ([]Found, []string) {
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
		found = append(found, Tagged("smoke", own)...)
		count := len(tests.SmokeIDs(k, smoke))
		if k.Smoke.EveryFile {
			lines = append(lines, fmt.Sprintf("%s: every file has a smoke test (%d in all)", file, count))
		} else {
			lines = append(lines, fmt.Sprintf("%s: every smoke test is live (%d in all)", file, count))
		}
	}
	return found, lines
}

// Findings is what config check finds, what it warns of, and the
// lines it prints when it finds nothing (config-check.ts's configFindings).
// Its code is 2 when the config is invalid or the ledger's folder is
// missing, since nothing else can be read, else 1 for any problem; a warning
// (a check static by a ci.cost.static pattern that written order runs late,
// ledger.Findings, a project's people file missing or unreadable,
// work.PeopleProblem, or an answered question naming an item the registry
// does not have, asksProblems) never sets it. The config and its data are
// read from src, the config located and loaded through v and trees.
// taskFlags are the flags of itos task, its subcommands' and the global
// ones, that name no group, which ledger.group.label may not be.
func Findings(ctx context.Context, v vcsport.VCS, trees port.Trees, src port.Source, ledgerFile string, taskFlags []string, ws *tests.Warnings) (int, []Found, []Found, []string, error) {
	path := config.Path(v, src)
	cfg, err := config.Load(v, trees, src, path)
	if err != nil {
		return failed(err, "config")
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
		if err != nil {
			return failed(err, "ledger")
		}
		for _, f := range listed {
			files = append(files, f.Path)
		}
	}
	smoke, smokeLines := SmokeFindings(cfg, ws)
	ledgerProblems, ledgerWarnings := ledger.Findings(cfg, files)
	// Values the config loads with that nothing reads as written (slice 129),
	// and the ledger layout's.
	unusable := append(cfg.Unusable(), groupLabelProblems(cfg, taskFlags)...)
	found := prefixed(&config.Error{File: path, Problems: append(unusable, layout...)}, "config")
	found = append(found, Tagged("ledger", ledgerProblems)...)
	registry, err := work.Problems(cfg)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	warnings := Tagged("ledger", ledgerWarnings)
	if p := work.PeopleProblem(cfg); p != nil {
		warnings = append(warnings, Found{*p, "people"})
	}
	found = append(found, Tagged("registry", registry)...)
	asks, stale, err := asksProblems(ctx, cfg)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	found = append(found, Tagged("asks", asks)...)
	warnings = append(warnings, Tagged("asks", stale)...)
	found = append(found, smoke...)
	// The decision records in the folder work.decisions names (slice 74);
	// a folder that is not there holds none to check.
	records, err := adr.List(cfg.Source, cfg.Work.Decisions)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	found = append(found, Tagged("decisions", adr.Problems(cfg.Work.Decisions, records))...)
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
// renamed or dropped, breaks nothing a command does. Both say the repair, in
// the message, which config check's text prints alone, and in the fix: no
// command re-points a question, so the asks file is edited by hand (bug 135).
// None when the file is not there; a registry that cannot be read is
// work.Problems' to report.
func asksProblems(ctx context.Context, cfg *config.Loaded) (problems, warnings []out.Problem, err error) {
	file := cfg.Work.Asks
	if !cfg.Source.Has(ctx, file) {
		return nil, nil, nil
	}
	text, err := cfg.Source.Read(ctx, file)
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
		repair := fmt.Sprintf("by hand, set %s's item in %s to %s's new id, or remove %s's item key", q.ID, file, q.Item, q.ID)
		if q.Status() == ask.Answered {
			warnings = append(warnings, out.Problem{
				Rule:    "asks-answered-unknown-item",
				Message: fmt.Sprintf("%s: %s, answered, names the item %s, which %s does not have; %s", file, q.ID, q.Item, cfg.Work.Registry, repair),
				Fix:     repair,
			})
			continue
		}
		problems = append(problems, out.Problem{
			Rule:    "asks-unknown-item",
			Message: fmt.Sprintf("%s: %s names the item %s, which %s does not have; %s", file, q.ID, q.Item, cfg.Work.Registry, repair),
			Fix:     repair,
		})
	}
	return problems, warnings, nil
}

// groupLabelProblems: itos task takes a group by --<ledger.group.label> as
// by --group, so a label that cannot be typed as one flag (empty, a space, an
// = or a leading -), or that is another of taskFlags (skip, pending, a
// global one such as json), never takes a group: --skip T-005 selected
// nothing.
func groupLabelProblems(cfg *config.Loaded, taskFlags []string) []out.Problem {
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
	case slices.Contains(taskFlags, flag):
		return []out.Problem{{
			Rule:    "config-group-label",
			Message: "ledger.group.label is " + label + ", and " + flag + " is already another flag of itos task, so it never takes a group",
			Fix:     "set ledger.group.label to a word that is no other flag of itos task, such as phase or milestone",
		}}
	}
	return nil
}

// failed is what Findings gives for an error: a config error's problems in
// the area given, code 2, since nothing past it can be read; else the error.
func failed(err error, area string) (int, []Found, []Found, []string, error) {
	var invalid *config.Error
	if errors.As(err, &invalid) {
		return 2, prefixed(invalid, area), nil, nil, nil
	}
	return 0, nil, nil, nil, err
}

// prefixed are a config error's problems, each message after the file's
// name, in the area given.
func prefixed(e *config.Error, area string) []Found {
	found := Tagged(area, e.Problems)
	for i := range found {
		found[i].Message = e.File + ": " + found[i].Message
	}
	return found
}
