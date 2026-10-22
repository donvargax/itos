// Package scratch is what the gates' self-tests share (T-143, from tools/selftest/scratch.ts):
// the repository's top folder, the real git and the environment they run everything in,
// tools/bin/itos and the CI plan it gives, a scratch worktree of the current tree whose hooks run
// as git runs them, and the scratch repositories of a check held to the last release.
//
// Each self-test is one Go test, TestSelftest, beside the program it tests (or in a folder of
// tools/selftest when it tests no single program), with one subtest per case. It runs only with
// ITOS_SELFTEST set (Gate): itos.yaml's CI and nightly steps run each by name with
// ITOS_SELFTEST=1, at the cost and the place in the plan its TypeScript had, and go test
// ./tools/..., which every push runs over the gate programs' unit tests, skips them.
//
// Everything runs with the git internal/git finds (git.Bin), never the name git, which may be an
// itos linked as git whose git commit is itos commit under its own policy (T-122), and in an
// environment without a hook's GIT_ variables or CI (OutsideEnv).
package scratch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/git"
	repoprogram "github.com/donvargax/itos/v8/tools/bin/repo-program"
)

// EnvSelftest is the variable that lets a self-test run (Gate).
const EnvSelftest = "ITOS_SELFTEST"

// Gate skips the calling test unless ITOS_SELFTEST is set: a self-test runs from its own step of
// the plan, never from go test ./tools/..., which a push runs on every change to tools/.
func Gate(t *testing.T) {
	t.Helper()
	if os.Getenv(EnvSelftest) == "" {
		t.Skipf("a gate's self-test: run it with %s=1, as itos.yaml's steps do", EnvSelftest)
	}
}

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

// Root is the repository's top folder: the nearest folder above the test's that holds go.mod.
func Root(t testing.TB) string {
	t.Helper()
	rootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			rootErr = err
			return
		}
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				rootDir = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				rootErr = errors.New("no go.mod above the test's folder")
				return
			}
			dir = parent
		}
	})
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	return rootDir
}

// Git is the real git, as internal/git finds it.
func Git() string { return git.Bin() }

// OutsideEnv is the environment a self-test runs everything in: no hook's GIT_ variables and no
// CI, ITOS_GIT naming the real git, as an itos run hands its git to what it starts, so a hook, a
// gate or a script that runs the name git reaches an itos linked as git with ITOS_GIT set and
// passes straight to that git, and no background maintenance (noMaintenance).
func OutsideEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || name == "CI" || name == git.EnvGit {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, noMaintenance...)
	return append(env, git.EnvGit+"="+Git())
}

// noMaintenance keeps git from maintaining a scratch repository on its own, as the features
// keep it from theirs (T-143): after a commit, a fetch or a merge git starts git maintenance run
// --auto, detached, which takes and drops objects/maintenance.lock (and, past gc.auto's
// threshold, gcs) after the command that started it has returned; in a scratch worktree that is
// the checkout's own object store. maintenance.auto false starts none, and gc.auto 0 leaves the
// gc off. Given as git's environment config, entries 0 and 1, so every git reads it; a self-test
// that adds entries of its own appends them (declareHooks).
var noMaintenance = []string{
	"GIT_CONFIG_COUNT=2",
	"GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false",
	"GIT_CONFIG_KEY_1=gc.auto", "GIT_CONFIG_VALUE_1=0",
}

// lookup is the value of name in env, or "".
func lookup(env []string, name string) string {
	value := ""
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			value = v
		}
	}
	return value
}

// With is env with each NAME=value of set in place of the variable's old value.
func With(env []string, set ...string) []string {
	names := map[string]bool{}
	for _, kv := range set {
		name, _, _ := strings.Cut(kv, "=")
		names[name] = true
	}
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !names[name] {
			out = append(out, kv)
		}
	}
	return append(out, set...)
}

// Result is how a command ended: its exit code (-1 when it could not start or a signal ended
// it), its stdout and its stderr, and how long it took.
type Result struct {
	Code    int
	Stdout  string
	Stderr  string
	Seconds float64
}

// Output is the command's stdout and stderr together.
func (r Result) Output() string { return r.Stdout + r.Stderr }

// Run runs name with args in dir, in env, with stdin given.
func Run(dir string, env []string, stdin string, name string, args ...string) Result {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	err := cmd.Run()
	r := Result{Code: 0, Stdout: stdout.String(), Stderr: stderr.String(), Seconds: time.Since(started).Seconds()}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			r.Code = exit.ExitCode()
		} else {
			r.Code = -1
			r.Stderr += err.Error() + "\n"
		}
	}
	return r
}

// Program is the command and the arguments that start one of the repository's programs, a shell
// script: the script itself where the kernel reads its #! line, sh and the script on windows,
// which starts no script that way (T-135). tools/bin's programs ask the same resolver.
func Program(script string, args ...string) (string, []string) {
	return repoprogram.Command(script, args...)
}

// Itos runs tools/bin/itos of dir in dir and returns its stdout, failing the test when it fails.
func Itos(t testing.TB, dir string, args ...string) string {
	t.Helper()
	name, argv := Program(filepath.Join(dir, "tools", "bin", "itos"), args...)
	r := Run(dir, OutsideEnv(), "", name, argv...)
	if r.Code != 0 {
		t.Fatalf("tools/bin/itos %s exited %d:\n%s", strings.Join(args, " "), r.Code, r.Output())
	}
	return r.Stdout
}

// Plan is what `itos ci plan --json` says a CI run does, as much of it as the self-tests read.
type Plan struct {
	Prose bool     `json:"prose"`
	Steps []string `json:"steps"`
	Tasks []string `json:"tasks"`
}

// CIPlan is tools/bin/itos ci plan <args> --json, run in dir.
func CIPlan(t testing.TB, dir string, args ...string) Plan {
	t.Helper()
	out := Itos(t, dir, append(append([]string{"ci", "plan"}, args...), "--json")...)
	var plan Plan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("itos ci plan %s --json: %v\n%s", strings.Join(args, " "), err, out)
	}
	return plan
}

// FeaturesStep is the plan's run of the features, the step that starts with the kind's whole
// run, or "".
func FeaturesStep(plan Plan, whole string) string {
	for _, step := range plan.Steps {
		if strings.HasPrefix(step, whole) {
			return step
		}
	}
	return ""
}

// Build builds ./tools/bin/<name> into a temporary folder and returns the binary's path.
func Build(t testing.TB, name string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	r := Run(Root(t), OutsideEnv(), "", "go", "build", "-o", bin, "./tools/bin/"+name)
	if r.Code != 0 {
		t.Fatalf("go build ./tools/bin/%s:\n%s", name, r.Output())
	}
	return bin
}

// WriteFiles writes each file of files, by its slash path under dir, making its folders.
func WriteFiles(t testing.TB, dir string, files map[string]string) {
	t.Helper()
	for path, text := range files {
		at := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Repo is a scratch worktree of the current tree: HEAD plus every tracked edit and the untracked
// files under tools/ and features/ a gate could run, committed as a base (Open), so nothing a
// self-test does there touches the checkout it was started from.
type Repo struct {
	t     testing.TB
	Root  string // the checkout
	Dir   string // the scratch worktree
	Env   []string
	hooks string
}

// NewRepo makes the scratch folder of a worktree, named after name, removed when the test ends.
func NewRepo(t testing.TB, name string) *Repo {
	t.Helper()
	dir, err := os.MkdirTemp("", name+"-")
	if err != nil {
		t.Fatal(err)
	}
	r := &Repo{t: t, Root: Root(t), Dir: dir}
	r.Env = With(OutsideEnv(),
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL=selftest@localhost",
		"GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL=selftest@localhost")
	t.Cleanup(r.remove)
	return r
}

// Sh runs command with sh -c in dir (the worktree when ""), with input on its stdin.
func (r *Repo) Sh(command, input, dir string) Result {
	if dir == "" {
		dir = r.Dir
	}
	return Run(dir, r.Env, input, "sh", "-c", command)
}

// Git runs the real git in the worktree and returns its trimmed stdout, failing the test when
// it fails.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *Repo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	res := Run(dir, r.Env, "", Git(), args...)
	if res.Code != 0 {
		r.t.Fatalf("git %s failed:\n%s", strings.Join(args, " "), res.Output())
	}
	return strings.TrimSpace(res.Stdout)
}

// Edit replaces the first from in the worktree's file with to, failing the test when the file
// no longer holds it.
func (r *Repo) Edit(file, from, to string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, filepath.FromSlash(file))
	text, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatal(err)
	}
	if !strings.Contains(string(text), from) {
		r.t.Fatalf("%s no longer contains %q", file, from)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(text), from, to, 1)), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// Commit commits everything in the worktree without the hooks and returns the commit.
func (r *Repo) Commit(message string) string {
	r.t.Helper()
	r.Git("add", "-A")
	sha := r.Git("commit-tree", r.Git("write-tree"), "-p", "HEAD", "-m", message)
	r.Git("reset", "-q", "--hard", sha)
	return sha
}

// Open builds the worktree and returns its base commit.
func (r *Repo) Open() string {
	r.t.Helper()
	r.gitIn(r.Root, "worktree", "add", "-q", "--detach", r.Dir, "HEAD")
	diff := Run(r.Root, r.Env, "", Git(), "diff", "HEAD", "--binary")
	if strings.TrimSpace(diff.Stdout) != "" {
		applied := Run(r.Dir, r.Env, diff.Stdout, Git(), "apply", "--whitespace=nowarn", "-")
		if applied.Code != 0 {
			r.t.Fatalf("could not copy the working tree:\n%s", applied.Output())
		}
	}
	untracked := r.gitIn(r.Root, "ls-files", "--others", "--exclude-standard", "--", "tools", "features")
	for _, file := range strings.Split(untracked, "\n") {
		if file == "" {
			continue
		}
		if err := copyFile(filepath.Join(r.Root, file), filepath.Join(r.Dir, file)); err != nil {
			r.t.Fatal(err)
		}
	}
	return r.Commit(filepath.Base(r.Dir) + " base")
}

func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// declareHooks declares itos's hooks for the worktree alone, as itos hook install declares them
// in a clone's git config (decision 37): the entries the worktree's own itos prints (hook install
// --print), in a file the environment includes where the git dir is the worktree's and nowhere
// else. The worktree's .git/config is the checkout's, which must not change; and git hands its
// environment to the hooks and all they run, so an include matched to the worktree's git dir
// leaves every other repository, the unit tests' among them, as it was. core.hooksPath, an empty
// folder there, keeps a hook folder the checkout's core.hooksPath may name from running beside
// them, so a hook run is what the git config declares. Made once, after Open.
func (r *Repo) declareHooks() string {
	r.t.Helper()
	if r.hooks != "" {
		return r.hooks
	}
	var printed struct {
		Hooks []struct{ Name, Event, Command string } `json:"hooks"`
	}
	out := Itos(r.t, r.Dir, "hook", "install", "--print", "--json")
	if err := json.Unmarshal([]byte(out), &printed); err != nil {
		r.t.Fatalf("itos hook install --print --json: %v\n%s", err, out)
	}
	if len(printed.Hooks) == 0 {
		r.t.Fatal("itos hook install --print declares no hook")
	}
	hooks, err := os.MkdirTemp("", filepath.Base(r.Dir)+"-hooks-")
	if err != nil {
		r.t.Fatal(err)
	}
	r.hooks = hooks
	none := filepath.Join(hooks, "none")
	if err := os.Mkdir(none, 0o755); err != nil {
		r.t.Fatal(err)
	}
	value := func(v string) string {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
	}
	config := "[core]\n\thooksPath = " + value(none) + "\n"
	for _, h := range printed.Hooks {
		config += fmt.Sprintf("[hook %s]\n\tevent = %s\n\tcommand = %s\n", value(h.Name), value(h.Event), value(h.Command))
	}
	file := filepath.Join(hooks, "config")
	if err := os.WriteFile(file, []byte(config), 0o644); err != nil {
		r.t.Fatal(err)
	}
	n, _ := strconv.Atoi(lookup(r.Env, "GIT_CONFIG_COUNT"))
	r.Env = With(r.Env, "GIT_CONFIG_COUNT="+strconv.Itoa(n+1),
		"GIT_CONFIG_KEY_"+strconv.Itoa(n)+"=includeIf.gitdir:"+r.Git("rev-parse", "--absolute-git-dir")+".path",
		"GIT_CONFIG_VALUE_"+strconv.Itoa(n)+"="+file)
	return hooks
}

// shQuote quotes s for sh.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// HookRun is the sh command that runs the hooks the worktree's git config declares for event
// (git hook run, which fails when none is declared), with its arguments and, given, its stdin.
func (r *Repo) HookRun(event string, args []string, input *string) string {
	r.t.Helper()
	folder := r.declareHooks()
	stdin := ""
	if input != nil {
		file := filepath.Join(folder, event+".stdin")
		if err := os.WriteFile(file, []byte(*input), 0o644); err != nil {
			r.t.Fatal(err)
		}
		stdin = " --to-stdin=" + shQuote(file)
	}
	return fmt.Sprintf("%s hook run%s %s -- %s", shQuote(Git()), stdin, event, strings.Join(args, " "))
}

func (r *Repo) remove() {
	Run(r.Root, r.Env, "", Git(), "worktree", "remove", "--force", r.Dir)
	os.RemoveAll(r.Dir)
	if r.hooks != "" {
		os.RemoveAll(r.hooks)
	}
}

// Hooks runs the real hooks in a Repo, each run logged with its time: pre-commit, this
// repository's own, from its file (tools/bin/pre-commit); commit-msg and pre-push, itos's, as git
// runs them from the worktree's git config (HookRun).
type Hooks struct {
	R *Repo
}

// Gate runs command in the worktree and logs how it went.
func (h Hooks) Gate(t testing.TB, label, command string) Result {
	t.Helper()
	res := h.R.Sh(command, "", "")
	verdict := "pass"
	if res.Code != 0 {
		verdict = "FAIL"
	}
	t.Logf("%-58s %s  %.1f s", label, verdict, res.Seconds)
	return res
}

// PreCommit stages everything and runs the pre-commit hook.
func (h Hooks) PreCommit(t testing.TB, label string) Result {
	t.Helper()
	h.R.Git("add", "-A")
	return h.Gate(t, "pre-commit, "+label, "sh tools/bin/pre-commit")
}

// CommitMsg runs the commit-msg hooks on the message file.
func (h Hooks) CommitMsg(t testing.TB, label, file string) Result {
	t.Helper()
	return h.Gate(t, "commit-msg, "+label, h.R.HookRun("commit-msg", []string{shQuote(file)}, nil))
}

// PrePush runs the pre-push hooks for a push of sha onto base.
func (h Hooks) PrePush(t testing.TB, label, base, sha string) Result {
	t.Helper()
	input := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", sha, base)
	return h.Gate(t, "pre-push, "+label,
		h.R.HookRun("pre-push", []string{"upstream", "git@example.invalid:upstream.git"}, &input))
}

// Release is the scratch repositories of a check held to the last release
// (tools/bin/schema-contract, tools/bin/previous-release): git run in a folder with a fixed
// author, in OutsideEnv with extra laid over it; an empty commit; and the two repositories every
// such check must not pass blindly, one with no release tag and a shallow clone.
type Release struct {
	t   testing.TB
	Tmp string
	Env []string
}

// NewRelease makes the scratch folder the repositories go in.
func NewRelease(t testing.TB, extra ...string) *Release {
	t.Helper()
	env := With(OutsideEnv(), extra...)
	env = With(env, "GIT_AUTHOR_NAME=selftest", "GIT_AUTHOR_EMAIL=selftest@localhost",
		"GIT_COMMITTER_NAME=selftest", "GIT_COMMITTER_EMAIL=selftest@localhost")
	return &Release{t: t, Tmp: t.TempDir(), Env: env}
}

// Git runs the real git in dir and returns its trimmed stdout, failing the test when it fails.
func (rel *Release) Git(dir string, args ...string) string {
	rel.t.Helper()
	res := Run(dir, rel.Env, "", Git(), args...)
	if res.Code != 0 {
		rel.t.Fatalf("git %s:\n%s", strings.Join(args, " "), res.Stderr)
	}
	return strings.TrimSpace(res.Stdout)
}

// Commit makes an empty commit in dir.
func (rel *Release) Commit(dir, message string) {
	rel.t.Helper()
	rel.Git(dir, "commit", "-q", "--allow-empty", "-m", message)
}

// Untagged is a repository with one feat and no release tag.
func (rel *Release) Untagged() string {
	rel.t.Helper()
	dir := filepath.Join(rel.Tmp, "untagged")
	rel.Git(rel.Tmp, "init", "-q", "-b", "main", dir)
	rel.Commit(dir, "feat: the first")
	return dir
}

// Shallow is a shallow clone of repo, with no tags.
func (rel *Release) Shallow(repo string) string {
	rel.t.Helper()
	dir := filepath.Join(rel.Tmp, "shallow")
	os.RemoveAll(dir)
	rel.Git(rel.Tmp, "clone", "-q", "--depth", "1", "--no-tags", "file://"+filepath.ToSlash(repo), dir)
	return dir
}
