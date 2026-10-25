package work

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

const (
	pullURL = "https://github.example/o/r/pull/7"
	// headedPull is a merged pull request with its head, h-1.
	headedPull = `[{"state":"MERGED","mergedAt":"2026-10-24T10:00:00Z","number":7,"url":"` + pullURL + `","headRefOid":"h-1"}]`
	// the lines work done says of the head, up to what comes after them.
	merged   = "T-1's pull request " + pullURL + " merged with the head h-1"
	judging  = ", and c-1a, T-1's newest commit on origin/main,"
	judgedBy = "so origin/main's CI run judges c-1a\n"
	bookkept = merged + judging + " has its tree but for the work registry and the ledger"
)

// trust is a fixture under push.mode pr with ci.watch: T-1's own commit,
// c-1a, is on origin/main, and its merged pull request 7 has the head h-1, a
// commit whose tree is c-1a's with the changes given (a text of "" is not a
// change). The clone has h-1 unless fetched is set, which a fetch of
// refs/pull/7/head then brings.
type trust struct {
	p       ports
	fetched *[]string
	talked  *[]vcsport.Talk
	// left is how long the fetch had to go on, as the context said.
	left *time.Duration
}

func trusting(t *testing.T, changes map[string]string, fetched bool, pulls string) trust {
	t.Helper()
	f := closable(false)
	f.config = watching + "push:\n  mode: pr\n"
	p := over(t, f)
	v := vcsOf(p)
	v.OnBranch = "main"
	v.Remote = map[string]*vcsfake.Remote{"origin": {URL: "u", Refs: map[string]string{"refs/heads/main": "c-4a"}}}
	v.Named["refs/remotes/origin/main"] = "c-1a"
	tree := maps.Clone(v.History[0].Tree)
	for path, text := range changes {
		tree[path] = text
	}
	head := vcsfake.Commit{Name: "h-1", Message: "chore: build\n\nTask: T-1\n", Tree: tree}
	tr := trust{p: p, fetched: &[]string{}, talked: &[]vcsport.Talk{}, left: new(time.Duration)}
	if fetched {
		bring := func(ctx context.Context, remote, refspec string, talk vcsport.Talk) error {
			*tr.fetched = append(*tr.fetched, remote+" "+refspec)
			*tr.talked = append(*tr.talked, talk)
			if d, ok := ctx.Deadline(); ok {
				*tr.left = time.Until(d)
			}
			v.History = append(v.History, head)
			return nil
		}
		tr.p.cfg.VCS = broken{VCS: v, fetch: bring}
	} else {
		v.History = append(v.History, head)
		tr.p.cfg.VCS = broken{VCS: v, fetch: func(ctx context.Context, remote, refspec string, talk vcsport.Talk) error {
			*tr.fetched = append(*tr.fetched, remote+" "+refspec)
			return nil
		}}
	}
	tr.p.pr = pr{head: "main", repository: "owner/scratch", ghOut: pulls, ghCalls: &[][]string{}, headCalls: &[]string{}}
	return tr
}

func passes(url, head string) providers.Run {
	return providers.Run{URL: url, Status: "completed", Conclusion: "success", HeadSHA: head,
		Jobs: []providers.Job{{Name: "ci", Status: "completed", Conclusion: "success"}}}
}

// bookkeeping is a head whose tree differs from the landing commit's in the
// work registry and the ledger alone.
var bookkeeping = map[string]string{"work-items.yaml": "gained\n", "tasks/phase-1.yaml": "gained\n"}

// When the merged pull request's head differs from the landing commit only in
// the registry and the ledger, and its run passed, that run judges the close:
// the line says so before the run's end, the close commit's body names it
// instead of main's, --json gives it as the run and the pull request as
// trusted, and main's run is not looked at; -q says no line.
func TestDoneTrustsTheMergedHeadsRun(t *testing.T) {
	runs := map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": {URL: "u-main", Status: "in_progress"}}
	tr := trusting(t, bookkeeping, false, headedPull)
	tr.p.watcher = look(runs)
	txs := txsOf(&tr.p)
	code, stdout, stderr, err := run(t, tr.p, "done", "T-1")
	if code != 0 || err != nil || strings.Contains(stderr, "merged with the head") || len(*txs) != 1 ||
		!strings.HasPrefix(stdout, bookkept+", so the head's CI run judges c-1a\nCI passed: u-head\nT-1 is done: committed ") ||
		!strings.Contains(bodyOf((*txs)[0]), " The CI run of h-1, the head of T-1's pull request "+pullURL+
			", whose tree c-1a has but for the work registry and the ledger, passed: u-head.") ||
		strings.Contains(bodyOf((*txs)[0]), "newest commit of T-1") || len(*tr.fetched) != 0 {
		t.Errorf("trusted: %d, %v, %q, %q, %v, fetches %v", code, err, stdout, stderr, bodyOf((*txs)[0]), *tr.fetched)
	}

	tr = trusting(t, bookkeeping, false, headedPull)
	tr.p.watcher = look(runs)
	code, stdout, stderr, err = run(t, tr.p, "--json", "done", "T-1")
	o := object(t, stdout)
	pulled, _ := o["pull_request"].(map[string]any)
	if code != 0 || err != nil || o["ci"] != "success" || o["run"].(map[string]any)["url"] != "u-head" ||
		pulled["url"] != pullURL || pulled["head"] != "h-1" || pulled["trusted"] != true || len(pulled) != 3 ||
		!strings.HasPrefix(stderr, bookkept+", so the head's CI run judges c-1a\nCI passed: u-head\n") {
		t.Errorf("trusted, --json: %d, %v, %v, %q", code, err, o, stderr)
	}

	tr = trusting(t, bookkeeping, false, headedPull)
	tr.p.watcher = look(runs)
	if code, stdout, stderr, err := run(t, tr.p, "-q", "done", "T-1"); code != 0 || err != nil || stdout != "" || strings.Contains(stderr, "merged with the head") || strings.Contains(stderr, "CI passed") {
		t.Errorf("trusted, -q: %d, %v, %q, %q", code, err, stdout, stderr)
	}
}

// A head whose tree differs in another path leaves the close to main's run,
// the line naming the first such path and how many more, the registry and a
// ledger file not counted; the close's body names main's run, as slice 127
// does, and --json gives the pull request as not trusted.
func TestDoneWaitsForMainsRunWhenTheTreesDiffer(t *testing.T) {
	runs := map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")}
	for name, c := range map[string]struct {
		changes map[string]string
		path    string
	}{
		"one path":            {map[string]string{"widget.md": "x\n"}, "widget.md"},
		"with the registry":   {map[string]string{"widget.md": "x\n", "work-items.yaml": "x\n"}, "widget.md"},
		"two more":            {map[string]string{"b.md": "x\n", "a.md": "x\n", "c.md": "x\n", "tasks/phase-1.yaml": "x\n"}, "a.md and 2 more"},
		"one more":            {map[string]string{"b.md": "x\n", "a.md": "x\n"}, "a.md and 1 more"},
		"a folder in ledger":  {map[string]string{"tasks/sub/phase-1.yaml": "x\n"}, "tasks/sub/phase-1.yaml"},
		"a name not ledger's": {map[string]string{"tasks/notes.yaml": "x\n"}, "tasks/notes.yaml"},
	} {
		tr := trusting(t, c.changes, false, headedPull)
		tr.p.watcher = look(runs)
		txs := txsOf(&tr.p)
		code, stdout, _, err := run(t, tr.p, "done", "T-1")
		want := merged + judging + " differs from it in " + c.path + ", " + judgedBy + "CI passed: u-main\n"
		if code != 0 || err != nil || !strings.HasPrefix(stdout, want) ||
			!strings.Contains(bodyOf((*txs)[0]), " The CI run of c-1, the newest commit of T-1, passed: u-main.") {
			t.Errorf("%s: %d, %v, %q, want it to start %q", name, code, err, stdout, want)
		}
		tr = trusting(t, c.changes, false, headedPull)
		tr.p.watcher = look(runs)
		_, stdout, _, _ = run(t, tr.p, "--json", "done", "T-1")
		pulled, _ := object(t, stdout)["pull_request"].(map[string]any)
		if pulled["trusted"] != false || pulled["head"] != "h-1" {
			t.Errorf("%s, --json: pull_request %v", name, pulled)
		}
	}
}

// A head's run that is no pass leaves the close to main's run and refuses
// nothing: one that failed, one still going, one whose jobs were all skipped
// and one that lists none, each said with its address; none found, or a look
// that fails, said as no run.
func TestDoneWaitsForMainsRunWhenTheHeadsRunIsNoPass(t *testing.T) {
	main := passes("u-main", "c-1a")
	skipped := providers.Run{URL: "u-head", Status: "completed", Conclusion: "success", HeadSHA: "h-1",
		Jobs: []providers.Job{{Name: "ci", Status: "completed", Conclusion: "skipped"}, {Name: "platform", Status: "completed", Conclusion: "skipped"}}}
	for name, c := range map[string]struct {
		watcher providers.Watcher
		line    string
	}{
		"failed": {look(map[string]providers.Run{"c-1a": main, "h-1": {URL: "u-head", Status: "completed", Conclusion: "failure",
			Jobs: []providers.Job{{Name: "ci", Status: "completed", Conclusion: "failure"}}}}), "but the head's CI run u-head has not passed, "},
		"still going": {look(map[string]providers.Run{"c-1a": main, "h-1": {URL: "u-head", Status: "in_progress",
			Jobs: []providers.Job{{Name: "ci", Status: "in_progress"}}}}), "but the head's CI run u-head has not passed, "},
		"all jobs skipped": {look(map[string]providers.Run{"c-1a": main, "h-1": skipped}), "but the head's CI run u-head has not passed, "},
		"no jobs": {look(map[string]providers.Run{"c-1a": main, "h-1": {URL: "u-head", Status: "completed", Conclusion: "success"}}),
			"but the head's CI run u-head has not passed, "},
		"no run": {look(map[string]providers.Run{"c-1a": main}), "but no CI run of the head was found, "},
		"a look that fails": {providers.Watcher{Look: func(sha string) (providers.Run, bool, error) {
			if sha == "h-1" {
				return providers.Run{}, false, errors.New("the API is down")
			}
			return main, true, nil
		}}, "but no CI run of the head was found, "},
	} {
		tr := trusting(t, bookkeeping, false, headedPull)
		tr.p.watcher = c.watcher
		code, stdout, _, err := run(t, tr.p, "done", "T-1")
		want := bookkept + ", " + c.line + judgedBy + "CI passed: u-main\n"
		if code != 0 || err != nil || !strings.HasPrefix(stdout, want) {
			t.Errorf("%s: %d, %v, %q, want it to start %q", name, code, err, stdout, want)
		}
	}
}

// A head the clone lacks is fetched once from the remote's refs/pull/<n>/head,
// asking for no credentials, and judges the close when the fetch brought it,
// saying nothing of the fetch; one the clone has is never fetched. A fetch
// that fails, or after which the head is still missing, leaves the close to
// main's run, the line saying so.
func TestDoneFetchesTheMergedHead(t *testing.T) {
	runs := map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")}
	tr := trusting(t, bookkeeping, true, headedPull)
	tr.p.watcher = look(runs)
	code, stdout, stderr, err := run(t, tr.p, "done", "T-1")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, bookkept+", so the head's CI run judges c-1a\nCI passed: u-head\n") ||
		!slices.Equal(*tr.fetched, []string{"origin refs/pull/7/head"}) || len(*tr.talked) != 1 || !(*tr.talked)[0].NoPrompt ||
		*tr.left <= 0 || *tr.left > ciwatch.FetchTimeout {
		t.Errorf("a head fetched: %d, %v, %q, %q, fetches %v %v", code, err, stdout, stderr, *tr.fetched, *tr.talked)
	}

	tr = trusting(t, bookkeeping, false, headedPull)
	tr.p.watcher = look(runs)
	if code, _, _, err := run(t, tr.p, "done", "T-1"); code != 0 || err != nil || len(*tr.fetched) != 0 {
		t.Errorf("a head the clone has: %d, %v, fetches %v", code, err, *tr.fetched)
	}

	lost := merged + ", which this clone does not have and could not fetch from origin's refs/pull/7/head, " + judgedBy + "CI passed: u-main\n"
	tr = trusting(t, bookkeeping, true, headedPull)
	tr.p.watcher = look(runs)
	tr.p.cfg.VCS = broken{VCS: vcsOf(tr.p), fetch: func(_ context.Context, remote, refspec string, _ vcsport.Talk) error {
		*tr.fetched = append(*tr.fetched, remote+" "+refspec)
		return &vcsport.Failed{Command: "git fetch", Code: 128, Failure: vcsport.Unreachable, Words: "fatal: no route\n"}
	}}
	code, stdout, _, err = run(t, tr.p, "done", "T-1")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, lost) || len(*tr.fetched) != 1 {
		t.Errorf("a fetch that fails: %d, %v, %q, want it to start %q", code, err, stdout, lost)
	}

	tr = trusting(t, bookkeeping, true, headedPull)
	tr.p.watcher = look(runs)
	tr.p.cfg.VCS = broken{VCS: vcsOf(tr.p), fetch: func(_ context.Context, remote, refspec string, _ vcsport.Talk) error {
		*tr.fetched = append(*tr.fetched, remote+" "+refspec)
		return nil
	}}
	code, stdout, stderr, err = run(t, tr.p, "--json", "done", "T-1")
	pulled, _ := object(t, stdout)["pull_request"].(map[string]any)
	if code != 0 || err != nil || !strings.HasPrefix(stderr, lost) || pulled["trusted"] != false || len(*tr.fetched) != 1 {
		t.Errorf("a head still missing: %d, %v, %q, %v", code, err, stderr, pulled)
	}
}

// A diff git cannot give is no comparison: main's run judges the close, said
// nowhere.
func TestDoneWaitsForMainsRunWhenTheDiffFails(t *testing.T) {
	tr := trusting(t, bookkeeping, false, headedPull)
	tr.p.watcher = look(map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")})
	tr.p.cfg.VCS = broken{VCS: vcsOf(tr.p), diff: errors.New("bad object")}
	code, stdout, stderr, err := run(t, tr.p, "--json", "done", "T-1")
	o := object(t, stdout)
	if pulled, _ := o["pull_request"].(map[string]any); code != 0 || err != nil || strings.Contains(stderr, "merged with the head") ||
		o["run"].(map[string]any)["url"] != "u-main" || pulled["trusted"] != false || pulled["head"] != "h-1" {
		t.Errorf("a diff that fails: %d, %v, %q, %v", code, err, stderr, o)
	}
}

// The pull request is no key of --json without a merged pull request, with
// one gh gave no head, and without ci.watch, and nothing is said of the head;
// a close that main's run refuses has it, as not trusted.
func TestDoneGivesThePullRequestOnlyWithAHeadAndCIWatch(t *testing.T) {
	main := map[string]providers.Run{"c-1a": passes("u-main", "c-1a")}
	for name, pulls := range map[string]string{
		"none":           "[]",
		"an empty head":  strings.Replace(headedPull, `"h-1"`, `""`, 1),
		"no head at all": strings.Replace(headedPull, `,"headRefOid":"h-1"`, "", 1),
	} {
		tr := trusting(t, bookkeeping, false, pulls)
		tr.p.watcher = look(main)
		code, stdout, stderr, err := run(t, tr.p, "--json", "done", "T-1")
		if _, has := object(t, stdout)["pull_request"]; code != 0 || err != nil || has || strings.Contains(stderr, "merged with the head") {
			t.Errorf("%s: %d, %v, %q, %q", name, code, err, stdout, stderr)
		}
		tr = trusting(t, bookkeeping, false, pulls)
		tr.p.watcher = look(main)
		if _, stdout, _, _ := run(t, tr.p, "done", "T-1"); strings.Contains(stdout, "merged with the head") {
			t.Errorf("%s: work done says %q", name, stdout)
		}
	}

	p, _, _ := prDone(t, headedPull)
	code, stdout, stderr, err := run(t, p, "--json", "done", "T-1")
	if _, has := object(t, stdout)["pull_request"]; code != 0 || err != nil || has || !strings.Contains(stderr, "ci.watch.provider is none") {
		t.Errorf("without ci.watch: %d, %v, %q, %q", code, err, stdout, stderr)
	}

	failed := providers.Run{URL: "u-main", Status: "completed", Conclusion: "failure", HeadSHA: "c-1a"}
	tr := trusting(t, bookkeeping, false, headedPull)
	tr.p.watcher = look(map[string]providers.Run{"c-1a": failed, "h-1": failed}, failed)
	code, stdout, stderr, err = run(t, tr.p, "--json", "done", "T-1")
	o := object(t, stdout)
	pulled, _ := o["pull_request"].(map[string]any)
	if code != refused || err != nil || o["ok"] != false || o["ci"] != "failure" || pulled["trusted"] != false || pulled["url"] != pullURL ||
		!strings.Contains(stderr, "but the head's CI run u-main has not passed") || registryIn(t, tr.p) != doneText {
		t.Errorf("a red main run: %d, %v, %v, %q", code, err, o, stderr)
	}
}

// The paths a head's tree differs in that are neither the registry nor a
// ledger file, as ledger.files lays the ledger out; with no ledger section no
// path is one, and a path is cleaned as git gives it.
func TestOutsideBookkeeping(t *testing.T) {
	p := over(t, sound())
	files := []string{"widget.md", "work-items.yaml", "tasks/phase-2.yaml", "tasks/notes.yaml", "tasks/sub/phase-1.yaml", "other/phase-1.yaml", "tasks/phase-1.yml"}
	want := []string{"widget.md", "tasks/notes.yaml", "tasks/sub/phase-1.yaml", "other/phase-1.yaml", "tasks/phase-1.yml"}
	if got := outsideBookkeeping(p.cfg, files); !slices.Equal(got, want) {
		t.Errorf("outsideBookkeeping = %q, want %q", got, want)
	}
	if got := outsideBookkeeping(p.cfg, nil); len(got) != 0 {
		t.Errorf("no paths, outsideBookkeeping = %q", got)
	}
	f := sound()
	f.config = strings.NewReplacer("ledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n", "",
		"    Task: { source: ledger }\n", "").Replace(configText)
	bare := over(t, f)
	if got := outsideBookkeeping(bare.cfg, []string{"work-items.yaml", "tasks/phase-1.yaml"}); !slices.Equal(got, []string{"tasks/phase-1.yaml"}) {
		t.Errorf("with no ledger section, outsideBookkeeping = %q", got)
	}
}
