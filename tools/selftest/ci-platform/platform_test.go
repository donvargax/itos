// Package platform holds the push workflow's three-platform coverage contract. Commands run
// against fake go/itos executables; the main job's delegated plan is read from the real planner.
// The nightly's GitHub query runs against fixture runs and jobs through real jq, without network
// access.
package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
	"go.yaml.in/yaml/v3"
)

type step struct {
	Name, ID, Uses, Run, If string
	Env                     map[string]string
}
type job struct {
	Name, If string
	RunsOn   string `yaml:"runs-on"`
	Needs    []string
	Outputs  map[string]string
	Steps    []step
}
type workflow struct{ Jobs map[string]job }

func readWorkflow(t *testing.T) workflow {
	return readWorkflowFile(t, "ci.yml")
}

func readWorkflowFile(t *testing.T, name string) workflow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(scratch.Root(t), ".github/workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var w workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func fakeEnv(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	scratch.WriteFiles(t, dir, map[string]string{
		"go": `#!/bin/sh
case "$*" in
  'test ./cmd/... ./internal/...') echo unit >> "$LOG";;
  'run ./tools/bin/ci-repo scope base head') printf '{"full":%s}\n' "$FULL";;
  'run ./tools/bin/ci-repo run base head')
    if [ "$FULL" = true ]; then printf 'unit\nnamed\n' >> "$LOG"; else echo validation >> "$LOG"; fi;;
  'run ./tools/bin/ci-repo tests base head') echo named >> "$LOG";;
  *) exit 91;;
esac
`,
		"tools/bin/itos": `#!/bin/sh
case "$*" in
  'ci run base head') printf 'unit\nnamed\n' >> "$LOG";;
  'ci run base head --tests') echo named >> "$LOG";;
  'ci plan base head --json') printf '%s\n' "$PLAN_JSON";;
  'version --json') printf '{"version":"%s"}\n' "${LAUNCHER_VERSION-8.0.0}";;
  *) exit 92;;
esac
`,
		// gh run list answers with $RUNS, gh run view with $JOBS, the run $WANT_RUN alone.
		"gh": `#!/bin/sh
[ "$GH_FAIL" != 1 ] || exit 93
sub="$1 $2" id="$3"
while [ "$1" != --jq ]; do [ "$#" -gt 0 ] || exit 94; shift; done
shift
case "$sub" in
  'run list') jq -r "$1" "$RUNS";;
  'run view') [ "$id" = "$WANT_RUN" ] || exit 96; jq -r "$1" "$JOBS";;
  *) exit 95;;
esac
`,
		"events": "",
	})
	for _, path := range []string{"go", "gh", "tools/bin/itos"} {
		if err := os.Chmod(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "tools/bin/itos"), filepath.Join(dir, "itos")); err != nil {
		t.Fatal(err)
	}
	return dir, scratch.With(scratch.OutsideEnv(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ITOS_BIN="+filepath.Join(dir, "tools/bin/itos"), "LOG="+filepath.Join(dir, "events"), "FROM=base", "TO=head", "GH_FAIL=")
}

// notDraft is the condition the plan and ci jobs carry alone, and the platform jobs carry beside
// the planner's: a draft pull request's run skips them, and marking it ready runs them (slice 179).
const (
	notDraftExpr = "github.event_name != 'pull_request' || !github.event.pull_request.draft"
	notDraft     = "${{ " + notDraftExpr + " }}"
	// A platform job runs unless the planner succeeded and said it has nothing to judge: the
	// range touches no platform path (ci plan --json's jobs.platform.runs) or needs metadata
	// validation only (ci-repo scope's full). A planner that fails or is skipped never skips it.
	plannerSaysRun = "needs.plan.result != 'success' || (needs.plan.outputs.platform != 'false' && needs.plan.outputs.full != 'false')"
	platformIf     = "${{ !cancelled() && (" + notDraftExpr + ") && (" + plannerSaysRun + ") }}"
	// The release follows a push to main whose jobs did not fail, a skipped platform job among
	// them: a fix to the plugin's Markdown touches no platform path and still releases.
	releaseIf = "${{ !failure() && !cancelled() && github.event_name == 'push' && github.ref == 'refs/heads/main' }}"
	// The shared setup of the platform jobs, whose first step selects the pinned Go.
	platformBuild = "./.github/actions/platform-build"
	goToolchain   = "./.github/actions/go-toolchain"
)

// A job of ci.yml that runs a test owner: its id, the name of its check, and its runner.
type owner struct{ id, name, runner string }

var (
	linuxOwner     = owner{"ci", "ci (ubuntu-latest)", "ubuntu-latest"}
	platformOwners = []owner{
		{"platform-macos", "platform (macos-latest)", "macos-latest"},
		{"platform-windows", "platform (windows-latest)", "windows-latest"},
	}
)

// This checks the planner, the test owners and the release, not arbitrary Actions expressions.
// Unsupported conditions on their jobs fail closed (each if: is compared whole); each selected
// command must actually invoke the expected program.
func contract(t *testing.T, w workflow) error {
	t.Helper()
	if err := plannerContract(t, w); err != nil {
		return err
	}
	for _, full := range []bool{true, false} {
		if err := scopedContract(t, w, full); err != nil {
			return err
		}
	}
	for _, id := range []string{"ci", "platform-macos", "platform-windows"} {
		if !slices.Contains(w.Jobs["release"].Needs, id) {
			return fmt.Errorf("release does not wait for %s", id)
		}
	}
	if w.Jobs["release"].If != releaseIf {
		return fmt.Errorf("release condition is %q", w.Jobs["release"].If)
	}
	return nil
}

// The planner answers once, for both platform jobs, whether the range touches a platform path
// and whether it has more than metadata to validate. It runs for every run the ci job does,
// builds itos to ask it, and reads jobs.platform.runs as true when ci.jobs does not declare the
// job. Its Scope and Jobs steps run as the workflow writes them, against fake programs.
func plannerContract(t *testing.T, w workflow) error {
	t.Helper()
	j, ok := w.Jobs["plan"]
	if !ok || j.If != notDraft || len(j.Needs) != 0 || j.RunsOn != "ubuntu-latest" {
		return fmt.Errorf("missing, conditional or misplaced plan job")
	}
	if j.Outputs["platform"] != "${{ steps.jobs.outputs.platform }}" || j.Outputs["full"] != "${{ steps.scope.outputs.full }}" {
		return fmt.Errorf("plan outputs are %v", j.Outputs)
	}
	built := false
	for _, s := range j.Steps {
		if s.Run == "tools/bin/itos version" {
			built = true
		}
	}
	if !built {
		return fmt.Errorf("plan does not build itos")
	}
	for _, tc := range []struct {
		full       bool
		plan, want string
	}{
		{true, `{"jobs":{"platform":{"runs":true,"reason":"touched"}}}`, "true"},
		{true, `{"jobs":{"platform":{"runs":false,"reason":"untouched"}}}`, "false"},
		{false, `{"jobs":{"platform":{"runs":true,"reason":"whole"}}}`, "true"},
		{false, `{"jobs":{"platform":{"runs":false,"reason":"untouched"},"ci":{"runs":true,"reason":"no-paths"}}}`, "false"},
		// A job ci.jobs does not declare is absent, and the workflow runs it.
		{true, `{"jobs":{}}`, "true"},
		{true, `{"jobs":{"other":{"runs":false,"reason":"untouched"}}}`, "true"},
	} {
		dir, env := fakeEnv(t)
		env = scratch.With(env, fmt.Sprintf("FULL=%t", tc.full), "PLAN_JSON="+tc.plan, "GITHUB_OUTPUT="+filepath.Join(dir, "output"))
		scoped, planned := false, false
		for _, s := range j.Steps {
			switch s.Name {
			case "Scope":
				if s.ID != "scope" || s.If != "" || planned {
					return fmt.Errorf("plan scope must run unconditionally, first")
				}
				scoped = true
			case "Jobs":
				if s.ID != "jobs" || s.If != "" || !scoped {
					return fmt.Errorf("plan jobs must run unconditionally, after scope")
				}
				planned = true
			default:
				continue
			}
			if r := scratch.Run(dir, env, "", "bash", "-eo", "pipefail", "-c", s.Run); r.Code != 0 {
				return fmt.Errorf("plan %s: %s", s.Name, r.Output())
			}
		}
		output, _ := os.ReadFile(filepath.Join(dir, "output"))
		if want := fmt.Sprintf("full=%t\nplatform=%s\n", tc.full, tc.want); string(output) != want {
			return fmt.Errorf("plan outputs for %s: %q, want %q", tc.plan, output, want)
		}
	}
	return nil
}

func scopedContract(t *testing.T, w workflow, full bool) error {
	t.Helper()
	counts := map[string]int{}
	for _, o := range append([]owner{linuxOwner}, platformOwners...) {
		j, ok := w.Jobs[o.id]
		wantIf, wantNeeds := notDraft, []string(nil)
		if o.id != linuxOwner.id {
			wantIf, wantNeeds = platformIf, []string{"plan"}
		}
		if !ok || j.Name != o.name || j.If != wantIf || !slices.Equal(j.Needs, wantNeeds) {
			return fmt.Errorf("missing, renamed or conditional %s job", o.id)
		}
		if j.RunsOn != o.runner {
			return fmt.Errorf("%s runs on %s, not %s", o.id, j.RunsOn, o.runner)
		}
		runner := j.RunsOn
		counts[runner]++
		dir, env := fakeEnv(t)
		env = scratch.With(env, fmt.Sprintf("FULL=%t", full), "GITHUB_OUTPUT="+filepath.Join(dir, "output"))
		shared, scoped, marked := false, false, false
		installed, checked := false, false
		for i, s := range j.Steps {
			if s.Uses == platformBuild {
				if o.id == linuxOwner.id || i != 1 || s.If != "" {
					return fmt.Errorf("%s shared setup moved", o.id)
				}
				shared = true
			}
			if s.Name == "Scope" {
				if s.ID != "scope" || s.If != "" || (o.id != linuxOwner.id && !shared) {
					return fmt.Errorf("scope must run unconditionally, after the shared setup")
				}
				result := scratch.Run(dir, env, "", "bash", "-eo", "pipefail", "-c", s.Run)
				output, _ := os.ReadFile(filepath.Join(dir, "output"))
				if result.Code != 0 || string(output) != fmt.Sprintf("full=%t\n", full) {
					return fmt.Errorf("scope output: %s %s", result.Output(), output)
				}
				scoped = true
			}
			if s.Name == "Validation-only range" {
				if !scoped || s.If != "${{ steps.scope.outputs.full == 'false' }}" {
					return fmt.Errorf("metadata marker is not scope guarded")
				}
				marked = true
			}
			if s.Uses == "./" {
				if s.ID != "launcher" || !scoped || s.If != "${{ !cancelled() && steps.scope.outputs.full == 'true' }}" {
					return fmt.Errorf("%s launcher install guard changed", runner)
				}
				installed = true
			}
			selected := s.Name == "Unit tests" || s.Name == "Named tests" || s.Name == "The launcher" || s.Name == "Plan"
			if !selected {
				continue
			}
			if !scoped {
				return fmt.Errorf("%s tests run before scope", runner)
			}
			if s.Name == "Unit tests" && s.If != "${{ steps.scope.outputs.full == 'true' }}" {
				return fmt.Errorf("%s unit tests lack scope guard", runner)
			}
			if s.Name == "Named tests" && s.If != "${{ !cancelled() && steps.scope.outputs.full == 'true' }}" {
				return fmt.Errorf("%s named-test guard changed", runner)
			}
			if o.id == linuxOwner.id && s.Name != "The launcher" && s.If != "" {
				return fmt.Errorf("Linux plan is conditional")
			}
			if s.Name == "The launcher" {
				if !installed || s.If != "${{ !cancelled() && steps.scope.outputs.full == 'true' && steps.launcher.outcome == 'success' }}" {
					return fmt.Errorf("%s launcher check has no successful install", runner)
				}
				checked = true
				if !full {
					continue
				}
				bad := scratch.Run(dir, scratch.With(env, "LAUNCHER_VERSION=invalid"), "", "bash", "-eo", "pipefail", "-c", s.Run)
				if bad.Code == 0 {
					return fmt.Errorf("%s launcher check accepts an invalid version", runner)
				}
			}
			if !full && s.Name != "Plan" {
				continue
			}
			r := scratch.Run(dir, env, "", "bash", "-eo", "pipefail", "-c", s.Run)
			if r.Code != 0 {
				return fmt.Errorf("%s %s: %s", runner, s.Name, r.Output())
			}
		}
		events, err := os.ReadFile(filepath.Join(dir, "events"))
		if err != nil {
			return err
		}
		want := "unit\nnamed\n"
		if !full {
			want = ""
			if o.id == linuxOwner.id {
				want = "validation\n"
			}
		}
		if string(events) != want {
			return fmt.Errorf("%s test invocations: %q", runner, events)
		}
		if !checked || !marked || (o.id != linuxOwner.id && !shared) {
			return fmt.Errorf("%s missing launcher check", runner)
		}
	}
	for _, runner := range []string{"ubuntu-latest", "macos-latest", "windows-latest"} {
		if counts[runner] != 1 {
			return fmt.Errorf("%s has %d test owners, want one", runner, counts[runner])
		}
	}
	return nil
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	t.Run("platform Go uses the exact verified toolchain", testToolchain)
	t.Run("the nightly runs every feature once a platform, in shards where it needs them", testNightlyShards)
	t.Run("the release publishes through a GitHub 5xx and uploads over a rerun's asset", testReleasePublish)
	t.Run("one test owner and launcher check per platform", func(t *testing.T) {
		if err := contract(t, readWorkflow(t)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a newer head cancels a pull request's run and never main's", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(scratch.Root(t), ".github/workflows/ci.yml"))
		if err != nil {
			t.Fatal(err)
		}
		var w struct {
			Concurrency struct {
				Group            string
				CancelInProgress string `yaml:"cancel-in-progress"`
			}
		}
		if err := yaml.Unmarshal(data, &w); err != nil {
			t.Fatal(err)
		}
		if w.Concurrency.Group != "ci-${{ github.ref }}" || w.Concurrency.CancelInProgress != "${{ github.event_name == 'pull_request' }}" {
			t.Fatalf("concurrency is %+v", w.Concurrency)
		}
	})
	t.Run("the main plan runs core units and named tests", func(t *testing.T) {
		plan := scratch.CIPlan(t, scratch.Root(t), "", "HEAD")
		units, named := 0, 0
		for _, command := range plan.Steps {
			if command == "go test ./cmd/... ./internal/..." {
				units++
			}
			if strings.HasPrefix(command, "go test ./features -count=1") {
				named++
			}
		}
		if units != 1 || named != 1 {
			t.Fatalf("plan has %d core unit runs, %d named runs", units, named)
		}
	})
	for _, remove := range []string{"ubuntu-latest", "macos-latest", "windows-latest", "launcher", "launcher version", "Linux launcher", "Linux launcher version", "unit", "named", "release", "duplicate Linux", "no draft condition", "another condition"} {
		t.Run("reject missing coverage "+remove, func(t *testing.T) {
			w := readWorkflow(t)
			j := w.Jobs["platform-macos"]
			switch remove {
			case "ubuntu-latest":
				delete(w.Jobs, "ci")
			case "macos-latest":
				delete(w.Jobs, "platform-macos")
			case "windows-latest":
				delete(w.Jobs, "platform-windows")
			case "launcher":
				j.Steps = slices.DeleteFunc(j.Steps, func(s step) bool { return s.Uses == "./" })
			case "launcher version":
				j.Steps = slices.DeleteFunc(j.Steps, func(s step) bool { return s.Name == "The launcher" })
			case "Linux launcher", "Linux launcher version":
				linux := w.Jobs["ci"]
				linux.Steps = slices.DeleteFunc(linux.Steps, func(s step) bool {
					return (remove == "Linux launcher" && s.Uses == "./") || (remove == "Linux launcher version" && s.Name == "The launcher")
				})
				w.Jobs["ci"] = linux
			case "unit":
				j.Steps = slices.DeleteFunc(j.Steps, func(s step) bool { return s.Name == "Unit tests" })
			case "named":
				j.Steps = slices.DeleteFunc(j.Steps, func(s step) bool { return s.Name == "Named tests" })
			case "release":
				delete(w.Jobs, "release")
			case "duplicate Linux":
				j.RunsOn = "ubuntu-latest"
			case "no draft condition":
				j.If = ""
			case "another condition":
				linux := w.Jobs["ci"]
				linux.If = "${{ github.event_name != 'pull_request' }}"
				w.Jobs["ci"] = linux
			}
			if remove != "macos-latest" {
				w.Jobs["platform-macos"] = j
			}
			if err := contract(t, w); err == nil {
				t.Fatal("coverage loss accepted")
			}
		})
	}
	// The planner, the skip and the release (slice 202): each change below would skip a platform
	// job a failed planner left unjudged, run the planner for a draft, lose what it answers, or
	// stop a release for a platform job it skipped.
	for _, tc := range []struct {
		name   string
		change func(w workflow)
	}{
		{"planner removed", func(w workflow) { delete(w.Jobs, "plan") }},
		{"planner without the draft condition", func(w workflow) {
			j := w.Jobs["plan"]
			j.If = ""
			w.Jobs["plan"] = j
		}},
		{"planner with another condition", func(w workflow) {
			j := w.Jobs["plan"]
			j.If = "${{ github.event_name != 'pull_request' }}"
			w.Jobs["plan"] = j
		}},
		{"planner without its platform output", func(w workflow) {
			j := w.Jobs["plan"]
			delete(j.Outputs, "platform")
			w.Jobs["plan"] = j
		}},
		{"planner reads another job", func(w workflow) {
			j := w.Jobs["plan"]
			j.Steps = slices.Clone(j.Steps)
			for i, s := range j.Steps {
				if s.Name == "Jobs" {
					j.Steps[i].Run = strings.ReplaceAll(s.Run, ".jobs.platform", ".jobs.ci")
				}
			}
			w.Jobs["plan"] = j
		}},
		{"planner answers false for an absent job", func(w workflow) {
			j := w.Jobs["plan"]
			j.Steps = slices.Clone(j.Steps)
			for i, s := range j.Steps {
				if s.Name == "Jobs" {
					j.Steps[i].Run = strings.ReplaceAll(s.Run, "else true", "else false")
				}
			}
			w.Jobs["plan"] = j
		}},
		{"planner whose runs: false reads as true", func(w workflow) {
			j := w.Jobs["plan"]
			j.Steps = slices.Clone(j.Steps)
			for i, s := range j.Steps {
				if s.Name == "Jobs" {
					j.Steps[i].Run = strings.ReplaceAll(s.Run, "if .jobs.platform then .jobs.platform.runs else true end", ".jobs.platform.runs // true")
				}
			}
			w.Jobs["plan"] = j
		}},
		{"platform skips on a failed planner", func(w workflow) {
			for _, id := range []string{"platform-macos", "platform-windows"} {
				j := w.Jobs[id]
				j.If = "${{ (" + notDraftExpr + ") && (" + plannerSaysRun + ") }}"
				w.Jobs[id] = j
			}
		}},
		{"platform requires the planner to succeed", func(w workflow) {
			j := w.Jobs["platform-windows"]
			j.If = "${{ !cancelled() && (" + notDraftExpr + ") && needs.plan.result == 'success' && needs.plan.outputs.platform != 'false' }}"
			w.Jobs["platform-windows"] = j
		}},
		{"platform ignores the planner", func(w workflow) {
			j := w.Jobs["platform-macos"]
			j.If = notDraft
			w.Jobs["platform-macos"] = j
		}},
		{"platform without the planner in needs", func(w workflow) {
			j := w.Jobs["platform-windows"]
			j.Needs = nil
			w.Jobs["platform-windows"] = j
		}},
		{"platform renamed", func(w workflow) {
			j := w.Jobs["platform-macos"]
			j.Name = "platform (${{ matrix.os }})"
			w.Jobs["platform-macos"] = j
		}},
		{"platform without the shared setup", func(w workflow) {
			j := w.Jobs["platform-windows"]
			j.Steps = slices.DeleteFunc(slices.Clone(j.Steps), func(s step) bool { return s.Uses == platformBuild })
			w.Jobs["platform-windows"] = j
		}},
		{"release without the push-to-main condition", func(w workflow) {
			j := w.Jobs["release"]
			j.If = "${{ !failure() && !cancelled() }}"
			w.Jobs["release"] = j
		}},
		{"release skipped with a skipped platform job", func(w workflow) {
			j := w.Jobs["release"]
			j.If = "${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}"
			w.Jobs["release"] = j
		}},
		{"release that ignores a failed job", func(w workflow) {
			j := w.Jobs["release"]
			j.If = "${{ always() && github.event_name == 'push' && github.ref == 'refs/heads/main' }}"
			w.Jobs["release"] = j
		}},
		{"release without a platform job in needs", func(w workflow) {
			j := w.Jobs["release"]
			j.Needs = []string{"ci", "platform-macos"}
			w.Jobs["release"] = j
		}},
	} {
		t.Run("reject "+tc.name, func(t *testing.T) {
			w := readWorkflow(t)
			tc.change(w)
			if err := contract(t, w); err == nil {
				t.Fatal("change accepted")
			}
		})
	}
	t.Run("the shared setup selects the pinned Go first, then builds itos and reads the range", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(scratch.Root(t), platformBuild, "action.yml"))
		if err != nil {
			t.Fatal(err)
		}
		var action struct{ Runs struct{ Steps []step } }
		if err := yaml.Unmarshal(data, &action); err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, s := range action.Runs.Steps {
			names = append(names, s.Name)
		}
		steps := action.Runs.Steps
		if len(steps) != 3 || steps[0].Uses != goToolchain || steps[0].If != "" || !slices.Equal(names[1:], []string{"Build itos", "Range"}) || steps[1].ID != "build" {
			t.Fatalf("the shared setup's steps: %+v", steps)
		}
	})
	t.Run("nightly requires a successful job for each platform", func(t *testing.T) {
		plan := scratch.CIPlan(t, scratch.Root(t), "--nightly")
		var command string
		for _, s := range plan.Steps {
			if strings.Contains(s, "gh run view") {
				command = s
			}
		}
		if command == "" {
			t.Fatal("nightly has no latest CI job check")
		}
		w := readWorkflow(t)
		names := []string{w.Jobs["ci"].Name, w.Jobs["platform-macos"].Name, w.Jobs["platform-windows"].Name}
		planner := w.Jobs["plan"].Name
		// ci.yml's newest runs when the nightly of 2026-10-22 (run 39773321788) read them, newest
		// first, under a pull request's run: one still going, two cancelled while they waited, then
		// the run to judge (T-277).
		runs := []string{
			`{"databaseId": 39773700000, "headBranch": "slice-1", "event": "pull_request", "status": "completed", "conclusion": "success"}`,
			`{"databaseId": 39773576645, "headBranch": "main", "event": "push", "status": "in_progress", "conclusion": ""}`,
			`{"databaseId": 39773494732, "headBranch": "main", "event": "push", "status": "completed", "conclusion": "cancelled"}`,
			`{"databaseId": 39773386143, "headBranch": "main", "event": "push", "status": "completed", "conclusion": "cancelled"}`,
			`{"databaseId": 39772979146, "headBranch": "main", "event": "push", "status": "completed", "conclusion": "success"}`,
		}
		const none = "No completed CI run on main among the newest 100 runs of ci.yml, so its platform jobs cannot be judged."
		type fake struct {
			name, conclusion string
			steps            []string
		}
		launcher := []string{"Install the launcher", "The launcher"}
		// A full run when the planner has just succeeded: the ci job's plan, each platform's unit and
		// named tests. A metadata run replaces each job's steps by the marker.
		green := func() []*fake {
			return []*fake{
				{names[0], "success", append(slices.Clone(launcher), "Plan")},
				{names[1], "success", append(slices.Clone(launcher), "Unit tests", "Named tests")},
				{names[2], "success", append(slices.Clone(launcher), "Unit tests", "Named tests")},
				{planner, "success", nil},
			}
		}
		metadata := func(jobs []*fake, which ...int) {
			for _, i := range which {
				jobs[i].steps = []string{"Validation-only range", "Plan"}
			}
		}
		// A platform job its planner skipped has no step and the conclusion skipped.
		skipPlatforms := func(jobs []*fake) {
			for _, i := range []int{1, 2} {
				jobs[i].conclusion, jobs[i].steps = "skipped", nil
			}
		}
		without := func(jobs []*fake, i int) []*fake { return slices.Delete(jobs, i, i+1) }
		withoutStep := func(job *fake, name string) {
			job.steps = slices.DeleteFunc(job.steps, func(s string) bool { return s == name })
		}
		for _, tc := range []struct {
			mode   string
			change func(jobs []*fake) []*fake
			ok     bool
		}{
			{"green", func(j []*fake) []*fake { return j }, true},
			{"missing Linux", func(j []*fake) []*fake { return without(j, 0) }, false},
			{"missing macOS", func(j []*fake) []*fake { return without(j, 1) }, false},
			{"missing Windows", func(j []*fake) []*fake { return without(j, 2) }, false},
			{"failed", func(j []*fake) []*fake { j[0].conclusion = "failed"; return j }, false},
			{"skipped", func(j []*fake) []*fake { j[0].conclusion = "skipped"; return j }, false},
			{"duplicate macOS", func(j []*fake) []*fake { j[2].name = names[1]; return j }, false},
			{"API failure", func(j []*fake) []*fake { return j }, false},
			{"metadata", func(j []*fake) []*fake { metadata(j, 0, 1, 2); return j }, true},
			{"mixed metadata", func(j []*fake) []*fake { metadata(j, 0); return j }, false},
			{"missing named tests", func(j []*fake) []*fake { withoutStep(j[1], "Named tests"); return j }, false},
			{"missing launcher", func(j []*fake) []*fake { withoutStep(j[2], "Install the launcher"); return j }, false},
			{"no completed run", func(j []*fake) []*fake { return j }, false},
			{"no run", func(j []*fake) []*fake { return j }, false},
			// A run from before the planner has none, and its platform jobs ran.
			{"before the planner", func(j []*fake) []*fake { return without(j, 3) }, true},
			// The planner skips the platform jobs of a range that touches no platform path (slice 202).
			{"platform skipped", func(j []*fake) []*fake { skipPlatforms(j); return j }, true},
			{"one platform skipped", func(j []*fake) []*fake { j[1].conclusion, j[1].steps = "skipped", nil; return j }, true},
			{"platform skipped, metadata", func(j []*fake) []*fake { skipPlatforms(j); metadata(j, 0); return j }, true},
			{"platform skipped, planner failed", func(j []*fake) []*fake { skipPlatforms(j); j[3].conclusion = "failure"; return j }, false},
			{"platform skipped, planner skipped", func(j []*fake) []*fake { skipPlatforms(j); j[3].conclusion = "skipped"; return j }, false},
			{"platform skipped, no planner", func(j []*fake) []*fake { skipPlatforms(j); return without(j, 3) }, false},
			{"platform skipped, Linux missing its launcher", func(j []*fake) []*fake {
				skipPlatforms(j)
				withoutStep(j[0], "The launcher")
				return j
			}, false},
			{"platform skipped, Linux skipped", func(j []*fake) []*fake { skipPlatforms(j); j[0].conclusion, j[0].steps = "skipped", nil; return j }, false},
			{"platform cancelled", func(j []*fake) []*fake { j[1].conclusion, j[1].steps = "cancelled", nil; return j }, false},
			{"platform failed after the planner", func(j []*fake) []*fake { j[2].conclusion = "failure"; return j }, false},
		} {
			t.Run(tc.mode, func(t *testing.T) {
				dir, env := fakeEnv(t)
				var jobs []map[string]any
				for _, f := range tc.change(green()) {
					var steps []map[string]string
					for _, name := range f.steps {
						steps = append(steps, map[string]string{"name": name, "conclusion": "success"})
					}
					jobs = append(jobs, map[string]any{"name": f.name, "conclusion": f.conclusion, "steps": steps})
				}
				data, _ := json.Marshal(map[string]any{"jobs": jobs})
				list := runs
				switch tc.mode {
				case "no completed run": // none on main both completed and run
					list = runs[:4]
				case "no run": // the empty list the nightly of 2026-10-22 (run 39817143873) was given
					list = nil
				}
				scratch.WriteFiles(t, dir, map[string]string{"jobs.json": string(data), "runs.json": "[" + strings.Join(list, ",") + "]"})
				env = scratch.With(env, "JOBS="+filepath.Join(dir, "jobs.json"), "RUNS="+filepath.Join(dir, "runs.json"), "WANT_RUN=39772979146")
				if tc.mode == "API failure" {
					env = scratch.With(env, "GH_FAIL=1")
				}
				r := scratch.Run(dir, env, "", "sh", "-c", command)
				if (r.Code == 0) != tc.ok {
					t.Fatalf("%s: exit %d\n%s", tc.mode, r.Code, r.Output())
				}
				if said := strings.Contains(r.Output(), none); said != (tc.mode == "no completed run" || tc.mode == "no run") || (said && r.Code != 1) {
					t.Fatalf("%s: exit %d\n%s", tc.mode, r.Code, r.Output())
				}
			})
		}
	})
}

// Execute the workflow's own scripts against a fake bootstrap Go. A newer bundled version must
// never replace the pin; CRLF and native Windows cache paths survive the environment handoff.
func testToolchain(t *testing.T) {
	const actionPath = "./.github/actions/go-toolchain"
	for file, ids := range map[string][]string{
		"ci.yml":      {"ci", "plan", "platform-macos", "platform-windows"},
		"nightly.yml": {"nightly", "platform", "report"},
		"release.yml": {"release"},
	} {
		w := readWorkflowFile(t, file)
		for id, j := range w.Jobs {
			checkedOut, setup := false, 0
			for _, s := range j.Steps {
				if strings.HasPrefix(s.Uses, "actions/setup-go@") {
					t.Fatalf("%s %s still predownloads setup-go", file, id)
				}
				if strings.HasPrefix(s.Uses, "actions/checkout@") {
					checkedOut = true
				}
				// The platform jobs set Go up in their shared action, whose first step is the pinned
				// toolchain, checked below.
				if s.Uses == actionPath || s.Uses == platformBuild {
					if !checkedOut || s.If != "" {
						t.Fatalf("%s %s must set up Go unconditionally after checkout", file, id)
					}
					setup++
				}
				if slices.Contains(ids, id) && s.Run != "" && setup != 1 {
					t.Fatalf("%s %s runs commands before Go setup", file, id)
				}
			}
			if slices.Contains(ids, id) && setup != 1 {
				t.Fatalf("%s %s has %d shared Go setups", file, id, setup)
			}
		}
		for _, id := range ids {
			if _, ok := w.Jobs[id]; !ok {
				t.Fatalf("%s has no %s job", file, id)
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(scratch.Root(t), actionPath, "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Runs struct {
			Using string
			Steps []step
		}
	}
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatal(err)
	}
	if action.Runs.Using != "composite" {
		t.Fatal("Go setup is not a composite action")
	}
	selectAt, verifyAt, cacheAt := -1, -1, -1
	var selectGo, verifyGo step
	for i, s := range action.Runs.Steps {
		if strings.HasPrefix(s.Uses, "actions/setup-go@") {
			t.Fatal("shared action still predownloads setup-go")
		}
		switch s.Name {
		case "Go toolchain":
			selectAt, selectGo = i, s
		case "Verify Go toolchain":
			verifyAt, verifyGo = i, s
		case "Go cache":
			cacheAt = i
			if s.Uses != "actions/cache@0057852bfaa89a56745cba8c7296529d2fc39830" {
				t.Fatal("shared setup lost its pinned restore/post-job-save action")
			}
		}
	}
	if selectAt < 0 || cacheAt <= selectAt || verifyAt <= cacheAt || selectGo.If != "" || verifyGo.If != "" {
		t.Fatal("shared setup must select the toolchain, restore cache, and verify unconditionally")
	}
	for _, tc := range []struct {
		name, pin, reported                     string
		failGo, ok                              bool
		cached, wrongArch, missingCache         bool
		windowsRoot, wrongLocal, wrongLocalArch bool
		windowsSlash, conversionFail            bool
	}{
		{name: "exact version", pin: "toolchain go1.27.2\n", ok: true},
		{name: "Windows drive root and CRLF", windowsRoot: true, pin: "toolchain go1.27.2\r\n", ok: true},
		{name: "Windows drive with forward slashes", windowsRoot: true, windowsSlash: true, pin: "toolchain go1.27.2\n", ok: true},
		{name: "Windows conversion failure", windowsRoot: true, conversionFail: true, pin: "toolchain go1.27.2\n"},
		{name: "release candidate", pin: "toolchain go1.28rc1\n", ok: true},
		{name: "missing pin"},
		{name: "offline child selects wrong version", pin: "toolchain go1.27.2\n", wrongLocal: true},
		{name: "offline child selects wrong architecture", pin: "toolchain go1.27.2\n", wrongLocalArch: true},
		{name: "language version is not a toolchain", pin: "toolchain go1.27\n"},
		{name: "automatic selection is not exact", pin: "toolchain go1.27.2+auto\n"},
		{name: "duplicate pin", pin: "toolchain go1.27.2\ntoolchain go1.27.2\n"},
		{name: "newer Go refused", pin: "toolchain go1.27.2\n", reported: "go1.28.0"},
		{name: "download failure", pin: "toolchain go1.27.2\n", failGo: true},
		{name: "native Go only in tool cache", pin: "toolchain go1.27.2\n", cached: true, ok: true},
		{name: "wrong architecture refused", pin: "toolchain go1.27.2\n", wrongArch: true},
		{name: "missing native cache refused", pin: "toolchain go1.27.2\n", cached: true, missingCache: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, env := fakeEnv(t)
			scratch.WriteFiles(t, dir, map[string]string{
				"go.mod":     "module example.test/platform\ngo 1.27\n" + tc.pin,
				"github-env": "", "github-output": "", "github-path": "",
				"go": `#!/bin/sh
case "$*" in
  'env GOHOSTARCH')
    [ "$GOTOOLCHAIN" = local ] || exit 80
    printf '%s\r\n' "$HOST_ARCH";;
  'env GOMODCACHE'|'env GOCACHE')
    [ "$GOTOOLCHAIN" = local ] || exit 81
    printf 'C:\\Users\\runner\\%s\r\n' "$2";;
  'env GOVERSION')
    [ "$FAIL_GO" != true ] || exit 82
    [ "$GOTOOLCHAIN" = "$EXPECTED" ] || exit 83
    printf '%s\r\n' "${REPORTED:-$GOTOOLCHAIN}";;
  'env GOROOT') printf '%s\r\n' "$SELECTED_ROOT";;
  version) echo "go version $GOTOOLCHAIN";;
  *) exit 84;;
esac
`,
			})
			// The bootstrap delegates to the pin only while GOTOOLCHAIN names it. Mutation
			// preparation forces local and offline in a fresh checkout, so PATH must resolve
			// the downloaded toolchain directly, independently of the parent environment.
			selectedRoot := filepath.Join(dir, "downloaded toolchain")
			selectedBin := filepath.Join(selectedRoot, "bin")
			scratch.WriteFiles(t, dir, map[string]string{
				"downloaded toolchain/bin/go": `#!/bin/sh
case "$*" in
  'env GOVERSION')
    [ "$GOTOOLCHAIN" = local ] || exit 85
    [ "$WRONG_LOCAL" != true ] || { echo go1.26.8; exit 0; }
    printf '%s\r\n' "$EXPECTED";;
  'env GOHOSTARCH')
    [ "$WRONG_LOCAL_ARCH" != true ] || { echo amd64; exit 0; }
    printf '%s\r\n' "$HOST_ARCH";;
  'list ./...')
    [ "$GOTOOLCHAIN" = local ] && [ "$GOPROXY" = off ] && [ "$GOSUMDB" = off ] || exit 86
    printf 'example.test/platform\n';;
  version) echo "go version $EXPECTED";;
  *) exit 87;;
esac
`,
				"frozen/go.mod": "module example.test/platform\ngo 1.27\n" + tc.pin,
			})
			if err := os.Chmod(filepath.Join(selectedBin, "go"), 0o755); err != nil {
				t.Fatal(err)
			}
			reportedRoot, exportedBin := selectedRoot, selectedBin
			runnerOS := "Linux"
			if tc.windowsRoot {
				runnerOS = "Windows"
				// Model a real drive: the native path is executable from this cwd, but
				// its colon splits Bash PATH. cygpath maps it to the actual shell path.
				reportedRoot = "C:/downloaded toolchain"
				exportedBin = reportedRoot + "/bin"
				if err := os.Mkdir(filepath.Join(dir, "C:"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(selectedRoot, filepath.Join(dir, "C:", "downloaded toolchain")); err != nil {
					t.Fatal(err)
				}
				if !tc.windowsSlash {
					reportedRoot = strings.ReplaceAll(reportedRoot, "/", `\`)
				}
				scratch.WriteFiles(t, dir, map[string]string{"cygpath": `#!/bin/sh
[ "$CONVERSION_FAIL" != true ] || exit 88
[ "$1" = -u ] && [ "$2" = 'C:/downloaded toolchain/bin' ] || exit 89
printf '%s\n' "$SHELL_BIN"
`})
				if err := os.Chmod(filepath.Join(dir, "cygpath"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			// Isolate PATH from the host's Go, but retain the two external shell utilities.
			for _, name := range []string{"awk", "tr"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(dir, name)); err != nil {
					t.Fatal(err)
				}
			}
			cache := filepath.Join(dir, "tool cache")
			cachedBin := filepath.Join(cache, "go", "1.26.8", "arm64", "bin")
			if tc.cached {
				if tc.missingCache {
					cachedBin = filepath.Join(cache, "go", "1.26.8", "x64", "bin")
				}
				if err := os.MkdirAll(cachedBin, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(dir, "go"), filepath.Join(cachedBin, "go")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Dir(cachedBin)+".complete", nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			hostArch := "arm64"
			if tc.wrongArch {
				hostArch = "amd64"
			}
			env = scratch.With(env, "GITHUB_ENV="+filepath.Join(dir, "github-env"),
				"GITHUB_PATH="+filepath.Join(dir, "github-path"), "PATH="+dir,
				"RUNNER_ARCH=ARM64", "RUNNER_OS="+runnerOS, "RUNNER_TOOL_CACHE="+cache, "HOST_ARCH="+hostArch,
				"SHELL_BIN="+selectedBin, fmt.Sprintf("CONVERSION_FAIL=%t", tc.conversionFail),
				"GITHUB_OUTPUT="+filepath.Join(dir, "github-output"), "GOTOOLCHAIN=auto",
				"EXPECTED="+strings.TrimSpace(strings.TrimPrefix(tc.pin, "toolchain ")),
				"REPORTED="+tc.reported, fmt.Sprintf("FAIL_GO=%t", tc.failGo),
				"SELECTED_ROOT="+reportedRoot, fmt.Sprintf("WRONG_LOCAL=%t", tc.wrongLocal),
				fmt.Sprintf("WRONG_LOCAL_ARCH=%t", tc.wrongLocalArch))
			selected := scratch.Run(dir, env, "", "bash", "-eo", "pipefail", "-c", selectGo.Run)
			result := selected
			if selected.Code == 0 {
				written, err := os.ReadFile(filepath.Join(dir, "github-env"))
				if err != nil {
					t.Fatal(err)
				}
				output, err := os.ReadFile(filepath.Join(dir, "github-output"))
				if err != nil {
					t.Fatal(err)
				}
				if string(output) != "mod=C:\\Users\\runner\\GOMODCACHE\nbuild=C:\\Users\\runner\\GOCACHE\n" {
					t.Fatalf("cache paths were not preserved: %q", output)
				}
				env = scratch.With(env, strings.TrimSpace(string(written)))
				path, err := os.ReadFile(filepath.Join(dir, "github-path"))
				if err != nil {
					t.Fatal(err)
				}
				if tc.cached {
					if string(path) != cachedBin+"\n" {
						t.Fatalf("bootstrap path not exported: %q", path)
					}
					env = scratch.With(env, "PATH="+cachedBin+string(os.PathListSeparator)+dir)
				} else if len(path) != 0 {
					t.Fatalf("existing PATH Go replaced: %q", path)
				}
				result = scratch.Run(dir, env, "", "bash", "-eo", "pipefail", "-c", verifyGo.Run)
				if result.Code == 0 {
					path, err = os.ReadFile(filepath.Join(dir, "github-path"))
					if err != nil {
						t.Fatal(err)
					}
					want := exportedBin + "\n"
					if tc.cached {
						want = cachedBin + "\n" + want
					}
					if string(path) != want {
						t.Fatalf("verified toolchain not exported for offline children: got %q, want %q", path, want)
					}
					childPath := dir
					for _, bin := range strings.Split(strings.TrimSpace(string(path)), "\n") {
						// The runner imports native GITHUB_PATH entries into the next Bash.
						if tc.windowsRoot && bin == exportedBin {
							bin = selectedBin
						}
						childPath = bin + string(os.PathListSeparator) + childPath
					}
					env = scratch.With(env, "PATH="+childPath,
						"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
					result = scratch.Run(filepath.Join(dir, "frozen"), env, "", "sh", "-c", "go list ./...")
				}

			}
			if (result.Code == 0) != tc.ok {
				t.Fatalf("exit %d, wanted success=%t: %s", result.Code, tc.ok, result.Output())
			}
		})
	}
}

// The release's Publish step, run as the workflow runs it against a fake gh whose release edit
// fails its first $FAIL_EDITS times, as GitHub's 500 did on v8.25.0 (bug-143): the upload must
// replace an asset a failed run left, the publish must be retried, and one that never succeeds
// must fail the job saying the release is still a draft.
func testReleasePublish(t *testing.T) {
	var publish step
	for _, s := range readWorkflowFile(t, "release.yml").Jobs["release"].Steps {
		if s.Name == "Publish" {
			publish = s
		}
	}
	if publish.Run == "" {
		t.Fatal("release.yml has no Publish step")
	}
	for _, tc := range []struct {
		name, prerelease string
		fails, edits     int
		code             int
		flags            string
	}{
		{"a 5xx twice, then published", "false", 2, 3, 0, "--draft=false --latest"},
		{"a candidate is a pre-release and not latest", "true", 1, 2, 0, "--draft=false --prerelease --latest=false"},
		{"never published", "false", 100, 6, 1, "--draft=false --latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			scratch.WriteFiles(t, dir, map[string]string{
				"bin/gh": `#!/bin/sh
echo "$*" >> "$LOG"
if [ "$1 $2" = 'release edit' ]; then
  n=$(cat "$DIR/count" 2>/dev/null || echo 0)
  echo $((n + 1)) > "$DIR/count"
  [ "$n" -ge "$FAIL_EDITS" ] || exit 1
fi
`,
				"bin/sleep": "#!/bin/sh\n",
				"log":       "",
			})
			for _, f := range []string{"bin/gh", "bin/sleep"} {
				if err := os.Chmod(filepath.Join(dir, f), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			env := scratch.With(scratch.OutsideEnv(), "PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
				"LOG="+filepath.Join(dir, "log"), "DIR="+dir, "RUNNER_TEMP=/rt", "TAG=v1.0.0", "PRERELEASE="+tc.prerelease,
				"FAIL_EDITS="+fmt.Sprint(tc.fails))
			r := scratch.Run(dir, env, "", "bash", "-e", "-c", publish.Run)
			if r.Code != tc.code {
				t.Fatalf("exit %d, not %d:\n%s", r.Code, tc.code, r.Output())
			}
			logged, err := os.ReadFile(filepath.Join(dir, "log"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(logged)), "\n")
			if lines[0] != "release upload v1.0.0 /rt/upgrading.json --clobber" {
				t.Errorf("the first call is %q, not the upload over an existing asset", lines[0])
			}
			if len(lines)-1 != tc.edits {
				t.Errorf("%d publish attempts, not %d:\n%s", len(lines)-1, tc.edits, logged)
			}
			want := "release edit v1.0.0 --notes-file /rt/notes.md " + tc.flags
			if last := lines[len(lines)-1]; last != want {
				t.Errorf("the publish is %q, not %q", last, want)
			}
			if said := strings.Contains(r.Output(), "still a draft"); said != (tc.code == 1) {
				t.Errorf("a release left a draft should say so, and only then:\n%s", r.Output())
			}
		})
	}
}
