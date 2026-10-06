package config

// The stealth mode (features/stealth.feature): one person's itos in a
// repository whose team does not use it. With no --config, no ITOS_CONFIG
// and no itos.yaml in the root, itos reads <git common dir>/itos/itos.yaml,
// the folder git rev-parse --git-common-dir names, which git never commits
// and every linked worktree shares, so nothing has to be set to find it. A
// project's own itos.yaml in the root always wins: that is the project's
// mode.
//
// The files that config names for itos's own data (the ledger, the work
// registry, the questions of itos question and decision records, the smoke sets and
// the notes itos go appends) are read beside it, in that folder; the
// project's own paths (a kind's tests, the scopes' globs, the commands) stay
// the root's. Its defaults fit one person whatever the file says
// (stealthOnly, defaults.go): hooks.bin is itos, the global launcher, and
// there is no work.people, so no people file is read. A config is the stealth one by where it is, not
// by how it was found, so the ITOS_CONFIG an extension is given, which a call
// back reads, reads the same files; a config anywhere else that --config or
// ITOS_CONFIG names resolves its paths from the root, as it always has.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/donvargax/itos/v5/internal/git"
	"github.com/donvargax/itos/v5/internal/source"
	"github.com/donvargax/itos/v5/internal/value"
)

// The config's name, in the root and in the stealth folder.
const fileName = "itos.yaml"

// StealthFolder is the stealth folder's name in the git common dir.
const StealthFolder = "itos"

// Path is the config's path: ITOS_CONFIG, which --config sets, else
// itos.yaml in the current folder, else the stealth config when there is
// one, else itos.yaml, which then reports itself missing.
func Path() string { return Locate("") }

// Locate is the config itos reads for the folder dir ("" for the current
// one), as Path picks it, relative to dir unless it is absolute. The
// launcher calls it before --root applies, with the folder --root names or
// the top Top finds.
func Locate(dir string) string {
	if p := os.Getenv("ITOS_CONFIG"); p != "" {
		return p
	}
	if dir == "" {
		if source.Has(fileName) || source.Worktree.Has(fileName) {
			return fileName
		}
	} else if _, err := os.Stat(filepath.Join(dir, fileName)); err == nil {
		return fileName
	}
	if s := stealthFile(dir); s != "" {
		if _, err := os.Stat(in(dir, s)); err == nil {
			return s
		}
	}
	return fileName
}

// IsStealth is whether the config at file, relative to the current folder,
// is the stealth one: <git common dir>/itos/itos.yaml.
func IsStealth(file string) bool { return IsStealthIn("", file) }

// IsStealthIn is IsStealth for a file relative to the folder dir.
func IsStealthIn(dir, file string) bool {
	// Most configs are an itos.yaml in a root: no git to ask about them.
	if filepath.Base(file) != fileName || filepath.Base(filepath.Dir(file)) != StealthFolder {
		return false
	}
	s := stealthFile(dir)
	if s == "" {
		return false
	}
	a, err := os.Stat(in(dir, file))
	if err != nil {
		return false
	}
	b, err := os.Stat(in(dir, s))
	return err == nil && os.SameFile(a, b)
}

// in is a path relative to dir as the current folder reads it.
func in(dir, p string) string {
	if dir == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

var (
	stealthMu    sync.Mutex
	stealthFiles = map[string]string{}
)

// stealthFile is where the stealth config is for the folder dir, as git
// names the common dir from there (relative to dir, or absolute in a linked
// worktree), whether or not it exists; "" outside a repository. Asked once
// per folder and run.
func stealthFile(dir string) string {
	key := dir
	if cwd, err := os.Getwd(); err == nil {
		key = cwd + "\x00" + dir
	}
	stealthMu.Lock()
	defer stealthMu.Unlock()
	if s, ok := stealthFiles[key]; ok {
		return s
	}
	args := []string{"rev-parse", "--git-common-dir"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	s := ""
	if out, err := git.Output(args...); err == nil {
		if common := strings.TrimSpace(out); common != "" {
			s = filepath.Join(common, StealthFolder, fileName)
		}
	}
	stealthFiles[key] = s
	return s
}

// beside resolves the stealth config's own data, the files it names for
// itos to read, in its folder, in the config as the tools read it (the file
// laid over the defaults), so the typed config and config get agree; an
// absolute path stays where it is.
func beside(tree *value.Map, file string) {
	dir := filepath.Dir(file)
	at := func(m any, key string) {
		holder, ok := m.(*value.Map)
		if !ok {
			return
		}
		if p, ok := holder.At(key).(string); ok && p != "" && !filepath.IsAbs(p) {
			holder.Set(key, filepath.Join(dir, p))
		}
	}
	at(tree.At("ledger"), "files")
	at(tree.At("work"), "registry")
	at(tree.At("work"), "asks")
	at(tree.At("work"), "decisions")
	at(tree.At("guide"), "orchestrating")
	if kinds, ok := tree.At("tests").(*value.Map); ok {
		for _, name := range kinds.Keys() {
			at(value.Prop(kinds.At(name), "smoke"), "file")
		}
	}
}
