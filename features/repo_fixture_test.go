package features

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// The git folder every scenario's repository starts from: git init once a
// run, copied into each scenario's folder (T-308), since on windows the git
// init and four git config starts cost a scenario more than all its other
// set-up. Made in the temporary folder the scenarios' are in, so the settings
// git init probes the filesystem for (filemode, symlinks, ignorecase) are the
// ones it would have written there.
var repoTemplate struct {
	once sync.Once
	dir  string
	err  error
}

// templateGitDir is the template's git folder, made on first use.
func (w *world) templateGitDir() (string, error) {
	repoTemplate.once.Do(func() {
		dir, err := os.MkdirTemp("", "itos-features-template-")
		if err != nil {
			repoTemplate.err = err
			return
		}
		repoTemplate.dir = dir
		if err := w.gitIn(dir, "init", "-q", "-b", "main"); err != nil {
			repoTemplate.err = err
			return
		}
		// What git init copies from its templates and nothing reads.
		_ = os.Remove(filepath.Join(dir, ".git", "description"))
		samples, _ := filepath.Glob(filepath.Join(dir, ".git", "hooks", "*.sample"))
		for _, sample := range samples {
			_ = os.Remove(sample)
		}
	})
	if repoTemplate.err != nil {
		return "", repoTemplate.err
	}
	return filepath.Join(repoTemplate.dir, ".git"), nil
}

// removeRepoTemplate removes the folder templateGitDir made.
func removeRepoTemplate() {
	if repoTemplate.dir != "" {
		_ = os.RemoveAll(repoTemplate.dir)
	}
}

// initRepository is git init -q -b main in dir, the template's git folder
// copied into it.
func (w *world) initRepository(dir string) error {
	template, err := w.templateGitDir()
	if err != nil {
		return err
	}
	return copyTree(template, filepath.Join(dir, ".git"))
}

// copyTree copies the folder src, files and folders, to dst.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		from, to := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(from, to); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// gitConfigValue is value as git config writes it after "key = ": bare, or in
// double quotes, with the escapes git reads back, when it has a leading or
// trailing blank or a ; or # that would start a comment.
func gitConfigValue(value string) string {
	var b strings.Builder
	quoted := strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") || strings.ContainsAny(value, ";#")
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	if quoted {
		return `"` + b.String() + `"`
	}
	return b.String()
}

// configSection is a section of a git config as git config --local writes it.
type configSection struct {
	name    string
	entries [][2]string
}

// appendGitConfig writes the sections at the end of the config of the
// repository in dir, when its git folder is a folder of its own, and says so;
// a linked worktree's or a submodule's is left to git config.
func appendGitConfig(dir string, sections ...configSection) (bool, error) {
	gitDir := filepath.Join(dir, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return false, nil
	}
	file := filepath.Join(gitDir, "config")
	existing, err := os.ReadFile(file)
	if err != nil {
		return false, nil
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		b.WriteString("\n")
	}
	for _, section := range sections {
		b.WriteString("[" + section.name + "]\n")
		for _, kv := range section.entries {
			b.WriteString("\t" + kv[0] + " = " + gitConfigValue(kv[1]) + "\n")
		}
	}
	return true, os.WriteFile(file, []byte(b.String()), 0o644)
}

// headSHA is the commit HEAD names in the repository in dir, read from the git
// folder's files: HEAD, and the loose or packed ref it points to. False where
// it is not plain to read (a git folder that is a file, a ref in no file),
// which is git rev-parse's to answer.
func headSHA(dir string) (string, bool) {
	gitDir := filepath.Join(dir, ".git")
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(string(head))
	ref, symbolic := strings.CutPrefix(text, "ref: ")
	if !symbolic {
		return text, isObjectID(text)
	}
	if loose, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(ref))); err == nil {
		sha := strings.TrimSpace(string(loose))
		return sha, isObjectID(sha)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	packed, err := os.ReadFile(filepath.Join(gitDir, "packed-refs"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(packed), "\n") {
		if sha, name, ok := strings.Cut(strings.TrimSpace(line), " "); ok && name == ref {
			return sha, isObjectID(sha)
		}
	}
	return "", false
}

func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}
