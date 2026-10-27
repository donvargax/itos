package message

import (
	"context"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// symrefCounter is a VCS that counts the times it is asked which branch the
// remote's HEAD names, which main's ledger is found by.
type symrefCounter struct {
	*vcstest.VCS
	asked int
}

func (c *symrefCounter) Symref(ctx context.Context, ref string) (string, bool) {
	c.asked++
	return c.VCS.Symref(ctx, ref)
}

func ledgerOf(ids ...string) map[string]string {
	text := ""
	for _, id := range ids {
		text += "- id: " + id + "\n"
	}
	return map[string]string{"tasks/phase-1.yaml": text}
}

// A ledger footer read at the commit may name a task only the remote's main
// holds, under pr, for a commit not on that main; everything else is judged
// by the commit's own ledger.
func TestLedgerFooterNamesATaskOfTheRemotesMain(t *testing.T) {
	const config = "version: 1\ncommits:\n  footers:\n    Task: { source: ledger, read_at: commit, validate_for: [chore] }\n" +
		"ledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n"
	const pr = "push: { mode: pr }\n"
	fake := func() *vcstest.VCS {
		return &vcstest.VCS{
			OnBranch: "item",
			History: []vcstest.Commit{
				{Name: "c-main"},
				{Name: "c-tip", Parents: []string{"c-main"}},
				{Name: "c-item", Parents: []string{"c-main"}},
				{Name: "c-up"},
			},
			Named: map[string]string{
				"HEAD":                       "c-item",
				"refs/remotes/origin/main":   "c-tip",
				"refs/remotes/upstream/main": "c-up",
			},
			Symbolic: map[string]string{
				"refs/remotes/origin/HEAD":   "refs/remotes/origin/main",
				"refs/remotes/upstream/HEAD": "refs/remotes/upstream/main",
			},
		}
	}
	trees := func() porttest.Trees {
		return porttest.Trees{Common: ".git", Files: map[string]map[string]string{
			"index":                      ledgerOf("T-1"),
			"c-item":                     ledgerOf("T-1"),
			"c-main":                     ledgerOf("T-1"),
			"refs/remotes/origin/main":   ledgerOf("T-1", "T-2"),
			"refs/remotes/upstream/main": ledgerOf("T-1", "T-4"),
		}}
	}
	for _, c := range []struct {
		name   string
		config string
		edit   func(*vcstest.VCS, *porttest.Trees)
		at     string
		ids    string
		want   string
	}{
		{"main's task is known", config + pr, nil, "", "T-2", ""},
		{"the commit's own is known", config + pr, nil, "", "T-1", ""},
		{"neither has it", config + pr, nil, "", "T-3", "unknown tasks: T-3"},
		{"one of two", config + pr, nil, "", "T-1, T-2, T-3", "unknown tasks: T-3"},
		{"a commit not on main", config + pr, nil, "c-item", "T-2", ""},
		{"a commit on main", config + pr, nil, "c-main", "T-2", "unknown tasks: T-2"},
		{"trunk", config + "push: { mode: trunk }\n", nil, "", "T-2", "unknown tasks: T-2"},
		{"no push section", config, nil, "", "T-2", "unknown tasks: T-2"},
		{"no remote HEAD, and no fallback to origin/main", config + pr, func(v *vcstest.VCS, _ *porttest.Trees) {
			v.Symbolic = nil
		}, "", "T-2", "unknown tasks: T-2"},
		{"a HEAD naming a branch of another remote", config + pr, func(v *vcstest.VCS, _ *porttest.Trees) {
			v.Symbolic["refs/remotes/origin/HEAD"] = "refs/remotes/upstream/main"
		}, "", "T-4", "unknown tasks: T-4"},
		{"a HEAD naming a branch the clone lacks", config + pr, func(v *vcstest.VCS, _ *porttest.Trees) {
			delete(v.Named, "refs/remotes/origin/main")
		}, "", "T-2", "unknown tasks: T-2"},
		{"a main without a ledger", config + pr, func(_ *vcstest.VCS, tr *porttest.Trees) {
			delete(tr.Files, "refs/remotes/origin/main")
		}, "", "T-2", "unknown tasks: T-2"},
		{"the upstream's remote", config + pr, func(v *vcstest.VCS, _ *porttest.Trees) {
			v.Tracking = map[string][2]string{"item": {"upstream", "refs/heads/item"}}
		}, "", "T-4", ""},
		{"not the origin's when the upstream is another remote", config + pr, func(v *vcstest.VCS, _ *porttest.Trees) {
			v.Tracking = map[string][2]string{"item": {"upstream", "refs/heads/item"}}
		}, "", "T-2", "unknown tasks: T-2"},
		{"a detached HEAD asks origin", config + pr, func(v *vcstest.VCS, _ *porttest.Trees) {
			v.OnBranch = ""
			v.Tracking = map[string][2]string{"": {"upstream", "refs/heads/item"}}
		}, "", "T-2", ""},
	} {
		v, tr := fake(), trees()
		if c.edit != nil {
			c.edit(v, &tr)
		}
		cfg := load(t, c.config)
		cfg.VCS, cfg.Trees = v, tr
		got, err := CheckFooter(cfg, "Task", "chore", "chore: x\n\nTask: "+c.ids+"\n", Reading{At: c.at})
		if err != nil || got != c.want {
			t.Errorf("%s: %q, %v; want %q", c.name, got, err, c.want)
		}
	}

	// A footer read at the working tree, or under the stealth config, whose
	// ledger is no commit's, is judged as ever.
	for _, c := range []struct {
		name    string
		config  string
		stealth bool
	}{
		{"read at the working tree", "version: 1\ncommits:\n  footers:\n    Task: { source: ledger, validate_for: [chore] }\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n" + pr, false},
		{"stealth", config + pr, true},
	} {
		cfg := load(t, c.config)
		cfg.VCS, cfg.Trees, cfg.Stealth = fake(), trees(), c.stealth
		source, err := porttest.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": ledgerOf("T-1")}}.At(t.Context(), "worktree")
		if err != nil {
			t.Fatal(err)
		}
		cfg.Source = source
		got, err := CheckFooter(cfg, "Task", "chore", "chore: x\n\nTask: T-2\n", Reading{})
		if err != nil || got != "unknown tasks: T-2" {
			t.Errorf("%s: %q, %v; want T-2 unknown", c.name, got, err)
		}
	}
}

// Main's ledger is read once for a footer, however many of its tasks the
// commit's own ledger lacks.
func TestMainsLedgerIsReadOnceForAFooterNamingSeveralTasks(t *testing.T) {
	cfg := load(t, "version: 1\ncommits:\n  footers:\n    Task: { source: ledger, read_at: commit, validate_for: [chore] }\n"+
		"ledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\npush: { mode: pr }\n")
	v := &symrefCounter{VCS: &vcstest.VCS{
		OnBranch: "item",
		History:  []vcstest.Commit{{Name: "c-item"}},
		Named:    map[string]string{"HEAD": "c-item", "refs/remotes/origin/main": "c-tip"},
		Symbolic: map[string]string{"refs/remotes/origin/HEAD": "refs/remotes/origin/main"},
	}}
	cfg.VCS, cfg.Trees = v, porttest.Trees{Common: ".git", Files: map[string]map[string]string{
		"index":                    ledgerOf("T-1"),
		"refs/remotes/origin/main": ledgerOf("T-1", "T-2", "T-3", "T-4"),
	}}
	got, err := CheckFooter(cfg, "Task", "chore", "chore: x\n\nTask: T-2, T-3, T-4, T-5\n", Reading{})
	if err != nil || got != "unknown tasks: T-5" {
		t.Fatalf("%q, %v", got, err)
	}
	if v.asked != 1 {
		t.Errorf("main's HEAD was asked for %d times, want once", v.asked)
	}
}
