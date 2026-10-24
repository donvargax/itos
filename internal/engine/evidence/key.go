package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/version"
)

// Schema is the version of the record's files; a record of another is not
// read.
const Schema = 1

// Binding is what a key binds that is the binary's: the OS, the
// architecture and itos's own version. A test varies them; a run uses
// ThisBinary's.
type Binding struct{ OS, Arch, Version string }

// ThisBinary is the binding of the itos that runs.
func ThisBinary() Binding {
	return Binding{OS: runtime.GOOS, Arch: runtime.GOARCH, Version: version.Version()}
}

// Pair is an environment variable the key binds: its name, whether it was
// set and its value.
type Pair struct {
	Name  string `json:"name"`
	Set   bool   `json:"set"`
	Value string `json:"value"`
}

// Input is a declared input the key binds: a path, its state (absent, file,
// link or dir) and a hash of its content (of a link, of its target), with
// whether a file is executable.
type Input struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	SHA256 string `json:"sha256,omitempty"`
	Exec   bool   `json:"exec,omitempty"`
}

// Tool is a declared program the key binds: its name, whether the PATH has
// it and a hash of its content.
type Tool struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	SHA256 string `json:"sha256,omitempty"`
}

// Manifest is everything a key binds, and so everything that makes one
// record another: the tree the check ran on, HEAD's commit (or "unborn") for a
// declaration that reads HEAD, the command and its timeout, the declaration, the policy (a hash of the config's settings), the
// binary, the environment and the declared inputs and tools. The mode, run:
// or fails:, is not among them: a record holds the raw exit and each check
// reads its own verdict from it.
type Manifest struct {
	Schema   int             `json:"schema"`
	Tree     string          `json:"tree"`
	Head     string          `json:"head,omitempty"`
	Command  string          `json:"command"`
	Timeout  string          `json:"timeout"`
	Evidence config.Evidence `json:"evidence"`
	Policy   string          `json:"policy"`
	Version  string          `json:"version"`
	OS       string          `json:"os"`
	Arch     string          `json:"arch"`
	Env      []Pair          `json:"env"`
	CIEnv    []Pair          `json:"ci_env,omitempty"`
	Inputs   []Input         `json:"inputs"`
	Tools    []Tool          `json:"tools"`
}

// Key is the manifest's key: the SHA-256 of its JSON, in hex.
func (m Manifest) Key() string {
	text, _ := json.Marshal(m)
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:])
}

// bound is a manifest, and what binding it read that the run needs: the
// untracked files and folders of the declared inputs, to copy into the
// checkout.
type bound struct {
	Manifest
	copies []copying
}

// copying is a declared input git does not track, as the working folder
// holds it: a folder, a file (executable or not) or a link (to its target).
type copying struct {
	path, state, target string
	exec                bool
}

// bind reads what the key binds for a request on the tree.
func (e *Engine) bind(ctx context.Context, req Request, tree string) (bound, error) {
	policy := sha256.Sum256([]byte(canonical(e.Cfg.Settings())))
	decl := req.Decl.Normal()
	b := bound{Manifest: Manifest{
		Schema:   Schema,
		Tree:     tree,
		Command:  config.Normal(req.Command),
		Timeout:  value.Number(req.Seconds),
		Evidence: decl,
		Policy:   hex.EncodeToString(policy[:]),
		Version:  e.Binding.Version,
		OS:       e.Binding.OS,
		Arch:     e.Binding.Arch,
		Env:      []Pair{},
		Inputs:   []Input{},
		Tools:    []Tool{},
	}}
	if decl.Head {
		head, err := e.Git.Head(ctx)
		if err != nil {
			return bound{}, err
		}
		b.Head = head
	}
	for _, name := range decl.Env {
		v, set := os.LookupEnv(name)
		b.Env = append(b.Env, Pair{Name: name, Set: set, Value: v})
	}
	if e.CIEnv != nil {
		names := make([]string, 0, len(e.CIEnv))
		for name := range e.CIEnv {
			names = append(names, name)
		}
		sort.Strings(names)
		b.CIEnv = []Pair{}
		for _, name := range names {
			b.CIEnv = append(b.CIEnv, Pair{Name: name, Set: true, Value: e.CIEnv[name]})
		}
	}
	if len(decl.Inputs) > 0 {
		tracked, err := e.indexed(ctx)
		if err != nil {
			return bound{}, err
		}
		for _, declared := range decl.Inputs {
			if err := e.read(&b, declared, tracked); err != nil {
				return bound{}, err
			}
		}
	}
	for _, name := range decl.Tools {
		tool, err := hashTool(name)
		if err != nil {
			return bound{}, err
		}
		b.Tools = append(b.Tools, tool)
	}
	return b, nil
}

// canonical is JSON text with its object keys in sorted order, so that a
// config written in another order has the same settings. Settings are JSON
// the config wrote itself.
func canonical(text string) string {
	var parsed any
	_ = json.Unmarshal([]byte(text), &parsed)
	again, _ := json.Marshal(parsed)
	return string(again)
}

// read hashes a declared input that git does not track, as the working
// folder holds it, adding what it finds to the manifest and the files to
// copy. A path git tracks is the tree's, and adds nothing; a missing one is
// absent, which is not an empty one; a folder is every file under it that
// git does not track, in the order Walk gives them, a folder named .git
// left out.
func (e *Engine) read(b *bound, declared string, tracked map[string]bool) error {
	if !config.IsRepoPath(declared) {
		return fmt.Errorf("evidence input %q is not a path from the repository's top", declared)
	}
	rel := path.Clean(declared)
	if tracked[rel] {
		return nil
	}
	local := filepath.Join(e.Work, filepath.FromSlash(rel))
	info, err := os.Lstat(local)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		b.Inputs = append(b.Inputs, Input{Path: rel, State: "absent"})
		return nil
	case err != nil:
		return err
	case !info.IsDir():
		return readFile(b, rel, local, info)
	}
	b.Inputs = append(b.Inputs, Input{Path: rel, State: "dir"})
	b.copies = append(b.copies, copying{path: rel, state: "dir"})
	return filepath.Walk(local, func(p string, info fs.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case info.Name() == ".git" && info.IsDir():
			return filepath.SkipDir
		case info.IsDir() || info.Name() == ".git":
			return nil
		}
		under, _ := filepath.Rel(local, p)
		file := path.Join(rel, filepath.ToSlash(under))
		if tracked[file] {
			return nil
		}
		return readFile(b, file, p, info)
	})
}

// readFile hashes one file or link of the working folder.
func readFile(b *bound, rel, local string, info fs.FileInfo) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(local)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(target))
		b.Inputs = append(b.Inputs, Input{Path: rel, State: "link", SHA256: hex.EncodeToString(sum[:])})
		b.copies = append(b.copies, copying{path: rel, state: "link", target: target})
	case info.Mode().IsRegular():
		sum, err := hashFile(local)
		if err != nil {
			return err
		}
		exec := info.Mode()&0o111 != 0
		b.Inputs = append(b.Inputs, Input{Path: rel, State: "file", SHA256: sum, Exec: exec})
		b.copies = append(b.copies, copying{path: rel, state: "file", exec: exec})
	default:
		return fmt.Errorf("%s is neither a file, a link nor a folder", rel)
	}
	return nil
}

// hashFile is the SHA-256 of a file's content, in hex.
func hashFile(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashTool resolves a program on the PATH as the check would find it and
// hashes what it finds; none is absent, and the check runs and fails as it
// would without it.
func hashTool(name string) (Tool, error) {
	found, err := exec.LookPath(name)
	if err != nil {
		return Tool{Name: name, State: "absent"}, nil
	}
	sum, err := hashFile(found)
	if err != nil {
		return Tool{}, err
	}
	return Tool{Name: name, State: "file", SHA256: sum}, nil
}
