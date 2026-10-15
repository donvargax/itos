// Package platform holds the push workflow's three-platform coverage contract. Commands run
// against fake go/itos executables; the main job's delegated plan is read from the real planner.
// The nightly's GitHub query runs against fixture jobs through real jq, without network access.
package platform

import (
	"encoding/json"
	"fmt"
	"os"
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
	t.Helper()
	data, err := os.ReadFile(filepath.Join(scratch.Root(t), ".github/workflows/ci.yml"))
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
		"go": "#!/bin/sh\n[ \"$*\" = 'test ./cmd/... ./internal/...' ] || exit 91\necho unit >> \"$LOG\"\n",
		"tools/bin/itos": `#!/bin/sh
case "$*" in
  'ci run base head') printf 'unit\nnamed\n' >> "$LOG";;
  'ci run base head --tests') echo named >> "$LOG";;
  'version --json') printf '{"version":"%s"}\n' "${LAUNCHER_VERSION-8.0.0}";;
  *) exit 92;;
esac
`,
		"gh": `#!/bin/sh
[ "$GH_FAIL" != 1 ] || exit 93
case "$1 $2" in
  'run list') echo 123;;
  'run view')
    while [ "$1" != --jq ]; do [ "$#" -gt 0 ] || exit 94; shift; done
    shift
    jq -r "$1" "$JOBS";;
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
	counts := map[string]int{}
	for _, id := range []string{"ci", "platform"} {
		j, ok := w.Jobs[id]
		if !ok || j.If != "" {
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
			installed, checked := false, false
			for _, s := range j.Steps {
				if s.Uses == "./" {
					if s.ID != "launcher" || s.If != "${{ !cancelled() }}" {
						return fmt.Errorf("%s launcher install guard changed", runner)
					}
					installed = true
				}
				selected := s.Name == "Unit tests" || s.Name == "Named tests" || s.Name == "The launcher" || strings.TrimSpace(s.Run) == `tools/bin/itos ci run "$FROM" "$TO"`
				if !selected {
					continue
				}
				if s.Name == "Unit tests" && s.If != "" {
					return fmt.Errorf("%s unit tests are conditional", runner)
				}
				if s.Name == "Named tests" && s.If != "${{ !cancelled() && steps.range.outcome == 'success' }}" {
					return fmt.Errorf("%s named-test guard changed", runner)
				}
				if id == "ci" && s.Name != "The launcher" && s.If != "" {
					return fmt.Errorf("Linux plan is conditional")
				}
				if s.Name == "The launcher" {
					if !installed || s.If != "${{ !cancelled() && steps.launcher.outcome == 'success' }}" {
						return fmt.Errorf("%s launcher check has no successful install", runner)
					}
					checked = true
					bad := scratch.Run(dir, scratch.With(env, "LAUNCHER_VERSION=invalid"), "", "bash", "-eo", "pipefail", "-c", s.Run)
					if bad.Code == 0 {
						return fmt.Errorf("%s launcher check accepts an invalid version", runner)
					}
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
			if string(events) != "unit\nnamed\n" {
				return fmt.Errorf("%s test invocations: %q", runner, events)
			}
			if !checked {
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
	t.Run("one test owner and launcher check per platform", func(t *testing.T) {
		if err := contract(t, readWorkflow(t)); err != nil {
			t.Fatal(err)
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
	for _, remove := range []string{"ubuntu-latest", "macos-latest", "windows-latest", "launcher", "launcher version", "Linux launcher", "Linux launcher version", "unit", "named", "release", "duplicate Linux"} {
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
		for _, mode := range []string{"green", "missing Linux", "missing macOS", "missing Windows", "failed", "skipped", "duplicate macOS", "API failure"} {
			t.Run(mode, func(t *testing.T) {
				dir, env := fakeEnv(t)
				var jobs []map[string]string
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
					jobs = append(jobs, map[string]string{"name": name, "conclusion": conclusion})
				}
				data, _ := json.Marshal(map[string]any{"jobs": jobs})
				scratch.WriteFiles(t, dir, map[string]string{"jobs.json": string(data)})
				env = scratch.With(env, "JOBS="+filepath.Join(dir, "jobs.json"))
				if mode == "API failure" {
					env = scratch.With(env, "GH_FAIL=1")
				}
				r := scratch.Run(dir, env, "", "sh", "-c", command)
				if (r.Code == 0) != (mode == "green") {
					t.Fatalf("%s: exit %d\n%s", mode, r.Code, r.Output())
				}
			})
		}
	})
}
