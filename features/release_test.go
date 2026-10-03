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

	"github.com/cucumber/godog"
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
}

func (r *releaseServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.asked = append(r.asked, req.URL.Path)
	path := req.URL.Path
	if asset, ok := strings.CutPrefix(path, "/latest/download/"); ok && r.latest != "" {
		path = releasePath(r.latest, asset)
	}
	body, ok := r.files[path]
	r.mu.Unlock()
	if !ok {
		http.NotFound(rw, req)
		return
	}
	_, _ = rw.Write(body)
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
	sc.Step(`^the repository has no itos\.yaml$`, w.noConfig)
	sc.Step(`^([A-Z][A-Z0-9_]*) is "([^"]*)"$`, w.setVariable)
	sc.Step(`^itos has already run "([^"]*)"$`, w.alreadyRan)

	sc.Step(`^itos runs "([^"]*)"$`, func(args string) error { return w.itos(strings.Fields(args)...) })
	sc.Step(`^itos runs "([^"]*)" from "([^"]*)"$`, func(args, folder string) error {
		return w.itosIn(filepath.Join(w.dir, folder), strings.Fields(args)...)
	})

	sc.Step(`^the version "([^"]*)" ran with the arguments "([^"]*)"$`, w.versionRanWithArguments)
	sc.Step(`^the version "([^"]*)" ran with ITOS_VERSION "([^"]*)"$`, w.versionRanWithVersion)
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

func (w *world) alreadyRan(args string) error {
	if err := w.itos(strings.Fields(args)...); err != nil {
		return err
	}
	if w.exit != 0 {
		return fmt.Errorf("itos %s exited %d before the run the scenario is about\n%s", args, w.exit, w.report())
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
	env := []string{"ITOS_CACHE=" + filepath.Join(w.support, "cache")}
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
}

func (w *world) stopReleaseServer() {
	if w.releases != nil {
		w.releases.server.Close()
	}
}
