// Package launch is the launcher every run of itos passes through first: it
// picks the version of itos to run and, when that is not the binary that was
// called, fetches that version's release into a cache, checks it and runs it
// with the same arguments, handing back its exit code
// (docs/decisions/0024-a-global-itos-is-a-launcher-that-runs-the-version-a-repository-pins.md,
// features/pin.feature). The binary that was called is never rewritten.
//
// The version to run is ITOS_VERSION when it is set, else the config's
// pin.version, else, where there is no config at all or the stealth config
// pins nothing, the newest release the launcher knows of (update.go,
// features/update.feature, features/stealth.feature). A binary whose own
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
//
// git-shim install and uninstall always run the binary that was called, the
// one they link as git, and so do pin, which moves the pin and may be newer
// than the version pinned (slice 47), upgrade, which moves it too and walks
// the releases since (slice 75), and init, which writes the config and
// its pin where there is none (slice 48); the git shim's own runs (git-shim run) are launched
// as any other, so in a pinned repository git commit is the pinned itos's,
// when it has the shim (Handed tells internal/shim the version, bug 7).
//
// hook pre-tool-use, Claude Code's guard, is handed on as any other run, but
// not to an itos older than the guard (cli.GuardSince), which has no such
// hook: its usage error's exit 2 would make Claude Code block the tool, so
// the launcher reads the input and answers nothing, exit 0 (slice 44).
package launch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/donvargax/itos/v5/internal/cli"
	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/release"
	"github.com/donvargax/itos/v5/internal/value"
	"github.com/donvargax/itos/v5/internal/version"
)

// The environment the launcher reads, and sets for the version it runs.
const (
	// EnvVersion names the version of itos to run, over the pin; the launcher
	// sets it for the version it runs, so it means "the itos version in
	// effect" to whatever that version starts.
	EnvVersion = "ITOS_VERSION"
	// EnvCache is the folder the releases are kept in. The base the releases
	// are fetched from is release.Env, ITOS_RELEASES.
	EnvCache = "ITOS_CACHE"
)

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
	if binaryCommand(args) {
		return 0, false
	}
	t, ok := choose(args, stderr)
	if !ok {
		return 0, false
	}
	if unguarded(args, t) {
		answerNothing(t, os.Stdin, stderr)
		return 0, true
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

// binaryCommand is whether the arguments run one of the launcher's own
// commands, which the binary that was called runs whatever the pin says:
// git-shim install and uninstall link that binary as git (slice 41), so no
// version a pin picks runs them, or the link would point into the cache; pin
// moves the pin (slice 47), which the version it names may predate, and so
// does upgrade (slice 75); init
// readies a repository (slice 48), where there is no config to pin a version,
// and the newest release must not run in its place.
func binaryCommand(args []string) bool {
	rest := cli.Parse(args).Rest
	switch {
	case len(rest) >= 1 && (rest[0] == "pin" || rest[0] == "upgrade" || rest[0] == "init"):
		return true
	case len(rest) >= 2 && rest[0] == "git-shim":
		return rest[1] == "install" || rest[1] == "uninstall"
	}
	return false
}

// unguarded is whether the arguments run hook pre-tool-use and the version
// they would be handed to predates the guard, so has no such hook to answer
// it (slice 44). A version the launcher cannot read (an ITOS_VERSION that is
// none) is handed on, to fail as it does for any command.
func unguarded(args []string, t target) bool {
	rest := cli.Parse(args).Rest
	return len(rest) >= 2 && rest[0] == "hook" && rest[1] == "pre-tool-use" &&
		config.PinVersion.MatchString(t.version) && version.Compare(t.version, cli.GuardSince) < 0
}

// answerNothing is the guard's answer for the version t, which has none:
// nothing on stdout, and on stderr, which Claude Code shows only in its debug
// output, one line saying why. It reads the input Claude Code sends, as the
// guard would, so the writer never meets a closed pipe; a terminal is not
// read, where nothing would end the input.
func answerNothing(t target, stdin *os.File, stderr io.Writer) {
	if info, err := stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice == 0 {
		_, _ = io.Copy(io.Discard, stdin)
	}
	who := "the newest release is"
	switch {
	case os.Getenv(EnvVersion) != "":
		who = EnvVersion + " names"
	case t.checksums != "":
		who = "this repository pins"
	}
	fmt.Fprintf(stderr, "itos: hook pre-tool-use answers nothing: %s itos %s, and the guard needs itos %s or later\n",
		who, t.version, cli.GuardSince)
}

// choose is the version to run and whether it is another than this binary:
// ITOS_VERSION's, else the pin's, else, with no config at all or a stealth
// config with no pin, the newest release the launcher knows of. A project's
// config with no pin runs this binary.
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

// Handed is the version of another itos than this binary that the launcher
// hands the arguments to by name: ITOS_VERSION's, else the pin's, and
// whether ITOS_VERSION named it; "" when it runs this binary, when neither
// names a version it can use (a pin it cannot read, an ITOS_VERSION that is
// no version) or when only the newest release would (no config). It reads
// the pin as choose does, and asks the release server nothing.
func Handed(args []string) (v string, env bool) {
	if v = os.Getenv(EnvVersion); v != "" {
		env = true
		if !config.PinVersion.MatchString(v) {
			return "", false
		}
	} else if c := readConfig(args); c.state == pinned {
		v = c.pin.Version
	}
	if v == version.Version() {
		return "", false
	}
	return v, env
}

// What the launcher finds where itos reads its config.
type configState int

const (
	// unpinned: a config with no pin, or one the launcher cannot read or
	// whose pin it cannot use.
	unpinned configState = iota
	// absent: no config at all, or the stealth config pinning nothing: one
	// person's itos in a repository that does not use it, as where there is
	// none.
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

// readConfig reads the config where itos reads it (--config, ITOS_CONFIG,
// itos.yaml or the stealth config, under --root, or, from a subfolder of a
// repository whose config is at its top, there (config.Top), the global flags
// read as the command line reads them: for an extension, only those before
// its name). Only the pin is read, not the rest of the config, so a config
// written for a newer itos than the launcher still reaches the version it
// pins.
func readConfig(args []string) foundConfig {
	g := cli.Parse(args)
	root := g.Root
	if g.Config == "" && root == "" {
		root = config.Top("")
	}
	file := g.Config
	if file == "" {
		file = config.Locate(root)
	}
	stealth := config.IsStealthIn(root, file)
	if !filepath.IsAbs(file) && root != "" {
		file = filepath.Join(root, file)
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
	if stealth && (pin == nil || pin == value.Undefined) {
		c.state = absent
		return c
	}
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

// archiveName is the release's archive for this platform
// (docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md).
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
	return err == nil && release.SHA256(sums) == t.checksums
}

// fetch downloads the target's release, checks it and puts its binary and
// checksums.txt in dir.
func fetch(cache, dir string, t target) error {
	sums, err := release.Get(release.URL(t.version, "checksums.txt"), fetchTimeout)
	if err != nil {
		return fmt.Errorf("cannot fetch itos %s: %w", t.version, err)
	}
	if got := release.SHA256(sums); t.checksums != "" && got != t.checksums {
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
	want, ok := release.Listed(sums, asset)
	if !ok {
		return fmt.Errorf("the checksums.txt of itos %s lists no %s: no release of it for this platform", v, asset)
	}
	archive, err := release.Get(release.URL(v, asset), fetchTimeout)
	if err != nil {
		return fmt.Errorf("cannot fetch itos %s: %w", v, err)
	}
	if got := release.SHA256(archive); got != want {
		return fmt.Errorf("%s is not the archive the checksums.txt of itos %s lists: its SHA-256 is %s, the list's %s",
			asset, v, got, want)
	}
	binary, err := extract(archive, asset)
	if err != nil {
		return fmt.Errorf("itos %s: %w", v, err)
	}
	return store(cache, dir, binary, sums)
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
		if cached(dir, target{checksums: release.SHA256(sums)}) {
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
