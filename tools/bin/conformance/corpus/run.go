package corpus

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	itosgit "github.com/donvargax/itos/v8/internal/git"
)

// Options are how a Runner runs its cases.
type Options struct {
	// Bin is the command each case runs: a path if it holds a separator, else a name on the
	// PATH.
	Bin string
	// Git is native Git, the git the runner builds and inspects each case's repository with, an
	// absolute path; "" for internal/git's Real, the first git on the PATH that is not an itos.
	Git string
	// Additive judges each case by what its output must still hold (the package's doc).
	Additive bool
	// CaseDeadline bounds each case, every process it starts; 0 for a minute.
	CaseDeadline time.Duration
	// TempRoot is where each case's scratch folder is made; "" for the system's.
	TempRoot string
	// Jobs is how many cases run at once; 0 for one fewer than the machine's processors, at
	// most four.
	Jobs int
}

// Runner runs cases: the caller's PATH with its itos extensions hidden and a folder holding
// native Git alone first, the binary's version learnt.
type Runner struct {
	opts      Options
	bin       string
	nativeGit string
	runPath   string // {{PATH}}
	paths     string // the folders the runner made for the PATH
	version   string // {{version}}
	release   string // {{release}}
}

// New readies a runner; Close removes what it made.
func New(opts Options) (*Runner, error) {
	r := &Runner{opts: opts, bin: opts.Bin}
	if r.opts.CaseDeadline == 0 {
		r.opts.CaseDeadline = time.Minute
	}
	if r.opts.Jobs == 0 {
		r.opts.Jobs = max(1, min(4, runtime.NumCPU()-1))
	}
	if strings.ContainsRune(r.bin, '/') || strings.ContainsRune(r.bin, filepath.Separator) {
		abs, err := filepath.Abs(r.bin)
		if err != nil {
			return nil, fixtureErr("%v", err)
		}
		r.bin = abs
	}
	r.nativeGit = opts.Git
	if r.nativeGit == "" {
		found, err := itosgit.Real()
		if err != nil {
			return nil, fixtureErr("no native git: %v", err)
		}
		r.nativeGit = found
	}
	if !filepath.IsAbs(r.nativeGit) {
		return nil, fixtureErr("--git names a non-absolute path: %s", r.nativeGit)
	}
	paths, err := os.MkdirTemp("", "itos-conformance-path-")
	if err != nil {
		return nil, fixtureErr("%v", err)
	}
	r.paths = paths
	hidden, err := hiddenPath(os.Getenv("PATH"), func(name string) bool { return strings.HasPrefix(name, "itos-") }, paths)
	if err != nil {
		r.Close()
		return nil, err
	}
	gitDir, err := nativeGitDirectory(r.nativeGit, paths)
	if err != nil {
		r.Close()
		return nil, err
	}
	r.runPath = nativeGitFirst(hidden, gitDir)
	return r, nil
}

// Close removes the folders the runner made.
func (r *Runner) Close() {
	if r.paths != "" {
		_ = os.RemoveAll(r.paths)
	}
}

// LearnVersion asks the binary its version, as a case would run it: in an empty folder, in the
// clean environment, with no config to read. {{version}} is the last word it says, {{release}}
// that version's X.Y.Z.
func (r *Runner) LearnVersion() error {
	scratch, err := os.MkdirTemp("", "itos-conformance-version-")
	if err != nil {
		return fixtureErr("%v", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	home := filepath.Join(scratch, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		return fixtureErr("%v", err)
	}
	cmd := exec.Command(r.bin, "version")
	cmd.Dir, cmd.Env = scratch, environ(cleanEnv(home, r.runPath))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	words := strings.Fields(string(out))
	if err != nil || len(words) == 0 {
		code := "?"
		if cmd.ProcessState != nil {
			code = strconv.Itoa(cmd.ProcessState.ExitCode())
		}
		said := stderr.String()
		if err != nil && cmd.ProcessState == nil {
			said = err.Error()
		}
		return fixtureErr("%s version (exit %s) says no version: %s", r.bin, code, said)
	}
	r.version = words[len(words)-1]
	r.release = r.version
	if m := regexp.MustCompile(`^\d+\.\d+\.\d+`).FindString(r.version); m != "" {
		r.release = m
	}
	return nil
}

// nativeGitDirectory is a folder of dir holding native Git alone, under its own name.
func nativeGitDirectory(nativeGit, dir string) (string, error) {
	folder := filepath.Join(dir, "native-git")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fixtureErr("%v", err)
	}
	link := filepath.Join(folder, filepath.Base(nativeGit))
	if _, err := os.Lstat(link); err == nil {
		return folder, nil
	}
	if err := os.Symlink(nativeGit, link); err != nil {
		data, err := os.ReadFile(nativeGit)
		if err != nil {
			return "", fixtureErr("%v", err)
		}
		info, err := os.Stat(nativeGit)
		if err != nil {
			return "", fixtureErr("%v", err)
		}
		if err := os.WriteFile(link, data, info.Mode().Perm()); err != nil {
			return "", fixtureErr("%v", err)
		}
	}
	return folder, nil
}

// nativeGitFirst is path with folder first, and nowhere else.
func nativeGitFirst(path, folder string) string {
	out := []string{folder}
	for _, entry := range filepath.SplitList(path) {
		if entry != folder {
			out = append(out, entry)
		}
	}
	return strings.Join(out, string(os.PathListSeparator))
}

// hiddenPath is a PATH on which no program hides names is found: each folder that holds one is
// replaced by a folder of links, in into, to everything else in it.
func hiddenPath(path string, hides func(string) bool, into string) (string, error) {
	folders := filepath.SplitList(path)
	for i, folder := range folders {
		entries, err := os.ReadDir(folder)
		if err != nil {
			continue
		}
		hidden := false
		for _, e := range entries {
			hidden = hidden || hides(e.Name())
		}
		if !hidden {
			continue
		}
		linked := filepath.Join(into, "path-"+strconv.Itoa(i))
		if err := os.MkdirAll(linked, 0o755); err != nil {
			return "", fixtureErr("%v", err)
		}
		for _, e := range entries {
			if hides(e.Name()) {
				continue
			}
			if err := os.Symlink(filepath.Join(folder, e.Name()), filepath.Join(linked, e.Name())); err != nil && !errors.Is(err, os.ErrExist) {
				return "", fixtureErr("%v", err)
			}
		}
		folders[i] = linked
	}
	return strings.Join(folders, string(os.PathListSeparator)), nil
}

// callerOnly is what of the caller's environment reaches into the caller's repository or changes
// what the tool does.
var callerOnly = regexp.MustCompile(`^(GIT_|ITOS_|GITHUB_|RUNNER_|PRE_COMMIT_|GH_)|^(CI|NO_COLOR|FORCE_COLOR)$`)

// cleanEnv is the caller's environment without what would reach into the caller's repository or
// change what the tool does, a git that reads no config but the scratch repository's, fixed
// authors and dates (so every scratch commit's SHA is the same on every run), and a launcher
// (internal/launch) that cannot fetch a release: a cache in the case's HOME, a release server
// nothing answers on, and no asking for the newest release. A case's config pins nothing, or the
// binary's own version, so it runs the binary itself.
func cleanEnv(home, path string) map[string]string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		key, value, _ := strings.Cut(kv, "=")
		if key == "" || callerOnly.MatchString(key) || strings.EqualFold(key, "PATH") || strings.EqualFold(key, "HOME") {
			continue
		}
		env[key] = value
	}
	for k, v := range map[string]string{
		"PATH":                path,
		"HOME":                home,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   filepath.Join(home, ".gitconfig"),
		"GIT_AUTHOR_NAME":     "itos conformance",
		"GIT_AUTHOR_EMAIL":    "conformance@localhost",
		"GIT_COMMITTER_NAME":  "itos conformance",
		"GIT_COMMITTER_EMAIL": "conformance@localhost",
		"GIT_AUTHOR_DATE":     "2000-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE":  "2000-01-01T00:00:00Z",
		"ITOS_CACHE":          filepath.Join(home, "itos-cache"),
		"ITOS_RELEASES":       "http://127.0.0.1:1/no-release-server",
		"ITOS_NO_UPDATE":      "1",
	} {
		env[k] = v
	}
	return env
}

// noMaintenance keeps git from maintaining a case's repository on its own, as the features and
// the self-tests do (T-143): after a commit, a fetch or a merge git starts git maintenance run
// --auto, detached, which writes to .git after the command that started it has returned, so a
// case that holds what is left there could see a file neither itos nor the case wrote.
// maintenance.auto false starts none; gc.auto 0 leaves the gc off too. Given as git's
// environment config, after any entries already there, so every git a case runs reads it,
// itos's and the runner's alike.
func noMaintenance(env map[string]string) {
	n, err := strconv.Atoi(env["GIT_CONFIG_COUNT"])
	if err != nil || n < 0 {
		n = 0
	}
	for i, kv := range [][2]string{{"maintenance.auto", "false"}, {"gc.auto", "0"}} {
		env["GIT_CONFIG_KEY_"+strconv.Itoa(n+i)] = kv[0]
		env["GIT_CONFIG_VALUE_"+strconv.Itoa(n+i)] = kv[1]
	}
	env["GIT_CONFIG_COUNT"] = strconv.Itoa(n + 2)
}

// environ is an environment as exec takes it, sorted.
func environ(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// subst replaces a case's {{…}} in its strings: the first one it cannot replace is its error.
type subst struct {
	dir, path, git, version, release string
	labels                           map[string]string
	err                              error
}

var placeholder = regexp.MustCompile(`\{\{(dir|PATH|PATH_SEP|git|version|release|sha\.[\w-]+|short\.[\w-]+)\}\}`)

func (s *subst) text(t string) string {
	return placeholder.ReplaceAllStringFunc(t, func(whole string) string {
		name := whole[2 : len(whole)-2]
		switch name {
		case "dir":
			return s.dir
		case "PATH":
			return s.path
		case "PATH_SEP":
			return string(os.PathListSeparator)
		case "git":
			return s.git
		case "version":
			return s.version
		case "release":
			return s.release
		}
		form, label, _ := strings.Cut(name, ".")
		sha, ok := s.labels[label]
		if !ok {
			if s.err == nil {
				s.err = fixtureErr("no commit is labelled %s (%s)", label, whole)
			}
			return whole
		}
		if form == "short" {
			return sha[:min(7, len(sha))]
		}
		return sha
	})
}

// deep is a value with every string in it, at any depth, replaced.
func (s *subst) deep(v any) any {
	switch t := v.(type) {
	case string:
		return s.text(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = s.deep(e)
		}
		return out
	case *mapping:
		out := newMapping()
		for _, k := range t.keys {
			out.set(k, s.deep(t.vals[k]))
		}
		return out
	}
	return v
}

// writeFiles writes files into dir, and says the paths it wrote.
func writeFiles(dir string, fs files, s *subst) ([]string, error) {
	var written []string
	for _, e := range fs {
		full := filepath.Join(dir, e.path)
		written = append(written, strings.TrimSuffix(e.path, "/"))
		switch {
		case e.null:
			if err := os.RemoveAll(full); err != nil {
				return nil, err
			}
		case strings.HasSuffix(e.path, "/"):
			if err := os.MkdirAll(full, 0o755); err != nil {
				return nil, err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(full, []byte(s.text(e.text)), 0o644); err != nil {
				return nil, err
			}
			if e.executable != nil && *e.executable {
				if err := os.Chmod(full, 0o755); err != nil {
					return nil, err
				}
			}
		}
	}
	return written, nil
}

// caseRun is one case's processes, folders and environments.
type caseRun struct {
	r          *Runner
	p          *procs
	dir        string
	inspection []string
}

// git runs native Git in the case's folder; its stdout, trimmed.
func (cr *caseRun) git(input string, args ...string) (string, error) {
	res, err := cr.p.run(cr.r.nativeGit, args, cr.dir, cr.inspection, input)
	if err != nil {
		return "", err
	}
	if res.code == nil || *res.code != 0 {
		return "", fixtureErr("%s", strings.TrimRight(fmt.Sprintf("git %s failed:\n%s%s", strings.Join(args, " "), res.stderr, res.stdout), " \t\r\n"))
	}
	return strings.TrimSpace(res.stdout), nil
}

// sh runs a shell command in the case's folder.
func (cr *caseRun) sh(command string) error {
	res, err := cr.p.run("sh", []string{"-c", command}, cr.dir, cr.inspection, "")
	if err != nil {
		return err
	}
	if res.code == nil || *res.code != 0 {
		return fixtureErr("%s", strings.TrimRight(fmt.Sprintf("`%s` failed:\n%s%s", command, res.stderr, res.stdout), " \t\r\n"))
	}
	return nil
}

// standInHooks are itos's two hooks, declared in a scratch repository's git config under the
// names itos hook install gives them (slice 91), each a command that runs nothing (: is sh's
// no-op). itos refuses to commit or push where its hooks are not declared, so every case's
// repository has them, and a case's commits run no hook's check; a case that wants them
// otherwise changes them in a run step.
var standInHooks = [][2]string{
	{"hook.itos-commit-msg.event", "commit-msg"},
	{"hook.itos-commit-msg.command", ": itos hook commit-msg"},
	{"hook.itos-pre-push.event", "pre-push"},
	{"hook.itos-pre-push.command", ": itos hook pre-push"},
}

// buildRepo builds the scratch repository a case's git steps describe, each labelled commit
// noted.
func (cr *caseRun) buildRepo(steps []step, s *subst) error {
	if _, err := cr.git("", "init", "-q", "-b", "main"); err != nil {
		return err
	}
	if _, err := cr.git("", "config", "commit.gpgsign", "false"); err != nil {
		return err
	}
	for _, kv := range standInHooks {
		if _, err := cr.git("", "config", kv[0], kv[1]); err != nil {
			return err
		}
	}
	for _, st := range steps {
		if err := cr.step(st, s); err != nil {
			return err
		}
		if st.label != "" {
			sha, err := cr.git("", "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			s.labels[st.label] = sha
		}
	}
	return s.err
}

// step is one step of a scratch repository, in the order its keys are read.
func (cr *caseRun) step(st step, s *subst) error {
	if st.hasWrite {
		if _, err := writeFiles(cr.dir, st.write, s); err != nil {
			return err
		}
	}
	if st.hasStage {
		paths, err := writeFiles(cr.dir, st.stage, s)
		if err != nil {
			return err
		}
		if _, err := cr.git("", append([]string{"add", "-A", "--"}, paths...)...); err != nil {
			return err
		}
	}
	if st.commit != nil {
		paths, err := writeFiles(cr.dir, st.files, s)
		if err != nil {
			return err
		}
		if len(paths) > 0 {
			if _, err := cr.git("", append([]string{"add", "-A", "--"}, paths...)...); err != nil {
				return err
			}
		}
		if _, err := cr.git(s.text(*st.commit), "commit", "-q", "--allow-empty", "-F", "-"); err != nil {
			return err
		}
	}
	if st.branch != "" {
		if _, err := cr.git("", "checkout", "-q", "-b", st.branch); err != nil {
			return err
		}
	}
	if st.checkout != "" {
		if _, err := cr.git("", "checkout", "-q", s.text(st.checkout)); err != nil {
			return err
		}
	}
	if st.run != "" {
		if err := cr.sh(s.text(st.run)); err != nil {
			return err
		}
	}
	return s.err
}

// Outcome is what a case's run did: its exit code (nil when it did not exit), its output, and the
// files and git config entries the case names, as the run left them.
type Outcome struct {
	Exit           *int
	Stdout, Stderr string
	files          map[string]*written
	gitConfig      map[string]*string
}

// written is a file as a run left it.
type written struct {
	text       string
	executable bool
}

// RunCase runs one case in a folder of its own: its outcome, and the substitution its
// expectations are read with.
func (r *Runner) RunCase(c *Case) (*Outcome, *subst, error) {
	scratch, err := os.MkdirTemp(r.opts.TempRoot, "itos-conformance-")
	if err != nil {
		return nil, nil, err
	}
	p := newProcs(r.opts.CaseDeadline)
	var stopGitHub func()
	defer func() {
		p.close()
		if stopGitHub != nil {
			stopGitHub()
		}
		_ = os.RemoveAll(scratch)
	}()
	dir, home := filepath.Join(scratch, "case"), filepath.Join(scratch, "home")
	for _, d := range []string{dir, home} {
		if err := os.Mkdir(d, 0o755); err != nil {
			return nil, nil, err
		}
	}
	inspection := cleanEnv(home, r.runPath)
	inspection["ITOS_GIT"] = r.nativeGit
	noMaintenance(inspection)
	cr := &caseRun{r: r, p: p, dir: dir, inspection: environ(inspection)}
	s := &subst{dir: dir, path: r.runPath, git: r.nativeGit, version: r.version, release: r.release, labels: map[string]string{}}
	if c.hasGit {
		if err := cr.buildRepo(c.git, s); err != nil {
			return nil, nil, err
		}
	}
	if _, err := writeFiles(dir, c.files, s); err != nil {
		return nil, nil, err
	}
	command := cleanEnv(home, r.runPath)
	if c.github != nil {
		url, stop, err := serveGitHub(s.deep(c.github))
		if err != nil {
			return nil, nil, err
		}
		stopGitHub = stop
		command["GITHUB_API_URL"] = url
		command["GITHUB_REPOSITORY"] = fakeRepository
		command["GITHUB_TOKEN"] = fakeToken
	}
	for _, kv := range c.env {
		if kv.value == nil {
			delete(command, kv.key)
			continue
		}
		command[kv.key] = s.text(*kv.value)
	}
	noMaintenance(command)
	if len(c.hide) > 0 {
		hidden, err := hiddenPath(command["PATH"], func(name string) bool {
			for _, h := range c.hide {
				if h == name {
					return true
				}
			}
			return false
		}, scratch)
		if err != nil {
			return nil, nil, err
		}
		command["PATH"] = hidden
	}
	argv := make([]string, len(c.Argv))
	for i, a := range c.Argv {
		argv[i] = s.text(a)
	}
	input := ""
	if c.stdin != nil {
		input = s.text(*c.stdin)
	}
	cwd := dir
	if c.cwd != "" {
		cwd = filepath.Join(dir, c.cwd)
	}
	if s.err != nil {
		return nil, nil, s.err
	}
	out := &Outcome{files: map[string]*written{}, gitConfig: map[string]*string{}}
	res, err := p.run(r.bin, argv, cwd, environ(command), input)
	var deadline *DeadlineError
	var unusable *FixtureError
	switch {
	case errors.As(err, &deadline), errors.As(err, &unusable):
		return nil, nil, err
	case err != nil: // it could not start, as a case may expect
		out.Stderr = err.Error() + "\n"
	default:
		out.Exit, out.Stdout, out.Stderr = res.code, res.stdout, res.stderr
	}
	for _, e := range c.filesAfter {
		full := filepath.Join(dir, e.path)
		info, err := os.Stat(full)
		if err != nil {
			out.files[e.path] = nil
			continue
		}
		text, err := os.ReadFile(full)
		if err != nil {
			return nil, nil, err
		}
		out.files[e.path] = &written{text: string(text), executable: info.Mode().Perm()&0o111 != 0}
	}
	for _, kv := range c.gitConfigAfter {
		value, err := cr.configValue(kv.key)
		if err != nil {
			return nil, nil, err
		}
		out.gitConfig[kv.key] = value
	}
	if err := p.expiredErr(); err != nil {
		return nil, nil, err
	}
	return out, s, nil
}

// configValue is a key's values in the scratch repository's native Git config (git config
// --get-all, a value a line), nil when it is unset.
func (cr *caseRun) configValue(key string) (*string, error) {
	res, err := cr.p.run(cr.r.nativeGit, []string{"config", "--get-all", key}, cr.dir, cr.inspection, "")
	if err != nil {
		return nil, err
	}
	switch {
	case res.code != nil && *res.code == 1:
		return nil, nil
	case res.code == nil || *res.code != 0:
		return nil, fixtureErr("%s", strings.TrimRight("git config --get-all "+key+" failed:\n"+res.stderr, " \t\r\n"))
	}
	value := strings.TrimSuffix(res.stdout, "\n")
	return &value, nil
}

// Result is a case's verdict: what differs between its expectations and its outcome, none when
// it passes.
type Result struct {
	Fixture  *Fixture
	Case     *Case
	Problems []string
}

// Run runs every case of the fixtures, Jobs at a time, the results in the fixtures' order.
func (r *Runner) Run(fixtures []*Fixture) []Result {
	var results []Result
	for _, f := range fixtures {
		for _, c := range f.Cases {
			results = append(results, Result{Fixture: f, Case: c})
		}
	}
	work := make(chan int)
	var wg sync.WaitGroup
	for range min(r.opts.Jobs, max(1, len(results))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				results[i].Problems = r.judge(results[i].Case)
			}
		}()
	}
	for i := range results {
		work <- i
	}
	close(work)
	wg.Wait()
	return results
}

// judge runs one case and says what differs from what it expects.
func (r *Runner) judge(c *Case) []string {
	out, s, err := r.RunCase(c)
	if err != nil {
		return []string{"could not run: " + err.Error()}
	}
	problems := compare(c, out, s, r.opts.Additive)
	if s.err != nil {
		return []string{"could not run: " + s.err.Error()}
	}
	return problems
}
