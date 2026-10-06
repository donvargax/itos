package tests

// The moves rule on a kind whose adapter is a command (slice 82, issue #9).
// The adapter lists no test's body, so the rule compares what it lists at
// both ends of a commit: outside the types except_types names, a commit may
// not add or lose a live test, nor switch one between live and wip, and when
// the adapter gives a live test a title at both ends, the title stays, unless
// allowed_renames lists the new one for its ID. A test's file, its body and a
// wip test are free. config check refuses the rule unless the adapter sets
// supports_at: true, since the rule lists the commit's parent.

import (
	"path"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
)

// testList is the kind's listing at a tree: "index", or a commit. A tree
// that names no commit (the empty tree before a root commit, HEAD before the
// first commit) lists nothing.
func (m *Moves) testList(name, tree string) (List, error) {
	key := name + ":" + tree
	if list, ok := m.lists[key]; ok {
		return list, nil
	}
	at := tree
	if tree != "index" {
		sha, err := git.Output("rev-parse", "--verify", "--quiet", tree+"^{commit}")
		if at = strings.TrimSpace(sha); err != nil || at == "" {
			m.lists[key] = List{Protocol: 1, Tests: []Test{}, Files: []string{}}
			return m.lists[key], nil
		}
	}
	list, err := ListTests(m.cfg, name, at)
	if err != nil {
		return List{}, err
	}
	m.lists[key] = list
	return list, nil
}

// listedProblems is a command kind's check from before to after, or for a
// merge (own not nil) from after with the tests of its own paths as before
// lists them.
func (m *Moves) listedProblems(c movesCheck, k config.Kind, before, after string, own []string) ([]string, error) {
	was, err := m.testList(c.kind, before)
	if err != nil {
		return nil, err
	}
	now, err := m.testList(c.kind, after)
	if err != nil {
		return nil, err
	}
	if own != nil {
		was = ownList(k, was, now, own)
	}
	return ListedProblems(was, now, k.TagPrefix, c.check.AllowedRenames), nil
}

// ownList is a merge's listing before its own changes: after's tests, with
// those in a file among its own paths as before lists them instead.
func ownList(k config.Kind, before, after List, own []string) List {
	mine := map[string]bool{}
	for _, p := range own {
		mine[p] = true
	}
	inOwn := func(t Test) bool {
		if k.Root == nil {
			return mine[t.File]
		}
		return mine[path.Join(*k.Root, t.File)]
	}
	list := List{Protocol: 1, Tests: []Test{}, Files: after.Files}
	for _, t := range after.Tests {
		if !inOwn(t) {
			list.Tests = append(list.Tests, t)
		}
	}
	for _, t := range before.Tests {
		if inOwn(t) {
			list.Tests = append(list.Tests, t)
		}
	}
	return list
}

// listed is a listing's tests by ID, the IDs in the order first listed; an
// ID listed twice keeps its first place and its last entry.
func listed(list List) ([]string, map[string]Test) {
	var ids []string
	byID := map[string]Test{}
	for _, t := range list.Tests {
		if _, seen := byID[t.ID]; !seen {
			ids = append(ids, t.ID)
		}
		byID[t.ID] = t
	}
	return ids, byID
}

// ListedProblems is what breaks the rule between two listings of a command
// kind, each problem a phrase for "a <type> commit …", a test named by its
// tag (the kind's tag prefix and its ID): the later listing's tests added,
// switched or retitled, then the earlier one's lost. renames are the allowed
// renames, by ID and new title.
func ListedProblems(before, after List, prefix string, renames map[string]string) []string {
	var problems []string
	wasIDs, was := listed(before)
	nowIDs, now := listed(after)
	for _, id := range nowIDs {
		t, tag := now[id], prefix+id
		old, kept := was[id]
		switch {
		case !kept:
			if t.Live {
				problems = append(problems, "adds the live test "+tag+" to "+t.File)
			}
		case old.Live && !t.Live:
			problems = append(problems, "turns the live test "+tag+" of "+t.File+" wip")
		case !old.Live && t.Live:
			problems = append(problems, "makes the wip test "+tag+" of "+t.File+" live")
		case t.Live && old.Title != nil && t.Title != nil && *old.Title != *t.Title && renames[id] != *t.Title:
			problems = append(problems, "retitles the live test "+tag+" of "+t.File+" from "+
				strconv.Quote(*old.Title)+" to "+strconv.Quote(*t.Title)+": a live test keeps its title")
		}
	}
	for _, id := range wasIDs {
		if _, kept := now[id]; !kept && was[id].Live {
			problems = append(problems, "loses the live test "+prefix+id+" of "+was[id].File)
		}
	}
	return problems
}
