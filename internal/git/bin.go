package git

// The real git (slice 41, features/shim.feature). itos can be linked as git
// before the real one on the PATH (internal/shim), so the git itos runs is
// never "git" looked up on the PATH, which may be itos itself: it is Bin, the
// git ITOS_GIT names, else the first git on the PATH that is not this binary.
// cmd/itos exports ITOS_GIT (Export) for everything a run starts, the version
// the launcher hands it to included, so an itos that predates the shim, a
// hook or a check calling git reaches the shim with ITOS_GIT set, and the
// shim passes straight to that git: under an itos run git is always the real
// one, and itos commit can never run itself again through a git that is itos.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// EnvGit names the real git, absolute: set by itos for every program it
// starts, read by every itos those start.
const EnvGit = "ITOS_GIT"

// ErrNoGit is Real's error when the PATH has no git but this binary.
var ErrNoGit = errors.New("no git on the PATH but itos itself")

var (
	binMu  sync.Mutex
	binKey string
	binVal string
)

// Bin is the git itos runs: the one ITOS_GIT names, unless it names this
// binary, else Real's, else "git", which then fails as a missing git does.
// Found again only when ITOS_GIT or the PATH changes.
func Bin() string {
	key := os.Getenv(EnvGit) + "\x00" + os.Getenv("PATH")
	binMu.Lock()
	defer binMu.Unlock()
	if key == binKey && binVal != "" {
		return binVal
	}
	binKey, binVal = key, "git"
	if p := Inherited(); p != "" {
		binVal = p
	} else if p, err := Real(); err == nil {
		binVal = p
	}
	return binVal
}

// Inherited is the git ITOS_GIT names, "" when it is unset or names this
// binary, which would have the shim run itself.
func Inherited() string {
	p := os.Getenv(EnvGit)
	if p == "" || IsSelf(p) {
		return ""
	}
	return p
}

// Export sets ITOS_GIT to the git Bin finds, when it finds one, for every
// program this run starts.
func Export() {
	if b := Bin(); b != "git" {
		_ = os.Setenv(EnvGit, b)
	}
}

// Real is the first git on the PATH that is not this binary, compared as
// files, so a link to it (symbolic or hard) is skipped as it is. A relative
// PATH folder is not searched, as exec.LookPath refuses one (exec.ErrDot).
func Real() (string, error) {
	self := selfInfo()
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range names() {
			p := filepath.Join(dir, name)
			info, err := os.Stat(p)
			if err != nil || !executable(info) || (self != nil && os.SameFile(info, self)) {
				continue
			}
			return p, nil
		}
	}
	return "", ErrNoGit
}

// IsSelf is whether the file at p, its links followed, is this binary.
func IsSelf(p string) bool {
	self := selfInfo()
	if self == nil {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && os.SameFile(info, self)
}

var selfInfo = sync.OnceValue(func() os.FileInfo {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	info, err := os.Stat(exe)
	if err != nil {
		return nil
	}
	return info
})

// names are the file names git may have in a PATH folder: git, or on windows
// git with each of PATHEXT's extensions.
func names() []string {
	if runtime.GOOS != "windows" {
		return []string{"git"}
	}
	exts := os.Getenv("PATHEXT")
	if exts == "" {
		exts = ".com;.exe;.bat;.cmd"
	}
	var found []string
	for _, ext := range strings.Split(strings.ToLower(exts), ";") {
		if ext != "" {
			found = append(found, "git"+ext)
		}
	}
	return found
}

// executable is whether a file can be run: a regular file, with an execute
// bit outside windows.
func executable(info os.FileInfo) bool {
	return info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0)
}
