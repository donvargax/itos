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
// In a repository pinned to an itos older than the shim (cli.GitShimSince),
// or with ITOS_VERSION naming one, that itos has no git-shim command to hand
// git commit or git push to, so they run the real git, after one line on
// stderr saying why (bug 7). The pin is read as the launcher reads it, the pin
// alone, from the folder the launcher then reads it in.
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

	"github.com/donvargax/itos/v3/internal/cli"
	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/launch"
	"github.com/donvargax/itos/v3/internal/version"
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
// line, unless the itos they would be handed to is older than the shim
// (tooOld). Otherwise it runs the real git with the arguments and gives its exit
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
			back, err := line.enter()
			if err != nil {
				fmt.Fprintf(stderr, "itos: git -C %s: %s\n", line.dir, err)
				return nil, 128, true
			}
			itos := append([]string{"git-shim", "run", "--", line.name}, line.rest...)
			if !tooOld(line, itos, back, stderr) {
				line.configure(real)
				return itos, 0, false
			}
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

// enter moves to the line's folder, giving the one to come back to: "" when
// it stays where it is, or when the current folder cannot be told (git -C
// with an absolute path from a folder since removed).
func (l gitLine) enter() (back string, err error) {
	if l.dir == "." {
		return "", nil
	}
	back, _ = os.Getwd()
	return back, os.Chdir(l.dir)
}

// tooOld is whether the itos the command would be handed to, named by
// ITOS_VERSION or the repository's pin, is older than the git shim, so has no
// git-shim command to run it (bug 7): then it says so in one line and comes
// back to the folder git was started in, for the real git to run the command
// as it is. A folder it cannot come back to, or a version neither names that
// it can read, hands the command on.
func tooOld(line gitLine, itos []string, back string, stderr io.Writer) bool {
	v, env := launch.Handed(itos)
	if v == "" || version.Compare(v, cli.GitShimSince) >= 0 {
		return false
	}
	if line.dir != "." && (back == "" || os.Chdir(back) != nil) {
		return false
	}
	who := "this repository pins"
	if env {
		who = launch.EnvVersion + " names"
	}
	fmt.Fprintf(stderr, "itos: git %s runs the real git: %s itos %s, and the git shim needs itos %s or later\n",
		line.name, who, v, cli.GitShimSince)
	return true
}

// configure makes the line's -c every git's that itos runs, and the real git
// ITOS_GIT, for itos and all it starts.
func (l gitLine) configure(real string) {
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
	_ = os.Setenv(git.EnvGit, real)
}
