// The itos under test as a pinned version (slice 116, status.feature). In a
// pinned repository the launcher runs the pinned version, and runs the
// binary that was called only when the pin is that binary's own version
// (pin.feature), so a scenario about what the pinned itos says pins the itos
// under test itself: this tree's ./cmd/itos built stamped with the version
// the scenario names, as tools/bin/itos stamps a build with dev-version's,
// once a run for each version, into a folder of its own removed after the
// run (never .tools/bin/itos, the binary the hooks run). It is a -cover build
// when the run measures coverage (cover_test.go), so its coverage is the
// scenario's as tools/bin/itos's would be.
//
// The release server offers the version too, as a fake whose run exits 97,
// so a launcher that ran it in place of the itos under test fails the
// scenario's exit code instead of passing on the fake's output. The
// launcher's own notice of a newer release (update.feature) is silenced with
// ITOS_NO_UPDATE_NOTICE: it names the newer release and itos upgrade on its
// own, so left on it would answer for the very lines the scenario reads.
//
// A binary ITOS_BIN names cannot be stamped again, so these scenarios build
// the tree the features are run from even then: run as the last release's
// suite by tools/bin/previous-release, they judge that release's own code,
// not the binary ITOS_BIN names.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
)

// The builds of the itos under test stamped with another version, each
// version's binary, all in one folder removed after the run.
var stamped struct {
	sync.Mutex
	dir  string
	bins map[string]string
}

// stampedItos is the itos under test built from this tree stamped with the
// version, built the first time a scenario asks for it.
func stampedItos(version string) (string, error) {
	stamped.Lock()
	defer stamped.Unlock()
	if bin, ok := stamped.bins[version]; ok {
		return bin, nil
	}
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	if stamped.dir == "" {
		if stamped.dir, err = os.MkdirTemp("", "itos-features-stamped-"); err != nil {
			return "", err
		}
		stamped.bins = map[string]string{}
	}
	bin := programPath(filepath.Join(stamped.dir, version, "itos"))
	args := []string{"build"}
	if coverBin != "" {
		args = append(args, "-cover", "-covermode=atomic")
	}
	args = append(args, "-trimpath", "-ldflags", "-s -w -X "+versionStamp+"="+version, "-o", bin, "./cmd/itos")
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if text, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building the itos under test stamped %s: %v\n%s", version, err, text)
	}
	stamped.bins[version] = bin
	return bin, nil
}

// removeStamped removes the stamped builds, once the run is over.
func removeStamped() {
	stamped.Lock()
	defer stamped.Unlock()
	if stamped.dir != "" {
		_ = os.RemoveAll(stamped.dir)
	}
}

// A release's version without a pre-release, which GitHub's latest release
// never is.
var stableVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// The config pins the itos under test, stamped as the version v, and a
// release server offers v and other, its newest the newer of them that is
// no pre-release (other, unless it is one).
func (w *world) pinItosUnderTest(v, other string) error {
	bin, err := stampedItos(v)
	if err != nil {
		return err
	}
	w.bin = bin
	if err := w.startReleaseServer(v); err != nil {
		return err
	}
	if err := w.offerRelease(other, ""); err != nil {
		return err
	}
	if stableVersion.MatchString(other) {
		w.releases.latest = other
	}
	if err := w.versionExitsWith(v, 97); err != nil {
		return err
	}
	w.vars = append(w.vars, "ITOS_NO_UPDATE_NOTICE=1")
	return w.pinVersion(v, v)
}
