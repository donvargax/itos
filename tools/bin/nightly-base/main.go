// Command nightly-base prints the commit the nightly's changed-sites mutation step judges from
// (T-161, ADR-0049): the head commit of the last run of nightly.yml on main that concluded
// success, so every Go function changed since the last green night is judged, strictly covered,
// until a green night moves the base.
//
//	base=$(go run ./tools/bin/nightly-base) && tools/bin/pinned itos-cc mutation run --since "$base" … cmd internal
//
// The run is read with gh (gh run list --workflow nightly.yml --branch main --status success
// --limit 1), with the token gh has: the nightly's step hands it github.token, whose actions: read
// lets it list the workflow's runs; by hand, a person's own login. Its head commit must be HEAD or
// an ancestor of HEAD in this clone (the nightly checks out the whole history).
//
// With no green nightly in reach (gh failing, no green run, or a head commit that is not in
// HEAD's history), it says so on stderr and prints the base of the last 24 hours' first-parent
// range instead: the newest commit on HEAD's first-parent line committed 24 hours ago or earlier,
// HEAD itself when nothing newer was committed (a range with no site, which itos-cc passes as not
// applicable), or the line's oldest commit when all of it is younger. It never prints nothing, so
// the step never judges every function of the tree. Stderr names the base it chose and why, so
// the night's log says what was judged.
//
// With -floor <commit> the base is never older than that commit: it prints the later of the floor
// and the last green run's head commit, the floor when that head is the floor or one of its
// ancestors, else the head, and the floor when no green nightly is in reach, in place of the last
// 24 hours' range. It moves the base past a pile of changes no night's batch empties (T-280: the
// nightly passes T-191's head, which judged the slices refactor's last batch, so the nights stop
// drawing from everything since the green night of 2026-10-08). A floor that is not HEAD or one of
// its ancestors is a usage error, read before gh.
//
// With -linux-files it prints instead, one a line, the non-test Go files under the folders it is
// given (relative to the repository's top, the working folder) that a Linux build compiles: the
// files go/build matches for GOOS=linux, so a _windows.go or _darwin.go file, or one whose build
// constraint Linux fails, is left out, as are testdata, folders go ignores and nested modules.
// The nightly step no longer needs them, as itos-cc v0.26.0 leaves out a file only another OS
// builds itself (itos-cc#18); T-191's checks, which replay nightlies run before it, still pass
// them. It never prints no file, which would let itos-cc judge the whole tree: none found is
// exit 2.
//
// It imports nothing but the standard library, as the other gate programs do, and runs git and gh
// by name.
//
// Exit status: 0 a base (or the files) printed, 2 a usage error (a floor out of HEAD's history
// among them), a git that could not read HEAD's history, or no Go file a Linux build compiles.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// window is how far back the fallback range reaches: one night's commits.
const window = 24 * time.Hour

// runner runs a program by name and returns its stdout; an error carries its stderr.
type runner func(name string, args ...string) (string, error)

func main() {
	os.Exit(run(os.Args[1:], execRun, time.Now(), os.Stdout, os.Stderr))
}

func run(args []string, cmd runner, now time.Time, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("nightly-base", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflow := flags.String("workflow", "nightly.yml", "the workflow whose last green run is the base")
	branch := flags.String("branch", "main", "the branch its runs are read on")
	floor := flags.String("floor", "", "a commit in HEAD's history the base is never older than")
	linux := flags.Bool("linux-files", false, "print the non-test Go files under the folders named that a Linux build compiles, not the base")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *linux {
		files, err := linuxFiles(".", flags.Args())
		if err != nil {
			fmt.Fprintf(stderr, "nightly-base: %v\n", err)
			return 2
		}
		for _, f := range files {
			fmt.Fprintln(stdout, f)
		}
		return 0
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "nightly-base: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *floor != "" {
		if _, err := cmd("git", "merge-base", "--is-ancestor", *floor, "HEAD"); err != nil {
			fmt.Fprintf(stderr, "nightly-base: the floor %s is not in HEAD's history: %s\n", *floor, firstLine(err.Error()))
			return 2
		}
	}
	base, note, err := find(cmd, now, *workflow, *branch, *floor)
	if err != nil {
		fmt.Fprintf(stderr, "nightly-base: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "nightly-base: %s\n", note)
	fmt.Fprintln(stdout, base)
	return 0
}

// greenRun is what gh run list says of a run, as much as this reads.
type greenRun struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSha    string `json:"headSha"`
	CreatedAt  string `json:"createdAt"`
}

// find is the base and a sentence saying where it came from: the last green run's head commit,
// or, with none in reach, the last 24 hours' first-parent base. With a floor, already held to
// HEAD's history, it is the later of the floor and that head, and the floor with none in reach.
// An error only when git cannot read HEAD's first-parent line.
func find(cmd runner, now time.Time, workflow, branch, floor string) (string, string, error) {
	r, why := lastGreen(cmd, workflow, branch)
	if why == "" {
		green := fmt.Sprintf("the head commit of the last green run of %s on %s (run %d, %s)", workflow, branch, r.DatabaseID, r.CreatedAt)
		if floor == "" {
			return r.HeadSha, fmt.Sprintf("the base is %s, %s", r.HeadSha, green), nil
		}
		if _, err := cmd("git", "merge-base", "--is-ancestor", r.HeadSha, floor); err == nil {
			return floor, fmt.Sprintf("the base is the floor %s, as %s, %s, is the floor or one of its ancestors", floor, green, r.HeadSha), nil
		}
		return r.HeadSha, fmt.Sprintf("the base is %s, %s, later than the floor %s", r.HeadSha, green, floor), nil
	}
	if floor != "" {
		return floor, fmt.Sprintf("no green nightly in reach (%s); the base is the floor %s", why, floor), nil
	}
	base, err := dayAgo(cmd, now)
	if err != nil {
		return "", "", err
	}
	return base, fmt.Sprintf("no green nightly in reach (%s); judging the last 24 hours' first-parent range instead, since %s", why, base), nil
}

// lastGreen is the last green run of workflow on branch, or why there is none in reach.
func lastGreen(cmd runner, workflow, branch string) (greenRun, string) {
	out, err := cmd("gh", "run", "list", "--workflow", workflow, "--branch", branch, "--status", "success",
		"--limit", "1", "--json", "databaseId,headSha,createdAt")
	if err != nil {
		return greenRun{}, "gh run list failed: " + firstLine(err.Error())
	}
	var runs []greenRun
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		return greenRun{}, "gh run list said no JSON: " + firstLine(out)
	}
	if len(runs) == 0 || runs[0].HeadSha == "" {
		return greenRun{}, fmt.Sprintf("no run of %s on %s concluded success", workflow, branch)
	}
	r := runs[0]
	if _, err := cmd("git", "merge-base", "--is-ancestor", r.HeadSha, "HEAD"); err != nil {
		return greenRun{}, fmt.Sprintf("the last green run's head commit %s is not in HEAD's history", r.HeadSha)
	}
	return r, ""
}

// dayAgo is the newest commit on HEAD's first-parent line committed at least 24 hours before now,
// or the line's oldest when every commit on it is younger.
func dayAgo(cmd runner, now time.Time) (string, error) {
	out, err := cmd("git", "log", "--first-parent", "--format=%H %ct", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git could not read HEAD's first-parent history: %s", firstLine(err.Error()))
	}
	cutoff := now.Add(-window).Unix()
	oldest := ""
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		sha, at, ok := strings.Cut(line, " ")
		seconds, err := strconv.ParseInt(at, 10, 64)
		if !ok || err != nil {
			return "", fmt.Errorf("git log said %q, not a commit and its time", line)
		}
		if seconds <= cutoff {
			return sha, nil
		}
		oldest = sha
	}
	if oldest == "" {
		return "", errors.New("HEAD has no commits")
	}
	return oldest, nil
}

// execRun runs name with args, returning its stdout, or an error naming its stderr.
func execRun(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return string(out), fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

// firstLine is s's first non-empty line, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "(nothing)"
}

// linuxFiles is every non-test Go file under the roots, folders relative to top, that a Linux
// build of top's module compiles, relative to top with forward slashes, in walk order. An error
// when a root cannot be read, a file's build constraint cannot be parsed, or no file is found.
func linuxFiles(top string, roots []string) ([]string, error) {
	if len(roots) == 0 {
		return nil, errors.New("-linux-files needs the folders to read, cmd internal for this repository")
	}
	ctx := build.Default
	ctx.GOOS = "linux"
	var files []string
	for _, root := range roots {
		start := filepath.Join(top, root)
		err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if path == start {
					return nil
				}
				if name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			ok, err := ctx.MatchFile(filepath.Dir(path), name)
			if err != nil {
				return err
			}
			if ok {
				rel, err := filepath.Rel(top, path)
				if err != nil {
					return err
				}
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Go file a Linux build compiles under %s", strings.Join(roots, " "))
	}
	return files, nil
}
