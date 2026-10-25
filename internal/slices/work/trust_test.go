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
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
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
	return trustingLedgers(t, changes, fetched, pulls, map[string]string{ledgerFile: staticLedgerText}, map[string]string{ledgerFile: staticLedgerText})
}

// trustingLedgers is trusting with the files the ledger is read from at the
// head, h-1, and at the landing commit, c-1a (bug 137): the same ledger at
// both, T-1's one static check, for trusting.
func trustingLedgers(t *testing.T, changes map[string]string, fetched bool, pulls string, atHead, atLanding map[string]string) trust {
	t.Helper()
	f := closable(false)
	f.config = watching + "push:\n  mode: pr\n"
	p := over(t, f)
	trees := p.cfg.Trees.(treefake.Trees)
	files := maps.Clone(trees.Files)
	files["h-1"], files["c-1a"] = atHead, atLanding
	p.cfg.Trees = treefake.Trees{Common: trees.Common, Files: files}
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
	if got, _ := outsideBookkeeping(p.cfg, files); !slices.Equal(got, want) {
		t.Errorf("outsideBookkeeping = %q, want %q", got, want)
	}
	if got, _ := outsideBookkeeping(p.cfg, nil); len(got) != 0 {
		t.Errorf("no paths, outsideBookkeeping = %q", got)
	}
	f := sound()
	f.config = strings.NewReplacer("ledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n", "",
		"    Task: { source: ledger }\n", "").Replace(configText)
	bare := over(t, f)
	if got, _ := outsideBookkeeping(bare.cfg, []string{"work-items.yaml", "tasks/phase-1.yaml"}); !slices.Equal(got, []string{"tasks/phase-1.yaml"}) {
		t.Errorf("with no ledger section, outsideBookkeeping = %q", got)
	}
}

// entry is a ledger file holding the task, T-1 unless it says otherwise, with
// the checks given, each a flow mapping; none gives it no done_when.
func entry(id string, checks ...string) string {
	text := "- id: " + id + "\n  type: chore\n  title: Adopt\n"
	if len(checks) > 0 {
		text += "  done_when:\n"
	}
	for _, c := range checks {
		text += "    - " + c + "\n"
	}
	return text
}

func ledgerOf(files ...string) map[string]string {
	ledger := map[string]string{}
	for i, text := range files {
		ledger[strings.Replace(ledgerFile, "1", string(rune('1'+i)), 1)] = text
	}
	return ledger
}

const (
	checked   = `{ run: "true", cost: static }`
	unreadMsg = ", but the ledger at "
	differMsg = ", but T-1's checks in the ledger differ from the head's, "
)

// For a task of the ledger whose files git lists, the head's run judges the
// close only when the task's own checks are the same at the head and at the
// landing commit (bug 137); otherwise a line says they differ and main's run
// judges, the head's run not looked at.
func TestDoneComparesTheTasksOwnChecks(t *testing.T) {
	timed := `{ run: "true", cost: static, timeout: 5 }`
	for name, c := range map[string]struct {
		head, landing map[string]string
		differ        bool
	}{
		"no entry at either":                        {ledgerOf(entry("T-2", checked)), ledgerOf(entry("T-2", checked)), false},
		"the same checks":                           {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked)), false},
		"no checks at either":                       {ledgerOf(entry("T-1")), ledgerOf(entry("T-1")), false},
		"an empty list and none":                    {ledgerOf(entry("T-1") + "  done_when: []\n"), ledgerOf(entry("T-1")), false},
		"another title, type and why":               {ledgerOf(entry("T-1", checked)), ledgerOf("- { id: T-1, type: fix, title: Other, why: Why, done_when: [" + checked + "] }\n"), false},
		"another task's checks":                     {ledgerOf(entry("T-1", checked) + entry("T-2")), ledgerOf(entry("T-1", checked) + entry("T-2", `{ run: "exit 1", cost: late }`)), false},
		"another task added":                        {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked) + entry("T-3", `{ run: "exit 1", cost: late }`)), false},
		"moved to another file":                     {ledgerOf(entry("T-1", checked), entry("T-2")), ledgerOf(entry("T-2"), entry("T-1", checked)), false},
		"gone and added with the same":              {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-9") + entry("T-1", checked)), false},
		"a check added":                             {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked, `{ run: "exit 1", cost: late }`)), true},
		"a check added to an entry with none":       {ledgerOf(entry("T-1")), ledgerOf(entry("T-1", checked)), true},
		"a check removed":                           {ledgerOf(entry("T-1", checked, `{ run: "exit 1", cost: late }`)), ledgerOf(entry("T-1", checked)), true},
		"the checks reordered":                      {ledgerOf(entry("T-1", checked, `{ run: "exit 1", cost: late }`)), ledgerOf(entry("T-1", `{ run: "exit 1", cost: late }`, checked)), true},
		"the entry gone":                            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-2", checked)), true},
		"the entry new":                             {ledgerOf(entry("T-2", checked)), ledgerOf(entry("T-1", checked)), true},
		"gone and added with other checks":          {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-9") + entry("T-1", `{ run: "false", cost: static }`)), true},
		"a second entry in another file":            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked), entry("T-1", checked)), true},
		"a second entry at the head":                {ledgerOf(entry("T-1", checked), entry("T-1", checked)), ledgerOf(entry("T-1", checked)), true},
		"another command":                           {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "false", cost: static }`)), true},
		"fails for run":                             {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ fails: "true", cost: static }`)), true},
		"another fails":                             {ledgerOf(entry("T-1", `{ fails: "a" }`)), ledgerOf(entry("T-1", `{ fails: "b" }`)), true},
		"after alone":                               {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, after: push }`)), true},
		"timeout alone":                             {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", timed)), true},
		"another timeout":                           {ledgerOf(entry("T-1", timed)), ledgerOf(entry("T-1", `{ run: "true", cost: static, timeout: 6 }`)), true},
		"cost alone":                                {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: late }`)), true},
		"prose alone":                               {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, prose: true }`)), true},
		"evidence alone":                            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, evidence: { inputs: [a] } }`)), true},
		"other evidence":                            {ledgerOf(entry("T-1", `{ run: "true", evidence: { inputs: [a] } }`)), ledgerOf(entry("T-1", `{ run: "true", evidence: { inputs: [b] } }`)), true},
		"snapshot alone":                            {ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", `{ run: "true", cost: static, snapshot: false }`)), true},
		"checks that cannot be read at the head":    {ledgerOf(entry("T-1") + "  done_when: nope\n"), ledgerOf(entry("T-1")), true},
		"checks that cannot be read at the landing": {ledgerOf(entry("T-1")), ledgerOf(entry("T-1") + "  done_when: nope\n"), true},
		"a check with no command at both":           {ledgerOf(entry("T-1", `{ cost: late }`)), ledgerOf(entry("T-1", `{ cost: late }`)), true},
	} {
		runs := map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")}
		tr := trustingLedgers(t, map[string]string{ledgerFile: "gained\n"}, false, headedPull, c.head, c.landing)
		tr.p.watcher = look(runs)
		code, stdout, _, err := run(t, tr.p, "done", "T-1")
		want := bookkept + ", so the head's CI run judges c-1a\nCI passed: u-head\n"
		if c.differ {
			want = bookkept + differMsg + judgedBy + "CI passed: u-main\n"
		}
		if code != 0 || err != nil || !strings.HasPrefix(stdout, want) {
			t.Errorf("%s: %d, %v, %q, want it to start %q", name, code, err, stdout, want)
		}
	}

	tr := trustingLedgers(t, bookkeeping, false, headedPull, ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1")))
	tr.p.watcher = look(map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")})
	code, stdout, stderr, err := run(t, tr.p, "--json", "done", "T-1")
	o := object(t, stdout)
	pulled, _ := o["pull_request"].(map[string]any)
	if code != 0 || err != nil || pulled["trusted"] != false || pulled["head"] != "h-1" || o["run"].(map[string]any)["url"] != "u-main" ||
		!strings.HasPrefix(stderr, bookkept+differMsg+judgedBy) {
		t.Errorf("checks that differ, --json: %d, %v, %v, %q", code, err, o, stderr)
	}
}

// A ledger work done cannot read at the head or at the landing commit (a file
// that is not YAML, or not a list, or no ledger folder) is no comparison: the
// line names the commit whose ledger it is, the head's first, and main's run
// judges.
func TestDoneWaitsForMainsRunWhenTheLedgerCannotBeRead(t *testing.T) {
	good := ledgerOf(entry("T-1", checked))
	for name, c := range map[string]struct {
		head, landing map[string]string
		at            string
	}{
		"not YAML at the head":      {ledgerOf("- [\n"), good, "h-1"},
		"not a list at the head":    {ledgerOf("a: b\n"), good, "h-1"},
		"no folder at the head":     {nil, good, "h-1"},
		"not YAML at the landing":   {good, ledgerOf("- [\n"), "c-1a"},
		"not a list at the landing": {good, ledgerOf("a: b\n"), "c-1a"},
		"no folder at the landing":  {good, nil, "c-1a"},
		"both":                      {ledgerOf("a: b\n"), ledgerOf("a: b\n"), "h-1"},
	} {
		runs := map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")}
		tr := trustingLedgers(t, bookkeeping, false, headedPull, c.head, c.landing)
		tr.p.watcher = look(runs)
		code, stdout, _, err := run(t, tr.p, "done", "T-1")
		want := bookkept + unreadMsg + c.at + " cannot be read, " + judgedBy + "CI passed: u-main\n"
		if code != 0 || err != nil || !strings.HasPrefix(stdout, want) {
			t.Errorf("%s: %d, %v, %q, want it to start %q", name, code, err, stdout, want)
		}
	}
}

// A commit whose tree the repository cannot give is a ledger that cannot be
// read as well.
func TestTrustHasNoComparisonWithoutTheCommitsTree(t *testing.T) {
	tr := trustingLedgers(t, bookkeeping, false, headedPull, ledgerOf(entry("T-1", checked)), ledgerOf(entry("T-1", checked)))
	var stdout, stderr strings.Builder
	tr.p.cfg.Trees = treefake.Trees{}
	m := &mergedHead{url: pullURL, head: "h-1", remote: "origin", on: "origin/main", number: 7}
	runs := map[string]providers.Run{"h-1": passes("u-head", "h-1")}
	_, ok := trustedRun(t.Context(), tr.p.cfg, "T-1", m, doneCommit{sha: "c-1a"}, look(runs), Out{Stdout: &stdout, Stderr: &stderr})
	if ok || stdout.String() != bookkept+unreadMsg+"h-1 cannot be read, "+judgedBy {
		t.Errorf("a tree that cannot be given: %v, %q", ok, stdout.String())
	}
}

// The ledger is read only for a task whose files git lists: a diff that lists
// the registry alone reads none (an unreadable ledger changes nothing), and
// an item that is no task of the ledger is trusted on the paths alone.
func TestTrustReadsTheLedgerOnlyForATaskWithLedgerFilesListed(t *testing.T) {
	unreadable := ledgerOf("a: b\n")
	runs := map[string]providers.Run{"h-1": passes("u-head", "h-1"), "c-1a": passes("u-main", "c-1a")}
	want := bookkept + ", so the head's CI run judges c-1a\nCI passed: u-head\n"

	tr := trustingLedgers(t, map[string]string{"work-items.yaml": "gained\n"}, false, headedPull, unreadable, unreadable)
	tr.p.watcher = look(runs)
	if code, stdout, _, err := run(t, tr.p, "done", "T-1"); code != 0 || err != nil || !strings.HasPrefix(stdout, want) {
		t.Errorf("the registry alone: %d, %v, %q, want it to start %q", code, err, stdout, want)
	}

	tr = trustingLedgers(t, map[string]string{}, false, headedPull, unreadable, unreadable)
	tr.p.watcher = look(runs)
	if code, stdout, _, err := run(t, tr.p, "done", "T-1"); code != 0 || err != nil || !strings.HasPrefix(stdout, want) {
		t.Errorf("an empty diff: %d, %v, %q, want it to start %q", code, err, stdout, want)
	}

	for name, id := range map[string]string{"a slice": "slice-9", "a bug": "bug-9"} {
		tr = trustingLedgers(t, bookkeeping, false, headedPull, unreadable, ledgerOf(entry("T-1", checked)))
		var stdout, stderr strings.Builder
		m := &mergedHead{url: pullURL, head: "h-1", remote: "origin", on: "origin/main", number: 7}
		_, ok := trustedRun(t.Context(), tr.p.cfg, id, m, doneCommit{sha: "c-1a"}, look(runs), Out{Stdout: &stdout, Stderr: &stderr})
		if !ok || !strings.Contains(stdout.String(), ", so the head's CI run judges c-1a\n") || stderr.Len() != 0 {
			t.Errorf("%s: %v, %q, %q", name, ok, stdout.String(), stderr.String())
		}
	}
}
