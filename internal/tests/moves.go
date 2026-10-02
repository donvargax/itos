package tests

// The moves rule (moves.ts), a built-in range check of a Gherkin kind:
// `{ name, builtin: moves, except_types, allowed_renames }` in
// tests.<kind>.range_checks. A commit of a type except_types does not name
// may move scenarios between feature files, create feature files and delete
// the ones left empty, provided every live scenario keeps its ID, name, tags
// and steps exactly, none is lost or added, and a file with a live scenario
// keeps its header and Background. A wip scenario may still be added, changed
// or removed: that is how a specification lands before its implementation. A
// rename the project allows is listed in allowed_renames, by ID (without its
// prefix) and new name. Comment lines (`#`) are not part of a scenario's ID,
// name, tags or steps, nor of a header, so they are dropped before anything
// is compared: a reason may be written beside a scenario in any commit.
//
// One judgement, Moves.Between, and its callers: the commit-msg hook judges
// HEAD against the index (Staged), verify each commit of its range against
// its parent (Commit), and `itos tests moves <kind>` HEAD against the index
// by hand (Index).

import (
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/git"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/value"
)

var (
	// scenarioLine is a Scenario line: its keyword with the spaces around it,
	// and its name. JavaScript's \s and dot, spelled out, since RE2's differ.
	scenarioLine = regexp.MustCompile(`^(` + value.Space + `*Scenario(?: Outline)?:` + value.Space + `*)` +
		`([^\n\r\x{2028}\x{2029}]*?)` + value.Space + `*$`)
	commentLine = regexp.MustCompile(`^` + value.Space + `*#`)
)

// withoutComments is a text without its comment lines, which the rule never
// compares.
func withoutComments(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if !commentLine.MatchString(line) {
			kept = append(kept, line)
		}
	}
	return strings.TrimRightFunc(strings.Join(kept, "\n"), isTrimmed)
}

// Scenario is one scenario of a feature set: its file, whether it is wip
// (itself or its file), and its block as written less its comment lines.
type Scenario struct {
	File string
	Wip  bool
	Body string
}

// FeatureSet is a tree's feature files read: each file's header (everything
// before its first scenario, the Background included) and every scenario by
// ID, each in the order first read, as JavaScript's Maps keep it.
type FeatureSet struct {
	Files     []string
	Headers   map[string]string
	IDs       []string
	Scenarios map[string]Scenario
}

// ReadFeatures reads feature files, path to text, into one set by the kind's
// ID pattern, tag prefix and wip tag, the files in git's order (by path). A
// scenario ID written twice keeps its first place and its last block.
func ReadFeatures(files map[string]string, o Options) (*FeatureSet, error) {
	set := &FeatureSet{Headers: map[string]string{}, Scenarios: map[string]Scenario{}}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, file := range paths {
		f, err := ParseFeature(files[file], o)
		if err != nil {
			return nil, err
		}
		set.Files = append(set.Files, file)
		set.Headers[file] = withoutComments(f.Header)
		for _, id := range f.IDs {
			if _, seen := set.Scenarios[id]; !seen {
				set.IDs = append(set.IDs, id)
			}
			block := f.Blocks[id]
			set.Scenarios[id] = Scenario{File: file, Wip: f.FileWip || block.Wip, Body: withoutComments(block.Body)}
		}
	}
	return set, nil
}

// renamedTo is whether after is before with its Scenario line's name changed
// to name and nothing else.
func renamedTo(before, after, name string) bool {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(a) != len(b) {
		return false
	}
	renamed := false
	for i := range a {
		if a[i] == b[i] {
			continue
		}
		from, to := scenarioLine.FindStringSubmatch(a[i]), scenarioLine.FindStringSubmatch(b[i])
		if renamed || from == nil || to == nil || from[1] != to[1] || to[2] != name {
			return false
		}
		renamed = true
	}
	return renamed
}

// hasLive is whether the set holds a live scenario in the file.
func (s *FeatureSet) hasLive(file string) bool {
	for _, id := range s.IDs {
		if sc := s.Scenarios[id]; sc.File == file && !sc.Wip {
			return true
		}
	}
	return false
}

// movedProblem is a scenario of the later set against its earlier self,
// if any: "" when it holds.
func movedProblem(id string, was *Scenario, now Scenario, renames map[string]string) string {
	if was == nil {
		if now.Wip {
			return ""
		}
		return "adds the live scenario " + id + " to " + now.File
	}
	if was.Body == now.Body || now.Wip {
		return ""
	}
	if allowed := renames[id]; allowed != "" && renamedTo(was.Body, now.Body, allowed) {
		return ""
	}
	return "changes the live scenario " + id + " in " + now.File +
		": a moved scenario keeps its ID, name, tags and steps exactly"
}

// MoveProblems is what breaks the rule between two sets, each problem a
// phrase for "a <type> commit …": the later set's scenarios added or changed,
// then the earlier set's lost, then the files whose header changed while
// they held a live scenario. renames are the allowed renames, by ID and new
// name.
func MoveProblems(before, after *FeatureSet, renames map[string]string) []string {
	var problems []string
	for _, id := range after.IDs {
		var was *Scenario
		if sc, ok := before.Scenarios[id]; ok {
			was = &sc
		}
		if p := movedProblem(id, was, after.Scenarios[id], renames); p != "" {
			problems = append(problems, p)
		}
	}
	for _, id := range before.IDs {
		if _, kept := after.Scenarios[id]; !kept && !before.Scenarios[id].Wip {
			problems = append(problems, "loses the scenario "+id+" of "+before.Scenarios[id].File)
		}
	}
	for _, file := range after.Files {
		was, ok := before.Headers[file]
		if !ok || was == after.Headers[file] || (!before.hasLive(file) && !after.hasLive(file)) {
			continue
		}
		problems = append(problems, "changes the header or Background of "+file)
	}
	return problems
}

// Moves is the moves rule over one config, each kind's feature files read
// once per tree (verify reads each commit as a child and as a parent).
type Moves struct {
	cfg  *config.Loaded
	sets map[string]*FeatureSet
}

// NewMoves is the moves rule of a config.
func NewMoves(cfg *config.Loaded) *Moves {
	return &Moves{cfg: cfg, sets: map[string]*FeatureSet{}}
}

// featureSet is the kind's feature files at a tree: a commit, or "index"
// for the staged tree. A tree that cannot be read (no HEAD yet) is empty.
func (m *Moves) featureSet(name, tree string) (*FeatureSet, error) {
	key := name + ":" + tree
	if set, ok := m.sets[key]; ok {
		return set, nil
	}
	k, err := KindOf(m.cfg, name)
	if err != nil {
		return nil, err
	}
	o, err := gherkinOptions(m.cfg, name, k)
	if err != nil {
		return nil, err
	}
	texts, err := featureTexts(tree, o.Root)
	if err != nil {
		return nil, err
	}
	set, err := ReadFeatures(texts, o)
	if err != nil {
		return nil, err
	}
	m.sets[key] = set
	return set, nil
}

// movesCheck is a kind's built-in moves check.
type movesCheck struct {
	kind  string
	check config.RangeCheck
}

// checks are each kind's built-in moves checks, in the config's order.
func (m *Moves) checks() []movesCheck {
	var found []movesCheck
	for _, name := range m.cfg.Tests.Keys {
		for _, c := range m.cfg.Tests.Values[name].RangeChecks {
			if c.Builtin != nil && *c.Builtin == "moves" {
				found = append(found, movesCheck{name, c})
			}
		}
	}
	return found
}

// judges is whether a check judges a commit of this type: one except_types
// does not name and, when the config lists the commit types, one of them, so
// a merge's or git's own revert's message ("Merge …", "Revert …") is the
// header lint's.
func (m *Moves) judges(c config.RangeCheck, typ string) bool {
	types := m.cfg.Commits.Types
	return !slices.Contains(c.ExceptTypes, typ) && (types == nil || slices.Contains(types, typ))
}

// Between is the moves checks' problems for a commit of a type, from the
// tree before to the tree after, each worded for "a <type> commit" under the
// check's name.
func (m *Moves) Between(typ, before, after string) ([]out.Problem, error) {
	found := []out.Problem{}
	for _, c := range m.checks() {
		if !m.judges(c.check, typ) {
			continue
		}
		was, err := m.featureSet(c.kind, before)
		if err != nil {
			return nil, err
		}
		now, err := m.featureSet(c.kind, after)
		if err != nil {
			return nil, err
		}
		for _, p := range MoveProblems(was, now, c.check.AllowedRenames) {
			found = append(found, out.Problem{Rule: c.check.Name, Message: "a " + typ + " commit " + p})
		}
	}
	return found, nil
}

// Staged is the commit-msg hook's moves rule: HEAD against the index.
func (m *Moves) Staged(typ string) ([]out.Problem, error) { return m.Between(typ, "HEAD", "index") }

// Commit is verify's moves rule for one commit: the commit against its
// parent, or the empty tree for a root commit. Nothing is read when no check
// judges the type.
func (m *Moves) Commit(sha, typ string) ([]out.Problem, error) {
	judged := false
	for _, c := range m.checks() {
		judged = judged || m.judges(c.check, typ)
	}
	if !judged {
		return []out.Problem{}, nil
	}
	return m.Between(typ, git.Parent(sha), sha)
}

// Index is `itos tests moves <kind>`'s judgement: HEAD against the index, by
// the kind's moves checks' allowed renames (none when it has no moves check),
// each problem under the rule moves, worded for "the index". A config error
// when the kind is missing or its adapter is not gherkin.
func (m *Moves) Index(name string) ([]out.Problem, error) {
	k, err := KindOf(m.cfg, name)
	if err != nil {
		return nil, err
	}
	if k.Adapter.Command != "" || k.Adapter.Name != "gherkin" {
		return nil, &config.Error{File: config.Path(), Problems: []out.Problem{{
			Rule:    "moves-not-gherkin",
			Message: "tests." + name + ".adapter is not gherkin, and the moves rule reads feature files",
			Fix:     "name a kind whose adapter is gherkin",
		}}}
	}
	renames := map[string]string{}
	for _, c := range m.checks() {
		if c.kind == name {
			for id, to := range c.check.AllowedRenames {
				renames[id] = to
			}
		}
	}
	was, err := m.featureSet(name, "HEAD")
	if err != nil {
		return nil, err
	}
	now, err := m.featureSet(name, "index")
	if err != nil {
		return nil, err
	}
	found := []out.Problem{}
	for _, p := range MoveProblems(was, now, renames) {
		found = append(found, out.Problem{Rule: "moves", Message: "the index " + p})
	}
	return found, nil
}

// RangeCommands are each kind's range commands (tests.<kind>.range_checks[]
// .range), their {from} and {to} filled in, each one shell word; {from}
// starts after commits.since.
func RangeCommands(cfg *config.Loaded, from, to string) []string {
	var commands []string
	for _, name := range cfg.Tests.Keys {
		for _, c := range cfg.Tests.Values[name].RangeChecks {
			if c.Range == nil || *c.Range == "" {
				continue
			}
			command := strings.ReplaceAll(*c.Range, "{from}", ShellWord(cfg.RangeStart(from)))
			commands = append(commands, strings.ReplaceAll(command, "{to}", ShellWord(to)))
		}
	}
	return commands
}
