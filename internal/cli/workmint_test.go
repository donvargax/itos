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

func TestWorkArgsOptionalIDDistinguishesMissingAndExplicitIDs(t *testing.T) {
	flags := []string{"--kind", "--title", "--why"}
	id, hasID, values, err := workArgsOptionalID("add", []string{"--kind", "slice", "--title", "Slice", "--why", "Because."}, flags...)
	if err != nil || id != "" || hasID || values["--kind"] != "slice" {
		t.Fatalf("no-ID parse = (%q, %t, %v, %v)", id, hasID, values, err)
	}
	id, hasID, _, err = workArgsOptionalID("add", []string{"p1-idea", "--kind", "idea", "--title", "Idea", "--why", "Because."}, flags...)
	if err != nil || id != "p1-idea" || !hasID {
		t.Fatalf("idea-ID parse = (%q, %t, %v)", id, hasID, err)
	}
	if _, _, _, err := workArgsOptionalID("add", []string{"slice-1", "slice-2", "--kind", "slice"}, flags...); err == nil {
		t.Fatal("workArgsOptionalID accepted two positional IDs")
	}
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

func TestWorkAddRejectsInvalidKindMissingIdeaIDAndBlankFields(t *testing.T) {
	for _, test := range []struct {
		name, want string
		args       []string
	}{
		{name: "unknown flag", args: []string{"--unknown", "x"}, want: "does not take --unknown"},
		{name: "missing title", args: []string{"--kind", "slice", "--why", "Because."}, want: "needs --title"},
		{name: "empty title", args: []string{"--kind", "slice", "--title", "", "--why", "Because."}, want: "takes one --title"},
		{name: "empty why", args: []string{"--kind", "slice", "--title", "Slice", "--why", ""}, want: "takes one --why"},
		{name: "whitespace why", args: []string{"--kind", "slice", "--title", "Slice", "--why", " "}, want: "needs --title"},
		{name: "unknown kind", args: []string{"--kind", "bug", "--title", "Bug", "--why", "Because."}, want: "takes --kind"},
		{name: "idea without an author id", args: []string{"--kind", "idea", "--title", "Idea", "--why", "Because."}, want: "needs <idea-id>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := workAdd(test.args, Out{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("workAdd error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestWorkAddKeepsTheCallerIDForIdeas(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems: []\n")
	var stdout, stderr strings.Builder
	code, err := workAdd([]string{"p1-note", "--kind", "idea", "--title", "Note", "--why", "Keep the author's slug."}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "p1-note") {
		t.Fatalf("workAdd idea = (%d, %v), stdout=%q stderr=%q, want caller ID p1-note", code, err, stdout.String(), stderr.String())
	}
	registry, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(registry), "id: p1-note") || !strings.Contains(string(registry), "kind: idea") {
		t.Fatalf("registry after workAdd idea = %q (%v), want idea p1-note", registry, err)
	}
}

func TestWorkAddMintsSliceAndRefusesCallerID(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	code, err := workAdd([]string{"--kind", "slice", "--phase", "1", "--title", "Ten", "--why", "A slice."}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "slice-10") {
		t.Fatalf("workAdd = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	text, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(text), "id: slice-10") {
		t.Fatalf("registry after workAdd = %q (%v), want slice-10", text, err)
	}
	before := string(text)
	code, err = workAdd([]string{"slice-11", "--kind", "slice", "--phase", "1", "--title", "Eleven", "--why", "Not caller named."}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the slice id") {
		t.Fatalf("explicit-ID workAdd = (%d, %v), want usage error", code, err)
	}
	after, readErr := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if readErr != nil || string(after) != before {
		t.Fatalf("explicit-ID workAdd changed registry: %q (%v)", after, readErr)
	}

	stdout.Reset()
	stderr.Reset()
	head := gitIn(t, "rev-parse", "HEAD")
	refusingHook(t)
	code, err = workAdd([]string{"--kind", "slice", "--phase", "1", "--title", "Eleven", "--why", "A slice."}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || stdout.Len() != 0 {
		t.Fatalf("workAdd with a refusing hook = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	after, readErr = os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if readErr != nil || string(after) != before || gitIn(t, "rev-parse", "HEAD") != head {
		t.Fatalf("refused workAdd changed registry or HEAD: registry=%q err=%v", after, readErr)
	}
}

// A caller-supplied id is a usage error, refused before the registry is
// read, whatever the registry would refuse: here a phase it needs and,
// with no registry at all, the registry itself.
func TestWorkAddRefusesACallerIDBeforeReadingTheRegistry(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null, 2: null }\nitems: []\n")
	var stdout, stderr strings.Builder
	code, err := workAdd([]string{"slice-9", "--kind", "slice", "--title", "Nine", "--why", "A slice."}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the slice id") || stderr.Len() != 0 {
		t.Fatalf("workAdd without required phase and with caller ID = (%d, %v), stdout=%q stderr=%q, want the caller-ID usage error", code, err, stdout.String(), stderr.String())
	}
	if err := os.Remove("registry.yaml"); err != nil {
		t.Fatal(err)
	}
	code, err = workAdd([]string{"T-9", "--kind", "task", "--phase", "1", "--title", "Nine", "--why", "A task."}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the task id") || stderr.Len() != 0 {
		t.Fatalf("workAdd with no registry and a caller ID = (%d, %v), stderr=%q, want the caller-ID usage error", code, err, stderr.String())
	}
}

func TestWorkAddRefusesCallerIDsWhenTheyAreInvalidOrAlreadyTaken(t *testing.T) {
	cases := []struct {
		name, registry, id, kind string
	}{
		{name: "invalid task id", registry: "phases: { 1: null }\nitems: []\n", id: "not-a-task-id", kind: "task"},
		{name: "duplicate slice id", registry: "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n", id: "slice-9", kind: "slice"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			registryWriterRepo(t, test.registry)
			args := []string{test.id, "--kind", test.kind, "--phase", "1", "--title", "Numbered", "--why", "Its ID is not caller-controlled."}
			_, err := workAdd(args, Out{})
			if err == nil || !strings.Contains(err.Error(), "mints the "+test.kind+" id") {
				t.Fatalf("workAdd error = %v, want supplied-ID usage error", err)
			}
		})
	}
}

func TestWorkAddMintsFirstSliceWithAnEmptyRegistry(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems: []\n")
	var stdout, stderr strings.Builder
	code, err := workAdd([]string{"--kind", "slice", "--phase", "1", "--title", "First", "--why", "No prior slices."}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "slice-1") {
		t.Fatalf("workAdd empty registry = (%d, %v), stdout=%q stderr=%q, want slice-1", code, err, stdout.String(), stderr.String())
	}
}

func TestWorkAddPropagatesIDReservationFailures(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems: []\n")
	useFailingGit(t)
	_, err := workAdd([]string{"--kind", "slice", "--phase", "1", "--title", "Slice", "--why", "Because."}, Out{})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("workAdd reservation error = %v, want injected Git command failure", err)
	}
}

func TestWorkAddPropagatesRegistryEditErrors(t *testing.T) {
	registryWriterRepo(t, "{phases: {1: null}, items: []}\n")
	_, err := workAdd([]string{"--kind", "slice", "--phase", "1", "--title", "Slice", "--why", "Because."}, Out{})
	if err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
		t.Fatalf("workAdd registry edit error = %v, want uneditable-registry error", err)
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
	var stdout, stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "task"}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "T-002") {
		t.Fatalf("workPromote task = (%d, %v), stdout=%q stderr=%q, want T-002", code, err, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "named now by") {
		t.Fatalf("workPromote without dependents named any: %q", stdout.String())
	}
	registry, err := os.ReadFile(filepath.Join(dir, "registry.yaml"))
	if err != nil || !strings.Contains(string(registry), "id: T-002") || !strings.Contains(string(registry), "kind: task") {
		t.Fatalf("registry after task promotion = %q (%v), want task T-002", registry, err)
	}
}

func TestWorkPromoteRejectsAnUnknownKind(t *testing.T) {
	_, err := workPromote([]string{"idea", "--kind", "bug"}, Out{})
	if err == nil || !strings.Contains(err.Error(), "needs --kind") {
		t.Fatalf("workPromote error = %v, want invalid-kind usage error", err)
	}
}

func TestWorkPromoteRejectsMalformedArguments(t *testing.T) {
	for _, args := range [][]string{
		{"idea", "--unknown", "value"},
		{"idea", "--kind"},
	} {
		if _, err := workPromote(args, Out{}); err == nil {
			t.Errorf("workPromote(%q) succeeded, want argument error", args)
		}
	}
}

func TestWorkPromotePropagatesReservationFailures(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	useFailingGit(t)
	_, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("workPromote reservation error = %v, want injected Git command failure", err)
	}
}

func TestWorkPromotePropagatesRegistryEditErrors(t *testing.T) {
	registryWriterRepo(t, "{phases: {1: null}, items: [{id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified}]}\n")
	_, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{})
	if err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
		t.Fatalf("workPromote registry edit error = %v, want uneditable-registry error", err)
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
