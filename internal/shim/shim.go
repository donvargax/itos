// Package shim is itos started as git (slice 41, features/shim.feature): a
// link named git to the itos binary, in a folder before the real git on the
// PATH (itos git-shim install), makes every git command pass through itos
// first. In a repository itos manages (config.Managed, file checks alone),
// git commit runs as itos commit and git push as itos push, with git's
// arguments, none of them read as itos's global flags; every other command,
// every command in any other folder, and every git started under an itos run
// (ITOS_GIT set, git.EnvGit) runs the real git with its arguments, its
// streams and its exit code untouched. The pass-through costs a start, the
// PATH's lookup and a few file checks: no git runs before the real one, and
// neither the launcher nor the command line is reached.
//
// Of git's options before the command, -C <path> and -c <name>=<value> are
// honoured, since tools write git -C <dir> commit and git -c <key>=<value>
// commit (an editor's, say) for an ordinary commit: the shim moves to the
// folder -C names, as git would, before it looks for the config, and hands
// each -c to every git itos runs in GIT_CONFIG_COUNT, GIT_CONFIG_KEY_<n> and
// GIT_CONFIG_VALUE_<n>. --no-pager and -P change nothing itos prints. Any other
// option before the command (--git-dir, --work-tree, --bare, --namespace,
// --exec-path, …), or GIT_DIR or GIT_WORK_TREE in the environment, names a
// repository the file checks do not follow, so that command runs the real
// git as it is.
package shim

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
)

// Named is whether itos was started under the name git: argv[0]'s base name
// git, or on windows git or git.exe in any case.
func Named(arg0 string) bool {
	name := filepath.Base(arg0)
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(strings.ToLower(name), ".exe")
	}
	return name == "git"
}

// Commands are the git commands the shim runs as itos's in a repository itos
// manages.
var Commands = []string{"commit", "push"}

// Main runs git's command line, the arguments after git. When the command is
// itos's here, it moves to the folder -C names, sets -c for every git itos
// runs and ITOS_GIT to the real git, and gives the itos arguments to run
// (`git-shim run -- <command> <args>…`), for the launcher and the command
// line. Otherwise it runs the real git with the arguments and gives its exit
// code, done; on unix it does not return, git replacing this process. No git
// on the PATH but itos is a missing environment, exit 3.
func Main(args []string, stderr io.Writer) (itos []string, code int, done bool) {
	real := git.Inherited()
	under := real != ""
	if !under {
		p, err := git.Real()
		if err != nil {
			fmt.Fprintln(stderr, "itos: git runs itos's git shim here, and the PATH has no other git to run; "+
				"install git, or remove the shim (itos git-shim uninstall)")
			return nil, 3, true
		}
		real = p
	}
	if !under {
		if line, ok := parse(args); ok && itosCommand(line.name) && !namedRepository() && config.Managed(line.dir) {
			if err := line.apply(real); err != nil {
				fmt.Fprintf(stderr, "itos: git -C %s: %s\n", line.dir, err)
				return nil, 128, true
			}
			return append([]string{"git-shim", "run", "--", line.name}, line.rest...), 0, false
		}
	}
	code, err := run(real, args)
	if err != nil {
		fmt.Fprintf(stderr, "itos: cannot run git (%s): %s\n", real, err)
		return nil, 3, true
	}
	return nil, code, true
}

// gitLine is what the shim reads of git's command line: the folder -C names
// (relative to the current one), each -c, and the command with its
// arguments.
type gitLine struct {
	dir     string
	configs []string
	name    string
	rest    []string
}

// parse reads git's options before the command, as git does: false when one
// is not -C, -c, --no-pager or -P, when a -C or -c has no value, or when
// there is no command.
func parse(args []string) (gitLine, bool) {
	l := gitLine{dir: "."}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-C" || a == "-c":
			if i+1 >= len(args) {
				return l, false
			}
			i++
			if a == "-c" {
				l.configs = append(l.configs, args[i])
			} else if v := args[i]; v != "" {
				// git -C "" leaves the folder as it is.
				if filepath.IsAbs(v) {
					l.dir = v
				} else {
					l.dir = filepath.Join(l.dir, v)
				}
			}
		case a == "--no-pager" || a == "-P":
		case strings.HasPrefix(a, "-"):
			return l, false
		default:
			l.name, l.rest = a, args[i+1:]
			return l, true
		}
	}
	return l, false
}

func itosCommand(name string) bool {
	for _, c := range Commands {
		if c == name {
			return true
		}
	}
	return false
}

// namedRepository is whether the environment names the repository git works
// on, which the file checks do not follow.
func namedRepository() bool {
	return os.Getenv("GIT_DIR") != "" || os.Getenv("GIT_WORK_TREE") != ""
}

// apply makes the line's folder the one itos runs in, its -c every git's
// that itos runs, and the real git ITOS_GIT, for itos and all it starts.
func (l gitLine) apply(real string) error {
	if l.dir != "." {
		if err := os.Chdir(l.dir); err != nil {
			return err
		}
	}
	if len(l.configs) > 0 {
		n, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
		for _, c := range l.configs {
			key, value, given := strings.Cut(c, "=")
			if !given {
				// git -c <name> alone sets the name to true.
				value = "true"
			}
			_ = os.Setenv("GIT_CONFIG_KEY_"+strconv.Itoa(n), key)
			_ = os.Setenv("GIT_CONFIG_VALUE_"+strconv.Itoa(n), value)
			n++
		}
		_ = os.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(n))
	}
	return os.Setenv(git.EnvGit, real)
}
