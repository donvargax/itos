package push

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// registryEverywhere holds the same registry in every tree, whichever commit
// the title is read at (the push reads it at the head it sends).
type registryEverywhere struct{ treefake.Trees }

func (t registryEverywhere) At(ctx context.Context, _ string) (port.Source, error) {
	return t.Trees.At(ctx, "worktree")
}

// titled is f under the config, whose registry gives the item the title, a
// YAML text ("" for none), at every commit.
func titled(t *testing.T, f *fake, configText, title string) {
	t.Helper()
	registry := "items:\n  - id: " + item + "\n"
	if title != "" {
		registry += "    title: " + title + "\n"
	}
	trees := registryEverywhere{treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": {registryFile: registry}}}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	f.cfg, err = config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml", configText, false)
	if err != nil {
		t.Fatal(err)
	}
}

// describe titles the pull request "<item>: <title>" and describes it by the
// title whole, then a "Closes" line for each issue of the list that ends the
// title, once each; an item with no title that is text is titled by its id.
func TestDescribe(t *testing.T) {
	for name, c := range map[string]struct{ title, want, body string }{
		"no issue":             {`"Add the widget"`, "slice-9: Add the widget", "Add the widget"},
		"one issue":            {`"Add the widget (#12)"`, "slice-9: Add the widget (#12)", "Add the widget (#12)\n\nCloses #12"},
		"several, in order":    {`"Watch (#15, #12, #20)"`, "slice-9: Watch (#15, #12, #20)", "Watch (#15, #12, #20)\n\nCloses #15\nCloses #12\nCloses #20"},
		"each once":            {`"Watch (#12, #15, #12)"`, "slice-9: Watch (#12, #15, #12)", "Watch (#12, #15, #12)\n\nCloses #12\nCloses #15"},
		"a mention":            {`"Follow #9 into (the docs)"`, "slice-9: Follow #9 into (the docs)", "Follow #9 into (the docs)"},
		"a mention, then one":  {`"Follow #9 (#12)"`, "slice-9: Follow #9 (#12)", "Follow #9 (#12)\n\nCloses #12"},
		"another repository's": {`"Follow (itos-cc#27)"`, "slice-9: Follow (itos-cc#27)", "Follow (itos-cc#27)"},
		"one of them":          {`"Follow (#12, itos-cc#27)"`, "slice-9: Follow (#12, itos-cc#27)", "Follow (#12, itos-cc#27)"},
		"without a space":      {`"Follow (#12,#15)"`, "slice-9: Follow (#12,#15)", "Follow (#12,#15)"},
		"text after":           {`"Follow (#12) later"`, "slice-9: Follow (#12) later", "Follow (#12) later"},
		"not the end":          {`"Follow (#12)) "`, "slice-9: Follow (#12))", "Follow (#12))"},
		"space collapsed":      {`"  Add\n  the\twidget\n(#12)\n"`, "slice-9: Add the widget (#12)", "Add the widget (#12)\n\nCloses #12"},
		"no title":             {``, "slice-9", "slice-9"},
		"empty":                {`""`, "slice-9", "slice-9"},
		"only space":           {`" \n "`, "slice-9", "slice-9"},
		"no text":              {`7`, "slice-9", "slice-9"},
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			titled(t, f, "version: 1\npush: { mode: pr }\n", c.title)
			title, body := pushRun{ports: f, cfg: f.cfg, sha: "abc"}.describe(item)
			if title != c.want || body != c.body {
				t.Errorf("got %q and %q, want %q and %q", title, body, c.want, c.body)
			}
		})
	}

	// GitHub keeps 256 characters of a title, counted in code points: one
	// longer is cut to its first 255 and an ellipsis, and the description
	// keeps the whole title and the issue its list ends with.
	for _, whole := range []int{255, 256, 257, 300} {
		t.Run(fmt.Sprintf("%d characters", whole), func(t *testing.T) {
			f := prRepo(t)
			name := strings.Repeat("\u00e9", whole-len([]rune(item+": "+" (#12)"))) + " (#12)"
			titled(t, f, "version: 1\npush: { mode: pr }\n", `"`+name+`"`)
			title, body := pushRun{ports: f, cfg: f.cfg, sha: "abc"}.describe(item)
			want := item + ": " + name
			if whole > titleLimit {
				want = string([]rune(want)[:titleLimit-1]) + "\u2026"
			}
			if title != want || len([]rune(title)) != min(whole, titleLimit) || body != name+"\n\nCloses #12" {
				t.Errorf("the title is %d characters, %q; the description %q", len([]rune(title)), title, body)
			}
		})
	}
}

// landing is --rebase unless push.pr.update is merge, and then --merge, the
// pull request's title as the subject, each value joined to its flag, and the
// registry footer as the body, none without one; ghFailed's advice names the
// same method.
func TestLanding(t *testing.T) {
	const footer = "commits:\n  footers:\n    Item: { source: registry }\n"
	for name, c := range map[string]struct {
		config string
		want   []string
	}{
		"the default":               {"version: 1\npush: { mode: pr }\n" + footer, []string{"--rebase"}},
		"rebase":                    {"version: 1\npush: { mode: pr, pr: { update: rebase } }\n" + footer, []string{"--rebase"}},
		"merge":                     {"version: 1\npush: { mode: pr, pr: { update: merge } }\n" + footer, []string{"--merge", "--subject=slice-9: -b the widget", "--body=Item: slice-9"}},
		"merge, no registry footer": {"version: 1\npush: { mode: pr, pr: { update: merge } }\n", []string{"--merge", "--subject=slice-9: -b the widget"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			titled(t, f, c.config, `"-b the widget"`)
			r := pushRun{ports: f, cfg: f.cfg, sha: "abc", branch: item}
			if got := r.landing(); !reflect.DeepEqual(got, c.want) {
				t.Errorf("landing is %q, want %q", got, c.want)
			}
			advice := r.ghFailed("gh pr merge", "no").Lines
			if want := "(gh pr merge --auto " + c.want[0] + ")."; !strings.HasSuffix(advice[len(advice)-1], want) {
				t.Errorf("the advice is %q, want it to end %q", advice, want)
			}
		})
	}
}

// cut keeps a title of up to 256 code points as it is, else its first 255
// and an ellipsis, counting code points and not bytes.
func TestCut(t *testing.T) {
	for _, c := range []struct {
		runes int
		cut   bool
	}{{1, false}, {255, false}, {256, false}, {257, true}, {1000, true}} {
		title := strings.Repeat("\U0001F642", c.runes)
		got := cut(title)
		if !c.cut && got != title {
			t.Errorf("%d characters: cut to %d", c.runes, len([]rune(got)))
		}
		if c.cut && got != strings.Repeat("\U0001F642", 255)+"…" {
			t.Errorf("%d characters: cut to %d characters, %q", c.runes, len([]rune(got)), got[len(got)-8:])
		}
	}
}

// The first push opens the pull request with the title and the description
// as two arguments of their own, --title=<title> and --body=<body>, in that
// order and before --draft, a value beginning with "-" or holding what a shell
// reads reaching gh as written.
func TestPushOpensThePullRequestTitledAfterTheItem(t *testing.T) {
	for name, c := range map[string]struct {
		config string
		last   []string
	}{
		"ready": {"version: 1\npush: { mode: pr }\n", nil},
		"draft": {"version: 1\npush: { mode: pr, pr: { draft: true } }\n", []string{"--draft"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			titled(t, f, c.config, `"Keep $HOME, `+"`pwd`"+`, 'quotes' & -b (#12, #15)"`)
			if code, _, stderr, _ := call(t, f); code != 0 {
				t.Fatalf("exit %d\n%s", code, stderr)
			}
			want := append([]string{"pr", "create", "--repo", "owner/name", "--base", "main", "--head", item,
				"--title=slice-9: Keep $HOME, `pwd`, 'quotes' & -b (#12, #15)",
				"--body=Keep $HOME, `pwd`, 'quotes' & -b (#12, #15)\n\nCloses #12\nCloses #15"}, c.last...)
			if got := f.ghCalls[1]; !reflect.DeepEqual(got, want) {
				t.Errorf("pr create was called\n%q\nwant\n%q", got, want)
			}
		})
	}
}

// A pull request still titled after the first commit the push adds to the
// item's branch, as gh pr create --fill titles one, is retitled with gh pr
// edit, the title alone, after gh pr list and before anything else; a title
// that is not that subject (a person's), or is the item's title already (a
// first commit titled so), is left as it is, and so is one when the commit cannot be shown, whatever
// title gh listed.
func TestPushRetitlesAPullRequestStillTitledAfterItsFirstCommit(t *testing.T) {
	const (
		config = "version: 1\npush: { mode: pr, pr: { draft: true } }\n"
		edit   = "pr edit slice-9 --repo owner/name --title=slice-9: Add the widget (#12)"
	)
	for name, c := range map[string]struct {
		listed  string
		showErr error
		edited  bool
	}{
		"the first commit's subject":      {"docs: mine", nil, true},
		"a person's title":                {"The widget, at last", nil, false},
		"the item's title, the subject":   {"slice-9: Add the widget (#12)", nil, false},
		"no title listed, no commit":      {"", errors.New("no such commit"), false},
		"the subject, the commit unshown": {"docs: mine", errors.New("no such commit"), false},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := draftRepo(t, "draft: true")
			titled(t, f, config, `"Add the widget (#12)"`)
			p.open, p.title, f.showErr = open7, c.listed, c.showErr
			if name == "the item's title, the subject" {
				f.History[2].Message = c.listed + "\n"
			}
			if code, _, stderr, _ := call(t, f); code != 0 {
				t.Fatalf("exit %d\n%s", code, stderr)
			}
			want := []string{list}
			if c.edited {
				want = append(want, edit)
			}
			if got := f.calls(); !reflect.DeepEqual(got, want) {
				t.Errorf("gh was called\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}

	// Ready or not, the edit comes before the draft is made ready, auto-merge
	// is turned on and the branch is rebased.
	f, p := draftRepo(t, "draft: true")
	titled(t, f, config, `"Add the widget (#12)"`)
	p.open, p.title, p.draft = open7, "docs: mine", true
	if code, _, stderr, _ := call(t, f, "--no-wait", "--ready"); code != 0 {
		t.Fatalf("--ready: exit %d\n%s", code, stderr)
	}
	if got := f.calls(); len(got) < 3 || got[0] != list || got[1] != edit || got[2] != ready {
		t.Errorf("--ready: gh was called\n%s", strings.Join(got, "\n"))
	}
	f = prRepo(t)
	titled(t, f, "version: 1\npush: { mode: pr }\n", `"Add the widget (#12)"`)
	f.gh = (&pulls{open: open7, title: "docs: mine"}).gh
	if code, _, stderr, _ := call(t, f); code != 0 {
		t.Fatalf("plain: exit %d\n%s", code, stderr)
	}
	if got := f.calls(); len(got) != 3 || got[1] != edit || !strings.HasPrefix(got[2], "pr merge ") {
		t.Errorf("plain: gh was called\n%s", strings.Join(got, "\n"))
	}
}

// The first commit is the first that goes to the item's branch: the
// registry's commits that go to main are left out.
func TestPushRetitlesByTheFirstCommitOfTheItemsBranch(t *testing.T) {
	f, p := draftRepo(t, "draft: true")
	titled(t, f, "version: 1\npush: { mode: pr, pr: { draft: true } }\n", `"Add the widget (#12)"`)
	f.on(item, "wk000")
	p.open, p.title = open7, "docs: take"
	if code, _, stderr, _ := call(t, f); code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if got := f.calls(); len(got) != 1 {
		t.Errorf("a title that is a registry commit's subject was edited:\n%s", strings.Join(got, "\n"))
	}
	f, p = draftRepo(t, "draft: true")
	titled(t, f, "version: 1\npush: { mode: pr, pr: { draft: true } }\n", `"Add the widget (#12)"`)
	f.on(item, "wk000")
	p.open, p.title = open7, "feat: work"
	if code, _, stderr, _ := call(t, f); code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if got := f.calls(); len(got) != 2 || !strings.HasPrefix(got[1], "pr edit slice-9 ") {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
}

// A gh pr edit that fails is any gh call that fails: exit 3, outcome
// pull-request-failed, naming gh pr edit.
func TestPushWhereGhPrEditFails(t *testing.T) {
	f, p := draftRepo(t, "draft: true")
	titled(t, f, "version: 1\npush: { mode: pr, pr: { draft: true } }\n", `"Add the widget (#12)"`)
	p.open, p.title = open7, "docs: mine"
	p.fail = map[string]error{"pr edit": errors.New("boom")}
	code, _, stderr, _ := call(t, f)
	if code != missing || !strings.Contains(stderr, "is pushed, but gh pr edit failed: boom") {
		t.Errorf("exit %d\n%s", code, stderr)
	}
}
