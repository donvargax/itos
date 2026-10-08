// Coverage of the itos under test (T-121). The features run itos as another
// process, where Go records coverage only for a binary built with -cover and
// run with GOCOVERDIR naming an existing folder to write it to (go.dev/doc/
// build-cover). So when GOCOVERDIR or ITOS_CC_TEST_COVERDIR is set, the run
// builds the itos under test once with go build -cover, stamped and linked as
// tools/bin/itos builds itos, into a folder of its own (never .tools/bin/itos,
// the binary the hooks run), and every scenario runs that build in place of
// tools/bin/itos. A binary ITOS_BIN names is the caller's choice, run as it
// is. With neither variable set, nothing here does anything.
//
// The build is -covermode=atomic, the one mode in which a process can write
// its counters itself (runtime/coverage.WriteCountersDir) before it ends
// without exiting: on unix the launcher, an extension's call and the git
// shim replace themselves with syscall.Exec, which runs none of the exit
// hooks that write a -cover binary's counters, so what such a process ran is
// recorded only if it writes them before it execs.
//
// GOCOVERDIR is the caller's environment and reaches every command as it is,
// so the whole run's coverage lands in that one folder. ITOS_CC_TEST_COVERDIR
// splits it by scenario, for itos-cc, which runs only the scenarios whose
// coverage reaches a mutant's line: every command a scenario starts runs with
// GOCOVERDIR=<ITOS_CC_TEST_COVERDIR>/<its @ID tag without the @>, a folder
// made before the scenario's first command, and that wins over a GOCOVERDIR
// also set. It has to reach every itos a scenario starts, through git's
// hooks, the git shim, the launcher's child or an extension's $ITOS_BIN,
// since a -cover binary without its folder writes a warning to stderr that
// would break any scenario reading itos's output: w.env() sets it, every
// command the steps start runs in w.env(), and itos hands its own environment
// on to what it starts. w.env() drops every ITOS_ variable from the commands'
// environment, so the harness reads ITOS_CC_TEST_COVERDIR itself, here. The
// harness starts no itos outside a scenario; the cover build and script-exe's
// are go builds, which write no coverage.
package features

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// The variable itos-cc sets to the folder each scenario's coverage goes
// under, in a folder named after the scenario's ID.
const scenarioCoverEnv = "ITOS_CC_TEST_COVERDIR"

// The variable the version's stamp is linked into, as tools/bin/itos links it.
const versionStamp = "github.com/donvargax/itos/v7/internal/version.stamp"

// coverBin is the -cover build of the itos under test, when the run made one:
// what a scenario runs unless ITOS_BIN names a binary.
var coverBin string

// buildCover builds the itos under test with -cover into a folder of its own
// when the run measures coverage (GOCOVERDIR or ITOS_CC_TEST_COVERDIR set)
// and ITOS_BIN names no binary, sets coverBin to it, and gives what removes
// the folder after the run. It builds nothing otherwise.
func buildCover() (func(), error) {
	nothing := func() {}
	if os.Getenv("ITOS_BIN") != "" || os.Getenv("GOCOVERDIR") == "" && os.Getenv(scenarioCoverEnv) == "" {
		return nothing, nil
	}
	root, err := moduleRoot()
	if err != nil {
		return nothing, err
	}
	// The version tools/bin/itos stamps: tools/bin/dev-version's for this
	// checkout.
	out, err := exec.Command("sh", filepath.Join(root, "tools", "bin", "dev-version"), root).Output()
	version := strings.TrimSpace(string(out))
	if err != nil || version == "" {
		return nothing, fmt.Errorf("the version to stamp the cover build with (tools/bin/dev-version): %v", err)
	}
	dir, err := os.MkdirTemp("", "itos-features-cover-")
	if err != nil {
		return nothing, err
	}
	remove := func() { _ = os.RemoveAll(dir) }
	bin := programPath(filepath.Join(dir, "itos"))
	cmd := exec.Command("go", "build", "-cover", "-covermode=atomic", "-trimpath",
		"-ldflags", "-s -w -X "+versionStamp+"="+version, "-o", bin, "./cmd/itos")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if text, err := cmd.CombinedOutput(); err != nil {
		remove()
		return nothing, fmt.Errorf("building the itos under test with -cover: %v\n%s", err, text)
	}
	coverBin = bin
	return remove, nil
}

// scenarioCoverDir is the folder a scenario's coverage goes in with
// ITOS_CC_TEST_COVERDIR set, made: <ITOS_CC_TEST_COVERDIR>/<its @ID tag
// without the @>. It is "" with the variable unset.
func scenarioCoverDir(s *godog.Scenario) (string, error) {
	top := os.Getenv(scenarioCoverEnv)
	if top == "" {
		return "", nil
	}
	top, err := filepath.Abs(top)
	if err != nil {
		return "", err
	}
	for _, tag := range s.Tags {
		if id, ok := strings.CutPrefix(tag.Name, "@"); ok && strings.HasPrefix(id, "ID-") {
			dir := filepath.Join(top, id)
			return dir, os.MkdirAll(dir, 0o755)
		}
	}
	return "", errors.New("the scenario " + s.Name + " has no @ID tag to name its coverage's folder")
}
