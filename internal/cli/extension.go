package cli

// Extensions (features/extensions.feature): a command itos does not have runs
// the program itos-<command> found on the PATH, and only the PATH, with the
// arguments after its name, unread, as git runs git-<command>; its exit code
// is the run's. A built-in command always wins, and a command that is neither
// stays the usage error it was. The global flags written before the name
// apply first (--root is the folder it runs in, and so is the repository's
// top when a run from a subfolder moves there to read its config) and reach
// it in the environment (extensionEnv).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/version"
)

// extensionPrefix starts the name of every extension's program.
const extensionPrefix = "itos-"

// The environment an extension is given.
const (
	envConfig  = "ITOS_CONFIG"  // the config itos would read, absolute
	envRoot    = "ITOS_ROOT"    // the folder itos runs in, after --root or the move to the top, absolute
	envJSON    = "ITOS_JSON"    // 1 with --json, else unset
	envBin     = "ITOS_BIN"     // the itos binary running, to call back
	envVersion = "ITOS_VERSION" // its version, which the launcher keeps a call back to
)

// builtin is whether itos has the command itself.
func builtin(name string) bool {
	_, ok := commands[name]
	return ok || name == "help"
}

// extensionPath is where the PATH has the program of the extension name, ""
// when it has none, when itos has the command itself, or when name could
// reach a program anywhere but the PATH (a path, a flag). A PATH folder that
// is relative is not searched (exec.ErrDot), as Go's LookPath refuses it.
func extensionPath(name string) string {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, `/\:`) || builtin(name) {
		return ""
	}
	path, err := exec.LookPath(extensionPrefix + name)
	if err != nil {
		return ""
	}
	return path
}

// commandAt is the index of the command's name in the arguments, the first
// that is neither a global flag nor a valued one's value, or -1 when a "--"
// or the end comes first.
func commandAt(args []string) int {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--":
			return -1
		case a == "--config" || a == "--root":
			i++
		case switches[a] == nil:
			return i
		}
	}
	return -1
}

// Parse reads the arguments as the command line does: the global flags
// wherever they stand up to a "--" (ParseGlobals), unless the command names an
// extension, whose arguments are its own, given to it unread, so that only
// the global flags before its name count and Extension is its program.
func Parse(args []string) Globals {
	if i := commandAt(args); i >= 0 {
		if path := extensionPath(args[i]); path != "" {
			g := ParseGlobals(args[:i])
			g.Rest = slices.Clone(args[i:])
			g.Extension = path
			return g
		}
	}
	return ParseGlobals(args)
}

// extension is an extension the PATH has: its command's name and its program.
type extension struct{ name, path string }

// extensions is every extension the PATH has, by name: each itos-<name> in
// its folders that extensionPath runs for <name>, the first folder's when
// several have one, as the PATH is searched.
func extensions() []extension {
	seen := map[string]bool{}
	var found []extension
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := strings.CutPrefix(e.Name(), extensionPrefix)
			if runtime.GOOS == "windows" {
				name = strings.TrimSuffix(name, filepath.Ext(name))
			}
			if !ok || seen[name] {
				continue
			}
			seen[name] = true
			if path := extensionPath(name); path != "" {
				found = append(found, extension{name, path})
			}
		}
	}
	slices.SortFunc(found, func(a, b extension) int { return strings.Compare(a.name, b.name) })
	return found
}

// extensionsHelp is the main help's list of the extensions the PATH has, ""
// when it has none, set out as the commands are.
func extensionsHelp() string {
	found := extensions()
	if len(found) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nExtensions (itos-<command> on the PATH):")
	for _, e := range found {
		if len([]rune(e.name)) > 32 {
			fmt.Fprintf(&b, "\n  %s\n  %32s %s", e.name, "", e.path)
		} else {
			fmt.Fprintf(&b, "\n  %-32s %s", e.name, e.path)
		}
	}
	return b.String()
}

// extensionEnv is the environment an extension runs in: itos's own, with what
// the global flags set, and the ITOS_* variables it is given in place of any
// it had.
func extensionEnv(g Globals) ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	file, err := filepath.Abs(config.Path())
	if err != nil {
		return nil, err
	}
	bin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot tell the itos binary running, for %s: %w", envBin, err)
	}
	given := []string{envConfig + "=" + file, envRoot + "=" + root, envBin + "=" + bin, envVersion + "=" + version.Version()}
	if g.JSON {
		given = append(given, envJSON+"=1")
	}
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.Contains([]string{envConfig, envRoot, envJSON, envBin, envVersion}, name) {
			env = append(env, kv)
		}
	}
	return append(env, given...), nil
}

// runExtension runs the extension's program with the arguments in the
// environment extensionEnv gives, and gives its exit code; one that cannot
// start is a missing environment.
func runExtension(g Globals, path string, args []string, o Out) int {
	env, err := extensionEnv(g)
	if err == nil {
		var code int
		code, err = runProgram(path, args, env, o)
		if err == nil {
			return code
		}
	}
	fmt.Fprintf(o.Stderr, "itos: cannot run the extension %s: %s\n", path, err)
	return ExitMissing
}
