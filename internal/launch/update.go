package launch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/donvargax/itos/v4/internal/release"
	"github.com/donvargax/itos/v4/internal/version"
)

// Keeping to the newest release (features/update.feature). The launcher asks
// the release server for its newest version, <base>/latest/download/
// checksums.txt, whose archive names carry it, at most once a day, and only
// where the answer is used: with no config at all, where it runs the newest
// release, and under a pin, which may have fallen behind. The answer and
// when it was had are kept in the cache, <cache>/state/latest, so a day's
// runs share one question; a question that gets no answer counts as asked,
// so a machine without the network waits on it once a day at most.
//
// CI never asks and never says; ITOS_NO_UPDATE stops the asking and
// ITOS_NO_UPDATE_NOTICE the saying (each when set to anything). A server
// that cannot be reached, or a newest release that cannot be fetched or
// checked, is not an error: the run goes on with what the cache has, saying
// nothing about it.

// The environment that silences the asking and the saying.
const (
	// EnvNoUpdate stops the launcher asking for the newest release.
	EnvNoUpdate = "ITOS_NO_UPDATE"
	// EnvNoUpdateNotice stops it saying a pin has fallen behind.
	EnvNoUpdateNotice = "ITOS_NO_UPDATE_NOTICE"
)

// askTimeout bounds the question, which every hook in a pinned repository
// passes through once a day: a server that does not answer soon is taken as
// one that cannot be reached.
const askTimeout = 3 * time.Second

// day is how long an answer, and a repository's notice, holds.
const day = 24 * time.Hour

// now is the time the daily state is read and written against.
var now = time.Now

// set is whether an environment variable is set to anything.
func set(name string) bool { return os.Getenv(name) != "" }

// mayAsk is whether the launcher may ask for the newest release.
func mayAsk() bool { return !set("CI") && !set(EnvNoUpdate) }

// newest is the newest release the launcher knows of, of this binary's
// version and the releases its cache holds, and whether it is another than
// this binary; the newest release the server announces is fetched into the
// cache first, when the launcher asks and the cache does not have it. A
// binary that does not know its own version (a build without one) runs
// itself and asks nothing.
func newest(own string) (target, bool) {
	if own == version.Unstamped {
		return target{}, false
	}
	cache, err := cacheDir()
	if err != nil {
		return target{}, false
	}
	if mayAsk() {
		if v, sums, asked := announced(cache, true); asked && stable.MatchString(v) && version.Compare(v, own) > 0 {
			dir := filepath.Join(cache, v)
			if !cached(dir, target{version: v}) {
				_ = install(cache, dir, v, sums)
			}
		}
	}
	best := own
	for _, v := range cachedVersions(cache) {
		if version.Compare(v, best) > 0 {
			best = v
		}
	}
	if best == own {
		return target{}, false
	}
	return target{version: best}, true
}

// stable is a release's version without a pre-release: the newest release is
// never one, as GitHub's latest release is not.
var stable = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// cachedVersions are the releases the cache holds, pre-releases left out.
func cachedVersions(cache string) []string {
	entries, err := os.ReadDir(cache)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		v := e.Name()
		if e.IsDir() && stable.MatchString(v) && cached(filepath.Join(cache, v), target{version: v}) {
			found = append(found, v)
		}
	}
	return found
}

// notice says on stderr that a newer release than the pinned one is out, at
// most once a day for the repository whose config is file, from the newest
// version the launcher knows the server announced (asked for first, when
// that is due and allowed).
func notice(file, pin string, stderr io.Writer) {
	if set("CI") || set(EnvNoUpdateNotice) {
		return
	}
	cache, err := cacheDir()
	if err != nil {
		return
	}
	latest, _, _ := announced(cache, mayAsk())
	if latest == "" || version.Compare(latest, pin) <= 0 {
		return
	}
	repo, err := filepath.Abs(file)
	if err != nil {
		return
	}
	said := filepath.Join(cache, "state", "notice-"+release.SHA256([]byte(repo))[:16])
	if _, at, ok := readState(said); ok && now().Sub(at) < day {
		return
	}
	if writeState(said, "") != nil {
		// A notice that cannot be remembered would be said on every run.
		return
	}
	fmt.Fprintf(stderr, "itos %s is out (this repository pins %s): %s\n", latest, pin, release.NotesURL(latest))
}

// announced is the newest version the release server announced, as the
// cache remembers it, "" when it knows none: asked for first when ask allows
// it and the last question is a day old, in which case asked is true and
// sums is the checksums.txt the server answered with, nil without an answer.
func announced(cache string, ask bool) (v string, sums []byte, asked bool) {
	file := filepath.Join(cache, "state", "latest")
	known, at, ok := readState(file)
	if !ask || (ok && now().Sub(at) < day) {
		return known, nil, false
	}
	body, err := release.Get(release.LatestURL("checksums.txt"), askTimeout)
	if err == nil {
		if got := release.VersionOf(body); got != "" {
			known, sums = got, body
		}
	}
	_ = writeState(file, known)
	return known, sums, true
}

// readState reads a state file of the cache: the Unix time it was written at
// and, after a space, what it holds.
func readState(file string) (string, time.Time, bool) {
	text, err := os.ReadFile(file)
	if err != nil {
		return "", time.Time{}, false
	}
	stamp, held, _ := strings.Cut(strings.TrimSpace(string(text)), " ")
	secs, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	return held, time.Unix(secs, 0), true
}

// writeState writes a state file of the cache, stamped with the time now.
func writeState(file, held string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(strconv.FormatInt(now().Unix(), 10)+" "+held+"\n"), 0o644)
}
