// The release server: fake releases of itos served over HTTP from inside the
// test, so the scenarios of the launcher (pin.feature) fetch, check and run a
// version without the network. Each release is what a real one holds for the
// running platform, itos-<version>-<os>-<arch>.tar.gz (.zip on windows) and a
// checksums.txt listing it, served at <base>/download/v<version>/<asset>, the
// base being what ITOS_RELEASES names, and the newest of them at
// <base>/latest/download/<asset> as well, as GitHub serves its latest release. The archive's itos is a shell script
// (on windows script-exe running one, program_test.go) that records the version it is, the ITOS_VERSION it ran with and its
// arguments, and exits with the code the scenario chose for it.
//
// No scenario reaches the network or a real cache: ITOS_CACHE is always a
// folder of the scenario's, and ITOS_RELEASES the release server when the
// scenario starts one, else an address nothing answers on.
package features

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"go.yaml.in/yaml/v3"
)

// Where ITOS_RELEASES points when the scenario starts no release server: a
// port nothing listens on, so a fetch fails at once and never leaves the
// machine.
const noReleaseServer = "http://127.0.0.1:1/no-release-server"

// The fake releases and what was asked of them.
type releaseServer struct {
	server *httptest.Server
	mu     sync.Mutex
	files  map[string][]byte // what each path serves
	latest string            // the newest version, which <base>/latest/ serves
	asked  []string          // each path asked for, in order
	// Whether the scenario made it unreachable: itos is then given an address
	// nothing answers on in its place.
	unreachable bool
	// The cache the first download of an archive waits on, when the scenario
	// holds it (bug 44): it is served once another run has cached the
	// release there. "" when nothing is held.
	holdCache string
	archives  int         // the archives asked for so far, while the hold is set
	cachedBy  os.FileInfo // the release folder the other run cached, as the held download found it
}

// How long the held download waits for the other run to cache the release
// before it is served anyway, so a run that never caches it cannot hang the
// scenario; otherRunsReleaseKept then says so.
const holdTimeout = 30 * time.Second

func (r *releaseServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.asked = append(r.asked, req.URL.Path)
	path := req.URL.Path
	if asset, ok := strings.CutPrefix(path, "/latest/download/"); ok && r.latest != "" {
		path = releasePath(r.latest, asset)
	}
	body, ok := r.files[path]
	held := false
	if r.holdCache != "" && ok && !strings.HasSuffix(path, "/checksums.txt") {
		r.archives++
		held = r.archives == 1
	}
	r.mu.Unlock()
	if held {
		r.waitForCache(path)
	}
	if !ok {
		http.NotFound(rw, req)
		return
	}
	_, _ = rw.Write(body)
}

// waitForCache holds the download of the archive at path until another run
// has cached its version, <cache>/<version>/ holding its checksums.txt, which
// a run moves into place with the binary as one folder; it keeps that folder
// as found, for otherRunsReleaseKept.
func (r *releaseServer) waitForCache(path string) {
	version, _, _ := strings.Cut(strings.TrimPrefix(path, "/download/v"), "/")
	dir := filepath.Join(r.holdCache, version)
	for deadline := time.Now().Add(holdTimeout); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if _, err := os.Stat(filepath.Join(dir, "checksums.txt")); err != nil {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		// On windows os.Stat reads a file's identity only when it is first
		// compared, from the path: it is read now, while the folder is the
		// one the other run cached.
		os.SameFile(info, info)
		r.mu.Lock()
		r.cachedBy = info
		r.mu.Unlock()
		return
	}
}

func (r *releaseServer) set(path string, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[path] = body
}

func (r *releaseServer) get(path string) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.files[path]
}

func (r *releaseServer) requests() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.asked)
}

// A release's path on the server, as the launcher asks for it.
func releasePath(version, asset string) string {
	return "/download/v" + version + "/" + asset
}

// The asset holding the binary for the platform the tests run on, which is
// the one the binary under test runs on.
func platformArchive(version string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("itos-%s-%s-%s.zip", version, runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf("itos-%s-%s-%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func initializeReleaseSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^a release server offering the versions "([^"]*)" and "([^"]*)"$`, func(a, b string) error {
		return w.startReleaseServer(a, b)
	})
	sc.Step(`^the version "([^"]*)" exits with code (\d+)$`, w.versionExitsWith)
	sc.Step(`^the config pins the version "([^"]*)" of the release server$`, func(version string) error {
		return w.pinVersion(version, version)
	})
	sc.Step(`^the config pins the version "([^"]*)" with the checksums of "([^"]*)"$`, w.pinVersion)
	sc.Step(`^the release server's checksums\.txt of "([^"]*)" is replaced$`, w.replaceChecksums)
	sc.Step(`^the release server's archive of "([^"]*)" for this platform is replaced$`, w.replaceArchive)
	sc.Step(`^the release server cannot be reached$`, w.releaseServerUnreachable)
	sc.Step(`^the release server holds the first download of a release until the other run has cached it$`, w.holdFirstDownload)
	sc.Step(`^the release the other run cached is still the one in the cache$`, w.otherRunsReleaseKept)
	sc.Step(`^the repository has no itos\.yaml$`, w.noConfig)
	sc.Step(`^([A-Z][A-Z0-9_]*) is "([^"]*)"$`, w.setVariable)
	sc.Step(`^itos has already run "([^"]*)"$`, w.alreadyRan)
	sc.Step(`^the launcher's last answer from the release server, (a minute|an hour|two hours|a day) old, is "([^"]*)"$`, w.lastAnswer)
	sc.Step(`^the cache holds the release "([^"]*)"$`, w.cacheHolds)

	sc.Step(`^itos runs "([^"]*)"$`, func(args string) error { return w.itos(strings.Fields(args)...) })
	// The command line split as a shell splits it, its quotes kept together
	// (slice 54), for an argument that holds spaces.
	sc.Step(`^itos runs the command line "([^"]*)"$`, func(line string) error {
		args, err := shellWords(line)
		if err != nil {
			return err
		}
		return w.itos(args...)
	})
	sc.Step(`^itos runs "([^"]*)" from "([^"]*)"$`, func(args, folder string) error {
		return w.itosIn(filepath.Join(w.dir, folder), strings.Fields(args)...)
	})

	sc.Step(`^the config has the comment "([^"]*)"$`, w.configComment)

	sc.Step(`^the config's pin is the version "([^"]*)" of the release server, with its checksums$`, w.configPins)
	sc.Step(`^the config still has the comment "([^"]*)"$`, w.configStillHasComment)
	sc.Step(`^the config is unchanged$`, w.configUnchanged)
	sc.Step(`^the version "([^"]*)" ran with the arguments "([^"]*)"$`, w.versionRanWithArguments)
	sc.Step(`^the version "([^"]*)" ran with ITOS_VERSION "([^"]*)"$`, w.versionRanWithVersion)
	sc.Step(`^the version "([^"]*)" ran with the arguments "([^"]*)" twice$`, func(version, args string) error {
		return w.versionRanTimes(version, args, 2)
	})
	// Two runs of the same command line at once (bug 44), started together as
	// allAtOnce starts them, each its own exit and output.
	sc.Step(`^two runs of itos "([^"]*)" start at once$`, func(line string) error {
		return w.allAtOnce([][]string{strings.Fields(line), strings.Fields(line)})
	})
	sc.Step(`^both runs exit with code 0$`, w.everyRunExited0)
	sc.Step(`^no version of the release server ran$`, func() error { return w.noVersionRan(false) })
	sc.Step(`^no version of the release server ran since the last run$`, func() error { return w.noVersionRan(true) })
	sc.Step(`^the release server was asked for nothing$`, func() error { return w.askedForNothing(false) })
	sc.Step(`^the release server was asked for nothing since the last run$`, func() error { return w.askedForNothing(true) })
}

// The release server, each version's release on it, the last one named the
// newest.
func (w *world) startReleaseServer(versions ...string) error {
	r := &releaseServer{files: map[string][]byte{}}
	r.server = httptest.NewServer(r)
	w.releases = r
	for _, version := range versions {
		if err := w.offerRelease(version, ""); err != nil {
			return err
		}
	}
	r.latest = versions[len(versions)-1]
	return nil
}

// The release server holds the first archive asked for until another run has
// cached the release, so of two runs fetching it at once the held one always
// finishes its fetch with the release already cached by the other: the race
// is forced every time, not left to chance (bug 44).
func (w *world) holdFirstDownload() error {
	if err := w.needReleases(); err != nil {
		return err
	}
	w.releases.mu.Lock()
	defer w.releases.mu.Unlock()
	w.releases.holdCache = w.cacheDir()
	return nil
}

// The release folder the other run cached while the held download waited is
// still the one in the cache: no run removed or replaced it.
func (w *world) otherRunsReleaseKept() error {
	w.releases.mu.Lock()
	cached := w.releases.cachedBy
	w.releases.mu.Unlock()
	if cached == nil {
		return fmt.Errorf("no run cached the release while the held download waited %s", holdTimeout)
	}
	dir := filepath.Join(w.releases.holdCache, cached.Name())
	now, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("the release the other run cached is gone: %v", err)
	}
	if !os.SameFile(cached, now) {
		return fmt.Errorf("the release the other run cached, %s, was replaced by another folder", dir)
	}
	return nil
}

func (w *world) releaseServerUnreachable() error {
	if err := w.needReleases(); err != nil {
		return err
	}
	w.releases.unreachable = true
	return nil
}

// A release of the version: its archive for this platform, whose itos says
// extra beside what every one says when extra is not empty, and its
// checksums.txt.
func (w *world) offerRelease(version, extra string) error {
	itos, err := w.program(w.fakeItos(version, extra))
	if err != nil {
		return err
	}
	archive, err := releaseArchive(string(itos))
	if err != nil {
		return err
	}
	asset := platformArchive(version)
	w.releases.set(releasePath(version, asset), archive)
	sums := fmt.Sprintf("%s  %s\n%s  itos.schema.json\n", sha256Hex(archive), asset, sha256Hex([]byte(version)))
	w.releases.set(releasePath(version, "checksums.txt"), []byte(sums))
	return nil
}

// The file every fake itos appends its run to, a line each: its version, the
// ITOS_VERSION it ran with and its arguments, tab-separated.
func (w *world) ranFile() string { return filepath.Join(w.support, "ran") }

// The file holding the code the version exits with, 0 when there is none.
func (w *world) exitFile(version string) string {
	return filepath.Join(w.support, "exit-"+version)
}

func (w *world) fakeItos(version, extra string) string {
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\t%%s\t%%s\n' %s "${ITOS_VERSION-}" "$*" >> %s
printf 'itos %%s\n' %s
code=$(cat %s 2>/dev/null) || code=0
exit "${code:-0}"
`, quote(version), quote(w.ranFile()), quote(version), quote(w.exitFile(version)))
	if extra != "" {
		script += "# " + extra + "\n"
	}
	return script
}

// An archive as a release's: the binary, LICENSE and README.md at its top.
func releaseArchive(itos string) ([]byte, error) {
	files := []struct {
		name, text string
		mode       int64
	}{{"itos", itos, 0o755}, {"LICENSE", "MIT\n", 0o644}, {"README.md", "# itos\n", 0o644}}
	var b bytes.Buffer
	if runtime.GOOS == "windows" {
		z := zip.NewWriter(&b)
		for _, f := range files {
			name := f.name
			if name == "itos" {
				name = "itos.exe"
			}
			out, err := z.Create(name)
			if err != nil {
				return nil, err
			}
			if _, err := out.Write([]byte(f.text)); err != nil {
				return nil, err
			}
		}
		if err := z.Close(); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	gz := gzip.NewWriter(&b)
	t := tar.NewWriter(gz)
	for _, f := range files {
		if err := t.WriteHeader(&tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.text)), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if _, err := t.Write([]byte(f.text)); err != nil {
			return nil, err
		}
	}
	if err := t.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (w *world) needReleases() error {
	if w.releases == nil {
		return fmt.Errorf("the scenario starts no release server")
	}
	return nil
}

func (w *world) versionExitsWith(version string, code int) error {
	return os.WriteFile(w.exitFile(version), []byte(fmt.Sprintf("%d\n", code)), 0o644)
}

// The config pins the version, its checksums the SHA-256 of the checksums.txt
// the release server has for another version, or the same one.
func (w *world) pinVersion(version, of string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	sums := w.releases.get(releasePath(of, "checksums.txt"))
	if sums == nil {
		return fmt.Errorf("the release server has no version %s", of)
	}
	w.config.pin = &[2]string{version, sha256Hex(sums)}
	return w.writeConfig()
}

// The version's checksums.txt replaced by another that still lists its
// archive as it is, so that only the pin can refuse it.
func (w *world) replaceChecksums(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	path := releasePath(version, "checksums.txt")
	sums := w.releases.get(path)
	if sums == nil {
		return fmt.Errorf("the release server has no version %s", version)
	}
	other := fmt.Sprintf("%s  itos-%s-plan9-amd64.tar.gz\n", sha256Hex([]byte("replaced")), version)
	w.releases.set(path, append(slices.Clone(sums), other...))
	return nil
}

// The version's archive replaced by another whose itos differs, its
// checksums.txt left as it was, so that only that list can refuse it.
func (w *world) replaceArchive(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	path := releasePath(version, platformArchive(version))
	if w.releases.get(path) == nil {
		return fmt.Errorf("the release server has no version %s", version)
	}
	itos, err := w.program(w.fakeItos(version, "replaced"))
	if err != nil {
		return err
	}
	archive, err := releaseArchive(string(itos))
	if err != nil {
		return err
	}
	w.releases.set(path, archive)
	return nil
}

func (w *world) noConfig() error {
	err := os.Remove(filepath.Join(w.dir, "itos.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// A variable of the environment itos runs in.
func (w *world) setVariable(name, val string) error {
	w.vars = append(w.vars, name+"="+val)
	return nil
}

// The ages a scenario gives the launcher's last answer.
var answerAges = map[string]time.Duration{
	"a minute":  time.Minute,
	"an hour":   time.Hour,
	"two hours": 2 * time.Hour,
	"a day":     24 * time.Hour,
}

// The launcher's last answer from the release server, the newest version it
// named, written where the launcher keeps it (<cache>/state/latest, "<unix
// seconds> <version>") as had age ago, so that the run reads it as it would
// one it had asked for itself.
func (w *world) lastAnswer(age, version string) error {
	file := filepath.Join(w.cacheDir(), "state", "latest")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	at := time.Now().Add(-answerAges[age]).Unix()
	return os.WriteFile(file, []byte(fmt.Sprintf("%d %s\n", at, version)), 0o644)
}

// The cache holds the release server's version, its binary and checksums.txt
// in <cache>/<version>/, as the launcher leaves a release it fetched, without
// the server being asked for it.
func (w *world) cacheHolds(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	sums := w.releases.get(releasePath(version, "checksums.txt"))
	if sums == nil {
		return fmt.Errorf("the release server has no version %s", version)
	}
	dir := filepath.Join(w.cacheDir(), version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := w.writeProgram(filepath.Join(dir, "itos"), w.fakeItos(version, "")); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "checksums.txt"), sums, 0o644)
}

// The cache every run of itos is given, ITOS_CACHE.
func (w *world) cacheDir() string { return filepath.Join(w.support, "cache") }

func (w *world) alreadyRan(args string) error {
	if err := w.itos(strings.Fields(args)...); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("itos %s exited %d before the run the scenario is about\n%s", args, w.exit, w.report())
	}
	// init --stealth wrote itos's data in the git folder, where the steps
	// that read the ledger and the registry look for it from now on.
	if fields := strings.Fields(args); len(fields) > 0 && fields[0] == "init" && slices.Contains(fields, "--stealth") {
		w.dataDir = stealthDir
	}
	return nil
}

// One run of a fake itos, as it recorded it.
type fakeRun struct{ version, itosVersion, args string }

// The fake itos runs recorded, all of them or those since the last run of
// itos began.
func (w *world) fakeRuns(sinceLast bool) ([]fakeRun, error) {
	text, err := os.ReadFile(w.ranFile())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var runs []fakeRun
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) == 3 {
			runs = append(runs, fakeRun{parts[0], parts[1], parts[2]})
		}
	}
	if sinceLast {
		runs = runs[min(w.ranMark, len(runs)):]
	}
	return runs, nil
}

// The runs of the last run of itos that the version made.
func (w *world) lastRunsOf(version string) ([]fakeRun, error) {
	runs, err := w.fakeRuns(true)
	if err != nil {
		return nil, err
	}
	var of []fakeRun
	for _, r := range runs {
		if r.version == version {
			of = append(of, r)
		}
	}
	if len(of) == 0 {
		return nil, fmt.Errorf("the version %s did not run; the runs: %v\n%s", version, runs, w.report())
	}
	return of, nil
}

func (w *world) versionRanWithArguments(version, args string) error {
	runs, err := w.lastRunsOf(version)
	if err != nil {
		return err
	}
	for _, r := range runs {
		if r.args == args {
			return nil
		}
	}
	return fmt.Errorf("the version %s did not run with the arguments %q; its runs: %v\n%s", version, args, runs, w.report())
}

// The version ran with the arguments exactly times times in the last run of
// itos, or the runs started at once.
func (w *world) versionRanTimes(version, args string, times int) error {
	runs, err := w.lastRunsOf(version)
	if err != nil {
		return err
	}
	n := 0
	for _, r := range runs {
		if r.args == args {
			n++
		}
	}
	if n != times {
		return fmt.Errorf("the version %s ran with the arguments %q %d times, not %d; its runs: %v\n%s", version, args, n, times, runs, w.report())
	}
	return nil
}

func (w *world) versionRanWithVersion(version, itosVersion string) error {
	runs, err := w.lastRunsOf(version)
	if err != nil {
		return err
	}
	for _, r := range runs {
		if r.itosVersion == itosVersion {
			return nil
		}
	}
	return fmt.Errorf("the version %s did not run with ITOS_VERSION %q; its runs: %v\n%s", version, itosVersion, runs, w.report())
}

func (w *world) noVersionRan(sinceLast bool) error {
	runs, err := w.fakeRuns(sinceLast)
	if err != nil {
		return err
	}
	if len(runs) > 0 {
		return fmt.Errorf("a version of the release server ran: %v\n%s", runs, w.report())
	}
	return nil
}

func (w *world) askedForNothing(sinceLast bool) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	asked := w.releases.requests()
	if sinceLast {
		asked = asked[min(w.askedMark, len(asked)):]
	}
	if len(asked) > 0 {
		return fmt.Errorf("the release server was asked for %v\n%s", asked, w.report())
	}
	return nil
}

// The environment that keeps a run of itos off the network and off any real
// cache, then the variables the scenario set.
func (w *world) launcherEnv() []string {
	env := []string{"ITOS_CACHE=" + w.cacheDir()}
	switch {
	case w.releases != nil && w.releases.unreachable:
		env = append(env, "ITOS_RELEASES="+noReleaseServer)
	case w.releases != nil:
		env = append(env, "ITOS_RELEASES="+w.releases.server.URL)
	default:
		// No scenario without a release server of its own asks for the newest
		// release (slice 28), so none waits on an address that never answers.
		env = append(env, "ITOS_RELEASES="+noReleaseServer, "ITOS_NO_UPDATE=1")
	}
	return append(env, w.vars...)
}

// Where the records of the run about to start begin, so a step can read
// that run's alone.
func (w *world) markRun() {
	if runs, err := w.fakeRuns(false); err == nil {
		w.ranMark = len(runs)
	}
	if w.releases != nil {
		w.askedMark = len(w.releases.requests())
	}
	w.configBefore, _ = os.ReadFile(w.configPath())
	if w.snapshotRuns {
		w.filesBefore, _ = w.snapshot()
	}
}

// The config itos finds in the scratch repository: itos.yaml in the root, or
// the stealth one in the git folder.
func (w *world) configPath() string { return filepath.Join(w.dir, w.data("itos.yaml")) }

// The config pins the version, with the SHA-256 of the checksums.txt the
// release server has for it, read back from the file as YAML.
func (w *world) configPins(version string) error {
	if err := w.needReleases(); err != nil {
		return err
	}
	sums := w.releases.get(releasePath(version, "checksums.txt"))
	if sums == nil {
		return fmt.Errorf("the release server has no version %s", version)
	}
	text, err := os.ReadFile(w.configPath())
	if err != nil {
		return err
	}
	var config struct {
		Pin struct{ Version, Checksums string }
	}
	if err := yaml.Unmarshal(text, &config); err != nil {
		return fmt.Errorf("the config is not YAML: %v\n%s", err, text)
	}
	if config.Pin.Version != version || config.Pin.Checksums != sha256Hex(sums) {
		return fmt.Errorf("the config pins %q with the checksums %q, not %q with %q\n%s\n%s",
			config.Pin.Version, config.Pin.Checksums, version, sha256Hex(sums), text, w.report())
	}
	return nil
}

// A comment line in the config, after the pin's line, kept whenever the
// scenario writes the config again.
func (w *world) configComment(comment string) error {
	w.config.comments = append(w.config.comments, comment)
	return w.writeConfig()
}

func (w *world) configStillHasComment(comment string) error {
	text, err := os.ReadFile(w.configPath())
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(text), "\n") {
		if line == comment {
			return nil
		}
	}
	return fmt.Errorf("the config has no line %q:\n%s\n%s", comment, text, w.report())
}

// The config byte for byte as it was before the last run of itos.
func (w *world) configUnchanged() error {
	if w.configBefore == nil {
		return fmt.Errorf("there was no config before the last run")
	}
	text, err := os.ReadFile(w.configPath())
	if err != nil {
		return err
	}
	if !bytes.Equal(text, w.configBefore) {
		return fmt.Errorf("the config changed; it was:\n%s\nit is:\n%s\n%s", w.configBefore, text, w.report())
	}
	return nil
}

func (w *world) stopReleaseServer() {
	if w.releases != nil {
		w.releases.server.Close()
	}
}
