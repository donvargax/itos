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

type step struct{ Name, ID, Uses, Run, If string }
type job struct {
	Name, If string
	RunsOn   string `yaml:"runs-on"`
	Needs    []string
	Strategy struct{ Matrix struct{ OS []string } }
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

// This checks the two test owners, not arbitrary Actions expressions. Unsupported conditions
// on their jobs fail closed; each selected command must actually invoke the expected program.
func contract(t *testing.T, w workflow) error {
	t.Helper()
	for _, full := range []bool{true, false} {
		if err := scopedContract(t, w, full); err != nil {
			return err
		}
	}
	return nil
}

// notDraft is the only condition the ci and platform jobs carry: a draft pull request's run
// skips both, and marking it ready runs them (slice 179).
const notDraft = "${{ github.event_name != 'pull_request' || !github.event.pull_request.draft }}"

func scopedContract(t *testing.T, w workflow, full bool) error {
	t.Helper()
	counts := map[string]int{}
	for _, id := range []string{"ci", "platform"} {
		j, ok := w.Jobs[id]
		if !ok || j.If != notDraft || len(j.Needs) != 0 {
			return fmt.Errorf("missing or conditional %s job", id)
		}
		oses := []string{j.RunsOn}
		if id == "platform" {
			if j.RunsOn != "${{ matrix.os }}" {
				return fmt.Errorf("platform does not use its OS matrix")
			}
			oses = j.Strategy.Matrix.OS
		}
		for _, runner := range oses {
			counts[runner]++
			dir, env := fakeEnv(t)
			env = scratch.With(env, fmt.Sprintf("FULL=%t", full), "GITHUB_OUTPUT="+filepath.Join(dir, "output"))
			scoped, marked := false, false
			installed, checked := false, false
			for _, s := range j.Steps {
				if s.Name == "Scope" {
					if s.ID != "scope" || s.If != "" {
						return fmt.Errorf("scope must run unconditionally")
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
				if id == "ci" && s.Name != "The launcher" && s.If != "" {
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
				if id == "ci" {
					want = "validation\n"
				}
			}
			if string(events) != want {
				return fmt.Errorf("%s test invocations: %q", runner, events)
			}
			if !checked || !marked {
				return fmt.Errorf("%s missing launcher check", runner)
			}
		}
	}
	for _, runner := range []string{"ubuntu-latest", "macos-latest", "windows-latest"} {
		if counts[runner] != 1 {
			return fmt.Errorf("%s has %d test owners, want one", runner, counts[runner])
		}
	}
	for _, need := range []string{"ci", "platform"} {
		if !slices.Contains(w.Jobs["release"].Needs, need) {
			return fmt.Errorf("release does not wait for %s", need)
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
			j := w.Jobs["platform"]
			switch remove {
			case "ubuntu-latest":
				delete(w.Jobs, "ci")
			case "macos-latest", "windows-latest":
				j.Strategy.Matrix.OS = slices.DeleteFunc(j.Strategy.Matrix.OS, func(s string) bool { return s == remove })
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
				j.Strategy.Matrix.OS = append(j.Strategy.Matrix.OS, "ubuntu-latest")
			case "no draft condition":
				j.If = ""
			case "another condition":
				linux := w.Jobs["ci"]
				linux.If = "${{ github.event_name != 'pull_request' }}"
				w.Jobs["ci"] = linux
			}
			w.Jobs["platform"] = j
			if err := contract(t, w); err == nil {
				t.Fatal("coverage loss accepted")
			}
		})
	}
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
		names := []string{w.Jobs["ci"].Name, "platform (macos-latest)", "platform (windows-latest)"}
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
		for _, mode := range []string{"green", "missing Linux", "missing macOS", "missing Windows", "failed", "skipped", "duplicate macOS", "API failure", "metadata", "mixed metadata", "missing named tests", "missing launcher", "no completed run", "no run"} {
			t.Run(mode, func(t *testing.T) {
				dir, env := fakeEnv(t)
				var jobs []map[string]any
				for i, name := range names {
					if mode == []string{"missing Linux", "missing macOS", "missing Windows"}[i] {
						continue
					}
					conclusion := "success"
					if i == 0 && (mode == "failed" || mode == "skipped") {
						conclusion = mode
					}
					if i == 2 && mode == "duplicate macOS" {
						name = names[1]
					}
					stepNames := []string{"Install the launcher", "The launcher"}
					if i == 0 {
						stepNames = append(stepNames, "Plan")
					} else {
						stepNames = append(stepNames, "Unit tests", "Named tests")
					}
					if mode == "metadata" || (mode == "mixed metadata" && i == 0) {
						stepNames = []string{"Validation-only range", "Plan"}
					}
					var steps []map[string]string
					for _, stepName := range stepNames {
						if mode == "missing named tests" && i == 1 && stepName == "Named tests" {
							continue
						}
						if mode == "missing launcher" && i == 2 && stepName == "Install the launcher" {
							continue
						}
						steps = append(steps, map[string]string{"name": stepName, "conclusion": "success"})
					}
					jobs = append(jobs, map[string]any{"name": name, "conclusion": conclusion, "steps": steps})
				}
				data, _ := json.Marshal(map[string]any{"jobs": jobs})
				list := runs
				switch mode {
				case "no completed run": // none on main both completed and run
					list = runs[:4]
				case "no run": // the empty list the nightly of 2026-10-22 (run 39817143873) was given
					list = nil
				}
				scratch.WriteFiles(t, dir, map[string]string{"jobs.json": string(data), "runs.json": "[" + strings.Join(list, ",") + "]"})
				env = scratch.With(env, "JOBS="+filepath.Join(dir, "jobs.json"), "RUNS="+filepath.Join(dir, "runs.json"), "WANT_RUN=39772979146")
				if mode == "API failure" {
					env = scratch.With(env, "GH_FAIL=1")
				}
				r := scratch.Run(dir, env, "", "sh", "-c", command)
				if (r.Code == 0) != (mode == "green" || mode == "metadata") {
					t.Fatalf("%s: exit %d\n%s", mode, r.Code, r.Output())
				}
				if said := strings.Contains(r.Output(), none); said != (mode == "no completed run" || mode == "no run") || (said && r.Code != 1) {
					t.Fatalf("%s: exit %d\n%s", mode, r.Code, r.Output())
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
		"ci.yml":      {"ci", "platform"},
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
				if s.Uses == actionPath {
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
