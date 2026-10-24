package features

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cucumber/godog"
	"github.com/donvargax/itos/v8/internal/git"
)

type initShimFixture struct {
	path                             []string
	launcher, link, explicit, marker string
	before                           map[string]initShimFile
}

type initShimFile struct {
	info   os.FileInfo
	target string
	digest [32]byte
}

// Steps for the pinned-init shim identity scenarios. The fixture uses a real
// build as the global launcher and the release harness for the pinned copy.
func initializeInitShimIdentitySteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a repository with a valid pin and completed itos setup$`, func() error {
		w.initShim = &initShimFixture{before: map[string]initShimFile{}}
		if err := w.itosUnderTestAs("8.1.0"); err != nil {
			return err
		}
		if err := w.startReleaseServer("8.0.0"); err != nil {
			return err
		}
		if err := w.releaseIsItosUnderTest("8.0.0"); err != nil {
			return err
		}
		if err := w.itos("init", "--plugin", "no", "--no-git-shim", "--no-agent-rules"); err != nil {
			return err
		}
		if w.exit != 0 {
			return fmt.Errorf("initial setup failed\n%s", w.report())
		}
		// A doctor adapter records the inherited pin, distinguishing a pinned
		// init from the launcher answering init itself (bug-104).
		probe := filepath.Join(w.support, "init-runtime")
		if err := w.writeProgram(probe, "#!/bin/sh\nprintf '%s\\n' \"${ITOS_VERSION-}\" > "+quote(filepath.Join(w.support, "init-version"))+"\nprintf '%s\\n' '{\"protocol\":1,\"tests\":[],\"files\":[]}'\n"); err != nil {
			return err
		}
		w.config.settings = append(w.config.settings, setting{key: "tests.probe", yaml: true,
			value: fmt.Sprintf("{ adapter: { command: %q }, smoke: { file: init-smoke.yaml } }", quote(programPath(probe)))})
		if err := w.write("init-smoke.yaml", "[]\n"); err != nil {
			return err
		}
		if err := w.pinVersion("8.0.0", "8.0.0"); err != nil {
			return err
		}
		if err := w.write("README.md", "# scratch\n"); err != nil {
			return err
		}
		if err := w.write("tasks/phase-1.yaml", "[]\n"); err != nil {
			return err
		}
		return w.commit("chore: prepare pinned init fixture")
	})
	sc.Step(`^init executes the itos under test from that pin's cache$`, w.initShimCached)
	sc.Step(`^a distinct global itos launcher exists in a stable PATH folder$`, w.initShimGlobalLauncher)
	sc.Step(`^the cache folder is not on PATH$`, w.initShimCacheOffPath)
	sc.Step(`^a real Git executable follows the stable folder on PATH$`, w.initShimGitFollows)
	sc.Step(`^the first Git on PATH links to the global launcher rather than the running runtime$`, w.initShimPlaceGlobalFirst)
	sc.Step(`^its shim report identifies that first Git path as kept and effective before real Git$`, w.initShimReport)
	sc.Step(`^JSON, when requested, retains the existing git_shim fields and truthful on_path and before_git values$`, w.initShimJSON)
	sc.Step(`^text, when requested, does not call the effective shim missing or advise another cache link$`, w.initShimText)
	sc.Step(`^text, when requested, does not name the cached runtime's path$`, w.initShimNoCachePath)
	sc.Step(`^neither the global shim nor its target changed$`, w.initShimUnchanged)
	sc.Step(`^no Git shim was created in the runtime cache$`, w.initShimNoCacheLink)
	sc.Step(`^git_shim action is "([^"]*)" and link names the original stable Git path$`, w.initShimActionLink)
	sc.Step(`^the original target and executable identity are unchanged$`, w.initShimUnchanged)
	sc.Step(`^the pinned runtime's cache gained no Git link$`, w.initShimNoCacheLink)
	sc.Step(`^the rest of init was handled by the selected pinned runtime, not handed back to the launcher$`, w.initShimPinned)
	sc.Step(`^the first Git on PATH is (.+) of the distinct global itos$`, w.initShimIdentity)
	sc.Step(`^itos exits with code 0 with that path kept as the effective shim$`, w.initShimReport)
	sc.Step(`^the candidate's content, link representation and target are unchanged$`, w.initShimUnchanged)
	sc.Step(`^its text says that first Git path is itos and does not name the cached runtime's path$`, w.initShimText)
	sc.Step(`^real Git comes before the launcher shim on PATH$`, w.initShimRealFirst)
	sc.Step(`^the cache has no Git link$`, w.initShimNoCacheLink)
	sc.Step(`^its report does not claim that later launcher shim is effective$`, w.initShimNotEffective)
	sc.Step(`^it has not scanned past the first executable Git to keep that later path$`, w.initShimNotEffective)
	sc.Step(`^the first entry is instead an unrecognized executable Git wrapper$`, w.initShimWrapperFirst)
	sc.Step(`^that wrapper is not treated as itos merely from its Git filename$`, w.initShimNotEffective)
	sc.Step(`^init neither executes it for identification nor replaces it$`, w.initShimUnchanged)
	sc.Step(`^the first Git on PATH is an effective global itos shim$`, w.initShimPlaceGlobalFirst)
	sc.Step(`^itos exits with code 0 with git_shim action "([^"]*)" and a null link$`, w.initShimDeclined)
	sc.Step(`^the effective shim is unchanged$`, w.initShimUnchanged)
	sc.Step(`^an explicitly requested shim directory contains an ordinary Git executable$`, w.initShimOrdinaryDir)
	sc.Step(`^init is run with --git-shim and --git-shim-dir naming that directory$`, w.initShimExplicitRun)
	sc.Step(`^itos exits with code 1 using the existing refusal to replace ordinary Git$`, w.initShimRefused)
	sc.Step(`^the requested executable and the global shim are unchanged$`, w.initShimUnchanged)
	sc.Step(`^it has not substituted the already active PATH shim for the explicitly requested location$`, w.initShimExplicitReported)
}

func (w *world) initShimGlobalLauncher() error {
	bin, err := stampedItos("8.1.0")
	if err != nil {
		return err
	}
	f := w.initShim
	f.launcher = programPath(filepath.Join(w.support, "stable", "itos"))
	f.link = programPath(filepath.Join(w.support, "stable", "git"))
	if err := initShimCopy(bin, f.launcher); err != nil {
		return err
	}
	f.path = []string{filepath.Dir(f.link)}
	return w.initShimRemember(f.launcher)
}

func (w *world) initShimPlaceGlobalFirst() error {
	return w.initShimIdentity("an absolute symbolic link")
}
func (w *world) initShimIdentity(identity string) error {
	f := w.initShim
	target := f.launcher
	switch identity {
	case "a native copy retaining itos build information":
		if err := initShimCopy(target, f.link); err != nil {
			return err
		}
	case "a hard link to an itos Go binary":
		if err := os.Link(target, f.link); err != nil {
			return err
		}
	case "an absolute symbolic link", "a relative symbolic link", "a valid symbolic-link chain":
		if identity == "a relative symbolic link" {
			target = filepath.Base(target)
		}
		if identity == "a valid symbolic-link chain" {
			middle := filepath.Join(filepath.Dir(f.link), "middle")
			if err := initShimSymlink(target, middle); err != nil {
				return err
			}
			if err := w.initShimRemember(middle); err != nil {
				return err
			}
			target = middle
		}
		if err := initShimSymlink(target, f.link); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown identity %q", identity)
	}
	return w.initShimRemember(f.link)
}
func (w *world) initShimReport() error {
	if err := w.exitsWith(0); err != nil {
		return err
	}
	if strings.Contains(w.lastCommand, "--json") {
		return w.initShimJSON()
	}
	return w.initShimText()
}
func (w *world) initShimJSON() error {
	var report struct {
		GitShim struct {
			Action    string `json:"action"`
			Link      any    `json:"link"`
			OnPath    bool   `json:"on_path"`
			BeforeGit bool   `json:"before_git"`
		} `json:"git_shim"`
	}
	jsonStart := strings.Index(w.output(), "{")
	if !strings.Contains(w.lastCommand, "--json") {
		return nil
	}
	if jsonStart < 0 {
		return fmt.Errorf("init JSON report is missing\n%s", w.report())
	}
	if err := json.Unmarshal([]byte(w.output()[jsonStart:]), &report); err != nil {
		return fmt.Errorf("init JSON report is invalid: %w\n%s", err, w.report())
	}
	path := filepath.Join(w.support, "stable", programPath("git"))
	if report.GitShim.Action != "kept" || report.GitShim.Link != path || !report.GitShim.OnPath || !report.GitShim.BeforeGit {
		return fmt.Errorf("git_shim is %#v, want kept at %s, on_path and before_git\n%s", report.GitShim, path, w.report())
	}
	var fields struct {
		GitShim map[string]json.RawMessage `json:"git_shim"`
	}
	if err := json.Unmarshal([]byte(w.output()[jsonStart:]), &fields); err != nil {
		return err
	}
	if len(fields.GitShim) != 4 {
		return fmt.Errorf("git_shim shape changed: %s", w.stdout)
	}
	return nil
}
func (w *world) initShimText() error {
	if strings.Contains(w.lastCommand, "--json") {
		return nil
	}
	want := "The git shim is there already: " + w.initShim.link + " is itos."
	if !strings.Contains(w.output(), want+"\n") {
		return fmt.Errorf("missing text line %q\n%s", want, w.report())
	}
	if strings.Contains(w.output(), "not linked") || strings.Contains(w.output(), "--git-shim-dir") {
		return fmt.Errorf("init called the effective shim missing or advised another link\n%s", w.report())
	}
	return w.initShimNoCachePath()
}
func (w *world) initShimNoCachePath() error {
	cache := filepath.Join(w.cacheDir(), "8.0.0", "itos")
	if strings.Contains(w.output(), cache) {
		return fmt.Errorf("output names cached runtime %s\n%s", cache, w.report())
	}
	return nil
}
func (w *world) initShimUnchanged() error {
	for path, before := range w.initShim.before {
		if err := w.initShimRemember(path); err != nil {
			return err
		}
		after := w.initShim.before[path]
		if !os.SameFile(before.info, after.info) || before.info.Mode() != after.info.Mode() || before.digest != after.digest || before.target != after.target {
			return fmt.Errorf("candidate, representation or target changed: %s", path)
		}
	}
	if marker := w.initShim.marker; marker != "" {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			return fmt.Errorf("wrapper ran for identification (marker: %s): %v", marker, err)
		}
	}
	return nil
}
func (w *world) initShimNoCacheLink() error {
	path := programPath(filepath.Join(filepath.Dir(w.initShimCacheBin()), "git"))
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("cache gained Git link %s: %v", path, err)
	}
	return nil
}
func (w *world) initShimActionLink(action string) error {
	return w.initShimOutcome(action, filepath.Join(w.support, "stable", programPath("git")))
}
func (w *world) initShimRealFirst() error {
	if err := w.initShimPlaceGlobalFirst(); err != nil {
		return err
	}
	w.initShim.path = []string{filepath.Dir(git.Bin(runtime.GOOS)), filepath.Dir(w.initShim.link)}
	return nil
}
func (w *world) initShimNotEffective() error {
	if err := w.exitsWith(0); err != nil {
		return err
	}
	return w.initShimOutcome("offered", programPath(filepath.Join(filepath.Dir(w.initShimCacheBin()), "git")))
}
func (w *world) initShimWrapperFirst() error {
	f := w.initShim
	f.marker = filepath.Join(w.support, "wrapper-probed")
	path := filepath.Join(w.support, "wrapper", "git")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Ordinary init work uses ITOS_GIT, so the marker observes only an
	// illicit probe of the wrapper for identification.
	w.vars = append(w.vars, "ITOS_GIT="+git.Bin(runtime.GOOS))
	script := "#!/bin/sh\nprintf probe > " + quote(f.marker) + "\nexec " + quote(git.Bin(runtime.GOOS)) + " \"$@\"\n"
	if err := w.writeProgram(path, script); err != nil {
		return err
	}
	if err := w.initShimRemember(programPath(path)); err != nil {
		return err
	}
	f.path = []string{filepath.Dir(path), filepath.Dir(f.link)}
	return w.itos("init", "--plugin", "no", "--no-agent-rules", "--json")
}
func (w *world) initShimDeclined(action string) error {
	if err := w.exitsWith(0); err != nil {
		return err
	}
	return w.initShimOutcome(action, "")
}
func (w *world) initShimOrdinaryDir() error {
	f := w.initShim
	f.explicit = programPath(filepath.Join(w.support, "requested", "git"))
	if err := initShimCopy(git.Bin(runtime.GOOS), f.explicit); err != nil {
		return err
	}
	return w.initShimRemember(f.explicit)
}
func (w *world) initShimExplicitRun() error {
	return w.itos("init", "--plugin", "no", "--no-agent-rules", "--git-shim", "--git-shim-dir", filepath.Dir(w.initShim.explicit), "--json")
}

func (w *world) initShimRefused() error {
	if err := w.exitsWith(1); err != nil {
		return err
	}
	var report struct {
		GitShim struct {
			Problem string `json:"problem"`
		} `json:"git_shim"`
	}
	if err := json.Unmarshal([]byte(w.stdout), &report); err != nil {
		return err
	}
	if !strings.Contains(report.GitShim.Problem, "init never replaces") {
		return fmt.Errorf("missing ordinary-Git refusal: %s", w.stdout)
	}
	return w.initShimExplicitReported()
}
func (w *world) initShimExplicitReported() error {
	return w.initShimOutcome("refused", w.initShim.explicit)
}

func (w *world) initShimPinned() error {
	data, err := os.ReadFile(filepath.Join(w.support, "init-version"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != "8.0.0" {
		return fmt.Errorf("init adapter ran under version %q, want pin 8.0.0", data)
	}
	return nil
}

func (w *world) initShimCacheBin() string {
	return programPath(filepath.Join(w.cacheDir(), "8.0.0", "itos"))
}
func (w *world) initShimCached() error {
	if err := w.itos("version"); err != nil {
		return err
	}
	if err := w.exitsWith(0); err != nil {
		return err
	}
	return w.initShimRemember(w.initShimCacheBin())
}
func initShimCopy(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return writeExecutable(to, data)
}
func initShimSymlink(target, link string) error {
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(link), target)
		}
		err = os.Link(target, link)
	}
	return err
}
func (w *world) initShimRemember(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	target := ""
	if linfo, err := os.Lstat(path); err != nil {
		return err
	} else if linfo.Mode()&os.ModeSymlink != 0 {
		target, err = os.Readlink(path)
		if err != nil {
			return err
		}
	}
	w.initShim.before[path] = initShimFile{info: info, digest: sha256.Sum256(data), target: target}
	return nil
}
func (w *world) initShimCacheOffPath() error {
	for _, kv := range w.env() {
		if strings.HasPrefix(kv, "PATH=") {
			for _, dir := range filepath.SplitList(strings.TrimPrefix(kv, "PATH=")) {
				if filepath.Clean(dir) == filepath.Dir(w.initShimCacheBin()) {
					return fmt.Errorf("cache is on PATH")
				}
			}
		}
	}
	return nil
}
func (w *world) initShimGitFollows() error {
	if len(w.initShim.path) != 1 || w.basePath() == "" {
		return fmt.Errorf("stable folder must precede real Git")
	}
	return nil
}

func (w *world) initShimOutcome(action, link string) error {
	var report struct {
		GitShim struct {
			Action string `json:"action"`
			Link   any    `json:"link"`
		} `json:"git_shim"`
	}
	jsonStart := strings.Index(w.output(), "{")
	if jsonStart < 0 {
		return fmt.Errorf("init JSON report is missing\n%s", w.report())
	}
	if err := json.Unmarshal([]byte(w.output()[jsonStart:]), &report); err != nil {
		return fmt.Errorf("init JSON report is invalid: %w\n%s", err, w.report())
	}
	if report.GitShim.Action != action {
		return fmt.Errorf("git_shim action is %q, want %q\n%s", report.GitShim.Action, action, w.report())
	}
	if link == "" {
		if report.GitShim.Link != nil {
			return fmt.Errorf("git_shim link is %#v, want null\n%s", report.GitShim.Link, w.report())
		}
	} else if report.GitShim.Link != link {
		return fmt.Errorf("git_shim link is %#v, want %s\n%s", report.GitShim.Link, link, w.report())
	}
	return nil
}
