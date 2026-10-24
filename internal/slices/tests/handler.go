package tests

import (
	"context"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ids"
	"github.com/donvargax/itos/v8/internal/engine/refs"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/shell"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	namedtests "github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what the tests command asks: Sub, list, moves, next-id or smoke,
// with smoke's Action, check, ids or run; the Kind; list's tree At, "" when
// --at is not given, and TSV, its live tests as rows (--tsv); smoke check's
// Features, the folder standing in for a Gherkin kind's root, nil without
// --features; smoke run's Runner arguments; next-id's Stem and Count,
// Counted when --count gave it.
type Query struct {
	Sub, Action string
	Kind        string
	At          string
	TSV         bool
	Features    *string
	Runner      []string
	Stem        string
	Count       int
	Counted     bool
}

// Ports are what the tests command reaches the world through, which
// internal/cli binds: the VCS, which list's --at commit is checked in;
// Config, the config as the working tree has it; Tracker, the tracker
// store of cfg, which next-id claims an area's ids through; Shell, a
// command run through cfg's shell with itos's streams, as smoke run runs
// its runner.
type Ports interface {
	vcsport.VCS
	Config(ctx context.Context) (*config.Loaded, error)
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
	Shell(ctx context.Context, cfg *config.Loaded, command string) shell.Result
}

// Handler is the tests command's application, over its ports, its
// listings warning on warnings (cli.Out's Tests).
type Handler struct {
	ports    Ports
	warnings *namedtests.Warnings
}

// NewHandler is the handler over p, warning on ws.
func NewHandler(p Ports, ws *namedtests.Warnings) Handler {
	return Handler{ports: p, warnings: ws}
}

// Answer is what Handle found or did: list's List, and under TSV its Rows
// or the Refused rows that keep it from having any; the moves rule's or the
// smoke rule's Problems, with the smoke set's Count of IDs and whether its
// rule is EveryFile; smoke ids' and next-id's IDs, Claimed when next-id
// claimed them; smoke run's Empty, a smoke set with nothing to run, or the
// runner's Code.
type Answer struct {
	List      namedtests.List
	Rows      []namedtests.Row
	Refused   []string
	Problems  []out.Problem
	Count     int
	EveryFile bool
	IDs       []string
	Claimed   bool
	Empty     bool
	Code      int
}

// Handle answers q.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	switch q.Sub {
	case "list":
		return h.list(ctx, q)
	case "moves":
		return h.moves(ctx, q)
	case "next-id":
		return h.nextID(ctx, q)
	}
	switch q.Action {
	case "check":
		return h.smokeCheck(ctx, q)
	case "ids":
		return h.smokeIDs(ctx, q)
	}
	return h.smokeRun(ctx, q)
}

// list is `tests list <kind> [--at <tree>] [--tsv]` (tests-command.ts): the
// kind's adapter's listing at the working tree, the index or a commit, a
// commit checked to be one before the config is read; under TSV its live
// tests as rows from the project's top, the folder itos runs in.
func (h Handler) list(ctx context.Context, q Query) (Answer, error) {
	at := q.At
	if at != "worktree" && at != "index" {
		if err := refs.Check(ctx, h.ports, "tests list", at); err != nil {
			return Answer{}, err
		}
	}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	if at == "" {
		at = "worktree"
	}
	k, err := namedtests.KindOf(cfg, q.Kind)
	if err != nil {
		return Answer{}, err
	}
	list, err := namedtests.ListTests(cfg, q.Kind, at, h.warnings)
	if err != nil || !q.TSV {
		return Answer{List: list}, err
	}
	root := ""
	if k.Root != nil {
		root = *k.Root
	}
	rows, refused := namedtests.TSV(list, root, h.ports.Folder(ctx))
	return Answer{List: list, Rows: rows, Refused: refused}, nil
}

// moves is `tests moves <kind>` (moves.ts's testsMoves): the staged
// feature files against HEAD's by the built-in moves rule, by the kind's
// moves checks' allowed renames.
func (h Handler) moves(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	found, err := namedtests.NewMoves(cfg, h.warnings).Index(q.Kind)
	return Answer{Problems: found}, err
}

// smokeSet is the config and the kind's smoke set in the working tree.
func (h Handler) smokeSet(ctx context.Context, name string) (*config.Loaded, config.Kind, []namedtests.SmokeFile, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return nil, config.Kind{}, nil, err
	}
	smoke, err := namedtests.LoadSmoke(cfg, name)
	if err != nil {
		return nil, config.Kind{}, nil, err
	}
	k, err := namedtests.KindOf(cfg, name)
	return cfg, k, smoke, err
}

// smokeCheck is `tests smoke check <kind> [--features <dir>]`
// (smoke-rule.ts's smokeCheck): the smoke rule over the working tree, or
// over a copy of a Gherkin kind's files in <dir>.
func (h Handler) smokeCheck(ctx context.Context, q Query) (Answer, error) {
	cfg, k, smoke, err := h.smokeSet(ctx, q.Kind)
	if err != nil {
		return Answer{}, err
	}
	found, err := namedtests.SmokeIssues(cfg, q.Kind, smoke, q.Features, h.warnings)
	if err != nil {
		return Answer{}, err
	}
	if found == nil {
		found = []out.Problem{}
	}
	return Answer{Problems: found, Count: len(namedtests.SmokeIDs(k, smoke)), EveryFile: k.Smoke.EveryFile}, nil
}

// smokeIDs is `tests smoke ids <kind>`: the smoke set's IDs without the tag
// prefix.
func (h Handler) smokeIDs(ctx context.Context, q Query) (Answer, error) {
	_, k, smoke, err := h.smokeSet(ctx, q.Kind)
	if err != nil {
		return Answer{}, err
	}
	return Answer{IDs: namedtests.SmokeIDs(k, smoke)}, nil
}

// smokeRun is `tests smoke run <kind> [--] [<runner args>…]`: the kind's run
// of exactly the smoke set (its select template over the smoke IDs), the
// arguments each one shell word after it, through the Shell port. Its code
// is the runner's, 1 when the runner did not exit by itself, as spawnSync's
// `status ?? 1`. An empty smoke set runs nothing.
func (h Handler) smokeRun(ctx context.Context, q Query) (Answer, error) {
	cfg, k, smoke, err := h.smokeSet(ctx, q.Kind)
	if err != nil {
		return Answer{}, err
	}
	run, ok, err := namedtests.CommandFor(cfg, q.Kind, []namedtests.Selection{namedtests.IDs(namedtests.SmokeIDs(k, smoke)...)})
	if err != nil {
		return Answer{}, err
	}
	if !ok {
		return Answer{Empty: true}, nil
	}
	words := []string{run}
	for _, a := range q.Runner {
		words = append(words, namedtests.ShellWord(a))
	}
	result := h.ports.Shell(ctx, cfg, strings.Join(words, " "))
	if result.Code < 0 || result.Err != nil {
		return Answer{Code: refused}, nil
	}
	return Answer{Code: result.Code}, nil
}

// nextID is `tests next-id <kind> <stem> [--count <n>]`: the kind's next
// free tag of the stem (namedtests.NextTag), counting the registry's ids
// too. Of an area it claims the tag, or Count tags in a row, through the
// ids engine and the tracker; of any other stem it only reads, so --count
// there is a usage error, as is a kind the config does not have. A bare stem
// that is a mistaken area (namedtests.Mistaken, slice 157: SEC for ID-SEC)
// is a usage error naming the command that claims, before the counter is
// touched.
func (h Handler) nextID(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	k, ok := cfg.Tests.Get(q.Kind)
	if !ok {
		return Answer{}, out.Usage("tests next-id: the config has no tests kind %s (it has %s)", q.Kind, kindNames(cfg))
	}
	area := namedtests.Area(k, q.Stem)
	if q.Counted && !area && !namedtests.Candidate(k, q.Stem) {
		return Answer{}, countRefused(q)
	}
	others, err := ids.Registry(ctx, cfg)
	if err != nil {
		return Answer{}, err
	}
	held, err := namedtests.HeldIDs(cfg, q.Kind, k, others)
	if err != nil {
		return Answer{}, err
	}
	if fixed, mistaken := namedtests.Mistaken(k, q.Stem, held); mistaken {
		command := "itos tests next-id " + q.Kind + " " + fixed
		if q.Counted {
			command += " --count " + strconv.Itoa(q.Count)
		}
		return Answer{}, out.Usage("tests next-id: %s is no area of tests.%s's id %s, and %s is: to claim its ids, run %s", q.Stem, q.Kind, *k.ID, fixed, command)
	}
	if q.Counted && !area {
		return Answer{}, countRefused(q)
	}
	next := namedtests.NextTagFrom(k, q.Stem, held)
	tags := []string{next}
	if area {
		if tags, err = ids.Claim(ctx, h.ports.Tracker(ctx, cfg), q.Stem, next, q.Count); err != nil {
			return Answer{}, err
		}
	}
	return Answer{IDs: tags, Claimed: area}, nil
}

// countRefused is the usage error of a --count for a stem that only reads.
func countRefused(q Query) error {
	return out.Usage("tests next-id --count claims an area's ids, and %s is no area of tests.%s's id: its next id is only read", q.Stem, q.Kind)
}

// kindNames are the config's tests kinds, as a usage error lists them.
func kindNames(cfg *config.Loaded) string {
	if len(cfg.Tests.Keys) == 0 {
		return "none"
	}
	return strings.Join(cfg.Tests.Keys, ", ")
}
