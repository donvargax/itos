package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Staging are the git commands that stage files, git add and its other name
// git stage, whose sweeping forms the shim refuses in a repository itos
// manages (slice 128).
var Staging = []string{"add", "stage"}

// addValued are git add's long options that take their value as the next
// argument when it is not joined by "=", and addShortValued its short ones
// that take the rest of their cluster, or the next argument: a value is never
// an option or a path.
var (
	addValued      = []string{"--unified", "--inter-hunk-context", "--chmod", "--pathspec-from-file"}
	addShortValued = "U"
)

// sweeping is the first argument of git add, run in the folder dir, that
// stages more than the files it names, with what it stages: -A, --all or
// --no-ignore-removal (every change in the work tree), -u or --update (every
// change to a tracked file), each in a cluster too (-Av) and as git takes an
// abbreviation of it (--al, --up), or a path that is a folder (. among
// them). "" when there is none: git add of files by their paths. Options are
// read as git reads them, so a valued option's value is neither an option
// nor a path, and everything after "--" is a path.
func sweeping(dir string, args []string) (arg, what string) {
	paths := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, _, joined := strings.Cut(a, "=")
		switch {
		case paths || a == "-" || !strings.HasPrefix(a, "-"):
			if folder(dir, a) {
				return a, "everything in the folder " + a
			}
		case a == "--":
			paths = true
		case abbreviates(a, "--all", len("--al")), abbreviates(a, "--no-ignore-removal", len("--no-ignore-r")):
			return a, "every change in the work tree"
		case abbreviates(a, "--update", len("--up")):
			return a, "every change to a tracked file"
		case strings.HasPrefix(a, "--"):
			if !joined && slices.Contains(addValued, name) {
				i++
			}
		default:
			cluster := a[1:]
			for j := 0; j < len(cluster); j++ {
				switch c := cluster[j]; {
				case c == 'A':
					return "-A", "every change in the work tree"
				case c == 'u':
					return "-u", "every change to a tracked file"
				case strings.IndexByte(addShortValued, c) >= 0:
					if j+1 == len(cluster) {
						i++
					}
					j = len(cluster)
				}
			}
		}
	}
	return "", ""
}

// refuseAdd says why the shim refuses git add's argument, and how to stage
// instead: by naming the files.
func refuseAdd(name, arg, what string) string {
	return fmt.Sprintf("itos: git %s %s would stage %s, which may hold someone else's work in progress; "+
		"stage your own files by their paths instead: git add <path>…", name, arg, what)
}

// folder is whether the path, from the folder dir, is a folder, a link to
// one not followed (git adds the link).
func folder(dir, path string) bool {
	info, err := os.Lstat(filepath.Join(dir, path))
	return err == nil && info.IsDir()
}

// abbreviates is whether arg is the long option full, or the start of it git
// takes for it, at least min characters long.
func abbreviates(arg, full string, min int) bool {
	return len(arg) >= min && strings.HasPrefix(full, arg)
}
