// Package budget holds the self-test of the mutation budget of q-41 and q-52 (T-131, a Go test
// since T-143), proved through the real CI planner and driver: a push's code range runs
// proof.code.check, one freshly judged mutation site of its changed Go functions, and a nightly
// runs twenty over cmd and internal with a seed of its own and without strict coverage (q-54),
// and neither plan runs any other mutation command. Each plan is this repository's own, whole,
// with only the configured shell replaced in a scratch config, so the driver dispatches every
// planned command to a fake that logs it (this test's own binary, TestMain), and nothing it names
// (itos-cc, the features, CI's other steps) runs. A provider's refusal, and a provider that cannot
// run, must still fail the run.
package budget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
)

// The fake shell's variables: it is one while envFake is set, logs each command it is handed
// to the file envLog names, and fails a mutation command as envFail says.
const (
	envFake = "CI_MUTATION_FAKE_SHELL"
	envLog  = "CI_MUTATION_COMMANDS"
	envFail = "CI_MUTATION_FAIL"
)

// TestMain is the fake shell when the driver starts this binary as one: it answers every
// command with exit 0, except a mutation command while CI_MUTATION_FAIL is set: "refuse" prints
// itos-cc's refusal of a survivor and exits 1, a number exits with it.
func TestMain(m *testing.M) {
	if os.Getenv(envFake) == "" {
		os.Exit(m.Run())
	}
	command := os.Args[len(os.Args)-1]
	line, _ := json.Marshal(map[string]string{"command": command})
	f, err := os.OpenFile(os.Getenv(envLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(9)
	}
	f.Write(append(line, '\n'))
	f.Close()
	fail := os.Getenv(envFail)
	if fail != "" && strings.Contains(command, "itos-cc mutation") {
		if fail == "refuse" {
			os.Stdout.WriteString(`{"schema":1,"ok":false,"problems":[{"rule":"mutation.survived","message":"a mutant of ci.Run survived","fix":"test it"}]}`)
			os.Exit(1)
		}
		code, _ := strconv.Atoi(fail)
		os.Exit(code)
	}
	os.Exit(0)
}

// answer is what itos ci run --json says, as much as this test reads.
type answer struct {
	OK       bool `json:"ok"`
	FailedAt struct {
		Step     string `json:"step"`
		Code     int    `json:"code"`
		Problems []struct {
			Rule string `json:"rule"`
		} `json:"problems"`
	} `json:"failed_at"`
}

func TestSelftest(t *testing.T) {
	scratch.Gate(t)
	root := scratch.Root(t)
	dir, err := os.MkdirTemp(root, ".ci-mutation-budget-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	env := scratch.With(scratch.OutsideEnv(), "GIT_INDEX_FILE="+filepath.Join(dir, "index"))
	git := func(input string, args ...string) string {
		t.Helper()
		r := scratch.Run(root, env, input, scratch.Git(), args...)
		if r.Code != 0 {
			t.Fatalf("git %s failed:\n%s", strings.Join(args, " "), r.Output())
		}
		return strings.TrimSpace(r.Stdout)
	}

	// This repository's config with the fake as its shell.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join(root, "itos.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	shell := regexp.MustCompile(`(?m)^shell: .*$`)
	if !shell.Match(policy) {
		t.Fatal("itos.yaml has no shell: line to replace")
	}
	quoted, _ := json.Marshal([]string{self})
	config := filepath.Join(dir, "itos.yaml")
	if err := os.WriteFile(config, shell.ReplaceAll(policy, append([]byte("shell: "), quoted...)), 0o644); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "commands.jsonl")

	// One run of tools/bin/itos with the fake shell: its exit code, its answer and every
	// itos-cc mutation command the plan dispatched, the plan's budget.
	invocation := func(t *testing.T, fail string, args ...string) (int, answer, []string) {
		t.Helper()
		if err := os.WriteFile(log, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		name, argv := scratch.Program(filepath.Join(root, "tools", "bin", "itos"), args...)
		r := scratch.Run(root, scratch.With(env, "ITOS_CONFIG="+config, envLog+"="+log, envFake+"=1", envFail+"="+fail), "", name, argv...)
		if r.Code < 0 {
			t.Fatalf("tools/bin/itos could not start:\n%s", r.Output())
		}
		var a answer
		if err := json.Unmarshal([]byte(r.Stdout), &a); err != nil {
			t.Fatalf("itos %s said no JSON: %v\n%s", strings.Join(args, " "), err, r.Output())
		}
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		var mutations []string
		for _, line := range strings.Split(string(data), "\n") {
			var c struct{ Command string }
			if line != "" && json.Unmarshal([]byte(line), &c) == nil && strings.Contains(c.Command, "itos-cc mutation") {
				mutations = append(mutations, c.Command)
			}
		}
		return r.Code, a, mutations
	}

	// Give the planner a real code-path range without moving HEAD, the shared index, or any
	// branch.
	git("", "read-tree", "HEAD")
	blob := git("ci mutation budget self-test\n", "hash-object", "-w", "--stdin")
	git("", "update-index", "--add", "--cacheinfo", "100644,"+blob+",internal/ci/ci.go")
	tree := git("", "write-tree")
	to := git("", "-c", "user.name=ci-mutation-budget", "-c", "user.email=selftest@localhost",
		"commit-tree", tree, "-p", "HEAD", "-m", "ci: mutation budget self-test range")
	from := git("", "rev-parse", "HEAD")

	expectedPush := "tools/bin/pinned itos-cc mutation run --count 1 --since " + from + " --seed " + from + " --fail-uncovered=lines --json cmd internal"
	const expectedNightly = `tools/bin/pinned itos-cc mutation run --count 20 --seed "${GITHUB_RUN_ID:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" --json cmd internal`

	// A push: one fresh mutant of the range's changed Go functions, drawn by the range's start
	// (T-159), strict coverage of its changed lines (q-57), cmd and internal, and no other
	// mutation command.
	t.Run("a push runs exactly the code proof", func(t *testing.T) {
		code, a, mutations := invocation(t, "", "ci", "run", from, to, "--json")
		if code != 0 || !a.OK {
			t.Fatalf("the push's CI driver did not pass with fake commands: exit %d", code)
		}
		if !slices.Equal(mutations, []string{expectedPush}) {
			t.Errorf("a push's CI must run exactly the code proof %s:\n%s", expectedPush, strings.Join(mutations, "\n"))
		}
	})

	// A nightly: twenty fresh mutants over cmd and internal, seeded by the run, not HEAD, with no
	// range, no strict coverage (q-54: with no range it judges every function), and no other
	// mutation command.
	t.Run("the nightly runs exactly twenty mutants with a seed of its own", func(t *testing.T) {
		code, a, mutations := invocation(t, "", "ci", "run", "--nightly", "--json")
		if code != 0 || !a.OK {
			t.Fatalf("the nightly's CI driver did not pass with fake commands: exit %d", code)
		}
		if !slices.Equal(mutations, []string{expectedNightly}) {
			t.Errorf("the nightly must run exactly %s:\n%s", expectedNightly, strings.Join(mutations, "\n"))
		}
	})

	// The seed as the nightly's sh expands it: GitHub's run id, so two runs of one commit draw
	// differently; by hand, the time.
	seed := regexp.MustCompile(`--seed ("[^"]*")`).FindStringSubmatch(expectedNightly)[1]
	seedArg := func(t *testing.T, vars ...string) string {
		t.Helper()
		r := scratch.Run(root, append([]string{"PATH=" + os.Getenv("PATH")}, vars...), "", "sh", "-c", "printf '%s' "+seed)
		if r.Code != 0 {
			t.Fatal(r.Stderr)
		}
		return r.Stdout
	}
	t.Run("the nightly's seed is GitHub's run id", func(t *testing.T) {
		if got := seedArg(t, "GITHUB_RUN_ID=1234"); got != "1234" {
			t.Errorf("the seed with GITHUB_RUN_ID=1234 is %q", got)
		}
	})
	t.Run("the nightly's seed by hand is the time", func(t *testing.T) {
		if got := seedArg(t); !regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$`).MatchString(got) {
			t.Errorf("the seed with no run id is %q, not the time", got)
		}
	})

	// A provider's refusal fails the push at the code proof, its problem kept.
	t.Run("a refusing provider blocks a push", func(t *testing.T) {
		code, a, _ := invocation(t, "refuse", "ci", "run", from, to, "--json")
		if code != 1 || a.OK || a.FailedAt.Step != expectedPush || len(a.FailedAt.Problems) == 0 || a.FailedAt.Problems[0].Rule != "mutation.survived" {
			t.Errorf("a refusing provider did not block CI at the code proof: exit %d, %+v", code, a)
		}
	})

	// A provider that cannot run (itos-cc's exit 3: not Linux, no Git, a tool missing) fails it
	// too, never a pass.
	t.Run("a provider that cannot run blocks a push", func(t *testing.T) {
		code, a, _ := invocation(t, "3", "ci", "run", from, to, "--json")
		if code != 1 || a.FailedAt.Step != expectedPush || len(a.FailedAt.Problems) == 0 || a.FailedAt.Problems[0].Rule != "ci-code-proof" {
			t.Errorf("a provider that cannot run did not block CI: exit %d, %+v", code, a)
		}
	})

	// And the nightly's step fails the night.
	t.Run("a failing provider blocks the nightly", func(t *testing.T) {
		code, a, _ := invocation(t, "1", "ci", "run", "--nightly", "--json")
		if code != 1 || a.FailedAt.Step != expectedNightly || a.FailedAt.Code != 1 {
			t.Errorf("a failing nightly provider did not block: exit %d, %+v", code, a)
		}
	})
}
