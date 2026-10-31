package work

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
)

// current is the revision the fake Revision port gives of every item, and
// other a well-formed one that is no item's.
const (
	current = "sha256:" + hex
	other   = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

// expecting is p with its Revision port giving current and keeping the ids
// it is asked.
func expecting(p *ports) *[]string {
	calls := []string{}
	p.revision, p.revisionCalls = current, &calls
	return &calls
}

// ruleOf is the rule of a refusal's one problem in --json's object.
func ruleOf(t *testing.T, o map[string]any) string {
	t.Helper()
	problems, _ := o["problems"].([]any)
	if len(problems) != 1 {
		t.Fatalf("the problems: %v", o)
	}
	problem, _ := problems[0].(map[string]any)
	return problem["rule"].(string)
}

// work take --expect (slice 208): the current revision takes the item,
// outcome taken; a stale one refuses it before its own refusals, nothing
// written, the message naming the current revision whole and the fix itos
// next; an item already in progress for the person is already, whatever the
// revision, which is not even asked, as it is not without --expect; an id
// next refuses is left to the take's own refusal, and what next cannot read
// is the error.
func TestTakeExpects(t *testing.T) {
	p := over(t, writable(false))
	calls := expecting(&p)
	code, stdout, _, err := run(t, p, "--json", "take", "slice-4", "--as", "a", "--expect", current)
	o := object(t, stdout)
	if code != 0 || err != nil || o["ok"] != true || o["outcome"] != "taken" || o["commit"] == nil ||
		o["item"].(map[string]any)["status"] != "doing" || !slices.Equal(*calls, []string{"slice-4"}) ||
		strings.Index(stdout, `"outcome"`) > strings.Index(stdout, `"item"`) {
		t.Errorf("work --json take slice-4 --expect current = %d, %v, %s, asked %q", code, err, stdout, *calls)
	}

	p = over(t, writable(false))
	expecting(&p)
	code, stdout, stderr, err := run(t, p, "take", "slice-4", "--as", "a", "--expect", other)
	if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "slice-4's revision is "+current+", not "+other) ||
		!strings.Contains(stderr, "  fix: run itos next slice-4 again") {
		t.Errorf("work take slice-4 --expect other = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if after := registryIn(t, p); after != writableText {
		t.Errorf("the stale take left the registry:\n%s", after)
	}
	code, stdout, _, err = run(t, p, "--json", "take", "slice-4", "--as", "a", "--expect", other)
	o = object(t, stdout)
	if code != refused || err != nil || o["ok"] != false || ruleOf(t, o) != "work-stale" {
		t.Errorf("work --json take slice-4 --expect other = %d, %v, %v", code, err, o)
	}

	// Stale precedes the take's own refusals: an idea with a stale revision
	// is stale; with the current one, it is refused as an idea.
	for revision, rule := range map[string]string{other: "work-stale", current: "work-take-idea"} {
		code, stdout, _, err = run(t, p, "--json", "take", "p1-a", "--as", "a", "--expect", revision)
		if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != rule {
			t.Errorf("work --json take p1-a --expect %s = %d, %v, %v, want %s", revision, code, err, o, rule)
		}
	}

	// Already precedes stale, and the revision is not asked.
	calls = expecting(&p)
	code, stdout, _, err = run(t, p, "--json", "take", "T-1", "--as", "a", "--expect", other)
	o = object(t, stdout)
	if code != 0 || err != nil || o["ok"] != true || o["outcome"] != "already" || o["commit"] != nil || len(*calls) != 0 {
		t.Errorf("work --json take T-1 --expect other = %d, %v, %v, asked %q", code, err, o, *calls)
	}
	if code, stdout, _, err = run(t, p, "take", "T-1", "--as", "a", "--expect", other); code != 0 || err != nil ||
		stdout != "T-1 is already in progress for a; nothing to change\n" {
		t.Errorf("work take T-1 --expect other = %d, %v, %q", code, err, stdout)
	}
	// Without --expect the revision is not asked either, and the outcome
	// is there.
	code, stdout, _, err = run(t, p, "--json", "take", "slice-4", "--as", "a")
	if o = object(t, stdout); code != 0 || err != nil || o["outcome"] != "taken" || len(*calls) != 0 {
		t.Errorf("work --json take slice-4 = %d, %v, %v, asked %q", code, err, o, *calls)
	}

	// Identity precedes both: nobody exits 3, the revision not asked.
	p = over(t, writable(false))
	calls = expecting(&p)
	if code, _, _, err = run(t, p, "take", "slice-4", "--expect", other); code != notAmong || err != nil || len(*calls) != 0 {
		t.Errorf("work take slice-4 --expect other as nobody = %d, %v, asked %q", code, err, *calls)
	}

	// An id next refuses is left to the take's own refusal.
	p.revisionRefused = &refusedRevision{id: "nope", problem: out.Problem{Rule: "work-unknown-item"}}
	code, stdout, _, err = run(t, p, "--json", "take", "nope", "--as", "a", "--expect", other)
	if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != "work-unknown-item" {
		t.Errorf("work --json take nope --expect other = %d, %v, %v", code, err, o)
	}

	// What next cannot read is the error, nothing written.
	p.revisionErr = errors.New("the questions cannot be read")
	if _, _, _, err = run(t, p, "take", "slice-4", "--as", "a", "--expect", other); !errors.Is(err, p.revisionErr) {
		t.Errorf("work take slice-4 --expect other, next failing = %v", err)
	}
	if after := registryIn(t, p); after != writableText {
		t.Errorf("the failed take left the registry:\n%s", after)
	}
}

// work done --expect (slice 208): the current revision closes the item,
// outcome done; a stale one refuses the close before the landing is
// checked (a scenario still @wip, a task's checks, which are not run) and
// before the registry's own refusals, nothing written; an item already done
// is already, whatever the revision, which is not asked, as it is not
// without --expect; an id next refuses is left to the close's own refusal,
// and what next cannot read is the error. Under push.mode pr the remote is
// not asked for a stale revision.
func TestDoneExpects(t *testing.T) {
	p := over(t, closable(false))
	calls := expecting(&p)
	code, stdout, _, err := run(t, p, "--json", "done", "slice-2", "--expect", current)
	o := object(t, stdout)
	if code != 0 || err != nil || o["ok"] != true || o["outcome"] != "done" || o["ci"] != "unwatched" || o["commit"] == nil ||
		!slices.Equal(*calls, []string{"slice-2"}) || strings.Index(stdout, `"outcome"`) > strings.Index(stdout, `"item"`) {
		t.Errorf("work --json done slice-2 --expect current = %d, %v, %s, asked %q", code, err, stdout, *calls)
	}

	p = over(t, closable(false))
	expecting(&p)
	checked := []string{}
	p.checked = &checked
	code, stdout, stderr, err := run(t, p, "done", "slice-3", "--expect", other)
	if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "slice-3's revision is "+current+", not "+other) ||
		!strings.Contains(stderr, "  fix: run itos next slice-3 again") {
		t.Errorf("work done slice-3 --expect other = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = run(t, p, "--json", "done", "T-1", "--expect", other)
	if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != "work-stale" || len(checked) != 0 {
		t.Errorf("work --json done T-1 --expect other = %d, %v, %v, checked %q", code, err, o, checked)
	}
	if after := registryIn(t, p); after != doneText {
		t.Errorf("the stale closes left the registry:\n%s", after)
	}
	if ledger := docIn(t, p, ledgerFile); ledger != staticLedgerText {
		t.Errorf("the stale close left the ledger:\n%s", ledger)
	}
	// With the current revision, slice-3 is refused for its @wip scenario.
	code, stdout, _, err = run(t, p, "--json", "done", "slice-3", "--expect", current)
	if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != "work-done-wip" {
		t.Errorf("work --json done slice-3 --expect current = %d, %v, %v", code, err, o)
	}

	// Stale precedes the registry's own refusals: an item not doing.
	f := closable(false)
	f.files["work-items.yaml"] = doneText + "  - { id: slice-5, title: Todo, phase: 1, owner: null, status: todo, depends_on: [] }\n"
	p = over(t, f)
	expecting(&p)
	for revision, rule := range map[string]string{other: "work-stale", current: "work-done-status"} {
		code, stdout, _, err = run(t, p, "--json", "done", "slice-5", "--expect", revision)
		if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != rule {
			t.Errorf("work --json done slice-5 --expect %s = %d, %v, %v, want %s", revision, code, err, o, rule)
		}
	}

	// Already precedes stale, and the revision is not asked.
	calls = expecting(&p)
	code, stdout, _, err = run(t, p, "--json", "done", "slice-1", "--expect", other)
	o = object(t, stdout)
	if _, ci := o["ci"]; code != 0 || err != nil || o["ok"] != true || o["outcome"] != "already" || o["commit"] != nil || ci || len(*calls) != 0 {
		t.Errorf("work --json done slice-1 --expect other = %d, %v, %v, asked %q", code, err, o, *calls)
	}
	if code, stdout, _, err = run(t, p, "done", "slice-1", "--expect", other); code != 0 || err != nil ||
		stdout != "slice-1 is already done; nothing to change\n" {
		t.Errorf("work done slice-1 --expect other = %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = run(t, p, "--json", "done", "slice-2")
	if o = object(t, stdout); code != 0 || err != nil || o["outcome"] != "done" || len(*calls) != 0 {
		t.Errorf("work --json done slice-2 = %d, %v, %v, asked %q", code, err, o, *calls)
	}

	// An id next refuses is left to the close's own refusal.
	p = over(t, closable(false))
	expecting(&p)
	p.revisionRefused = &refusedRevision{id: "nope", problem: out.Problem{Rule: "work-unknown-item"}}
	code, stdout, _, err = run(t, p, "--json", "done", "nope", "--expect", other)
	if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != "work-unknown-item" {
		t.Errorf("work --json done nope --expect other = %d, %v, %v", code, err, o)
	}

	// What next cannot read is the error.
	p.revisionErr = errors.New("the questions cannot be read")
	if _, _, _, err = run(t, p, "done", "slice-2", "--expect", other); !errors.Is(err, p.revisionErr) {
		t.Errorf("work done slice-2 --expect other, next failing = %v", err)
	}

	// Under push.mode pr a stale revision is judged before the remote's
	// HEAD is asked.
	p, ghCalls, heads := prDone(t, openPull)
	expecting(&p)
	vcsOf(p).OnBranch = "slice-2"
	p.pr.head = "slice-2"
	code, stdout, _, err = run(t, p, "--json", "done", "slice-2", "--expect", other)
	if o = object(t, stdout); code != refused || err != nil || ruleOf(t, o) != "work-stale" || len(*ghCalls) != 0 || len(*heads) != 0 {
		t.Errorf("work --json done slice-2 --expect other under pr = %d, %v, %v, gh %v, heads %q", code, err, o, *ghCalls, *heads)
	}
}
