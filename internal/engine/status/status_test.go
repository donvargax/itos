package status

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
	"github.com/donvargax/itos/v8/internal/work"
)

// configText names the registry and the people; ci.watch's provider is
// none.
const configText = "version: 1\nwork:\n  registry: work-items.yaml\n  people: { source: yaml, file: people.yaml }\n"

// ciText is configText with ci.watch's and ci.range's github providers,
// ci.yml their workflow and nightly.yml the nightly's.
const ciText = configText + "ci:\n  steps: []\n" +
	"  watch: { provider: github, interval: 0, github: { workflow: ci.yml, nightly_workflow: nightly.yml } }\n" +
	"  range: { provider: github, github: { workflow: ci.yml } }\n"

// configOf is the config text over v, its working tree holding files, the
// test standing in an empty folder, so no install script or AGENTS.md of
// the repository's is read.
func configOf(t *testing.T, v vcsport.VCS, text string, files map[string]string) *config.Loaded {
	t.Helper()
	t.Chdir(t.TempDir())
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(v, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// items is a registry of the items, each todo, unowned and in phase 1, and
// the item lines given.
func items(ids []string, lines ...string) string {
	text := "phases: { 1: null }\nitems:\n"
	for _, id := range ids {
		text += "  - { id: " + id + ", title: " + id + ", phase: 1, owner: null, status: todo, depends_on: [] }\n"
	}
	for _, line := range lines {
		text += "  - " + line + "\n"
	}
	return text
}

// envOf is the environment holding vars alone.
func envOf(vars map[string]string) providers.Env {
	return func(name string) string { return vars[name] }
}

// inOf is what the status reads over v with the environment env: no
// identity, no questions and no repository for drafts, stderr dropped.
func inOf(v vcsport.VCS, env map[string]string) In {
	return In{
		Identity:  func() providers.Answer { return providers.Answer{Problem: "nobody is asked"} },
		Stderr:    io.Discard,
		Git:       v,
		Setup:     providers.WatchSetup{Env: envOf(env)},
		Questions: func() (ask.File, error) { return ask.File{}, nil },
		Drafts:    func() (draft.File, bool, error) { return draft.File{}, false, nil },
	}
}

// printed is the status as text.
func printed(st *Standing) string {
	var b strings.Builder
	st.Print(&b)
	return b.String()
}

// emitted is the status as --json gives it.
func emitted(t *testing.T, st *Standing) string {
	t.Helper()
	var b strings.Builder
	if err := out.Emit(&b, st.Fields()...); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// secondReadFails is the working tree, but for the registry, which reads as
// it is once and fails after: a registry changed between its check and its
// load.
type secondReadFails struct {
	port.Source
	reads int
}

func (s *secondReadFails) Read(ctx context.Context, path string) (string, error) {
	if path == "work-items.yaml" {
		if s.reads++; s.reads > 1 {
			return "", errors.New("changed while read")
		}
	}
	return s.Source.Read(ctx, path)
}

// A registry that is not there is its problems, one that cannot be read or
// loaded after its check the error, and --as not among the people is
// missing, said on stderr.
func TestOfRefuses(t *testing.T) {
	v := &vcsfake.VCS{}
	cfg := configOf(t, v, configText, map[string]string{"people.yaml": "- a\n"})
	if st, found, missing, err := Of(t.Context(), cfg, "a", inOf(v, nil)); st != nil || len(found) != 1 ||
		found[0].Fix != work.Missing("work-items.yaml", false).Fix || missing || err != nil {
		t.Errorf("no registry: %v %v %v %v", st, found, missing, err)
	}
	cfg = configOf(t, v, configText, map[string]string{"work-items.yaml": "items: [\n", "people.yaml": "- a\n"})
	if st, found, missing, err := Of(t.Context(), cfg, "a", inOf(v, nil)); st != nil || found != nil || missing || err == nil {
		t.Errorf("a registry that cannot be read: %v %v %v %v", st, found, missing, err)
	}
	cfg = configOf(t, v, configText, map[string]string{"work-items.yaml": items([]string{"one"}), "people.yaml": "- a\n"})
	cfg.Source = &secondReadFails{Source: cfg.Source}
	if st, found, missing, err := Of(t.Context(), cfg, "a", inOf(v, nil)); st != nil || found != nil || missing || err == nil ||
		!strings.Contains(err.Error(), "changed while read") {
		t.Errorf("the second read's failure is not the error: %v %v %v", found, missing, err)
	}
	cfg = configOf(t, v, configText, map[string]string{"work-items.yaml": items([]string{"one"}), "people.yaml": "- a\n"})
	in := inOf(v, nil)
	var stderr strings.Builder
	in.Stderr = &stderr
	st, found, missing, err := Of(t.Context(), cfg, "zed", in)
	if st != nil || found != nil || !missing || err != nil || stderr.String() != "\"zed\" is not in people.yaml\n" {
		t.Errorf("--as not among the people: %v %v %v %v %q", st, found, missing, err, stderr.String())
	}
}

// Under a stealth config with no --as the session owns every item, which
// --json says; with nobody among the people only the unowned are listed,
// five of them and how many more, and a question names its item.
func TestOfEveryAndNobody(t *testing.T) {
	v := &vcsfake.VCS{}
	cfg := configOf(t, v, configText, map[string]string{
		"work-items.yaml": items([]string{"i1", "i2", "i3", "i4", "i5", "i6"}),
		"people.yaml":     "[]\n",
	})
	in := inOf(v, nil)
	in.Questions = func() (ask.File, error) {
		return ask.File{Questions: []ask.Question{{ID: "q-1", Item: "i1", Question: "Which way?"}}}, nil
	}
	cfg.Stealth = true
	st, _, _, err := Of(t.Context(), cfg, "", in)
	if err != nil || !st.Every {
		t.Fatalf("a stealth config's session does not own every item: %v %+v", err, st)
	}
	if got := emitted(t, st); !strings.Contains(got, `"every_item": true`) {
		t.Errorf("--json does not say every_item:\n%s", got)
	}
	if got := printed(st); !strings.Contains(got, "\nNothing in progress.\nNext, in the queue's order:\n  i1  i1\n") {
		t.Errorf("every item is not the session's:\n%s", got)
	}
	cfg.Stealth = false
	if st, _, _, err = Of(t.Context(), cfg, "", in); err != nil {
		t.Fatal(err)
	}
	if got := emitted(t, st); strings.Contains(got, "every_item") {
		t.Errorf("--json says every_item for nobody:\n%s", got)
	}
	got := printed(st)
	for _, want := range []string{"\nWorking for nobody: only the unowned items are listed.\n", "  i5  i5  (unowned)\n  and 1 more: itos work\n",
		"Open questions:\n  q-1  Which way?  (i1)\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("the status does not say %q:\n%s", want, got)
		}
	}
	if st.More != 1 || len(st.Next) != handful {
		t.Errorf("next %d and %d more, not five and 1", len(st.Next), st.More)
	}
}

// The person's items in progress and next are theirs, an open question is
// one line clipped to its width, ids aligned and answered ones left out,
// the drafts follow the questions, and questions or drafts that cannot be
// read are each a line, the rest still read.
func TestOfThePersons(t *testing.T) {
	v := &vcsfake.VCS{}
	cfg := configOf(t, v, configText, map[string]string{
		"work-items.yaml": items(nil, "{ id: d1, title: Doing one, phase: 1, owner: a, status: doing, depends_on: [] }",
			"{ id: n1, title: Next one, phase: 1, owner: a, status: todo, depends_on: [] }"),
		"people.yaml": "- a\n",
	})
	long := strings.Repeat("word ", 20) + "\n  and more"
	in := inOf(v, nil)
	in.Questions = func() (ask.File, error) {
		return ask.File{Questions: []ask.Question{{ID: "q-1", Question: "Done?", Answer: "Yes."},
			{ID: "q-2", Question: "Short?"}, {ID: "q-10", Question: long, Item: "d1"}}}, errors.New("half read")
	}
	in.Drafts = func() (draft.File, bool, error) {
		return draft.File{Drafts: []draft.Draft{{ID: "d-1", Command: []string{"work", "done", "d1"}}}}, true, errors.New("drafts broken")
	}
	st, _, _, err := Of(t.Context(), cfg, "", in)
	if err != nil {
		t.Fatal(err)
	}
	clippedLong := strings.TrimRight(strings.Repeat("word ", 16), " ")
	want := "# Where things stand\n\n" +
		"No remote origin, so neither its head nor its CI run is read.\n" +
		"The questions cannot be read: half read\n" +
		"The drafts cannot be read: drafts broken\n\n" +
		"In progress for a:\n  d1  Doing one\n" +
		"Next for a, in the queue's order:\n  n1  Next one\n" +
		"Open questions:\n  q-2   Short?\n  q-10  " + clippedLong + "…  (d1)\n" +
		"Drafts waiting, for itos draft promote once no work is going on:\n  d-1  itos work done d1\n"
	if got := printed(st); got != want {
		t.Errorf("the status:\n%s\nnot:\n%s", got, want)
	}
	if len(st.Questions) != 2 || len(st.Drafts) != 1 || st.Drafts[0].Kind != "command" {
		t.Errorf("questions %+v, drafts %+v", st.Questions, st.Drafts)
	}
	in.Drafts = func() (draft.File, bool, error) { return draft.File{}, false, errors.New("never read") }
	in.Questions = func() (ask.File, error) { return ask.File{}, nil }
	cfg = configOf(t, v, configText, map[string]string{"work-items.yaml": items(nil), "people.yaml": "- a\n"})
	if st, _, _, err = Of(t.Context(), cfg, "", in); err != nil {
		t.Fatal(err)
	}
	if got := printed(st); !strings.HasSuffix(got, "\n\nNothing in progress for a.\nNothing to start for a.\nNo open questions.\n") ||
		len(st.Unread) != 1 || st.Drafts == nil || len(st.Drafts) != 0 {
		t.Errorf("no drafts outside a repository, nothing to do:\n%s%q", got, st.Unread)
	}
}

// Who the identity provider names among several people is the session, a
// handle not among them said on stderr, and a provider that cannot say
// leaves the session nobody, its problem said.
func TestOfAsksTheIdentity(t *testing.T) {
	v := &vcsfake.VCS{}
	cfg := configOf(t, v, configText, map[string]string{"work-items.yaml": items([]string{"one"}), "people.yaml": "- a\n- b\n"})
	for _, c := range []struct {
		answer providers.Answer
		person any
		stderr string
	}{
		{providers.Answer{Handle: "b"}, "b", ""},
		{providers.Answer{Handle: "c"}, "c", "c is not in people.yaml: nothing is theirs yet\n"},
		{providers.Answer{Problem: "gh is signed out"}, nil, "gh is signed out\n"},
	} {
		in := inOf(v, nil)
		var stderr strings.Builder
		in.Stderr = &stderr
		in.Identity = func() providers.Answer { return c.answer }
		st, _, missing, err := Of(t.Context(), cfg, "", in)
		if err != nil || missing || st.Person != c.person || stderr.String() != c.stderr {
			t.Errorf("%+v: %v %v person %v, stderr %q", c.answer, err, missing, st.Person, stderr.String())
		}
	}
}

// Commit names of the world the head tests read.
var (
	c1  = strings.Repeat("1", 40)
	c2  = strings.Repeat("2", 40)
	c3  = strings.Repeat("3", 40)
	c4  = strings.Repeat("4", 40)
	tag = strings.Repeat("a", 40)
)

// world is a clone on main whose origin, a GitHub repository, has main at
// c3 and v1.0.0, an annotated tag of c1, as fetched here; c2 is a fix, c3 a
// chore.
func world() *vcsfake.VCS {
	return &vcsfake.VCS{
		History: []vcsfake.Commit{
			{Name: c1, Message: "feat: one\n"},
			{Name: c2, Parents: []string{c1}, Message: "fix: two\n\nIts body.\n"},
			{Name: c3, Parents: []string{c2}, Message: "chore: three\n"},
		},
		Tags:     map[string]string{tag: c1},
		Named:    map[string]string{"refs/remotes/origin/main": c3, "refs/tags/v1.0.0": tag},
		OnBranch: "main",
		Remote: map[string]*vcsfake.Remote{"origin": {URL: "https://github.com/o/n.git",
			Refs: map[string]string{"refs/heads/main": c3, "refs/tags/v1.0.0": tag}}},
	}
}

// gitHub is a GitHub API answering each path, its head_sha or branch query
// beside it, with its body, and 404 for any other; the environment to reach
// it, as o/n with a token.
func gitHub(t *testing.T, answers map[string]string) map[string]string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if sha := r.URL.Query().Get("head_sha"); sha != "" {
			key += "?head_sha=" + sha
		}
		if branch := r.URL.Query().Get("branch"); branch != "" {
			key += "?branch=" + branch
		}
		body, ok := answers[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return map[string]string{"GITHUB_API_URL": server.URL, "GITHUB_REPOSITORY": "o/n", "GITHUB_TOKEN": "t"}
}

// runs is a list of runs as GitHub's API gives it: one, its id, address,
// conclusion and head.
func runs(id, url, conclusion, head string) string {
	return `{"workflow_runs":[{"id":` + id + `,"html_url":"` + url + `","status":"completed","conclusion":"` + conclusion +
		`","head_sha":"` + head + `","created_at":"2026-10-01T00:00:00Z"}]}`
}

const (
	ciRuns      = "/repos/o/n/actions/workflows/ci.yml/runs?head_sha="
	nightlyRuns = "/repos/o/n/actions/workflows/nightly.yml/runs?branch=main"
)

// The head's line names the visibility, its CI run the jobs that failed,
// then the commit main last proved and the nightly; the newest release is
// the remote's, an annotated tag peeled, and the fix since it is
// unreleased, the chore not.
func TestOfReadsTheHead(t *testing.T) {
	v := world()
	env := gitHub(t, map[string]string{
		"/repos/o/n":                     `{"visibility":"public"}`,
		ciRuns + c3:                      runs("1", "u", "failure", c3),
		"/repos/o/n/actions/runs/1/jobs": `{"jobs":[{"name":"build","status":"completed","conclusion":"failure"},{"name":"lint","status":"completed","conclusion":"success"}]}`,
		ciRuns + c2:                      runs("2", "u2", "success", c2),
		nightlyRuns:                      runs("3", "n", "success", c3),
		"/repos/o/n/actions/runs/3/jobs": `{"jobs":[]}`,
	})
	cfg := configOf(t, v, ciText, map[string]string{"work-items.yaml": items([]string{"one"}), "people.yaml": "- a\n"})
	st, _, _, err := Of(t.Context(), cfg, "", inOf(v, env))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Where things stand\n\n" +
		"origin/main (public): 3333333 chore: three\n" +
		"CI: failure, u; failed jobs: build\n" +
		"Last green: 2222222 fix: two\n" +
		"Nightly: success, n\n\n" +
		"Newest release: v1.0.0\n" +
		"Unreleased since v1.0.0, oldest first:\n  fix: two\n\n" +
		"Nothing in progress for a.\n" +
		"Next for a, in the queue's order:\n  one  one  (unowned)\n" +
		"No open questions.\n"
	if got := printed(st); got != want {
		t.Errorf("the status:\n%s\nnot:\n%s", got, want)
	}
	if st.Head == nil || *st.Head != (Head{Remote: "origin", Branch: "main", Commit: c3, Header: "chore: three", Visibility: "public"}) ||
		st.Release == nil || *st.Release != (Release{Tag: "v1.0.0", Commit: c1}) ||
		st.LastGreen == nil || *st.LastGreen != (LastGreen{Commit: c2, Header: "fix: two"}) ||
		st.CI == nil || st.CI.Result != "failure" || st.Nightly == nil || st.Nightly.Jobs == nil || len(st.Unread) != 0 {
		t.Errorf("the standing: %+v", st)
	}
}

// From a detached HEAD the head is origin's HEAD, and a run that passed has
// no last green read.
func TestOfDetachedReadsOriginsHead(t *testing.T) {
	v := world()
	v.OnBranch = ""
	env := gitHub(t, map[string]string{
		ciRuns + c3:                      runs("1", "u", "success", c3),
		"/repos/o/n/actions/runs/1/jobs": `{"jobs":[]}`,
	})
	cfg := configOf(t, v, ciText, map[string]string{"work-items.yaml": items(nil), "people.yaml": "- a\n"})
	st, _, _, err := Of(t.Context(), cfg, "", inOf(v, env))
	if err != nil {
		t.Fatal(err)
	}
	if got := printed(st); !strings.HasPrefix(got, "# Where things stand\n\norigin/HEAD: 3333333 chore: three\nCI: success, u\nThe nightly cannot be read: ") {
		t.Errorf("the status:\n%s", got)
	}
}

// failing is the fake VCS but where it is told to fail: listing the
// remote's tags (all its refs), the tags as fetched, the log or the fetch.
type failing struct {
	*vcsfake.VCS
	tags, refs, log, fetch bool
}

func (f failing) Fetch(ctx context.Context, remote, refspec string, talk vcsport.Talk) (string, error) {
	if f.fetch {
		return "", &vcsport.Failed{Command: "git fetch " + remote, Code: 128, Words: "fatal: boom\nsecond line\n"}
	}
	return f.VCS.Fetch(ctx, remote, refspec, talk)
}

func (f failing) RemoteRefs(ctx context.Context, remote string, refs []string, talk vcsport.Talk) (map[string]string, error) {
	if f.tags && len(refs) == 0 {
		return nil, errors.New("no tags")
	}
	return f.VCS.RemoteRefs(ctx, remote, refs, talk)
}

func (f failing) Refs(ctx context.Context, prefix string) (map[string]string, error) {
	if f.refs {
		return nil, errors.New("no refs")
	}
	return f.VCS.Refs(ctx, prefix)
}

func (f failing) Log(ctx context.Context, r vcsport.Range) ([]vcsport.Entry, error) {
	if f.log {
		return nil, errors.New("no log")
	}
	return f.VCS.Log(ctx, r)
}

// unreachable is the line of a fetch from the world's remote when it is down.
const unreachable = "origin/main cannot be fetched: fatal: unable to access 'https://github.com/o/n.git': Could not resolve host"

// readBranch's lines and the release section's, each case's world changed
// from world(), ci.watch's provider none.
func TestReadBranch(t *testing.T) {
	for _, c := range []struct {
		name     string
		change   func(v *vcsfake.VCS) vcsport.VCS
		start    []string
		released []string
	}{
		{"no remote", func(v *vcsfake.VCS) vcsport.VCS { v.Remote = nil; return v },
			[]string{"No remote origin, so neither its head nor its CI run is read."}, nil},
		{"no branch yet", func(v *vcsfake.VCS) vcsport.VCS { delete(v.Remote["origin"].Refs, "refs/heads/main"); return v },
			[]string{"origin has no branch main yet, so there is no CI run to read."}, nil},
		{"unreached, as fetched", func(v *vcsfake.VCS) vcsport.VCS { v.Remote["origin"].Down = true; return v },
			[]string{unreachable, "origin cannot be reached, so its main is as last fetched.", "origin/main: 3333333 chore: three",
				"CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0 (as last fetched)", "Unreleased since v1.0.0, oldest first:", "  fix: two"}},
		{"unreached, never fetched", func(v *vcsfake.VCS) vcsport.VCS {
			v.Remote["origin"].Down = true
			delete(v.Named, "refs/remotes/origin/main")
			return v
		}, []string{unreachable, "origin cannot be reached and origin/main was never fetched, so neither its head nor its CI run is read."}, nil},
		{"the head not fetched here", func(v *vcsfake.VCS) vcsport.VCS {
			v.Remote["origin"].Refs["refs/heads/main"] = c4
			return failing{VCS: v, fetch: true}
		},
			[]string{"origin/main cannot be fetched: fatal: boom", "origin/main: 4444444 (not fetched here)",
				"CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0",
				"origin/main is not fetched here up to its head, so the commits since v1.0.0 may be behind.",
				"Unreleased since v1.0.0, as fetched here, oldest first:", "  fix: two"}},
		{"the head fetched, its commit not in the log", func(v *vcsfake.VCS) vcsport.VCS {
			v.Remote["origin"].Refs["refs/heads/main"] = c4
			return v
		}, []string{"origin/main: 4444444 (not fetched here)", "CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0", "origin/main is not fetched here, so the commits since v1.0.0 are not listed."}},
		{"nothing fetched here", func(v *vcsfake.VCS) vcsport.VCS {
			v.Remote["origin"].Refs["refs/heads/main"] = c4
			delete(v.Named, "refs/remotes/origin/main")
			return v
		}, []string{"origin/main: 4444444 (not fetched here)", "CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0", "origin/main is not fetched here, so the commits since v1.0.0 are not listed."}},
		{"the release not fetched here", func(v *vcsfake.VCS) vcsport.VCS { v.Tags[tag] = c4; return v },
			[]string{"origin/main: 3333333 chore: three", "CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0", "v1.0.0's commit is not fetched here, so the commits since it are not listed."}},
		{"nothing unreleased", func(v *vcsfake.VCS) vcsport.VCS { v.History[1].Message = "docs: two\n"; return v },
			[]string{"origin/main: 3333333 chore: three", "CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0", "Nothing unreleased since v1.0.0."}},
		{"no release yet", func(v *vcsfake.VCS) vcsport.VCS {
			v.Remote["origin"].Refs = map[string]string{"refs/heads/main": c3, "refs/tags/v1.0.0-rc.1": c1}
			return v
		}, []string{"origin/main: 3333333 chore: three", "CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: no release yet."}},
		{"the remote's tags unread", func(v *vcsfake.VCS) vcsport.VCS { return failing{VCS: v, tags: true} },
			[]string{"origin/main: 3333333 chore: three", "CI is not read: ci.watch.provider is none."},
			[]string{"origin's tags cannot be read, so its newest release is as last fetched.",
				"Newest release: v1.0.0 (as last fetched)", "Unreleased since v1.0.0, oldest first:", "  fix: two"}},
		{"no tags as fetched", func(v *vcsfake.VCS) vcsport.VCS { return failing{VCS: v, tags: true, refs: true} },
			[]string{"origin/main: 3333333 chore: three", "CI is not read: ci.watch.provider is none."},
			[]string{"origin's tags cannot be read, so its newest release is as last fetched.", "Newest release: no release yet."}},
		{"no log", func(v *vcsfake.VCS) vcsport.VCS { return failing{VCS: v, log: true} },
			[]string{"origin/main: 3333333 chore: three", "CI is not read: ci.watch.provider is none."},
			[]string{"Newest release: v1.0.0", "The commits since v1.0.0 cannot be read."}},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := c.change(world())
			cfg := configOf(t, v, configText, nil)
			st := &Standing{}
			st.readBranch(t.Context(), cfg, "origin", "refs/heads/main", "main", inOf(v, nil))
			if strings.Join(st.start, "\n") != strings.Join(c.start, "\n") || strings.Join(st.released, "\n") != strings.Join(c.released, "\n") {
				t.Errorf("lines:\n%s\n--\n%s\nnot:\n%s\n--\n%s", strings.Join(st.start, "\n"), strings.Join(st.released, "\n"),
					strings.Join(c.start, "\n"), strings.Join(c.released, "\n"))
			}
		})
	}
}

// A head read as last fetched says so, its header the fetched commit's,
// and the remote is not asked for its tags.
func TestReadBranchAsLastFetched(t *testing.T) {
	v := world()
	v.Remote["origin"].Down = true
	cfg := configOf(t, v, configText, nil)
	st := &Standing{}
	st.readBranch(t.Context(), cfg, "origin", "refs/heads/main", "main", inOf(v, nil))
	if h := st.Head; h == nil || *h != (Head{Remote: "origin", Branch: "main", Commit: c3, Header: "chore: three", LastFetched: true}) ||
		st.Release == nil || !st.Release.LastFetched || len(st.Unread) != 3 {
		t.Errorf("last fetched: head %+v, release %+v, unread %q", st.Head, st.Release, st.Unread)
	}
}

// The last green commit is not read with ci.range's provider none, said
// when it cannot be looked at or the look fails, none found when no first
// parent has a green run, and named with its header when one has.
func TestReadLastGreen(t *testing.T) {
	v := world()
	env := gitHub(t, map[string]string{ciRuns + c3: runs("1", "u", "failure", c3), ciRuns + c2: `{"workflow_runs":[]}`,
		ciRuns + c1: runs("2", "u", "success", c1)})
	none := configOf(t, v, configText+"ci:\n  steps: []\n  range: { provider: none }\n", nil)
	github := configOf(t, v, ciText, nil)
	for _, c := range []struct {
		name string
		cfg  *config.Loaded
		env  map[string]string
		head string
		line string
	}{
		{"none", none, env, "main", ""},
		{"no token", github, nil, "main", "The last green commit cannot be read: ci.range's github provider needs a token"},
		{"a look that fails", github, env, "elsewhere", "The last green commit cannot be read: the first parents of refs/remotes/origin/elsewhere cannot be listed"},
		{"a green parent", github, env, "main", "Last green: 1111111 feat: one"},
	} {
		st := &Standing{}
		st.readLastGreen(t.Context(), c.cfg, "origin", c.head, inOf(v, c.env))
		if got := strings.Join(st.start, "\n"); !strings.HasPrefix(got, c.line) || (c.line == "") != (got == "") {
			t.Errorf("%s: %q", c.name, got)
		}
	}
	env = gitHub(t, map[string]string{ciRuns + c3: `{"workflow_runs":[]}`, ciRuns + c2: `{"workflow_runs":[]}`, ciRuns + c1: `{"workflow_runs":[]}`})
	st := &Standing{}
	st.readLastGreen(t.Context(), github, "origin", "main", inOf(v, env))
	if strings.Join(st.start, "\n") != "Last green: none found" || st.LastGreen != nil {
		t.Errorf("none green: %q %+v", st.start, st.LastGreen)
	}
}

// The nightly is not read when ci.watch names none, said when it cannot be
// looked at or the look fails, no run when it has none, and its run's line
// when it has one.
func TestReadNightly(t *testing.T) {
	v := world()
	github := configOf(t, v, ciText, nil)
	noNightly := configOf(t, v, configText+"ci:\n  steps: []\n  watch: { provider: github, interval: 0 }\n", nil)
	for _, c := range []struct {
		name    string
		cfg     *config.Loaded
		answers map[string]string
		token   bool
		line    string
	}{
		{"none", noNightly, nil, true, ""},
		{"no token", github, nil, false, "The nightly cannot be read: ci.watch's github provider needs a token"},
		{"a look that fails", github, map[string]string{}, true, "The nightly cannot be read: GitHub's API answered 404"},
		{"no run yet", github, map[string]string{nightlyRuns: `{"workflow_runs":[]}`}, true, "Nightly: no run on main yet"},
		{"a run", github, map[string]string{nightlyRuns: runs("3", "n", "failure", c3),
			"/repos/o/n/actions/runs/3/jobs": `{"jobs":[{"name":"all","status":"completed","conclusion":"failure"}]}`}, true,
			"Nightly: failure, n; failed jobs: all"},
	} {
		env := gitHub(t, c.answers)
		if !c.token {
			env = nil
		}
		st := &Standing{}
		st.readNightly(t.Context(), c.cfg, "origin", "main", inOf(v, env))
		if got := strings.Join(st.start, "\n"); !strings.HasPrefix(got, c.line) || (c.line == "") != (got == "") {
			t.Errorf("%s: %q", c.name, got)
		}
	}
}

// visibility is "" for no provider, one that cannot look or cannot tell, and
// a look that fails.
func TestVisibilityOfAProvider(t *testing.T) {
	tells := providers.Watcher{Visibility: func() (string, error) { return "internal", nil }}
	fails := providers.Watcher{Visibility: func() (string, error) { return "public", errors.New("no network") }}
	for name, got := range map[string]string{
		"none":        visibility(tells, false, nil),
		"cannot look": visibility(tells, true, errors.New("no token")),
		"cannot tell": visibility(providers.Watcher{}, true, nil),
		"fails":       visibility(fails, true, nil),
	} {
		if got != "" {
			t.Errorf("%s: %q", name, got)
		}
	}
	if got := visibility(tells, true, nil); got != "internal" {
		t.Errorf("tells: %q", got)
	}
}

// readCI says why it did not read the run, for a provider that cannot look,
// none, and a look that fails, with no CI standing; a commit with no run yet
// is CI none, and a run found is its result, each said on the head's lines.
func TestReadsCI(t *testing.T) {
	found := providers.Run{URL: "u", Status: "completed", Conclusion: "success"}
	for _, c := range []struct {
		name   string
		ok     bool
		err    error
		look   providers.Watch
		result string
		line   string
	}{
		{"cannot look", true, errors.New("no token"), nil, "", "CI cannot be read: no token"},
		{"none", false, nil, nil, "", "CI is not read: ci.watch.provider is none."},
		{"a look that fails", true, nil, func(string) (providers.Run, bool, error) { return providers.Run{}, false, errors.New("502") }, "", "CI cannot be read: 502"},
		{"no run yet", true, nil, func(string) (providers.Run, bool, error) { return providers.Run{}, false, nil }, "none", "CI: no run of abcdef1 yet"},
		{"a run", true, nil, func(string) (providers.Run, bool, error) { return found, true, nil }, "success", "CI: success, u"},
	} {
		st := &Standing{}
		st.readCI(providers.Watcher{Look: c.look}, c.ok, c.err, "abcdef1234567")
		result := ""
		if st.CI != nil {
			result = st.CI.Result
		}
		unread := c.result == ""
		if result != c.result || len(st.start) != 1 || st.start[0] != c.line || (len(st.Unread) == 1) != unread {
			t.Errorf("%s: CI %+v, lines %q, unread %q", c.name, st.CI, st.start, st.Unread)
		}
	}
	st := &Standing{}
	st.readCI(providers.Watcher{Look: func(string) (providers.Run, bool, error) { return found, true, nil }}, true, nil, "abc")
	if st.CI.Run == nil || st.CI.Run.Jobs == nil {
		t.Errorf("a run with no jobs has them nil: %+v", st.CI.Run)
	}
}

// A run not done is going, its address left out when it has none, and the
// jobs that failed are named, not those skipped, neutral, passed or going.
func TestRunLine(t *testing.T) {
	run := providers.Run{Status: "in_progress", Jobs: []providers.Job{
		{Name: "a", Status: "completed", Conclusion: "failure"},
		{Name: "b", Status: "completed", Conclusion: "skipped"},
		{Name: "c", Status: "completed", Conclusion: "neutral"},
		{Name: "d", Status: "completed", Conclusion: "success"},
		{Name: "e", Status: "in_progress"},
		{Name: "f", Status: "completed", Conclusion: "cancelled"},
	}}
	if result, line := runLine("CI", run); result != "going" || line != "CI: going; failed jobs: a, f" {
		t.Errorf("%q %q", result, line)
	}
}

// talks records how the remote was asked.
type talks struct {
	*vcsfake.VCS
	asked []vcsport.Talk
}

func (r *talks) RemoteRefs(ctx context.Context, remote string, refs []string, talk vcsport.Talk) (map[string]string, error) {
	r.asked = append(r.asked, talk)
	return r.VCS.RemoteRefs(ctx, remote, refs, talk)
}

// The remote is asked for its head and its tags never prompting for
// credentials.
func TestAskRemoteNeverPrompts(t *testing.T) {
	v := &talks{VCS: world()}
	cfg := configOf(t, v, configText, nil)
	st := &Standing{}
	st.readBranch(t.Context(), cfg, "origin", "refs/heads/main", "main", inOf(v, nil))
	if len(v.asked) != 2 || !v.asked[0].NoPrompt || !v.asked[1].NoPrompt {
		t.Errorf("the remote was asked %+v", v.asked)
	}
}

// Questions' ids are aligned to the widest, however narrow.
func TestPrintAlignsTheQuestions(t *testing.T) {
	st := &Standing{Questions: []ask.Entry{{Question: "Unnamed?"}}}
	if got := printed(st); !strings.HasSuffix(got, "Open questions:\n    Unnamed?\n") {
		t.Errorf("%q", got)
	}
}

// peeled names each tag by the commit it names, an annotated tag's peeled
// entry over its own object, and leaves out refs that are no tags.
func TestPeeled(t *testing.T) {
	got := peeled(map[string]string{"refs/tags/v1": "t1", "refs/tags/v1^{}": "c1", "refs/tags/v2": "c2", "refs/heads/main": "m"},
		func(ref string) (string, bool) { return strings.CutPrefix(ref, "refs/tags/") })
	if len(got) != 2 || got["v1"] != "c1" || got["v2"] != "c2" {
		t.Errorf("%v", got)
	}
}

// clipped cuts at n characters, not bytes, with an ellipsis, its trailing
// spaces trimmed; text that fits is as it is.
func TestClipped(t *testing.T) {
	for text, want := range map[string]string{"éééé": "éééé", "ab cdef": "ab…", "abcdef": "abc…"} {
		if got := clipped(text, 4); got != want {
			t.Errorf("%q: %q", text, got)
		}
	}
}

// short is a commit's first seven characters, or all of a shorter name.
func TestShort(t *testing.T) {
	if short("abcdefgh") != "abcdefg" || short("abc") != "abc" || short("abcdefg") != "abcdefg" {
		t.Error(short("abcdefgh"), short("abc"))
	}
}

// The questions are read once, and then the drafts, after the items.
func TestOfReadsQuestionsThenDrafts(t *testing.T) {
	v := &vcsfake.VCS{}
	cfg := configOf(t, v, configText, map[string]string{"work-items.yaml": items(nil), "people.yaml": "- a\n"})
	var order bytes.Buffer
	in := inOf(v, nil)
	in.Questions = func() (ask.File, error) { order.WriteString("questions "); return ask.File{}, nil }
	in.Drafts = func() (draft.File, bool, error) { order.WriteString("drafts"); return draft.File{}, true, nil }
	if _, _, _, err := Of(t.Context(), cfg, "", in); err != nil || order.String() != "questions drafts" {
		t.Errorf("%v, read %q", err, order.String())
	}
}
