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
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	"github.com/donvargax/itos/v8/internal/version"
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
	// EnvUpgradeFrom is the version of the itos that handed an upgrade to
	// the version it moves to (handUpgrade); an itos run with it set never
	// hands an upgrade on again.
	EnvUpgradeFrom = "ITOS_UPGRADE_FROM"
)

// GuardSince is the first itos with guard claude-code (v6.0.0, decision 36;
// guard claude-code before it): the launcher answers nothing for the guard
// where it would hand it to an older itos, which has no such command, and
// whose usage error's exit 2 Claude Code would take as a block (slice 44).
const GuardSince = "6.0.0"

// CLI is what the launcher needs of the command line, internal/cli's, which
// cmd/itos gives it: the launcher runs before the command line and imports
// none of it.
type CLI struct {
	// Parse reads the arguments as the command line does.
	Parse func(args []string) Line
	// UpgradeTarget reads the arguments for an itos upgrade the launcher can
	// hand on: ok is false for anything left to this binary, and an error is
	// a version that cannot be had, its kind as upgrade gives it.
	UpgradeTarget func(args []string) (h Handoff, ok bool, err error)
	// ExitCode is the exit code of an error's kind.
	ExitCode func(err error) int
}

// Line is what the launcher reads of the command line, as CLI.Parse reads
// it: the global flags it needs, the command and its arguments in Rest, and
// the program of an extension the command names, "" for none.
type Line struct {
	Help bool
	// Config and Root are "" when not given.
	Config, Root string
	Rest         []string
	Extension    string
}

// Handoff is the upgrade the launcher hands to the version it moves to: that
// version, its checksums.txt and the command line to run it with.
type Handoff struct {
	Version   string
	Checksums []byte
	Args      []string
}

// fetchTimeout bounds each download, so a server that stops answering cannot
// hold a commit for ever.
const fetchTimeout = 5 * time.Minute

// target is a version to run, and the SHA-256 its checksums.txt must have, ""
// when nothing pins one (ITOS_VERSION naming a version the config does not
// pin): then checksums.txt is trusted as fetched.
type target struct{ version, checksums string }

// Args are the arguments as itos reads them: a first argument --version is
// the command version, the rest as they stand (itos --version --json is itos
// version --json). Only the first is read so: -v and -V stay unknown, and a
// --version after a command is that command's.
func Args(args []string) []string {
	if len(args) > 0 && args[0] == "--version" {
		return append([]string{"version"}, args[1:]...)
	}
	return args
}

// Main runs the version of itos the arguments and the environment pick, when
// it is not this binary, and gives that version's exit code. When this binary
// is the one to run, it gives nil, no code at all, and the caller carries on.
// Where a repository's pin has fallen behind the newest release, it says so
// on stderr first (update.go). A release is fetched and run for p; cl reads
// the arguments.
func Main(cl CLI, args []string, stderr io.Writer, p Platform) *int {
	if code, handed := handUpgrade(cl, args, stderr, p); handed {
		return new(code)
	}
	if binaryCommand(cl, args) {
		return nil
	}
	t, ok := choose(cl, args, stderr, p)
	if !ok {
		return nil
	}
	if unguarded(cl, args, t) {
		answerNothing(t, os.Stdin, stderr)
		return new(0)
	}
	bin, err := ensure(t, p)
	var code int
	if err == nil {
		code, err = run(bin, args, t.version)
	}
	if err != nil {
		fmt.Fprintf(stderr, "itos: %s\n", err)
		code = failed(cl, err)
	}
	return new(code)
}

// failed is the exit code of a release the launcher could not fetch, check
// or run: a release server that cannot be reached, or fails, may answer when
// asked again (75, release.Get); any other failure is one this machine does
// not have (3).
func failed(cl CLI, err error) int { return cl.ExitCode(kind.Else(kind.Missing, err)) }

// handUpgrade hands itos upgrade to the version it moves to when that is
// newer than this binary, by semver's precedence alone, a build of a
// checkout included (slice 126): that version is fetched as a pinned one is
// and runs upgrade with the version named, the same flags and
// ITOS_UPGRADE_FROM set to this binary's version, and its exit code is
// upgrade's. It writes nothing of its own but the one line of a version it
// cannot fetch or run, as upgrade says it, exit 75 or 3. It hands nothing
// on when ITOS_UPGRADE_FROM is set, so a release that is not the version it
// was fetched as cannot loop, nor anything cl.UpgradeTarget leaves to this
// binary, nor an upgrade to a version no newer than it.
func handUpgrade(cl CLI, args []string, stderr io.Writer, p Platform) (int, bool) {
	if os.Getenv(EnvUpgradeFrom) != "" {
		return 0, false
	}
	h, ok, err := cl.UpgradeTarget(args)
	own := version.Version()
	if !ok || err == nil && version.Compare(h.Version, own) <= 0 {
		return 0, false
	}
	var bin string
	if err == nil {
		bin, err = ensure(target{h.Version, release.SHA256(h.Checksums)}, p)
	}
	var code int
	if err == nil {
		// Set here, it reaches the version run as the rest of the
		// environment does (environ).
		_ = os.Setenv(EnvUpgradeFrom, own)
		code, err = run(bin, h.Args, h.Version)
	}
	if err != nil {
		fmt.Fprintf(stderr, "itos: %s; every file is left as it was\n", err)
		code = failed(cl, err)
	}
	return code, true
}

// binaryCommand is whether the arguments run one of the launcher's own
// commands, which the binary that was called runs whatever the pin says:
// git-shim install and uninstall link that binary as git (slice 41), so no
// version a pin picks runs them, or the link would point into the cache; pin
// moves the pin (slice 47), which the version it names may predate, and so
// does upgrade (slice 75), when handUpgrade has not handed it on. init
// readies a repository (slice 48), where there is no config to pin a version,
// and the newest release must not run in its place, so it is the binary's
// own only where the config pins nothing: where it pins a version, that
// version runs init as it runs every other command (bug 104), so a newer itos
// called never writes its own agent rules block and hooks over the pin's.
func binaryCommand(cl CLI, args []string) bool {
	rest := cl.Parse(args).Rest
	switch {
	case len(rest) >= 1 && (rest[0] == "pin" || rest[0] == "upgrade"):
		return true
	case len(rest) >= 1 && rest[0] == "init":
		return readConfig(cl, args).state != pinned
	case len(rest) >= 2 && rest[0] == "git-shim":
		return rest[1] == "install" || rest[1] == "uninstall"
	}
	return false
}

// unguarded is whether the arguments run guard claude-code and the version
// they would be handed to predates it, so has no such command to answer it
// (slice 44). A version the launcher cannot read (an ITOS_VERSION that is
// none) is handed on, to fail as it does for any command.
func unguarded(cl CLI, args []string, t target) bool {
	rest := cl.Parse(args).Rest
	return len(rest) >= 2 && rest[0] == "guard" && rest[1] == "claude-code" &&
		config.PinVersion.MatchString(t.version) && version.Compare(t.version, GuardSince) < 0
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
	fmt.Fprintf(stderr, "itos: guard claude-code answers nothing: %s itos %s, and the guard needs itos %s or later\n",
		who, t.version, GuardSince)
}

// choose is the version to run and whether it is another than this binary:
// ITOS_VERSION's, else the pin's, else, with no config at all or a stealth
// config with no pin, the newest release the launcher knows of. A project's
// config with no pin runs this binary.
func choose(cl CLI, args []string, stderr io.Writer, p Platform) (target, bool) {
	own := version.Version()
	c := readConfig(cl, args)
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
		pinLine(cl, args, c.pin.Version, own, stderr)
		notice(c.file, c.pin.Version, own, stderr, p)
		if c.pin.Version == own {
			return target{}, false
		}
		return target{c.pin.Version, c.pin.Checksums}, true
	case absent:
		return newest(own, p)
	}
	return target{}, false
}

// pinLine says on stderr, when the arguments run itos version, that the
// repository pins the version pin and which version own, the itos that was
// called, is: the version line on stdout is the pin's, which alone does not
// say it is a pin's (slice 87). Human output, on stderr
// (docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md).
func pinLine(cl CLI, args []string, pin, own string, stderr io.Writer) {
	g := cl.Parse(args)
	if g.Extension != "" || g.Help || len(g.Rest) == 0 || g.Rest[0] != "version" {
		return
	}
	fmt.Fprintf(stderr, "this repository pins %s; the itos that was called is %s\n", pin, own)
}

// Handed is the version of another itos than this binary that the launcher
// hands the arguments to by name: ITOS_VERSION's, else the pin's, and
// whether ITOS_VERSION named it; "" when it runs this binary, when neither
// names a version it can use (a pin it cannot read, an ITOS_VERSION that is
// no version) or when only the newest release would (no config). It reads
// the pin as choose does, the arguments read by cl, and asks the release
// server nothing.
func Handed(cl CLI, args []string) (v string, env bool) {
	if v = os.Getenv(EnvVersion); v != "" {
		env = true
		if !config.PinVersion.MatchString(v) {
			return "", false
		}
	} else if c := readConfig(cl, args); c.state == pinned {
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
// read as the command line reads them, by cl: for an extension, only those
// before its name). Only the pin is read, not the rest of the config, so a
// config written for a newer itos than the launcher still reaches the
// version it pins.
func readConfig(cl CLI, args []string) foundConfig {
	g := cl.Parse(args)
	// git runs in the folder the process stands in, named, so the command
	// line's locate of the same folder asks git nothing more.
	here, _ := os.Getwd()
	git := vcs.Git{Dir: here}
	root := g.Root
	if g.Config == "" && root == "" {
		root = config.Top(git, "")
	}
	file := g.Config
	if file == "" {
		file = config.Locate(git, root)
	}
	stealth := config.IsStealthIn(git, root, file)
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

// Platform is the OS and the architecture itos runs on, as GOOS and GOARCH
// name them, which cmd/itos detects: a release's archive and its binary are
// named by it.
type Platform struct{ OS, Arch string }

// binary is the binary's name in a release's archive.
func (p Platform) binary() string {
	if p.OS == "windows" {
		return "itos.exe"
	}
	return "itos"
}

// archive is the release's archive for the platform
// (docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md).
func (p Platform) archive(v string) string {
	ext := "tar.gz"
	if p.OS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("itos-%s-%s-%s.%s", v, p.OS, p.Arch, ext)
}

// ensure is the path of the target's binary in the cache, fetched and checked
// first when the cache does not have it, or has it checked against another
// checksums.txt than the one pinned.
func ensure(t target, p Platform) (string, error) {
	cache, err := cacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, t.version)
	bin := filepath.Join(dir, p.binary())
	if cached(dir, t, p) {
		return bin, nil
	}
	if err := fetch(cache, dir, t, p); err != nil {
		return "", err
	}
	return bin, nil
}

// cached is whether the cache holds the target's binary, checked against the
// checksums.txt it pins when it pins one.
func cached(dir string, t target, p Platform) bool {
	if info, err := os.Stat(filepath.Join(dir, p.binary())); err != nil || !info.Mode().IsRegular() {
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
func fetch(cache, dir string, t target, p Platform) error {
	sums, err := release.Get(release.URL(t.version, "checksums.txt"), fetchTimeout)
	if err != nil {
		return fmt.Errorf("cannot fetch itos %s: %w", t.version, err)
	}
	if got := release.SHA256(sums); t.checksums != "" && got != t.checksums {
		return fmt.Errorf("the checksums.txt of itos %s is not the one pinned (pin.checksums): its SHA-256 is %s, the pin %s",
			t.version, got, t.checksums)
	}
	return install(cache, dir, t.version, sums, p)
}

// install fetches the version's archive for the platform, holds it to its
// line in sums, the release's checksums.txt, and puts its binary and sums in
// dir, through a folder of its own beside it that is removed whatever
// happens, so the cache never holds a half or unchecked release.
func install(cache, dir, v string, sums []byte, p Platform) error {
	asset := p.archive(v)
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
	binary, err := extract(archive, asset, p.binary())
	if err != nil {
		return fmt.Errorf("itos %s: %w", v, err)
	}
	return store(cache, dir, binary, sums, p)
}

// extract is the binary, the file name, at the top of a release's archive.
func extract(archive []byte, asset, name string) ([]byte, error) {
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
// folder of the run's own beside it, then moved into place whole, keeping the
// release another run cached there first.
func store(cache, dir string, binary, sums []byte, p Platform) error {
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return fmt.Errorf("cannot create the cache %s: %w", cache, err)
	}
	tmp, err := os.MkdirTemp(cache, "."+filepath.Base(dir)+".")
	if err != nil {
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, p.binary()), binary, 0o755); err != nil {
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "checksums.txt"), sums, 0o644); err != nil {
		return fmt.Errorf("cannot write to the cache %s: %w", cache, err)
	}
	// The checked release goes into place only where there is none: a run that
	// finds the version cached by another meanwhile keeps that one, never
	// removing what the other is about to run (bug 44; on windows a running
	// itos.exe can be neither removed nor replaced).
	sum := target{checksums: release.SHA256(sums)}
	if os.Rename(tmp, dir) == nil || cached(dir, sum, p) {
		return nil
	}
	// What the cache holds for the version was checked against another
	// checksums.txt, or is half of one: it is moved aside, out of the way of
	// the checked release, and removed after.
	old := tmp + ".old"
	if err := os.Rename(dir, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("cannot replace %s in the cache: %w", dir, err)
	}
	defer os.RemoveAll(old)
	if err := os.Rename(tmp, dir); err != nil && !cached(dir, sum, p) {
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
