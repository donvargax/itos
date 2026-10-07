package git

// The real git (slice 41, features/shim.feature). itos can be linked as git
// before the real one on the PATH (internal/shim), so the git itos runs is
// never "git" looked up on the PATH, which may be an itos: it is Bin, the git
// ITOS_GIT names, else the first git on the PATH that is not an itos (IsItos).
// Not this binary alone (bug 45): the shim links the itos installed globally,
// and the itos running is often another file, a pinned release the launcher
// runs from its cache or a repository's own build, which took the global
// shim for git and ran itos where it meant git. cmd/itos exports ITOS_GIT
// (Export) for everything a run starts, the version the launcher hands it to
// included, so an itos that predates the shim, a hook or a check calling git
// reaches the shim with ITOS_GIT set, and the shim passes straight to that
// git: under an itos run git is always the real one, and itos commit can never
// run itself again through a git that is itos.

import (
	"debug/buildinfo"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/donvargax/itos/v7/internal/kind"
)

// EnvGit names the real git, absolute: set by itos for every program it
// starts, read by every itos those start.
const EnvGit = "ITOS_GIT"

// ErrNoGit is Real's error when the PATH has no git but an itos, a missing
// environment (kind.Missing, exit 3).
var ErrNoGit = kind.Wrap(kind.Missing, errors.New("no git on the PATH but itos"))

var (
	binMu  sync.Mutex
	binKey string
	binVal string
)

// Bin is the git itos runs: the one ITOS_GIT names, unless it names an itos,
// else Real's, else "git", which then fails as a missing git does.
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

// Inherited is the git ITOS_GIT names, "" when it is unset or names an itos,
// which would have the shim run itself or another itos, as one that predates
// bug 45 exports the global shim.
func Inherited() string {
	p := os.Getenv(EnvGit)
	if p == "" || IsItos(p) {
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

// Real is the first git on the PATH that is not an itos (IsItos), so this
// binary's shim and another itos's, linked symbolically or hard, are skipped
// alike. A relative PATH folder is not searched, as exec.LookPath refuses one
// (exec.ErrDot).
func Real() (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range names() {
			p := filepath.Join(dir, name)
			info, err := os.Stat(p)
			if err != nil || !executable(info) || isItos(p, info) {
				continue
			}
			return p, nil
		}
	}
	return "", ErrNoGit
}

// IsItos is whether the program at p is an itos, so never the real git: this
// binary, its links followed; a symbolic link whose target, at any link of
// the chain, is a file named itos (itos.exe), as git-shim install makes one
// and as linksItos (internal/cli) reads it, whatever the target holds; or a
// Go binary built from itos's cmd/itos, any major version, as its build
// information records it, which a hard link or a copy of any itos keeps and
// costs a read of the file, never a run of it.
func IsItos(p string) bool {
	info, err := os.Stat(p)
	return err == nil && isItos(p, info)
}

func isItos(p string, info os.FileInfo) bool {
	if self := selfInfo(); self != nil && os.SameFile(info, self) {
		return true
	}
	return linksNamedItos(p) || builtAsItos(p)
}

// linksNamedItos is whether p is a symbolic link whose target, or the target
// of any link after it, is named itos.
func linksNamedItos(p string) bool {
	for hops := 0; hops < 40; hops++ {
		target, err := os.Readlink(p)
		if err != nil {
			return false
		}
		if strings.TrimSuffix(strings.ToLower(filepath.Base(target)), ".exe") == "itos" {
			return true
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(p), target)
		}
		p = target
	}
	return false
}

// itosMain is the main package of an itos build, at any major version of its
// module.
var itosMain = regexp.MustCompile(`^github\.com/donvargax/itos(/v[0-9]+)?/cmd/itos$`)

// builtAsItos is whether the program at p is a Go binary built from itos's
// cmd/itos.
func builtAsItos(p string) bool {
	info, err := buildinfo.ReadFile(p)
	return err == nil && itosMain.MatchString(info.Path)
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
