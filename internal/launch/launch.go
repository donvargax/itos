// Package launch is the launcher every run of itos passes through first: it
// picks the version of itos to run and, when that is not the binary that was
// called, fetches that version's release into a cache, checks it and runs it
// with the same arguments, handing back its exit code (PLAN.md §10,
// features/pin.feature). The binary that was called is never rewritten.
//
// The version to run is ITOS_VERSION when it is set, else the config's
// pin.version, else, where there is no config at all, the newest release the
// launcher knows of (update.go, features/update.feature). A binary whose own
// version is the one to run runs itself, so the version the launcher runs,
// which it tells by ITOS_VERSION, never launches again. A config with no pin,
// or one the launcher cannot read, runs the binary that was called (which
// then reports what is wrong with the config, if anything).
//
// A release is fetched from ITOS_RELEASES (the GitHub releases of itos by
// default), <base>/download/v<version>/<asset>: its checksums.txt, held to
// pin.checksums when the config pins that version, then the archive for this
// platform, held to its line in checksums.txt. Anything that cannot be
// fetched or does not match exits 3, a missing environment, with a line
// naming what failed; nothing runs in its place, and nothing unverified is
// left in the cache, ITOS_CACHE (itos/ in the user's cache folder by
// default), which holds <version>/itos and the <version>/checksums.txt it was
// checked against.
package launch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/donvargax/itos/v2/internal/cli"
	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/value"
	"github.com/donvargax/itos/v2/internal/version"
)

// The environment the launcher reads, and sets for the version it runs.
const (
	// EnvVersion names the version of itos to run, over the pin; the launcher
	// sets it for the version it runs, so it means "the itos version in
	// effect" to whatever that version starts.
	EnvVersion = "ITOS_VERSION"
	// EnvReleases is the base the releases are fetched from.
	EnvReleases = "ITOS_RELEASES"
	// EnvCache is the folder the releases are kept in.
	EnvCache = "ITOS_CACHE"
)

// DefaultReleases is where releases come from when ITOS_RELEASES is not set.
const DefaultReleases = "https://github.com/donvargax/itos/releases"

// fetchTimeout bounds each download, so a server that stops answering cannot
// hold a commit for ever.
const fetchTimeout = 5 * time.Minute

// target is a version to run, and the SHA-256 its checksums.txt must have, ""
// when nothing pins one (ITOS_VERSION naming a version the config does not
// pin): then checksums.txt is trusted as fetched.
type target struct{ version, checksums string }

// Main runs the version of itos the arguments and the environment pick, when
// it is not this binary: it gives that version's exit code and true. When this
// binary is the one to run, it gives false, and the caller carries on. Where
// a repository's pin has fallen behind the newest release, it says so on
// stderr first (update.go).
func Main(args []string, stderr io.Writer) (int, bool) {
	t, ok := choose(args, stderr)
	if !ok {
		return 0, false
	}
	bin, err := ensure(t)
	if err == nil {
		var code int
		code, err = run(bin, args, t.version)
		if err == nil {
			return code, true
		}
	}
	fmt.Fprintf(stderr, "itos: %s\n", err)
	return cli.ExitMissing, true
}

// choose is the version to run and whether it is another than this binary:
// ITOS_VERSION's, else the pin's, else, with no config at all, the newest
// release the launcher knows of. A config with no pin runs this binary.
func choose(args []string, stderr io.Writer) (target, bool) {
	own := version.Version()
	c := readConfig(args)
	if v := os.Getenv(EnvVersion); v != "" {
		if v == own {
			return target{}, false
		}
		t := target{version: v}
		if c.state == pinned && c.pin.Version == v {
			t.checksums = c.pin.Checksums
		}
		return t, true
	}
	switch c.state {
	case pinned:
		notice(c.file, c.pin.Version, stderr)
		if c.pin.Version == own {
			return target{}, false
		}
		return target{c.pin.Version, c.pin.Checksums}, true
	case absent:
		return newest(own)
	}
	return target{}, false
}

// What the launcher finds where itos reads its config.
type configState int

const (
	// unpinned: a config with no pin, or one the launcher cannot read or
	// whose pin it cannot use.
	unpinned configState = iota
	// absent: no config at all.
	absent
	// pinned: a config pinning a version the launcher can fetch.
	pinned
)

// foundConfig is the config's path, as itos would open it, what the launcher
// finds there and the pin, when there is one.
type foundConfig struct {
	file  string
	state configState
	pin   config.Pin
}

// readConfig reads the config where itos reads it (--config or ITOS_CONFIG,
// under --root). Only the pin is read, not the rest of the config, so a
// config written for a newer itos than the launcher still reaches the
// version it pins.
func readConfig(args []string) foundConfig {
	g := cli.ParseGlobals(args)
	file := g.Config
	if file == "" {
		file = config.Path()
	}
	if !filepath.IsAbs(file) && g.Root != "" {
		file = filepath.Join(g.Root, file)
	}
	c := foundConfig{file: file}
	text, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		c.state = absent
		return c
	}
	if err != nil {
		return c
	}
	tree, err := value.Parse(string(text))
	if err != nil {
		return c
	}
	pin := value.Prop(tree, "pin")
	v, _ := value.Prop(pin, "version").(string)
	sums, _ := value.Prop(pin, "checksums").(string)
	if !config.PinVersion.MatchString(v) || !config.PinChecksums.MatchString(sums) {
		return c
	}
	c.state, c.pin = pinned, config.Pin{Version: v, Checksums: sums}
	return c
}

// cacheDir is ITOS_CACHE, else itos/ in the user's cache folder.
func cacheDir() (string, error) {
	if dir := os.Getenv(EnvCache); dir != "" {
		return dir, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("no cache folder for itos releases (set %s): %w", EnvCache, err)
	}
	return filepath.Join(dir, "itos"), nil
}

// binaryName is the binary's name in a release's archive.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "itos.exe"
	}
	return "itos"
}

// archiveName is the release's archive for this platform (PLAN.md §10).
func archiveName(v string) string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("itos-%s-%s-%s.%s", v, runtime.GOOS, runtime.GOARCH, ext)
}

// ensure is the path of the target's binary in the cache, fetched and checked
// first when the cache does not have it, or has it checked against another
// checksums.txt than the one pinned.
func ensure(t target) (string, error) {
	cache, err := cacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, t.version)
	bin := filepath.Join(dir, binaryName())
	if cached(dir, t) {
		return bin, nil
	}
	if err := fetch(cache, dir, t); err != nil {
		return "", err
	}
	return bin, nil
}

// cached is whether the cache holds the target's binary, checked against the
// checksums.txt it pins when it pins one.
func cached(dir string, t target) bool {
	if info, err := os.Stat(filepath.Join(dir, binaryName())); err != nil || !info.Mode().IsRegular() {
		return false
	}
	if t.checksums == "" {
		return true
	}
	sums, err := os.ReadFile(filepath.Join(dir, "checksums.txt"))
	return err == nil && sha256Hex(sums) == t.checksums
}

// fetch downloads the target's release, checks it and puts its binary and
// checksums.txt in dir.
func fetch(cache, dir string, t target) error {
	sums, err := get(releaseURL(t.version, "checksums.txt"), fetchTimeout)
	if err != nil {
		return fmt.Errorf("cannot fetch itos %s: %w", t.version, err)
	}
	if got := sha256Hex(sums); t.checksums != "" && got != t.checksums {
		return fmt.Errorf("the checksums.txt of itos %s is not the one pinned (pin.checksums): its SHA-256 is %s, the pin %s",
			t.version, got, t.checksums)
	}
	return install(cache, dir, t.version, sums)
}

// install fetches the version's archive for this platform, holds it to its
// line in sums, the release's checksums.txt, and puts its binary and sums in
// dir, through a folder of its own beside it that is removed whatever
// happens, so the cache never holds a half or unchecked release.
func install(cache, dir, v string, sums []byte) error {
	asset := archiveName(v)
	want, ok := listed(sums, asset)
	if !ok {
		return fmt.Errorf("the checksums.txt of itos %s lists no %s: no release of it for this platform", v, asset)
	}
	archive, err := get(releaseURL(v, asset), fetchTimeout)
	if err != nil {
		return fmt.Errorf("cannot fetch itos %s: %w", v, err)
	}
	if got := sha256Hex(archive); got != want {
		return fmt.Errorf("%s is not the archive the checksums.txt of itos %s lists: its SHA-256 is %s, the list's %s",
			asset, v, got, want)
	}
	binary, err := extract(archive, asset)
	if err != nil {
		return fmt.Errorf("itos %s: %w", v, err)
	}
	return store(cache, dir, binary, sums)
}

// releases is the base the releases are fetched from: ITOS_RELEASES, else
// the GitHub releases of itos.
func releases() string {
	if base := strings.TrimRight(os.Getenv(EnvReleases), "/"); base != "" {
		return base
	}
	return DefaultReleases
}

// releaseURL is where the version's asset is fetched from.
func releaseURL(v, asset string) string {
	return releases() + "/download/v" + v + "/" + asset
}

// get is the body at the address, or why it cannot be had within timeout.
func get(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return body, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// listed is the SHA-256 checksums.txt gives the asset, as sha256sum writes a
// line ("<hex>  <name>", or "<hex> *<name>" in binary mode).
func listed(sums []byte, asset string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// extract is the binary at the top of a release's archive.
func extract(archive []byte, asset string) ([]byte, error) {
	name := binaryName()
	if strings.HasSuffix(asset, ".zip") {
		z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("%s cannot be read: %w", asset, err)
		}
		for _, f := range z.File {
			if f.Name == name {
				r, err := f.Open()
				if err != nil {
					return nil, fmt.Errorf("%s cannot be read: %w", asset, err)
				}
				defer r.Close()
				return io.ReadAll(r)
			}
		}
		return nil, fmt.Errorf("%s holds no %s", asset, name)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read: %w", asset, err)
	}
	t := tar.NewReader(gz)
	for {
		h, err := t.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s holds no %s", asset, name)
		}
		if err != nil {
			return nil, fmt.Errorf("%s cannot be read: %w", asset, err)
		}
		if strings.TrimPrefix(h.Name, "./") == name && h.Typeflag == tar.TypeReg {
			return io.ReadAll(t)
		}
	}
}

// store puts the checked binary and its checksums.txt in dir: written into a
// folder beside it, then moved into place whole.
func store(cache, dir string, binary, sums []byte) error {
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return fmt.Errorf("cannot create the cache %s: %w", cache, err)
	}
	tmp, err := os.MkdirTemp(cache, "."+filepath.Base(dir)+".")
	if err != nil {
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, binaryName()), binary, 0o755); err != nil {
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "checksums.txt"), sums, 0o644); err != nil {
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	// What the cache held for the version was checked against another
	// checksums.txt, or is half of one: the checked release replaces it.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cannot replace %s in the cache: %w", dir, err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another run may have put the same release there meanwhile.
		if cached(dir, target{checksums: sha256Hex(sums)}) {
			return nil
		}
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	return nil
}

// environ is the environment with ITOS_VERSION set to the version run.
func environ(v string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, EnvVersion+"=") {
			env = append(env, kv)
		}
	}
	return append(env, EnvVersion+"="+v)
}
