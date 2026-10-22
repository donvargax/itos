package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func registryWriterRepo(t *testing.T, registry string) string {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\nwork:\n  registry: registry.yaml\n")
	standInHooks(t)
	gitIn(t, "config", "user.name", "itos test")
	gitIn(t, "config", "user.email", "test@localhost")
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "itos.yaml", "registry.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: start")
	return dir
}

func TestWorkArgsParsedRejectsMalformedFlagsAndAllowsDeclaredEmptyValues(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown", args: []string{"item", "--unknown", "value"}, want: "does not take --unknown"},
		{name: "missing final value", args: []string{"item", "--title"}, want: "--title needs a value"},
		{name: "duplicate", args: []string{"item", "--title", "one", "--title", "two"}, want: "takes one --title"},
		{name: "empty value", args: []string{"item", "--title", ""}, want: "takes one --title with a value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := workArgsParsed("edit", test.args, nil, "--title")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("workArgsParsed error = %v, want it to contain %q", err, test.want)
			}
		})
	}
	_, values, err := workArgsParsed("edit", []string{"item", "--depends-on", ""}, []string{"--depends-on"}, "--depends-on")
	if err != nil || values["--depends-on"] != "" {
		t.Fatalf("workArgsParsed declared empty value = (%v, %v), want empty depends-on", values, err)
	}
}

// work add's minting through the git the counter is kept by: a git that
// fails is the error, exit 70 (bug 119).
func TestWorkAddPropagatesIDReservationFailures(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems: []\n")
	useFailingGit(t)
	code, err := runWork([]string{"add", "--kind", "slice", "--phase", "1", "--title", "Slice", "--why", "Because."}, Out{Stderr: &strings.Builder{}})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "Command failed: git rev-parse") {
		t.Fatalf("work add = (%d, %v), want exit code 0 and the injected Git command failure", code, err)
	}
	if got := ExitCode(err); got != ExitSoftware {
		t.Fatalf("work add's reservation failure exits %d, want %d (bug 119)", got, ExitSoftware)
	}
}

// As work add's: a caller-supplied id is refused before the registry is
// read, whatever the registry would refuse.
func TestWorkPromoteRefusesACallerIDBeforeReadingTheRegistry(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: ready, title: Ready, phase: 1, owner: null, status: todo, kind: task, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"ready", "--id", "slice-1", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the slice id") || stderr.Len() != 0 {
		t.Fatalf("workPromote non-idea target with caller ID = (%d, %v), stdout=%q stderr=%q, want the caller-ID usage error", code, err, stdout.String(), stderr.String())
	}
	code, err = workPromote([]string{"ready", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), "not an idea") {
		t.Fatalf("workPromote non-idea target = (%d, %v), stderr=%q, want target policy refusal", code, err, stderr.String())
	}
	if err := os.Remove("registry.yaml"); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	code, err = workPromote([]string{"idea", "--id", "T-9", "--kind", "task"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the task id") || stderr.Len() != 0 {
		t.Fatalf("workPromote with no registry and a caller ID = (%d, %v), stderr=%q, want the caller-ID usage error", code, err, stderr.String())
	}
}

func TestWorkPromoteRefusesACallerIDAlreadyTaken(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n"+"  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, kind: slice, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--id", "slice-9", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the slice id") {
		t.Fatalf("workPromote with a taken caller ID = (%d, %v), stdout=%q stderr=%q, want usage refusal", code, err, stdout.String(), stderr.String())
	}
}

func TestWorkPromoteUnknownItemWithEmptyRegistryReturnsPolicyError(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems: []\n")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"missing", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), "missing") {
		t.Fatalf("workPromote unknown item = (%d, %v), stdout=%q stderr=%q, want policy refusal", code, err, stdout.String(), stderr.String())
	}
}

func TestWorkPromoteMintsTaskIDFromTheLedger(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, status: todo, kind: idea, why: Specified }\n"+"  - { id: T-001, title: Existing task, phase: 1, owner: null, status: done, kind: task, depends_on: [] }\n")
	ledger := filepath.Join(dir, "tasks", "phase-1.yaml")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte("- { id: T-001, type: chore, title: Existing task }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", ledger)
	gitIn(t, "commit", "-q", "-m", "docs: a ledger")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "task", "--type", "build", "--check", "true"}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || stdout.String() != "p1-idea is the task T-002 of tasks/phase-1.yaml: committed "+
		strings.TrimSpace(gitIn(t, "rev-parse", "--short=7", "HEAD"))+" docs: promote p1-idea to T-002\n" {
		t.Fatalf("workPromote task = (%d, %v), stdout=%q stderr=%q, want T-002", code, err, stdout.String(), stderr.String())
	}
	registry, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(registry), "id: T-002") || !strings.Contains(string(registry), "kind: task") ||
		strings.Contains(string(registry), "Specified") {
		t.Fatalf("registry after task promotion = %q (%v), want task T-002 with no why", registry, err)
	}
	if text, err := os.ReadFile(ledger); err != nil || !strings.HasSuffix(string(text), "- id: T-002\n  type: build\n  title: Idea\n  why: >\n    Specified\n  done_when:\n    - run: \"true\"\n") {
		t.Fatalf("ledger after task promotion = %q (%v), want T-002 with the idea's title and why", text, err)
	}
	if files := strings.Fields(gitIn(t, "show", "--name-only", "--format=", "HEAD")); strings.Join(files, " ") != "registry.yaml tasks/phase-1.yaml" {
		t.Errorf("the promote committed %q, want the registry and the ledger file", files)
	}
	if body := strings.Join(strings.Fields(gitIn(t, "log", "-1", "--format=%b")), " "); !strings.HasSuffix(body,
		"with itos work promote, and write its ledger entry, with its why and checks, in tasks/phase-1.yaml.") {
		t.Errorf("the promote's body is %q, want it to name the ledger entry", body)
	}
}

func TestWorkPromoteRejectsAnUnknownKind(t *testing.T) {
	code, err := workPromote([]string{"idea", "--kind", "bug"}, Out{})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "needs --kind") {
		t.Fatalf("workPromote = (%d, %v), want exit code 0 and the invalid-kind usage error", code, err)
	}
}

func TestWorkPromoteRejectsMalformedArguments(t *testing.T) {
	for _, args := range [][]string{
		{"idea", "--unknown", "value"},
		{"idea", "--kind"},
	} {
		if code, err := workPromote(args, Out{}); code != 0 || err == nil {
			t.Errorf("workPromote(%q) = (%d, %v), want exit code 0 and an argument error", args, code, err)
		}
	}
}

func TestWorkPromotePropagatesReservationFailures(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	// A counter that cannot be read, so the reservation, which comes after
	// every check of the promote's, fails.
	if err := os.MkdirAll(filepath.Join(dir, ".git", "itos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "itos", "ids.yaml"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{})
	if code != 0 || err == nil {
		t.Fatalf("workPromote = (%d, %v), want exit code 0 and the counter's failure", code, err)
	}
}

func TestWorkPromotePropagatesRegistryEditErrors(t *testing.T) {
	registryWriterRepo(t, "{phases: {1: null}, items: [{id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified}]}\n")
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
		t.Fatalf("workPromote = (%d, %v), want exit code 0 and the uneditable-registry error", code, err)
	}
}

func TestWorkPromoteMintsSliceAndRewritesDependencies(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, status: todo, kind: idea, why: Missing }\n"+"  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [p1-idea] }\n")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "slice-10") {
		t.Fatalf("workPromote = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "named now by slice-9") {
		t.Fatalf("workPromote output = %q, want the rewritten dependent", stdout.String())
	}
	text, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(text), "id: slice-10") || !strings.Contains(string(text), "depends_on: [slice-10]") {
		t.Fatalf("registry after workPromote = %q (%v), want promoted ID and rewritten dependency", text, err)
	}
}

func TestWorkPromoteRollsBackWhenTheCommitHookRefuses(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n"+"  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, kind: slice, depends_on: [p1-idea] }\n")
	before, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	head := gitIn(t, "rev-parse", "HEAD")
	refusingHook(t)
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || stdout.Len() != 0 {
		t.Fatalf("workPromote with a refusing hook = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	after, readErr := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if readErr != nil || string(after) != string(before) || gitIn(t, "rev-parse", "HEAD") != head {
		t.Fatalf("refused workPromote changed registry or HEAD: registry=%q err=%v", after, readErr)
	}
}

// registryRefusal is a registryWriterRepo whose registry is gone, so
// soundRegistry refuses before anything is read or minted; run, the
// command, is held to that refusal, exit ExitPolicy with no error, and
// to HEAD and the registry left as they were.
func registryRefusal(t *testing.T, run func(Out) (int, error)) {
	t.Helper()
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	if err := os.Remove("registry.yaml"); err != nil {
		t.Fatal(err)
	}
	head := gitIn(t, "rev-parse", "HEAD")
	var stdout, stderr strings.Builder
	code, err := run(Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "no work registry at registry.yaml") {
		t.Fatalf("= (%d, %v), stdout=%q stderr=%q, want the registry's refusal", code, err, stdout.String(), stderr.String())
	}
	if _, err := os.Stat("registry.yaml"); !os.IsNotExist(err) {
		t.Fatalf("the refused command wrote a registry: %v", err)
	}
	if gitIn(t, "rev-parse", "HEAD") != head {
		t.Fatal("the refused command moved HEAD")
	}
}

// As work add's: work promote returns soundRegistry's refusal and promotes
// nothing.
func TestWorkPromoteReturnsTheRegistrysRefusal(t *testing.T) {
	registryRefusal(t, func(o Out) (int, error) {
		return workPromote([]string{"p1-idea", "--kind", "slice"}, o)
	})
}

// Every question naming the idea names the new id, committed with the
// registry in the promote's one commit, whose body says which; a question
// naming another item is kept (bug 105).
func TestWorkPromoteRenamesTheIdeaInTheQuestions(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	writeFiles(t, map[string]string{"asks.yaml": "questions:\n  - { id: q-1, item: p1-idea, question: \"Why?\" }\n" +
		"  - { id: q-2, item: p1-other, question: \"How?\" }\n  - { id: q-3, item: p1-idea, question: \"When?\", answer: \"Now.\" }\n"})
	gitIn(t, "add", "--", "asks.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: ask")
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	if err != nil || code != 0 || !strings.Contains(strings.Join(strings.Fields(stdout.String()), ""), `"questions":["q-1","q-3"]`) {
		t.Fatalf("workPromote = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	asks, err := os.ReadFile("asks.yaml")
	if err != nil || strings.Count(string(asks), `item: "slice-1"`) != 2 || !strings.Contains(string(asks), `item: "p1-other"`) {
		t.Fatalf("questions after = %q (%v)", asks, err)
	}
	if files := strings.TrimSpace(gitIn(t, "show", "--name-only", "--format=", "HEAD")); files != "asks.yaml\nregistry.yaml" {
		t.Errorf("the promote committed %q, want the questions and the registry", files)
	}
	if body := strings.Join(strings.Fields(gitIn(t, "log", "-1", "--format=%b")), " "); !strings.HasSuffix(body, "with itos work promote, and rename it in q-1 and q-3 of asks.yaml.") {
		t.Errorf("the body %q does not name the questions", body)
	}
	if status := gitIn(t, "status", "--porcelain"); status != "" {
		t.Errorf("left %q", status)
	}
}

// With no question naming the idea the registry is committed alone, the
// questions as they were, and the report names none.
func TestWorkPromoteLeavesQuestionsNamingOthers(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	const asks = "questions:\n  - { id: q-1, item: p1-other, question: \"How?\" }\n"
	writeFiles(t, map[string]string{"asks.yaml": asks})
	gitIn(t, "add", "--", "asks.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: ask")
	var stdout strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &strings.Builder{}, JSON: true})
	if err != nil || code != 0 || strings.Contains(stdout.String(), "questions") {
		t.Fatalf("workPromote = (%d, %v), stdout=%q", code, err, stdout.String())
	}
	if after, err := os.ReadFile("asks.yaml"); err != nil || string(after) != asks {
		t.Errorf("questions after = %q (%v)", after, err)
	}
	if files := strings.TrimSpace(gitIn(t, "show", "--name-only", "--format=", "HEAD")); files != "registry.yaml" {
		t.Errorf("the promote committed %q, want the registry alone", files)
	}
}

// Questions that cannot be parsed refuse the promote, exit 1, nothing
// written; questions that cannot be read at all are an error.
func TestWorkPromoteRefusesQuestionsItCannotRead(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	head := gitIn(t, "rev-parse", "HEAD")
	writeFiles(t, map[string]string{"asks.yaml": "questions:\n  - { id: q-1, item: p1-idea, question: \"How?\", colour: red }\n"})
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != ExitPolicy || stdout.Len() != 0 ||
		!strings.Contains(stderr.String(), "correct asks.yaml to what itos decision writes, then promote p1-idea again") {
		t.Fatalf("workPromote = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	if gitIn(t, "rev-parse", "HEAD") != head || gitIn(t, "status", "--porcelain", "--", "registry.yaml") != "" {
		t.Fatal("the refused promote wrote or committed")
	}
	if err := os.Remove("asks.yaml"); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, map[string]string{"asks.yaml/x": ""})
	if code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr}); err == nil || code != 0 {
		t.Fatalf("questions that cannot be read: (%d, %v)", code, err)
	}
}

// One question naming the idea is reported too, in the text and as JSON.
func TestWorkPromoteReportsOneQuestion(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n"+
		"  - { id: p1-next, title: Next, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	writeFiles(t, map[string]string{"asks.yaml": "questions:\n  - { id: q-1, item: p1-idea, question: \"Why?\" }\n" +
		"  - { id: q-2, item: p1-next, question: \"How?\" }\n"})
	gitIn(t, "add", "--", "asks.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: ask")
	var stdout strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &strings.Builder{}, JSON: true})
	if err != nil || code != 0 || !strings.Contains(strings.Join(strings.Fields(stdout.String()), ""), `"questions":["q-1"]`) {
		t.Fatalf("workPromote --json = (%d, %v), stdout=%q", code, err, stdout.String())
	}
	stdout.Reset()
	code, err = workPromote([]string{"p1-next", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &strings.Builder{}})
	if err != nil || code != 0 || !strings.HasPrefix(stdout.String(), "p1-next is the slice slice-2, named now by q-2: committed ") {
		t.Fatalf("workPromote = (%d, %v), stdout=%q", code, err, stdout.String())
	}
}
